package provider

import (
	"context"
	"fmt"
	"time"

	modelsv2 "github.com/doitintl/terraform-provider-doit/internal/provider/models/v2"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_cloudconnect_gcp_connection"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type cloudconnectGcpConnectionResourceModel struct {
	resource_cloudconnect_gcp_connection.CloudconnectGcpConnectionModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

// populateState fetches the connection and maps it into state. On 404,
// state.Id is set to null so that Read removes the resource.
func (r *cloudconnectGcpConnectionResource) populateState(ctx context.Context, state *cloudconnectGcpConnectionResourceModel) diag.Diagnostics {
	id := state.Id.ValueString()

	getResp, err := r.client.GetGcpCloudConnectConnectionWithResponse(ctx, id, &modelsv2.GetGcpCloudConnectConnectionParams{})
	if err != nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading GCP Cloud Connect Connection", fmt.Sprintf("Could not read GCP Cloud Connect connection %s: %s", id, err)),
		}
	}

	if getResp.StatusCode() == 404 {
		state.Id = types.StringNull()
		return nil
	}

	if getResp.StatusCode() != 200 || getResp.JSON200 == nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading GCP Cloud Connect Connection", fmt.Sprintf("Could not read GCP Cloud Connect connection %s, status: %d, body: %s", id, getResp.StatusCode(), string(getResp.Body))),
		}
	}

	return mapCloudconnectGcpConnectionToModel(ctx, getResp.JSON200, state)
}

// mapCloudconnectGcpConnectionToModel maps the API response to the Terraform model.
func mapCloudconnectGcpConnectionToModel(ctx context.Context, resp *modelsv2.GcpCloudConnectConnection, state *cloudconnectGcpConnectionResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	state.Id = types.StringPointerValue(resp.Id)
	state.Etag = types.StringPointerValue(resp.Etag)
	state.Revision = types.StringPointerValue(resp.Revision)
	state.BindingId = types.StringPointerValue(resp.BindingId)
	state.ScopeKind = types.StringValue(string(resp.ScopeKind))
	state.OrganizationId = types.StringPointerValue(nullableToPointer(resp.OrganizationId))
	state.ProjectId = types.StringPointerValue(nullableToPointer(resp.ProjectId))
	state.DatasetId = types.StringPointerValue(nullableToPointer(resp.DatasetId))
	state.RoleId = types.StringPointerValue(nullableToPointer(resp.RoleId))
	state.CatalogVersion = types.StringPointerValue(nullableToPointer(resp.CatalogVersion))
	state.ClientUserAgent = types.StringPointerValue(nullableToPointer(resp.ClientUserAgent))

	state.RegisteredVia = types.StringNull()
	if resp.RegisteredVia != nil {
		state.RegisteredVia = types.StringValue(string(*resp.RegisteredVia))
	}

	serviceAccount, d := resource_cloudconnect_gcp_connection.NewServiceAccountValue(
		resource_cloudconnect_gcp_connection.ServiceAccountValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"email":      types.StringValue(resp.ServiceAccount.Email),
			"project_id": types.StringValue(resp.ServiceAccount.ProjectId),
			"unique_id":  types.StringValue(resp.ServiceAccount.UniqueId),
		},
	)
	diags.Append(d...)
	state.ServiceAccount = serviceAccount

	featureIDs := resp.FeatureIds
	if featureIDs == nil {
		featureIDs = []string{}
	}
	state.FeatureIds, d = types.SetValueFrom(ctx, types.StringType, featureIDs)
	diags.Append(d...)

	featureInputs, d := buildGcpFeatureInputs(ctx, nullableToPointer(resp.FeatureInputs))
	diags.Append(d...)
	state.FeatureInputs = featureInputs

	registration, d := buildGcpConnectionRegistration(ctx, resp.Registration)
	diags.Append(d...)
	state.Registration = registration

	baselineChecks, d := buildGcpConnectionBaselineChecks(ctx, resp.BaselineChecks)
	diags.Append(d...)
	state.BaselineChecks = baselineChecks

	featureVerification, d := buildGcpConnectionFeatureVerification(ctx, resp.FeatureVerification)
	diags.Append(d...)
	state.FeatureVerification = featureVerification

	followUps, d := buildGcpConnectionFollowUps(ctx, resp.FollowUps)
	diags.Append(d...)
	state.FollowUps = followUps

	return diags
}

