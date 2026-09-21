package provider

import (
	"context"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_ps4c_commitment_policies"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// mapCommitmentPoliciesToItemsList converts a slice of CommitmentPolicy
// into a types.List of ItemsValue objects. Empty slices return an empty list, not null.
func mapCommitmentPoliciesToItemsList(ctx context.Context, policies []models.CommitmentPolicy) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := datasource_ps4c_commitment_policies.ItemsValue{}.Type(ctx)
	attrTypes := datasource_ps4c_commitment_policies.ItemsValue{}.AttributeTypes(ctx)

	if len(policies) == 0 {
		return types.ListValueMust(elemType, []attr.Value{}), diags
	}

	vals := make([]attr.Value, 0, len(policies))
	for _, p := range policies {
		attrs := map[string]attr.Value{
			"bootstrap_percent":    types.Float64Value(p.BootstrapPercent),
			"id":                   types.StringValue(p.Id),
			"is_built_in":          types.BoolValue(p.IsBuiltIn),
			"ladder_interval_days": types.Int64Value(int64(p.LadderIntervalDays)),
			"ladder_step_percent":  types.Float64Value(p.LadderStepPercent),
			"lookback_days":        types.Int64Value(int64(p.LookbackDays)),
			"name":                 types.StringValue(p.Name),
			"target_coverage":      types.Float64Value(p.TargetCoverage),
		}
		item, d := datasource_ps4c_commitment_policies.NewItemsValue(attrTypes, attrs)
		diags.Append(d...)
		vals = append(vals, item)
	}

	list, d := types.ListValue(elemType, vals)
	diags.Append(d...)
	return list, diags
}
