package provider

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_ps4c_gcp_billing_accounts"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestMapGcpBillingAccountsToItemsList_FullyPopulated(t *testing.T) {
	ctx := t.Context()

	syncTime := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	onboardingStartedAt := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)

	accounts := []models.GcpBillingAccount{
		{
			BillingAccountId:    "012345-6789AB-CDEF01",
			DisplayName:         valueToNullable("GCP Prod Billing"),
			Currency:            valueToNullable("USD"),
			CudExportHealthy:    valueToNullable(true),
			CommitmentsSyncTime: valueToNullable(syncTime),
			OnboardingStatus: &models.GcpOnboardingStatus{
				Compute: &models.GcpOnboardingStatusEntry{
					Status:              models.GcpOnboardingStatusEntryStatus("done"),
					OnboardingStartedAt: valueToNullable(onboardingStartedAt),
				},
			},
			Stats30d: &models.GcpBillingAccountStats30d{
				Compute: &models.Stats30dSummary{
					Esr:     valueToNullable(0.25),
					Savings: &models.Money{Amount: "250.75", Currency: "USD"},
				},
			},
			SavingsTotals: &models.GcpBillingAccountSavingsTotals{
				Compute: &models.GcpSavingsTotals{
					Lifetime: models.Money{Amount: "5000.00", Currency: "USD"},
					Ytd:      models.Money{Amount: "1200.00", Currency: "USD"},
				},
			},
		},
	}

	list, diags := mapGcpBillingAccountsToItemsList(ctx, accounts)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	elements := list.Elements()
	if len(elements) != 1 {
		t.Fatalf("items has %d elements, want 1", len(elements))
	}

	item, ok := elements[0].(datasource_ps4c_gcp_billing_accounts.ItemsValue)
	if !ok {
		t.Fatalf("items[0] has unexpected type %T", elements[0])
	}

	if got := item.BillingAccountId.ValueString(); got != "012345-6789AB-CDEF01" {
		t.Errorf("billing_account_id = %q, want %q", got, "012345-6789AB-CDEF01")
	}
	if got := item.DisplayName.ValueString(); got != "GCP Prod Billing" {
		t.Errorf("display_name = %q, want %q", got, "GCP Prod Billing")
	}
	if got := item.Currency.ValueString(); got != "USD" {
		t.Errorf("currency = %q, want %q", got, "USD")
	}
	if got := item.CudExportHealthy.ValueBool(); !got {
		t.Errorf("cud_export_healthy = %v, want true", got)
	}

	if item.OnboardingStatus.IsNull() {
		t.Fatal("onboarding_status should not be null when populated")
	}
	if got := item.OnboardingStatus.Compute.Status.ValueString(); got != "done" {
		t.Errorf("onboarding_status.compute.status = %q, want %q", got, "done")
	}

	if got := item.Stats30d.Compute.Esr.ValueFloat64(); got != 0.25 {
		t.Errorf("stats30d.compute.esr = %v, want 0.25", got)
	}
	if got := item.Stats30d.Compute.Savings.Amount.ValueString(); got != "250.75" {
		t.Errorf("stats30d.compute.savings.amount = %q, want %q", got, "250.75")
	}

	if got := item.SavingsTotals.Compute.Lifetime.Amount.ValueString(); got != "5000.00" {
		t.Errorf("savings_totals.compute.lifetime.amount = %q, want %q", got, "5000.00")
	}
	if got := item.SavingsTotals.Compute.Ytd.Amount.ValueString(); got != "1200.00" {
		t.Errorf("savings_totals.compute.ytd.amount = %q, want %q", got, "1200.00")
	}
}

func TestPs4cGcpBillingAccountsRead(t *testing.T) {
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
				`{"items":[{"billingAccountId":"012345-6789AB-CDEF01","displayName":"Account 1"}],"rowCount":1}`,
			},
			wantCalls: 1,
			wantItems: 1,
			wantCount: 1,
		},
		{
			name: "auto-pagination multiple pages",
			responses: []string{
				`{"items":[{"billingAccountId":"ACC-1"}],"pageToken":"next-page"}`,
				`{"items":[{"billingAccountId":"ACC-2"}],"pageToken":null}`,
			},
			wantCalls: 2,
			wantItems: 2,
			wantCount: 2,
		},
		{
			name:       "user-controlled pagination",
			maxResults: new(int64(1)),
			responses: []string{
				`{"items":[{"billingAccountId":"ACC-1"}],"pageToken":"next-page","rowCount":5}`,
			},
			wantCalls: 1,
			wantItems: 1,
			wantCount: 5,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			callIdx := 0
			server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/ps4commitments/v1/gcp/billing-accounts" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				if callIdx >= len(tc.responses) {
					t.Fatalf("unexpected call %d", callIdx)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.responses[callIdx]))
				callIdx++
			}))

			resp := readPs4cGcpBillingAccounts(t, server, tc.maxResults)
			if resp.Diagnostics.HasError() {
				t.Fatal(resp.Diagnostics)
			}
			if callIdx != tc.wantCalls {
				t.Fatalf("got %d calls, want %d", callIdx, tc.wantCalls)
			}

			var state ps4cGcpBillingAccountsDataSourceModel
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

func TestPs4cGcpBillingAccountsReadErrors(t *testing.T) {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal server error"}`))
	}))

	resp := readPs4cGcpBillingAccounts(t, server, nil)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for 500, got none")
	}
	if !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "status 500") {
		t.Errorf("expected error containing 'status 500', got: %v", resp.Diagnostics)
	}
}

func readPs4cGcpBillingAccounts(t *testing.T, server *httptest.Server, maxResults *int64) datasource.ReadResponse {
	t.Helper()
	ctx := t.Context()
	httpClient := server.Client()
	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}

	ds := NewPs4cGcpBillingAccountsDataSource()
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
