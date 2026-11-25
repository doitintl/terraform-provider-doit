package provider

import (
	"context"
	"encoding/json"

	ds "github.com/doitintl/terraform-provider-doit/internal/provider/datasource_cloudconnect_gcp_requirements"
	modelsv2 "github.com/doitintl/terraform-provider-doit/internal/provider/models/v2"
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// mapCloudconnectGcpRequirementsToModel maps the computed part of the
// requirements response. The scope inputs stay as configured. Lists and maps
// are empty, never null, when the API returns none.
func mapCloudconnectGcpRequirementsToModel(ctx context.Context, resp *modelsv2.GcpRequirementsResponse, data *cloudconnectGcpRequirementsDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	var d diag.Diagnostics

	data.CatalogVersion = types.StringValue(resp.CatalogVersion)

	data.Preconditions, d = buildGcpRequirementsPreconditions(ctx, resp.Preconditions)
	diags.Append(d...)

	data.Identity, d = buildGcpRequirementsIdentity(ctx, resp.Identity)
	diags.Append(d...)

	data.Features, d = buildGcpRequirementsFeatures(ctx, resp.Features)
	diags.Append(d...)

	data.Artifacts, d = buildGcpRequirementsArtifacts(ctx, resp.Artifacts)
	diags.Append(d...)

	data.FollowUps, d = buildGcpRequirementsFollowUps(ctx, resp.FollowUps)
	diags.Append(d...)

	return diags
}

func buildGcpRequirementsFindings(ctx context.Context, findings *[]modelsv2.OnboardingFinding) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := ds.FindingsValue{}.Type(ctx)

	values := []ds.FindingsValue{}
	if findings != nil {
		for _, finding := range *findings {
			missingInputs, d := mapStringList(ctx, finding.MissingInputs)
			diags.Append(d...)
			missingPermissions, d := mapStringList(ctx, finding.MissingPermissions)
			diags.Append(d...)

			value, d := ds.NewFindingsValue(ds.FindingsValue{}.AttributeTypes(ctx), map[string]attr.Value{
				"code":                types.StringValue(finding.Code),
				"feature_id":          types.StringPointerValue(finding.FeatureId),
				"field":               types.StringPointerValue(finding.Field),
				"message":             types.StringValue(finding.Message),
				"missing_inputs":      missingInputs,
				"missing_permissions": missingPermissions,
			})
			diags.Append(d...)
			values = append(values, value)
		}
	}
	if diags.HasError() {
		return types.ListNull(elemType), diags
	}

	list, d := types.ListValueFrom(ctx, elemType, values)
	diags.Append(d...)
	return list, diags
}

func buildGcpRequirementsPreconditions(ctx context.Context, preconditions modelsv2.RequirementsPreconditions) (ds.PreconditionsValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	findings, d := buildGcpRequirementsFindings(ctx, &preconditions.Findings)
	diags.Append(d...)
	if diags.HasError() {
		return ds.NewPreconditionsValueNull(), diags
	}

	value, d := ds.NewPreconditionsValue(ds.PreconditionsValue{}.AttributeTypes(ctx), map[string]attr.Value{
		"findings": findings,
		"status":   types.StringValue(string(preconditions.Status)),
	})
	diags.Append(d...)
	return value, diags
}

