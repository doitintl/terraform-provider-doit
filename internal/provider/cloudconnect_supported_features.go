package provider

import (
	"context"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_cloudconnect_supported_features"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func mapCloudconnectSupportedFeaturesToModel(ctx context.Context, resp *models.SupportedFeaturesResponse, data *cloudconnectSupportedFeaturesDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	featElems := make([]datasource_cloudconnect_supported_features.SupportedFeaturesValue, 0)
	if resp.SupportedFeatures != nil {
		for _, f := range *resp.SupportedFeatures {
			featVal, featDiags := datasource_cloudconnect_supported_features.NewSupportedFeaturesValue(
				datasource_cloudconnect_supported_features.SupportedFeaturesValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"name":                     types.StringPointerValue(f.Name),
					"has_required_permissions": types.BoolPointerValue(f.HasRequiredPermissions),
				},
			)
			diags.Append(featDiags...)
			if featDiags.HasError() {
				return diags
			}
			featElems = append(featElems, featVal)
		}
	}

	featList, listDiags := types.ListValueFrom(ctx, datasource_cloudconnect_supported_features.SupportedFeaturesValue{}.Type(ctx), featElems)
	diags.Append(listDiags...)
	if listDiags.HasError() {
		return diags
	}
	data.SupportedFeatures = featList

	return diags
}
