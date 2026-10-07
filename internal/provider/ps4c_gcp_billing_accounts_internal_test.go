package provider

import (
	"fmt"
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
			OnboardingStatus: &[]models.GcpOnboardingStatusByService{
				{
					Service:             models.GcpOnboardingStatusByServiceServiceCompute,
					Status:              models.GcpOnboardingStatusByServiceStatus("done"),
					OnboardingStartedAt: valueToNullable(onboardingStartedAt),
				},
				{
					Service: models.GcpOnboardingStatusByServiceServiceCloudSql,
					Status:  models.GcpOnboardingStatusByServiceStatus("onboarding"),
				},
			},
			Stats30d: &[]models.GcpStats30dByService{
				{
					Service: models.GcpStats30dByServiceServiceCompute,
					Esr:     valueToNullable(0.25),
					Savings: &models.Money{Amount: "250.75", Currency: "USD"},
				},
				{
					Service: models.GcpStats30dByServiceServiceCloudSql,
					Esr:     valueToNullable(0.4),
					Savings: &models.Money{Amount: "80.25", Currency: "USD"},
				},
			},
			SavingsTotals: &[]models.GcpSavingsTotalsByService{
				{
					Service:  models.GcpSavingsTotalsByServiceServiceCompute,
					Lifetime: models.Money{Amount: "5000.00", Currency: "USD"},
					Ytd:      models.Money{Amount: "1200.00", Currency: "USD"},
				},
				{
					Service:  models.GcpSavingsTotalsByServiceServiceCloudSql,
					Lifetime: models.Money{Amount: "800.00", Currency: "USD"},
					Ytd:      models.Money{Amount: "300.00", Currency: "USD"},
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

	onboarding := item.OnboardingStatus.Elements()
	if len(onboarding) != 2 {
		t.Fatalf("onboarding_status has %d elements, want 2", len(onboarding))
	}
	cloudSQLOnboarding := onboarding[1].(datasource_ps4c_gcp_billing_accounts.OnboardingStatusValue)
	if got := cloudSQLOnboarding.Service.ValueString(); got != "cloud_sql" {
		t.Errorf("onboarding_status[1].service = %q, want cloud_sql", got)
	}
	if got := cloudSQLOnboarding.Status.ValueString(); got != "onboarding" {
		t.Errorf("onboarding_status[1].status = %q, want onboarding", got)
	}

	stats := item.Stats30d.Elements()
	if len(stats) != 2 {
		t.Fatalf("stats30d has %d elements, want 2", len(stats))
	}
	computeStats := stats[0].(datasource_ps4c_gcp_billing_accounts.Stats30dValue)
	if got := computeStats.Esr.ValueFloat64(); got != 0.25 {
		t.Errorf("stats30d[0].esr = %v, want 0.25", got)
	}
	if got := computeStats.Savings.Amount.ValueString(); got != "250.75" {
		t.Errorf("stats30d[0].savings.amount = %q, want 250.75", got)
	}

	totals := item.SavingsTotals.Elements()
	if len(totals) != 2 {
		t.Fatalf("savings_totals has %d elements, want 2", len(totals))
	}
	cloudSQLTotals := totals[1].(datasource_ps4c_gcp_billing_accounts.SavingsTotalsValue)
	if got := cloudSQLTotals.Service.ValueString(); got != "cloud_sql" {
		t.Errorf("savings_totals[1].service = %q, want cloud_sql", got)
	}
	if got := cloudSQLTotals.Lifetime.Amount.ValueString(); got != "800.00" {
		t.Errorf("savings_totals[1].lifetime.amount = %q, want 800.00", got)
	}
}

func TestMapGcpBillingAccountsToItemsList_OmittedComputedListsAreEmpty(t *testing.T) {
	list, diags := mapGcpBillingAccountsToItemsList(t.Context(), []models.GcpBillingAccount{{BillingAccountId: "012345-6789AB-CDEF01"}})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	item := list.Elements()[0].(datasource_ps4c_gcp_billing_accounts.ItemsValue)
	for name, value := range map[string]interface{ IsNull() bool }{
		"onboarding_status": item.OnboardingStatus,
		"savings_totals":    item.SavingsTotals,
		"stats30d":          item.Stats30d,
	} {
		if value.IsNull() {
			t.Errorf("%s is null, want an empty list", name)
		}
	}
	if len(item.OnboardingStatus.Elements()) != 0 || len(item.SavingsTotals.Elements()) != 0 || len(item.Stats30d.Elements()) != 0 {
		t.Fatal("omitted computed lists should be empty")
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

			var overrides map[string]tftypes.Value
			if tc.maxResults != nil {
				overrides = map[string]tftypes.Value{
					"max_results": tftypes.NewValue(tftypes.Number, float64(*tc.maxResults)),
				}
			}
			resp := readPs4cGcpBillingAccounts(t, server, overrides)
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

func TestPs4cGcpBillingAccountsDataSource_Read_Pagination(t *testing.T) {
	t.Run("max_results_only sends a single manual call and preserves the returned page_token", func(t *testing.T) {
		t.Parallel()

		var requests []*http.Request
		server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests = append(requests, r)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{
				"items": [
					{"billingAccountId": "012345-6789AB-CDEF01"},
					{"billingAccountId": "012345-6789AB-CDEF02"}
				],
				"pageToken": "next-page-token",
				"rowCount": 2
			}`)
		}))

		resp := readPs4cGcpBillingAccounts(t, server, map[string]tftypes.Value{
			"max_results": tftypes.NewValue(tftypes.Number, 2.0),
		})
		if resp.Diagnostics.HasError() {
			t.Fatalf("Read() returned diagnostics: %v", resp.Diagnostics)
		}

		if len(requests) != 1 {
			t.Fatalf("expected exactly 1 API call in manual mode, got %d", len(requests))
		}
		if got := requests[0].URL.Query().Get("maxResults"); got != "2" {
			t.Errorf("maxResults query param = %q, want %q", got, "2")
		}
		if got := requests[0].URL.Query().Get("pageToken"); got != "" {
			t.Errorf("pageToken query param = %q, want empty", got)
		}

		var data ps4cGcpBillingAccountsDataSourceModel
		if diags := resp.State.Get(t.Context(), &data); diags.HasError() {
			t.Fatalf("failed to read state: %v", diags)
		}
		if got := len(data.Items.Elements()); got != 2 {
			t.Errorf("items count = %d, want 2", got)
		}
		if got := data.RowCount.ValueInt64(); got != 2 {
			t.Errorf("row_count = %d, want 2", got)
		}
		if got := data.PageToken.ValueString(); got != "next-page-token" {
			t.Errorf("page_token = %q, want %q", got, "next-page-token")
		}
	})

	t.Run("page_token_only auto-paginates from the token until exhausted", func(t *testing.T) {
		t.Parallel()

		var requests []*http.Request
		server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests = append(requests, r)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if r.URL.Query().Get("pageToken") == "start-token" {
				_, _ = fmt.Fprint(w, `{
					"items": [{"billingAccountId": "012345-6789AB-CDEF03"}],
					"pageToken": "second-page-token",
					"rowCount": 1
				}`)
				return
			}
			_, _ = fmt.Fprint(w, `{
				"items": [{"billingAccountId": "012345-6789AB-CDEF04"}],
				"rowCount": 1
			}`)
		}))

		resp := readPs4cGcpBillingAccounts(t, server, map[string]tftypes.Value{
			"page_token": tftypes.NewValue(tftypes.String, "start-token"),
		})
		if resp.Diagnostics.HasError() {
			t.Fatalf("Read() returned diagnostics: %v", resp.Diagnostics)
		}

		if len(requests) != 2 {
			t.Fatalf("expected 2 API calls while auto-paginating to exhaustion, got %d", len(requests))
		}
		if got := requests[0].URL.Query().Get("pageToken"); got != "start-token" {
			t.Errorf("first pageToken = %q, want %q", got, "start-token")
		}
		if got := requests[1].URL.Query().Get("pageToken"); got != "second-page-token" {
			t.Errorf("second pageToken = %q, want %q", got, "second-page-token")
		}

		var data ps4cGcpBillingAccountsDataSourceModel
		if diags := resp.State.Get(t.Context(), &data); diags.HasError() {
			t.Fatalf("failed to read state: %v", diags)
		}
		if got := len(data.Items.Elements()); got != 2 {
			t.Errorf("items count = %d, want 2 (accumulated across both pages)", got)
		}
		if got := data.RowCount.ValueInt64(); got != 2 {
			t.Errorf("row_count = %d, want 2", got)
		}
		if !data.PageToken.IsNull() {
			t.Errorf("page_token = %q, want null after auto-pagination completes", data.PageToken.ValueString())
		}
	})

	t.Run("max_results and page_token together send a single manual call with both params", func(t *testing.T) {
		t.Parallel()

		var requests []*http.Request
		server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests = append(requests, r)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{
				"items": [{"billingAccountId": "012345-6789AB-CDEF05"}],
				"pageToken": "third-page-token",
				"rowCount": 1
			}`)
		}))

		resp := readPs4cGcpBillingAccounts(t, server, map[string]tftypes.Value{
			"max_results": tftypes.NewValue(tftypes.Number, 1.0),
			"page_token":  tftypes.NewValue(tftypes.String, "second-page-token"),
		})
		if resp.Diagnostics.HasError() {
			t.Fatalf("Read() returned diagnostics: %v", resp.Diagnostics)
		}

		if len(requests) != 1 {
			t.Fatalf("expected exactly 1 API call when both params are user-controlled, got %d", len(requests))
		}
		if got := requests[0].URL.Query().Get("maxResults"); got != "1" {
			t.Errorf("maxResults query param = %q, want %q", got, "1")
		}
		if got := requests[0].URL.Query().Get("pageToken"); got != "second-page-token" {
			t.Errorf("pageToken query param = %q, want %q", got, "second-page-token")
		}

		var data ps4cGcpBillingAccountsDataSourceModel
		if diags := resp.State.Get(t.Context(), &data); diags.HasError() {
			t.Fatalf("failed to read state: %v", diags)
		}
		if got := len(data.Items.Elements()); got != 1 {
			t.Errorf("items count = %d, want 1", got)
		}
		if got := data.PageToken.ValueString(); got != "third-page-token" {
			t.Errorf("page_token = %q, want %q", got, "third-page-token")
		}
	})
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

func readPs4cGcpBillingAccounts(t *testing.T, server *httptest.Server, overrides map[string]tftypes.Value) datasource.ReadResponse {
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
		if val, ok := overrides[name]; ok {
			values[name] = val
		} else {
			values[name] = tftypes.NewValue(tfType, nil)
		}
	}

	resp := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	ds.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(tftypes.Object{AttributeTypes: attrTypes}, values)}}, &resp)
	return resp
}
