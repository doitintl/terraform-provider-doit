package provider

import (
	"context"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_ps4c_gcp_billing_accounts"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// mapGcpBillingAccountsToItemsList maps a ListGcpBillingAccounts API response's
// items into the ps4c_gcp_billing_accounts data source's list attribute.
func mapGcpBillingAccountsToItemsList(ctx context.Context, accounts []models.GcpBillingAccount) (types.List, diag.Diagnostics) {
	elemType := datasource_ps4c_gcp_billing_accounts.ItemsValue{}.Type(ctx)
	attrTypes := datasource_ps4c_gcp_billing_accounts.ItemsValue{}.AttributeTypes(ctx)

	return buildGcpAccountsObjectList(elemType, attrTypes, len(accounts),
		func(i int) (map[string]attr.Value, diag.Diagnostics) {
			return gcpBillingAccountItemAttrs(ctx, accounts[i])
		},
		datasource_ps4c_gcp_billing_accounts.NewItemsValue,
	)
}

func gcpBillingAccountItemAttrs(ctx context.Context, account models.GcpBillingAccount) (map[string]attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics

	commitmentsSyncTime := types.StringNull()
	if syncTime := nullableToPointer(account.CommitmentsSyncTime); syncTime != nil {
		commitmentsSyncTime = types.StringValue(syncTime.UTC().Format(time.RFC3339))
	}

	onboardingStatus, d := mapGcpAccountsOnboardingStatus(ctx, account.OnboardingStatus)
	diags.Append(d...)

	stats30d, d := mapGcpAccountsStats30d(ctx, account.Stats30d)
	diags.Append(d...)

	savingsTotals, d := mapGcpAccountsSavingsTotals(ctx, account.SavingsTotals)
	diags.Append(d...)

	return map[string]attr.Value{
		"billing_account_id":    types.StringValue(account.BillingAccountId),
		"commitments_sync_time": commitmentsSyncTime,
		"cud_export_healthy":    types.BoolPointerValue(nullableToPointer(account.CudExportHealthy)),
		"currency":              types.StringPointerValue(nullableToPointer(account.Currency)),
		"display_name":          types.StringPointerValue(nullableToPointer(account.DisplayName)),
		"onboarding_status":     onboardingStatus,
		"savings_totals":        savingsTotals,
		"stats30d":              stats30d,
	}, diags
}

func moneyAttrsAccounts(amount, currency string) map[string]attr.Value {
	return map[string]attr.Value{
		"amount":   types.StringValue(amount),
		"currency": types.StringValue(currency),
	}
}

func buildGcpAccountsMoneyValue[V attr.Value](m *models.Money, attrTypes map[string]attr.Type, newFn func(map[string]attr.Type, map[string]attr.Value) (V, diag.Diagnostics), newNullFn func() V) (V, diag.Diagnostics) {
	if m == nil {
		return newNullFn(), nil
	}
	return newFn(attrTypes, moneyAttrsAccounts(m.Amount, m.Currency))
}

func buildGcpAccountsObjectList[V attr.Value](elemType attr.Type, attrTypes map[string]attr.Type, n int, attrsAt func(i int) (map[string]attr.Value, diag.Diagnostics), newFn func(map[string]attr.Type, map[string]attr.Value) (V, diag.Diagnostics)) (types.List, diag.Diagnostics) {
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

func mapGcpAccountsOnboardingStatus(ctx context.Context, status *models.GcpOnboardingStatus) (datasource_ps4c_gcp_billing_accounts.OnboardingStatusValue, diag.Diagnostics) {
	if status == nil {
		return datasource_ps4c_gcp_billing_accounts.NewOnboardingStatusValueNull(), nil
	}

	var diags diag.Diagnostics
	computeVal, d := mapGcpAccountsOnboardingStatusCompute(ctx, status.Compute)
	diags.Append(d...)

	val, d := datasource_ps4c_gcp_billing_accounts.NewOnboardingStatusValue(
		datasource_ps4c_gcp_billing_accounts.OnboardingStatusValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"compute": computeVal},
	)
	diags.Append(d...)
	return val, diags
}

func mapGcpAccountsOnboardingStatusCompute(ctx context.Context, entry *models.GcpOnboardingStatusEntry) (datasource_ps4c_gcp_billing_accounts.ComputeValue, diag.Diagnostics) {
	if entry == nil {
		return datasource_ps4c_gcp_billing_accounts.NewComputeValueNull(), nil
	}

	var startedAt types.String
	if t := nullableToPointer(entry.OnboardingStartedAt); t != nil {
		startedAt = types.StringValue(t.UTC().Format(time.RFC3339))
	} else {
		startedAt = types.StringNull()
	}

	return datasource_ps4c_gcp_billing_accounts.NewComputeValue(
		datasource_ps4c_gcp_billing_accounts.ComputeValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"onboarding_started_at": startedAt,
			"status":                types.StringValue(string(entry.Status)),
		},
	)
}

