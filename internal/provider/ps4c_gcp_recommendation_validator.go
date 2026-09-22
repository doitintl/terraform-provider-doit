package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Compile-time interface assertions.
var (
	_ datasource.ConfigValidator = ps4cGcpRecommendationConfigValidator{}
	_ datasource.ConfigValidator = ps4cGcpRecommendationsConfigValidator{}
)

// ps4cGcpRecommendationConfigValidator validates that gcp_service and region combinations
// comply with PS4C GCP recommendation API constraints for the singular data source.
type ps4cGcpRecommendationConfigValidator struct{}

func (v ps4cGcpRecommendationConfigValidator) Description(_ context.Context) string {
	return "Validates that gcp_service and region combinations comply with PS4C GCP recommendation API constraints."
}

func (v ps4cGcpRecommendationConfigValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v ps4cGcpRecommendationConfigValidator) ValidateDataSource(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	var gcpService types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("gcp_service"), &gcpService)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var region types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("region"), &region)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if gcpService.IsUnknown() || region.IsUnknown() {
		return
	}

	if gcpService.IsNull() {
		return
	}

	switch gcpService.ValueString() {
	case "compute":
		if !region.IsNull() && region.ValueString() != "global" {
			resp.Diagnostics.AddAttributeError(
				path.Root("region"),
				"Invalid Attribute Combination",
				`region must be "global" or omitted when gcp_service is "compute"; Compute recommendations only support global scope.`,
			)
		}
	case "cloud_sql":
		if region.IsNull() {
			resp.Diagnostics.AddAttributeError(
				path.Root("region"),
				"Missing Required Attribute",
				`region is required when gcp_service is "cloud_sql".`,
			)
			return
		}
		if region.ValueString() == "global" {
			resp.Diagnostics.AddAttributeError(
				path.Root("region"),
				"Invalid Attribute Combination",
				`region cannot be "global" when gcp_service is "cloud_sql"; Cloud SQL recommendations require a concrete GCP region (e.g. "us_central1").`,
			)
		}
	}
}

// ps4cGcpRecommendationsConfigValidator validates that gcp_service and region filter combinations
// comply with PS4C GCP recommendations API constraints for the plural data source.
type ps4cGcpRecommendationsConfigValidator struct{}

func (v ps4cGcpRecommendationsConfigValidator) Description(_ context.Context) string {
	return "Validates that gcp_service and region filter combinations comply with PS4C GCP recommendations API constraints."
}

func (v ps4cGcpRecommendationsConfigValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v ps4cGcpRecommendationsConfigValidator) ValidateDataSource(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	var gcpService types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("gcp_service"), &gcpService)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var region types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("region"), &region)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if gcpService.IsUnknown() || region.IsUnknown() {
		return
	}

	// In the list endpoint, region filter requires gcp_service to be specified.
	if !region.IsNull() && gcpService.IsNull() {
		resp.Diagnostics.AddAttributeError(
			path.Root("region"),
			"Missing Required Attribute",
			`region requires gcp_service to be specified; specify gcp_service when filtering by region.`,
		)
		return
	}

	if region.IsNull() || gcpService.IsNull() {
		return
	}

	switch gcpService.ValueString() {
	case "compute":
		if region.ValueString() != "global" {
			resp.Diagnostics.AddAttributeError(
				path.Root("region"),
				"Invalid Attribute Combination",
				`region must be "global" when gcp_service is "compute"; Compute recommendations only support global scope.`,
			)
		}
	case "cloud_sql":
		if region.ValueString() == "global" {
			resp.Diagnostics.AddAttributeError(
				path.Root("region"),
				"Invalid Attribute Combination",
				`region cannot be "global" when gcp_service is "cloud_sql"; Cloud SQL recommendations require a concrete GCP region (e.g. "us_central1").`,
			)
		}
	}
}
