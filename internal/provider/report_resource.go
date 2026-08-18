package provider

import (
	"context"
	"fmt"

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
	_ resource.ResourceWithModifyPlan       = (*reportResource)(nil)
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

	// Category A: nested clearable filter values and mode.
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

		// Category A: group is clearable. Omitting the block proposes [], which the
		// request builder sends as "group": [], so removal takes effect instead of
		// being dropped by the PATCH merge. Clearing a group that carries a limit
		// relies on the API pruning the limit's server-side filter on the grouped
		// dimension; that filter is not exposed through the public `filters` field,
		// so the provider cannot reconcile it itself.
		//
		// Sending [] is also what keeps plans stable: group[*].limit.metric is
		// Required, so an API-populated group left in state while absent from config
		// permanently diffs the resource — see useNullForUnconfiguredMetricMirror.
		if attr, ok := configAttr.Attributes["group"].(schema.ListNestedAttribute); ok {
			attr.PlanModifiers = append(attr.PlanModifiers, useNullForUnknownListWhenConfigNull())
			configAttr.Attributes["group"] = attr
		}

		if fsAttr, ok := configAttr.Attributes["forecast_settings"].(schema.SingleNestedAttribute); ok {
			fsAttr.PlanModifiers = append(fsAttr.PlanModifiers, useNullOrDefaultForForecastSettings())
			configAttr.Attributes["forecast_settings"] = fsAttr
		}

		// metric (deprecated) and metrics are two views of one API field, and the
		// API returns both populated whichever one is configured. Keep the
		// unconfigured mirror out of state; their type/value leaves are Required, so
		// storing the echo permanently diffs the resource. Not a clearing
		// classification — both stay Category B below, since nothing is cleared
		// server-side. See useNullForUnconfiguredMetricMirror.
		//
		// They are also mutually exclusive: toExternalConfig sends only metrics when
		// both are set, so differing values would drift on every refresh. Reject the
		// combination instead of silently discarding one.
		if attr, ok := configAttr.Attributes["metric"].(schema.SingleNestedAttribute); ok {
			attr.PlanModifiers = append(attr.PlanModifiers, useNullForUnconfiguredMetricMirror())
			attr.Validators = append(attr.Validators, metricMirrorConflictValidator())
			configAttr.Attributes["metric"] = attr
		}
		if attr, ok := configAttr.Attributes["metrics"].(schema.ListNestedAttribute); ok {
			attr.PlanModifiers = append(attr.PlanModifiers, useEmptyForUnconfiguredMetricsMirror())
			configAttr.Attributes["metrics"] = attr
		}

		s.Attributes["config"] = configAttr
	}

	// Category B: API-computed defaults — not clearable.
	acknowledgeNotClearable(s,
		"name",   // API generates default name
		"labels", // preset labels are API-assigned

		// config top-level
		"config",                             // top-level O+C wrapper, silently preserved (no drift)
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

		// config.group
		"config.group[*].id",          // API-assigned group ID
		"config.group[*].type",        // API provides default type
		"config.group[*].limit.sort",  // API defaults sort direction
		"config.group[*].limit.value", // API defaults limit value

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

		// config.secondary_time_range
		"config.secondary_time_range.unit",                   // API defaults unit
		"config.secondary_time_range.amount",                 // API defaults amount
		"config.secondary_time_range.include_current",        // API defaults to false
		"config.secondary_time_range.custom_time_range.from", // API defaults from date
		"config.secondary_time_range.custom_time_range.to",   // API defaults to date

		// config.forecast_settings
		"config.forecast_settings.future_custom_date_range",
		"config.forecast_settings.future_custom_date_range.from",
		"config.forecast_settings.future_custom_date_range.to",
		"config.forecast_settings.historical_custom_date_range",
		"config.forecast_settings.historical_custom_date_range.from",
		"config.forecast_settings.historical_custom_date_range.to",
		"config.forecast_settings.future_time_intervals",
		"config.forecast_settings.historical_time_intervals",

		// Nested single-nested object containers that are silently preserved when
		// removed (Category B): omitting the block is idempotent — the prior value
		// sticks with no drift and no error, matching the default Optional+Computed
		// behavior for unclearable leaves. Verified by the TestAccReport_*_NotClearable
		// acceptance tests. (The *harmful* containers — count, limit_by_change,
		// metric_filter, custom_time_range — are handled as Category C via
		// requiresReplaceWhenCleared in ModifyPlan instead, not acknowledged here.)
		"config.advanced_analysis",                      // silently preserved on removal
		"config.display_settings",                       // silently preserved on removal
		"config.metric",                                 // deprecated mirror of metrics[0]; API always derives it, never clears
		"config.secondary_time_range",                   // silently preserved on removal
		"config.secondary_time_range.custom_time_range", // silently preserved on removal
		"config.time_range",                             // silently preserved on removal
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
		// forecast_settings + forecast=false is contradictory.
		reportForecastConflictValidator{},
		// custom_time_range.from/to must be valid RFC3339 timestamps.
		reportTimestampValidator{},
		// count is only valid when aggregation is "count".
		reportCountAggregationValidator{},
		// unit has no meaning when time_range.mode is "custom".
		reportCustomTimeRangeUnitValidator{},
		// Warn when legacy [... N/A] NullFallback sentinels are used in filter values.
		reportFilterNAValidator{},
	}
}

// ModifyPlan forces replacement for Optional+Computed nested objects that the API
// cannot clear in place when they are removed from config. The report update is a
// PATCH that merges config and these fields have no null representation, so
// removing them from config is either impossible or actively harmful:
//
//   - config.count: perpetual drift (or a 400 if aggregation also leaves "count").
//   - config.limit_by_change: perpetual drift (the merge retains the stored value).
//   - config.metric_filter: the update sends an empty metricFilter and the API
//     rejects it (400 "invalid number of values: 0").
//   - config.custom_time_range: the stored range is retained and conflicts with a
//     changed time_range.mode, so the API rejects the update (400).
//
// Replacing the resource honors the removal via destroy+create instead. This is a
// stopgap (Category C) until the API marks these fields nullable (then they become
// Category A, cleared with an explicit null like config.forecast_settings).
//
// Silently-preserved Optional+Computed objects (advanced_analysis, display_settings,
// time_range, secondary_time_range, metric, and the forecast_settings custom date
// ranges) are intentionally NOT listed: removing them is idempotent (the prior
// value sticks with no drift), matching the default Category B behavior for
// unclearable leaves, so forcing a replace would be gratuitously destructive.
func (r *reportResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// Skip on create (no prior state) and destroy (no plan).
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}

	requiresReplaceWhenCleared[resource_report.CountValue](ctx, req, resp, path.Root("config").AtName("count"))
	requiresReplaceWhenCleared[resource_report.LimitByChangeValue](ctx, req, resp, path.Root("config").AtName("limit_by_change"))
	requiresReplaceWhenCleared[resource_report.MetricFilterValue](ctx, req, resp, path.Root("config").AtName("metric_filter"))
	requiresReplaceWhenCleared[resource_report.CustomTimeRangeValue](ctx, req, resp, path.Root("config").AtName("custom_time_range"))
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

	createTimeout, diags := plan.Timeouts.Create(ctx, DefaultCreateTimeout)
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

	readTimeout, diags := state.Timeouts.Read(ctx, DefaultReadTimeout)
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

	updateTimeout, diags := plan.Timeouts.Update(ctx, DefaultUpdateTimeout)
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

	deleteTimeout, diags := state.Timeouts.Delete(ctx, DefaultDeleteTimeout)
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
