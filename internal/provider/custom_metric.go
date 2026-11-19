package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_custom_metric"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type customMetricResourceModel struct {
	resource_custom_metric.CustomMetricModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

// populateState fetches the custom metric from the API and populates the Terraform state.
// On 404, state.Id is set to null to signal Terraform to remove the resource from state.
func (r *customMetricResource) populateState(ctx context.Context, state *customMetricResourceModel) diag.Diagnostics {
	id := state.Id.ValueString()

	getResp, err := r.client.GetCustomMetricWithResponse(ctx, id, &models.GetCustomMetricParams{})
	if err != nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Custom Metric", fmt.Sprintf("Could not read custom metric %s: %s", id, err)),
		}
	}

	if getResp.StatusCode() == 404 {
		state.Id = types.StringNull()
		return nil
	}

	if getResp.StatusCode() != 200 {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Custom Metric", fmt.Sprintf("Unexpected status code %d for custom metric %s: %s", getResp.StatusCode(), id, string(getResp.Body))),
		}
	}

	if getResp.JSON200 == nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Custom Metric", fmt.Sprintf("Received empty response body for custom metric %s", id)),
		}
	}

	return mapCustomMetricToModel(ctx, getResp.JSON200, state)
}

// mapCustomMetricToModel maps the API response to the Terraform model. The API
// returns name, description and formula exactly as sent, so no prior-state
// reconciliation is needed.
func mapCustomMetricToModel(ctx context.Context, resp *models.CustomMetric, state *customMetricResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	state.Id = types.StringPointerValue(resp.Id)
	state.Etag = types.StringPointerValue(resp.Etag)
	state.Name = types.StringValue(resp.Name)
	state.Description = types.StringValue(resp.Description)
	state.Formula = types.StringValue(resp.Formula)
	state.UnitType = types.StringValue(string(resp.UnitType))

	state.Owner = types.StringNull()
	if resp.Owner != nil {
		state.Owner = types.StringValue(string(*resp.Owner))
	}

	state.CreateTime = types.StringNull()
	if resp.CreateTime != nil {
		state.CreateTime = types.StringValue(resp.CreateTime.Format(time.RFC3339Nano))
	}
	state.UpdateTime = types.StringNull()
	if resp.UpdateTime != nil {
		state.UpdateTime = types.StringValue(resp.UpdateTime.Format(time.RFC3339Nano))
	}

	decimals, d := buildCustomMetricDecimals(ctx, nullableToPointer(resp.Decimals))
	diags.Append(d...)
	state.Decimals = decimals

	variables, d := buildCustomMetricVariables(ctx, resp.Variables)
	diags.Append(d...)
	state.Variables = variables

	return diags
}

// buildCustomMetricDecimals maps decimals. The API never fills it in: a metric
// without display precision returns null.
func buildCustomMetricDecimals(ctx context.Context, decimals *models.CustomMetricDecimals) (resource_custom_metric.DecimalsValue, diag.Diagnostics) {
	if decimals == nil {
		return resource_custom_metric.NewDecimalsValueNull(), nil
	}
	return resource_custom_metric.NewDecimalsValue(
		resource_custom_metric.DecimalsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"min": types.Int64Value(int64(decimals.Min)),
			"max": types.Int64Value(int64(decimals.Max)),
		},
	)
}

// buildCustomMetricVariables maps the variables list, in letter order.
func buildCustomMetricVariables(ctx context.Context, variables []models.CustomMetricVariable) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics

	elemType := resource_custom_metric.VariablesValue{}.Type(ctx)
	values := make([]resource_custom_metric.VariablesValue, 0, len(variables))
	for _, v := range variables {
		metric, d := resource_custom_metric.NewMetricValue(
			resource_custom_metric.MetricValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"type":  types.StringValue(string(v.Metric.Type)),
				"value": types.StringValue(v.Metric.Value),
			},
		)
		diags.Append(d...)
		if d.HasError() {
			return types.ListNull(elemType), diags
		}

		variable, d := resource_custom_metric.NewVariablesValue(
			resource_custom_metric.VariablesValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"allocation_id": types.StringValue(v.AllocationId),
				"letter":        types.StringPointerValue(v.Letter),
				"metric":        metric,
			},
		)
		diags.Append(d...)
		if d.HasError() {
			return types.ListNull(elemType), diags
		}
		values = append(values, variable)
	}

	list, d := types.ListValueFrom(ctx, elemType, values)
	diags.Append(d...)
	return list, diags
}

