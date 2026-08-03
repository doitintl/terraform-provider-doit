package provider

import (
	"context"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_ps4c_aws_organization"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// mapAwsOrganizationDetailToModel maps a GetAwsOrganization API response onto
// the ps4c_aws_organization data source model.
func mapAwsOrganizationDetailToModel(ctx context.Context, apiResp *models.AwsOrganizationDetail, data *ps4cAwsOrganizationDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	data.ManagementAccountId = types.StringValue(apiResp.ManagementAccountId)
	data.DisplayName = types.StringPointerValue(nullableToPointer(apiResp.DisplayName))

	if syncTime := nullableToPointer(apiResp.SavingsPlansSyncTime); syncTime != nil {
		data.SavingsPlansSyncTime = types.StringValue(syncTime.UTC().Format(time.RFC3339))
	} else {
		data.SavingsPlansSyncTime = types.StringNull()
	}

	onboardingStatus, d := mapOrgOnboardingStatus(ctx, apiResp.OnboardingStatus)
	diags.Append(d...)
	data.OnboardingStatus = onboardingStatus

	stats30d, d := mapOrgStats30d(ctx, apiResp.Stats30d)
	diags.Append(d...)
	data.Stats30d = stats30d

	savingsTotals, d := mapOrgSavingsTotals(ctx, apiResp.SavingsTotals)
	diags.Append(d...)
	data.SavingsTotals = savingsTotals

	monthlyPotentialSavings, d := mapOrgMonthlyPotentialSavings(ctx, apiResp.MonthlyPotentialSavings)
	diags.Append(d...)
	data.MonthlyPotentialSavings = monthlyPotentialSavings

	monthlyStats, d := mapOrgMonthlyStats(ctx, apiResp.MonthlyStats)
	diags.Append(d...)
	data.MonthlyStats = monthlyStats

	dailyCoverage, d := mapOrgDailyCoverage(ctx, apiResp.DailyCoverage)
	diags.Append(d...)
	data.DailyCoverage = dailyCoverage

	return diags
}

// moneyAttrs builds the attribute map shared by every generated Money-shaped
// nested object ({amount, currency}) in this schema, regardless of which
// distinct Go type wraps it at a given nesting path.
func moneyAttrs(amount, currency string) map[string]attr.Value {
	return map[string]attr.Value{
		"amount":   types.StringValue(amount),
		"currency": types.StringValue(currency),
	}
}

// buildMoneyValue constructs a Money-shaped nested object using the given
// package-generated constructor, or its null variant when m is nil. Each
// nesting path in this schema generates its own distinct Go type for what is
// conceptually the same {amount, currency} shape, so the constructors must be
// supplied per call site.
func buildMoneyValue[V attr.Value](m *models.Money, attrTypes map[string]attr.Type, newFn func(map[string]attr.Type, map[string]attr.Value) (V, diag.Diagnostics), newNullFn func() V) (V, diag.Diagnostics) {
	if m == nil {
		return newNullFn(), nil
	}
	return newFn(attrTypes, moneyAttrs(m.Amount, m.Currency))
}

// buildObjectList maps a slice of API items into a Terraform list of a
// generated nested-object value type, defaulting to an empty (never null)
// list when there are no items, per the Computed-Only List Attributes rule.
func buildObjectList[V attr.Value](elemType attr.Type, attrTypes map[string]attr.Type, n int, attrsAt func(i int) (map[string]attr.Value, diag.Diagnostics), newFn func(map[string]attr.Type, map[string]attr.Value) (V, diag.Diagnostics)) (types.List, diag.Diagnostics) {
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

func mapOrgOnboardingStatus(ctx context.Context, status *models.AwsOnboardingStatus) (datasource_ps4c_aws_organization.OnboardingStatusValue, diag.Diagnostics) {
	if status == nil {
		return datasource_ps4c_aws_organization.NewOnboardingStatusValueNull(), nil
	}

	var diags diag.Diagnostics

	compute, d := mapOnboardingStatusComputeEntry(ctx, status.Compute)
	diags.Append(d...)

	database, d := mapOnboardingStatusDatabaseEntry(ctx, status.Database)
	diags.Append(d...)

	val, d := datasource_ps4c_aws_organization.NewOnboardingStatusValue(
		datasource_ps4c_aws_organization.OnboardingStatusValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"compute": compute, "database": database},
	)
	diags.Append(d...)
	return val, diags
}

func onboardingStatusEntryAttrs(entry *models.AwsOnboardingStatusEntry) map[string]attr.Value {
	var startedAt types.String
	if t := nullableToPointer(entry.OnboardingStartedAt); t != nil {
		startedAt = types.StringValue(t.UTC().Format(time.RFC3339))
	} else {
		startedAt = types.StringNull()
	}

	return map[string]attr.Value{
		"onboarding_started_at": startedAt,
		"status":                types.StringValue(string(entry.Status)),
	}
}

func mapOnboardingStatusComputeEntry(ctx context.Context, entry *models.AwsOnboardingStatusEntry) (datasource_ps4c_aws_organization.OnboardingStatusComputeValue, diag.Diagnostics) {
	if entry == nil {
		return datasource_ps4c_aws_organization.NewOnboardingStatusComputeValueNull(), nil
	}
	return datasource_ps4c_aws_organization.NewOnboardingStatusComputeValue(
		datasource_ps4c_aws_organization.OnboardingStatusComputeValue{}.AttributeTypes(ctx),
		onboardingStatusEntryAttrs(entry),
	)
}

func mapOnboardingStatusDatabaseEntry(ctx context.Context, entry *models.AwsOnboardingStatusEntry) (datasource_ps4c_aws_organization.OnboardingStatusDatabaseValue, diag.Diagnostics) {
	if entry == nil {
		return datasource_ps4c_aws_organization.NewOnboardingStatusDatabaseValueNull(), nil
	}
	return datasource_ps4c_aws_organization.NewOnboardingStatusDatabaseValue(
		datasource_ps4c_aws_organization.OnboardingStatusDatabaseValue{}.AttributeTypes(ctx),
		onboardingStatusEntryAttrs(entry),
	)
}

// --- stats30d ---

func mapOrgStats30d(ctx context.Context, s *models.AwsOrganizationStats30d) (datasource_ps4c_aws_organization.Stats30dValue, diag.Diagnostics) {
	if s == nil {
		return datasource_ps4c_aws_organization.NewStats30dValueNull(), nil
	}

	var diags diag.Diagnostics

	compute, d := mapStats30dCompute(ctx, s.Compute)
	diags.Append(d...)

	database, d := mapStats30dDatabase(ctx, s.Database)
	diags.Append(d...)

	val, d := datasource_ps4c_aws_organization.NewStats30dValue(
		datasource_ps4c_aws_organization.Stats30dValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"compute": compute, "database": database},
	)
	diags.Append(d...)
	return val, diags
}

