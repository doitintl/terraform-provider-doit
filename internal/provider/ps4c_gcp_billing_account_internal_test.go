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
		MonthlyStats: &[]models.GcpMonthlyStatsByService{
			{
				Service: models.GcpMonthlyStatsByServiceServiceCompute,
				Months: []models.GcpMonthlyStatsEntry{
					{
						Month:           "2026-06",
						Esr:             0.25,
						OnDemandCost:    models.Money{Amount: "1000.00", Currency: "USD"},
						CostWithSavings: models.Money{Amount: "750.00", Currency: "USD"},
					},
				},
			},
			{
				Service: models.GcpMonthlyStatsByServiceServiceCloudSql,
				Months: []models.GcpMonthlyStatsEntry{
					{
						Month:           "2026-06",
						Esr:             0.4,
						OnDemandCost:    models.Money{Amount: "200.00", Currency: "USD"},
						CostWithSavings: models.Money{Amount: "120.00", Currency: "USD"},
					},
				},
			},
		},
		DailyCoverage: &[]models.GcpDailyCoverageByService{
			{
				Service: models.GcpDailyCoverageByServiceServiceCompute,
				Days: []models.GcpDailyCoverageEntry{
					{
						Date:            coverageDate,
						BigQuery:        &models.Money{Amount: "15.00", Currency: "USD"},
						CloudRun:        &models.Money{Amount: "25.00", Currency: "USD"},
						ComputeFlexible: &models.Money{Amount: "30.00", Currency: "USD"},
						OnDemand:        &models.Money{Amount: "50.00", Currency: "USD"},
						ResourceBased:   &models.Money{Amount: "40.00", Currency: "USD"},
					},
				},
			},
			{
				Service: models.GcpDailyCoverageByServiceServiceCloudSql,
				Days: []models.GcpDailyCoverageEntry{
					{
						Date:     coverageDate,
						CloudSql: &models.Money{Amount: "30.00", Currency: "USD"},
						OnDemand: &models.Money{Amount: "10.00", Currency: "USD"},
					},
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

	onboarding := data.OnboardingStatus.Elements()
	if len(onboarding) != 2 {
		t.Fatalf("onboarding_status has %d elements, want 2", len(onboarding))
	}
	computeOnboarding := onboarding[0].(datasource_ps4c_gcp_billing_account.OnboardingStatusValue)
	if got := computeOnboarding.Status.ValueString(); got != "done" {
		t.Errorf("onboarding_status[0].status = %q, want done", got)
	}
	if computeOnboarding.OnboardingStartedAt.IsNull() {
		t.Error("onboarding_status[0].onboarding_started_at should be known")
	}

	stats := data.Stats30d.Elements()
	if len(stats) != 2 {
		t.Fatalf("stats30d has %d elements, want 2", len(stats))
	}
	cloudSQLStats := stats[1].(datasource_ps4c_gcp_billing_account.Stats30dValue)
	if got := cloudSQLStats.Service.ValueString(); got != "cloud_sql" {
		t.Errorf("stats30d[1].service = %q, want cloud_sql", got)
	}
	if got := cloudSQLStats.Savings.Amount.ValueString(); got != "80.25" {
		t.Errorf("stats30d[1].savings.amount = %q, want 80.25", got)
	}

	totals := data.SavingsTotals.Elements()
	if len(totals) != 2 {
		t.Fatalf("savings_totals has %d elements, want 2", len(totals))
	}
	computeTotals := totals[0].(datasource_ps4c_gcp_billing_account.SavingsTotalsValue)
	if got := computeTotals.Lifetime.Amount.ValueString(); got != "5000.00" {
		t.Errorf("savings_totals[0].lifetime.amount = %q, want 5000.00", got)
	}

	monthlyStats := data.MonthlyStats.Elements()
	if len(monthlyStats) != 2 {
		t.Fatalf("monthly_stats has %d elements, want 2", len(monthlyStats))
	}
	computeMonthly := monthlyStats[0].(datasource_ps4c_gcp_billing_account.MonthlyStatsValue)
	computeStats := computeMonthly.Months.Elements()
	statsEntry, ok := computeStats[0].(datasource_ps4c_gcp_billing_account.MonthsValue)
	if !ok {
		t.Fatalf("monthly_stats[0].months[0] has unexpected type %T", computeStats[0])
	}
	if got := statsEntry.Month.ValueString(); got != "2026-06" {
		t.Errorf("monthly_stats[0].months[0].month = %q, want 2026-06", got)
	}
	if got := statsEntry.CostWithSavings.Amount.ValueString(); got != "750.00" {
		t.Errorf("monthly_stats[0].months[0].cost_with_savings.amount = %q, want 750.00", got)
	}

	dailyCoverage := data.DailyCoverage.Elements()
	if len(dailyCoverage) != 2 {
		t.Fatalf("daily_coverage has %d elements, want 2", len(dailyCoverage))
	}
	cloudSQLCoverage := dailyCoverage[1].(datasource_ps4c_gcp_billing_account.DailyCoverageValue)
	cloudSQLDays := cloudSQLCoverage.Days.Elements()
	covEntry, ok := cloudSQLDays[0].(datasource_ps4c_gcp_billing_account.DaysValue)
	if !ok {
		t.Fatalf("daily_coverage[1].days[0] has unexpected type %T", cloudSQLDays[0])
	}
	if got := cloudSQLCoverage.Service.ValueString(); got != "cloud_sql" {
		t.Errorf("daily_coverage[1].service = %q, want cloud_sql", got)
	}
	if got := covEntry.CloudSql.Amount.ValueString(); got != "30.00" {
		t.Errorf("daily_coverage[1].days[0].cloud_sql.amount = %q, want 30.00", got)
	}
}

func TestMapGcpBillingAccountDetailToModel_OmittedComputedListsAreEmpty(t *testing.T) {
	apiResp := &models.GcpBillingAccountDetail{BillingAccountId: "012345-6789AB-CDEF01"}
	var data ps4cGcpBillingAccountDataSourceModel
	diags := mapGcpBillingAccountDetailToModel(t.Context(), apiResp, &data)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if len(data.DailyCoverage.Elements()) != 0 || len(data.MonthlyStats.Elements()) != 0 || len(data.OnboardingStatus.Elements()) != 0 || len(data.SavingsTotals.Elements()) != 0 || len(data.Stats30d.Elements()) != 0 {
		t.Fatal("omitted computed lists should be empty")
	}
	if data.DailyCoverage.IsNull() || data.MonthlyStats.IsNull() || data.OnboardingStatus.IsNull() || data.SavingsTotals.IsNull() || data.Stats30d.IsNull() {
		t.Fatal("omitted computed lists should not be null")
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
