package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_cloudflow_connection"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

type cloudflowConnectionResourceModel struct {
	resource_cloudflow_connection.CloudflowConnectionModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

// cloudflowConnectionData is a connection as the API returned it.
type cloudflowConnectionData struct {
	conn *models.CloudflowConnection
	// awsManagedPolicies holds awsConfig.permissions.awsManagedPolicies. The
	// generated response type has no field for it (the spec describes the response
	// permissions object separately from the request one), so it is read from the
	// raw response body.
	awsManagedPolicies []string
}

// newCloudflowConnectionData wraps a decoded response and reads the fields the
// generated type lacks from its raw body.
func newCloudflowConnectionData(conn *models.CloudflowConnection, body []byte) *cloudflowConnectionData {
	return &cloudflowConnectionData{conn: conn, awsManagedPolicies: cloudflowConnectionManagedPolicies(body)}
}

func cloudflowConnectionManagedPolicies(body []byte) []string {
	var raw struct {
		AwsConfig *struct {
			Permissions *struct {
				AwsManagedPolicies []string `json:"awsManagedPolicies"`
			} `json:"permissions"`
		} `json:"awsConfig"`
	}
	if err := json.Unmarshal(body, &raw); err != nil || raw.AwsConfig == nil || raw.AwsConfig.Permissions == nil {
		return nil
	}
	return raw.AwsConfig.Permissions.AwsManagedPolicies
}

// fetchConnection reads a connection. found is false when it does not exist.
func (r *cloudflowConnectionResource) fetchConnection(ctx context.Context, id string) (*cloudflowConnectionData, bool, diag.Diagnostics) {
	getResp, err := r.client.GetCloudflowConnectionWithResponse(ctx, id)
	if err != nil {
		return nil, false, diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading CloudFlow Connection", fmt.Sprintf("Could not read CloudFlow connection %s: %s", id, err)),
		}
	}

	if getResp.StatusCode() == 404 {
		return nil, false, nil
	}

	if getResp.StatusCode() != 200 {
		return nil, false, diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading CloudFlow Connection", fmt.Sprintf("Unexpected status code %d for CloudFlow connection %s: %s", getResp.StatusCode(), id, string(getResp.Body))),
		}
	}

	if getResp.JSON200 == nil {
		return nil, false, diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading CloudFlow Connection", fmt.Sprintf("Received empty response body for CloudFlow connection %s", id)),
		}
	}

	return newCloudflowConnectionData(getResp.JSON200, getResp.Body), true, nil
}

// populateState fetches the connection from the API and populates the Terraform state.
// On 404, state.ConnectionId is set to null to signal Terraform to remove the resource from state.
func (r *cloudflowConnectionResource) populateState(ctx context.Context, state *cloudflowConnectionResourceModel) diag.Diagnostics {
	data, found, diags := r.fetchConnection(ctx, state.ConnectionId.ValueString())
	if diags.HasError() {
		return diags
	}

	if !found {
		state.ConnectionId = types.StringNull()
		return diags
	}

	diags.Append(mapCloudflowConnectionToModel(ctx, data, state)...)
	return diags
}

// mapCloudflowConnectionToModel maps the API response to the Terraform model.
//
// Clearable (Category A) strings map nil to "" and lists map nil to [], which is
// what the plan modifiers propose when a value is removed from the configuration.
func mapCloudflowConnectionToModel(ctx context.Context, data *cloudflowConnectionData, state *cloudflowConnectionResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	c := data.conn

	state.ConnectionId = types.StringPointerValue(c.ConnectionId)
	state.Name = types.StringPointerValue(c.Name)
	state.Description = cloudflowConnectionString(c.Description)
	// enabled defaults to true in the schema; an omitted value means enabled.
	state.Enabled = types.BoolValue(c.Enabled == nil || *c.Enabled)
	state.Etag = types.StringPointerValue(c.Etag)
	state.Status = types.StringPointerValue(c.Status)
	state.CreatedAt = cloudflowConnectionTime(c.CreatedAt)
	state.UpdatedAt = cloudflowConnectionTime(c.UpdatedAt)

	collaborators, d := buildCloudflowConnectionCollaborators(ctx, c.Collaborators)
	diags.Append(d...)
	state.Collaborators = collaborators

	awsConfig, d := buildCloudflowConnectionAwsConfig(ctx, c, data.awsManagedPolicies)
	diags.Append(d...)
	state.AwsConfig = awsConfig

	gcpConfig, d := buildCloudflowConnectionGcpConfig(ctx, c)
	diags.Append(d...)
	state.GcpConfig = gcpConfig

	return diags
}

