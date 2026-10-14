package provider

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"uuid"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_user_notification"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

type userNotificationResource struct {
	client *models.ClientWithResponses
}

var (
	_ resource.Resource                = (*userNotificationResource)(nil)
	_ resource.ResourceWithConfigure   = (*userNotificationResource)(nil)
	_ resource.ResourceWithImportState = (*userNotificationResource)(nil)
	_ resource.ResourceWithModifyPlan  = (*userNotificationResource)(nil)
)

func NewUserNotificationResource() resource.Resource {
	return &userNotificationResource{}
}

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
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("notification_id"), parts[1])...)
}

func (r *userNotificationResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := resource_user_notification.UserNotificationResourceSchema(ctx)

	if attr, ok := s.Attributes["user_id"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.RequiresReplace())
		s.Attributes["user_id"] = attr
	}

	for _, name := range []string{"id", "notification_id", "create_time"} {
		if attr, ok := s.Attributes[name].(schema.StringAttribute); ok {
			attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
			// notification_id is a path-param echo of id; keep it computed-only.
			if name == "notification_id" {
				attr.Optional = false
				attr.Required = false
				attr.Computed = true
			}
			s.Attributes[name] = attr
		}
	}

	if attr, ok := s.Attributes["settings"].(schema.SingleNestedAttribute); ok {
		if ca, ok := attr.Attributes["cost_anomalies"].(schema.SingleNestedAttribute); ok {
			if repeat, ok := ca.Attributes["repeat"].(schema.SingleNestedAttribute); ok {
				repeat.PlanModifiers = append(repeat.PlanModifiers, useNullForUnknownRepeatWhenConfigNull())
				ca.Attributes["repeat"] = repeat
			}
			attr.Attributes["cost_anomalies"] = ca
		}
		s.Attributes["settings"] = attr
	}

	s.Attributes["timeouts"] = timeouts.Attributes(ctx, timeouts.Opts{
		Create: true,
		Read:   true,
		Update: true,
		Delete: true,
	})

	resp.Schema = s
}

func useNullForUnknownRepeatWhenConfigNull() planmodifier.Object {
	return useNullForUnknownRepeatModifier{}
}

type useNullForUnknownRepeatModifier struct{}

func (m useNullForUnknownRepeatModifier) Description(_ context.Context) string {
	return "Proposes null when config.repeat is null so that omitting the block clears it."
}

func (m useNullForUnknownRepeatModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m useNullForUnknownRepeatModifier) PlanModifyObject(_ context.Context, req planmodifier.ObjectRequest, resp *planmodifier.ObjectResponse) {
	if req.ConfigValue.IsNull() {
		resp.PlanValue = req.ConfigValue
	}
}

func (r *userNotificationResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}

	var plan userNotificationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	normalized, diags := normalizeSettingsMoneyInPlace(ctx, plan.Settings)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.Settings = normalized

	resp.Diagnostics.Append(resp.Plan.Set(ctx, &plan)...)
}

func (r *userNotificationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan userNotificationResourceModel

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

	resp.Diagnostics.Append(overlayUserNotificationComputedFields(ctx, createResp.JSON201, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *userNotificationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state userNotificationResourceModel

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

	diags = r.populateState(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if state.Id.IsNull() {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *userNotificationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan userNotificationResourceModel

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

	var state userNotificationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body, diags := plan.updateBodyJSON(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := &models.UpdateUserNotificationParams{
		IfMatch:        state.Etag.ValueString(),
		IdempotencyKey: uuid.NewV4().String(),
	}

	updateResp, err := r.client.UpdateUserNotificationWithBodyWithResponse(
		ctx,
		plan.UserId.ValueString(),
		state.Id.ValueString(),
		params,
		"application/merge-patch+json",
		bytes.NewReader(body),
	)
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

	resp.Diagnostics.Append(overlayUserNotificationComputedFields(ctx, updateResp.JSON200, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *userNotificationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state userNotificationResourceModel

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
	notificationID := state.Id.ValueString()
	etag := state.Etag.ValueString()

	if etag == "" {
		getResp, err := r.client.GetUserNotificationWithResponse(ctx, userID, notificationID)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Deleting User Notification",
				"Could not fetch user notification to get ETag: "+err.Error(),
			)
			return
		}
		if getResp.StatusCode() == 404 {
			return
		}
		if getResp.StatusCode() != 200 || getResp.JSON200 == nil {
			resp.Diagnostics.AddError(
				"Error Deleting User Notification",
				fmt.Sprintf("Could not fetch user notification to get ETag, status: %d, body: %s", getResp.StatusCode(), string(getResp.Body)),
			)
			return
		}
		if getResp.JSON200.Etag.IsSpecified() && !getResp.JSON200.Etag.IsNull() {
			etag = getResp.JSON200.Etag.MustGet()
		}
	}

	params := &models.DeleteUserNotificationParams{
		IfMatch:        etag,
		IdempotencyKey: uuid.NewV4().String(),
	}

	deleteResp, err := r.client.DeleteUserNotificationWithResponse(ctx, userID, notificationID, params)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting User Notification",
			"Could not delete user notification, unexpected error: "+err.Error(),
		)
		return
	}

	if deleteResp.StatusCode() == 412 {
		getResp, err := r.client.GetUserNotificationWithResponse(ctx, userID, notificationID)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Deleting User Notification",
				"Could not refetch user notification after 412: "+err.Error(),
			)
			return
		}
		if getResp.StatusCode() == 404 {
			return
		}
		if getResp.StatusCode() == 200 && getResp.JSON200 != nil && getResp.JSON200.Etag.IsSpecified() && !getResp.JSON200.Etag.IsNull() {
			params.IfMatch = getResp.JSON200.Etag.MustGet()
			params.IdempotencyKey = uuid.NewV4().String()
			deleteResp, err = r.client.DeleteUserNotificationWithResponse(ctx, userID, notificationID, params)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error Deleting User Notification",
					"Could not delete user notification on retry: "+err.Error(),
				)
				return
			}
		}
	}

	if deleteResp.StatusCode() != 200 && deleteResp.StatusCode() != 204 && deleteResp.StatusCode() != 404 {
		resp.Diagnostics.AddError(
			"Error Deleting User Notification",
			fmt.Sprintf("Could not delete user notification, status: %d, body: %s", deleteResp.StatusCode(), string(deleteResp.Body)),
		)
		return
	}
}