// buildGcpFeatureInputs maps featureInputs. The API returns null unless a
// BigQuery FinOps location is stored.
func buildGcpFeatureInputs(ctx context.Context, inputs *modelsv2.GcpFeatureInputs) (resource_cloudconnect_gcp_connection.FeatureInputsValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	if inputs == nil {
		return resource_cloudconnect_gcp_connection.NewFeatureInputsValueNull(), diags
	}

	bigqueryFinops := resource_cloudconnect_gcp_connection.NewBigqueryFinopsValueNull()
	if finops := nullableToPointer(inputs.BigqueryFinops); finops != nil {
		value, d := resource_cloudconnect_gcp_connection.NewBigqueryFinopsValue(
			resource_cloudconnect_gcp_connection.BigqueryFinopsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"location": types.StringPointerValue(nullableToPointer(finops.Location)),
			},
		)
		diags.Append(d...)
		bigqueryFinops = value
	}

	featureInputs, d := resource_cloudconnect_gcp_connection.NewFeatureInputsValue(
		resource_cloudconnect_gcp_connection.FeatureInputsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"bigquery_finops": bigqueryFinops,
		},
	)
	diags.Append(d...)
	return featureInputs, diags
}

func buildGcpConnectionRegistration(ctx context.Context, registration *modelsv2.Registration) (resource_cloudconnect_gcp_connection.RegistrationValue, diag.Diagnostics) {
	if registration == nil {
		return resource_cloudconnect_gcp_connection.NewRegistrationValueNull(), nil
	}
	return resource_cloudconnect_gcp_connection.NewRegistrationValue(
		resource_cloudconnect_gcp_connection.RegistrationValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"registration_time": timeValue(nullableToPointer(registration.RegistrationTime)),
			"status":            types.StringValue(string(registration.Status)),
		},
	)
}

func buildGcpConnectionBaselineChecks(ctx context.Context, checks *modelsv2.BaselineChecks) (resource_cloudconnect_gcp_connection.BaselineChecksValue, diag.Diagnostics) {
	if checks == nil {
		return resource_cloudconnect_gcp_connection.NewBaselineChecksValueNull(), nil
	}
	return resource_cloudconnect_gcp_connection.NewBaselineChecksValue(
		resource_cloudconnect_gcp_connection.BaselineChecksValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"check_time": timeValue(nullableToPointer(checks.CheckTime)),
			"status":     types.StringValue(string(checks.Status)),
		},
	)
}

func buildGcpConnectionFeatureVerification(ctx context.Context, verification *modelsv2.VerificationSummary) (resource_cloudconnect_gcp_connection.FeatureVerificationValue, diag.Diagnostics) {
	if verification == nil {
		return resource_cloudconnect_gcp_connection.NewFeatureVerificationValueNull(), nil
	}
	return resource_cloudconnect_gcp_connection.NewFeatureVerificationValue(
		resource_cloudconnect_gcp_connection.FeatureVerificationValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"catalog_version":  types.StringPointerValue(nullableToPointer(verification.CatalogVersion)),
			"checked_revision": types.StringPointerValue(nullableToPointer(verification.CheckedRevision)),
			"observation_time": timeValue(nullableToPointer(verification.ObservationTime)),
			"status":           types.StringValue(string(verification.Status)),
		},
	)
}

