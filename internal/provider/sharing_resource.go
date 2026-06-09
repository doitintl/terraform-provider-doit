package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_sharing"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type (
	sharingResource struct {
		client *models.ClientWithResponses
	}
	sharingResourceModel struct {
		resource_sharing.SharingModel
		Timeouts timeouts.Value `tfsdk:"timeouts"`
	}
)

// Ensure the implementation satisfies expected interfaces.
var (
	_ resource.Resource                     = (*sharingResource)(nil)
	_ resource.ResourceWithConfigure        = (*sharingResource)(nil)
	_ resource.ResourceWithImportState      = (*sharingResource)(nil)
	_ resource.ResourceWithConfigValidators = (*sharingResource)(nil)
)

// NewSharingResource creates a new resource sharing resource instance.
func NewSharingResource() resource.Resource {
	return &sharingResource{}
}

// Configure adds the provider configured client to the resource.
func (r *sharingResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *sharingResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sharing"
}

func (r *sharingResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := resource_sharing.SharingResourceSchema(ctx)

	// resource_type: make Required + RequiresReplace (codegen sets Optional+Computed for path params)
	if attr, ok := s.Attributes["resource_type"].(schema.StringAttribute); ok {
		attr.Required = true
		attr.Optional = false
		attr.Computed = false
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.RequiresReplace())
		s.Attributes["resource_type"] = attr
	}

	// resource_id: make Required + RequiresReplace
	if attr, ok := s.Attributes["resource_id"].(schema.StringAttribute); ok {
		attr.Required = true
		attr.Optional = false
		attr.Computed = false
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.RequiresReplace())
		s.Attributes["resource_id"] = attr
	}

	// permissions: make Required (not Optional+Computed — user must always specify permissions)
	if attr, ok := s.Attributes["permissions"].(schema.ListNestedAttribute); ok {
		attr.Required = true
		attr.Optional = false
		attr.Computed = false

		// user and role: make Required inside nested permissions
		// The codegen generates these as Optional+Computed because the response schema
		// includes them, but they are required inputs for each permission entry.
		if userAttr, uOk := attr.NestedObject.Attributes["user"].(schema.StringAttribute); uOk {
			userAttr.Required = true
			userAttr.Optional = false
			userAttr.Computed = false
			attr.NestedObject.Attributes["user"] = userAttr
		}
		if roleAttr, rOk := attr.NestedObject.Attributes["role"].(schema.StringAttribute); rOk {
			roleAttr.Required = true
			roleAttr.Optional = false
			roleAttr.Computed = false
			attr.NestedObject.Attributes["role"] = roleAttr
		}

		s.Attributes["permissions"] = attr
	}

	// UseStateForUnknown on stable Computed-only fields so they don't clutter plans
	if attr, ok := s.Attributes["id"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["id"] = attr
	}
	if attr, ok := s.Attributes["name"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["name"] = attr
	}
	if attr, ok := s.Attributes["description"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["description"] = attr
	}
	if attr, ok := s.Attributes["create_time"].(schema.Int64Attribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, int64planmodifier.UseStateForUnknown())
		s.Attributes["create_time"] = attr
	}
	if attr, ok := s.Attributes["update_time"].(schema.Int64Attribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, int64planmodifier.UseStateForUnknown())
		s.Attributes["update_time"] = attr
	}

	// public: allow clearing by setting null in config (workaround for framework issue #603),
	// and preserve the prior value when the user doesn't change it (avoids "known after apply" noise).
	// Order matters: useEmptyForUnknownWhenConfigNull must run first so that removing `public`
	// from config resolves to null, not to the prior state value.
	if attr, ok := s.Attributes["public"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers,
			useNullForUnknownStringWhenConfigNull(),
			stringplanmodifier.UseStateForUnknown(),
		)
		s.Attributes["public"] = attr
	}

	s.Attributes["timeouts"] = timeouts.Attributes(ctx, timeouts.Opts{
		Create: true,
		Read:   true,
		Update: true,
		Delete: true,
	})

	resp.Schema = s
}

