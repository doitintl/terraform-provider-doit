package provider

import (
	"context"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_ps4c_gcp_recommendation"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func mapGcpRecommendationToModel(ctx context.Context, apiResp *models.GcpRecommendationWithEligibleSpend, data *ps4cGcpRecommendationDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	data.EstimatedEquivalentRecommendedCommitment = types.Float64PointerValue(apiResp.EstimatedEquivalentRecommendedCommitment)

	if apiResp.Recommendation == nil {
		data.Recommendation = datasource_ps4c_gcp_recommendation.NewRecommendationValueNull()
	} else {
		recVal, recDiags := mapGcpRecommendationValue(ctx, apiResp.Recommendation)
		diags.Append(recDiags...)
		data.Recommendation = recVal
	}

	var usageList []models.EligibleSpendDataPoint
	if apiResp.EligibleUsage != nil {
		usageList = *apiResp.EligibleUsage
	}

	usageVal, usageDiags := mapEligibleUsageToModel(ctx, usageList)
	diags.Append(usageDiags...)
	data.EligibleUsage = usageVal

	if data.Granularity.IsNull() {
		data.Granularity = types.StringValue("day")
	}

	if data.Region.IsNull() {
		if apiResp.Recommendation != nil && apiResp.Recommendation.Region != nil {
			data.Region = types.StringValue(*apiResp.Recommendation.Region)
		} else if data.GcpService.ValueString() == "compute" {
			data.Region = types.StringValue("global")
		}
	}

	return diags
}

func mapGcpRecommendationValue(ctx context.Context, rec *models.GcpRecommendation) (datasource_ps4c_gcp_recommendation.RecommendationValue, diag.Diagnostics) {
	attrTypes := datasource_ps4c_gcp_recommendation.RecommendationValue{}.AttributeTypes(ctx)

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

	return datasource_ps4c_gcp_recommendation.NewRecommendationValue(attrTypes, attrs)
}

func mapEligibleUsageToModel(ctx context.Context, usage []models.EligibleSpendDataPoint) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := datasource_ps4c_gcp_recommendation.EligibleUsageValue{}.Type(ctx)
	attrTypes := datasource_ps4c_gcp_recommendation.EligibleUsageValue{}.AttributeTypes(ctx)

	if len(usage) == 0 {
		return types.ListValueMust(elemType, []attr.Value{}), diags
	}

	vals := make([]attr.Value, 0, len(usage))
	for _, point := range usage {
		usageTime := types.StringNull()
		if t := nullableToPointer(point.UsageTime); t != nil {
			usageTime = types.StringValue(t.UTC().Format(time.RFC3339))
		}

		attrs := map[string]attr.Value{
			"max_usage":    types.Float64PointerValue(point.MaxUsage),
			"median_usage": types.Float64PointerValue(point.MedianUsage),
			"min_usage":    types.Float64PointerValue(point.MinUsage),
			"total_usage":  types.Float64PointerValue(point.TotalUsage),
			"usage_time":   usageTime,
		}

		item, d := datasource_ps4c_gcp_recommendation.NewEligibleUsageValue(attrTypes, attrs)
		diags.Append(d...)
		vals = append(vals, item)
	}

	list, d := types.ListValue(elemType, vals)
	diags.Append(d...)
	return list, diags
}