func buildGcpRequirementsIdentity(ctx context.Context, identity modelsv2.RequirementsIdentity) (ds.IdentityValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	findings, d := buildGcpRequirementsFindings(ctx, identity.Findings)
	diags.Append(d...)
	trustedPrincipals, d := mapStringList(ctx, identity.TrustedPrincipals)
	diags.Append(d...)
	if diags.HasError() {
		return ds.NewIdentityValueNull(), diags
	}

	kind := types.StringNull()
	if identity.Kind != nil {
		kind = types.StringValue(string(*identity.Kind))
	}

	value, d := ds.NewIdentityValue(ds.IdentityValue{}.AttributeTypes(ctx), map[string]attr.Value{
		"application_id":             types.StringPointerValue(identity.ApplicationId),
		"customer_binding":           types.StringPointerValue(identity.CustomerBinding),
		"external_id":                types.StringPointerValue(identity.ExternalId),
		"findings":                   findings,
		"impersonation_principal":    types.StringPointerValue(identity.ImpersonationPrincipal),
		"impersonation_role":         types.StringPointerValue(identity.ImpersonationRole),
		"kind":                       kind,
		"role_name":                  types.StringPointerValue(identity.RoleName),
		"service_account_email":      types.StringPointerValue(identity.ServiceAccountEmail),
		"service_account_id":         types.StringPointerValue(identity.ServiceAccountId),
		"service_account_project_id": types.StringPointerValue(identity.ServiceAccountProjectId),
		"status":                     types.StringValue(string(identity.Status)),
		"trusted_principals":         trustedPrincipals,
	})
	diags.Append(d...)
	return value, diags
}

func buildGcpRequirementsFeatures(ctx context.Context, features []modelsv2.RequirementsFeature) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := ds.FeaturesValue{}.Type(ctx)

	values := make([]ds.FeaturesValue, 0, len(features))
	for _, feature := range features {
		value, d := buildGcpRequirementsFeature(ctx, feature)
		diags.Append(d...)
		values = append(values, value)
	}
	if diags.HasError() {
		return types.ListNull(elemType), diags
	}

	list, d := types.ListValueFrom(ctx, elemType, values)
	diags.Append(d...)
	return list, diags
}

func buildGcpRequirementsFeature(ctx context.Context, feature modelsv2.RequirementsFeature) (ds.FeaturesValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	additionalScopes := []ds.AdditionalScopesValue{}
	if feature.AdditionalScopes != nil {
		for _, scope := range *feature.AdditionalScopes {
			permissions, d := mapStringList(ctx, &scope.Permissions)
			diags.Append(d...)
			value, d := ds.NewAdditionalScopesValue(ds.AdditionalScopesValue{}.AttributeTypes(ctx), map[string]attr.Value{
				"permissions": permissions,
				"phase":       types.StringValue(scope.Phase),
				"recipient":   types.StringValue(scope.Recipient),
				"scope_kind":  types.StringValue(scope.ScopeKind),
			})
			diags.Append(d...)
			additionalScopes = append(additionalScopes, value)
		}
	}

	requiredInputs := []ds.RequiredInputsValue{}
	if feature.RequiredInputs != nil {
		for _, input := range *feature.RequiredInputs {
			allowedValues, d := mapStringList(ctx, input.AllowedValues)
			diags.Append(d...)
			value, d := ds.NewRequiredInputsValue(ds.RequiredInputsValue{}.AttributeTypes(ctx), map[string]attr.Value{
				"allowed_values": allowedValues,
				"default":        types.StringPointerValue(input.Default),
				"description":    types.StringPointerValue(input.Description),
				"name":           types.StringValue(input.Name),
				"required":       types.BoolValue(input.Required),
				"type":           types.StringValue(string(input.Type)),
			})
			diags.Append(d...)
			requiredInputs = append(requiredInputs, value)
		}
	}

	roles := []ds.RolesValue{}
	if feature.Roles != nil {
		for _, role := range *feature.Roles {
			value, d := ds.NewRolesValue(ds.RolesValue{}.AttributeTypes(ctx), map[string]attr.Value{
				"name":               types.StringValue(role.Name),
				"role_definition_id": types.StringValue(role.RoleDefinitionId.String()),
			})
			diags.Append(d...)
			roles = append(roles, value)
		}
	}
	if diags.HasError() {
		return ds.NewFeaturesValueNull(), diags
	}

	additionalScopesList, d := types.ListValueFrom(ctx, ds.AdditionalScopesValue{}.Type(ctx), additionalScopes)
	diags.Append(d...)
	requiredInputsList, d := types.ListValueFrom(ctx, ds.RequiredInputsValue{}.Type(ctx), requiredInputs)
	diags.Append(d...)
	rolesList, d := types.ListValueFrom(ctx, ds.RolesValue{}.Type(ctx), roles)
	diags.Append(d...)
	findings, d := buildGcpRequirementsFindings(ctx, feature.Findings)
	diags.Append(d...)
	healthChecks, d := mapStringList(ctx, feature.HealthChecks)
	diags.Append(d...)
	managedPolicies, d := mapStringList(ctx, feature.ManagedPolicies)
	diags.Append(d...)
	permissions, d := mapStringList(ctx, feature.Permissions)
	diags.Append(d...)
	resolvedInputs, d := mapStringMap(ctx, feature.ResolvedInputs)
	diags.Append(d...)
	if diags.HasError() {
		return ds.NewFeaturesValueNull(), diags
	}

	value, d := ds.NewFeaturesValue(ds.FeaturesValue{}.AttributeTypes(ctx), map[string]attr.Value{
		"additional_scopes":      additionalScopesList,
		"findings":               findings,
		"health_checks":          healthChecks,
		"id":                     types.StringValue(feature.Id),
		"managed_policies":       managedPolicies,
		"permissions":            permissions,
		"required_inputs":        requiredInputsList,
		"requirements_satisfied": types.BoolValue(feature.RequirementsSatisfied),
		"resolved_inputs":        resolvedInputs,
		"roles":                  rolesList,
		"status":                 types.StringValue(string(feature.Status)),
	})
	diags.Append(d...)
	return value, diags
}

