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
	for _, f := range features {
		if f.IsUnknown() {
			// Cannot safely validate S3 requirements if any feature is unknown.
			return
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

	// !IsNull() is sufficient — if the value is unknown the user configured
	// something (e.g. a reference), so validation should not reject it.
	hasS3 := !s3bucket.IsNull()
	hasS3Region := !s3bucketRegion.IsNull()

	if hasRealTime && (!hasS3 || !hasS3Region) {
		resp.Diagnostics.AddError(
			"Missing S3 Configuration for Real-Time Data",
			`When enabled_features contains "real-time-data", both s3bucket and s3bucket_region must be set.`,
		)
	}

	if !hasRealTime && (hasS3 || hasS3Region) {
		resp.Diagnostics.AddError(
			"S3 Configuration Without Real-Time Data",
			`s3bucket and s3bucket_region can only be set when enabled_features contains "real-time-data".`,
		)
	}
}
