package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_role"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type roleResourceModel struct {
	resource_role.RoleModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

// populateState fetches the role from the API and populates the Terraform state.
// On 404, state.Id is set to null to signal Terraform to remove the resource from state.
func (r *roleResource) populateState(ctx context.Context, state *roleResourceModel) diag.Diagnostics {
	getResp, err := r.client.GetRoleWithResponse(ctx, state.Id.ValueString())
	if err != nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Role", "Could not read role ID "+state.Id.ValueString()+": "+err.Error()),
		}
	}

	if getResp.StatusCode() == 404 {
		state.Id = types.StringNull()
		return nil
	}

	if getResp.StatusCode() != 200 {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Role", fmt.Sprintf("Unexpected status code %d for role ID %s: %s", getResp.StatusCode(), state.Id.ValueString(), string(getResp.Body))),
		}
	}

	if getResp.JSON200 == nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Role", "Received empty response body for role ID "+state.Id.ValueString()),
		}
	}

	return mapRoleToModel(ctx, getResp.JSON200, state)
}

// mapRoleToModel maps the API response to the Terraform model.
func mapRoleToModel(ctx context.Context, resp *models.Role, state *roleResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	state.Id = types.StringValue(resp.Id)
	state.Name = types.StringValue(resp.Name)
	state.Description = types.StringValue(resp.Description)
	state.Type = types.StringValue(string(resp.Type))
	state.Customer = types.StringValue(resp.Customer)
	state.ChildTenantEligible = types.BoolValue(resp.ChildTenantEligible)

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

// overlayRoleComputedFields uses the two-phase overlay pattern to reconcile
// the Terraform plan with the API response after Create/Update.
//
// Phase 1 (Resolve): Build a fully-resolved state from the API response using
// mapRoleToModel — the same mapping function used by Read.
//
// Phase 2 (Overlay): Walk the plan field-by-field. Known values are preserved.
// Unknown values are replaced with the resolved counterpart.
func overlayRoleComputedFields(ctx context.Context, apiResp *models.Role, plan *roleResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	// Phase 1: Build fully-resolved state from API response.
	resolved := *plan
	diags.Append(mapRoleToModel(ctx, apiResp, &resolved)...)
	if diags.HasError() {
		return diags
	}

	// Phase 2: Overlay Computed-only fields unconditionally from resolved.
	plan.Id = resolved.Id
	plan.Type = resolved.Type
	plan.Customer = resolved.Customer
	plan.ChildTenantEligible = resolved.ChildTenantEligible

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

// toCreateRequest converts the TF model to a CreateRoleRequest.
func (plan *roleResourceModel) toCreateRequest(ctx context.Context) (models.CreateRoleJSONRequestBody, diag.Diagnostics) {
	var diags diag.Diagnostics

	req := models.CreateRoleJSONRequestBody{
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

// toUpdateRequest converts the TF model to an UpdateRoleRequest. The API only
// changes fields present in the body, so clearing must send explicit empty values.
func (plan *roleResourceModel) toUpdateRequest(ctx context.Context) (models.UpdateRoleJSONRequestBody, diag.Diagnostics) {
	var diags diag.Diagnostics

	req := models.UpdateRoleJSONRequestBody{
		Name: new(plan.Name.ValueString()),
	}

	// description: Category A clearable string.
	// Upstream API: "An empty string clears it; omission leaves it unchanged."
	if !plan.Description.IsUnknown() {
		req.Description = new(plan.Description.ValueString())
	}

	// permissions: Category A clearable list.
	// Upstream API: "An empty array clears all permissions; omission leaves them unchanged."
	if !plan.Permissions.IsUnknown() {
		perms := []string{}
		if !plan.Permissions.IsNull() {
			diags.Append(plan.Permissions.ElementsAs(ctx, &perms, false)...)
			if diags.HasError() {
				return req, diags
			}
		}
		req.Permissions = &perms
	}

	return req, diags
}
