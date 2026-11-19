package provider

import (
	"context"
	"fmt"
	"uuid"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_custom_metric"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

type customMetricResource struct {
	client *models.ClientWithResponses
}

// Ensure the implementation satisfies expected interfaces.
var (
	_ resource.Resource                = (*customMetricResource)(nil)
	_ resource.ResourceWithConfigure   = (*customMetricResource)(nil)
	_ resource.ResourceWithImportState = (*customMetricResource)(nil)
	_ resource.ResourceWithModifyPlan  = (*customMetricResource)(nil)
)

// NewCustomMetricResource creates a new custom metric resource instance.
func NewCustomMetricResource() resource.Resource {
	return &customMetricResource{}
}

// Configure adds the provider configured client to the resource.
func (r *customMetricResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *customMetricResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_custom_metric"
}

// ImportState imports a custom metric by its ID.
func (r *customMetricResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// ModifyPlan marks etag and update_time unknown on updates that only clear
// decimals; the framework leaves them known in that case.
func (r *customMetricResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	planVolatileAttributesUnknownOnUpdate(ctx, req, resp, path.Root("etag"), path.Root("update_time"))
}

func (r *customMetricResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := resource_custom_metric.CustomMetricResourceSchema(ctx)

	// Add UseStateForUnknown to stable Computed fields so they don't show as
	// "(known after apply)" on every plan that modifies the resource. etag and
	// update_time are intentionally excluded: the API changes them on updates.
	if attr, ok := s.Attributes["id"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["id"] = attr
	}
	if attr, ok := s.Attributes["owner"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["owner"] = attr
	}
	if attr, ok := s.Attributes["create_time"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["create_time"] = attr
	}

	// decimals is user-authored and the API never fills it in: omitting the
	// block removes the display precision (the API clears it with null).
	if attr, ok := s.Attributes["decimals"].(schema.SingleNestedAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, useNullForUnknownCustomMetricDecimalsWhenConfigNull())
		attr.Validators = append(attr.Validators, customMetricDecimalsRangeValidator{})
		s.Attributes["decimals"] = attr
	}

	if attr, ok := s.Attributes["variables"].(schema.ListNestedAttribute); ok {
		if letter, ok := attr.NestedObject.Attributes["letter"].(schema.StringAttribute); ok {
			letter.PlanModifiers = append(letter.PlanModifiers, customMetricVariableLetterFromPosition())
			attr.NestedObject.Attributes["letter"] = letter
		}
		if allocationID, ok := attr.NestedObject.Attributes["allocation_id"].(schema.StringAttribute); ok {
			allocationID.Description = "ID of the single allocation the variable measures, such as `doit_allocation.example.id`. A group allocation is rejected, and a preset allocation needs the preset allocations entitlement."
			allocationID.MarkdownDescription = allocationID.Description
			attr.NestedObject.Attributes["allocation_id"] = allocationID
		}
		if metric, ok := attr.NestedObject.Attributes["metric"].(schema.SingleNestedAttribute); ok {
			if value, ok := metric.Attributes["value"].(schema.StringAttribute); ok {
				value.Description = "For `basic`: `cost`, `usage` or `savings`. For `extended`: the key of an extended metric of the `billing` or `billing-datahub` data source, the same key a report uses with `type = \"extended\"`. The API rejects any other value."
				value.MarkdownDescription = value.Description
				metric.Attributes["value"] = value
			}
			metric.Description = "The metric the variable measures. A custom metric cannot be a variable of another custom metric."
			metric.MarkdownDescription = metric.Description
			attr.NestedObject.Attributes["metric"] = metric
		}
		s.Attributes["variables"] = attr
	}

	// The generated descriptions are written for API clients (If-Match,
	// camelCase names); describe the Terraform behavior instead.
	setCostAnomalyDescriptions(s.Attributes, map[string]string{
		"id":          "The ID of the custom metric. Reports use it as `metric = { type = \"custom\", value = <id> }`.",
		"etag":        "Current ETag of the custom metric. The provider sends it with updates and deletes to detect changes made outside Terraform.",
		"owner":       "Email address of the user or service account that created the custom metric. Only the owner can delete it.",
		"name":        "Name of the custom metric. It must contain a character other than a space and is stored exactly as written.",
		"description": "Description of the custom metric. Defaults to an empty string.",
		"formula":     "Arithmetic over the variable letters: `A` for the first entry of `variables`, `B` for the second, up to `E`. Allowed are those letters, numbers such as `100` or `0.5`, the operators `+`, `-`, `*` and `/`, parentheses and spaces. The formula is stored exactly as written (`A / B` stays `A / B`). The API rejects a formula that does not parse, divides by a literal `0`, or uses a letter without a variable.",
		"unit_type":   "How reports show the value: a plain `number`, a `percentage`, or a `currency` amount in the customer's currency. Defaults to `number`.",
		"decimals":    "Number of decimal places reports show, from `min` to `max`. Omit it to use the reports' default formatting.",
		"variables":   "The variables of the formula, 1 to 5. Their order sets their letters: the first is `A`, the second `B`, up to `E`. Reordering the list changes which variable a letter in `formula` refers to.",
	})

	s.Description = "Manages a custom metric: a formula over the costs, usage, savings or extended metrics of single allocations, such as cost per ride. Reports, alerts and allocation splits can use it as a metric.\n\n" +
		"~> **Note:** Custom metrics require the calculated metrics entitlement. A custom metric that a report, an alert or an allocation uses cannot be deleted; remove it from them first."
	s.MarkdownDescription = s.Description

	s.Attributes["timeouts"] = timeouts.Attributes(ctx, timeouts.Opts{
		Create: true,
		Read:   true,
		Update: true,
		Delete: true,
	})

	rejectNullCollectionElements(s.Attributes)
	resp.Schema = s
}

func (r *customMetricResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan customMetricResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
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

	apiReq, diags := plan.toCreateRequest(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := &models.CreateCustomMetricParams{
		IdempotencyKey: uuid.NewV4().String(),
	}

	createResp, err := r.client.CreateCustomMetricWithResponse(ctx, params, apiReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Custom Metric",
			"Could not create custom metric, unexpected error: "+err.Error(),
		)
		return
	}

	if createResp.StatusCode() != 201 {
		resp.Diagnostics.AddError(
			"Error Creating Custom Metric",
			fmt.Sprintf("Could not create custom metric, status: %d, body: %s", createResp.StatusCode(), string(createResp.Body)),
		)
		return
	}

	if createResp.JSON201 == nil {
		resp.Diagnostics.AddError(
			"Error Creating Custom Metric",
			"Received empty response body for created custom metric",
		)
		return
	}

	// Plan-first state pattern: overlay Computed-only and resolved fields from API response.
	resp.Diagnostics.Append(overlayCustomMetricComputedFields(ctx, createResp.JSON201, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *customMetricResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state customMetricResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
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

	resp.Diagnostics.Append(r.populateState(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Resource was deleted outside Terraform
	if state.Id.IsNull() {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *customMetricResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan customMetricResourceModel

	// Read Terraform plan data
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
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

	// Get the ID and ETag from the prior state
	var state customMetricResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq, diags := plan.toUpdateRequest(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := &models.UpdateCustomMetricParams{
		IfMatch:        state.Etag.ValueString(),
		IdempotencyKey: uuid.NewV4().String(),
	}

	updateResp, err := r.client.UpdateCustomMetricWithApplicationMergePatchPlusJSONBodyWithResponse(ctx, state.Id.ValueString(), params, apiReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Custom Metric",
			"Could not update custom metric, unexpected error: "+err.Error(),
		)
		return
	}

	if updateResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Updating Custom Metric",
			fmt.Sprintf("Could not update custom metric, status: %d, body: %s", updateResp.StatusCode(), string(updateResp.Body)),
		)
		return
	}

	if updateResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Updating Custom Metric",
			"Received empty response body for updated custom metric",
		)
		return
	}

	// Plan-first state pattern: overlay Computed-only and resolved fields from API response.
	resp.Diagnostics.Append(overlayCustomMetricComputedFields(ctx, updateResp.JSON200, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *customMetricResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state customMetricResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
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

	id := state.Id.ValueString()

	etag := state.Etag.ValueString()
	if etag == "" {
		// If state lacks ETag, fetch it from API. A metric that is already
		// gone is treated as deleted rather than sent without If-Match.
		current, found, diags := r.currentEtag(ctx, id)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() || !found {
			return
		}
		etag = current
	}

	params := &models.DeleteCustomMetricParams{
		IfMatch:        etag,
		IdempotencyKey: uuid.NewV4().String(),
	}

	deleteResp, err := r.client.DeleteCustomMetricWithResponse(ctx, id, params)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Custom Metric",
			"Could not delete custom metric, unexpected error: "+err.Error(),
		)
		return
	}

	// If 412 (stale ETag), retry once with a fresh ETag
	if deleteResp.StatusCode() == 412 {
		current, found, diags := r.currentEtag(ctx, id)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() || !found {
			return
		}
		params.IfMatch = current
		params.IdempotencyKey = uuid.NewV4().String()
		deleteResp, err = r.client.DeleteCustomMetricWithResponse(ctx, id, params)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Deleting Custom Metric",
				"Could not delete custom metric on retry: "+err.Error(),
			)
			return
		}
	}

	// The API returns 404 for a metric that is already deleted.
	if deleteResp.StatusCode() != 200 && deleteResp.StatusCode() != 204 && deleteResp.StatusCode() != 404 {
		resp.Diagnostics.AddError(
			"Error Deleting Custom Metric",
			fmt.Sprintf("Could not delete custom metric, status: %d, body: %s", deleteResp.StatusCode(), string(deleteResp.Body)),
		)
	}
}

// currentEtag fetches the custom metric's current ETag. found is false when the
// metric no longer exists.
func (r *customMetricResource) currentEtag(ctx context.Context, id string) (etag string, found bool, diags diag.Diagnostics) {
	getResp, err := r.client.GetCustomMetricWithResponse(ctx, id, &models.GetCustomMetricParams{})
	if err != nil {
		diags.AddError(
			"Error Deleting Custom Metric",
			"Could not fetch custom metric to get ETag: "+err.Error(),
		)
		return "", false, diags
	}
	if getResp.StatusCode() == 404 {
		return "", false, diags
	}
	if getResp.StatusCode() != 200 || getResp.JSON200 == nil {
		diags.AddError(
			"Error Deleting Custom Metric",
			fmt.Sprintf("Could not fetch custom metric to get ETag, status: %d, body: %s", getResp.StatusCode(), string(getResp.Body)),
		)
		return "", false, diags
	}
	if getResp.JSON200.Etag != nil {
		etag = *getResp.JSON200.Etag
	}
	return etag, true, diags
}
