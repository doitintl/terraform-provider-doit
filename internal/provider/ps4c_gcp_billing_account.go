package provider

import (
	"context"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_ps4c_gcp_billing_account"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// mapGcpBillingAccountDetailToModel maps a GetGcpBillingAccount API response onto
// the ps4c_gcp_billing_account data source model.
func mapGcpBillingAccountDetailToModel(ctx context.Context, apiResp *models.GcpBillingAccountDetail, data *ps4cGcpBillingAccountDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	data.BillingAccountId = types.StringValue(apiResp.BillingAccountId)
	data.DisplayName = types.StringPointerValue(nullableToPointer(apiResp.DisplayName))
	data.Currency = types.StringPointerValue(nullableToPointer(apiResp.Currency))
	data.CudExportHealthy = types.BoolPointerValue(nullableToPointer(apiResp.CudExportHealthy))

	if syncTime := nullableToPointer(apiResp.CommitmentsSyncTime); syncTime != nil {
		data.CommitmentsSyncTime = types.StringValue(syncTime.UTC().Format(time.RFC3339))
	} else {
		data.CommitmentsSyncTime = types.StringNull()
	}

	onboardingStatus, d := mapGcpAccountOnboardingStatus(ctx, apiResp.OnboardingStatus)
	diags.Append(d...)
	data.OnboardingStatus = onboardingStatus

	stats30d, d := mapGcpAccountStats30d(ctx, apiResp.Stats30d)
	diags.Append(d...)
	data.Stats30d = stats30d

	savingsTotals, d := mapGcpAccountSavingsTotals(ctx, apiResp.SavingsTotals)
	diags.Append(d...)
	data.SavingsTotals = savingsTotals

	monthlyStats, d := mapGcpAccountMonthlyStats(ctx, apiResp.MonthlyStats)
	diags.Append(d...)
	data.MonthlyStats = monthlyStats

	dailyCoverage, d := mapGcpAccountDailyCoverage(ctx, apiResp.DailyCoverage)
	diags.Append(d...)
	data.DailyCoverage = dailyCoverage

	return diags
}

func moneyAttrsAccount(amount, currency string) map[string]attr.Value {
	return map[string]attr.Value{
		"amount":   types.StringValue(amount),
		"currency": types.StringValue(currency),
	}
}

func buildGcpAccountMoneyValue[V attr.Value](m *models.Money, attrTypes map[string]attr.Type, newFn func(map[string]attr.Type, map[string]attr.Value) (V, diag.Diagnostics), newNullFn func() V) (V, diag.Diagnostics) {
	if m == nil {
		return newNullFn(), nil
	}
	return newFn(attrTypes, moneyAttrsAccount(m.Amount, m.Currency))
}

func buildGcpAccountObjectList[V attr.Value](elemType attr.Type, attrTypes map[string]attr.Type, n int, attrsAt func(i int) (map[string]attr.Value, diag.Diagnostics), newFn func(map[string]attr.Type, map[string]attr.Value) (V, diag.Diagnostics)) (types.List, diag.Diagnostics) {
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

func mapGcpAccountOnboardingStatus(ctx context.Context, status *models.GcpOnboardingStatus) (datasource_ps4c_gcp_billing_account.OnboardingStatusValue, diag.Diagnostics) {
	if status == nil {
		return datasource_ps4c_gcp_billing_account.NewOnboardingStatusValueNull(), nil
	}

	var diags diag.Diagnostics
	computeVal, d := mapGcpAccountOnboardingStatusCompute(ctx, status.Compute)
	diags.Append(d...)

	val, d := datasource_ps4c_gcp_billing_account.NewOnboardingStatusValue(
		datasource_ps4c_gcp_billing_account.OnboardingStatusValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"compute": computeVal},
	)
	diags.Append(d...)
	return val, diags
}

func mapGcpAccountOnboardingStatusCompute(ctx context.Context, entry *models.GcpOnboardingStatusEntry) (datasource_ps4c_gcp_billing_account.OnboardingStatusComputeValue, diag.Diagnostics) {
	if entry == nil {
		return datasource_ps4c_gcp_billing_account.NewOnboardingStatusComputeValueNull(), nil
	}

	var startedAt types.String
	if t := nullableToPointer(entry.OnboardingStartedAt); t != nil {
		startedAt = types.StringValue(t.UTC().Format(time.RFC3339))
	} else {
		startedAt = types.StringNull()
	}

	return datasource_ps4c_gcp_billing_account.NewOnboardingStatusComputeValue(
		datasource_ps4c_gcp_billing_account.OnboardingStatusComputeValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"onboarding_started_at": startedAt,
			"status":                types.StringValue(string(entry.Status)),
		},
	)
}

