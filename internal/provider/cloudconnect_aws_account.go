// Package provider implements the DoiT Terraform provider.
package provider

import (
	"context"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_cloudconnect_aws_account"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// mapCloudConnectAwsAccountToModel maps ALL fields from the API response to the
// Terraform model. Used by Read and as Phase 1 of the overlay pattern.
func mapCloudConnectAwsAccountToModel(ctx context.Context, resp *models.AwsAccountResponse, state *cloudconnectAwsAccountResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	state.AccountId = types.StringPointerValue(resp.AccountID)
	state.RoleArn = types.StringPointerValue(resp.RoleArn)
	state.S3bucket = types.StringPointerValue(resp.S3Bucket)
	state.S3bucketRegion = types.StringPointerValue(resp.S3BucketRegion)
	state.TimeLinked = types.StringPointerValue(resp.TimeLinked)

	// Map enabled_features from API response.
	if resp.EnabledFeatures != nil {
		elems := make([]attr.Value, 0, len(*resp.EnabledFeatures))
		for _, f := range *resp.EnabledFeatures {
			elems = append(elems, types.StringValue(f))
		}
		state.EnabledFeatures, diags = types.ListValue(types.StringType, elems)
		if diags.HasError() {
			return diags
		}
	} else {
		state.EnabledFeatures = types.ListValueMust(types.StringType, []attr.Value{})
	}

	// Build supported_features list from API response.
	featElems := make([]resource_cloudconnect_aws_account.SupportedFeaturesValue, 0)
	if resp.SupportedFeatures != nil {
		for _, f := range *resp.SupportedFeatures {
			featVal, featDiags := resource_cloudconnect_aws_account.NewSupportedFeaturesValue(
				resource_cloudconnect_aws_account.SupportedFeaturesValue{}.AttributeTypes(ctx),
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

	featList, listDiags := types.ListValueFrom(ctx, resource_cloudconnect_aws_account.SupportedFeaturesType{
		ObjectType: types.ObjectType{
			AttrTypes: resource_cloudconnect_aws_account.SupportedFeaturesValue{}.AttributeTypes(ctx),
		},
	}, featElems)
	diags.Append(listDiags...)
	if listDiags.HasError() {
		return diags
	}
	state.SupportedFeatures = featList

	return diags
}

// overlayCloudConnectAwsAccountComputedFields uses the two-phase overlay pattern
// to reconcile the Terraform plan with the API response after Create/Update.
//
// Phase 1: Build a fully-resolved state from the API response.
// Phase 2: Overlay only Computed / Optional+Computed fields onto the plan.
//
// Field classification:
//   - account_id:          Required, ForceNew → never touch
//   - role_arn:            Required            → never touch
//   - enabled_features:    Required            → never touch (plan is authoritative)
//   - s3bucket:            Optional+Computed   → resolve when IsUnknown()
//   - s3bucket_region:     Optional+Computed   → resolve when IsUnknown()
//   - supported_features:  Computed-only       → always set from API
//   - time_linked:         Computed-only       → always set from API
func overlayCloudConnectAwsAccountComputedFields(ctx context.Context, apiResp *models.AwsAccountResponse, plan *cloudconnectAwsAccountResourceModel) diag.Diagnostics {
	// Phase 1: Build fully-resolved state from API response.
	resolved := *plan
	diags := mapCloudConnectAwsAccountToModel(ctx, apiResp, &resolved)
	if diags.HasError() {
		return diags
	}

	// Phase 2: Overlay Computed-only fields — always set from API.
	plan.SupportedFeatures = resolved.SupportedFeatures
	plan.TimeLinked = resolved.TimeLinked

	// Optional+Computed fields — resolve only when unknown (user omitted).
	if plan.S3bucket.IsUnknown() {
		plan.S3bucket = resolved.S3bucket
	}
	if plan.S3bucketRegion.IsUnknown() {
		plan.S3bucketRegion = resolved.S3bucketRegion
	}

	// account_id, role_arn: Required — never touch.

	return diags
}