// cloudflowConnectionString maps nil to "", the value the clearing modifier proposes.
func cloudflowConnectionString(s *string) types.String {
	if s == nil {
		return types.StringValue("")
	}
	return types.StringValue(*s)
}

// cloudflowConnectionStringOrNull maps nil and "" to null. It is used for
// Computed-only values, where "" and absent mean the same.
func cloudflowConnectionStringOrNull(s *string) types.String {
	if s == nil || *s == "" {
		return types.StringNull()
	}
	return types.StringValue(*s)
}

// cloudflowConnectionTime formats an API timestamp as UTC RFC 3339. Reading it
// back from GET after a write keeps the precision consistent across operations.
func cloudflowConnectionTime(t *time.Time) types.String {
	if t == nil {
		return types.StringNull()
	}
	return types.StringValue(t.UTC().Format(time.RFC3339Nano))
}

// cloudflowConnectionStringList maps nil to an empty list: the API omits empty
// lists, and the schema defaults them to [].
func cloudflowConnectionStringList(ctx context.Context, values *[]string) (types.List, diag.Diagnostics) {
	items := []string{}
	if values != nil {
		items = append(items, *values...)
	}
	return types.ListValueFrom(ctx, types.StringType, items)
}

// cloudflowConnectionStrings reads a list of strings; null and unknown lists read as empty.
func cloudflowConnectionStrings(ctx context.Context, list types.List) ([]string, diag.Diagnostics) {
	items := []string{}
	if list.IsNull() || list.IsUnknown() {
		return items, nil
	}
	diags := list.ElementsAs(ctx, &items, false)
	return items, diags
}

func buildCloudflowConnectionCollaborators(ctx context.Context, collaborators *[]models.CloudflowCollaborator) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics

	attrTypes := resource_cloudflow_connection.CollaboratorsValue{}.AttributeTypes(ctx)
	items := []resource_cloudflow_connection.CollaboratorsValue{}
	if collaborators != nil {
		for _, c := range *collaborators {
			email := types.StringNull()
			if c.Email != nil {
				email = types.StringValue(string(*c.Email))
			}
			role := types.StringNull()
			if c.Role != nil {
				role = types.StringValue(string(*c.Role))
			}
			v, d := resource_cloudflow_connection.NewCollaboratorsValue(attrTypes, map[string]attr.Value{
				"email": email,
				"role":  role,
			})
			diags.Append(d...)
			items = append(items, v)
		}
	}

	list, d := types.ListValueFrom(ctx, resource_cloudflow_connection.CollaboratorsValue{}.Type(ctx), items)
	diags.Append(d...)
	return list, diags
}

func buildCloudflowConnectionAwsConfig(ctx context.Context, c *models.CloudflowConnection, managedPolicies []string) (resource_cloudflow_connection.AwsConfigValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	aws := c.AwsConfig
	if aws == nil {
		return resource_cloudflow_connection.NewAwsConfigValueNull(), diags
	}

	contextAttrTypes := resource_cloudflow_connection.ContextValue{}.AttributeTypes(ctx)
	contextItems := []resource_cloudflow_connection.ContextValue{}
	if aws.Context != nil {
		for _, item := range *aws.Context {
			regions, d := cloudflowConnectionStringList(ctx, item.Regions)
			diags.Append(d...)
			v, d := resource_cloudflow_connection.NewContextValue(contextAttrTypes, map[string]attr.Value{
				"account_id": types.StringPointerValue(item.AccountId),
				"regions":    regions,
				"status":     cloudflowConnectionStringOrNull(item.Status),
			})
			diags.Append(d...)
			contextItems = append(contextItems, v)
		}
	}
	contextList, d := types.ListValueFrom(ctx, resource_cloudflow_connection.ContextValue{}.Type(ctx), contextItems)
	diags.Append(d...)

	permissions, d := buildCloudflowConnectionPermissions(ctx, aws.Permissions, managedPolicies)
	diags.Append(d...)

	excluded, d := cloudflowConnectionStringList(ctx, aws.ScopeExcludedAccountIds)
	diags.Append(d...)
	explicit, d := cloudflowConnectionStringList(ctx, aws.ScopeExplicitAccountIds)
	diags.Append(d...)
	targeted, d := cloudflowConnectionStringList(ctx, aws.ScopeTargetedOrganizationalUnitIds)
	diags.Append(d...)

	// scope_management_account_explicit_in_scope defaults to false in the schema.
	managementInScope := aws.ScopeManagementAccountExplicitInScope != nil && *aws.ScopeManagementAccountExplicitInScope

	value, d := resource_cloudflow_connection.NewAwsConfigValue(
		resource_cloudflow_connection.AwsConfigValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"context":                    contextList,
			"management_account":         cloudflowConnectionString(aws.ManagementAccount),
			"organization_root_id":       cloudflowConnectionString(aws.OrganizationRootId),
			"permissions":                permissions,
			"role_name":                  types.StringPointerValue(aws.RoleName),
			"scope_excluded_account_ids": excluded,
			"scope_explicit_account_ids": explicit,
			"scope_management_account_explicit_in_scope": types.BoolValue(managementInScope),
			"scope_targeted_organizational_unit_ids":     targeted,
		},
	)
	diags.Append(d...)
	return value, diags
}

