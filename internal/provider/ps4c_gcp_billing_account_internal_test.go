package provider

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_ps4c_gcp_billing_account"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func TestMapGcpBillingAccountDetailToModel_FullyPopulated(t *testing.T) {
	ctx := t.Context()

	coverageDate := openapi_types.Date{Time: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)}
	syncTime := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	onboardingStartedAt := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)

	apiResp := &models.GcpBillingAccountDetail{
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
		MonthlyStats: &models.GcpBillingAccountDetailAllOf1MonthlyStats{
			Compute: &[]models.GcpMonthlyStatsEntry{
				{
					Month:           "2026-06",
					Esr:             0.25,
					OnDemandCost:    models.Money{Amount: "1000.00", Currency: "USD"},
					CostWithSavings: models.Money{Amount: "750.00", Currency: "USD"},
				},
			},
		},
		DailyCoverage: &models.GcpBillingAccountDetailAllOf1DailyCoverage{
			Compute: &[]models.GcpDailyCoverageEntry{
				{
					Date:          coverageDate,
					BigQuery:      &models.Money{Amount: "15.00", Currency: "USD"},
					CloudRun:      &models.Money{Amount: "25.00", Currency: "USD"},
					CloudSql:      &models.Money{Amount: "30.00", Currency: "USD"},
					OnDemand:      &models.Money{Amount: "50.00", Currency: "USD"},
					ResourceBased: &models.Money{Amount: "40.00", Currency: "USD"},
				},
			},
		},
	}

	var data ps4cGcpBillingAccountDataSourceModel
	diags := mapGcpBillingAccountDetailToModel(ctx, apiResp, &data)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if got := data.BillingAccountId.ValueString(); got != "012345-6789AB-CDEF01" {
		t.Errorf("billing_account_id = %q, want %q", got, "012345-6789AB-CDEF01")
	}
	if got := data.DisplayName.ValueString(); got != "GCP Prod Billing" {
		t.Errorf("display_name = %q, want %q", got, "GCP Prod Billing")
	}
	if got := data.Currency.ValueString(); got != "USD" {
		t.Errorf("currency = %q, want %q", got, "USD")
	}
	if got := data.CudExportHealthy.ValueBool(); !got {
		t.Errorf("cud_export_healthy = %v, want true", got)
	}
	if data.CommitmentsSyncTime.IsNull() || data.CommitmentsSyncTime.IsUnknown() {
		t.Error("commitments_sync_time should be known and non-null")
	}

	if data.OnboardingStatus.IsNull() {
		t.Fatal("onboarding_status should not be null when populated")
	}
	if got := data.OnboardingStatus.Compute.Status.ValueString(); got != "done" {
		t.Errorf("onboarding_status.compute.status = %q, want %q", got, "done")
	}
	if data.OnboardingStatus.Compute.OnboardingStartedAt.IsNull() {
		t.Error("onboarding_status.compute.onboarding_started_at should be known")
	}

	if got := data.Stats30d.Compute.Esr.ValueFloat64(); got != 0.25 {
		t.Errorf("stats30d.compute.esr = %v, want 0.25", got)
	}
	if got := data.Stats30d.Compute.Savings.Amount.ValueString(); got != "250.75" {
		t.Errorf("stats30d.compute.savings.amount = %q, want %q", got, "250.75")
	}

	if got := data.SavingsTotals.Compute.Lifetime.Amount.ValueString(); got != "5000.00" {
		t.Errorf("savings_totals.compute.lifetime.amount = %q, want %q", got, "5000.00")
	}
	if got := data.SavingsTotals.Compute.Ytd.Amount.ValueString(); got != "1200.00" {
		t.Errorf("savings_totals.compute.ytd.amount = %q, want %q", got, "1200.00")
	}

	computeStats := data.MonthlyStats.Compute.Elements()
	if len(computeStats) != 1 {
		t.Fatalf("monthly_stats.compute has %d elements, want 1", len(computeStats))
	}
	statsEntry, ok := computeStats[0].(datasource_ps4c_gcp_billing_account.MonthlyStatsComputeValue)
	if !ok {
		t.Fatalf("monthly_stats.compute[0] has unexpected type %T", computeStats[0])
	}
	if got := statsEntry.Month.ValueString(); got != "2026-06" {
		t.Errorf("monthly_stats.compute[0].month = %q, want %q", got, "2026-06")
	}
	if got := statsEntry.CostWithSavings.Amount.ValueString(); got != "750.00" {
		t.Errorf("monthly_stats.compute[0].cost_with_savings.amount = %q, want %q", got, "750.00")
	}

	computeCoverage := data.DailyCoverage.Compute.Elements()
	if len(computeCoverage) != 1 {
		t.Fatalf("daily_coverage.compute has %d elements, want 1", len(computeCoverage))
	}
	covEntry, ok := computeCoverage[0].(datasource_ps4c_gcp_billing_account.ComputeValue)
	if !ok {
		t.Fatalf("daily_coverage.compute[0] has unexpected type %T", computeCoverage[0])
	}
	if got := covEntry.BigQuery.Amount.ValueString(); got != "15.00" {
		t.Errorf("daily_coverage.compute[0].big_query.amount = %q, want %q", got, "15.00")
	}
	if got := covEntry.ResourceBased.Amount.ValueString(); got != "40.00" {
		t.Errorf("daily_coverage.compute[0].resource_based.amount = %q, want %q", got, "40.00")
	}
}

func TestPs4cGcpBillingAccountRead(t *testing.T) {
	body := `{
		"billingAccountId": "012345-6789AB-CDEF01",
		"displayName": "Test Billing Account",
		"currency": "USD",
		"cudExportHealthy": true
	}`

	calls := 0
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodGet || r.URL.Path != "/ps4commitments/v1/gcp/billing-accounts/012345-6789AB-CDEF01" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))

	resp := readPs4cGcpBillingAccount(t, server, "012345-6789AB-CDEF01")
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if calls != 1 {
		t.Fatalf("got %d calls, want 1", calls)
	}

	var state ps4cGcpBillingAccountDataSourceModel
	if diags := resp.State.Get(t.Context(), &state); diags.HasError() {
		t.Fatal(diags)
	}

	if got := state.BillingAccountId.ValueString(); got != "012345-6789AB-CDEF01" {
		t.Errorf("billing_account_id = %q, want %q", got, "012345-6789AB-CDEF01")
	}
	if got := state.DisplayName.ValueString(); got != "Test Billing Account" {
		t.Errorf("display_name = %q, want %q", got, "Test Billing Account")
	}
	if got := state.Currency.ValueString(); got != "USD" {
		t.Errorf("currency = %q, want %q", got, "USD")
	}
}

func TestPs4cGcpBillingAccountRead_NotFound(t *testing.T) {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message": "account not found"}`))
	}))

	resp := readPs4cGcpBillingAccount(t, server, "NONEXISTENT")
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for 404, got none")
	}
	if !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "not found") {
		t.Errorf("expected error containing 'not found', got: %v", resp.Diagnostics)
	}
}

func readPs4cGcpBillingAccount(t *testing.T, server *httptest.Server, billingAccountId string) datasource.ReadResponse {
	t.Helper()
	ctx := t.Context()
	httpClient := server.Client()
	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}

	ds := NewPs4cGcpBillingAccountDataSource()
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
		if name == "billing_account_id" {
			values[name] = tftypes.NewValue(tfType, billingAccountId)
		} else {
			values[name] = tftypes.NewValue(tfType, nil)
		}
	}

	resp := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	ds.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(tftypes.Object{AttributeTypes: attrTypes}, values)}}, &resp)
	return resp
}