func buildGcpRequirementsArtifacts(ctx context.Context, artifacts []modelsv2.RequirementsArtifact) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := ds.ArtifactsValue{}.Type(ctx)

	values := make([]ds.ArtifactsValue, 0, len(artifacts))
	for _, artifact := range artifacts {
		value, d := buildGcpRequirementsArtifact(ctx, artifact)
		diags.Append(d...)
		values = append(values, value)
	}
	if diags.HasError() {
		return types.ListNull(elemType), diags
	}

	list, d := types.ListValueFrom(ctx, elemType, values)
	diags.Append(d...)
	return list, diags
}

func buildGcpRequirementsArtifact(ctx context.Context, artifact modelsv2.RequirementsArtifact) (ds.ArtifactsValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	changes := []ds.ChangesValue{}
	if artifact.Changes != nil {
		for _, change := range *artifact.Changes {
			changeValues, d := mapStringList(ctx, change.Values)
			diags.Append(d...)
			value, d := ds.NewChangesValue(ds.ChangesValue{}.AttributeTypes(ctx), map[string]attr.Value{
				"action":   types.StringValue(change.Action),
				"member":   types.StringPointerValue(change.Member),
				"resource": types.StringValue(change.Resource),
				"role":     types.StringPointerValue(change.Role),
				"values":   changeValues,
			})
			diags.Append(d...)
			changes = append(changes, value)
		}
	}

	files := []ds.FilesValue{}
	if artifact.Files != nil {
		for _, file := range *artifact.Files {
			value, d := ds.NewFilesValue(ds.FilesValue{}.AttributeTypes(ctx), map[string]attr.Value{
				"path":   types.StringValue(file.Path),
				"sha256": types.StringValue(file.Sha256),
				"url":    types.StringValue(file.Url),
			})
			diags.Append(d...)
			files = append(files, value)
		}
	}

	trustStatements := []ds.TrustStatementsValue{}
	if artifact.TrustStatements != nil {
		for _, statement := range *artifact.TrustStatements {
			value, d := ds.NewTrustStatementsValue(ds.TrustStatementsValue{}.AttributeTypes(ctx), map[string]attr.Value{
				"external_id": types.StringPointerValue(statement.ExternalId),
				"principal":   types.StringValue(statement.Principal),
			})
			diags.Append(d...)
			trustStatements = append(trustStatements, value)
		}
	}

	registrationHandoff := ds.NewRegistrationHandoffValueNull()
	if handoff := nullableToPointer(artifact.RegistrationHandoff); handoff != nil {
		formFields, d := mapStringMap(ctx, handoff.FormFields)
		diags.Append(d...)
		value, d := ds.NewRegistrationHandoffValue(ds.RegistrationHandoffValue{}.AttributeTypes(ctx), map[string]attr.Value{
			"connection_id":   types.StringPointerValue(handoff.ConnectionId),
			"descriptor_file": types.StringPointerValue(handoff.DescriptorFile),
			"endpoint_path":   types.StringValue(handoff.EndpointPath),
			"form_fields":     formFields,
			"method":          types.StringValue(string(handoff.Method)),
		})
		diags.Append(d...)
		registrationHandoff = value
	}

	parameters := jsontypes.NewNormalizedNull()
	if artifact.Parameters != nil {
		encoded, err := json.Marshal(*artifact.Parameters)
		if err != nil {
			diags.AddError("Error Mapping GCP Cloud Connect Requirements", "Could not encode artifact parameters: "+err.Error())
		} else {
			parameters = jsontypes.NewNormalizedValue(string(encoded))
		}
	}
	if diags.HasError() {
		return ds.NewArtifactsValueNull(), diags
	}

	changesList, d := types.ListValueFrom(ctx, ds.ChangesValue{}.Type(ctx), changes)
	diags.Append(d...)
	filesList, d := types.ListValueFrom(ctx, ds.FilesValue{}.Type(ctx), files)
	diags.Append(d...)
	trustStatementsList, d := types.ListValueFrom(ctx, ds.TrustStatementsValue{}.Type(ctx), trustStatements)
	diags.Append(d...)
	findings, d := buildGcpRequirementsFindings(ctx, artifact.Findings)
	diags.Append(d...)
	requiredTools, d := mapStringList(ctx, artifact.RequiredTools)
	diags.Append(d...)
	if diags.HasError() {
		return ds.NewArtifactsValueNull(), diags
	}

	value, d := ds.NewArtifactsValue(ds.ArtifactsValue{}.AttributeTypes(ctx), map[string]attr.Value{
		"artifact_id":            types.StringValue(artifact.ArtifactId),
		"changes":                changesList,
		"entrypoint":             types.StringPointerValue(artifact.Entrypoint),
		"files":                  filesList,
		"findings":               findings,
		"inline_content":         types.StringPointerValue(artifact.InlineContent),
		"kind":                   types.StringValue(string(artifact.Kind)),
		"parameters":             parameters,
		"registration":           types.StringValue(string(artifact.Registration)),
		"registration_handoff":   registrationHandoff,
		"required_tools":         requiredTools,
		"requirements_satisfied": types.BoolValue(artifact.RequirementsSatisfied),
		"source":                 types.StringPointerValue(artifact.Source),
		"template_url":           types.StringPointerValue(artifact.TemplateUrl),
		"trust_statements":       trustStatementsList,
		"version":                types.StringPointerValue(artifact.Version),
	})
	diags.Append(d...)
	return value, diags
}