// buildCloudflowConnectionPermissions maps the permissions object. The API always
// returns it, empty when nothing is granted, so it is always present in state.
// The managed policies come from the raw response body.
func buildCloudflowConnectionPermissions(ctx context.Context, p *models.CloudflowConnectionAwsConfigAllOf1Permissions, managedPolicies []string) (resource_cloudflow_connection.PermissionsValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	actionAttrTypes := resource_cloudflow_connection.ActionsValue{}.AttributeTypes(ctx)
	actions := []resource_cloudflow_connection.ActionsValue{}
	if p != nil && p.Actions != nil {
		for _, a := range *p.Actions {
			v, d := resource_cloudflow_connection.NewActionsValue(actionAttrTypes, map[string]attr.Value{
				"id":    cloudflowConnectionStringOrNull(a.Id),
				"value": types.StringPointerValue(a.Value),
			})
			diags.Append(d...)
			actions = append(actions, v)
		}
	}

	actionList, d := types.ListValueFrom(ctx, resource_cloudflow_connection.ActionsValue{}.Type(ctx), actions)
	diags.Append(d...)
	policyList, d := cloudflowConnectionStringList(ctx, &managedPolicies)
	diags.Append(d...)

	value, d := resource_cloudflow_connection.NewPermissionsValue(
		resource_cloudflow_connection.PermissionsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"actions":              actionList,
			"aws_managed_policies": policyList,
		},
	)
	diags.Append(d...)
	return value, diags
}

func buildCloudflowConnectionGcpConfig(ctx context.Context, c *models.CloudflowConnection) (resource_cloudflow_connection.GcpConfigValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	gcp := c.GcpConfig
	if gcp == nil {
		return resource_cloudflow_connection.NewGcpConfigValueNull(), diags
	}

	level := types.StringNull()
	if gcp.Level != nil {
		level = types.StringValue(string(*gcp.Level))
	}

	predefinedRoles, d := cloudflowConnectionStringList(ctx, gcp.PredefinedRoles)
	diags.Append(d...)

	// The API always returns the custom role, as {"roleId": ""} when none is set.
	roleID := types.StringValue("")
	var rolePermissions *[]string
	if cr := gcp.CustomRole; cr != nil {
		roleID = cloudflowConnectionString(cr.RoleId)
		rolePermissions = cr.Permissions
	}
	customRolePermissions, d := cloudflowConnectionStringList(ctx, rolePermissions)
	diags.Append(d...)
	customRole, d := resource_cloudflow_connection.NewCustomRoleValue(
		resource_cloudflow_connection.CustomRoleValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"permissions": customRolePermissions,
			"role_id":     roleID,
		},
	)
	diags.Append(d...)

	value, d := resource_cloudflow_connection.NewGcpConfigValue(
		resource_cloudflow_connection.GcpConfigValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"custom_role":                   customRole,
			"deployment_command":            cloudflowConnectionStringOrNull(gcp.DeploymentCommand),
			"folder_id":                     cloudflowConnectionString(gcp.FolderId),
			"infra_manager_location":        cloudflowConnectionString(gcp.InfraManagerLocation),
			"infra_manager_project":         cloudflowConnectionString(gcp.InfraManagerProject),
			"infra_manager_service_account": cloudflowConnectionString(gcp.InfraManagerServiceAccount),
			"level":                         level,
			"organization_id":               cloudflowConnectionString(gcp.OrganizationId),
			"predefined_roles":              predefinedRoles,
			"project_id":                    cloudflowConnectionString(gcp.ProjectId),
			"service_account_name":          types.StringPointerValue(gcp.ServiceAccountName),
			"status":                        cloudflowConnectionStringOrNull(gcp.Status),
		},
	)
	diags.Append(d...)
	return value, diags
}

