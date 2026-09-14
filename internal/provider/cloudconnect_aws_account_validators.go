package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// cloudconnectAwsS3RealTimeValidator validates that s3bucket and s3bucket_region
// are set when enabled_features contains "real-time-data", and vice versa.
type cloudconnectAwsS3RealTimeValidator struct{}

var _ resource.ConfigValidator = cloudconnectAwsS3RealTimeValidator{}

func (v cloudconnectAwsS3RealTimeValidator) Description(_ context.Context) string {
	return `s3bucket and s3bucket_region are required when enabled_features contains "real-time-data", and must not be set otherwise`
}

func (v cloudconnectAwsS3RealTimeValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v cloudconnectAwsS3RealTimeValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	// Read enabled_features from config.
	var enabledFeatures types.List
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("enabled_features"), &enabledFeatures)...)
	if resp.Diagnostics.HasError() || enabledFeatures.IsUnknown() {
		return
	}

	// Use []types.String so individual unknown elements are preserved
	// ([]string would error on unknown elements with allowUnhandled=false).
	var features []types.String
	resp.Diagnostics.Append(enabledFeatures.ElementsAs(ctx, &features, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hasRealTime := false
	hasUnknownFeature := false
	for _, f := range features {
		if f.IsUnknown() {
			hasUnknownFeature = true
			continue
		}
		if f.ValueString() == "real-time-data" {
			hasRealTime = true
		}
	}

	// Read s3bucket from config.
	var s3bucket types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("s3bucket"), &s3bucket)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var s3bucketRegion types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("s3bucket_region"), &s3bucketRegion)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if hasRealTime && (s3bucket.IsNull() || s3bucketRegion.IsNull()) {
		resp.Diagnostics.AddError(
			"Missing S3 Configuration for Real-Time Data",
			`When enabled_features contains "real-time-data", both s3bucket and s3bucket_region must be set.`,
		)
		return
	}

	if hasRealTime || hasUnknownFeature {
		if hasUnknownFeature {
			if (!s3bucket.IsUnknown() && !s3bucket.IsNull() && s3bucketRegion.IsNull()) ||
				(!s3bucketRegion.IsUnknown() && !s3bucketRegion.IsNull() && s3bucket.IsNull()) {
				resp.Diagnostics.AddError(
					"Incomplete S3 Configuration",
					"s3bucket and s3bucket_region must either both be set or both be omitted while enabled_features contains an unknown value.",
				)
			}
		}
		return
	}

	if (!s3bucket.IsNull() && !s3bucket.IsUnknown()) ||
		(!s3bucketRegion.IsNull() && !s3bucketRegion.IsUnknown()) {
		resp.Diagnostics.AddError(
			"S3 Configuration Without Real-Time Data",
			`s3bucket and s3bucket_region can only be set when enabled_features contains "real-time-data".`,
		)
	}
}
