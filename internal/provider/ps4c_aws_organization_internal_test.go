package provider

import (
	"context"
	"testing"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_ps4c_aws_organization"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// TestMapAwsOrganizationDetailToModel_FullyPopulated exercises every nested
// constructor path in mapAwsOrganizationDetailToModel with a fully populated
// API response. The acceptance tests only ever see a non-onboarded org (every
// optional nested object entirely absent), so the populated path — including
// the deeply nested compute/database type collision that motivated the
// terraform-plugin-codegen-framework fork bump — was otherwise untested.
func TestMapAwsOrganizationDetailToModel_FullyPopulated(t *testing.T) {
	ctx := context.Background()

	coverageDate := openapi_types.Date{Time: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)}
	syncTime := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	onboardingStartedAt := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)

	apiResp := &models.AwsOrganizationDetail{
		ManagementAccountId:  "123456789012",
		DisplayName:          valueToNullable("Acme Prod"),
		SavingsPlansSyncTime: valueToNullable(syncTime),
		OnboardingStatus: &models.AwsOnboardingStatus{
			Compute: &models.AwsOnboardingStatusEntry{
				Status:              models.AwsOnboardingStatusEntryStatus("done"),
				OnboardingStartedAt: valueToNullable(onboardingStartedAt),
			},
			Database: &models.AwsOnboardingStatusEntry{
				Status: models.AwsOnboardingStatusEntryStatus("onboarding"),
			},
		},
		Stats30d: &models.AwsOrganizationStats30d{
			Compute: &models.Stats30dSummary{
				Esr:     valueToNullable(0.42),
				Savings: &models.Money{Amount: "100.50", Currency: "USD"},
			},
			Database: &models.Stats30dSummary{
				Esr:     valueToNullable(0.30),
				Savings: &models.Money{Amount: "50.25", Currency: "USD"},
			},
		},
		SavingsTotals: &models.AwsOrganizationSavingsTotals{
			Compute: &models.AwsSavingsTotals{
				Lifetime: models.Money{Amount: "1000.00", Currency: "USD"},
				Ytd:      models.Money{Amount: "200.00", Currency: "USD"},
			},
			Database: &models.AwsSavingsTotals{
				Lifetime: models.Money{Amount: "500.00", Currency: "USD"},
				Ytd:      models.Money{Amount: "100.00", Currency: "USD"},
			},
		},
		MonthlyPotentialSavings: &models.AwsMonthlyPotentialSavings{
			Compute:  &models.Money{Amount: "75.00", Currency: "USD"},
			Database: &models.Money{Amount: "25.00", Currency: "USD"},
		},
		MonthlyStats: &models.AwsOrganizationDetailAllOf1MonthlyStats{
			Compute: &[]models.AwsMonthlyStatsEntry{
				{
					Month:           "2026-06",
					Esr:             0.42,
					OnDemandCost:    models.Money{Amount: "300.00", Currency: "USD"},
					CostWithSavings: models.Money{Amount: "174.00", Currency: "USD"},
				},
			},
			Database: &[]models.AwsMonthlyStatsEntry{
				{
					Month:           "2026-06",
					Esr:             0.30,
					OnDemandCost:    models.Money{Amount: "150.00", Currency: "USD"},
					CostWithSavings: models.Money{Amount: "105.00", Currency: "USD"},
				},
			},
		},
		DailyCoverage: &models.AwsOrganizationDetailAllOf1DailyCoverage{
			Compute: &[]models.AwsDailyCoverageEntry{
				{
					Date:             coverageDate,
					OnDemandCost:     &models.Money{Amount: "10.00", Currency: "USD"},
					FlexsaveCost:     &models.Money{Amount: "1.00", Currency: "USD"},
					ReservedInstCost: &models.Money{Amount: "2.00", Currency: "USD"},
					SavingsPlanCost:  &models.Money{Amount: "3.00", Currency: "USD"},
					SpotCost:         &models.Money{Amount: "4.00", Currency: "USD"},
				},
			},
			Database: &[]models.AwsDailyCoverageEntry{
				{
					Date:         coverageDate,
					OnDemandCost: &models.Money{Amount: "5.00", Currency: "USD"},
				},
			},
		},
	}

	var data ps4cAwsOrganizationDataSourceModel
	diags := mapAwsOrganizationDetailToModel(ctx, apiResp, &data)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if got := data.ManagementAccountId.ValueString(); got != "123456789012" {
		t.Errorf("management_account_id = %q, want %q", got, "123456789012")
	}
	if got := data.DisplayName.ValueString(); got != "Acme Prod" {
		t.Errorf("display_name = %q, want %q", got, "Acme Prod")
	}
	if data.SavingsPlansSyncTime.IsNull() || data.SavingsPlansSyncTime.IsUnknown() {
		t.Error("savings_plans_sync_time should be known and non-null")
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
	if got := data.OnboardingStatus.Database.Status.ValueString(); got != "onboarding" {
		t.Errorf("onboarding_status.database.status = %q, want %q", got, "onboarding")
	}
	if !data.OnboardingStatus.Database.OnboardingStartedAt.IsNull() {
		t.Error("onboarding_status.database.onboarding_started_at should be null (not set in fixture)")
	}

	if got := data.Stats30d.Compute.Esr.ValueFloat64(); got != 0.42 {
		t.Errorf("stats30d.compute.esr = %v, want 0.42", got)
	}
	if got := data.Stats30d.Compute.Savings.Amount.ValueString(); got != "100.50" {
		t.Errorf("stats30d.compute.savings.amount = %q, want %q", got, "100.50")
	}
	if got := data.Stats30d.Database.Savings.Amount.ValueString(); got != "50.25" {
		t.Errorf("stats30d.database.savings.amount = %q, want %q", got, "50.25")
	}

	if got := data.SavingsTotals.Compute.Lifetime.Amount.ValueString(); got != "1000.00" {
		t.Errorf("savings_totals.compute.lifetime.amount = %q, want %q", got, "1000.00")
	}
	if got := data.SavingsTotals.Compute.Ytd.Amount.ValueString(); got != "200.00" {
		t.Errorf("savings_totals.compute.ytd.amount = %q, want %q", got, "200.00")
	}
	if got := data.SavingsTotals.Database.Lifetime.Amount.ValueString(); got != "500.00" {
		t.Errorf("savings_totals.database.lifetime.amount = %q, want %q", got, "500.00")
	}

	if got := data.MonthlyPotentialSavings.Compute.Amount.ValueString(); got != "75.00" {
		t.Errorf("monthly_potential_savings.compute.amount = %q, want %q", got, "75.00")
	}
	if got := data.MonthlyPotentialSavings.Database.Amount.ValueString(); got != "25.00" {
		t.Errorf("monthly_potential_savings.database.amount = %q, want %q", got, "25.00")
	}

	computeStats := data.MonthlyStats.Compute.Elements()
	if len(computeStats) != 1 {
		t.Fatalf("monthly_stats.compute has %d elements, want 1", len(computeStats))
	}
	statsEntry, ok := computeStats[0].(datasource_ps4c_aws_organization.MonthlyStatsComputeValue)
	if !ok {
		t.Fatalf("monthly_stats.compute[0] has unexpected type %T", computeStats[0])
	}
	if got := statsEntry.Month.ValueString(); got != "2026-06" {
		t.Errorf("monthly_stats.compute[0].month = %q, want %q", got, "2026-06")
	}
	if got := statsEntry.CostWithSavings.Amount.ValueString(); got != "174.00" {
		t.Errorf("monthly_stats.compute[0].cost_with_savings.amount = %q, want %q", got, "174.00")
	}

	computeCoverage := data.DailyCoverage.Compute.Elements()
	if len(computeCoverage) != 1 {
		t.Fatalf("daily_coverage.compute has %d elements, want 1", len(computeCoverage))
	}
	coverageEntry, ok := computeCoverage[0].(datasource_ps4c_aws_organization.ComputeValue)
	if !ok {
		t.Fatalf("daily_coverage.compute[0] has unexpected type %T", computeCoverage[0])
	}
	if got := coverageEntry.FlexsaveCost.Amount.ValueString(); got != "1.00" {
		t.Errorf("daily_coverage.compute[0].flexsave_cost.amount = %q, want %q", got, "1.00")
	}
	if got := coverageEntry.SpotCost.Amount.ValueString(); got != "4.00" {
		t.Errorf("daily_coverage.compute[0].spot_cost.amount = %q, want %q", got, "4.00")
	}

	databaseCoverage := data.DailyCoverage.Database.Elements()
	if len(databaseCoverage) != 1 {
		t.Fatalf("daily_coverage.database has %d elements, want 1", len(databaseCoverage))
	}
	dbCoverageEntry, ok := databaseCoverage[0].(datasource_ps4c_aws_organization.DatabaseValue)
	if !ok {
		t.Fatalf("daily_coverage.database[0] has unexpected type %T", databaseCoverage[0])
	}
	if got := dbCoverageEntry.OnDemandCost.Amount.ValueString(); got != "5.00" {
		t.Errorf("daily_coverage.database[0].on_demand_cost.amount = %q, want %q", got, "5.00")
	}
}