// overlayCloudflowConnectionComputedFields applies the plan-first overlay for Create/Update.
func overlayCloudflowConnectionComputedFields(ctx context.Context, data *cloudflowConnectionData, plan *cloudflowConnectionResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	// Phase 1: Build fully-resolved state from API response.
	resolved := *plan
	diags.Append(mapCloudflowConnectionToModel(ctx, data, &resolved)...)
	if diags.HasError() {
		return diags
	}

	// Phase 2: Overlay Computed-only fields from resolved.
	plan.ConnectionId = resolved.ConnectionId
	plan.CreatedAt = resolved.CreatedAt

	// updated_at, etag and status change server-side on every write.
	if plan.UpdatedAt.IsUnknown() {
		plan.UpdatedAt = resolved.UpdatedAt //nolint:overlaycheck // guarded to avoid inconsistent result on Update
	}
	if plan.Etag.IsUnknown() {
		plan.Etag = resolved.Etag //nolint:overlaycheck // guarded to avoid inconsistent result on Update
	}
	if plan.Status.IsUnknown() {
		plan.Status = resolved.Status //nolint:overlaycheck // guarded to avoid inconsistent result on Update
	}

	// Phase 3: Optional+Computed fields, resolved only when unknown.
	if plan.Description.IsUnknown() {
		plan.Description = resolved.Description
	}

	if !plan.GcpConfig.IsNull() && !plan.GcpConfig.IsUnknown() {
		overlayCloudflowConnectionGcpConfig(&resolved.GcpConfig, &plan.GcpConfig)
	} else if plan.GcpConfig.IsUnknown() {
		plan.GcpConfig = resolved.GcpConfig
	}
	if !plan.AwsConfig.IsNull() && !plan.AwsConfig.IsUnknown() {
		diags.Append(overlayCloudflowConnectionAwsConfig(ctx, &resolved.AwsConfig, &plan.AwsConfig)...)
	} else if plan.AwsConfig.IsUnknown() {
		plan.AwsConfig = resolved.AwsConfig
	}

	// name, enabled and collaborators: Required or defaulted — never touch.

	return diags
}

func overlayCloudflowConnectionGcpConfig(resolved, plan *resource_cloudflow_connection.GcpConfigValue) {
	if resolved.IsNull() || resolved.IsUnknown() {
		return
	}

	if plan.Status.IsUnknown() {
		plan.Status = resolved.Status //nolint:overlaycheck // guarded to avoid inconsistent result on Update
	}
	if plan.DeploymentCommand.IsUnknown() {
		plan.DeploymentCommand = resolved.DeploymentCommand //nolint:overlaycheck // guarded to avoid inconsistent result on Update
	}
	if plan.OrganizationId.IsUnknown() {
		plan.OrganizationId = resolved.OrganizationId
	}
	if plan.FolderId.IsUnknown() {
		plan.FolderId = resolved.FolderId
	}
	if plan.ProjectId.IsUnknown() {
		plan.ProjectId = resolved.ProjectId
	}
	if plan.InfraManagerProject.IsUnknown() {
		plan.InfraManagerProject = resolved.InfraManagerProject
	}
	if plan.InfraManagerLocation.IsUnknown() {
		plan.InfraManagerLocation = resolved.InfraManagerLocation
	}
	if plan.InfraManagerServiceAccount.IsUnknown() {
		plan.InfraManagerServiceAccount = resolved.InfraManagerServiceAccount
	}
	if plan.PredefinedRoles.IsUnknown() {
		plan.PredefinedRoles = resolved.PredefinedRoles
	}

	if !plan.CustomRole.IsNull() && !plan.CustomRole.IsUnknown() {
		overlayCloudflowConnectionCustomRole(&resolved.CustomRole, &plan.CustomRole)
	} else if plan.CustomRole.IsUnknown() {
		plan.CustomRole = resolved.CustomRole
	}

}

