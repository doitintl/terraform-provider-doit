package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ps4cGcpPlannedPurchasesServiceRegionValidator validates cross-attribute constraints
// between gcp_service and region on doit_ps4c_gcp_planned_purchases:
// 1. region requires gcp_service.
// 2. when gcp_service is "compute", region must be "global".
// 3. when gcp_service is "cloud_sql", region must be a concrete region, never "global".
type ps4cGcpPlannedPurchasesServiceRegionValidator struct{}

var _ datasource.ConfigValidator = ps4cGcpPlannedPurchasesServiceRegionValidator{}

func (v ps4cGcpPlannedPurchasesServiceRegionValidator) Description(_ context.Context) string {
	return "Validates that region and gcp_service configurations are compatible"
}

func (v ps4cGcpPlannedPurchasesServiceRegionValidator) MarkdownDescription(_ context.Context) string {
	return "Validates that `region` requires `gcp_service`, `compute` only accepts `global`, and `cloud_sql` rejects `global`"
}

func (v ps4cGcpPlannedPurchasesServiceRegionValidator) ValidateDataSource(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	var gcpService types.String
	var region types.String

	diags := req.Config.GetAttribute(ctx, path.Root("gcp_service"), &gcpService)
	resp.Diagnostics.Append(diags...)
	diags = req.Config.GetAttribute(ctx, path.Root("region"), &region)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if region.IsNull() || region.IsUnknown() {
		return
	}

	if gcpService.IsUnknown() {
		return
	}

	if gcpService.IsNull() {
		resp.Diagnostics.AddAttributeError(
			path.Root("region"),
			"Invalid Attribute Combination",
			"Attribute \"region\" requires \"gcp_service\" to be specified.",
		)
		return
	}

	serviceVal := gcpService.ValueString()
	regionVal := region.ValueString()

	if serviceVal == "compute" && regionVal != "global" {
		resp.Diagnostics.AddAttributeError(
			path.Root("region"),
			"Invalid Region for Service",
			fmt.Sprintf("When \"gcp_service\" is \"compute\", \"region\" must be \"global\", got: %q.", regionVal),
		)
		return
	}

	if serviceVal == "cloud_sql" && regionVal == "global" {
		resp.Diagnostics.AddAttributeError(
			path.Root("region"),
			"Invalid Region for Service",
			"When \"gcp_service\" is \"cloud_sql\", \"region\" must be a concrete GCP region (e.g. \"us_central1\"), never \"global\".",
		)
		return
	}
}

// ps4cGcpPlannedPurchasesPageTokenValidator validates that page_token requires max_results
// on doit_ps4c_gcp_planned_purchases.
type ps4cGcpPlannedPurchasesPageTokenValidator struct{}

var _ datasource.ConfigValidator = ps4cGcpPlannedPurchasesPageTokenValidator{}

func (v ps4cGcpPlannedPurchasesPageTokenValidator) Description(_ context.Context) string {
	return "Validates that page_token requires max_results"
}

func (v ps4cGcpPlannedPurchasesPageTokenValidator) MarkdownDescription(_ context.Context) string {
	return "Validates that `page_token` requires `max_results`"
}

func (v ps4cGcpPlannedPurchasesPageTokenValidator) ValidateDataSource(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	var pageToken types.String
	var maxResults types.Int64

	diags := req.Config.GetAttribute(ctx, path.Root("page_token"), &pageToken)
	resp.Diagnostics.Append(diags...)
	diags = req.Config.GetAttribute(ctx, path.Root("max_results"), &maxResults)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if pageToken.IsNull() || pageToken.IsUnknown() {
		return
	}

	if maxResults.IsUnknown() {
		return
	}

	if maxResults.IsNull() {
		resp.Diagnostics.AddAttributeError(
			path.Root("page_token"),
			"Missing Required Attribute",
			"Attribute \"page_token\" requires \"max_results\" to be specified.",
		)
		return
	}
}
