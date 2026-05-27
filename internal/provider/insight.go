// Package provider implements the DoiT Terraform provider.
package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// populateState fetches the insight from the API and populates the Terraform state.
// On 404, state.InsightKey is set to null to signal Terraform to remove the resource from state.
func (r *insightResource) populateState(ctx context.Context, state *insightResourceModel) diag.Diagnostics {
	sourceID := state.SourceId.ValueString()
	insightKey := state.InsightKey.ValueString()

	// Handle import: derive defaults when fields are empty
	if sourceID == "" {
		sourceID = string(models.PostInsightResultParamsSourceIDPublicApi)
	}
	if insightKey == "" {
		insightKey = state.Key.ValueString()
	}

	getResp, err := r.client.GetInsightResultWithResponse(ctx, sourceID, insightKey)
	if err != nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Insight", fmt.Sprintf("Could not read insight %s/%s: %s", sourceID, insightKey, err.Error())),
		}
	}

	if getResp.StatusCode() == 404 {
		state.InsightKey = types.StringNull()
		return nil
	}

	if getResp.StatusCode() != 200 || getResp.JSON200 == nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Insight", fmt.Sprintf("Unexpected status %d for insight %s/%s: %s", getResp.StatusCode(), sourceID, insightKey, string(getResp.Body))),
		}
	}

	return mapInsightRespToResourceModel(ctx, getResp.JSON200, state)
}
