package provider

import (
	"context"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_ps4c_commitment_policy"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// mapCommitmentPolicyDetailToModel maps a GetCommitmentPolicy API response onto
// the ps4c_commitment_policy data source model.
func mapCommitmentPolicyDetailToModel(ctx context.Context, apiResp *models.CommitmentPolicyDetail, data *ps4cCommitmentPolicyDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	data.Id = types.StringValue(apiResp.Id)
	data.PolicyId = types.StringValue(apiResp.Id)
	data.Name = types.StringValue(apiResp.Name)
	data.IsBuiltIn = types.BoolValue(apiResp.IsBuiltIn)
	data.TargetCoverage = types.Float64Value(apiResp.TargetCoverage)
	data.LookbackDays = types.Int64Value(int64(apiResp.LookbackDays))
	data.LadderStepPercent = types.Float64Value(apiResp.LadderStepPercent)
	data.LadderIntervalDays = types.Int64Value(int64(apiResp.LadderIntervalDays))
	data.BootstrapPercent = types.Float64Value(apiResp.BootstrapPercent)

	if createdTime := nullableToPointer(apiResp.CreatedTime); createdTime != nil {
		data.CreatedTime = types.StringValue(createdTime.UTC().Format(time.RFC3339))
	} else {
		data.CreatedTime = types.StringNull()
	}

	if updatedTime := nullableToPointer(apiResp.UpdatedTime); updatedTime != nil {
		data.UpdatedTime = types.StringValue(updatedTime.UTC().Format(time.RFC3339))
	} else {
		data.UpdatedTime = types.StringNull()
	}

	assignments, d := mapCommitmentPolicyAssignmentsToList(ctx, apiResp.Assignments)
	diags.Append(d...)
	data.Assignments = assignments

	return diags
}

// mapCommitmentPolicyAssignmentsToList converts a slice of CommitmentPolicyAssignment
// into a types.List of AssignmentsValue objects. Empty slices return an empty list, not null.
func mapCommitmentPolicyAssignmentsToList(ctx context.Context, assignments []models.CommitmentPolicyAssignment) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := datasource_ps4c_commitment_policy.AssignmentsValue{}.Type(ctx)
	attrTypes := datasource_ps4c_commitment_policy.AssignmentsValue{}.AttributeTypes(ctx)

	if len(assignments) == 0 {
		return types.ListValueMust(elemType, []attr.Value{}), diags
	}

	vals := make([]attr.Value, 0, len(assignments))
	for _, a := range assignments {
		var regionVal types.String
		if r := nullableToPointer(a.Region); r != nil {
			regionVal = types.StringValue(*r)
		} else {
			regionVal = types.StringNull()
		}

		attrs := map[string]attr.Value{
			"account_id": types.StringValue(a.AccountId),
			"cloud":      types.StringValue(string(a.Cloud)),
			"region":     regionVal,
			"service":    types.StringValue(string(a.Service)),
		}
		val, d := datasource_ps4c_commitment_policy.NewAssignmentsValue(attrTypes, attrs)
		diags.Append(d...)
		vals = append(vals, val)
	}

	list, d := types.ListValue(elemType, vals)
	diags.Append(d...)
	return list, diags
}
