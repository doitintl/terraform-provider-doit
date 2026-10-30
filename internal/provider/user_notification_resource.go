package provider

import (
	"context"
	"fmt"
	"strings"
	"uuid"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_user_notification"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

type userNotificationResource struct {
	client *models.ClientWithResponses
}

// Ensure the implementation satisfies expected interfaces.
var (
	_ resource.Resource                     = (*userNotificationResource)(nil)
	_ resource.ResourceWithConfigure        = (*userNotificationResource)(nil)
	_ resource.ResourceWithImportState      = (*userNotificationResource)(nil)
	_ resource.ResourceWithConfigValidators = (*userNotificationResource)(nil)
)

// NewUserNotificationResource creates a new user notification resource instance.
func NewUserNotificationResource() resource.Resource {
	return &userNotificationResource{}
}

// Configure adds the provider configured client to the resource.
func (r *userNotificationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *userNotificationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_notification"
}

// ImportState imports a notification by its composite ID `<user_id>/<id>`.
// The notification ID alone cannot address the resource, because the API nests
// notifications under their user.
func (r *userNotificationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected import ID in the format 'user_id/notification_id', got: %s", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

func (r *userNotificationResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		userNotificationSettingsValidator{},
	}
}

func (r *userNotificationResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := resource_user_notification.UserNotificationResourceSchema(ctx)

	// Notifications are nested under their user, so moving one replaces it.
	if attr, ok := s.Attributes["user_id"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.RequiresReplace())
		attr.Description = "The ID of the user the notification belongs to. Changing it replaces the notification."
		attr.MarkdownDescription = attr.Description
		s.Attributes["user_id"] = attr
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
	if attr, ok := s.Attributes["destinations"].(schema.ListNestedAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, listplanmodifier.UseStateForUnknown())
		s.Attributes["destinations"] = attr
	}

	// The generated descriptions are written for API clients (dry runs,
	// If-Match, camelCase names); describe the Terraform behavior instead.
	setCostAnomalyDescriptions(s.Attributes, map[string]string{
		"id":          "The ID of the notification.",
		"name":        "Display name of the notification (1–200 characters).",
		"create_time": "When the notification was created (RFC 3339, UTC).",
		"update_time": "When the notification was last changed (RFC 3339, UTC).",
		"etag":        "Current ETag of the notification. The provider sends it with updates and deletes to detect changes made outside Terraform.",
		"settings":    "Notification settings. Only cost-anomaly settings are supported.",
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

	s.Description = "Manages a personal notification of a user. Cost-anomaly notifications are supported.\n\n" +
		"The notification is always delivered by email to the user's own address, which the API fills in as the only entry in `destinations`.\n\n" +
		"~> **Note:** Managing notifications requires the `usersManager` permission."
	s.MarkdownDescription = s.Description

	s.Attributes["timeouts"] = timeouts.Attributes(ctx, timeouts.Opts{
		Create: true,
		Read:   true,
		Update: true,
		Delete: true,
	})

	resp.Schema = s
}

func (r *userNotificationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan userNotificationResourceModel

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

	params := &models.CreateUserNotificationParams{
		IdempotencyKey: uuid.NewV4().String(),
	}

	createResp, err := r.client.CreateUserNotificationWithResponse(ctx, plan.UserId.ValueString(), params, apiReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating User Notification",
			"Could not create user notification, unexpected error: "+err.Error(),
		)
		return
	}

	if createResp.StatusCode() != 201 {
		resp.Diagnostics.AddError(
			"Error Creating User Notification",
			fmt.Sprintf("Could not create user notification, status: %d, body: %s", createResp.StatusCode(), string(createResp.Body)),
		)
		return
	}

	if createResp.JSON201 == nil {
		resp.Diagnostics.AddError(
			"Error Creating User Notification",
			"Received empty response body for created user notification",
		)
		return
	}

	// Plan-first state pattern: overlay Computed-only and resolved fields from API response.
	resp.Diagnostics.Append(overlayUserNotificationComputedFields(ctx, createResp.JSON201, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *userNotificationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state userNotificationResourceModel

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

func (r *userNotificationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan userNotificationResourceModel

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
	var state userNotificationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq, diags := plan.toUpdateRequest(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := &models.UpdateUserNotificationParams{
		IfMatch:        state.Etag.ValueString(),
		IdempotencyKey: uuid.NewV4().String(),
	}

	updateResp, err := r.client.UpdateUserNotificationWithApplicationMergePatchPlusJSONBodyWithResponse(ctx, state.UserId.ValueString(), state.Id.ValueString(), params, apiReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating User Notification",
			"Could not update user notification, unexpected error: "+err.Error(),
		)
		return
	}

	if updateResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Updating User Notification",
			fmt.Sprintf("Could not update user notification, status: %d, body: %s", updateResp.StatusCode(), string(updateResp.Body)),
		)
		return
	}

	if updateResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Updating User Notification",
			"Received empty response body for updated user notification",
		)
		return
	}

	// Plan-first state pattern: overlay Computed-only and resolved fields from API response.
	resp.Diagnostics.Append(overlayUserNotificationComputedFields(ctx, updateResp.JSON200, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *userNotificationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state userNotificationResourceModel

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

	userID := state.UserId.ValueString()
	id := state.Id.ValueString()

	etag := state.Etag.ValueString()
	if etag == "" {
		// If state lacks ETag, fetch it from API
		current, found, diags := r.currentEtag(ctx, userID, id)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() || !found {
			return
		}
		etag = current
	}

	params := &models.DeleteUserNotificationParams{
		IfMatch:        etag,
		IdempotencyKey: uuid.NewV4().String(),
	}

	deleteResp, err := r.client.DeleteUserNotificationWithResponse(ctx, userID, id, params)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting User Notification",
			"Could not delete user notification, unexpected error: "+err.Error(),
		)
		return
	}

	// If 412 (stale ETag), retry once with a fresh ETag
	if deleteResp.StatusCode() == 412 {
		current, found, diags := r.currentEtag(ctx, userID, id)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() || !found {
			return
		}
		params.IfMatch = current
		params.IdempotencyKey = uuid.NewV4().String()
		deleteResp, err = r.client.DeleteUserNotificationWithResponse(ctx, userID, id, params)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Deleting User Notification",
				"Could not delete user notification on retry: "+err.Error(),
			)
			return
		}
	}

	// Treat 404 as success - resource is already gone (deleted outside Terraform)
	if deleteResp.StatusCode() != 200 && deleteResp.StatusCode() != 204 && deleteResp.StatusCode() != 404 {
		resp.Diagnostics.AddError(
			"Error Deleting User Notification",
			fmt.Sprintf("Could not delete user notification, status: %d, body: %s", deleteResp.StatusCode(), string(deleteResp.Body)),
		)
	}
}

// currentEtag fetches the notification's current ETag. found is false when the
// notification no longer exists.
func (r *userNotificationResource) currentEtag(ctx context.Context, userID, id string) (etag string, found bool, diags diag.Diagnostics) {
	getResp, err := r.client.GetUserNotificationWithResponse(ctx, userID, id)
	if err != nil {
		diags.AddError(
			"Error Deleting User Notification",
			"Could not fetch user notification to get ETag: "+err.Error(),
		)
		return "", false, diags
	}
	if getResp.StatusCode() == 404 {
		return "", false, diags
	}
	if getResp.StatusCode() != 200 || getResp.JSON200 == nil {
		diags.AddError(
			"Error Deleting User Notification",
			fmt.Sprintf("Could not fetch user notification to get ETag, status: %d, body: %s", getResp.StatusCode(), string(getResp.Body)),
		)
		return "", false, diags
	}
	if e := nullableToPointer(getResp.JSON200.Etag); e != nil {
		etag = *e
	}
	return etag, true, diags
}
