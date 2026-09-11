package provider

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_ps4c_gcp_settings"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestMapGcpBillingAccountsSettingsToItemsList_FullyPopulated(t *testing.T) {
	ctx := t.Context()

	items := []models.GcpBillingAccountSettingsItem{
		{
			BillingAccountId: "012345-6789AB-CDEF01",
			Services: []models.GcpBillingAccountServiceSettings{
				{
					Service: models.GcpBillingAccountServiceSettingsService("COMPUTE"),
					Settings: models.GcpBillingAccountSettings{
						LastDayOfMonthForPurchase: 25,
						MaximumCommitment:         500.0,
						MinimumCommitment:         10.0,
						Policy:                    models.Policy("automatic"),
						PurchaseMode:              models.GcpBillingAccountSettingsPurchaseMode("regular"),
						Term:                      models.CommitmentTerm("1_YEAR"),
					},
				},
			},
		},
	}

	list, diags := mapGcpBillingAccountsSettingsToItemsList(ctx, items)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	elements := list.Elements()
	if len(elements) != 1 {
		t.Fatalf("items has %d elements, want 1", len(elements))
	}

	item, ok := elements[0].(datasource_ps4c_gcp_settings.ItemsValue)
	if !ok {
		t.Fatalf("items[0] has unexpected type %T", elements[0])
	}

	if got := item.BillingAccountId.ValueString(); got != "012345-6789AB-CDEF01" {
		t.Errorf("billing_account_id = %q, want %q", got, "012345-6789AB-CDEF01")
	}

	services := item.Services.Elements()
	if len(services) != 1 {
		t.Fatalf("services has %d elements, want 1", len(services))
	}

	svc, ok := services[0].(datasource_ps4c_gcp_settings.ServicesValue)
	if !ok {
		t.Fatalf("services[0] has unexpected type %T", services[0])
	}

	if got := svc.Service.ValueString(); got != "COMPUTE" {
		t.Errorf("service = %q, want %q", got, "COMPUTE")
	}

	settings := svc.Settings
	if got := settings.LastDayOfMonthForPurchase.ValueInt64(); got != 25 {
		t.Errorf("last_day_of_month_for_purchase = %d, want 25", got)
	}
	if got := settings.MaximumCommitment.ValueFloat64(); got != 500.0 {
		t.Errorf("maximum_commitment = %v, want 500.0", got)
	}
	if got := settings.MinimumCommitment.ValueFloat64(); got != 10.0 {
		t.Errorf("minimum_commitment = %v, want 10.0", got)
	}
	if got := settings.Policy.ValueString(); got != "automatic" {
		t.Errorf("policy = %q, want %q", got, "automatic")
	}
	if got := settings.PurchaseMode.ValueString(); got != "regular" {
		t.Errorf("purchase_mode = %q, want %q", got, "regular")
	}
	if got := settings.Term.ValueString(); got != "1_YEAR" {
		t.Errorf("term = %q, want %q", got, "1_YEAR")
	}
}

func TestPs4cGcpSettingsRead(t *testing.T) {
	for _, tc := range []struct {
		name       string
		maxResults *int64
		responses  []string
		wantCalls  int
		wantItems  int
		wantCount  int64
	}{
		{
			name: "auto-pagination single page",
			responses: []string{
				`{"items":[{"billingAccountId":"012345-6789AB-CDEF01","services":[]}],"rowCount":1}`,
			},
			wantCalls: 1,
			wantItems: 1,
			wantCount: 1,
		},
		{
			name: "auto-pagination multiple pages",
			responses: []string{
				`{"items":[{"billingAccountId":"ACC-1","services":[]}],"pageToken":"page-2"}`,
				`{"items":[{"billingAccountId":"ACC-2","services":[]}],"pageToken":null}`,
			},
			wantCalls: 2,
			wantItems: 2,
			wantCount: 2,
		},
		{
			name:       "user-controlled pagination",
			maxResults: new(int64(1)),
			responses: []string{
				`{"items":[{"billingAccountId":"ACC-1","services":[]}],"pageToken":"page-2","rowCount":5}`,
			},
			wantCalls: 1,
			wantItems: 1,
			wantCount: 5,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			callIdx := 0
			server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/ps4commitments/v1/gcp/settings" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				if callIdx >= len(tc.responses) {
					t.Fatalf("unexpected call %d", callIdx)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.responses[callIdx]))
				callIdx++
			}))

			resp := readPs4cGcpSettings(t, server, tc.maxResults)
			if resp.Diagnostics.HasError() {
				t.Fatal(resp.Diagnostics)
			}
			if callIdx != tc.wantCalls {
				t.Fatalf("got %d calls, want %d", callIdx, tc.wantCalls)
			}

			var state ps4cGcpSettingsDataSourceModel
			if diags := resp.State.Get(t.Context(), &state); diags.HasError() {
				t.Fatal(diags)
			}

			if len(state.Items.Elements()) != tc.wantItems {
				t.Errorf("got %d items, want %d", len(state.Items.Elements()), tc.wantItems)
			}
			if state.RowCount.ValueInt64() != tc.wantCount {
				t.Errorf("got row_count %d, want %d", state.RowCount.ValueInt64(), tc.wantCount)
			}
		})
	}
}

func TestPs4cGcpSettingsReadErrors(t *testing.T) {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal server error"}`))
	}))

	resp := readPs4cGcpSettings(t, server, nil)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for 500, got none")
	}
	if !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "status 500") {
		t.Errorf("expected error containing 'status 500', got: %v", resp.Diagnostics)
	}
}

func readPs4cGcpSettings(t *testing.T, server *httptest.Server, maxResults *int64) datasource.ReadResponse {
	t.Helper()
	ctx := t.Context()
	httpClient := server.Client()
	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}

	ds := NewPs4cGcpSettingsDataSource()
	var configureResp datasource.ConfigureResponse
	ds.(datasource.DataSourceWithConfigure).Configure(ctx, datasource.ConfigureRequest{ProviderData: client}, &configureResp)
	if configureResp.Diagnostics.HasError() {
		t.Fatal(configureResp.Diagnostics)
	}

	var schemaResp datasource.SchemaResponse
	ds.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
	if diags := schemaResp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatal(diags)
	}

	attrTypes := map[string]tftypes.Type{}
	values := map[string]tftypes.Value{}
	for name, attribute := range schemaResp.Schema.Attributes {
		tfType := attribute.GetType().TerraformType(ctx)
		attrTypes[name] = tfType
		if name == "max_results" && maxResults != nil {
			values[name] = tftypes.NewValue(tfType, *maxResults)
		} else {
			values[name] = tftypes.NewValue(tfType, nil)
		}
	}

	resp := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	ds.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(tftypes.Object{AttributeTypes: attrTypes}, values)}}, &resp)
	return resp
}
