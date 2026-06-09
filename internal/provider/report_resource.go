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
		attr.PlanModifiers = append(attr.PlanModifiers, useNullForUnknownWhenConfigNull())
		s.Attributes["description"] = attr
	}

	// Category B: API may asynchronously assign preset labels (e.g. "Cached").
	if attr, ok := s.Attributes["labels"].(schema.ListAttribute); ok { //nolint:clearableattr // preset labels are API-assigned
		s.Attributes["labels"] = attr
	}

	// Category B: API always provides a value.
	if attr, ok := s.Attributes["name"].(schema.StringAttribute); ok { //nolint:clearableattr // API generates default name
		s.Attributes["name"] = attr
	}

	// Category B: nested config fields — API-computed defaults.
	if configAttr, ok := s.Attributes["config"].(schema.SingleNestedAttribute); ok {
		if attr, ok := configAttr.Attributes["currency"].(schema.StringAttribute); ok { //nolint:clearableattr // API defaults to org currency
			configAttr.Attributes["currency"] = attr
		}
		if attr, ok := configAttr.Attributes["time_interval"].(schema.StringAttribute); ok { //nolint:clearableattr // API defaults time interval
			configAttr.Attributes["time_interval"] = attr
		}
		if attr, ok := configAttr.Attributes["data_source"].(schema.StringAttribute); ok { //nolint:clearableattr // API defaults data source
			configAttr.Attributes["data_source"] = attr
		}

		// Nested dimensions.
		if dimsAttr, ok := configAttr.Attributes["dimensions"].(schema.ListNestedAttribute); ok {
			if attr, ok := dimsAttr.NestedObject.Attributes["type"].(schema.StringAttribute); ok { //nolint:clearableattr // API provides default type
				dimsAttr.NestedObject.Attributes["type"] = attr
			}
			configAttr.Attributes["dimensions"] = dimsAttr
		}

		// Nested filters.
		if filtersAttr, ok := configAttr.Attributes["filters"].(schema.ListNestedAttribute); ok {
			if attr, ok := filtersAttr.NestedObject.Attributes["inverse"].(schema.BoolAttribute); ok { //nolint:clearableattr // API defaults to false
				filtersAttr.NestedObject.Attributes["inverse"] = attr
			}
			if attr, ok := filtersAttr.NestedObject.Attributes["values"].(schema.ListAttribute); ok {
				attr.PlanModifiers = append(attr.PlanModifiers, useNullForUnknownListWhenConfigNull())
				filtersAttr.NestedObject.Attributes["values"] = attr
			}
			configAttr.Attributes["filters"] = filtersAttr
		}

		// Nested metrics.
		if metricsAttr, ok := configAttr.Attributes["metrics"].(schema.ListNestedAttribute); ok {
			if attr, ok := metricsAttr.NestedObject.Attributes["type"].(schema.StringAttribute); ok { //nolint:clearableattr // API provides default type
				metricsAttr.NestedObject.Attributes["type"] = attr
			}
			if attr, ok := metricsAttr.NestedObject.Attributes["value"].(schema.StringAttribute); ok { //nolint:clearableattr // API provides default value
				metricsAttr.NestedObject.Attributes["value"] = attr
			}
			configAttr.Attributes["metrics"] = metricsAttr
		}

		// Nested metric_filter.
		if mfAttr, ok := configAttr.Attributes["metric_filter"].(schema.SingleNestedAttribute); ok {
			if attr, ok := mfAttr.Attributes["values"].(schema.ListAttribute); ok {
				attr.PlanModifiers = append(attr.PlanModifiers, useNullForUnknownListWhenConfigNull())
				mfAttr.Attributes["values"] = attr
			}
			if attr, ok := mfAttr.Attributes["operator"].(schema.StringAttribute); ok { //nolint:clearableattr // API defaults operator
				mfAttr.Attributes["operator"] = attr
			}
			if metricAttr, ok := mfAttr.Attributes["metric"].(schema.SingleNestedAttribute); ok {
				if attr, ok := metricAttr.Attributes["type"].(schema.StringAttribute); ok { //nolint:clearableattr // API provides default type
					metricAttr.Attributes["type"] = attr
				}
				if attr, ok := metricAttr.Attributes["value"].(schema.StringAttribute); ok { //nolint:clearableattr // API provides default value
					metricAttr.Attributes["value"] = attr
				}
				mfAttr.Attributes["metric"] = metricAttr
			}
			configAttr.Attributes["metric_filter"] = mfAttr
		}

		// Nested group.
		if groupAttr, ok := configAttr.Attributes["group"].(schema.ListNestedAttribute); ok {
			if attr, ok := groupAttr.NestedObject.Attributes["id"].(schema.StringAttribute); ok { //nolint:clearableattr // API-assigned group ID
				groupAttr.NestedObject.Attributes["id"] = attr
			}
			if attr, ok := groupAttr.NestedObject.Attributes["type"].(schema.StringAttribute); ok { //nolint:clearableattr // API provides default type
				groupAttr.NestedObject.Attributes["type"] = attr
			}
			if limitAttr, ok := groupAttr.NestedObject.Attributes["limit"].(schema.SingleNestedAttribute); ok {
				if attr, ok := limitAttr.Attributes["sort"].(schema.StringAttribute); ok { //nolint:clearableattr // API defaults sort direction
					limitAttr.Attributes["sort"] = attr
				}
				if attr, ok := limitAttr.Attributes["value"].(schema.Int64Attribute); ok { //nolint:clearableattr // API defaults limit value
					limitAttr.Attributes["value"] = attr
				}
				if metricAttr, ok := limitAttr.Attributes["metric"].(schema.SingleNestedAttribute); ok {
					if attr, ok := metricAttr.Attributes["type"].(schema.StringAttribute); ok { //nolint:clearableattr // API provides default type
						metricAttr.Attributes["type"] = attr
					}
					if attr, ok := metricAttr.Attributes["value"].(schema.StringAttribute); ok { //nolint:clearableattr // API provides default value
						metricAttr.Attributes["value"] = attr
					}
					limitAttr.Attributes["metric"] = metricAttr
				}
				groupAttr.NestedObject.Attributes["limit"] = limitAttr
			}
			configAttr.Attributes["group"] = groupAttr
		}

		// Nested advanced_analysis.
		if aaAttr, ok := configAttr.Attributes["advanced_analysis"].(schema.SingleNestedAttribute); ok {
			if attr, ok := aaAttr.Attributes["forecast"].(schema.BoolAttribute); ok { //nolint:clearableattr // API defaults to false
				aaAttr.Attributes["forecast"] = attr
			}
			if attr, ok := aaAttr.Attributes["not_trending"].(schema.BoolAttribute); ok { //nolint:clearableattr // API defaults to false
				aaAttr.Attributes["not_trending"] = attr
			}
			if attr, ok := aaAttr.Attributes["trending_up"].(schema.BoolAttribute); ok { //nolint:clearableattr // API defaults to false
				aaAttr.Attributes["trending_up"] = attr
			}
			if attr, ok := aaAttr.Attributes["trending_down"].(schema.BoolAttribute); ok { //nolint:clearableattr // API defaults to false
				aaAttr.Attributes["trending_down"] = attr
			}
			configAttr.Attributes["advanced_analysis"] = aaAttr
		}

		// Nested display_settings.
		if dsAttr, ok := configAttr.Attributes["display_settings"].(schema.SingleNestedAttribute); ok {
			if attr, ok := dsAttr.Attributes["number_scale"].(schema.StringAttribute); ok { //nolint:clearableattr // API defaults number scale
				dsAttr.Attributes["number_scale"] = attr
			}
			if attr, ok := dsAttr.Attributes["axis_label_font_size"].(schema.Int64Attribute); ok { //nolint:clearableattr // API defaults font size
				dsAttr.Attributes["axis_label_font_size"] = attr
			}
			if attr, ok := dsAttr.Attributes["data_label_font_size"].(schema.Int64Attribute); ok { //nolint:clearableattr // API defaults font size
				dsAttr.Attributes["data_label_font_size"] = attr
			}
			if attr, ok := dsAttr.Attributes["decimal_precision"].(schema.Int64Attribute); ok { //nolint:clearableattr // API defaults precision
				dsAttr.Attributes["decimal_precision"] = attr
			}
			configAttr.Attributes["display_settings"] = dsAttr
		}

		// Nested time_range.
		if trAttr, ok := configAttr.Attributes["time_range"].(schema.SingleNestedAttribute); ok {
			if attr, ok := trAttr.Attributes["mode"].(schema.StringAttribute); ok { //nolint:clearableattr // API defaults mode
				trAttr.Attributes["mode"] = attr
			}
			if attr, ok := trAttr.Attributes["unit"].(schema.StringAttribute); ok { //nolint:clearableattr // API defaults unit
				trAttr.Attributes["unit"] = attr
			}
			if attr, ok := trAttr.Attributes["amount"].(schema.Int64Attribute); ok { //nolint:clearableattr // API defaults amount
				trAttr.Attributes["amount"] = attr
			}
			if attr, ok := trAttr.Attributes["include_current"].(schema.BoolAttribute); ok { //nolint:clearableattr // API defaults to false
				trAttr.Attributes["include_current"] = attr
			}
			configAttr.Attributes["time_range"] = trAttr
		}

		// Nested custom_time_range.
		if ctrAttr, ok := configAttr.Attributes["custom_time_range"].(schema.SingleNestedAttribute); ok {
			if attr, ok := ctrAttr.Attributes["from"].(schema.StringAttribute); ok { //nolint:clearableattr // API defaults from date
				ctrAttr.Attributes["from"] = attr
			}
			if attr, ok := ctrAttr.Attributes["to"].(schema.StringAttribute); ok { //nolint:clearableattr // API defaults to date
				ctrAttr.Attributes["to"] = attr
			}
			configAttr.Attributes["custom_time_range"] = ctrAttr
		}

		if attr, ok := configAttr.Attributes["aggregation"].(schema.StringAttribute); ok { //nolint:clearableattr // API defaults aggregation
			configAttr.Attributes["aggregation"] = attr
		}
		if attr, ok := configAttr.Attributes["display_values"].(schema.StringAttribute); ok { //nolint:clearableattr // API defaults display values
			configAttr.Attributes["display_values"] = attr
		}

		// Nested splits.
		if splitsAttr, ok := configAttr.Attributes["splits"].(schema.ListNestedAttribute); ok {
			if attr, ok := splitsAttr.NestedObject.Attributes["id"].(schema.StringAttribute); ok { //nolint:clearableattr // API-assigned split ID
				splitsAttr.NestedObject.Attributes["id"] = attr
			}
			if attr, ok := splitsAttr.NestedObject.Attributes["type"].(schema.StringAttribute); ok { //nolint:clearableattr // API provides default type
				splitsAttr.NestedObject.Attributes["type"] = attr
			}
			if attr, ok := splitsAttr.NestedObject.Attributes["mode"].(schema.StringAttribute); ok { //nolint:clearableattr // API provides default mode
				splitsAttr.NestedObject.Attributes["mode"] = attr
			}
			if originAttr, ok := splitsAttr.NestedObject.Attributes["origin"].(schema.SingleNestedAttribute); ok {
				if attr, ok := originAttr.Attributes["type"].(schema.StringAttribute); ok { //nolint:clearableattr // API provides default type
					originAttr.Attributes["type"] = attr
				}
				if attr, ok := originAttr.Attributes["id"].(schema.StringAttribute); ok { //nolint:clearableattr // API-assigned origin ID
					originAttr.Attributes["id"] = attr
				}
				splitsAttr.NestedObject.Attributes["origin"] = originAttr
			}
			if targetsAttr, ok := splitsAttr.NestedObject.Attributes["targets"].(schema.ListNestedAttribute); ok {
				if attr, ok := targetsAttr.NestedObject.Attributes["id"].(schema.StringAttribute); ok { //nolint:clearableattr // API-assigned target ID
					targetsAttr.NestedObject.Attributes["id"] = attr
				}
				if attr, ok := targetsAttr.NestedObject.Attributes["type"].(schema.StringAttribute); ok { //nolint:clearableattr // API provides default type
					targetsAttr.NestedObject.Attributes["type"] = attr
				}
				if attr, ok := targetsAttr.NestedObject.Attributes["value"].(schema.StringAttribute); ok { //nolint:clearableattr // API provides default value
					targetsAttr.NestedObject.Attributes["value"] = attr
				}
				splitsAttr.NestedObject.Attributes["targets"] = targetsAttr
			}
			configAttr.Attributes["splits"] = splitsAttr
		}

		if attr, ok := configAttr.Attributes["include_promotional_credits"].(schema.BoolAttribute); ok { //nolint:clearableattr // API defaults to false
			configAttr.Attributes["include_promotional_credits"] = attr
		}
		if attr, ok := configAttr.Attributes["layout"].(schema.StringAttribute); ok { //nolint:clearableattr // API defaults layout
			configAttr.Attributes["layout"] = attr
		}

		// Nested dimensions — id field.
		if dimsAttr, ok := configAttr.Attributes["dimensions"].(schema.ListNestedAttribute); ok {
			if attr, ok := dimsAttr.NestedObject.Attributes["id"].(schema.StringAttribute); ok { //nolint:clearableattr // API-assigned dimension ID
				dimsAttr.NestedObject.Attributes["id"] = attr
			}
			configAttr.Attributes["dimensions"] = dimsAttr
		}

		// Nested metric (top-level config.metric).
		if metricAttr, ok := configAttr.Attributes["metric"].(schema.SingleNestedAttribute); ok {
			if attr, ok := metricAttr.Attributes["type"].(schema.StringAttribute); ok { //nolint:clearableattr // API provides default type
				metricAttr.Attributes["type"] = attr
			}
			if attr, ok := metricAttr.Attributes["value"].(schema.StringAttribute); ok { //nolint:clearableattr // API provides default value
				metricAttr.Attributes["value"] = attr
			}
			configAttr.Attributes["metric"] = metricAttr
		}

		// Nested splits — include_origin.
		if splitsAttr, ok := configAttr.Attributes["splits"].(schema.ListNestedAttribute); ok {
			if attr, ok := splitsAttr.NestedObject.Attributes["include_origin"].(schema.BoolAttribute); ok { //nolint:clearableattr // API defaults to false
				splitsAttr.NestedObject.Attributes["include_origin"] = attr
			}
			configAttr.Attributes["splits"] = splitsAttr
		}

		// Nested secondary_time_range.
		if strAttr, ok := configAttr.Attributes["secondary_time_range"].(schema.SingleNestedAttribute); ok {
			if attr, ok := strAttr.Attributes["unit"].(schema.StringAttribute); ok { //nolint:clearableattr // API defaults unit
				strAttr.Attributes["unit"] = attr
			}
			if attr, ok := strAttr.Attributes["amount"].(schema.Int64Attribute); ok { //nolint:clearableattr // API defaults amount
				strAttr.Attributes["amount"] = attr
			}
			if attr, ok := strAttr.Attributes["include_current"].(schema.BoolAttribute); ok { //nolint:clearableattr // API defaults to false
				strAttr.Attributes["include_current"] = attr
			}
			if ctrAttr, ok := strAttr.Attributes["custom_time_range"].(schema.SingleNestedAttribute); ok {
				if attr, ok := ctrAttr.Attributes["from"].(schema.StringAttribute); ok { //nolint:clearableattr // API defaults from date
					ctrAttr.Attributes["from"] = attr
				}
				if attr, ok := ctrAttr.Attributes["to"].(schema.StringAttribute); ok { //nolint:clearableattr // API defaults to date
					ctrAttr.Attributes["to"] = attr
				}
				strAttr.Attributes["custom_time_range"] = ctrAttr
			}
			configAttr.Attributes["secondary_time_range"] = strAttr
		}

		s.Attributes["config"] = configAttr
	}

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
