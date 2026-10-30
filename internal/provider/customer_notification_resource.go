package provider

import (
	"context"
	"fmt"
	"uuid"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_customer_notification"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

type customerNotificationResource struct {
	client *models.ClientWithResponses
}

// Ensure the implementation satisfies expected interfaces.
var (
	_ resource.Resource                     = (*customerNotificationResource)(nil)
	_ resource.ResourceWithConfigure        = (*customerNotificationResource)(nil)
	_ resource.ResourceWithImportState      = (*customerNotificationResource)(nil)
	_ resource.ResourceWithConfigValidators = (*customerNotificationResource)(nil)
)

// NewCustomerNotificationResource creates a new customer notification resource instance.
func NewCustomerNotificationResource() resource.Resource {
	return &customerNotificationResource{}
}

// Configure adds the provider configured client to the resource.
func (r *customerNotificationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *customerNotificationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_customer_notification"
}

// ImportState imports a notification by its ID.
func (r *customerNotificationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *customerNotificationResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		customerNotificationSettingsValidator{},
		customerNotificationDestinationsValidator{},
	}
}

func (r *customerNotificationResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := resource_customer_notification.CustomerNotificationResourceSchema(ctx)

	// The API trims the name, so reject padding instead of drifting from it.
	if attr, ok := s.Attributes["name"].(schema.StringAttribute); ok {
		attr.Validators = append(attr.Validators, customerNotificationTrimmedValidator{})
		s.Attributes["name"] = attr
	}

	// Add UseStateForUnknown to stable Computed fields so they don't show as
	// "(known after apply)" on every plan that modifies the resource. update_time
	// and etag are intentionally excluded: the API changes them on every update.
	if attr, ok := s.Attributes["id"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["id"] = attr
	}
	if attr, ok := s.Attributes["create_time"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["create_time"] = attr
	}
	if destinations, ok := s.Attributes["destinations"].(schema.SingleNestedAttribute); ok {
		s.Attributes["destinations"] = customizeCustomerNotificationDestinationsSchema(destinations)
	}

	// The generated descriptions are written for API clients (dry runs,
	// If-Match, camelCase names); describe the Terraform behavior instead.
	setCostAnomalyDescriptions(s.Attributes, map[string]string{
		"id":           "The ID of the notification.",
		"name":         "Display name of the notification (1–200 characters).",
		"create_time":  "When the notification was created (RFC 3339, UTC).",
		"update_time":  "When the notification was last changed (RFC 3339, UTC).",
		"etag":         "Current ETag of the notification. The provider sends it with updates and deletes to detect changes made outside Terraform.",
		"settings":     "Notification settings. Only cost-anomaly settings are supported.",
		"destinations": "Where the notification is delivered. All three lists are required; use `[]` for an unused destination type. At least one list must be non-empty.",
	})

	if settings, ok := s.Attributes["settings"].(schema.SingleNestedAttribute); ok {
		if costAnomalies, ok := settings.Attributes["cost_anomalies"].(schema.SingleNestedAttribute); ok {
			if minExcessCost, ok := costAnomalies.Attributes["min_excess_cost"].(schema.SingleNestedAttribute); ok {
				appendCostAnomalyMoneyValidators(minExcessCost.Attributes, false)
				setCostAnomalyDescriptions(minExcessCost.Attributes, map[string]string{
					"amount": costAnomalyAmountDescription + " Must be between `0` and `1000000000`.",
				})
				costAnomalies.Attributes["min_excess_cost"] = minExcessCost
			}

			// repeat and everything under it is user-authored: the API never
			// fills it in, and omits it when unset. Plain Optional (not
			// Computed) lets removal from config plan a clear (sent as null).
			if repeat, ok := costAnomalies.Attributes["repeat"].(schema.SingleNestedAttribute); ok {
				repeat.Computed = false
				if everyHours, ok := repeat.Attributes["every_hours"].(schema.Int64Attribute); ok {
					everyHours.Computed = false
					repeat.Attributes["every_hours"] = everyHours
				}
				if whenCostGrows, ok := repeat.Attributes["when_cost_grows"].(schema.SingleNestedAttribute); ok {
					whenCostGrows.Computed = false
					if match, ok := whenCostGrows.Attributes["match"].(schema.StringAttribute); ok {
						match.Computed = false
						whenCostGrows.Attributes["match"] = match
					}
					if percentIncrease, ok := whenCostGrows.Attributes["percent_increase"].(schema.Int64Attribute); ok {
						percentIncrease.Computed = false
						whenCostGrows.Attributes["percent_increase"] = percentIncrease
					}
					if withinHours, ok := whenCostGrows.Attributes["within_hours"].(schema.Int64Attribute); ok {
						withinHours.Computed = false
						whenCostGrows.Attributes["within_hours"] = withinHours
					}
					if minIncrease, ok := whenCostGrows.Attributes["min_increase"].(schema.SingleNestedAttribute); ok {
						minIncrease.Computed = false
						appendCostAnomalyMoneyValidators(minIncrease.Attributes, true)
						setCostAnomalyDescriptions(minIncrease.Attributes, map[string]string{
							"amount": costAnomalyAmountDescription + " Must be between `1` and `1000000000`.",
						})
						whenCostGrows.Attributes["min_increase"] = minIncrease
					}
					setCostAnomalyDescriptions(whenCostGrows.Attributes, map[string]string{
						"match":            "How to combine `percent_increase` and `min_increase`: `or` resends when either is reached, `and` when both are. Required when both are set.",
						"percent_increase": "Minimum cost increase in percent (`10` means 10%) that triggers a resend.",
						"min_increase":     "Minimum cost increase that triggers a resend.",
						"within_hours":     "Only count growth within this many hours (1–24). Omit it to turn the time window off.",
					})
					repeat.Attributes["when_cost_grows"] = whenCostGrows
				}
				setCostAnomalyDescriptions(repeat.Attributes, map[string]string{
					"when_cost_grows": "Resend when the anomaly's cost grows. Set `percent_increase`, `min_increase`, or both.",
				})
				costAnomalies.Attributes["repeat"] = repeat
			}

			if allocations, ok := costAnomalies.Attributes["allocations"].(schema.SingleNestedAttribute); ok {
				setCostAnomalyDescriptions(allocations.Attributes, map[string]string{
					"ids": "IDs of custom single allocations, kept in the configured order. Must be `[]` when `mode` is `all`, and non-empty without duplicates for `include` and `exclude`. Preset and group allocations are rejected.",
				})
				costAnomalies.Attributes["allocations"] = allocations
			}

			setCostAnomalyDescriptions(costAnomalies.Attributes, map[string]string{
				"min_severity":    "Minimum anomaly severity: `warning` notifies about warning and critical anomalies, `critical` only about critical ones.",
				"allocations":     "Allocations whose anomalies trigger the notification.",
				"min_excess_cost": "Minimum excess cost of an anomaly (not total spend) that triggers the notification.",
				"repeat":          "Resend controls while an anomaly is active. Set `every_hours`, `when_cost_grows`, or both; remove the block to turn resending off.",
			})
			settings.Attributes["cost_anomalies"] = costAnomalies
			setCostAnomalyDescriptions(settings.Attributes, map[string]string{
				"cost_anomalies": "Cost-anomaly notification settings.",
			})
		}
		s.Attributes["settings"] = settings
	}

	s.Description = "Manages an organization-wide notification. Cost-anomaly notifications are supported.\n\n" +
		"Slack channel IDs and Microsoft Teams conversation IDs are supplied by you; the API has no lookup for them yet. " +
		"A channel that is deleted, archived or disconnected later stays in `destinations` without an error, so a listed channel does not guarantee delivery.\n\n" +
		"~> **Note:** Managing notifications requires the `settings` (Manage settings) permission."
	s.MarkdownDescription = s.Description

	s.Attributes["timeouts"] = timeouts.Attributes(ctx, timeouts.Opts{
		Create: true,
		Read:   true,
		Update: true,
		Delete: true,
	})

	resp.Schema = s
}

