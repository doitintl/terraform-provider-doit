package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// reportMetricsLengthValidator validates that metrics list has 1-4 items when specified.
// The API supports up to 4 metrics per report.
type reportMetricsLengthValidator struct{}

var _ resource.ConfigValidator = reportMetricsLengthValidator{}

func (v reportMetricsLengthValidator) Description(_ context.Context) string {
	return "Validates that metrics list has 1-4 items when specified"
}

func (v reportMetricsLengthValidator) MarkdownDescription(_ context.Context) string {
	return "Validates that `metrics` list has 1-4 items when specified"
}

func (v reportMetricsLengthValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var metrics types.List

	// Get config.metrics
	diags := req.Config.GetAttribute(ctx, path.Root("config").AtName("metrics"), &metrics)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Skip validation if null/unknown - let API handle default behavior
	if metrics.IsNull() || metrics.IsUnknown() {
		return
	}

	// Check for empty list
	// The API silently preserves existing metrics when an empty array is sent,
	// rather than clearing them. This causes a Terraform consistency error
	// because the planned state (empty) differs from the actual state (preserved).
	// To prevent this, we require at least one metric if the attribute is specified.
	if len(metrics.Elements()) == 0 {
		resp.Diagnostics.AddAttributeError(
			path.Root("config").AtName("metrics"),
			"Empty Metrics List Not Supported",
			"The 'metrics' attribute cannot be set to an empty list. "+
				"The API does not support clearing metrics with an empty array - it silently preserves existing values, "+
				"causing Terraform state inconsistency. Options:\n"+
				"  - Specify 1-4 metrics: metrics = [{type = \"basic\", value = \"cost\"}]\n"+
				"  - Omit the attribute entirely to let the API use its default metric",
		)
		return
	}

	// Check for max 4 metrics
	if len(metrics.Elements()) > 4 {
		resp.Diagnostics.AddAttributeError(
			path.Root("config").AtName("metrics"),
			"Invalid Metrics Configuration",
			fmt.Sprintf("Report can have up to 4 metrics. Found %d metrics.", len(metrics.Elements())),
		)
	}
}

// reportTimestampValidator validates that custom_time_range.from/to values are valid
// RFC3339 timestamps at plan time. This is a ConfigValidator because attribute-level
// validators do not fire on attributes inside SingleNestedAttribute with CustomType
// (which the code generator adds to all nested objects).
type reportTimestampValidator struct{}

var _ resource.ConfigValidator = reportTimestampValidator{}

func (v reportTimestampValidator) Description(_ context.Context) string {
	return "Validates RFC3339 timestamps in custom_time_range and secondary_time_range.custom_time_range"
}

func (v reportTimestampValidator) MarkdownDescription(_ context.Context) string {
	return "Validates RFC3339 timestamps in `custom_time_range` and `secondary_time_range.custom_time_range`"
}

func (v reportTimestampValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	timestampPaths := []path.Path{
		path.Root("config").AtName("custom_time_range").AtName("from"),
		path.Root("config").AtName("custom_time_range").AtName("to"),
		path.Root("config").AtName("secondary_time_range").AtName("custom_time_range").AtName("from"),
		path.Root("config").AtName("secondary_time_range").AtName("custom_time_range").AtName("to"),
	}

	for _, p := range timestampPaths {
		var val types.String
		diags := req.Config.GetAttribute(ctx, p, &val)
		resp.Diagnostics.Append(diags...)
		if diags.HasError() {
			continue
		}
		if val.IsNull() || val.IsUnknown() {
			continue
		}
		validateRFC3339(val.ValueString(), p, &resp.Diagnostics)
	}
}