func overlayCloudflowConnectionCustomRole(resolved, plan *resource_cloudflow_connection.CustomRoleValue) {
	if plan.RoleId.IsUnknown() {
		plan.RoleId = resolved.RoleId
	}
	if plan.Permissions.IsUnknown() {
		plan.Permissions = resolved.Permissions
	}
}

func overlayCloudflowConnectionAwsConfig(ctx context.Context, resolved, plan *resource_cloudflow_connection.AwsConfigValue) diag.Diagnostics {
	var diags diag.Diagnostics
	if resolved.IsNull() || resolved.IsUnknown() {
		return diags
	}

	if plan.ManagementAccount.IsUnknown() {
		plan.ManagementAccount = resolved.ManagementAccount
	}
	if plan.OrganizationRootId.IsUnknown() {
		plan.OrganizationRootId = resolved.OrganizationRootId
	}
	if plan.ScopeExcludedAccountIds.IsUnknown() {
		plan.ScopeExcludedAccountIds = resolved.ScopeExcludedAccountIds
	}
	if plan.ScopeExplicitAccountIds.IsUnknown() {
		plan.ScopeExplicitAccountIds = resolved.ScopeExplicitAccountIds
	}
	if plan.ScopeTargetedOrganizationalUnitIds.IsUnknown() {
		plan.ScopeTargetedOrganizationalUnitIds = resolved.ScopeTargetedOrganizationalUnitIds
	}
	if plan.ScopeManagementAccountExplicitInScope.IsUnknown() {
		plan.ScopeManagementAccountExplicitInScope = resolved.ScopeManagementAccountExplicitInScope
	}

	diags.Append(overlayListElements(ctx, &resolved.Context, &plan.Context, overlayCloudflowConnectionContext)...)

	if !plan.Permissions.IsNull() && !plan.Permissions.IsUnknown() {
		diags.Append(overlayCloudflowConnectionPermissions(ctx, &resolved.Permissions, &plan.Permissions)...)
	} else if plan.Permissions.IsUnknown() {
		plan.Permissions = resolved.Permissions
	}

	return diags
}

func overlayCloudflowConnectionContext(_ context.Context, resolved, plan *resource_cloudflow_connection.ContextValue) diag.Diagnostics {
	if plan.Regions.IsUnknown() {
		plan.Regions = resolved.Regions
	}
	if plan.Status.IsUnknown() {
		plan.Status = resolved.Status //nolint:overlaycheck // guarded to avoid inconsistent result on Update
	}
	return nil
}

func overlayCloudflowConnectionPermissions(ctx context.Context, resolved, plan *resource_cloudflow_connection.PermissionsValue) diag.Diagnostics {
	var diags diag.Diagnostics
	if resolved.IsNull() || resolved.IsUnknown() {
		return diags
	}

	if plan.AwsManagedPolicies.IsUnknown() {
		plan.AwsManagedPolicies = resolved.AwsManagedPolicies
	}
	if !plan.Actions.IsUnknown() {
		diags.Append(overlayListElements(ctx, &resolved.Actions, &plan.Actions, overlayCloudflowConnectionAction)...)
	} else {
		plan.Actions = resolved.Actions
	}
	return diags
}

func overlayCloudflowConnectionAction(_ context.Context, resolved, plan *resource_cloudflow_connection.ActionsValue) diag.Diagnostics {
	if plan.Id.IsUnknown() {
		plan.Id = resolved.Id //nolint:overlaycheck // guarded: the API does not always return an identifier
	}
	return nil
}

// cloudflowConnectionRequestParts are the pieces of a request body shared by create and update.
type cloudflowConnectionRequestParts struct {
	collaborators *[]models.CloudflowCollaborator
	awsConfig     *models.CloudflowAWSConfigRequest
	gcpConfig     *models.CloudflowGCPConfigRequest
}