// --- stats30d ---

func mapGcpAccountStats30d(ctx context.Context, s *models.GcpBillingAccountStats30d) (datasource_ps4c_gcp_billing_account.Stats30dValue, diag.Diagnostics) {
	if s == nil {
		return datasource_ps4c_gcp_billing_account.NewStats30dValueNull(), nil
	}

	var diags diag.Diagnostics
	computeVal, d := mapGcpAccountStats30dCompute(ctx, s.Compute)
	diags.Append(d...)

	val, d := datasource_ps4c_gcp_billing_account.NewStats30dValue(
		datasource_ps4c_gcp_billing_account.Stats30dValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"compute": computeVal},
	)
	diags.Append(d...)
	return val, diags
}

func mapGcpAccountStats30dCompute(ctx context.Context, s *models.Stats30dSummary) (datasource_ps4c_gcp_billing_account.Stats30dComputeValue, diag.Diagnostics) {
	if s == nil {
		return datasource_ps4c_gcp_billing_account.NewStats30dComputeValueNull(), nil
	}

	var diags diag.Diagnostics

	esr := types.Float64Null()
	if e := nullableToPointer(s.Esr); e != nil {
		esr = types.Float64Value(*e)
	}

	savings, d := buildGcpAccountMoneyValue(s.Savings,
		datasource_ps4c_gcp_billing_account.SavingsValue{}.AttributeTypes(ctx),
		datasource_ps4c_gcp_billing_account.NewSavingsValue,
		datasource_ps4c_gcp_billing_account.NewSavingsValueNull,
	)
	diags.Append(d...)

	val, d := datasource_ps4c_gcp_billing_account.NewStats30dComputeValue(
		datasource_ps4c_gcp_billing_account.Stats30dComputeValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"esr": esr, "savings": savings},
	)
	diags.Append(d...)
	return val, diags
}

// --- savings_totals ---

func mapGcpAccountSavingsTotals(ctx context.Context, totals *models.GcpBillingAccountSavingsTotals) (datasource_ps4c_gcp_billing_account.SavingsTotalsValue, diag.Diagnostics) {
	if totals == nil {
		return datasource_ps4c_gcp_billing_account.NewSavingsTotalsValueNull(), nil
	}

	var diags diag.Diagnostics
	computeVal, d := mapGcpAccountSavingsTotalsCompute(ctx, totals.Compute)
	diags.Append(d...)

	val, d := datasource_ps4c_gcp_billing_account.NewSavingsTotalsValue(
		datasource_ps4c_gcp_billing_account.SavingsTotalsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"compute": computeVal},
	)
	diags.Append(d...)
	return val, diags
}

func mapGcpAccountSavingsTotalsCompute(ctx context.Context, s *models.GcpSavingsTotals) (datasource_ps4c_gcp_billing_account.SavingsTotalsComputeValue, diag.Diagnostics) {
	if s == nil {
		return datasource_ps4c_gcp_billing_account.NewSavingsTotalsComputeValueNull(), nil
	}

	var diags diag.Diagnostics

	lifetime, d := buildGcpAccountMoneyValue(&s.Lifetime,
		datasource_ps4c_gcp_billing_account.LifetimeValue{}.AttributeTypes(ctx),
		datasource_ps4c_gcp_billing_account.NewLifetimeValue,
		datasource_ps4c_gcp_billing_account.NewLifetimeValueNull,
	)
	diags.Append(d...)

	ytd, d := buildGcpAccountMoneyValue(&s.Ytd,
		datasource_ps4c_gcp_billing_account.YtdValue{}.AttributeTypes(ctx),
		datasource_ps4c_gcp_billing_account.NewYtdValue,
		datasource_ps4c_gcp_billing_account.NewYtdValueNull,
	)
	diags.Append(d...)

	val, d := datasource_ps4c_gcp_billing_account.NewSavingsTotalsComputeValue(
		datasource_ps4c_gcp_billing_account.SavingsTotalsComputeValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"lifetime": lifetime, "ytd": ytd},
	)
	diags.Append(d...)
	return val, diags
}

// --- monthly_stats ---

