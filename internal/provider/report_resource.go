package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_report"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

var (
	_ resource.Resource                     = (*reportResource)(nil)
	_ resource.ResourceWithConfigure        = (*reportResource)(nil)
	_ resource.ResourceWithImportState      = (*reportResource)(nil)
	_ resource.ResourceWithConfigValidators = (*reportResource)(nil)
)

// NewReportResource creates a new report resource instance.
func NewReportResource() resource.Resource {
	return &reportResource{}
}

type (
	reportResource struct {
		client *models.ClientWithResponses
	}
	reportResourceModel struct {
		resource_report.ReportModel
		Timeouts timeouts.Value `tfsdk:"timeouts"`
	}
)

func (r *reportResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_report"
}

func (r *reportResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := resource_report.ReportResourceSchema(ctx)

	// Add UseStateForUnknown to stable Computed-only fields so they don't
	// show as "(known after apply)" on every plan that modifies the resource.
	if attr, ok := s.Attributes["id"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["id"] = attr
	}
	if attr, ok := s.Attributes["type"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["type"] = attr
	}

	// Classify Optional+Computed attributes (clearableattr).
	// See: https://github.com/doitintl/terraform-provider-doit/issues/233

	// Category A: user-authored content — clearable.
	if attr, ok := s.Attributes["description"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, useEmptyForUnknownWhenConfigNull())
		s.Attributes["description"] = attr
	}

	// Category A: nested clearable filter/metric_filter values and mode.
	if configAttr, ok := s.Attributes["config"].(schema.SingleNestedAttribute); ok {
		if filtersAttr, ok := configAttr.Attributes["filters"].(schema.ListNestedAttribute); ok {
			if attr, ok := filtersAttr.NestedObject.Attributes["values"].(schema.ListAttribute); ok {
				attr.PlanModifiers = append(attr.PlanModifiers, useNullForUnknownListWhenConfigNull())
				filtersAttr.NestedObject.Attributes["values"] = attr
			}
			if attr, ok := filtersAttr.NestedObject.Attributes["mode"].(schema.StringAttribute); ok {
				attr.PlanModifiers = append(attr.PlanModifiers, useNullForUnknownStringWhenConfigNull())
				filtersAttr.NestedObject.Attributes["mode"] = attr
			}
			configAttr.Attributes["filters"] = filtersAttr
		}
		if mfAttr, ok := configAttr.Attributes["metric_filter"].(schema.SingleNestedAttribute); ok {
			if attr, ok := mfAttr.Attributes["values"].(schema.ListAttribute); ok {
				attr.PlanModifiers = append(attr.PlanModifiers, useNullForUnknownListWhenConfigNull())
				mfAttr.Attributes["values"] = attr
			}
			configAttr.Attributes["metric_filter"] = mfAttr
		}
		// limit_by_change is an Optional+Computed nested object. The generator marks
		// all its children Required (the upstream schema lists them as required). But
		// an Optional+Computed object with Required children is a problem: when the
		// object is omitted, Terraform cannot copy the prior state (as it does for an
		// all-optional object like `metric`) and instead marks the whole object — and
		// the entire parent `config` — "known after apply", producing a permadiff on
		// every plan. Relaxing the children to Optional+Computed makes limit_by_change
		// behave like `metric` (prior state is retained on omit, no drift). The API
		// still requires these fields when the object is set; that is enforced at plan
		// time by reportLimitByChangeFieldsValidator.
		if lbcAttr, ok := configAttr.Attributes["limit_by_change"].(schema.SingleNestedAttribute); ok {
			if a, ok := lbcAttr.Attributes["change_type"].(schema.StringAttribute); ok {
				a.Required, a.Optional, a.Computed = false, true, true
				lbcAttr.Attributes["change_type"] = a
			}
			if a, ok := lbcAttr.Attributes["operator"].(schema.StringAttribute); ok {
				a.Required, a.Optional, a.Computed = false, true, true
				lbcAttr.Attributes["operator"] = a
			}
			if a, ok := lbcAttr.Attributes["values"].(schema.ListAttribute); ok {
				a.Required, a.Optional, a.Computed = false, true, true
				lbcAttr.Attributes["values"] = a
			}
			if a, ok := lbcAttr.Attributes["include_incomplete_data"].(schema.BoolAttribute); ok {
				a.Required, a.Optional, a.Computed = false, true, true
				lbcAttr.Attributes["include_incomplete_data"] = a
			}
			if a, ok := lbcAttr.Attributes["metric"].(schema.SingleNestedAttribute); ok {
				a.Required, a.Optional, a.Computed = false, true, true
				lbcAttr.Attributes["metric"] = a
			}
			configAttr.Attributes["limit_by_change"] = lbcAttr
		}
		s.Attributes["config"] = configAttr
	}

	// Category B: API-computed defaults — not clearable.
	acknowledgeNotClearable(s,
		"name",   // API generates default name
		"labels", // preset labels are API-assigned

		// config top-level
		"config.currency",                    // API defaults to org currency
		"config.time_interval",               // API defaults time interval
		"config.data_source",                 // API defaults data source
		"config.aggregation",                 // API defaults aggregation
		"config.display_values",              // API defaults display values
		"config.include_promotional_credits", // API defaults to false
		"config.layout",                      // API defaults layout

		// config.dimensions
		"config.dimensions[*].type", // API provides default type
		"config.dimensions[*].id",   // API-assigned dimension ID

		// config.filters
		"config.filters[*].inverse", // API defaults to false

		// config.metrics
		"config.metrics[*].type",  // API requires type (not clearable)
		"config.metrics[*].value", // API requires value (not clearable)

		// config.metric_filter
		"config.metric_filter.operator",     // API defaults operator
		"config.metric_filter.metric.type",  // API requires type (not clearable)
		"config.metric_filter.metric.value", // API requires value (not clearable)

		// config.limit_by_change: metric.type/value are Optional+Computed leaves the
		// API requires (not clearable). The object's other children are Required in the
		// generated schema (relaxed to Optional+Computed at runtime to avoid an omit-time
		// permadiff; presence enforced by reportLimitByChangeFieldsValidator).
		"config.limit_by_change.metric.type",  // API requires type (not clearable)
		"config.limit_by_change.metric.value", // API requires value (not clearable)

		// config.group
		"config.group[*].id",                 // API-assigned group ID
		"config.group[*].type",               // API provides default type
		"config.group[*].limit.sort",         // API defaults sort direction
		"config.group[*].limit.value",        // API defaults limit value
		"config.group[*].limit.metric.type",  // API requires type (not clearable)
		"config.group[*].limit.metric.value", // API requires value (not clearable)

		// config.advanced_analysis
		"config.advanced_analysis.forecast",      // API defaults to false
		"config.advanced_analysis.not_trending",  // API defaults to false
		"config.advanced_analysis.trending_up",   // API defaults to false
		"config.advanced_analysis.trending_down", // API defaults to false

		// config.display_settings
		"config.display_settings.number_scale",         // API defaults number scale
		"config.display_settings.axis_label_font_size", // API defaults font size
		"config.display_settings.data_label_font_size", // API defaults font size
		"config.display_settings.decimal_precision",    // API defaults precision

		// config.time_range
		"config.time_range.mode",            // API defaults mode
		"config.time_range.unit",            // API defaults unit
		"config.time_range.amount",          // API defaults amount
		"config.time_range.include_current", // API defaults to false

		// config.custom_time_range
		"config.custom_time_range.from", // API defaults from date
		"config.custom_time_range.to",   // API defaults to date

		// config.splits
		"config.splits[*].id",               // API-assigned split ID
		"config.splits[*].type",             // API provides default type
		"config.splits[*].mode",             // API provides default mode
		"config.splits[*].include_origin",   // API defaults to false
		"config.splits[*].origin.type",      // API provides default type
		"config.splits[*].origin.id",        // API-assigned origin ID
		"config.splits[*].targets[*].id",    // API-assigned target ID
		"config.splits[*].targets[*].type",  // API provides default type
		"config.splits[*].targets[*].value", // API provides default value

		// config.metric (top-level)
		"config.metric.type",  // API requires type (not clearable)
		"config.metric.value", // API requires value (not clearable)

		// config.secondary_time_range
		"config.secondary_time_range.unit",                   // API defaults unit
		"config.secondary_time_range.amount",                 // API defaults amount
		"config.secondary_time_range.include_current",        // API defaults to false
		"config.secondary_time_range.custom_time_range.from", // API defaults from date
		"config.secondary_time_range.custom_time_range.to",   // API defaults to date
	)

	s.Attributes["timeouts"] = timeouts.Attributes(ctx, timeouts.Opts{
		Create: true,
		Read:   true,
		Update: true,
		Delete: true,
	})

	resp.Schema = s
}
func (r *reportResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*models.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *models.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *reportResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		// Must have 1-4 metrics if specified; empty list causes state inconsistency.
		reportMetricsLengthValidator{},
		// custom_time_range.from/to must be valid RFC3339 timestamps.
		reportTimestampValidator{},
		// Every configured metric object must set both type and value (API requires them).
		reportMetricFieldsValidator{},
		// When limit_by_change is set, its API-required fields must be provided.
		reportLimitByChangeFieldsValidator{},
		// Warn when legacy [... N/A] NullFallback sentinels are used in filter values.
		reportFilterNAValidator{},
	}
}