func (r *customerNotificationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan customerNotificationResourceModel

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

	params := &models.CreateCustomerNotificationParams{
		IdempotencyKey: uuid.NewV4().String(),
	}

	createResp, err := r.client.CreateCustomerNotificationWithResponse(ctx, params, apiReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Customer Notification",
			"Could not create customer notification, unexpected error: "+err.Error(),
		)
		return
	}

	if createResp.StatusCode() != 201 {
		resp.Diagnostics.AddError(
			"Error Creating Customer Notification",
			fmt.Sprintf("Could not create customer notification, status: %d, body: %s", createResp.StatusCode(), string(createResp.Body)),
		)
		return
	}

	if createResp.JSON201 == nil {
		resp.Diagnostics.AddError(
			"Error Creating Customer Notification",
			"Received empty response body for created customer notification",
		)
		return
	}

	// Plan-first state pattern: overlay Computed-only and resolved fields from API response.
	resp.Diagnostics.Append(overlayCustomerNotificationComputedFields(ctx, createResp.JSON201, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *customerNotificationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state customerNotificationResourceModel

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

func (r *customerNotificationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan customerNotificationResourceModel

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
	var state customerNotificationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq, diags := plan.toUpdateRequest(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := &models.UpdateCustomerNotificationParams{
		IfMatch:        state.Etag.ValueString(),
		IdempotencyKey: uuid.NewV4().String(),
	}

	updateResp, err := r.client.UpdateCustomerNotificationWithApplicationMergePatchPlusJSONBodyWithResponse(ctx, state.Id.ValueString(), params, apiReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Customer Notification",
			"Could not update customer notification, unexpected error: "+err.Error(),
		)
		return
	}

	if updateResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Updating Customer Notification",
			fmt.Sprintf("Could not update customer notification, status: %d, body: %s", updateResp.StatusCode(), string(updateResp.Body)),
		)
		return
	}

	if updateResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Updating Customer Notification",
			"Received empty response body for updated customer notification",
		)
		return
	}

	// Plan-first state pattern: overlay Computed-only and resolved fields from API response.
	resp.Diagnostics.Append(overlayCustomerNotificationComputedFields(ctx, updateResp.JSON200, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *customerNotificationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state customerNotificationResourceModel

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
		// If state lacks ETag, fetch it from API
		current, found, diags := r.currentEtag(ctx, id)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() || !found {
			return
		}
		etag = current
	}

	params := &models.DeleteCustomerNotificationParams{
		IfMatch:        etag,
		IdempotencyKey: uuid.NewV4().String(),
	}

	deleteResp, err := r.client.DeleteCustomerNotificationWithResponse(ctx, id, params)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Customer Notification",
			"Could not delete customer notification, unexpected error: "+err.Error(),
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
		deleteResp, err = r.client.DeleteCustomerNotificationWithResponse(ctx, id, params)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Deleting Customer Notification",
				"Could not delete customer notification on retry: "+err.Error(),
			)
			return
		}
	}

	// Treat 404 as success - resource is already gone (deleted outside Terraform)
	if deleteResp.StatusCode() != 200 && deleteResp.StatusCode() != 204 && deleteResp.StatusCode() != 404 {
		resp.Diagnostics.AddError(
			"Error Deleting Customer Notification",
			fmt.Sprintf("Could not delete customer notification, status: %d, body: %s", deleteResp.StatusCode(), string(deleteResp.Body)),
		)
	}
}