func mapStats30dCompute(ctx context.Context, s *models.Stats30dSummary) (datasource_ps4c_aws_organization.Stats30dComputeValue, diag.Diagnostics) {
	if s == nil {
		return datasource_ps4c_aws_organization.NewStats30dComputeValueNull(), nil
	}

	var diags diag.Diagnostics

	esr := types.Float64Null()
	if e := nullableToPointer(s.Esr); e != nil {
		esr = types.Float64Value(*e)
	}

	savings, d := buildMoneyValue(s.Savings,
		datasource_ps4c_aws_organization.SavingsValue{}.AttributeTypes(ctx),
		datasource_ps4c_aws_organization.NewSavingsValue,
		datasource_ps4c_aws_organization.NewSavingsValueNull,
	)
	diags.Append(d...)

	val, d := datasource_ps4c_aws_organization.NewStats30dComputeValue(
		datasource_ps4c_aws_organization.Stats30dComputeValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"esr": esr, "savings": savings},
	)
	diags.Append(d...)
	return val, diags
}

func mapStats30dDatabase(ctx context.Context, s *models.Stats30dSummary) (datasource_ps4c_aws_organization.Stats30dDatabaseValue, diag.Diagnostics) {
	if s == nil {
		return datasource_ps4c_aws_organization.NewStats30dDatabaseValueNull(), nil
	}

	var diags diag.Diagnostics

	esr := types.Float64Null()
	if e := nullableToPointer(s.Esr); e != nil {
		esr = types.Float64Value(*e)
	}

	savings, d := buildMoneyValue(s.Savings,
		datasource_ps4c_aws_organization.SavingsValue{}.AttributeTypes(ctx),
		datasource_ps4c_aws_organization.NewSavingsValue,
		datasource_ps4c_aws_organization.NewSavingsValueNull,
	)
	diags.Append(d...)

	val, d := datasource_ps4c_aws_organization.NewStats30dDatabaseValue(
		datasource_ps4c_aws_organization.Stats30dDatabaseValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"esr": esr, "savings": savings},
	)
	diags.Append(d...)
	return val, diags
}

