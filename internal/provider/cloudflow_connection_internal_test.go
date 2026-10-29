package provider

import (
	"encoding/json"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_cloudflow_connection"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const testCloudflowAwsJSON = `{
	"connectionId": "conn-aws",
	"name": "AWS prod",
	"description": "Prod accounts",
	"awsConfig": {
		"context": [
			{"accountId": "123456789012", "regions": ["us-east-1", "eu-west-1"], "status": "pending_permissions"},
			{"accountId": "210987654321", "status": "pending_permissions"}
		],
		"roleName": "doit-cloudflow",
		"permissions": {
			"awsManagedPolicies": ["arn:aws:iam::aws:policy/ReadOnlyAccess"],
			"actions": [{"value": "s3:ListBucket", "id": "a1"}, {"value": "ec2:DescribeInstances"}]
		},
		"managementAccount": "111111111111",
		"organizationRootId": "r-abcd",
		"scopeExplicitAccountIds": ["123456789012"],
		"scopeManagementAccountExplicitInScope": true
	},
	"collaborators": [
		{"email": "owner@example.com", "role": "owner"},
		{"email": "editor@example.com", "role": "editor"}
	],
	"enabled": true,
	"status": "active",
	"createdAt": "2026-10-26T13:39:40.248794Z",
	"updatedAt": "2026-10-26T13:40:11.444956Z",
	"etag": "\"etag-1\""
}`

// testCloudflowAwsMinimalJSON is what the API returns for an AWS connection
// created with only the required fields: unset strings and lists are absent,
// and permissions is an empty object.
const testCloudflowAwsMinimalJSON = `{
	"connectionId": "conn-aws-min",
	"name": "AWS minimal",
	"awsConfig": {
		"context": [{"accountId": "123456789012", "status": "pending_permissions"}],
		"roleName": "doit-cloudflow",
		"permissions": {},
		"scopeManagementAccountExplicitInScope": false
	},
	"collaborators": [{"email": "owner@example.com", "role": "owner"}],
	"status": "active",
	"createdAt": "2026-10-26T13:39:40.248794Z",
	"updatedAt": "2026-10-26T13:39:40.248794Z",
	"etag": "\"etag-min\""
}`

const testCloudflowGcpJSON = `{
	"connectionId": "conn-gcp",
	"name": "GCP project",
	"description": "Project connection",
	"gcpConfig": {
		"organizationId": "",
		"folderId": "",
		"projectId": "my-project",
		"level": "project",
		"serviceAccountName": "doit-cloudflow",
		"predefinedRoles": ["roles/viewer"],
		"customRole": {"roleId": "doitRole", "permissions": ["compute.instances.list"]},
		"infraManagerProject": "infra-project",
		"infraManagerLocation": "us-central1",
		"infraManagerServiceAccount": "infra-sa",
		"status": "pending",
		"deploymentCommand": "gcloud deploy"
	},
	"collaborators": [{"email": "owner@example.com", "role": "owner"}],
	"enabled": false,
	"status": "disabled",
	"createdAt": "2026-10-26T13:39:40Z",
	"updatedAt": "2026-10-26T13:39:41Z",
	"etag": "\"etag-gcp\""
}`

// testCloudflowGcpMinimalJSON mirrors the API's answer for a GCP connection
// without optional settings: ids come back as "", the custom role as {"roleId":""}
// and predefinedRoles as null.
const testCloudflowGcpMinimalJSON = `{
	"connectionId": "conn-gcp-min",
	"name": "GCP minimal",
	"gcpConfig": {
		"organizationId": "",
		"folderId": "",
		"projectId": "my-project",
		"level": "project",
		"serviceAccountName": "doit-cloudflow",
		"predefinedRoles": null,
		"customRole": {"roleId": ""},
		"status": ""
	},
	"collaborators": [{"email": "owner@example.com", "role": "owner"}],
	"enabled": true,
	"status": "active",
	"createdAt": "2026-10-26T13:39:40Z",
	"updatedAt": "2026-10-26T13:39:40Z",
	"etag": "\"etag-gcp-min\""
}`

func cloudflowConnectionTestData(t *testing.T, body string) *cloudflowConnectionData {
	t.Helper()
	var conn models.CloudflowConnection
	if err := json.Unmarshal([]byte(body), &conn); err != nil {
		t.Fatalf("unmarshal connection: %v", err)
	}
	return newCloudflowConnectionData(&conn, []byte(body))
}

// cloudflowConnectionNullTimeouts is the value of an unset timeouts block.
func cloudflowConnectionNullTimeouts() timeouts.Value {
	return timeouts.Value{Object: types.ObjectNull(map[string]attr.Type{
		"create": types.StringType,
		"read":   types.StringType,
		"update": types.StringType,
		"delete": types.StringType,
	})}
}

// mapCloudflowConnectionForTest maps an API body the way Read does.
func mapCloudflowConnectionForTest(t *testing.T, body string) cloudflowConnectionResourceModel {
	t.Helper()
	state := cloudflowConnectionResourceModel{Timeouts: cloudflowConnectionNullTimeouts()}
	if diags := mapCloudflowConnectionToModel(t.Context(), cloudflowConnectionTestData(t, body), &state); diags.HasError() {
		t.Fatalf("map: %v", diags)
	}
	return state
}

func cloudflowConnectionStringsOf(t *testing.T, list types.List) []string {
	t.Helper()
	out, diags := cloudflowConnectionStrings(t.Context(), list)
	if diags.HasError() {
		t.Fatalf("read list: %v", diags)
	}
	return out
}

func assertStringSlice(t *testing.T, name string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s = %v, want %v", name, got, want)
		return
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%s = %v, want %v", name, got, want)
			return
		}
	}
}

func TestMapCloudflowConnectionToModel_AWS(t *testing.T) {
	state := mapCloudflowConnectionForTest(t, testCloudflowAwsJSON)

	if state.ConnectionId.ValueString() != "conn-aws" || state.Name.ValueString() != "AWS prod" || state.Description.ValueString() != "Prod accounts" {
		t.Errorf("top-level fields: %q %q %q", state.ConnectionId.ValueString(), state.Name.ValueString(), state.Description.ValueString())
	}
	if !state.Enabled.ValueBool() || state.Status.ValueString() != "active" || state.Etag.ValueString() != `"etag-1"` {
		t.Errorf("enabled/status/etag: %v %q %q", state.Enabled, state.Status.ValueString(), state.Etag.ValueString())
	}
	if state.CreatedAt.ValueString() != "2026-10-26T13:39:40.248794Z" || state.UpdatedAt.ValueString() != "2026-10-26T13:40:11.444956Z" {
		t.Errorf("timestamps: %q %q", state.CreatedAt.ValueString(), state.UpdatedAt.ValueString())
	}
	if !state.GcpConfig.IsNull() {
		t.Errorf("gcp_config = %v, want null", state.GcpConfig)
	}

	var collaborators []resource_cloudflow_connection.CollaboratorsValue
	if diags := state.Collaborators.ElementsAs(t.Context(), &collaborators, false); diags.HasError() {
		t.Fatalf("collaborators: %v", diags)
	}
	if len(collaborators) != 2 || collaborators[0].Email.ValueString() != "owner@example.com" || collaborators[0].Role.ValueString() != "owner" || collaborators[1].Role.ValueString() != "editor" {
		t.Errorf("collaborators = %v", collaborators)
	}

	aws := state.AwsConfig
	if aws.RoleName.ValueString() != "doit-cloudflow" || aws.ManagementAccount.ValueString() != "111111111111" || aws.OrganizationRootId.ValueString() != "r-abcd" {
		t.Errorf("aws strings: %q %q %q", aws.RoleName.ValueString(), aws.ManagementAccount.ValueString(), aws.OrganizationRootId.ValueString())
	}
	if !aws.ScopeManagementAccountExplicitInScope.ValueBool() {
		t.Errorf("scope_management_account_explicit_in_scope = false, want true")
	}
	assertStringSlice(t, "scope_explicit_account_ids", cloudflowConnectionStringsOf(t, aws.ScopeExplicitAccountIds), []string{"123456789012"})
	assertStringSlice(t, "scope_excluded_account_ids", cloudflowConnectionStringsOf(t, aws.ScopeExcludedAccountIds), []string{})
	assertStringSlice(t, "scope_targeted_organizational_unit_ids", cloudflowConnectionStringsOf(t, aws.ScopeTargetedOrganizationalUnitIds), []string{})

	var contexts []resource_cloudflow_connection.ContextValue
	if diags := aws.Context.ElementsAs(t.Context(), &contexts, false); diags.HasError() {
		t.Fatalf("context: %v", diags)
	}
	if len(contexts) != 2 {
		t.Fatalf("context = %d elements, want 2", len(contexts))
	}
	if contexts[0].AccountId.ValueString() != "123456789012" || contexts[0].Status.ValueString() != "pending_permissions" {
		t.Errorf("context[0] = %v", contexts[0])
	}
	assertStringSlice(t, "context[0].regions", cloudflowConnectionStringsOf(t, contexts[0].Regions), []string{"us-east-1", "eu-west-1"})
	assertStringSlice(t, "context[1].regions", cloudflowConnectionStringsOf(t, contexts[1].Regions), []string{})

	if aws.Permissions.IsNull() {
		t.Fatalf("permissions is null, want a value")
	}
	assertStringSlice(t, "aws_managed_policies", cloudflowConnectionStringsOf(t, aws.Permissions.AwsManagedPolicies), []string{"arn:aws:iam::aws:policy/ReadOnlyAccess"})
	var actions []resource_cloudflow_connection.ActionsValue
	if diags := aws.Permissions.Actions.ElementsAs(t.Context(), &actions, false); diags.HasError() {
		t.Fatalf("actions: %v", diags)
	}
	if len(actions) != 2 || actions[0].Value.ValueString() != "s3:ListBucket" || actions[0].Id.ValueString() != "a1" || actions[1].Value.ValueString() != "ec2:DescribeInstances" || !actions[1].Id.IsNull() {
		t.Errorf("actions = %v", actions)
	}
}

func TestMapCloudflowConnectionToModel_AWSMinimal(t *testing.T) {
	state := mapCloudflowConnectionForTest(t, testCloudflowAwsMinimalJSON)

	if !state.Description.IsNull() {
		t.Errorf("description = %v, want null", state.Description)
	}
	// An omitted enabled means enabled, which is the schema default.
	if !state.Enabled.ValueBool() {
		t.Errorf("enabled = false, want true")
	}
	aws := state.AwsConfig
	if !aws.Permissions.IsNull() {
		t.Errorf("permissions = %v, want null for an empty object", aws.Permissions)
	}
	if !aws.ManagementAccount.IsNull() || !aws.OrganizationRootId.IsNull() {
		t.Errorf("optional strings = %v %v, want null", aws.ManagementAccount, aws.OrganizationRootId)
	}
	if aws.ScopeManagementAccountExplicitInScope.ValueBool() {
		t.Errorf("scope_management_account_explicit_in_scope = true, want false")
	}
	for name, list := range map[string]types.List{
		"scope_excluded_account_ids":             aws.ScopeExcludedAccountIds,
		"scope_explicit_account_ids":             aws.ScopeExplicitAccountIds,
		"scope_targeted_organizational_unit_ids": aws.ScopeTargetedOrganizationalUnitIds,
	} {
		if list.IsNull() || len(list.Elements()) != 0 {
			t.Errorf("%s = %v, want an empty list (never null)", name, list)
		}
	}
	var contexts []resource_cloudflow_connection.ContextValue
	if diags := aws.Context.ElementsAs(t.Context(), &contexts, false); diags.HasError() {
		t.Fatalf("context: %v", diags)
	}
	if contexts[0].Regions.IsNull() || len(contexts[0].Regions.Elements()) != 0 {
		t.Errorf("regions = %v, want an empty list", contexts[0].Regions)
	}
}

func TestMapCloudflowConnectionToModel_PermissionsOnlyPolicies(t *testing.T) {
	body := `{"connectionId":"c","name":"n","awsConfig":{"context":[{"accountId":"123456789012"}],"roleName":"r","permissions":{"awsManagedPolicies":["arn:p"]}},"collaborators":[]}`
	state := mapCloudflowConnectionForTest(t, body)

	// The generated response type has no field for awsManagedPolicies; it is read from the raw body.
	if state.AwsConfig.Permissions.IsNull() {
		t.Fatal("permissions is null, want the managed policies read from the raw body")
	}
	assertStringSlice(t, "aws_managed_policies", cloudflowConnectionStringsOf(t, state.AwsConfig.Permissions.AwsManagedPolicies), []string{"arn:p"})
	if state.AwsConfig.Permissions.Actions.IsNull() || len(state.AwsConfig.Permissions.Actions.Elements()) != 0 {
		t.Errorf("actions = %v, want an empty list", state.AwsConfig.Permissions.Actions)
	}
}

func TestMapCloudflowConnectionToModel_GCP(t *testing.T) {
	state := mapCloudflowConnectionForTest(t, testCloudflowGcpJSON)

	if state.Enabled.ValueBool() || state.Status.ValueString() != "disabled" {
		t.Errorf("enabled/status = %v %q", state.Enabled, state.Status.ValueString())
	}
	if !state.AwsConfig.IsNull() {
		t.Errorf("aws_config = %v, want null", state.AwsConfig)
	}
	gcp := state.GcpConfig
	if gcp.Level.ValueString() != "project" || gcp.ProjectId.ValueString() != "my-project" || gcp.ServiceAccountName.ValueString() != "doit-cloudflow" {
		t.Errorf("level/project/service account: %q %q %q", gcp.Level.ValueString(), gcp.ProjectId.ValueString(), gcp.ServiceAccountName.ValueString())
	}
	// "" ids are the API's spelling of "not set".
	if !gcp.OrganizationId.IsNull() || !gcp.FolderId.IsNull() {
		t.Errorf("organization_id/folder_id = %v %v, want null", gcp.OrganizationId, gcp.FolderId)
	}
	if gcp.InfraManagerProject.ValueString() != "infra-project" || gcp.InfraManagerLocation.ValueString() != "us-central1" || gcp.InfraManagerServiceAccount.ValueString() != "infra-sa" {
		t.Errorf("infra manager: %q %q %q", gcp.InfraManagerProject.ValueString(), gcp.InfraManagerLocation.ValueString(), gcp.InfraManagerServiceAccount.ValueString())
	}
	if gcp.Status.ValueString() != "pending" || gcp.DeploymentCommand.ValueString() != "gcloud deploy" {
		t.Errorf("status/deployment_command: %q %q", gcp.Status.ValueString(), gcp.DeploymentCommand.ValueString())
	}
	assertStringSlice(t, "predefined_roles", cloudflowConnectionStringsOf(t, gcp.PredefinedRoles), []string{"roles/viewer"})
	if gcp.CustomRole.IsNull() || gcp.CustomRole.RoleId.ValueString() != "doitRole" {
		t.Fatalf("custom_role = %v", gcp.CustomRole)
	}
	assertStringSlice(t, "custom_role.permissions", cloudflowConnectionStringsOf(t, gcp.CustomRole.Permissions), []string{"compute.instances.list"})
}

func TestMapCloudflowConnectionToModel_GCPMinimal(t *testing.T) {
	state := mapCloudflowConnectionForTest(t, testCloudflowGcpMinimalJSON)

	gcp := state.GcpConfig
	if !gcp.CustomRole.IsNull() {
		t.Errorf("custom_role = %v, want null for the API's empty role", gcp.CustomRole)
	}
	if gcp.PredefinedRoles.IsNull() || len(gcp.PredefinedRoles.Elements()) != 0 {
		t.Errorf("predefined_roles = %v, want an empty list for the API's null", gcp.PredefinedRoles)
	}
	for name, v := range map[string]types.String{
		"organization_id": gcp.OrganizationId, "folder_id": gcp.FolderId, "status": gcp.Status, "deployment_command": gcp.DeploymentCommand,
		"infra_manager_project": gcp.InfraManagerProject, "infra_manager_location": gcp.InfraManagerLocation, "infra_manager_service_account": gcp.InfraManagerServiceAccount,
	} {
		if !v.IsNull() {
			t.Errorf("%s = %v, want null", name, v)
		}
	}
}

func TestMapCloudflowConnectionToModel_AbsentTimestamps(t *testing.T) {
	state := mapCloudflowConnectionForTest(t, `{"connectionId":"c","name":"n","gcpConfig":{"level":"project","projectId":"p","serviceAccountName":"s"}}`)
	if !state.CreatedAt.IsNull() || !state.UpdatedAt.IsNull() || !state.Etag.IsNull() {
		t.Errorf("created/updated/etag = %v %v %v, want null", state.CreatedAt, state.UpdatedAt, state.Etag)
	}
	if state.Collaborators.IsNull() || len(state.Collaborators.Elements()) != 0 {
		t.Errorf("collaborators = %v, want an empty list", state.Collaborators)
	}
}

func TestCloudflowConnectionManagedPolicies(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{"present", `{"awsConfig":{"permissions":{"awsManagedPolicies":["a","b"]}}}`, []string{"a", "b"}},
		{"absent", `{"awsConfig":{"permissions":{}}}`, nil},
		{"no permissions", `{"awsConfig":{}}`, nil},
		{"no aws config", `{"gcpConfig":{}}`, nil},
		{"not json", `oops`, nil},
		{"empty body", ``, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertStringSlice(t, "policies", cloudflowConnectionManagedPolicies([]byte(tt.body)), tt.want)
		})
	}
}

// cloudflowConnectionUnknownComputed marks the Computed-only members of a model unknown,
// the way the plan carries them on create.
func cloudflowConnectionUnknownComputed(t *testing.T, plan *cloudflowConnectionResourceModel) {
	t.Helper()
	plan.ConnectionId = types.StringUnknown()
	plan.CreatedAt = types.StringUnknown()
	plan.UpdatedAt = types.StringUnknown()
	plan.Etag = types.StringUnknown()
	plan.Status = types.StringUnknown()

	if !plan.GcpConfig.IsNull() {
		plan.GcpConfig.Status = types.StringUnknown()
		plan.GcpConfig.DeploymentCommand = types.StringUnknown()
	}
	if !plan.AwsConfig.IsNull() {
		var contexts []resource_cloudflow_connection.ContextValue
		if diags := plan.AwsConfig.Context.ElementsAs(t.Context(), &contexts, false); diags.HasError() {
			t.Fatalf("context: %v", diags)
		}
		for i := range contexts {
			contexts[i].Status = types.StringUnknown()
		}
		list, diags := types.ListValueFrom(t.Context(), resource_cloudflow_connection.ContextValue{}.Type(t.Context()), contexts)
		if diags.HasError() {
			t.Fatalf("context list: %v", diags)
		}
		plan.AwsConfig.Context = list

		if !plan.AwsConfig.Permissions.IsNull() {
			var actions []resource_cloudflow_connection.ActionsValue
			if diags := plan.AwsConfig.Permissions.Actions.ElementsAs(t.Context(), &actions, false); diags.HasError() {
				t.Fatalf("actions: %v", diags)
			}
			for i := range actions {
				actions[i].Id = types.StringUnknown()
			}
			list, diags := types.ListValueFrom(t.Context(), resource_cloudflow_connection.ActionsValue{}.Type(t.Context()), actions)
			if diags.HasError() {
				t.Fatalf("action list: %v", diags)
			}
			plan.AwsConfig.Permissions.Actions = list
		}
	}
}

func TestOverlayCloudflowConnectionComputedFields_Create(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"aws", testCloudflowAwsJSON},
		{"gcp", testCloudflowGcpJSON},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := mapCloudflowConnectionForTest(t, tt.body)
			// The plan keeps what the user configured, even where the API echoes it differently.
			plan.Description = types.StringValue("configured description")
			cloudflowConnectionUnknownComputed(t, &plan)

			if diags := overlayCloudflowConnectionComputedFields(t.Context(), cloudflowConnectionTestData(t, tt.body), &plan); diags.HasError() {
				t.Fatalf("overlay: %v", diags)
			}

			want := mapCloudflowConnectionForTest(t, tt.body)
			if plan.ConnectionId != want.ConnectionId || plan.CreatedAt != want.CreatedAt || plan.UpdatedAt != want.UpdatedAt || plan.Etag != want.Etag || plan.Status != want.Status {
				t.Errorf("computed fields not resolved: %v %v %v %v %v", plan.ConnectionId, plan.CreatedAt, plan.UpdatedAt, plan.Etag, plan.Status)
			}
			if plan.Description.ValueString() != "configured description" {
				t.Errorf("description = %q, want the configured value", plan.Description.ValueString())
			}
			if !plan.GcpConfig.IsNull() && (plan.GcpConfig.Status != want.GcpConfig.Status || plan.GcpConfig.DeploymentCommand != want.GcpConfig.DeploymentCommand) {
				t.Errorf("gcp_config computed members = %v %v", plan.GcpConfig.Status, plan.GcpConfig.DeploymentCommand)
			}
			if !plan.AwsConfig.IsNull() {
				if !plan.AwsConfig.Context.Equal(want.AwsConfig.Context) {
					t.Errorf("context = %v, want %v", plan.AwsConfig.Context, want.AwsConfig.Context)
				}
				if !plan.AwsConfig.Permissions.Actions.Equal(want.AwsConfig.Permissions.Actions) {
					t.Errorf("actions = %v, want %v", plan.AwsConfig.Permissions.Actions, want.AwsConfig.Permissions.Actions)
				}
			}
			if plan.ConnectionId.IsUnknown() || plan.Etag.IsUnknown() {
				t.Errorf("unknown value left in plan")
			}
		})
	}
}

// On Update the plan carries known prior values for the volatile fields; they are kept
// (the next Read refreshes them) and the configured values are never overwritten.
func TestOverlayCloudflowConnectionComputedFields_UpdateKeepsKnownValues(t *testing.T) {
	plan := mapCloudflowConnectionForTest(t, testCloudflowAwsJSON)
	plan.Etag = types.StringValue("old-etag")
	plan.UpdatedAt = types.StringValue("2026-01-01T00:00:00Z")
	plan.Status = types.StringValue("old-status")
	plan.Name = types.StringValue("renamed")

	if diags := overlayCloudflowConnectionComputedFields(t.Context(), cloudflowConnectionTestData(t, testCloudflowAwsJSON), &plan); diags.HasError() {
		t.Fatalf("overlay: %v", diags)
	}

	if plan.Etag.ValueString() != "old-etag" || plan.UpdatedAt.ValueString() != "2026-01-01T00:00:00Z" || plan.Status.ValueString() != "old-status" {
		t.Errorf("known volatile fields overwritten: %q %q %q", plan.Etag.ValueString(), plan.UpdatedAt.ValueString(), plan.Status.ValueString())
	}
	if plan.Name.ValueString() != "renamed" {
		t.Errorf("name = %q, want the planned value", plan.Name.ValueString())
	}
	// Stable fields are always taken from the response.
	if plan.ConnectionId.ValueString() != "conn-aws" {
		t.Errorf("connection_id = %q", plan.ConnectionId.ValueString())
	}
}

func TestOverlayCloudflowConnectionComputedFields_UnknownActionIDWithoutAPIValue(t *testing.T) {
	// The API does not always return action identifiers: an unknown id resolves to null.
	plan := mapCloudflowConnectionForTest(t, testCloudflowAwsJSON)
	cloudflowConnectionUnknownComputed(t, &plan)

	if diags := overlayCloudflowConnectionComputedFields(t.Context(), cloudflowConnectionTestData(t, testCloudflowAwsJSON), &plan); diags.HasError() {
		t.Fatalf("overlay: %v", diags)
	}

	var actions []resource_cloudflow_connection.ActionsValue
	if diags := plan.AwsConfig.Permissions.Actions.ElementsAs(t.Context(), &actions, false); diags.HasError() {
		t.Fatalf("actions: %v", diags)
	}
	if actions[0].Id.ValueString() != "a1" || actions[1].Id.IsUnknown() || !actions[1].Id.IsNull() {
		t.Errorf("action ids = %v %v, want a1 and null", actions[0].Id, actions[1].Id)
	}
}

func TestCloudflowConnectionToCreateRequest(t *testing.T) {
	t.Run("aws full", func(t *testing.T) {
		plan := mapCloudflowConnectionForTest(t, testCloudflowAwsJSON)
		req, diags := plan.toCreateRequest(t.Context())
		if diags.HasError() {
			t.Fatalf("diagnostics: %v", diags)
		}
		assertJSONEqual(t, req, `{
			"name": "AWS prod",
			"description": "Prod accounts",
			"enabled": true,
			"collaborators": [{"email": "owner@example.com", "role": "owner"}, {"email": "editor@example.com", "role": "editor"}],
			"awsConfig": {
				"context": [
					{"accountId": "123456789012", "regions": ["us-east-1", "eu-west-1"]},
					{"accountId": "210987654321", "regions": []}
				],
				"roleName": "doit-cloudflow",
				"permissions": {
					"awsManagedPolicies": ["arn:aws:iam::aws:policy/ReadOnlyAccess"],
					"actions": [{"value": "s3:ListBucket"}, {"value": "ec2:DescribeInstances"}]
				},
				"managementAccount": "111111111111",
				"organizationRootId": "r-abcd",
				"scopeExcludedAccountIds": [],
				"scopeExplicitAccountIds": ["123456789012"],
				"scopeTargetedOrganizationalUnitIds": [],
				"scopeManagementAccountExplicitInScope": true
			}
		}`)
	})

	t.Run("aws minimal sends empty permissions and lists", func(t *testing.T) {
		plan := mapCloudflowConnectionForTest(t, testCloudflowAwsMinimalJSON)
		req, diags := plan.toCreateRequest(t.Context())
		if diags.HasError() {
			t.Fatalf("diagnostics: %v", diags)
		}
		assertJSONEqual(t, req, `{
			"name": "AWS minimal",
			"enabled": true,
			"collaborators": [{"email": "owner@example.com", "role": "owner"}],
			"awsConfig": {
				"context": [{"accountId": "123456789012", "regions": []}],
				"roleName": "doit-cloudflow",
				"permissions": {"awsManagedPolicies": [], "actions": []},
				"scopeExcludedAccountIds": [],
				"scopeExplicitAccountIds": [],
				"scopeTargetedOrganizationalUnitIds": [],
				"scopeManagementAccountExplicitInScope": false
			}
		}`)
	})

	t.Run("gcp full", func(t *testing.T) {
		plan := mapCloudflowConnectionForTest(t, testCloudflowGcpJSON)
		req, diags := plan.toCreateRequest(t.Context())
		if diags.HasError() {
			t.Fatalf("diagnostics: %v", diags)
		}
		assertJSONEqual(t, req, `{
			"name": "GCP project",
			"description": "Project connection",
			"enabled": false,
			"collaborators": [{"email": "owner@example.com", "role": "owner"}],
			"gcpConfig": {
				"level": "project",
				"projectId": "my-project",
				"serviceAccountName": "doit-cloudflow",
				"predefinedRoles": ["roles/viewer"],
				"customRole": {"roleId": "doitRole", "permissions": ["compute.instances.list"]},
				"infraManagerProject": "infra-project",
				"infraManagerLocation": "us-central1",
				"infraManagerServiceAccount": "infra-sa"
			}
		}`)
	})

	t.Run("gcp minimal omits unset values", func(t *testing.T) {
		plan := mapCloudflowConnectionForTest(t, testCloudflowGcpMinimalJSON)
		req, diags := plan.toCreateRequest(t.Context())
		if diags.HasError() {
			t.Fatalf("diagnostics: %v", diags)
		}
		assertJSONEqual(t, req, `{
			"name": "GCP minimal",
			"enabled": true,
			"collaborators": [{"email": "owner@example.com", "role": "owner"}],
			"gcpConfig": {
				"level": "project",
				"projectId": "my-project",
				"serviceAccountName": "doit-cloudflow",
				"predefinedRoles": []
			}
		}`)
	})
}

func TestCloudflowConnectionToUpdateRequest(t *testing.T) {
	t.Run("sends the complete configuration", func(t *testing.T) {
		plan := mapCloudflowConnectionForTest(t, testCloudflowAwsJSON)
		plan.Name = types.StringValue("renamed")
		req, diags := plan.toUpdateRequest(t.Context())
		if diags.HasError() {
			t.Fatalf("diagnostics: %v", diags)
		}
		assertJSONEqual(t, req, `{
			"name": "renamed",
			"description": "Prod accounts",
			"enabled": true,
			"collaborators": [{"email": "owner@example.com", "role": "owner"}, {"email": "editor@example.com", "role": "editor"}],
			"awsConfig": {
				"context": [
					{"accountId": "123456789012", "regions": ["us-east-1", "eu-west-1"]},
					{"accountId": "210987654321", "regions": []}
				],
				"roleName": "doit-cloudflow",
				"permissions": {
					"awsManagedPolicies": ["arn:aws:iam::aws:policy/ReadOnlyAccess"],
					"actions": [{"value": "s3:ListBucket"}, {"value": "ec2:DescribeInstances"}]
				},
				"managementAccount": "111111111111",
				"organizationRootId": "r-abcd",
				"scopeExcludedAccountIds": [],
				"scopeExplicitAccountIds": ["123456789012"],
				"scopeTargetedOrganizationalUnitIds": [],
				"scopeManagementAccountExplicitInScope": true
			}
		}`)
	})

	t.Run("removed description is sent as an empty string", func(t *testing.T) {
		plan := mapCloudflowConnectionForTest(t, testCloudflowGcpJSON)
		plan.Description = types.StringNull()
		req, diags := plan.toUpdateRequest(t.Context())
		if diags.HasError() {
			t.Fatalf("diagnostics: %v", diags)
		}
		if req.Description == nil || *req.Description != "" {
			t.Errorf("description = %v, want an explicit empty string", req.Description)
		}
	})

	t.Run("removed optional values are omitted so the replace clears them", func(t *testing.T) {
		plan := mapCloudflowConnectionForTest(t, testCloudflowAwsJSON)
		plan.AwsConfig.ManagementAccount = types.StringNull()
		plan.AwsConfig.OrganizationRootId = types.StringNull()
		plan.AwsConfig.Permissions = resource_cloudflow_connection.NewPermissionsValueNull()
		req, diags := plan.toUpdateRequest(t.Context())
		if diags.HasError() {
			t.Fatalf("diagnostics: %v", diags)
		}
		if req.AwsConfig.ManagementAccount != nil || req.AwsConfig.OrganizationRootId != nil {
			t.Errorf("removed strings still sent: %v %v", req.AwsConfig.ManagementAccount, req.AwsConfig.OrganizationRootId)
		}
		assertJSONEqual(t, req.AwsConfig.Permissions, `{"awsManagedPolicies": [], "actions": []}`)
	})

	t.Run("removed custom role is omitted", func(t *testing.T) {
		plan := mapCloudflowConnectionForTest(t, testCloudflowGcpJSON)
		plan.GcpConfig.CustomRole = resource_cloudflow_connection.NewCustomRoleValueNull()
		req, diags := plan.toUpdateRequest(t.Context())
		if diags.HasError() {
			t.Fatalf("diagnostics: %v", diags)
		}
		if req.GcpConfig.CustomRole != nil {
			t.Errorf("custom role = %v, want omitted", req.GcpConfig.CustomRole)
		}
	})

	t.Run("only the configured kind is sent", func(t *testing.T) {
		gcp := mapCloudflowConnectionForTest(t, testCloudflowGcpJSON)
		req, diags := gcp.toUpdateRequest(t.Context())
		if diags.HasError() || req.AwsConfig != nil || req.GcpConfig == nil {
			t.Errorf("gcp request = aws %v gcp %v (%v)", req.AwsConfig, req.GcpConfig, diags)
		}
		aws := mapCloudflowConnectionForTest(t, testCloudflowAwsJSON)
		req, diags = aws.toUpdateRequest(t.Context())
		if diags.HasError() || req.GcpConfig != nil || req.AwsConfig == nil {
			t.Errorf("aws request = aws %v gcp %v (%v)", req.AwsConfig, req.GcpConfig, diags)
		}
	})
}

// The schema overrides must produce the attribute classes the mapping and the
// request builders rely on; a silent miss in Schema() would otherwise only show
// up as drift in acceptance tests.
func TestCloudflowConnectionSchema(t *testing.T) {
	ctx := t.Context()
	r := NewCloudflowConnectionResource()
	resp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema: %v", resp.Diagnostics)
	}
	if diags := resp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("schema does not validate: %v", diags)
	}

	type flags struct{ required, optional, computed bool }
	classOf := func(a interface {
		IsRequired() bool
		IsOptional() bool
		IsComputed() bool
	}) flags {
		return flags{a.IsRequired(), a.IsOptional(), a.IsComputed()}
	}

	top := map[string]flags{
		"connection_id": {computed: true},
		"name":          {required: true},
		"description":   {optional: true},
		"enabled":       {optional: true, computed: true},
		"collaborators": {required: true},
		"gcp_config":    {optional: true},
		"aws_config":    {optional: true},
		"status":        {computed: true},
		"etag":          {computed: true},
		"created_at":    {computed: true},
		"updated_at":    {computed: true},
	}
	for name, want := range top {
		a, ok := resp.Schema.Attributes[name]
		if !ok {
			t.Errorf("attribute %q missing", name)
			continue
		}
		if got := classOf(a); got != want {
			t.Errorf("%s: %+v, want %+v", name, got, want)
		}
	}
	if _, ok := resp.Schema.Attributes["id"]; ok {
		t.Errorf("the identifier is connection_id; an id attribute must not exist")
	}

	// Nested Optional+Computed attributes must carry a Default, so unset values are
	// stable ([] / false) and never "known after apply".
	pathsWithDefault := [][]string{
		{"gcp_config", "predefined_roles"},
		{"gcp_config", "custom_role", "permissions"},
		{"aws_config", "scope_excluded_account_ids"},
		{"aws_config", "scope_explicit_account_ids"},
		{"aws_config", "scope_targeted_organizational_unit_ids"},
		{"aws_config", "scope_management_account_explicit_in_scope"},
		{"aws_config", "permissions", "actions"},
		{"aws_config", "permissions", "aws_managed_policies"},
	}
	for _, p := range pathsWithDefault {
		fieldPath := path.Root(p[0])
		for _, name := range p[1:] {
			fieldPath = fieldPath.AtName(name)
		}
		a, diags := resp.Schema.AttributeAtPath(ctx, fieldPath)
		if diags.HasError() {
			t.Errorf("%v: %v", p, diags)
			continue
		}
		if !a.IsOptional() || !a.IsComputed() {
			t.Errorf("%v: want Optional+Computed with a default", p)
		}
		hasDefault := false
		switch v := a.(type) {
		case schema.ListAttribute:
			hasDefault = v.Default != nil
		case schema.ListNestedAttribute:
			hasDefault = v.Default != nil
		case schema.BoolAttribute:
			hasDefault = v.Default != nil
		}
		if !hasDefault {
			t.Errorf("%v: no Default", p)
		}
	}
}