// currentEtag fetches the notification's current ETag. found is false when the
// notification no longer exists.
func (r *customerNotificationResource) currentEtag(ctx context.Context, id string) (etag string, found bool, diags diag.Diagnostics) {
	getResp, err := r.client.GetCustomerNotificationWithResponse(ctx, id)
	if err != nil {
		diags.AddError(
			"Error Deleting Customer Notification",
			"Could not fetch customer notification to get ETag: "+err.Error(),
		)
		return "", false, diags
	}
	if getResp.StatusCode() == 404 {
		return "", false, diags
	}
	if getResp.StatusCode() != 200 || getResp.JSON200 == nil {
		diags.AddError(
			"Error Deleting Customer Notification",
			fmt.Sprintf("Could not fetch customer notification to get ETag, status: %d, body: %s", getResp.StatusCode(), string(getResp.Body)),
		)
		return "", false, diags
	}
	if e := nullableToPointer(getResp.JSON200.Etag); e != nil {
		etag = *e
	}
	return etag, true, diags
}

// customizeCustomerNotificationDestinationsSchema describes destinations for
// Terraform users and rejects values the API would trim.
func customizeCustomerNotificationDestinationsSchema(destinations schema.SingleNestedAttribute) schema.SingleNestedAttribute {
	if emails, ok := destinations.Attributes["emails"].(schema.ListAttribute); ok {
		emails.Validators = append(emails.Validators, listvalidator.ValueStringsAre(
			customerNotificationTrimmedValidator{},
		))
		destinations.Attributes["emails"] = emails
	}
	if slack, ok := destinations.Attributes["slack_channels"].(schema.ListAttribute); ok {
		slack.Validators = append(slack.Validators, listvalidator.ValueStringsAre(customerNotificationTrimmedValidator{}))
		destinations.Attributes["slack_channels"] = slack
	}
	if teams, ok := destinations.Attributes["teams_channels"].(schema.ListAttribute); ok {
		teams.Validators = append(teams.Validators, listvalidator.ValueStringsAre(customerNotificationTrimmedValidator{}))
		destinations.Attributes["teams_channels"] = teams
	}

	setCostAnomalyDescriptions(destinations.Attributes, map[string]string{
		"emails":         "Email addresses that receive the notification, in the configured order. The API stores them in lowercase; state keeps the configured spelling while the address is unchanged. Use `[]` when email is unused.",
		"slack_channels": "Slack channel IDs (such as `C0123ABCD`), in the configured order. Use `[]` when Slack is unused.",
		"teams_channels": "Microsoft Teams conversation IDs (such as `19:abc@thread.tacv2`), in the configured order. Use `[]` when Teams is unused. Requires a Teams entitlement.",
	})
	return destinations
}
