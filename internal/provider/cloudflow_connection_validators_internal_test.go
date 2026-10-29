package provider

import (
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_cloudflow_connection"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func testCollaborator(t *testing.T, email, role attr.Value) resource_cloudflow_connection.CollaboratorsValue {
	t.Helper()
	return resource_cloudflow_connection.NewCollaboratorsValueMust(
		resource_cloudflow_connection.CollaboratorsValue{}.AttributeTypes(t.Context()),
		map[string]attr.Value{"email": email, "role": role},
	)
}

func testCollaboratorList(t *testing.T, elements ...attr.Value) types.List {
	t.Helper()
	return types.ListValueMust(resource_cloudflow_connection.CollaboratorsValue{}.Type(t.Context()), elements)
}

func TestCloudflowConnectionCollaboratorsValidator(t *testing.T) {
	owner := testCollaborator(t, types.StringValue("owner@example.com"), types.StringValue("owner"))
	owner2 := testCollaborator(t, types.StringValue("other@example.com"), types.StringValue("owner"))
	editor := testCollaborator(t, types.StringValue("editor@example.com"), types.StringValue("editor"))
	user := testCollaborator(t, types.StringValue("user@example.com"), types.StringValue("user"))
	editorSameMail := testCollaborator(t, types.StringValue("OWNER@example.com"), types.StringValue("editor"))
	unknownElement := resource_cloudflow_connection.NewCollaboratorsValueUnknown()
	unknownRole := testCollaborator(t, types.StringValue("x@example.com"), types.StringUnknown())
	unknownEmailOwner := testCollaborator(t, types.StringUnknown(), types.StringValue("owner"))
	nullEmail := testCollaborator(t, types.StringNull(), types.StringValue("editor"))
	nullRole := testCollaborator(t, types.StringValue("n@example.com"), types.StringNull())

	tests := []struct {
		name    string
		value   types.List
		summary string // empty: no error expected
	}{
		{"null list", types.ListNull(resource_cloudflow_connection.CollaboratorsValue{}.Type(t.Context())), ""},
		{"unknown list", types.ListUnknown(resource_cloudflow_connection.CollaboratorsValue{}.Type(t.Context())), ""},
		{"single owner", testCollaboratorList(t, owner), ""},
		{"owner with editor and user", testCollaboratorList(t, owner, editor, user), ""},
		{"owner not first", testCollaboratorList(t, editor, owner), ""},
		{"empty list", testCollaboratorList(t), "Missing owner"},
		{"only editors", testCollaboratorList(t, editor, user), "Missing owner"},
		{"two owners", testCollaboratorList(t, owner, owner2), "Too many owners"},
		{"duplicate email ignoring case", testCollaboratorList(t, owner, editorSameMail), "Duplicate collaborator"},
		// An unknown element may turn out to be the owner: the missing owner is not a definite error.
		{"unknown element and no owner", testCollaboratorList(t, editor, unknownElement), ""},
		{"unknown role and no owner", testCollaboratorList(t, editor, unknownRole), ""},
		{"unknown element with owner", testCollaboratorList(t, owner, unknownElement), ""},
		// Definite errors survive unknown elements before and after them.
		{"two owners with unknown after", testCollaboratorList(t, owner, owner2, unknownElement), "Too many owners"},
		{"two owners with unknown before", testCollaboratorList(t, unknownElement, owner, owner2), "Too many owners"},
		{"duplicate with unknown before", testCollaboratorList(t, unknownElement, owner, editorSameMail), "Duplicate collaborator"},
		{"duplicate with unknown after", testCollaboratorList(t, owner, editorSameMail, unknownElement), "Duplicate collaborator"},
		// An owner whose email is unknown still counts as the owner.
		{"owner with unknown email", testCollaboratorList(t, unknownEmailOwner, editor), ""},
		{"two owners one with unknown email", testCollaboratorList(t, unknownEmailOwner, owner), "Too many owners"},
		// Null members are reported by the required-attribute checks, not here.
		{"null email", testCollaboratorList(t, owner, nullEmail), ""},
		// A missing or unrecognised role is reported by the role attribute itself.
		{"null role only", testCollaboratorList(t, nullRole), ""},
		{"unrecognised role only", testCollaboratorList(t, testCollaborator(t, types.StringValue("a@example.com"), types.StringValue("admin"))), ""},
		{"unrecognised role beside an owner", testCollaboratorList(t, owner, testCollaborator(t, types.StringValue("a@example.com"), types.StringValue("admin"))), ""},
		{"unrecognised role beside two owners", testCollaboratorList(t, owner, owner2, testCollaborator(t, types.StringValue("a@example.com"), types.StringValue("admin"))), "Too many owners"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validator.ListRequest{Path: path.Root("collaborators"), ConfigValue: tt.value}
			resp := &validator.ListResponse{}
			cloudflowConnectionCollaboratorsValidator{}.ValidateList(t.Context(), req, resp)
			assertSingleError(t, resp.Diagnostics, tt.summary, "collaborators")
		})
	}
}