func (r *sharingResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		sharingOwnerValidator{},
		sharingAllocationPublicValidator{},
	}
}

func (r *sharingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan sharingResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
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

	// Build API request from plan
	apiReq, diags := buildSharingRequest(ctx, &plan, plan.ResourceType.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resType := models.UpdateResourcePermissionParamsResourceType(plan.ResourceType.ValueString())
	resID := plan.ResourceId.ValueString()

	// Create (PUT) permissions via API
	putResp, err := r.client.UpdateResourcePermissionWithResponse(ctx, resType, resID, apiReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Resource Sharing",
			"Could not set resource permissions, unexpected error: "+err.Error(),
		)
		return
	}

	if putResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Creating Resource Sharing",
			fmt.Sprintf("Could not set resource permissions, status: %d, body: %s", putResp.StatusCode(), string(putResp.Body)),
		)
		return
	}

	if putResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Creating Resource Sharing",
			"Received empty response body",
		)
		return
	}

	// Plan-first state pattern: keep user values, overlay only Computed-only fields
	resp.Diagnostics.Append(overlaySharingComputedFields(ctx, putResp.JSON200, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *sharingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state sharingResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
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

	resp.Diagnostics.Append(r.populateState(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Handle externally deleted resource (populateState sets Id to null on 404)
	if state.Id.IsNull() {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *sharingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan sharingResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
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

	// Build API request from plan
	apiReq, diags := buildSharingRequest(ctx, &plan, plan.ResourceType.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resType := models.UpdateResourcePermissionParamsResourceType(plan.ResourceType.ValueString())
	resID := plan.ResourceId.ValueString()

	// Update (PUT) permissions via API
	putResp, err := r.client.UpdateResourcePermissionWithResponse(ctx, resType, resID, apiReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Resource Sharing",
			"Could not update resource permissions, unexpected error: "+err.Error(),
		)
		return
	}

	if putResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Updating Resource Sharing",
			fmt.Sprintf("Could not update resource permissions, status: %d, body: %s", putResp.StatusCode(), string(putResp.Body)),
		)
		return
	}

	if putResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Updating Resource Sharing",
			"Received empty response body",
		)
		return
	}

	// Plan-first state pattern: preserve user's plan values, overlay computed fields only
	resp.Diagnostics.Append(overlaySharingComputedFields(ctx, putResp.JSON200, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *sharingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state sharingResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
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

	resType := state.ResourceType.ValueString()
	resID := state.ResourceId.ValueString()

	// Step 1: GET current permissions to discover the current owner
	getResp, err := r.client.GetResourcePermissionWithResponse(
		ctx,
		models.GetResourcePermissionParamsResourceType(resType),
		resID,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Resource Sharing",
			"Could not read current resource permissions: "+err.Error(),
		)
		return
	}

	// Treat 404 as success — resource is already gone
	if getResp.StatusCode() == 404 {
		return
	}

	if getResp.StatusCode() != 200 || getResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Deleting Resource Sharing",
			fmt.Sprintf("Could not read current resource permissions, status: %d, body: %s", getResp.StatusCode(), string(getResp.Body)),
		)
		return
	}

	// Step 2: Find the current owner from the existing permissions
	var ownerPerm *models.ResourcePermission
	if getResp.JSON200.Permissions != nil {
		for _, p := range *getResp.JSON200.Permissions {
			if p.Role != nil && string(*p.Role) == "owner" {
				ownerPerm = &models.ResourcePermission{
					User: p.User,
					Role: p.Role,
				}
				break
			}
		}
	}

	if ownerPerm == nil {
		resp.Diagnostics.AddError(
			"Error Deleting Resource Sharing",
			"Could not find the current owner in the resource permissions. Cannot safely reset permissions.",
		)
		return
	}

	// Step 3: PUT with just the owner (strips all editors/viewers) and null public.
	// Allocations require the public field to be present (API returns 500 if omitted),
	// so we send an explicit empty string for that resource type.
	resetPerms := []models.ResourcePermission{*ownerPerm}
	resetReq := models.UpdateResourcePermissionJSONRequestBody{
		Permissions: &resetPerms,
	}
	if resType == "allocations" {
		resetReq.Public = new(models.UpdateResourcePermissionRequestBodyPublic(""))
	}

	putResp, err := r.client.UpdateResourcePermissionWithResponse(
		ctx,
		models.UpdateResourcePermissionParamsResourceType(resType),
		resID,
		resetReq,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Resource Sharing",
			"Could not reset resource permissions: "+err.Error(),
		)
		return
	}

	// Treat 404 as success — resource is already gone
	if putResp.StatusCode() != 200 && putResp.StatusCode() != 404 {
		resp.Diagnostics.AddError(
			"Error Deleting Resource Sharing",
			fmt.Sprintf("Could not reset resource permissions, status: %d, body: %s", putResp.StatusCode(), string(putResp.Body)),
		)
	}
}