func buildGcpConnectionFollowUps(ctx context.Context, followUps *[]modelsv2.FollowUp) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := resource_cloudconnect_gcp_connection.FollowUpsValue{}.Type(ctx)

	values := []resource_cloudconnect_gcp_connection.FollowUpsValue{}
	if followUps != nil {
		for _, followUp := range *followUps {
			lastError := resource_cloudconnect_gcp_connection.NewLastErrorValueNull()
			if stepError := nullableToPointer(followUp.LastError); stepError != nil {
				value, d := resource_cloudconnect_gcp_connection.NewLastErrorValue(
					resource_cloudconnect_gcp_connection.LastErrorValue{}.AttributeTypes(ctx),
					map[string]attr.Value{
						"code":      types.StringValue(stepError.Code),
						"message":   types.StringValue(stepError.Message),
						"operation": types.StringPointerValue(stepError.Operation),
					},
				)
				diags.Append(d...)
				lastError = value
			}

			value, d := resource_cloudconnect_gcp_connection.NewFollowUpsValue(
				resource_cloudconnect_gcp_connection.FollowUpsValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"binding_id": types.StringValue(followUp.BindingId),
					"feature_id": types.StringPointerValue(nullableToPointer(followUp.FeatureId)),
					"kind":       types.StringValue(followUp.Kind),
					"last_error": lastError,
					"status":     types.StringValue(string(followUp.Status)),
				},
			)
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

// timeValue formats an API timestamp, or returns null when there is none.
func timeValue(t *time.Time) types.String {
	if t == nil {
		return types.StringNull()
	}
	return types.StringValue(t.UTC().Format(time.RFC3339Nano))
}

// overlayCloudconnectGcpConnectionComputedFields keeps the planned values and
// resolves the Computed and unknown Optional+Computed fields from the response.
func overlayCloudconnectGcpConnectionComputedFields(ctx context.Context, resp *modelsv2.GcpCloudConnectConnection, plan *cloudconnectGcpConnectionResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	resolved := *plan
	diags.Append(mapCloudconnectGcpConnectionToModel(ctx, resp, &resolved)...)
	if diags.HasError() {
		return diags
	}

	// Computed-only, stable.
	plan.Id = resolved.Id
	plan.BindingId = resolved.BindingId
	plan.RegisteredVia = resolved.RegisteredVia
	plan.Registration = resolved.Registration

	// Computed-only, volatile: ModifyPlan marks them unknown on every update.
	if plan.Etag.IsUnknown() {
		plan.Etag = resolved.Etag
	}
	if plan.Revision.IsUnknown() {
		plan.Revision = resolved.Revision
	}
	if plan.ClientUserAgent.IsUnknown() {
		plan.ClientUserAgent = resolved.ClientUserAgent
	}
	if plan.FollowUps.IsUnknown() {
		plan.FollowUps = resolved.FollowUps
	}
	if plan.BaselineChecks.IsUnknown() {
		plan.BaselineChecks = resolved.BaselineChecks
	}
	if plan.FeatureVerification.IsUnknown() {
		plan.FeatureVerification = resolved.FeatureVerification
	}

	// Optional+Computed: resolved only when omitted.
	if plan.OrganizationId.IsUnknown() {
		plan.OrganizationId = resolved.OrganizationId
	}
	if plan.ProjectId.IsUnknown() {
		plan.ProjectId = resolved.ProjectId
	}
	if plan.DatasetId.IsUnknown() {
		plan.DatasetId = resolved.DatasetId
	}
	if plan.RoleId.IsUnknown() {
		plan.RoleId = resolved.RoleId
	}
	if plan.CatalogVersion.IsUnknown() {
		plan.CatalogVersion = resolved.CatalogVersion
	}

	// feature_inputs: the plan modifier plans null when it is omitted, so it is
	// only unknown for an unknown configured value.
	if plan.FeatureInputs.IsUnknown() {
		plan.FeatureInputs = resolved.FeatureInputs
	} else if !plan.FeatureInputs.IsNull() {
		diags.Append(overlayGcpConnectionFeatureInputs(ctx, &resolved.FeatureInputs, &plan.FeatureInputs)...)
	}

	// scope_kind, service_account, feature_ids: Required — never touch.

	return diags
}

// overlayGcpConnectionFeatureInputs resolves the unknown parts of a configured feature_inputs block.
func overlayGcpConnectionFeatureInputs(ctx context.Context, resolved, plan *resource_cloudconnect_gcp_connection.FeatureInputsValue) diag.Diagnostics {
	if plan.BigqueryFinops.IsUnknown() {
		plan.BigqueryFinops = resolved.BigqueryFinops
	} else if !plan.BigqueryFinops.IsNull() {
		return overlayGcpConnectionBigqueryFinops(ctx, &resolved.BigqueryFinops, &plan.BigqueryFinops)
	}
	return nil
}

// overlayGcpConnectionBigqueryFinops resolves an unknown location of a configured bigquery_finops block.
func overlayGcpConnectionBigqueryFinops(_ context.Context, resolved, plan *resource_cloudconnect_gcp_connection.BigqueryFinopsValue) diag.Diagnostics {
	if plan.Location.IsUnknown() {
		plan.Location = resolved.Location
	}
	return nil
}

// toCreateRequest converts the plan to a register request.
func (plan *cloudconnectGcpConnectionResourceModel) toCreateRequest(ctx context.Context) (modelsv2.CreateGcpCloudConnectConnectionJSONRequestBody, diag.Diagnostics) {
	var diags diag.Diagnostics

	var featureIDs []string
	diags.Append(plan.FeatureIds.ElementsAs(ctx, &featureIDs, false)...)

	req := modelsv2.CreateGcpCloudConnectConnectionJSONRequestBody{
		ScopeKind: modelsv2.GcpCloudConnectConnectionCreateScopeKind(plan.ScopeKind.ValueString()),
		ServiceAccount: modelsv2.GcpServiceAccount{
			Email:     plan.ServiceAccount.Email.ValueString(),
			ProjectId: plan.ServiceAccount.ProjectId.ValueString(),
			UniqueId:  plan.ServiceAccount.UniqueId.ValueString(),
		},
		FeatureIds: featureIDs,
	}

	if !plan.OrganizationId.IsUnknown() && !plan.OrganizationId.IsNull() {
		req.OrganizationId = valueToNullable(plan.OrganizationId.ValueString())
	}
	if !plan.ProjectId.IsUnknown() && !plan.ProjectId.IsNull() {
		req.ProjectId = valueToNullable(plan.ProjectId.ValueString())
	}
	if !plan.DatasetId.IsUnknown() && !plan.DatasetId.IsNull() {
		req.DatasetId = valueToNullable(plan.DatasetId.ValueString())
	}
	if !plan.RoleId.IsUnknown() && !plan.RoleId.IsNull() {
		req.RoleId = valueToNullable(plan.RoleId.ValueString())
	}
	if !plan.CatalogVersion.IsUnknown() && !plan.CatalogVersion.IsNull() {
		req.CatalogVersion = valueToNullable(plan.CatalogVersion.ValueString())
	}
	if !plan.FeatureInputs.IsUnknown() && !plan.FeatureInputs.IsNull() {
		req.FeatureInputs.Set(toGcpFeatureInputs(plan.FeatureInputs))
	}

	return req, diags
}

// toUpdateRequest converts the plan to a merge patch. feature_ids is always
// sent. feature_inputs is sent only when it changed, so that an update of the
// features does not repeat the stored inputs; removing the block sends null,
// which clears them. The scope, the service account and role_id cannot change
// in place (they require replacement).
func (plan *cloudconnectGcpConnectionResourceModel) toUpdateRequest(ctx context.Context, state *cloudconnectGcpConnectionResourceModel) (modelsv2.UpdateGcpCloudConnectConnectionApplicationMergePatchPlusJSONRequestBody, diag.Diagnostics) {
	var diags diag.Diagnostics

	var featureIDs []string
	diags.Append(plan.FeatureIds.ElementsAs(ctx, &featureIDs, false)...)

	req := modelsv2.UpdateGcpCloudConnectConnectionApplicationMergePatchPlusJSONRequestBody{
		FeatureIds: &featureIDs,
	}

	if !plan.CatalogVersion.IsUnknown() && !plan.CatalogVersion.IsNull() {
		req.CatalogVersion = valueToNullable(plan.CatalogVersion.ValueString())
	}

	setGcpFeatureInputsUpdate(&req, plan.FeatureInputs, state.FeatureInputs)

	return req, diags
}

// setGcpFeatureInputsUpdate sets feature_inputs in an update request when it
// changed. feature_inputs is a Category A clearable object: removing the block
// sends featureInputs: null, which clears the stored inputs. An unknown value
// is left out.
func setGcpFeatureInputsUpdate(req *modelsv2.UpdateGcpCloudConnectConnectionApplicationMergePatchPlusJSONRequestBody, featureInputs, stored resource_cloudconnect_gcp_connection.FeatureInputsValue) {
	if featureInputs.IsUnknown() || featureInputs.Equal(stored) {
		return
	}
	if featureInputs.IsNull() {
		req.FeatureInputs.SetNull()
		return
	}
	req.FeatureInputs.Set(toGcpFeatureInputs(featureInputs))
}

// toGcpFeatureInputs converts a known, non-null feature_inputs block. Its
// validators require bigquery_finops and its location whenever it is set.
func toGcpFeatureInputs(featureInputs resource_cloudconnect_gcp_connection.FeatureInputsValue) modelsv2.GcpFeatureInputs {
	var inputs modelsv2.GcpFeatureInputs
	finops := featureInputs.BigqueryFinops
	if finops.IsNull() || finops.IsUnknown() {
		return inputs
	}

	var bigqueryFinops modelsv2.GcpBigQueryFinOpsInputs
	if !finops.Location.IsUnknown() && !finops.Location.IsNull() {
		bigqueryFinops.Location = valueToNullable(finops.Location.ValueString())
	}
	inputs.BigqueryFinops.Set(bigqueryFinops)
	return inputs
}

// cloudconnectGcpFollowUpWarnings returns a warning for every follow-up step
// that failed. The connection is registered and kept in state, but the feature
// of the step does not work until the step succeeds.
func cloudconnectGcpFollowUpWarnings(followUps *[]modelsv2.FollowUp) diag.Diagnostics {
	var diags diag.Diagnostics
	if followUps == nil {
		return diags
	}
	for _, followUp := range *followUps {
		if followUp.Status != modelsv2.FollowUpStatusFailed {
			continue
		}
		detail := fmt.Sprintf("The connection is registered, but its follow-up step %q", followUp.Kind)
		if featureID := nullableToPointer(followUp.FeatureId); featureID != nil {
			detail += fmt.Sprintf(" for feature %q", *featureID)
		}
		detail += " failed"
		if stepError := nullableToPointer(followUp.LastError); stepError != nil {
			detail += fmt.Sprintf(": %s (%s)", stepError.Message, stepError.Code)
		}
		detail += ". The feature does not work until the step succeeds. Fix the cause, then replace the connection " +
			"(terraform apply -replace) to run the step again; Terraform does not resend an unchanged configuration."
		diags.AddWarning("GCP Cloud Connect Follow-Up Step Failed", detail)
	}
	return diags
}
