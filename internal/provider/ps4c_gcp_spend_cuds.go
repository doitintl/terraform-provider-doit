package provider

import (
	"context"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_ps4c_gcp_spend_cuds"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// mapGcpSpendCudsToItemsList maps a ListGcpSpendCuds API response's
// items into the ps4c_gcp_spend_cuds data source's list attribute.
func mapGcpSpendCudsToItemsList(ctx context.Context, cuds []models.GcpSpendCud) (types.List, diag.Diagnostics) {
	elemType := datasource_ps4c_gcp_spend_cuds.ItemsValue{}.Type(ctx)
	attrTypes := datasource_ps4c_gcp_spend_cuds.ItemsValue{}.AttributeTypes(ctx)

	return buildGcpSpendCudsObjectList(elemType, attrTypes, len(cuds),
		func(i int) (map[string]attr.Value, diag.Diagnostics) {
			return gcpSpendCudItemAttrs(ctx, cuds[i])
		},
		datasource_ps4c_gcp_spend_cuds.NewItemsValue,
	)
}

func gcpSpendCudItemAttrs(_ context.Context, cud models.GcpSpendCud) (map[string]attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics

	endTime := types.StringNull()
	if t := nullableToPointer(cud.EndTime); t != nil {
		endTime = types.StringValue(t.UTC().Format(time.RFC3339))
	}

	exportDate := types.StringNull()
	if t := cud.ExportDate; t != nil {
		exportDate = types.StringValue(t.UTC().Format(time.RFC3339))
	}

	startTime := types.StringNull()
	if t := nullableToPointer(cud.StartTime); t != nil {
		startTime = types.StringValue(t.UTC().Format(time.RFC3339))
	}

	return map[string]attr.Value{
		"commitment_amount":      types.Float64PointerValue(cud.CommitmentAmount),
		"commitment_unit":        types.StringPointerValue(cud.CommitmentUnit),
		"consumption_model_id":   types.StringPointerValue(cud.ConsumptionModelId),
		"cud_id":                 types.StringValue(cud.CudId),
		"cud_product_id":         types.StringPointerValue(cud.CudProductId),
		"cud_product_name":       types.StringValue(cud.CudProductName),
		"cud_product_type":       types.StringPointerValue(cud.CudProductType),
		"end_time":               endTime,
		"entitlement_scope":      types.StringPointerValue(cud.EntitlementScope),
		"export_date":            exportDate,
		"last_month_savings":     types.Float64PointerValue(cud.LastMonthSavings),
		"last_month_utilization": types.Float64PointerValue(cud.LastMonthUtilization),
		"name":                   types.StringPointerValue(cud.Name),
		"region":                 types.StringPointerValue(cud.Region),
		"start_time":             startTime,
		"state":                  types.StringValue(cud.State),
		"sub_type":               types.StringPointerValue(cud.SubType),
		"term":                   types.StringPointerValue(cud.Term),
	}, diags
}

func buildGcpSpendCudsObjectList[V attr.Value](elemType attr.Type, attrTypes map[string]attr.Type, n int, attrsAt func(i int) (map[string]attr.Value, diag.Diagnostics), newFn func(map[string]attr.Type, map[string]attr.Value) (V, diag.Diagnostics)) (types.List, diag.Diagnostics) {
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
