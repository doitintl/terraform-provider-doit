package provider

import (
	"context"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_cloudconnect_gcp_connection"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// gcpBigQueryFinOpsFeatureID is the feature that feature_inputs.bigquery_finops configures.
const gcpBigQueryFinOpsFeatureID = "bigquery-finops"

// cloudconnectGcpFeatureInputsValidator rejects feature_inputs.bigquery_finops
// when feature_ids does not contain "bigquery-finops". The API drops the inputs
// of a feature that is not enabled, so the configured inputs would never be
// stored and every plan would show them as a change.
type cloudconnectGcpFeatureInputsValidator struct{}

var _ resource.ConfigValidator = cloudconnectGcpFeatureInputsValidator{}

func (v cloudconnectGcpFeatureInputsValidator) Description(_ context.Context) string {
	return `feature_inputs.bigquery_finops can only be set when feature_ids contains "bigquery-finops"`
}

func (v cloudconnectGcpFeatureInputsValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v cloudconnectGcpFeatureInputsValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var featureInputs resource_cloudconnect_gcp_connection.FeatureInputsValue
	diags := req.Config.GetAttribute(ctx, path.Root("feature_inputs"), &featureInputs)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	if featureInputs.IsNull() || featureInputs.IsUnknown() {
		return
	}
	if featureInputs.BigqueryFinops.IsNull() || featureInputs.BigqueryFinops.IsUnknown() {
		return
	}

	var featureIDs types.Set
	diags = req.Config.GetAttribute(ctx, path.Root("feature_ids"), &featureIDs)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	if featureIDs.IsNull() || featureIDs.IsUnknown() {
		return
	}

	hasUnknown := false
	for _, element := range featureIDs.Elements() {
		featureID, ok := element.(types.String)
		if !ok {
			continue
		}
		if featureID.IsUnknown() {
			hasUnknown = true
			continue
		}
		if featureID.ValueString() == gcpBigQueryFinOpsFeatureID {
			return
		}
	}
	if hasUnknown {
		return
	}

	resp.Diagnostics.AddAttributeError(
		path.Root("feature_inputs").AtName("bigquery_finops"),
		"BigQuery FinOps Inputs Without the Feature",
		`feature_inputs.bigquery_finops can only be set when feature_ids contains "bigquery-finops". The API drops the inputs of a feature that is not enabled.`,
	)
}
