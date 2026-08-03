package provider

import (
	"context"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_ps4c_aws_organizations"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// mapAwsOrganizationsToItemsList maps a ListAwsOrganizations API response's
// items into the ps4c_aws_organizations data source's list attribute. This
// data source generates its own distinct nested-object types from
// datasource_ps4c_aws_organizations, even though several are structurally
// identical to the ones in datasource_ps4c_aws_organization used by the
// singular data source's mapper in ps4c_aws_organization.go — the two
// packages cannot share constructors, so the mapping logic is intentionally
// duplicated per package rather than genericized across them.
func mapAwsOrganizationsToItemsList(ctx context.Context, orgs []models.AwsOrganization) (types.List, diag.Diagnostics) {
	elemType := datasource_ps4c_aws_organizations.ItemsValue{}.Type(ctx)
	attrTypes := datasource_ps4c_aws_organizations.ItemsValue{}.AttributeTypes(ctx)

	return buildOrgsObjectList(elemType, attrTypes, len(orgs),
		func(i int) (map[string]attr.Value, diag.Diagnostics) { return awsOrganizationItemAttrs(ctx, orgs[i]) },
		datasource_ps4c_aws_organizations.NewItemsValue,
	)
}

func awsOrganizationItemAttrs(ctx context.Context, org models.AwsOrganization) (map[string]attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics

	savingsPlansSyncTime := types.StringNull()
	if syncTime := nullableToPointer(org.SavingsPlansSyncTime); syncTime != nil {
		savingsPlansSyncTime = types.StringValue(syncTime.UTC().Format(time.RFC3339))
	}

	onboardingStatus, d := mapItemsOnboardingStatus(ctx, org.OnboardingStatus)
	diags.Append(d...)

	stats30d, d := mapItemsStats30d(ctx, org.Stats30d)
	diags.Append(d...)

	savingsTotals, d := mapItemsSavingsTotals(ctx, org.SavingsTotals)
	diags.Append(d...)

	monthlyPotentialSavings, d := mapItemsMonthlyPotentialSavings(ctx, org.MonthlyPotentialSavings)
	diags.Append(d...)

	return map[string]attr.Value{
		"display_name":              types.StringPointerValue(nullableToPointer(org.DisplayName)),
		"management_account_id":     types.StringValue(org.ManagementAccountId),
		"savings_plans_sync_time":   savingsPlansSyncTime,
		"onboarding_status":         onboardingStatus,
		"stats30d":                  stats30d,
		"savings_totals":            savingsTotals,
		"monthly_potential_savings": monthlyPotentialSavings,
	}, diags
}

// moneyAttrsItems builds the {amount, currency} attribute map shared by every
// generated Money-shaped nested object in this package.
func moneyAttrsItems(amount, currency string) map[string]attr.Value {
	return map[string]attr.Value{
		"amount":   types.StringValue(amount),
		"currency": types.StringValue(currency),
	}
}

// buildOrgsMoneyValue mirrors buildMoneyValue (ps4c_aws_organization.go) for
// this package's independently-generated types.
func buildOrgsMoneyValue[V attr.Value](m *models.Money, attrTypes map[string]attr.Type, newFn func(map[string]attr.Type, map[string]attr.Value) (V, diag.Diagnostics), newNullFn func() V) (V, diag.Diagnostics) {
	if m == nil {
		return newNullFn(), nil
	}
	return newFn(attrTypes, moneyAttrsItems(m.Amount, m.Currency))
}

// buildOrgsObjectList mirrors buildObjectList (ps4c_aws_organization.go) for
// this package's independently-generated types.
func buildOrgsObjectList[V attr.Value](elemType attr.Type, attrTypes map[string]attr.Type, n int, attrsAt func(i int) (map[string]attr.Value, diag.Diagnostics), newFn func(map[string]attr.Type, map[string]attr.Value) (V, diag.Diagnostics)) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics

	if n == 0 {
		return types.ListValueMust(elemType, []attr.Value{}), diags
	}

	vals := make([]attr.Value, 0, n)
	for i := range n {
		a, d := attrsAt(i)
		diags.Append(d...)
		v, d := newFn(attrTypes, a)
		diags.Append(d...)
		vals = append(vals, v)
	}

	list, d := types.ListValue(elemType, vals)
	diags.Append(d...)
	return list, diags
}

// --- onboarding_status ---

func mapItemsOnboardingStatus(ctx context.Context, status *models.AwsOnboardingStatus) (datasource_ps4c_aws_organizations.OnboardingStatusValue, diag.Diagnostics) {
	if status == nil {
		return datasource_ps4c_aws_organizations.NewOnboardingStatusValueNull(), nil
	}

	var diags diag.Diagnostics

	compute, d := mapItemsOnboardingStatusComputeEntry(ctx, status.Compute)
	diags.Append(d...)

	database, d := mapItemsOnboardingStatusDatabaseEntry(ctx, status.Database)
	diags.Append(d...)

	val, d := datasource_ps4c_aws_organizations.NewOnboardingStatusValue(
		datasource_ps4c_aws_organizations.OnboardingStatusValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"compute": compute, "database": database},
	)
	diags.Append(d...)
	return val, diags
}

