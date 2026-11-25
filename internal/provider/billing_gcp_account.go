package provider

import (
	"context"
	"fmt"

	modelsv2 "github.com/doitintl/terraform-provider-doit/internal/provider/models/v2"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_billing_gcp_account"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Problem codes of the GCP billing onboarding API that the resource handles.
const (
	billingAccountAccessMissingCode = "onboarding.billing_account_access_missing"
	billingExportNotReadyCode       = "onboarding.billing_export_not_ready"
	billingAccountBusyCode          = "onboarding.billing_account_busy"
	billingConnectionActivatingCode = "onboarding.billing_connection_activating"
)

type billingGcpAccountResourceModel struct {
	resource_billing_gcp_account.BillingGcpAccountModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

// populateState fetches the billing account connection and maps it into state.
// On 404, state.Id is set to null so that Read removes the resource.
func (r *billingGcpAccountResource) populateState(ctx context.Context, state *billingGcpAccountResourceModel) diag.Diagnostics {
	id := state.Id.ValueString()

	getResp, err := r.client.GetGcpBillingAccountWithResponse(ctx, id, &modelsv2.GetGcpBillingAccountParams{})
	if err != nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading GCP Billing Account", fmt.Sprintf("Could not read GCP billing account connection %s: %s", id, err)),
		}
	}

	if getResp.StatusCode() == 404 {
		state.Id = types.StringNull()
		return nil
	}

	if getResp.StatusCode() != 200 || getResp.JSON200 == nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading GCP Billing Account", fmt.Sprintf("Could not read GCP billing account connection %s, status: %d, body: %s", id, getResp.StatusCode(), string(getResp.Body))),
		}
	}

	mapBillingGcpAccountToModel(getResp.JSON200, state)
	return nil
}

// mapBillingGcpAccountToModel maps the API response to the Terraform model. The
// export fields are absent only on a failed connection whose export is no longer
// known; they then read as null and the configured values plan a replacement.
func mapBillingGcpAccountToModel(resp *modelsv2.GcpBillingAccount, state *billingGcpAccountResourceModel) {
	state.Id = types.StringPointerValue(resp.Id)
	state.BillingAccountId = types.StringValue(resp.BillingAccountId)
	state.ExportProjectId = types.StringPointerValue(resp.ExportProjectId)
	state.ExportDatasetId = types.StringPointerValue(resp.ExportDatasetId)
	state.FailureReason = types.StringPointerValue(resp.FailureReason)
	state.Status = types.StringNull()
	if resp.Status != nil {
		state.Status = types.StringValue(string(*resp.Status))
	}
}

// overlayBillingGcpAccountComputedFields keeps the planned values and takes the
// Computed-only fields from the API response.
func overlayBillingGcpAccountComputedFields(resp *modelsv2.GcpBillingAccount, plan *billingGcpAccountResourceModel) {
	resolved := *plan
	mapBillingGcpAccountToModel(resp, &resolved)

	plan.Id = resolved.Id
	plan.Status = resolved.Status
	plan.FailureReason = resolved.FailureReason
}

// toCreateRequest converts the plan to a connect request.
func (plan *billingGcpAccountResourceModel) toCreateRequest() modelsv2.CreateGcpBillingAccountJSONRequestBody {
	return modelsv2.CreateGcpBillingAccountJSONRequestBody{
		BillingAccountId: plan.BillingAccountId.ValueString(),
		ExportProjectId:  plan.ExportProjectId.ValueString(),
		ExportDatasetId:  plan.ExportDatasetId.ValueString(),
	}
}