func mapGcpAccountMonthlyStats(ctx context.Context, stats *models.GcpBillingAccountDetailAllOf1MonthlyStats) (datasource_ps4c_gcp_billing_account.MonthlyStatsValue, diag.Diagnostics) {
	if stats == nil {
		return datasource_ps4c_gcp_billing_account.NewMonthlyStatsValueNull(), nil
	}

	var diags diag.Diagnostics

	var computeEntries []models.GcpMonthlyStatsEntry
	if stats.Compute != nil {
		computeEntries = *stats.Compute
	}

	elemType := datasource_ps4c_gcp_billing_account.MonthlyStatsComputeValue{}.Type(ctx)
	attrTypes := datasource_ps4c_gcp_billing_account.MonthlyStatsComputeValue{}.AttributeTypes(ctx)

	computeList, d := buildGcpAccountObjectList(elemType, attrTypes, len(computeEntries),
		func(i int) (map[string]attr.Value, diag.Diagnostics) {
			return gcpAccountMonthlyStatsEntryAttrs(ctx, computeEntries[i])
		},
		datasource_ps4c_gcp_billing_account.NewMonthlyStatsComputeValue,
	)
	diags.Append(d...)

	val, d := datasource_ps4c_gcp_billing_account.NewMonthlyStatsValue(
		datasource_ps4c_gcp_billing_account.MonthlyStatsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"compute": computeList},
	)
	diags.Append(d...)
	return val, diags
}

func gcpAccountMonthlyStatsEntryAttrs(ctx context.Context, e models.GcpMonthlyStatsEntry) (map[string]attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics

	costWithSavings, d := buildGcpAccountMoneyValue(&e.CostWithSavings,
		datasource_ps4c_gcp_billing_account.CostWithSavingsValue{}.AttributeTypes(ctx),
		datasource_ps4c_gcp_billing_account.NewCostWithSavingsValue,
		datasource_ps4c_gcp_billing_account.NewCostWithSavingsValueNull,
	)
	diags.Append(d...)

	onDemandCost, d := buildGcpAccountMoneyValue(&e.OnDemandCost,
		datasource_ps4c_gcp_billing_account.OnDemandCostValue{}.AttributeTypes(ctx),
		datasource_ps4c_gcp_billing_account.NewOnDemandCostValue,
		datasource_ps4c_gcp_billing_account.NewOnDemandCostValueNull,
	)
	diags.Append(d...)

	return map[string]attr.Value{
		"cost_with_savings": costWithSavings,
		"esr":               types.Float64Value(e.Esr),
		"month":             types.StringValue(e.Month),
		"on_demand_cost":    onDemandCost,
	}, diags
}

// --- daily_coverage ---

func mapGcpAccountDailyCoverage(ctx context.Context, cov *models.GcpBillingAccountDetailAllOf1DailyCoverage) (datasource_ps4c_gcp_billing_account.DailyCoverageValue, diag.Diagnostics) {
	if cov == nil {
		return datasource_ps4c_gcp_billing_account.NewDailyCoverageValueNull(), nil
	}

	var diags diag.Diagnostics

	var computeEntries []models.GcpDailyCoverageEntry
	if cov.Compute != nil {
		computeEntries = *cov.Compute
	}

	elemType := datasource_ps4c_gcp_billing_account.ComputeValue{}.Type(ctx)
	attrTypes := datasource_ps4c_gcp_billing_account.ComputeValue{}.AttributeTypes(ctx)

	computeList, d := buildGcpAccountObjectList(elemType, attrTypes, len(computeEntries),
		func(i int) (map[string]attr.Value, diag.Diagnostics) {
			return gcpAccountDailyCoverageEntryAttrs(ctx, computeEntries[i])
		},
		datasource_ps4c_gcp_billing_account.NewComputeValue,
	)
	diags.Append(d...)

	val, d := datasource_ps4c_gcp_billing_account.NewDailyCoverageValue(
		datasource_ps4c_gcp_billing_account.DailyCoverageValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"compute": computeList},
	)
	diags.Append(d...)
	return val, diags
}