// toCreateRequest converts the TF model to a create request.
func (plan *cloudflowConnectionResourceModel) toCreateRequest(ctx context.Context) (models.CreateCloudflowConnectionJSONRequestBody, diag.Diagnostics) {
	parts, diags := plan.toRequestParts(ctx)

	req := models.CreateCloudflowConnectionJSONRequestBody{
		Name:          plan.Name.ValueString(),
		Enabled:       new(plan.Enabled.ValueBool()),
		Collaborators: parts.collaborators,
		AwsConfig:     parts.awsConfig,
		GcpConfig:     parts.gcpConfig,
	}
	req.Description = cloudflowConnectionOptionalString(plan.Description)

	return req, diags
}

// toUpdateRequest converts the TF model to an update request.
//
// The API replaces gcpConfig, awsConfig and collaborators as a whole, so the
// request always carries the complete desired configuration: anything omitted
// from it is cleared. The description is sent explicitly, as an empty string
// when it was removed from configuration.
func (plan *cloudflowConnectionResourceModel) toUpdateRequest(ctx context.Context) (models.UpdateCloudflowConnectionJSONRequestBody, diag.Diagnostics) {
	parts, diags := plan.toRequestParts(ctx)

	req := models.UpdateCloudflowConnectionJSONRequestBody{
		Name:          new(plan.Name.ValueString()),
		Description:   new(cloudflowConnectionStringOrEmpty(plan.Description)),
		Enabled:       new(plan.Enabled.ValueBool()),
		Collaborators: parts.collaborators,
		AwsConfig:     parts.awsConfig,
		GcpConfig:     parts.gcpConfig,
	}

	return req, diags
}

func (plan *cloudflowConnectionResourceModel) toRequestParts(ctx context.Context) (cloudflowConnectionRequestParts, diag.Diagnostics) {
	var diags diag.Diagnostics
	var parts cloudflowConnectionRequestParts

	var collaborators []resource_cloudflow_connection.CollaboratorsValue
	diags.Append(plan.Collaborators.ElementsAs(ctx, &collaborators, false)...)
	reqCollaborators := make([]models.CloudflowCollaborator, 0, len(collaborators))
	for _, c := range collaborators {
		reqCollaborators = append(reqCollaborators, models.CloudflowCollaborator{
			Email: new(openapi_types.Email(c.Email.ValueString())),
			Role:  new(models.CloudflowCollaboratorRole(c.Role.ValueString())),
		})
	}
	parts.collaborators = &reqCollaborators

	if !plan.AwsConfig.IsNull() && !plan.AwsConfig.IsUnknown() {
		aws, d := cloudflowConnectionAwsConfigRequest(ctx, plan.AwsConfig)
		diags.Append(d...)
		parts.awsConfig = aws
	}

	if !plan.GcpConfig.IsNull() && !plan.GcpConfig.IsUnknown() {
		gcp, d := cloudflowConnectionGcpConfigRequest(ctx, plan.GcpConfig)
		diags.Append(d...)
		parts.gcpConfig = gcp
	}

	return parts, diags
}

// cloudflowConnectionOptionalString returns nil for values that are not set: null,
// unknown (not resolved yet on create) or "" (the cleared spelling).
func cloudflowConnectionOptionalString(v types.String) *string {
	if v.IsNull() || v.IsUnknown() || v.ValueString() == "" {
		return nil
	}
	return new(v.ValueString())
}

// cloudflowConnectionStringOrEmpty returns the value, or "" for null and unknown.
func cloudflowConnectionStringOrEmpty(v types.String) string {
	if v.IsNull() || v.IsUnknown() {
		return ""
	}
	return v.ValueString()
}