func itemsOnboardingStatusEntryAttrs(entry *models.AwsOnboardingStatusEntry) map[string]attr.Value {
	startedAt := types.StringNull()
	if t := nullableToPointer(entry.OnboardingStartedAt); t != nil {
		startedAt = types.StringValue(t.UTC().Format(time.RFC3339))
	}

	return map[string]attr.Value{
		"onboarding_started_at": startedAt,
		"status":                types.StringValue(string(entry.Status)),
	}
}

func mapItemsOnboardingStatusComputeEntry(ctx context.Context, entry *models.AwsOnboardingStatusEntry) (datasource_ps4c_aws_organizations.OnboardingStatusComputeValue, diag.Diagnostics) {
	if entry == nil {
		return datasource_ps4c_aws_organizations.NewOnboardingStatusComputeValueNull(), nil
	}
	return datasource_ps4c_aws_organizations.NewOnboardingStatusComputeValue(
		datasource_ps4c_aws_organizations.OnboardingStatusComputeValue{}.AttributeTypes(ctx),
		itemsOnboardingStatusEntryAttrs(entry),
	)
}

func mapItemsOnboardingStatusDatabaseEntry(ctx context.Context, entry *models.AwsOnboardingStatusEntry) (datasource_ps4c_aws_organizations.OnboardingStatusDatabaseValue, diag.Diagnostics) {
	if entry == nil {
		return datasource_ps4c_aws_organizations.NewOnboardingStatusDatabaseValueNull(), nil
	}
	return datasource_ps4c_aws_organizations.NewOnboardingStatusDatabaseValue(
		datasource_ps4c_aws_organizations.OnboardingStatusDatabaseValue{}.AttributeTypes(ctx),
		itemsOnboardingStatusEntryAttrs(entry),
	)
}

// --- stats30d ---

func mapItemsStats30d(ctx context.Context, s *models.AwsOrganizationStats30d) (datasource_ps4c_aws_organizations.Stats30dValue, diag.Diagnostics) {
	if s == nil {
		return datasource_ps4c_aws_organizations.NewStats30dValueNull(), nil
	}

	var diags diag.Diagnostics

	compute, d := mapItemsStats30dCompute(ctx, s.Compute)
	diags.Append(d...)

	database, d := mapItemsStats30dDatabase(ctx, s.Database)
	diags.Append(d...)

	val, d := datasource_ps4c_aws_organizations.NewStats30dValue(
		datasource_ps4c_aws_organizations.Stats30dValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"compute": compute, "database": database},
	)
	diags.Append(d...)
	return val, diags
}

func mapItemsStats30dCompute(ctx context.Context, s *models.Stats30dSummary) (datasource_ps4c_aws_organizations.Stats30dComputeValue, diag.Diagnostics) {
	if s == nil {
		return datasource_ps4c_aws_organizations.NewStats30dComputeValueNull(), nil
	}

	var diags diag.Diagnostics

	esr := types.Float64Null()
	if e := nullableToPointer(s.Esr); e != nil {
		esr = types.Float64Value(*e)
	}

	savings, d := buildOrgsMoneyValue(s.Savings,
		datasource_ps4c_aws_organizations.SavingsValue{}.AttributeTypes(ctx),
		datasource_ps4c_aws_organizations.NewSavingsValue,
		datasource_ps4c_aws_organizations.NewSavingsValueNull,
	)
	diags.Append(d...)

	val, d := datasource_ps4c_aws_organizations.NewStats30dComputeValue(
		datasource_ps4c_aws_organizations.Stats30dComputeValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"esr": esr, "savings": savings},
	)
	diags.Append(d...)
	return val, diags
}

func mapItemsStats30dDatabase(ctx context.Context, s *models.Stats30dSummary) (datasource_ps4c_aws_organizations.Stats30dDatabaseValue, diag.Diagnostics) {
	if s == nil {
		return datasource_ps4c_aws_organizations.NewStats30dDatabaseValueNull(), nil
	}

	var diags diag.Diagnostics

	esr := types.Float64Null()
	if e := nullableToPointer(s.Esr); e != nil {
		esr = types.Float64Value(*e)
	}

	savings, d := buildOrgsMoneyValue(s.Savings,
		datasource_ps4c_aws_organizations.SavingsValue{}.AttributeTypes(ctx),
		datasource_ps4c_aws_organizations.NewSavingsValue,
		datasource_ps4c_aws_organizations.NewSavingsValueNull,
	)
	diags.Append(d...)

	val, d := datasource_ps4c_aws_organizations.NewStats30dDatabaseValue(
		datasource_ps4c_aws_organizations.Stats30dDatabaseValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"esr": esr, "savings": savings},
	)
	diags.Append(d...)
	return val, diags
}