func (r *sharingResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import ID format: {resourceType}/{resourceId}
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected import ID in the format 'resourceType/resourceId' (e.g. 'reports/abc123'), got: %s", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("resource_type"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("resource_id"), parts[1])...)
}

// populateState reads the current permissions from the API and populates the state model.
// On 404, sets Id to null to signal the caller to call RemoveResource().
func (r *sharingResource) populateState(ctx context.Context, state *sharingResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	resType := state.ResourceType.ValueString()
	resID := state.ResourceId.ValueString()

	getResp, err := r.client.GetResourcePermissionWithResponse(
		ctx,
		models.GetResourcePermissionParamsResourceType(resType),
		resID,
	)
	if err != nil {
		diags.AddError(
			"Error Reading Resource Sharing",
			"Could not read resource permissions: "+err.Error(),
		)
		return diags
	}

	// Handle externally deleted resource
	if getResp.StatusCode() == 404 {
		state.Id = types.StringNull()
		return diags
	}

	if getResp.StatusCode() != 200 || getResp.JSON200 == nil {
		diags.AddError(
			"Error Reading Resource Sharing",
			fmt.Sprintf("Could not read resource permissions, status: %d, body: %s", getResp.StatusCode(), string(getResp.Body)),
		)
		return diags
	}

	diags.Append(mapSharingToModel(ctx, getResp.JSON200, state)...)
	return diags
}

// mapSharingToModel maps the full API response to the Terraform model.
// Used by Read and ImportState (not by Create/Update, which use the overlay pattern).
func mapSharingToModel(ctx context.Context, apiResp *models.ResourcePermissionsResponse, state *sharingResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	// Computed-only fields from the response
	state.Id = types.StringPointerValue(apiResp.Id)
	state.Name = types.StringPointerValue(apiResp.Name)
	state.Description = types.StringPointerValue(apiResp.Description)

	if apiResp.CreateTime != nil {
		state.CreateTime = types.Int64Value(*apiResp.CreateTime)
	} else {
		state.CreateTime = types.Int64Null()
	}

	if apiResp.UpdateTime != nil {
		state.UpdateTime = types.Int64Value(*apiResp.UpdateTime)
	} else {
		state.UpdateTime = types.Int64Null()
	}

	// Map permissions
	state.Permissions = mapPermissionsToList(ctx, apiResp.Permissions, &diags)

	// Map public — normalize empty string to null.
	// Allocations return public:"" to mean "no public access" while other resource
	// types return null. We normalize both to null to prevent spurious drift.
	if apiResp.Public != nil && string(*apiResp.Public) != "" {
		state.Public = types.StringValue(string(*apiResp.Public))
	} else {
		state.Public = types.StringNull()
	}

	return diags
}

