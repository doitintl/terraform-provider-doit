package provider

import (
	"context"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_ps4c_gcp_settings"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// mapGcpBillingAccountsSettingsToItemsList maps a ListGcpBillingAccountsSettings API response's
// items into the ps4c_gcp_settings data source's list attribute.
func mapGcpBillingAccountsSettingsToItemsList(ctx context.Context, items []models.GcpBillingAccountSettingsItem) (types.List, diag.Diagnostics) {
	elemType := datasource_ps4c_gcp_settings.ItemsValue{}.Type(ctx)
	attrTypes := datasource_ps4c_gcp_settings.ItemsValue{}.AttributeTypes(ctx)

	return buildGcpSettingsObjectList(elemType, attrTypes, len(items),
		func(i int) (map[string]attr.Value, diag.Diagnostics) {
			return gcpSettingsItemAttrs(ctx, items[i])
		},
		datasource_ps4c_gcp_settings.NewItemsValue,
	)
}

func gcpSettingsItemAttrs(ctx context.Context, item models.GcpBillingAccountSettingsItem) (map[string]attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics

	servicesList, d := mapGcpSettingsServicesList(ctx, item.Services)
	diags.Append(d...)

	return map[string]attr.Value{
		"billing_account_id": types.StringValue(item.BillingAccountId),
		"services":           servicesList,
	}, diags
}

func mapGcpSettingsServicesList(ctx context.Context, services []models.GcpBillingAccountServiceSettings) (types.List, diag.Diagnostics) {
	elemType := datasource_ps4c_gcp_settings.ServicesValue{}.Type(ctx)
	attrTypes := datasource_ps4c_gcp_settings.ServicesValue{}.AttributeTypes(ctx)

	return buildGcpSettingsObjectList(elemType, attrTypes, len(services),
		func(i int) (map[string]attr.Value, diag.Diagnostics) {
			return gcpSettingsServiceAttrs(ctx, services[i])
		},
		datasource_ps4c_gcp_settings.NewServicesValue,
	)
}

func gcpSettingsServiceAttrs(ctx context.Context, service models.GcpBillingAccountServiceSettings) (map[string]attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics

	settingsVal, d := mapGcpSettingsDetails(ctx, service.Settings)
	diags.Append(d...)

	return map[string]attr.Value{
		"service":  types.StringValue(string(service.Service)),
		"settings": settingsVal,
	}, diags
}

func mapGcpSettingsDetails(ctx context.Context, s models.GcpBillingAccountSettings) (datasource_ps4c_gcp_settings.SettingsValue, diag.Diagnostics) {
	return datasource_ps4c_gcp_settings.NewSettingsValue(
		datasource_ps4c_gcp_settings.SettingsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"last_day_of_month_for_purchase": types.Int64Value(int64(s.LastDayOfMonthForPurchase)),
			"maximum_commitment":             types.Float64Value(s.MaximumCommitment),
			"minimum_commitment":             types.Float64Value(s.MinimumCommitment),
			"policy":                         types.StringValue(string(s.Policy)),
			"purchase_mode":                  types.StringValue(string(s.PurchaseMode)),
			"term":                           types.StringValue(string(s.Term)),
		},
	)
}

func buildGcpSettingsObjectList[V attr.Value](elemType attr.Type, attrTypes map[string]attr.Type, n int, attrsAt func(i int) (map[string]attr.Value, diag.Diagnostics), newFn func(map[string]attr.Type, map[string]attr.Value) (V, diag.Diagnostics)) (types.List, diag.Diagnostics) {
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