// --- savings_totals ---

func mapOrgSavingsTotals(ctx context.Context, totals *models.AwsOrganizationSavingsTotals) (datasource_ps4c_aws_organization.SavingsTotalsValue, diag.Diagnostics) {
	if totals == nil {
		return datasource_ps4c_aws_organization.NewSavingsTotalsValueNull(), nil
	}

	var diags diag.Diagnostics

	compute, d := mapSavingsTotalsCompute(ctx, totals.Compute)
	diags.Append(d...)

	database, d := mapSavingsTotalsDatabase(ctx, totals.Database)
	diags.Append(d...)

	val, d := datasource_ps4c_aws_organization.NewSavingsTotalsValue(
		datasource_ps4c_aws_organization.SavingsTotalsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"compute": compute, "database": database},
	)
	diags.Append(d...)
	return val, diags
}

func mapSavingsTotalsCompute(ctx context.Context, t *models.AwsSavingsTotals) (datasource_ps4c_aws_organization.SavingsTotalsComputeValue, diag.Diagnostics) {
	if t == nil {
		return datasource_ps4c_aws_organization.NewSavingsTotalsComputeValueNull(), nil
	}

	var diags diag.Diagnostics

	lifetime, d := datasource_ps4c_aws_organization.NewLifetimeValue(
		datasource_ps4c_aws_organization.LifetimeValue{}.AttributeTypes(ctx),
		moneyAttrs(t.Lifetime.Amount, t.Lifetime.Currency),
	)
	diags.Append(d...)

	ytd, d := datasource_ps4c_aws_organization.NewYtdValue(
		datasource_ps4c_aws_organization.YtdValue{}.AttributeTypes(ctx),
		moneyAttrs(t.Ytd.Amount, t.Ytd.Currency),
	)
	diags.Append(d...)

	val, d := datasource_ps4c_aws_organization.NewSavingsTotalsComputeValue(
		datasource_ps4c_aws_organization.SavingsTotalsComputeValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"lifetime": lifetime, "ytd": ytd},
	)
	diags.Append(d...)
	return val, diags
}

func mapSavingsTotalsDatabase(ctx context.Context, t *models.AwsSavingsTotals) (datasource_ps4c_aws_organization.SavingsTotalsDatabaseValue, diag.Diagnostics) {
	if t == nil {
		return datasource_ps4c_aws_organization.NewSavingsTotalsDatabaseValueNull(), nil
	}

	var diags diag.Diagnostics

	lifetime, d := datasource_ps4c_aws_organization.NewLifetimeValue(
		datasource_ps4c_aws_organization.LifetimeValue{}.AttributeTypes(ctx),
		moneyAttrs(t.Lifetime.Amount, t.Lifetime.Currency),
	)
	diags.Append(d...)

	ytd, d := datasource_ps4c_aws_organization.NewYtdValue(
		datasource_ps4c_aws_organization.YtdValue{}.AttributeTypes(ctx),
		moneyAttrs(t.Ytd.Amount, t.Ytd.Currency),
	)
	diags.Append(d...)

	val, d := datasource_ps4c_aws_organization.NewSavingsTotalsDatabaseValue(
		datasource_ps4c_aws_organization.SavingsTotalsDatabaseValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"lifetime": lifetime, "ytd": ytd},
	)
	diags.Append(d...)
	return val, diags
}

// --- monthly_potential_savings ---