// overlayCustomMetricComputedFields uses the two-phase overlay pattern to reconcile
// the Terraform plan with the API response after Create/Update.
//
// Phase 1 (Resolve): Build a fully-resolved state from the API response using
// mapCustomMetricToModel — the same mapping function used by Read.
//
// Phase 2 (Overlay): Walk the plan field-by-field. Known values are preserved.
// Unknown values are replaced with the resolved counterpart.
func overlayCustomMetricComputedFields(ctx context.Context, apiResp *models.CustomMetric, plan *customMetricResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	// Phase 1: Build fully-resolved state from API response.
	resolved := *plan
	diags.Append(mapCustomMetricToModel(ctx, apiResp, &resolved)...)
	if diags.HasError() {
		return diags
	}

	// Phase 2: Overlay Computed-only fields unconditionally from resolved.
	plan.Id = resolved.Id
	plan.Owner = resolved.Owner
	plan.CreateTime = resolved.CreateTime

	// update_time and etag: volatile, marked unknown on every update by ModifyPlan.
	if plan.UpdateTime.IsUnknown() {
		plan.UpdateTime = resolved.UpdateTime
	}
	if plan.Etag.IsUnknown() {
		plan.Etag = resolved.Etag
	}

	// decimals: Optional+Computed clearable object. The plan modifier plans null
	// when it is omitted, so it is only unknown for an unknown configured value.
	if plan.Decimals.IsUnknown() {
		plan.Decimals = resolved.Decimals
	}

	// variables: Required list with the Computed letter in each element.
	if plan.Variables.IsUnknown() {
		plan.Variables = resolved.Variables
	} else if !plan.Variables.IsNull() {
		diags.Append(overlayListElements(ctx, &resolved.Variables, &plan.Variables, overlayCustomMetricVariable)...)
	}

	// name, formula: Required — never touch.
	// description, unit_type: schema defaults — always known.

	return diags
}

// overlayCustomMetricVariable copies the API-assigned letter into a planned
// variable. The plan modifier already plans it from the position, so both agree.
func overlayCustomMetricVariable(_ context.Context, resolved, plan *resource_custom_metric.VariablesValue) diag.Diagnostics {
	plan.Letter = resolved.Letter
	return nil
}

// toCreateRequest converts the TF model to a CreateCustomMetric request.
func (plan *customMetricResourceModel) toCreateRequest(ctx context.Context) (models.CreateCustomMetricJSONRequestBody, diag.Diagnostics) {
	var diags diag.Diagnostics

	variables, d := toCustomMetricVariableInputs(ctx, plan.Variables)
	diags.Append(d...)
	if diags.HasError() {
		return models.CreateCustomMetricJSONRequestBody{}, diags
	}

	req := models.CreateCustomMetricJSONRequestBody{
		Name:        plan.Name.ValueString(),
		Description: new(plan.Description.ValueString()),
		Variables:   variables,
		Formula:     plan.Formula.ValueString(),
		UnitType:    new(models.CustomMetricUnitType(plan.UnitType.ValueString())),
	}

	if !plan.Decimals.IsUnknown() && !plan.Decimals.IsNull() {
		req.Decimals = new(toCustomMetricDecimals(plan.Decimals))
	}

	return req, diags
}

// toUpdateRequest converts the TF model to an UpdateCustomMetric request using
// RFC 7396 merge patch semantics. Every configurable field is sent: variables
// replaces the whole list, and the formula must match it.
func (plan *customMetricResourceModel) toUpdateRequest(ctx context.Context) (models.UpdateCustomMetricApplicationMergePatchPlusJSONRequestBody, diag.Diagnostics) {
	var diags diag.Diagnostics

	variables, d := toCustomMetricVariableInputs(ctx, plan.Variables)
	diags.Append(d...)
	if diags.HasError() {
		return models.UpdateCustomMetricApplicationMergePatchPlusJSONRequestBody{}, diags
	}

	req := models.UpdateCustomMetricApplicationMergePatchPlusJSONRequestBody{
		Name:        new(plan.Name.ValueString()),
		Description: new(plan.Description.ValueString()),
		Variables:   &variables,
		Formula:     new(plan.Formula.ValueString()),
		UnitType:    new(models.CustomMetricUnitType(plan.UnitType.ValueString())),
	}

	setCustomMetricDecimalsUpdate(&req, plan.Decimals)

	return req, diags
}

// setCustomMetricDecimalsUpdate sets decimals in an update request.
// decimals is a Category A clearable object. Upstream API: "decimals: null
// removes the display precision." An unknown value is left out.
func setCustomMetricDecimalsUpdate(req *models.UpdateCustomMetricApplicationMergePatchPlusJSONRequestBody, decimals resource_custom_metric.DecimalsValue) {
	if decimals.IsUnknown() {
		return
	}
	if decimals.IsNull() {
		req.Decimals.SetNull()
		return
	}
	req.Decimals.Set(toCustomMetricDecimals(decimals))
}

// toCustomMetricDecimals converts a known decimals object to the API model.
func toCustomMetricDecimals(decimals resource_custom_metric.DecimalsValue) models.CustomMetricDecimals {
	return models.CustomMetricDecimals{
		Min: int(decimals.Min.ValueInt64()),
		Max: int(decimals.Max.ValueInt64()),
	}
}

// toCustomMetricVariableInputs converts the variables list to the API model.
// The letters are not sent: the API assigns them from the order.
func toCustomMetricVariableInputs(ctx context.Context, list types.List) ([]models.CustomMetricVariableInput, diag.Diagnostics) {
	var diags diag.Diagnostics

	var variables []resource_custom_metric.VariablesValue
	diags.Append(list.ElementsAs(ctx, &variables, false)...)
	if diags.HasError() {
		return nil, diags
	}

	inputs := make([]models.CustomMetricVariableInput, 0, len(variables))
	for _, v := range variables {
		inputs = append(inputs, models.CustomMetricVariableInput{
			AllocationId: v.AllocationId.ValueString(),
			Metric: models.CustomMetricVariableMetric{
				Type:  models.CustomMetricVariableMetricType(v.Metric.MetricType.ValueString()),
				Value: v.Metric.Value.ValueString(),
			},
		})
	}

	return inputs, diags
}
