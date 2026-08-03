package provider

import (
	"context"
	"testing"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_ps4c_aws_organizations"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
)

// TestMapAwsOrganizationsToItemsList_FullyPopulated exercises every nested
// constructor path in mapAwsOrganizationsToItemsList with a fully populated
// API response. The live acceptance test only ever sees a non-onboarded org
// (every optional nested object entirely absent), so this package's distinct
// populated-value constructors — generated separately from, and with
// different type-collision resolutions than, the singular data source's
// package — were otherwise untested.
func TestMapAwsOrganizationsToItemsList_FullyPopulated(t *testing.T) {
	ctx := context.Background()

	onboardingStartedAt := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)

	orgs := []models.AwsOrganization{
		{
			ManagementAccountId: "123456789012",
			DisplayName:         valueToNullable("Acme Prod"),
			OnboardingStatus: &models.AwsOnboardingStatus{
				Compute: &models.AwsOnboardingStatusEntry{
					Status:              models.AwsOnboardingStatusEntryStatus("done"),
					OnboardingStartedAt: valueToNullable(onboardingStartedAt),
				},
				Database: &models.AwsOnboardingStatusEntry{
					Status: models.AwsOnboardingStatusEntryStatus("not_started"),
				},
			},
			Stats30d: &models.AwsOrganizationStats30d{
				Compute: &models.Stats30dSummary{
					Esr:     valueToNullable(0.55),
					Savings: &models.Money{Amount: "80.00", Currency: "USD"},
				},
				Database: &models.Stats30dSummary{
					Esr: valueToNullable(0.10),
				},
			},
			SavingsTotals: &models.AwsOrganizationSavingsTotals{
				Compute: &models.AwsSavingsTotals{
					Lifetime: models.Money{Amount: "2000.00", Currency: "USD"},
					Ytd:      models.Money{Amount: "300.00", Currency: "USD"},
				},
				Database: &models.AwsSavingsTotals{
					Lifetime: models.Money{Amount: "400.00", Currency: "USD"},
					Ytd:      models.Money{Amount: "50.00", Currency: "USD"},
				},
			},
			MonthlyPotentialSavings: &models.AwsMonthlyPotentialSavings{
				Compute:  &models.Money{Amount: "60.00", Currency: "USD"},
				Database: &models.Money{Amount: "15.00", Currency: "USD"},
			},
		},
	}

	list, diags := mapAwsOrganizationsToItemsList(ctx, orgs)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	elements := list.Elements()
	if len(elements) != 1 {
		t.Fatalf("items has %d elements, want 1", len(elements))
	}

	item, ok := elements[0].(datasource_ps4c_aws_organizations.ItemsValue)
	if !ok {
		t.Fatalf("items[0] has unexpected type %T", elements[0])
	}

	if got := item.ManagementAccountId.ValueString(); got != "123456789012" {
		t.Errorf("management_account_id = %q, want %q", got, "123456789012")
	}
	if got := item.DisplayName.ValueString(); got != "Acme Prod" {
		t.Errorf("display_name = %q, want %q", got, "Acme Prod")
	}

	if item.OnboardingStatus.IsNull() {
		t.Fatal("onboarding_status should not be null when populated")
	}
	if got := item.OnboardingStatus.Compute.Status.ValueString(); got != "done" {
		t.Errorf("onboarding_status.compute.status = %q, want %q", got, "done")
	}
	if got := item.OnboardingStatus.Database.Status.ValueString(); got != "not_started" {
		t.Errorf("onboarding_status.database.status = %q, want %q", got, "not_started")
	}

	if got := item.Stats30d.Compute.Esr.ValueFloat64(); got != 0.55 {
		t.Errorf("stats30d.compute.esr = %v, want 0.55", got)
	}
	if got := item.Stats30d.Compute.Savings.Amount.ValueString(); got != "80.00" {
		t.Errorf("stats30d.compute.savings.amount = %q, want %q", got, "80.00")
	}
	if !item.Stats30d.Database.Savings.IsNull() {
		t.Error("stats30d.database.savings should be null (not set in fixture)")
	}

	if got := item.SavingsTotals.Compute.Lifetime.Amount.ValueString(); got != "2000.00" {
		t.Errorf("savings_totals.compute.lifetime.amount = %q, want %q", got, "2000.00")
	}
	if got := item.SavingsTotals.Database.Ytd.Amount.ValueString(); got != "50.00" {
		t.Errorf("savings_totals.database.ytd.amount = %q, want %q", got, "50.00")
	}

	// monthly_potential_savings.compute/database in this package are the
	// plain Money-shaped ComputeValue/DatabaseValue types (the first
	// occurrence of the compute/database attribute name in this schema's
	// processing order) — distinct from the singular data source's package,
	// where the same names resolve to the AwsDailyCoverageEntry-shaped types.
	if got := item.MonthlyPotentialSavings.Compute.Amount.ValueString(); got != "60.00" {
		t.Errorf("monthly_potential_savings.compute.amount = %q, want %q", got, "60.00")
	}
	if got := item.MonthlyPotentialSavings.Database.Amount.ValueString(); got != "15.00" {
		t.Errorf("monthly_potential_savings.database.amount = %q, want %q", got, "15.00")
	}
}

// TestMapAwsOrganizationsToItemsList_Empty verifies that an empty result maps
// to an empty (never null) list, per the Computed-Only List Attributes rule.
func TestMapAwsOrganizationsToItemsList_Empty(t *testing.T) {
	ctx := context.Background()

	list, diags := mapAwsOrganizationsToItemsList(ctx, []models.AwsOrganization{})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if list.IsNull() {
		t.Error("expected an empty, non-null list")
	}
	if len(list.Elements()) != 0 {
		t.Errorf("expected 0 elements, got %d", len(list.Elements()))
	}
}
