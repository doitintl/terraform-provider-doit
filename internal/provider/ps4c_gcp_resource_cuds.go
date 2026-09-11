package provider

import (
	"context"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_ps4c_gcp_resource_cuds"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// mapGcpResourceCudsToItemsList maps a ListGcpResourceCuds API response's
// items into the ps4c_gcp_resource_cuds data source's list attribute.
func mapGcpResourceCudsToItemsList(ctx context.Context, cuds []models.GcpResourceCud) (types.List, diag.Diagnostics) {
	elemType := datasource_ps4c_gcp_resource_cuds.ItemsValue{}.Type(ctx)
	attrTypes := datasource_ps4c_gcp_resource_cuds.ItemsValue{}.AttributeTypes(ctx)

	return buildGcpResourceCudsObjectList(elemType, attrTypes, len(cuds),
		func(i int) (map[string]attr.Value, diag.Diagnostics) {
			return gcpResourceCudItemAttrs(ctx, cuds[i])
		},
		datasource_ps4c_gcp_resource_cuds.NewItemsValue,
	)
}

func gcpResourceCudItemAttrs(ctx context.Context, cud models.GcpResourceCud) (map[string]attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics

	createdTime := types.StringNull()
	if t := nullableToPointer(cud.CreatedTime); t != nil {
		createdTime = types.StringValue(t.UTC().Format(time.RFC3339))
	}

	customTermEligibleUntil := types.StringNull()
	if t := nullableToPointer(cud.CustomTermEligibleUntil); t != nil {
		customTermEligibleUntil = types.StringValue(t.UTC().Format(time.RFC3339))
	}

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

	var reservedResourcesItems []models.GcpResourceCudReservedResourcesItem
	if cud.ReservedResources != nil {
		reservedResourcesItems = *cud.ReservedResources
	}

	rrElemType := datasource_ps4c_gcp_resource_cuds.ReservedResourcesValue{}.Type(ctx)
	rrAttrTypes := datasource_ps4c_gcp_resource_cuds.ReservedResourcesValue{}.AttributeTypes(ctx)

	reservedResourcesList, d := buildGcpResourceCudsObjectList(rrElemType, rrAttrTypes, len(reservedResourcesItems),
		func(i int) (map[string]attr.Value, diag.Diagnostics) {
			return map[string]attr.Value{
				"amount":        types.StringValue(reservedResourcesItems[i].Amount),
				"resource_kind": types.StringValue(reservedResourcesItems[i].ResourceKind),
			}, nil
		},
		datasource_ps4c_gcp_resource_cuds.NewReservedResourcesValue,
	)
	diags.Append(d...)

	return map[string]attr.Value{
		"auto_renew":                    types.BoolPointerValue(cud.AutoRenew),
		"billing_project_number":        types.StringPointerValue(cud.BillingProjectNumber),
		"commitment_numeric_id":         types.StringValue(cud.CommitmentNumericId),
		"created_time":                  createdTime,
		"custom_term_eligible_until":    customTermEligibleUntil,
		"end_time":                      endTime,
		"export_date":                   exportDate,
		"hardware_scope":                types.StringPointerValue(cud.HardwareScope),
		"host_project_id":               types.StringPointerValue(cud.HostProjectId),
		"inventory_state":               types.StringPointerValue(cud.InventoryState),
		"last_month_cpucoverage":        types.Float64PointerValue(cud.LastMonthCPUCoverage),
		"last_month_cpusavings":         types.Float64PointerValue(cud.LastMonthCPUSavings),
		"last_month_cpuutilization":     types.Float64PointerValue(cud.LastMonthCPUUtilization),
		"last_month_memory_coverage":    types.Float64PointerValue(cud.LastMonthMemoryCoverage),
		"last_month_memory_savings":     types.Float64PointerValue(cud.LastMonthMemorySavings),
		"last_month_memory_utilization": types.Float64PointerValue(cud.LastMonthMemoryUtilization),
		"name":                          types.StringPointerValue(cud.Name),
		"organization_id":               types.StringPointerValue(cud.OrganizationId),
		"plan":                          types.StringPointerValue(cud.Plan),
		"region":                        types.StringPointerValue(cud.Region),
		"reserved_resources":            reservedResourcesList,
		"start_time":                    startTime,
		"state":                         types.StringValue(cud.State),
	}, diags
}

func buildGcpResourceCudsObjectList[V attr.Value](elemType attr.Type, attrTypes map[string]attr.Type, n int, attrsAt func(i int) (map[string]attr.Value, diag.Diagnostics), newFn func(map[string]attr.Type, map[string]attr.Value) (V, diag.Diagnostics)) (types.List, diag.Diagnostics) {
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