func buildGcpRequirementsFollowUps(ctx context.Context, followUps []modelsv2.RequirementsFollowUp) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := ds.FollowUpsValue{}.Type(ctx)

	values := make([]ds.FollowUpsValue, 0, len(followUps))
	for _, followUp := range followUps {
		requiredFor, d := mapStringList(ctx, &followUp.RequiredFor)
		diags.Append(d...)
		value, d := ds.NewFollowUpsValue(ds.FollowUpsValue{}.AttributeTypes(ctx), map[string]attr.Value{
			"executor":     types.StringValue(string(followUp.Executor)),
			"kind":         types.StringValue(followUp.Kind),
			"required_for": requiredFor,
		})
		diags.Append(d...)
		values = append(values, value)
	}
	if diags.HasError() {
		return types.ListNull(elemType), diags
	}

	list, d := types.ListValueFrom(ctx, elemType, values)
	diags.Append(d...)
	return list, diags
}

// mapStringMap maps a *map[string]string to a Terraform map of strings, empty when absent.
func mapStringMap(ctx context.Context, values *map[string]string) (types.Map, diag.Diagnostics) {
	if values == nil {
		return types.MapValueFrom(ctx, types.StringType, map[string]string{})
	}
	return types.MapValueFrom(ctx, types.StringType, *values)
}