func testGcpConfig(t *testing.T, level, organizationID, folderID, projectID attr.Value) resource_cloudflow_connection.GcpConfigValue {
	t.Helper()
	ctx := t.Context()
	return resource_cloudflow_connection.NewGcpConfigValueMust(
		resource_cloudflow_connection.GcpConfigValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"custom_role":                   resource_cloudflow_connection.NewCustomRoleValueNull(),
			"deployment_command":            types.StringNull(),
			"folder_id":                     folderID,
			"infra_manager_location":        types.StringNull(),
			"infra_manager_project":         types.StringNull(),
			"infra_manager_service_account": types.StringNull(),
			"level":                         level,
			"organization_id":               organizationID,
			"predefined_roles":              types.ListNull(types.StringType),
			"project_id":                    projectID,
			"service_account_name":          types.StringValue("sa"),
			"status":                        types.StringNull(),
		},
	)
}

func TestValidateCloudflowConnectionGcpConfig(t *testing.T) {
	null, unknown := types.StringNull(), types.StringUnknown()
	id := types.StringValue("some-id")
	level := func(l string) attr.Value { return types.StringValue(l) }

	tests := []struct {
		name    string
		value   resource_cloudflow_connection.GcpConfigValue
		summary string
		path    string
	}{
		{"null config", resource_cloudflow_connection.NewGcpConfigValueNull(), "", ""},
		{"unknown config", resource_cloudflow_connection.NewGcpConfigValueUnknown(), "", ""},
		{"unknown level", testGcpConfig(t, unknown, null, null, null), "", ""},
		{"null level", testGcpConfig(t, null, null, null, null), "", ""},
		{"project with project id", testGcpConfig(t, level("project"), null, null, id), "", ""},
		{"folder with folder id", testGcpConfig(t, level("folder"), null, id, null), "", ""},
		{"organization with organization id", testGcpConfig(t, level("organization"), id, null, null), "", ""},
		{"project without project id", testGcpConfig(t, level("project"), id, id, null), "Missing GCP ID", "gcp_config.project_id"},
		{"folder without folder id", testGcpConfig(t, level("folder"), id, null, id), "Missing GCP ID", "gcp_config.folder_id"},
		{"organization without organization id", testGcpConfig(t, level("organization"), null, id, id), "Missing GCP ID", "gcp_config.organization_id"},
		{"project with unknown project id", testGcpConfig(t, level("project"), null, null, unknown), "", ""},
		{"folder with unknown folder id", testGcpConfig(t, level("folder"), null, unknown, null), "", ""},
		{"organization with unknown organization id", testGcpConfig(t, level("organization"), unknown, null, null), "", ""},
		{"unrecognised level is left to the enum validator", testGcpConfig(t, level("bogus"), null, null, null), "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var diags diag.Diagnostics
			validateCloudflowConnectionGcpConfig(tt.value, &diags)
			assertSingleError(t, diags, tt.summary, tt.path)
		})
	}
}

