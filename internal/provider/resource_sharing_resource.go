package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_resource_sharing"
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
	resourceSharingResource struct {
		client *models.ClientWithResponses
	}
	resourceSharingResourceModel struct {
		resource_resource_sharing.ResourceSharingModel
		Timeouts timeouts.Value `tfsdk:"timeouts"`
	}
)

// Ensure the implementation satisfies expected interfaces.
var (
	_ resource.Resource                     = (*resourceSharingResource)(nil)
	_ resource.ResourceWithConfigure        = (*resourceSharingResource)(nil)
	_ resource.ResourceWithImportState      = (*resourceSharingResource)(nil)
	_ resource.ResourceWithConfigValidators = (*resourceSharingResource)(nil)
)

// NewResourceSharingResource creates a new resource sharing resource instance.
func NewResourceSharingResource() resource.Resource {
	return &resourceSharingResource{}
}

// Configure adds the provider configured client to the resource.
func (r *resourceSharingResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *resourceSharingResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_resource_sharing"
}

func (r *resourceSharingResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := resource_resource_sharing.ResourceSharingResourceSchema(ctx)

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

	// public: allow clearing by setting null in config (workaround for framework issue #603)
	if attr, ok := s.Attributes["public"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, useNullForUnknownWhenConfigNull())
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

func (r *resourceSharingResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourceSharingOwnerValidator{},
	}
}

func (r *resourceSharingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan resourceSharingResourceModel

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
	apiReq, diags := buildSharingRequest(ctx, &plan)
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
	overlayResourceSharingComputedFields(putResp.JSON200, &plan)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *resourceSharingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state resourceSharingResourceModel

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

	// Handle externally deleted resource (populateState sets ResourceType to null on 404)
	if state.ResourceType.IsNull() {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *resourceSharingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan resourceSharingResourceModel

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
	apiReq, diags := buildSharingRequest(ctx, &plan)
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
	overlayResourceSharingComputedFields(putResp.JSON200, &plan)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *resourceSharingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state resourceSharingResourceModel

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

	// Step 3: PUT with just the owner (strips all editors/viewers) and null public
	resetPerms := []models.ResourcePermission{*ownerPerm}
	resetReq := models.UpdateResourcePermissionJSONRequestBody{
		Permissions: &resetPerms,
		Public:      nil,
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

func (r *resourceSharingResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
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
// On 404, sets ResourceType to null to signal the caller to call RemoveResource().
func (r *resourceSharingResource) populateState(ctx context.Context, state *resourceSharingResourceModel) diag.Diagnostics {
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
		state.ResourceType = types.StringNull()
		return diags
	}

	if getResp.StatusCode() != 200 || getResp.JSON200 == nil {
		diags.AddError(
			"Error Reading Resource Sharing",
			fmt.Sprintf("Could not read resource permissions, status: %d, body: %s", getResp.StatusCode(), string(getResp.Body)),
		)
		return diags
	}

	diags.Append(mapResourceSharingToModel(ctx, getResp.JSON200, state)...)
	return diags
}

// mapResourceSharingToModel maps the full API response to the Terraform model.
// Used by Read and ImportState (not by Create/Update, which use the overlay pattern).
func mapResourceSharingToModel(ctx context.Context, apiResp *models.ResourcePermissionsResponse, state *resourceSharingResourceModel) diag.Diagnostics {
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

	// Map public
	if apiResp.Public != nil {
		state.Public = types.StringValue(string(*apiResp.Public))
	} else {
		state.Public = types.StringNull()
	}

	return diags
}

// overlayResourceSharingComputedFields overlays only the Computed-only fields from the
// API response onto the plan. User-configured values (permissions, public, resource_type,
// resource_id) are preserved from the plan exactly as-is.
func overlayResourceSharingComputedFields(apiResp *models.ResourcePermissionsResponse, plan *resourceSharingResourceModel) {
	// Computed-only: always set from API response
	plan.Id = types.StringPointerValue(apiResp.Id)
	plan.Name = types.StringPointerValue(apiResp.Name)
	plan.Description = types.StringPointerValue(apiResp.Description)

	if apiResp.CreateTime != nil {
		plan.CreateTime = types.Int64Value(*apiResp.CreateTime)
	} else {
		plan.CreateTime = types.Int64Null()
	}

	if apiResp.UpdateTime != nil {
		plan.UpdateTime = types.Int64Value(*apiResp.UpdateTime)
	} else {
		plan.UpdateTime = types.Int64Null()
	}

	// Required (permissions) and explicit Optional (public when set): never touch — plan values preserved

	// Optional+Computed (public): resolve only when unknown (user omitted it)
	if plan.Public.IsUnknown() {
		if apiResp.Public != nil {
			plan.Public = types.StringValue(string(*apiResp.Public))
		} else {
			plan.Public = types.StringNull()
		}
	}
}

// buildSharingRequest converts the Terraform model into an API request body.
func buildSharingRequest(ctx context.Context, plan *resourceSharingResourceModel) (models.UpdateResourcePermissionJSONRequestBody, diag.Diagnostics) {
	var diags diag.Diagnostics
	var reqBody models.UpdateResourcePermissionJSONRequestBody

	// Map permissions from plan to API type
	if !plan.Permissions.IsNull() && !plan.Permissions.IsUnknown() {
		var permVals []resource_resource_sharing.PermissionsValue
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
				user := pv.User.ValueString()
				perm.User = &user
			}
			if !pv.Role.IsNull() && !pv.Role.IsUnknown() {
				role := models.ResourcePermissionRole(pv.Role.ValueString())
				perm.Role = &role
			}

			perms = append(perms, perm)
		}
		reqBody.Permissions = &perms
	}

	// Map public from plan to API type
	if !plan.Public.IsNull() && !plan.Public.IsUnknown() {
		pub := models.UpdateResourcePermissionRequestBodyPublic(plan.Public.ValueString())
		reqBody.Public = &pub
	}

	return reqBody, diags
}

// mapPermissionsToList converts the API permissions array to a Terraform list.
func mapPermissionsToList(ctx context.Context, apiPerms *[]models.ResourcePermission, diags *diag.Diagnostics) types.List {
	elemType := resource_resource_sharing.PermissionsValue{}.Type(ctx)

	if apiPerms == nil || len(*apiPerms) == 0 {
		emptyList, d := types.ListValueFrom(ctx, elemType, []resource_resource_sharing.PermissionsValue{})
		diags.Append(d...)
		return emptyList
	}

	vals := make([]resource_resource_sharing.PermissionsValue, 0, len(*apiPerms))
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

		permVal, d := resource_resource_sharing.NewPermissionsValue(
			resource_resource_sharing.PermissionsValue{}.AttributeTypes(ctx),
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