func mapOrgMonthlyPotentialSavings(ctx context.Context, mps *models.AwsMonthlyPotentialSavings) (datasource_ps4c_aws_organization.MonthlyPotentialSavingsValue, diag.Diagnostics) {
	if mps == nil {
		return datasource_ps4c_aws_organization.NewMonthlyPotentialSavingsValueNull(), nil
	}

	var diags diag.Diagnostics

	compute, d := buildMoneyValue(mps.Compute,
		datasource_ps4c_aws_organization.MonthlyPotentialSavingsComputeValue{}.AttributeTypes(ctx),
		datasource_ps4c_aws_organization.NewMonthlyPotentialSavingsComputeValue,
		datasource_ps4c_aws_organization.NewMonthlyPotentialSavingsComputeValueNull,
	)
	diags.Append(d...)

	database, d := buildMoneyValue(mps.Database,
		datasource_ps4c_aws_organization.MonthlyPotentialSavingsDatabaseValue{}.AttributeTypes(ctx),
		datasource_ps4c_aws_organization.NewMonthlyPotentialSavingsDatabaseValue,
		datasource_ps4c_aws_organization.NewMonthlyPotentialSavingsDatabaseValueNull,
	)
	diags.Append(d...)

	val, d := datasource_ps4c_aws_organization.NewMonthlyPotentialSavingsValue(
		datasource_ps4c_aws_organization.MonthlyPotentialSavingsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"compute": compute, "database": database},
	)
	diags.Append(d...)
	return val, diags
}

// --- monthly_stats ---

func monthlyStatsEntryAttrs(ctx context.Context, e models.AwsMonthlyStatsEntry) (map[string]attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics

	costWithSavings, d := datasource_ps4c_aws_organization.NewCostWithSavingsValue(
		datasource_ps4c_aws_organization.CostWithSavingsValue{}.AttributeTypes(ctx),
		moneyAttrs(e.CostWithSavings.Amount, e.CostWithSavings.Currency),
	)
	diags.Append(d...)

	onDemandCost, d := datasource_ps4c_aws_organization.NewOnDemandCostValue(
		datasource_ps4c_aws_organization.OnDemandCostValue{}.AttributeTypes(ctx),
		moneyAttrs(e.OnDemandCost.Amount, e.OnDemandCost.Currency),
	)
	diags.Append(d...)

	return map[string]attr.Value{
		"cost_with_savings": costWithSavings,
		"esr":               types.Float64Value(e.Esr),
		"month":             types.StringValue(e.Month),
		"on_demand_cost":    onDemandCost,
	}, diags
}

func mapOrgMonthlyStats(ctx context.Context, ms *models.AwsOrganizationDetailAllOf1MonthlyStats) (datasource_ps4c_aws_organization.MonthlyStatsValue, diag.Diagnostics) {
	if ms == nil {
		return datasource_ps4c_aws_organization.NewMonthlyStatsValueNull(), nil
	}

	var diags diag.Diagnostics

	var computeEntries, databaseEntries []models.AwsMonthlyStatsEntry
	if ms.Compute != nil {
		computeEntries = *ms.Compute
	}
	if ms.Database != nil {
		databaseEntries = *ms.Database
	}

	computeList, d := buildObjectList(
		datasource_ps4c_aws_organization.MonthlyStatsComputeValue{}.Type(ctx),
		datasource_ps4c_aws_organization.MonthlyStatsComputeValue{}.AttributeTypes(ctx),
		len(computeEntries),
		func(i int) (map[string]attr.Value, diag.Diagnostics) {
			return monthlyStatsEntryAttrs(ctx, computeEntries[i])
		},
		datasource_ps4c_aws_organization.NewMonthlyStatsComputeValue,
	)
	diags.Append(d...)

	databaseList, d := buildObjectList(
		datasource_ps4c_aws_organization.MonthlyStatsDatabaseValue{}.Type(ctx),
		datasource_ps4c_aws_organization.MonthlyStatsDatabaseValue{}.AttributeTypes(ctx),
		len(databaseEntries),
		func(i int) (map[string]attr.Value, diag.Diagnostics) {
			return monthlyStatsEntryAttrs(ctx, databaseEntries[i])
		},
		datasource_ps4c_aws_organization.NewMonthlyStatsDatabaseValue,
	)
	diags.Append(d...)

	val, d := datasource_ps4c_aws_organization.NewMonthlyStatsValue(
		datasource_ps4c_aws_organization.MonthlyStatsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"compute": computeList, "database": databaseList},
	)
	diags.Append(d...)
	return val, diags
}

