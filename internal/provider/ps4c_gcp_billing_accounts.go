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

func mapGcpAccountsOnboardingStatus(ctx context.Context, status *[]models.GcpOnboardingStatusByService) (types.List, diag.Diagnostics) {
	var entries []models.GcpOnboardingStatusByService
	if status != nil {
		entries = *status
	}

	elemType := datasource_ps4c_gcp_billing_accounts.OnboardingStatusValue{}.Type(ctx)
	attrTypes := datasource_ps4c_gcp_billing_accounts.OnboardingStatusValue{}.AttributeTypes(ctx)

	return buildGcpAccountsObjectList(elemType, attrTypes, len(entries),
		func(i int) (map[string]attr.Value, diag.Diagnostics) {
			startedAt := types.StringNull()
			if t := nullableToPointer(entries[i].OnboardingStartedAt); t != nil {
				startedAt = types.StringValue(t.UTC().Format(time.RFC3339))
			}

			return map[string]attr.Value{
				"onboarding_started_at": startedAt,
				"service":               types.StringValue(string(entries[i].Service)),
				"status":                types.StringValue(string(entries[i].Status)),
			}, nil
		},
		datasource_ps4c_gcp_billing_accounts.NewOnboardingStatusValue,
	)
}

// --- stats30d ---

func mapGcpAccountsStats30d(ctx context.Context, stats *[]models.GcpStats30dByService) (types.List, diag.Diagnostics) {
	var entries []models.GcpStats30dByService
	if stats != nil {
		entries = *stats
	}

	elemType := datasource_ps4c_gcp_billing_accounts.Stats30dValue{}.Type(ctx)
	attrTypes := datasource_ps4c_gcp_billing_accounts.Stats30dValue{}.AttributeTypes(ctx)

	return buildGcpAccountsObjectList(elemType, attrTypes, len(entries),
		func(i int) (map[string]attr.Value, diag.Diagnostics) {
			var diags diag.Diagnostics
			entry := entries[i]

			savings, d := buildGcpAccountsMoneyValue(entry.Savings,
				datasource_ps4c_gcp_billing_accounts.SavingsValue{}.AttributeTypes(ctx),
				datasource_ps4c_gcp_billing_accounts.NewSavingsValue,
				datasource_ps4c_gcp_billing_accounts.NewSavingsValueNull,
			)
			diags.Append(d...)

			return map[string]attr.Value{
				"esr":     types.Float64PointerValue(nullableToPointer(entry.Esr)),
				"savings": savings,
				"service": types.StringValue(string(entry.Service)),
			}, diags
		},
		datasource_ps4c_gcp_billing_accounts.NewStats30dValue,
	)
}

// --- savings_totals ---

func mapGcpAccountsSavingsTotals(ctx context.Context, totals *[]models.GcpSavingsTotalsByService) (types.List, diag.Diagnostics) {
	var entries []models.GcpSavingsTotalsByService
	if totals != nil {
		entries = *totals
	}

	elemType := datasource_ps4c_gcp_billing_accounts.SavingsTotalsValue{}.Type(ctx)
	attrTypes := datasource_ps4c_gcp_billing_accounts.SavingsTotalsValue{}.AttributeTypes(ctx)

	return buildGcpAccountsObjectList(elemType, attrTypes, len(entries),
		func(i int) (map[string]attr.Value, diag.Diagnostics) {
			var diags diag.Diagnostics
			entry := entries[i]

			lifetime, d := buildGcpAccountsMoneyValue(&entry.Lifetime,
				datasource_ps4c_gcp_billing_accounts.LifetimeValue{}.AttributeTypes(ctx),
				datasource_ps4c_gcp_billing_accounts.NewLifetimeValue,
				datasource_ps4c_gcp_billing_accounts.NewLifetimeValueNull,
			)
			diags.Append(d...)

			ytd, d := buildGcpAccountsMoneyValue(&entry.Ytd,
				datasource_ps4c_gcp_billing_accounts.YtdValue{}.AttributeTypes(ctx),
				datasource_ps4c_gcp_billing_accounts.NewYtdValue,
				datasource_ps4c_gcp_billing_accounts.NewYtdValueNull,
			)
			diags.Append(d...)

			return map[string]attr.Value{
				"lifetime": lifetime,
				"service":  types.StringValue(string(entry.Service)),
				"ytd":      ytd,
			}, diags
		},
		datasource_ps4c_gcp_billing_accounts.NewSavingsTotalsValue,
	)
}