// overlaySharingComputedFields overlays only the Computed-only fields from the
// API response onto the plan. User-configured values (permissions, public, resource_type,
// resource_id) are preserved from the plan exactly as-is.
func overlaySharingComputedFields(ctx context.Context, apiResp *models.ResourcePermissionsResponse, plan *sharingResourceModel) diag.Diagnostics {
	// Phase 1: Build fully-resolved state from API response.
	var resolved sharingResourceModel
	diags := mapSharingToModel(ctx, apiResp, &resolved)
	if diags.HasError() {
		return diags
	}

	// Phase 2: Overlay computed-only fields — always from resolved.
	plan.Id = resolved.Id
	plan.Name = resolved.Name
	plan.Description = resolved.Description
	plan.CreateTime = resolved.CreateTime
	plan.UpdateTime = resolved.UpdateTime

	// Required (permissions) and explicit Optional (public when set): never touch — plan values preserved

	// Optional+Computed (public): resolve only when unknown (user omitted it)
	if plan.Public.IsUnknown() {
		plan.Public = resolved.Public
	}

	return diags
}

// buildSharingRequest converts the Terraform model into an API request body.
func buildSharingRequest(ctx context.Context, plan *sharingResourceModel, resourceType string) (models.UpdateResourcePermissionJSONRequestBody, diag.Diagnostics) {
	var diags diag.Diagnostics
	var reqBody models.UpdateResourcePermissionJSONRequestBody

	// Map permissions from plan to API type
	if !plan.Permissions.IsNull() {
		var permVals []resource_sharing.PermissionsValue
		diags.Append(plan.Permissions.ElementsAs(ctx, &permVals, false)...)
		if diags.HasError() {
			return reqBody, diags
		}

		perms := make([]models.ResourcePermission, 0, len(permVals))
		for _, pv := range permVals {
			if pv.IsNull() || pv.IsUnknown() {
				continue
			}

			perm := models.ResourcePermission{}
			if !pv.User.IsNull() && !pv.User.IsUnknown() {
				perm.User = new(pv.User.ValueString())
			}
			if !pv.Role.IsNull() && !pv.Role.IsUnknown() {
				perm.Role = new(models.ResourcePermissionRole(pv.Role.ValueString()))
			}

			perms = append(perms, perm)
		}
		reqBody.Permissions = &perms
	}

	// Map public from plan to API type.
	if !plan.Public.IsNull() && !plan.Public.IsUnknown() {
		reqBody.Public = new(models.UpdateResourcePermissionRequestBodyPublic(plan.Public.ValueString()))
	} else if resourceType == "allocations" {
		// Allocations require the public field to be present in the request body.
		// The API returns 500 if public is omitted or null. An empty string signals
		// "no public access" and is the only accepted sentinel for allocations.
		reqBody.Public = new(models.UpdateResourcePermissionRequestBodyPublic(""))
	}

	return reqBody, diags
}

// mapPermissionsToList converts the API permissions array to a Terraform list.
func mapPermissionsToList(ctx context.Context, apiPerms *[]models.ResourcePermission, diags *diag.Diagnostics) types.List {
	elemType := resource_sharing.PermissionsValue{}.Type(ctx)

	if apiPerms == nil || len(*apiPerms) == 0 {
		emptyList, d := types.ListValueFrom(ctx, elemType, []resource_sharing.PermissionsValue{})
		diags.Append(d...)
		return emptyList
	}

	vals := make([]resource_sharing.PermissionsValue, 0, len(*apiPerms))
	for _, p := range *apiPerms {
		var user, role types.String
		if p.User != nil {
			user = types.StringValue(*p.User)
		} else {
			user = types.StringNull()
		}
		if p.Role != nil {
			role = types.StringValue(string(*p.Role))
		} else {
			role = types.StringNull()
		}

		permVal, d := resource_sharing.NewPermissionsValue(
			resource_sharing.PermissionsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"user": user,
				"role": role,
			},
		)
		diags.Append(d...)
		if diags.HasError() {
			return types.ListNull(elemType)
		}
		vals = append(vals, permVal)
	}

	listVal, d := types.ListValueFrom(ctx, elemType, vals)
	diags.Append(d...)
	return listVal
}