// --- savings_totals ---

func mapItemsSavingsTotals(ctx context.Context, totals *models.AwsOrganizationSavingsTotals) (datasource_ps4c_aws_organizations.SavingsTotalsValue, diag.Diagnostics) {
	if totals == nil {
		return datasource_ps4c_aws_organizations.NewSavingsTotalsValueNull(), nil
	}

	var diags diag.Diagnostics

	compute, d := mapItemsSavingsTotalsCompute(ctx, totals.Compute)
	diags.Append(d...)

	database, d := mapItemsSavingsTotalsDatabase(ctx, totals.Database)
	diags.Append(d...)

	val, d := datasource_ps4c_aws_organizations.NewSavingsTotalsValue(
		datasource_ps4c_aws_organizations.SavingsTotalsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"compute": compute, "database": database},
	)
	diags.Append(d...)
	return val, diags
}

func mapItemsSavingsTotalsCompute(ctx context.Context, t *models.AwsSavingsTotals) (datasource_ps4c_aws_organizations.SavingsTotalsComputeValue, diag.Diagnostics) {
	if t == nil {
		return datasource_ps4c_aws_organizations.NewSavingsTotalsComputeValueNull(), nil
	}

	var diags diag.Diagnostics

	lifetime, d := datasource_ps4c_aws_organizations.NewLifetimeValue(
		datasource_ps4c_aws_organizations.LifetimeValue{}.AttributeTypes(ctx),
		moneyAttrsItems(t.Lifetime.Amount, t.Lifetime.Currency),
	)
	diags.Append(d...)

	ytd, d := datasource_ps4c_aws_organizations.NewYtdValue(
		datasource_ps4c_aws_organizations.YtdValue{}.AttributeTypes(ctx),
		moneyAttrsItems(t.Ytd.Amount, t.Ytd.Currency),
	)
	diags.Append(d...)

	val, d := datasource_ps4c_aws_organizations.NewSavingsTotalsComputeValue(
		datasource_ps4c_aws_organizations.SavingsTotalsComputeValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"lifetime": lifetime, "ytd": ytd},
	)
	diags.Append(d...)
	return val, diags
}

func mapItemsSavingsTotalsDatabase(ctx context.Context, t *models.AwsSavingsTotals) (datasource_ps4c_aws_organizations.SavingsTotalsDatabaseValue, diag.Diagnostics) {
	if t == nil {
		return datasource_ps4c_aws_organizations.NewSavingsTotalsDatabaseValueNull(), nil
	}

	var diags diag.Diagnostics

	lifetime, d := datasource_ps4c_aws_organizations.NewLifetimeValue(
		datasource_ps4c_aws_organizations.LifetimeValue{}.AttributeTypes(ctx),
		moneyAttrsItems(t.Lifetime.Amount, t.Lifetime.Currency),
	)
	diags.Append(d...)

	ytd, d := datasource_ps4c_aws_organizations.NewYtdValue(
		datasource_ps4c_aws_organizations.YtdValue{}.AttributeTypes(ctx),
		moneyAttrsItems(t.Ytd.Amount, t.Ytd.Currency),
	)
	diags.Append(d...)

	val, d := datasource_ps4c_aws_organizations.NewSavingsTotalsDatabaseValue(
		datasource_ps4c_aws_organizations.SavingsTotalsDatabaseValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"lifetime": lifetime, "ytd": ytd},
	)
	diags.Append(d...)
	return val, diags
}

// --- monthly_potential_savings ---

func mapItemsMonthlyPotentialSavings(ctx context.Context, mps *models.AwsMonthlyPotentialSavings) (datasource_ps4c_aws_organizations.MonthlyPotentialSavingsValue, diag.Diagnostics) {
	if mps == nil {
		return datasource_ps4c_aws_organizations.NewMonthlyPotentialSavingsValueNull(), nil
	}

	var diags diag.Diagnostics

	compute, d := buildOrgsMoneyValue(mps.Compute,
		datasource_ps4c_aws_organizations.ComputeValue{}.AttributeTypes(ctx),
		datasource_ps4c_aws_organizations.NewComputeValue,
		datasource_ps4c_aws_organizations.NewComputeValueNull,
	)
	diags.Append(d...)

	database, d := buildOrgsMoneyValue(mps.Database,
		datasource_ps4c_aws_organizations.DatabaseValue{}.AttributeTypes(ctx),
		datasource_ps4c_aws_organizations.NewDatabaseValue,
		datasource_ps4c_aws_organizations.NewDatabaseValueNull,
	)
	diags.Append(d...)

	val, d := datasource_ps4c_aws_organizations.NewMonthlyPotentialSavingsValue(
		datasource_ps4c_aws_organizations.MonthlyPotentialSavingsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"compute": compute, "database": database},
	)
	diags.Append(d...)
	return val, diags
}
