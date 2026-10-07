// Package provider implements the DoiT Terraform provider.
package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_service_account"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type serviceAccountResourceModel struct {
	resource_service_account.ServiceAccountModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

// populateState fetches the service account from the API and populates the Terraform state.
// On 404, state.Id is set to null to signal Terraform to remove the resource from state.
func (r *serviceAccountResource) populateState(ctx context.Context, state *serviceAccountResourceModel) diag.Diagnostics {
	getResp, err := r.client.GetServiceAccountWithResponse(ctx, state.Id.ValueString())
	if err != nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Service Account", "Could not read service account ID "+state.Id.ValueString()+": "+err.Error()),
		}
	}

	if getResp.StatusCode() == 404 {
		state.Id = types.StringNull()
		return nil
	}

	if getResp.StatusCode() != 200 {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Service Account", fmt.Sprintf("Unexpected status code %d for service account ID %s: %s", getResp.StatusCode(), state.Id.ValueString(), string(getResp.Body))),
		}
	}

	if getResp.JSON200 == nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Service Account", "Received empty response body for service account ID "+state.Id.ValueString()),
		}
	}

	return mapServiceAccountToModel(ctx, getResp.JSON200, state)
}

// mapServiceAccountToModel maps the API response to the Terraform model.
func mapServiceAccountToModel(ctx context.Context, resp *models.ServiceAccount, state *serviceAccountResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	state.Id = types.StringPointerValue(nullableToPointer(resp.Id))
	state.CustomerId = types.StringPointerValue(resp.CustomerId)
	state.Name = types.StringValue(resp.Name)
	state.Description = types.StringValue(resp.Description)
	state.CreatedBy = types.StringPointerValue(nullableToPointer(resp.CreatedBy))
	state.CreatedByEmail = types.StringPointerValue(nullableToPointer(resp.CreatedByEmail))
	state.CreatedByUserId = types.StringPointerValue(nullableToPointer(resp.CreatedByUserId))
	state.Etag = types.StringPointerValue(nullableToPointer(resp.Etag))

	if t := nullableToPointer(resp.CreateTime); t != nil {
		state.CreateTime = types.StringValue(t.Format(time.RFC3339))
	} else {
		state.CreateTime = types.StringNull()
	}

	if t := nullableToPointer(resp.UpdateTime); t != nil {
		state.UpdateTime = types.StringValue(t.Format(time.RFC3339))
	} else {
		state.UpdateTime = types.StringNull()
	}

	if len(resp.Permissions) > 0 {
		listVal, listDiags := types.ListValueFrom(ctx, types.StringType, resp.Permissions)
		diags.Append(listDiags...)
		state.Permissions = listVal
	} else {
		listVal, listDiags := types.ListValueFrom(ctx, types.StringType, []string{})
		diags.Append(listDiags...)
		state.Permissions = listVal
	}

	return diags
}

// overlayServiceAccountComputedFields uses the two-phase overlay pattern to reconcile
// the Terraform plan with the API response after Create/Update.
//
// Phase 1 (Resolve): Build a fully-resolved state from the API response using
// mapServiceAccountToModel — the same mapping function used by Read.
//
// Phase 2 (Overlay): Walk the plan field-by-field. Known values are preserved.
// Unknown values are replaced with the resolved counterpart.
func overlayServiceAccountComputedFields(ctx context.Context, apiResp *models.ServiceAccount, plan *serviceAccountResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	// Phase 1: Build fully-resolved state from API response.
	resolved := *plan
	diags.Append(mapServiceAccountToModel(ctx, apiResp, &resolved)...)
	if diags.HasError() {
		return diags
	}

	// Phase 2: Overlay Computed-only fields unconditionally from resolved.
	plan.Id = resolved.Id
	plan.CustomerId = resolved.CustomerId
	plan.CreatedBy = resolved.CreatedBy
	plan.CreatedByEmail = resolved.CreatedByEmail
	plan.CreatedByUserId = resolved.CreatedByUserId
	plan.CreateTime = resolved.CreateTime

	// update_time and etag: Computed-only but change server-side on every Update.
	// On Create the plan value is Unknown -> overlay fills from API.
	// On Update the framework copies the prior state into the plan.
	// Unconditional assignment would cause "inconsistent result" if the plan had a known value.
	if plan.UpdateTime.IsUnknown() {
		plan.UpdateTime = resolved.UpdateTime //nolint:overlaycheck // guarded to avoid inconsistent result on Update
	}
	if plan.Etag.IsUnknown() {
		plan.Etag = resolved.Etag //nolint:overlaycheck // guarded to avoid inconsistent result on Update
	}

	// Optional+Computed clearable fields:
	if plan.Description.IsUnknown() {
		plan.Description = resolved.Description
	}
	if plan.Permissions.IsUnknown() {
		plan.Permissions = resolved.Permissions
	}

	// Name: Required — never touch.

	return diags
}

// toCreateRequest converts the TF model to a CreateServiceAccountRequest.
func (plan *serviceAccountResourceModel) toCreateRequest(ctx context.Context) (models.CreateServiceAccountJSONRequestBody, diag.Diagnostics) {
	var diags diag.Diagnostics

	req := models.CreateServiceAccountJSONRequestBody{
		Name: plan.Name.ValueString(),
	}

	if !plan.Description.IsUnknown() && !plan.Description.IsNull() {
		req.Description = new(plan.Description.ValueString())
	}

	if !plan.Permissions.IsUnknown() && !plan.Permissions.IsNull() {
		var perms []string
		diags.Append(plan.Permissions.ElementsAs(ctx, &perms, false)...)
		if diags.HasError() {
			return req, diags
		}
		req.Permissions = &perms
	}

	return req, diags
}

// toUpdateRequest converts the TF model to an UpdateServiceAccountRequest using RFC 7396 merge patch semantics.
func (plan *serviceAccountResourceModel) toUpdateRequest(ctx context.Context) (models.UpdateServiceAccountApplicationMergePatchPlusJSONRequestBody, diag.Diagnostics) {
	var diags diag.Diagnostics

	req := models.UpdateServiceAccountApplicationMergePatchPlusJSONRequestBody{
		Name: new(plan.Name.ValueString()),
	}

	// description: Category A clearable string.
	// Upstream API: "null leaves it unchanged; an explicit empty string clears it."
	if !plan.Description.IsUnknown() {
		if plan.Description.IsNull() || plan.Description.ValueString() == "" {
			req.Description = valueToNullable("")
		} else {
			req.Description = valueToNullable(plan.Description.ValueString())
		}
	}

	// permissions: Category A clearable list.
	// Upstream API: "permissions replaces the whole list, and [] removes every permission."
	if !plan.Permissions.IsUnknown() {
		if plan.Permissions.IsNull() || len(plan.Permissions.Elements()) == 0 {
			req.Permissions = valueToNullable([]string{})
		} else {
			var perms []string
			diags.Append(plan.Permissions.ElementsAs(ctx, &perms, false)...)
			if diags.HasError() {
				return req, diags
			}
			req.Permissions = valueToNullable(perms)
		}
	}

	return req, diags
}
