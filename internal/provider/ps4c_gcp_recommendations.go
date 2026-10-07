package provider

import (
	"context"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_ps4c_gcp_recommendations"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// mapGcpRecommendationsToItemsList converts a slice of GcpRecommendation
// into a types.List of ItemsValue objects. Empty slices return an empty list, not null.
func mapGcpRecommendationsToItemsList(ctx context.Context, items []models.GcpRecommendation) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := datasource_ps4c_gcp_recommendations.ItemsValue{}.Type(ctx)
	attrTypes := datasource_ps4c_gcp_recommendations.ItemsValue{}.AttributeTypes(ctx)

	if len(items) == 0 {
		return types.ListValueMust(elemType, []attr.Value{}), diags
	}

	vals := make([]attr.Value, 0, len(items))
	for _, rec := range items {
		policy := types.StringPointerValue(rec.Policy)
		service := types.StringNull()
		if rec.Service != nil {
			service = types.StringValue(string(*rec.Service))
		}

		attrs := map[string]attr.Value{
			"current_commitment":           types.Float64PointerValue(rec.CurrentCommitment),
			"estimated_average_coverage":   types.Float64PointerValue(rec.EstimatedAverageCoverage),
			"policy":                       policy,
			"potential_additional_savings": types.Float64PointerValue(rec.PotentialAdditionalSavings),
			"recommended_commitment":       types.Float64PointerValue(rec.RecommendedCommitment),
			"region":                       types.StringPointerValue(rec.Region),
			"service":                      service,
		}
		item, d := datasource_ps4c_gcp_recommendations.NewItemsValue(attrTypes, attrs)
		diags.Append(d...)
		vals = append(vals, item)
	}

	list, d := types.ListValue(elemType, vals)
	diags.Append(d...)
	return list, diags
}
