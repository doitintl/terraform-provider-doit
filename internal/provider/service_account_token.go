// Package provider implements the DoiT Terraform provider.
package provider

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_service_account_token"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/oapi-codegen/nullable"
)

// serviceAccountTokenExpiresTimePattern is the canonical form of expires_time:
// UTC, whole seconds. The API normalizes timestamps to UTC (truncated to
// microseconds) on read, while the provider formats with time.RFC3339, so any
// other spelling would never match state and — because expires_time forces
// replacement — would recreate the token on every apply.
var serviceAccountTokenExpiresTimePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`)

const serviceAccountTokenExpiresTimeMessage = "must be an RFC 3339 timestamp in UTC with whole seconds, e.g. `2027-01-01T00:00:00Z`"

type serviceAccountTokenResourceModel struct {
	resource_service_account_token.ServiceAccountTokenModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

// populateState fetches the token from the API and populates the Terraform state.
// On 404, state.Id is set to null to signal Terraform to remove the resource from state.
func (r *serviceAccountTokenResource) populateState(ctx context.Context, state *serviceAccountTokenResourceModel) diag.Diagnostics {
	serviceAccountID := state.ServiceAccountId.ValueString()
	id := state.Id.ValueString()

	getResp, err := r.client.GetServiceAccountTokenWithResponse(ctx, serviceAccountID, id)
	if err != nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Service Account Token", fmt.Sprintf("Could not read service account token %s of service account %s: %s", id, serviceAccountID, err)),
		}
	}

	// A token ID under the wrong service account also returns 404.
	if getResp.StatusCode() == 404 {
		state.Id = types.StringNull()
		return nil
	}

	if getResp.StatusCode() != 200 {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Service Account Token", fmt.Sprintf("Unexpected status code %d for service account token %s of service account %s: %s", getResp.StatusCode(), id, serviceAccountID, string(getResp.Body))),
		}
	}

	if getResp.JSON200 == nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Service Account Token", fmt.Sprintf("Received empty response body for service account token %s of service account %s", id, serviceAccountID)),
		}
	}

	mapServiceAccountTokenToModel(getResp.JSON200, state)
	return nil
}

// mapServiceAccountTokenToModel maps the API response to the Terraform model.
//
// access_token is deliberately not mapped: the API returns it only from create,
// never from GET, so Read keeps whatever is already in state (null after import).
func mapServiceAccountTokenToModel(resp *models.ServiceAccountToken, state *serviceAccountTokenResourceModel) {
	state.Id = types.StringPointerValue(nullableToPointer(resp.Id))
	state.CustomerId = types.StringPointerValue(resp.CustomerId)
	state.Name = types.StringValue(resp.Name)
	state.State = types.StringValue(string(resp.State))
	state.CreateTime = nullableTimeToString(resp.CreateTime)
	state.ExpiresTime = nullableTimeToString(resp.ExpiresTime)
	state.LastUsedTime = nullableTimeToString(resp.LastUsedTime)

	// The path parameter is the source of truth during import; keep it when the
	// response omits the field.
	if resp.ServiceAccountId != nil {
		state.ServiceAccountId = types.StringValue(*resp.ServiceAccountId)
	}
}

// nullableTimeToString formats an optional API timestamp as UTC RFC 3339.
func nullableTimeToString(n nullable.Nullable[time.Time]) types.String {
	if t := nullableToPointer(n); t != nil {
		return types.StringValue(t.UTC().Format(time.RFC3339))
	}
	return types.StringNull()
}

// serviceAccountTokenFromCreateResponse converts the create response (the token
// plus its one-time access token) to the plain token shape used by the mapper.
func serviceAccountTokenFromCreateResponse(resp *models.CreateServiceAccountTokenResponse) *models.ServiceAccountToken {
	return &models.ServiceAccountToken{
		CreateTime:       resp.CreateTime,
		CustomerId:       resp.CustomerId,
		ExpiresTime:      resp.ExpiresTime,
		Id:               resp.Id,
		LastUsedTime:     resp.LastUsedTime,
		Name:             resp.Name,
		ServiceAccountId: resp.ServiceAccountId,
		State:            models.ServiceAccountTokenState(resp.State),
	}
}

// overlayServiceAccountTokenComputedFields uses the two-phase overlay pattern to
// reconcile the Terraform plan with the API response after Create/Update.
//
// Phase 1 (Resolve): Build a fully-resolved state from the API response using
// mapServiceAccountTokenToModel — the same mapping function used by Read.
//
// Phase 2 (Overlay): Walk the plan field-by-field. Known values are preserved.
// Unknown values are replaced with the resolved counterpart.
func overlayServiceAccountTokenComputedFields(apiResp *models.ServiceAccountToken, plan *serviceAccountTokenResourceModel) {
	// Phase 1: Build fully-resolved state from API response.
	resolved := *plan
	mapServiceAccountTokenToModel(apiResp, &resolved)

	// Phase 2: Overlay Computed-only fields unconditionally from resolved.
	plan.Id = resolved.Id
	plan.CustomerId = resolved.CustomerId
	plan.CreateTime = resolved.CreateTime

	// last_used_time: Computed-only but changes server-side whenever the token is used.
	// On Create the plan value is Unknown -> overlay fills from API.
	// On Update the framework marks it Unknown too (no UseStateForUnknown), but a
	// known value must never be replaced or Terraform reports an inconsistent result.
	if plan.LastUsedTime.IsUnknown() {
		plan.LastUsedTime = resolved.LastUsedTime //nolint:overlaycheck // guarded to avoid inconsistent result on Update
	}

	// Optional+Computed fields: resolve from the API only when the plan left them open.
	if plan.ExpiresTime.IsUnknown() {
		plan.ExpiresTime = resolved.ExpiresTime
	}
	if plan.State.IsUnknown() {
		plan.State = resolved.State
	}

	// access_token: Computed-only and returned only by create. Create sets it on the
	// plan before calling the overlay; it is stable afterwards (UseStateForUnknown).
	// Never resolve it from a response that does not carry it.
	if plan.AccessToken.IsUnknown() {
		plan.AccessToken = types.StringNull() //nolint:overlaycheck // create-only secret, not part of the Read mapping
	}

	// Name and service_account_id: Required — never touch.
}

// toCreateRequest converts the TF model to a CreateServiceAccountTokenRequest.
func (plan *serviceAccountTokenResourceModel) toCreateRequest() (models.CreateServiceAccountTokenJSONRequestBody, diag.Diagnostics) {
	var diags diag.Diagnostics

	req := models.CreateServiceAccountTokenJSONRequestBody{
		Name: plan.Name.ValueString(),
	}

	if !plan.ExpiresTime.IsUnknown() && !plan.ExpiresTime.IsNull() {
		expires, err := time.Parse(time.RFC3339, plan.ExpiresTime.ValueString())
		if err != nil {
			diags.AddAttributeError(
				path.Root("expires_time"),
				"Invalid expires_time",
				fmt.Sprintf("expires_time %s: %s", serviceAccountTokenExpiresTimeMessage, err),
			)
			return req, diags
		}
		req.ExpiresTime = &expires
	}

	return req, diags
}

// toUpdateRequest converts the TF model to an UpdateServiceAccountTokenRequest.
// Only state can be updated in place; every other attribute forces replacement.
func (plan *serviceAccountTokenResourceModel) toUpdateRequest() models.UpdateServiceAccountTokenJSONRequestBody {
	req := models.UpdateServiceAccountTokenJSONRequestBody{}

	if !plan.State.IsUnknown() {
		req.State = models.UpdateServiceAccountTokenRequestState(plan.State.ValueString())
	}

	return req
}