// --- daily_coverage ---

func dailyCoverageEntryAttrs(ctx context.Context, e models.AwsDailyCoverageEntry) (map[string]attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics

	flexsaveCost, d := buildMoneyValue(e.FlexsaveCost,
		datasource_ps4c_aws_organization.FlexsaveCostValue{}.AttributeTypes(ctx),
		datasource_ps4c_aws_organization.NewFlexsaveCostValue,
		datasource_ps4c_aws_organization.NewFlexsaveCostValueNull,
	)
	diags.Append(d...)

	onDemandCost, d := buildMoneyValue(e.OnDemandCost,
		datasource_ps4c_aws_organization.OnDemandCostValue{}.AttributeTypes(ctx),
		datasource_ps4c_aws_organization.NewOnDemandCostValue,
		datasource_ps4c_aws_organization.NewOnDemandCostValueNull,
	)
	diags.Append(d...)

	reservedInstCost, d := buildMoneyValue(e.ReservedInstCost,
		datasource_ps4c_aws_organization.ReservedInstCostValue{}.AttributeTypes(ctx),
		datasource_ps4c_aws_organization.NewReservedInstCostValue,
		datasource_ps4c_aws_organization.NewReservedInstCostValueNull,
	)
	diags.Append(d...)

	savingsPlanCost, d := buildMoneyValue(e.SavingsPlanCost,
		datasource_ps4c_aws_organization.SavingsPlanCostValue{}.AttributeTypes(ctx),
		datasource_ps4c_aws_organization.NewSavingsPlanCostValue,
		datasource_ps4c_aws_organization.NewSavingsPlanCostValueNull,
	)
	diags.Append(d...)

	spotCost, d := buildMoneyValue(e.SpotCost,
		datasource_ps4c_aws_organization.SpotCostValue{}.AttributeTypes(ctx),
		datasource_ps4c_aws_organization.NewSpotCostValue,
		datasource_ps4c_aws_organization.NewSpotCostValueNull,
	)
	diags.Append(d...)

	return map[string]attr.Value{
		"date":               types.StringValue(e.Date.String()),
		"flexsave_cost":      flexsaveCost,
		"on_demand_cost":     onDemandCost,
		"reserved_inst_cost": reservedInstCost,
		"savings_plan_cost":  savingsPlanCost,
		"spot_cost":          spotCost,
	}, diags
}

func mapOrgDailyCoverage(ctx context.Context, dc *models.AwsOrganizationDetailAllOf1DailyCoverage) (datasource_ps4c_aws_organization.DailyCoverageValue, diag.Diagnostics) {
	if dc == nil {
		return datasource_ps4c_aws_organization.NewDailyCoverageValueNull(), nil
	}

	var diags diag.Diagnostics

	var computeEntries, databaseEntries []models.AwsDailyCoverageEntry
	if dc.Compute != nil {
		computeEntries = *dc.Compute
	}
	if dc.Database != nil {
		databaseEntries = *dc.Database
	}

	computeList, d := buildObjectList(
		datasource_ps4c_aws_organization.ComputeValue{}.Type(ctx),
		datasource_ps4c_aws_organization.ComputeValue{}.AttributeTypes(ctx),
		len(computeEntries),
		func(i int) (map[string]attr.Value, diag.Diagnostics) {
			return dailyCoverageEntryAttrs(ctx, computeEntries[i])
		},
		datasource_ps4c_aws_organization.NewComputeValue,
	)
	diags.Append(d...)

	databaseList, d := buildObjectList(
		datasource_ps4c_aws_organization.DatabaseValue{}.Type(ctx),
		datasource_ps4c_aws_organization.DatabaseValue{}.AttributeTypes(ctx),
		len(databaseEntries),
		func(i int) (map[string]attr.Value, diag.Diagnostics) {
			return dailyCoverageEntryAttrs(ctx, databaseEntries[i])
		},
		datasource_ps4c_aws_organization.NewDatabaseValue,
	)
	diags.Append(d...)

	val, d := datasource_ps4c_aws_organization.NewDailyCoverageValue(
		datasource_ps4c_aws_organization.DailyCoverageValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"compute": computeList, "database": databaseList},
	)
	diags.Append(d...)
	return val, diags
}
