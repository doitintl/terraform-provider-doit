// Package provider implements the DoiT Terraform provider.
package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_cloudconnect_aws_account"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// populateState fetches the CloudConnect AWS account from the API and populates the
// Terraform state. On 404, state.AccountId is set to null to signal Terraform to
// remove the resource from state.
func (r *cloudconnectAwsAccountResource) populateState(ctx context.Context, state *cloudconnectAwsAccountResourceModel) diag.Diagnostics {
	accountResp, err := r.client.GetAwsAccountWithResponse(ctx, state.AccountId.ValueString())
	if err != nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading CloudConnect AWS Account",
				"Could not read CloudConnect AWS account ID "+state.AccountId.ValueString()+": "+err.Error()),
		}
	}

	if accountResp.StatusCode() == 404 {
		state.AccountId = types.StringNull()
		return nil
	}

	if accountResp.StatusCode() != 200 {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading CloudConnect AWS Account",
				fmt.Sprintf("Unexpected status code %d for CloudConnect AWS account ID %s: %s",
					accountResp.StatusCode(), state.AccountId.ValueString(), string(accountResp.Body))),
		}
	}

	if accountResp.JSON200 == nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading CloudConnect AWS Account",
				"Received empty response body for CloudConnect AWS account ID "+state.AccountId.ValueString()),
		}
	}

	return mapCloudConnectAwsAccountToModel(ctx, accountResp.JSON200, state)
}

// mapCloudConnectAwsAccountToModel maps ALL fields from the API response to the
// Terraform model. Used by populateState (Read/ImportState) and as Phase 1 of the overlay pattern.
func mapCloudConnectAwsAccountToModel(ctx context.Context, resp *models.AwsAccountResponse, state *cloudconnectAwsAccountResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	state.AccountId = types.StringPointerValue(resp.AccountID)
	state.RoleArn = types.StringPointerValue(resp.RoleArn)
	state.TimeLinked = types.StringPointerValue(resp.TimeLinked)

	// s3bucket and s3bucket_region are clearable (Category A): normalize nil → ""
	// to match the useEmptyForUnknownWhenConfigNull() modifier's plan value.
	// Without this, a nil API response would produce StringNull(), drifting
	// against the "" proposed by the modifier when the user omits the field.
	if resp.S3Bucket != nil {
		state.S3bucket = types.StringValue(*resp.S3Bucket)
	} else {
		state.S3bucket = types.StringValue("")
	}
	if resp.S3BucketRegion != nil {
		state.S3bucketRegion = types.StringValue(*resp.S3BucketRegion)
	} else {
		state.S3bucketRegion = types.StringValue("")
	}

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