func testAwsConfig(t *testing.T, permissions resource_cloudflow_connection.PermissionsValue) resource_cloudflow_connection.AwsConfigValue {
	t.Helper()
	ctx := t.Context()
	return resource_cloudflow_connection.NewAwsConfigValueMust(
		resource_cloudflow_connection.AwsConfigValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"context":                    types.ListNull(resource_cloudflow_connection.ContextValue{}.Type(ctx)),
			"management_account":         types.StringNull(),
			"organization_root_id":       types.StringNull(),
			"permissions":                permissions,
			"role_name":                  types.StringValue("role"),
			"scope_excluded_account_ids": types.ListNull(types.StringType),
			"scope_explicit_account_ids": types.ListNull(types.StringType),
			"scope_management_account_explicit_in_scope": types.BoolNull(),
			"scope_targeted_organizational_unit_ids":     types.ListNull(types.StringType),
		},
	)
}

func testPermissions(t *testing.T, actions, policies types.List) resource_cloudflow_connection.PermissionsValue {
	t.Helper()
	return resource_cloudflow_connection.NewPermissionsValueMust(
		resource_cloudflow_connection.PermissionsValue{}.AttributeTypes(t.Context()),
		map[string]attr.Value{"actions": actions, "aws_managed_policies": policies},
	)
}

func TestValidateCloudflowConnectionAwsConfig(t *testing.T) {
	ctx := t.Context()
	actionType := resource_cloudflow_connection.ActionsValue{}.Type(ctx)
	nullActions := types.ListNull(actionType)
	unknownActions := types.ListUnknown(actionType)
	emptyActions := types.ListValueMust(actionType, []attr.Value{})
	someActions := types.ListValueMust(actionType, []attr.Value{
		resource_cloudflow_connection.NewActionsValueMust(
			resource_cloudflow_connection.ActionsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{"id": types.StringNull(), "value": types.StringValue("s3:ListBucket")},
		),
	})
	nullPolicies := types.ListNull(types.StringType)
	unknownPolicies := types.ListUnknown(types.StringType)
	emptyPolicies := types.ListValueMust(types.StringType, []attr.Value{})
	somePolicies := types.ListValueMust(types.StringType, []attr.Value{types.StringValue("arn:p")})
	somePoliciesWithUnknown := types.ListValueMust(types.StringType, []attr.Value{types.StringUnknown()})

	tests := []struct {
		name    string
		value   resource_cloudflow_connection.AwsConfigValue
		summary string
	}{
		{"null config", resource_cloudflow_connection.NewAwsConfigValueNull(), ""},
		{"unknown config", resource_cloudflow_connection.NewAwsConfigValueUnknown(), ""},
		{"no permissions block", testAwsConfig(t, resource_cloudflow_connection.NewPermissionsValueNull()), ""},
		{"unknown permissions block", testAwsConfig(t, resource_cloudflow_connection.NewPermissionsValueUnknown()), ""},
		{"actions only", testAwsConfig(t, testPermissions(t, someActions, nullPolicies)), ""},
		{"policies only", testAwsConfig(t, testPermissions(t, nullActions, somePolicies)), ""},
		{"both", testAwsConfig(t, testPermissions(t, someActions, somePolicies)), ""},
		{"both unset", testAwsConfig(t, testPermissions(t, nullActions, nullPolicies)), "Empty permissions"},
		{"both empty", testAwsConfig(t, testPermissions(t, emptyActions, emptyPolicies)), "Empty permissions"},
		{"empty actions and unset policies", testAwsConfig(t, testPermissions(t, emptyActions, nullPolicies)), "Empty permissions"},
		{"unknown actions", testAwsConfig(t, testPermissions(t, unknownActions, nullPolicies)), ""},
		{"unknown policies", testAwsConfig(t, testPermissions(t, emptyActions, unknownPolicies)), ""},
		{"policies with unknown element", testAwsConfig(t, testPermissions(t, nullActions, somePoliciesWithUnknown)), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var diags diag.Diagnostics
			validateCloudflowConnectionAwsConfig(tt.value, &diags)
			assertSingleError(t, diags, tt.summary, "aws_config.permissions")
		})
	}
}