func (r *reportResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *reportResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan reportResourceModel

	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, diags := plan.Timeouts.Create(ctx, 5*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	reportReq, diags := plan.toCreateRequest(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	reportResp, err := r.client.CreateReportWithResponse(ctx, reportReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating report",
			"Could not create report, unexpected error: "+err.Error(),
		)
		return
	}

	if reportResp.StatusCode() != 201 {
		resp.Diagnostics.AddError(
			"Error creating report",
			fmt.Sprintf("Could not create report, status: %d, body: %s", reportResp.StatusCode(), string(reportResp.Body)),
		)
		return
	}

	if reportResp.JSON201 == nil {
		resp.Diagnostics.AddError(
			"Error creating report",
			"Could not create report, empty response",
		)
		return
	}

	if reportResp.JSON201.Id == nil {
		resp.Diagnostics.AddError(
			"Error creating report",
			"Could not create report, response missing ID",
		)
		return
	}

	// Plan-first: preserve the user's explicit plan values, while resolving Unknown
	// fields from the API response (id, type, labels, name, description, and nested
	// config fields). This avoids API normalization drift (sentinel stripping, alias
	// renaming, etc.) for all user-configured values.
	diags = overlayReportComputedFields(ctx, reportResp.JSON201, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *reportResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state reportResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	readTimeout, diags := state.Timeouts.Read(ctx, 2*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	diags = r.populateState(ctx, &state)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Handle externally deleted resource (populateState sets Id to null on 404)
	if state.Id.IsNull() {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *reportResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan reportResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateTimeout, diags := plan.Timeouts.Update(ctx, 5*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	var state reportResourceModel
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)

	reportReq, diags := plan.toUpdateRequest(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	reportResp, err := r.client.UpdateReportWithResponse(ctx, state.Id.ValueString(), reportReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating report",
			"Could not update report, unexpected error: "+err.Error(),
		)
		return
	}

	if reportResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error updating report",
			fmt.Sprintf("Could not update report, status: %d, body: %s", reportResp.StatusCode(), string(reportResp.Body)),
		)
		return
	}

	if reportResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error updating report",
			"Could not update report, empty response",
		)
		return
	}

	if reportResp.JSON200.Id == nil {
		resp.Diagnostics.AddError(
			"Error updating report",
			"Could not update report, response missing ID",
		)
		return
	}

	// Plan-first: preserve the user's explicit plan values, while resolving Unknown
	// fields from the API response (id, type, labels, name, description, and nested
	// config fields). This avoids API normalization drift (sentinel stripping, alias
	// renaming, etc.) for all user-configured values.
	diags = overlayReportComputedFields(ctx, reportResp.JSON200, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *reportResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state reportResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, diags := state.Timeouts.Delete(ctx, 2*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	deleteResp, err := r.client.DeleteReportWithResponse(ctx, state.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting report",
			"Could not delete report, unexpected error: "+err.Error(),
		)
		return
	}

	// Treat 404 as success - resource is already gone (deleted outside Terraform)
	if deleteResp.StatusCode() != 200 && deleteResp.StatusCode() != 204 && deleteResp.StatusCode() != 404 {
		resp.Diagnostics.AddError(
			"Error deleting report",
			fmt.Sprintf("Could not delete report, status: %d, body: %s", deleteResp.StatusCode(), string(deleteResp.Body)),
		)
		return
	}
}