// --- stats30d ---

func mapGcpAccountsStats30d(ctx context.Context, s *models.GcpBillingAccountStats30d) (datasource_ps4c_gcp_billing_accounts.Stats30dValue, diag.Diagnostics) {
	if s == nil {
		return datasource_ps4c_gcp_billing_accounts.NewStats30dValueNull(), nil
	}

	var diags diag.Diagnostics
	computeVal, d := mapGcpAccountsStats30dCompute(ctx, s.Compute)
	diags.Append(d...)

	val, d := datasource_ps4c_gcp_billing_accounts.NewStats30dValue(
		datasource_ps4c_gcp_billing_accounts.Stats30dValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"compute": computeVal},
	)
	diags.Append(d...)
	return val, diags
}

func mapGcpAccountsStats30dCompute(ctx context.Context, s *models.Stats30dSummary) (datasource_ps4c_gcp_billing_accounts.Stats30dComputeValue, diag.Diagnostics) {
	if s == nil {
		return datasource_ps4c_gcp_billing_accounts.NewStats30dComputeValueNull(), nil
	}

	var diags diag.Diagnostics

	esr := types.Float64Null()
	if e := nullableToPointer(s.Esr); e != nil {
		esr = types.Float64Value(*e)
	}

	savings, d := buildGcpAccountsMoneyValue(s.Savings,
		datasource_ps4c_gcp_billing_accounts.SavingsValue{}.AttributeTypes(ctx),
		datasource_ps4c_gcp_billing_accounts.NewSavingsValue,
		datasource_ps4c_gcp_billing_accounts.NewSavingsValueNull,
	)
	diags.Append(d...)

	val, d := datasource_ps4c_gcp_billing_accounts.NewStats30dComputeValue(
		datasource_ps4c_gcp_billing_accounts.Stats30dComputeValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"esr": esr, "savings": savings},
	)
	diags.Append(d...)
	return val, diags
}

// --- savings_totals ---

func mapGcpAccountsSavingsTotals(ctx context.Context, totals *models.GcpBillingAccountSavingsTotals) (datasource_ps4c_gcp_billing_accounts.SavingsTotalsValue, diag.Diagnostics) {
	if totals == nil {
		return datasource_ps4c_gcp_billing_accounts.NewSavingsTotalsValueNull(), nil
	}

	var diags diag.Diagnostics
	computeVal, d := mapGcpAccountsSavingsTotalsCompute(ctx, totals.Compute)
	diags.Append(d...)

	val, d := datasource_ps4c_gcp_billing_accounts.NewSavingsTotalsValue(
		datasource_ps4c_gcp_billing_accounts.SavingsTotalsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"compute": computeVal},
	)
	diags.Append(d...)
	return val, diags
}

func mapGcpAccountsSavingsTotalsCompute(ctx context.Context, s *models.GcpSavingsTotals) (datasource_ps4c_gcp_billing_accounts.SavingsTotalsComputeValue, diag.Diagnostics) {
	if s == nil {
		return datasource_ps4c_gcp_billing_accounts.NewSavingsTotalsComputeValueNull(), nil
	}

	var diags diag.Diagnostics

	lifetime, d := buildGcpAccountsMoneyValue(&s.Lifetime,
		datasource_ps4c_gcp_billing_accounts.LifetimeValue{}.AttributeTypes(ctx),
		datasource_ps4c_gcp_billing_accounts.NewLifetimeValue,
		datasource_ps4c_gcp_billing_accounts.NewLifetimeValueNull,
	)
	diags.Append(d...)

	ytd, d := buildGcpAccountsMoneyValue(&s.Ytd,
		datasource_ps4c_gcp_billing_accounts.YtdValue{}.AttributeTypes(ctx),
		datasource_ps4c_gcp_billing_accounts.NewYtdValue,
		datasource_ps4c_gcp_billing_accounts.NewYtdValueNull,
	)
	diags.Append(d...)

	val, d := datasource_ps4c_gcp_billing_accounts.NewSavingsTotalsComputeValue(
		datasource_ps4c_gcp_billing_accounts.SavingsTotalsComputeValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"lifetime": lifetime, "ytd": ytd},
	)
	diags.Append(d...)
	return val, diags
}