func cloudflowConnectionAwsConfigRequest(ctx context.Context, aws resource_cloudflow_connection.AwsConfigValue) (*models.CloudflowAWSConfigRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	var contexts []resource_cloudflow_connection.ContextValue
	diags.Append(aws.Context.ElementsAs(ctx, &contexts, false)...)
	reqContext := make([]models.CloudflowAWSConfigRequestContextItem, 0, len(contexts))
	for _, c := range contexts {
		regions, d := cloudflowConnectionStrings(ctx, c.Regions)
		diags.Append(d...)
		reqContext = append(reqContext, models.CloudflowAWSConfigRequestContextItem{
			AccountId: new(c.AccountId.ValueString()),
			Regions:   &regions,
		})
	}

	// Always send the permissions object: omitting it would not be a no-op, the
	// replace semantics would clear it.
	permissions := models.CloudflowAWSPermissionsRequest{
		Actions:            &[]models.CloudflowAWSActionRequest{},
		AwsManagedPolicies: &[]string{},
	}
	if !aws.Permissions.IsNull() && !aws.Permissions.IsUnknown() {
		// The list the user leaves out is unknown on create and sent as empty.
		if !aws.Permissions.Actions.IsUnknown() && !aws.Permissions.Actions.IsNull() {
			var actions []resource_cloudflow_connection.ActionsValue
			diags.Append(aws.Permissions.Actions.ElementsAs(ctx, &actions, false)...)
			reqActions := make([]models.CloudflowAWSActionRequest, 0, len(actions))
			for _, a := range actions {
				reqActions = append(reqActions, models.CloudflowAWSActionRequest{Value: a.Value.ValueString()})
			}
			permissions.Actions = &reqActions
		}

		policies, d := cloudflowConnectionStrings(ctx, aws.Permissions.AwsManagedPolicies)
		diags.Append(d...)
		permissions.AwsManagedPolicies = &policies
	}

	excluded, d := cloudflowConnectionStrings(ctx, aws.ScopeExcludedAccountIds)
	diags.Append(d...)
	explicit, d := cloudflowConnectionStrings(ctx, aws.ScopeExplicitAccountIds)
	diags.Append(d...)
	targeted, d := cloudflowConnectionStrings(ctx, aws.ScopeTargetedOrganizationalUnitIds)
	diags.Append(d...)

	// Not resolved yet on create: the API default is false.
	managementInScope := false
	if !aws.ScopeManagementAccountExplicitInScope.IsUnknown() {
		managementInScope = aws.ScopeManagementAccountExplicitInScope.ValueBool()
	}

	return &models.CloudflowAWSConfigRequest{
		Context:                               &reqContext,
		RoleName:                              new(aws.RoleName.ValueString()),
		Permissions:                           &permissions,
		ManagementAccount:                     cloudflowConnectionOptionalString(aws.ManagementAccount),
		OrganizationRootId:                    cloudflowConnectionOptionalString(aws.OrganizationRootId),
		ScopeExcludedAccountIds:               &excluded,
		ScopeExplicitAccountIds:               &explicit,
		ScopeTargetedOrganizationalUnitIds:    &targeted,
		ScopeManagementAccountExplicitInScope: &managementInScope,
	}, diags
}

func cloudflowConnectionGcpConfigRequest(ctx context.Context, gcp resource_cloudflow_connection.GcpConfigValue) (*models.CloudflowGCPConfigRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	roles, d := cloudflowConnectionStrings(ctx, gcp.PredefinedRoles)
	diags.Append(d...)

	req := &models.CloudflowGCPConfigRequest{
		Level:                      new(models.CloudflowGCPConfigRequestLevel(gcp.Level.ValueString())),
		ServiceAccountName:         new(gcp.ServiceAccountName.ValueString()),
		OrganizationId:             cloudflowConnectionOptionalString(gcp.OrganizationId),
		FolderId:                   cloudflowConnectionOptionalString(gcp.FolderId),
		ProjectId:                  cloudflowConnectionOptionalString(gcp.ProjectId),
		InfraManagerProject:        cloudflowConnectionOptionalString(gcp.InfraManagerProject),
		InfraManagerLocation:       cloudflowConnectionOptionalString(gcp.InfraManagerLocation),
		InfraManagerServiceAccount: cloudflowConnectionOptionalString(gcp.InfraManagerServiceAccount),
		PredefinedRoles:            &roles,
	}

	// The API reports an unset custom role as {"roleId": ""}; only a role with an ID is sent.
	if !gcp.CustomRole.IsNull() && !gcp.CustomRole.IsUnknown() && cloudflowConnectionStringOrEmpty(gcp.CustomRole.RoleId) != "" {
		permissions, d := cloudflowConnectionStrings(ctx, gcp.CustomRole.Permissions)
		diags.Append(d...)
		req.CustomRole = &models.CloudflowGCPConfigRequestCustomRole{
			RoleId:      new(cloudflowConnectionStringOrEmpty(gcp.CustomRole.RoleId)),
			Permissions: &permissions,
		}
	}

	return req, diags
}