func gcpAccountDailyCoverageEntryAttrs(ctx context.Context, e models.GcpDailyCoverageEntry) (map[string]attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics

	alloyDb, d := buildGcpAccountMoneyValue(e.AlloyDb,
		datasource_ps4c_gcp_billing_account.AlloyDbValue{}.AttributeTypes(ctx),
		datasource_ps4c_gcp_billing_account.NewAlloyDbValue,
		datasource_ps4c_gcp_billing_account.NewAlloyDbValueNull,
	)
	diags.Append(d...)

	backupForOracle, d := buildGcpAccountMoneyValue(e.BackupForOracle,
		datasource_ps4c_gcp_billing_account.BackupForOracleValue{}.AttributeTypes(ctx),
		datasource_ps4c_gcp_billing_account.NewBackupForOracleValue,
		datasource_ps4c_gcp_billing_account.NewBackupForOracleValueNull,
	)
	diags.Append(d...)

	bigQuery, d := buildGcpAccountMoneyValue(e.BigQuery,
		datasource_ps4c_gcp_billing_account.BigQueryValue{}.AttributeTypes(ctx),
		datasource_ps4c_gcp_billing_account.NewBigQueryValue,
		datasource_ps4c_gcp_billing_account.NewBigQueryValueNull,
	)
	diags.Append(d...)

	bigtable, d := buildGcpAccountMoneyValue(e.Bigtable,
		datasource_ps4c_gcp_billing_account.BigtableValue{}.AttributeTypes(ctx),
		datasource_ps4c_gcp_billing_account.NewBigtableValue,
		datasource_ps4c_gcp_billing_account.NewBigtableValueNull,
	)
	diags.Append(d...)

	cloudFirestore, d := buildGcpAccountMoneyValue(e.CloudFirestore,
		datasource_ps4c_gcp_billing_account.CloudFirestoreValue{}.AttributeTypes(ctx),
		datasource_ps4c_gcp_billing_account.NewCloudFirestoreValue,
		datasource_ps4c_gcp_billing_account.NewCloudFirestoreValueNull,
	)
	diags.Append(d...)

	cloudRun, d := buildGcpAccountMoneyValue(e.CloudRun,
		datasource_ps4c_gcp_billing_account.CloudRunValue{}.AttributeTypes(ctx),
		datasource_ps4c_gcp_billing_account.NewCloudRunValue,
		datasource_ps4c_gcp_billing_account.NewCloudRunValueNull,
	)
	diags.Append(d...)

	cloudSpanner, d := buildGcpAccountMoneyValue(e.CloudSpanner,
		datasource_ps4c_gcp_billing_account.CloudSpannerValue{}.AttributeTypes(ctx),
		datasource_ps4c_gcp_billing_account.NewCloudSpannerValue,
		datasource_ps4c_gcp_billing_account.NewCloudSpannerValueNull,
	)
	diags.Append(d...)

	cloudSql, d := buildGcpAccountMoneyValue(e.CloudSql,
		datasource_ps4c_gcp_billing_account.CloudSqlValue{}.AttributeTypes(ctx),
		datasource_ps4c_gcp_billing_account.NewCloudSqlValue,
		datasource_ps4c_gcp_billing_account.NewCloudSqlValueNull,
	)
	diags.Append(d...)

	computeFlexible, d := buildGcpAccountMoneyValue(e.ComputeFlexible,
		datasource_ps4c_gcp_billing_account.ComputeFlexibleValue{}.AttributeTypes(ctx),
		datasource_ps4c_gcp_billing_account.NewComputeFlexibleValue,
		datasource_ps4c_gcp_billing_account.NewComputeFlexibleValueNull,
	)
	diags.Append(d...)

	kafka, d := buildGcpAccountMoneyValue(e.Kafka,
		datasource_ps4c_gcp_billing_account.KafkaValue{}.AttributeTypes(ctx),
		datasource_ps4c_gcp_billing_account.NewKafkaValue,
		datasource_ps4c_gcp_billing_account.NewKafkaValueNull,
	)
	diags.Append(d...)

	memorystoreForRedis, d := buildGcpAccountMoneyValue(e.MemorystoreForRedis,
		datasource_ps4c_gcp_billing_account.MemorystoreForRedisValue{}.AttributeTypes(ctx),
		datasource_ps4c_gcp_billing_account.NewMemorystoreForRedisValue,
		datasource_ps4c_gcp_billing_account.NewMemorystoreForRedisValueNull,
	)
	diags.Append(d...)

	onDemand, d := buildGcpAccountMoneyValue(e.OnDemand,
		datasource_ps4c_gcp_billing_account.OnDemandValue{}.AttributeTypes(ctx),
		datasource_ps4c_gcp_billing_account.NewOnDemandValue,
		datasource_ps4c_gcp_billing_account.NewOnDemandValueNull,
	)
	diags.Append(d...)

	resourceBased, d := buildGcpAccountMoneyValue(e.ResourceBased,
		datasource_ps4c_gcp_billing_account.ResourceBasedValue{}.AttributeTypes(ctx),
		datasource_ps4c_gcp_billing_account.NewResourceBasedValue,
		datasource_ps4c_gcp_billing_account.NewResourceBasedValueNull,
	)
	diags.Append(d...)

	return map[string]attr.Value{
		"alloy_db":              alloyDb,
		"backup_for_oracle":     backupForOracle,
		"big_query":             bigQuery,
		"bigtable":              bigtable,
		"cloud_firestore":       cloudFirestore,
		"cloud_run":             cloudRun,
		"cloud_spanner":         cloudSpanner,
		"cloud_sql":             cloudSql,
		"compute_flexible":      computeFlexible,
		"date":                  types.StringValue(e.Date.String()),
		"kafka":                 kafka,
		"memorystore_for_redis": memorystoreForRedis,
		"on_demand":             onDemand,
		"resource_based":        resourceBased,
	}, diags
}
