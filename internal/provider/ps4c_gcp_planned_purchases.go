package provider

import (
	"context"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_ps4c_gcp_planned_purchases"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// mapGcpPlannedPurchasesToItemsList maps a ListGcpPlannedPurchases API response's
// items into the ps4c_gcp_planned_purchases data source's items list attribute.
func mapGcpPlannedPurchasesToItemsList(ctx context.Context, purchases []models.GcpPlannedPurchaseByService) (types.List, diag.Diagnostics) {
	elemType := datasource_ps4c_gcp_planned_purchases.ItemsValue{}.Type(ctx)
	attrTypes := datasource_ps4c_gcp_planned_purchases.ItemsValue{}.AttributeTypes(ctx)

	return buildGcpPlannedPurchasesObjectList(elemType, attrTypes, len(purchases),
		func(i int) (map[string]attr.Value, diag.Diagnostics) {
			return gcpPlannedPurchaseItemAttrs(ctx, purchases[i])
		},
		datasource_ps4c_gcp_planned_purchases.NewItemsValue,
	)
}

func gcpPlannedPurchaseItemAttrs(ctx context.Context, purchase models.GcpPlannedPurchaseByService) (map[string]attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics

	regionsList, d := mapGcpPlannedPurchasesRegions(ctx, purchase.Regions)
	diags.Append(d...)

	return map[string]attr.Value{
		"billing_account_id": types.StringValue(purchase.BillingAccountId),
		"regions":            regionsList,
		"service":            types.StringValue(string(purchase.Service)),
	}, diags
}

func mapGcpPlannedPurchasesRegions(ctx context.Context, regions []models.GcpPlannedPurchase) (types.List, diag.Diagnostics) {
	elemType := datasource_ps4c_gcp_planned_purchases.RegionsValue{}.Type(ctx)
	attrTypes := datasource_ps4c_gcp_planned_purchases.RegionsValue{}.AttributeTypes(ctx)

	return buildGcpPlannedPurchasesObjectList(elemType, attrTypes, len(regions),
		func(i int) (map[string]attr.Value, diag.Diagnostics) {
			return gcpPlannedPurchaseRegionAttrs(ctx, regions[i])
		},
		datasource_ps4c_gcp_planned_purchases.NewRegionsValue,
	)
}

func gcpPlannedPurchaseRegionAttrs(ctx context.Context, region models.GcpPlannedPurchase) (map[string]attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics

	estimatedSavings, d := buildGcpMoneyPtr(
		region.EstimatedSavings,
		datasource_ps4c_gcp_planned_purchases.EstimatedSavingsValue{}.AttributeTypes(ctx),
		datasource_ps4c_gcp_planned_purchases.NewEstimatedSavingsValue,
		datasource_ps4c_gcp_planned_purchases.NewEstimatedSavingsValueNull,
	)
	diags.Append(d...)

	finalCommitment, d := buildGcpMoneyPtr(
		region.FinalCommitment,
		datasource_ps4c_gcp_planned_purchases.FinalCommitmentValue{}.AttributeTypes(ctx),
		datasource_ps4c_gcp_planned_purchases.NewFinalCommitmentValue,
		datasource_ps4c_gcp_planned_purchases.NewFinalCommitmentValueNull,
	)
	diags.Append(d...)

	planningCycleEndDate := types.StringNull()
	if d := nullableToPointer(region.PlanningCycleEndDate); d != nil {
		planningCycleEndDate = types.StringValue(d.String())
	}

	planningCycleStartDate := types.StringNull()
	if d := nullableToPointer(region.PlanningCycleStartDate); d != nil {
		planningCycleStartDate = types.StringValue(d.String())
	}

	profile := types.StringPointerValue(region.Profile)

	purchaseApprovalStatus := types.StringNull()
	if region.PurchaseApprovalStatus != nil {
		purchaseApprovalStatus = types.StringValue(string(*region.PurchaseApprovalStatus))
	}

	stepsList, d := mapGcpPurchasePlanSteps(ctx, region.Steps)
	diags.Append(d...)

	term := types.StringNull()
	if region.Term != nil {
		term = types.StringValue(string(*region.Term))
	}

	weeksToTarget := types.Int64Null()
	if w := nullableToPointer(region.WeeksToTarget); w != nil {
		weeksToTarget = types.Int64Value(int64(*w))
	}

	return map[string]attr.Value{
		"estimated_savings":         estimatedSavings,
		"final_commitment":          finalCommitment,
		"pause_note":                types.StringPointerValue(nullableToPointer(region.PauseNote)),
		"planning_cycle_end_date":   planningCycleEndDate,
		"planning_cycle_start_date": planningCycleStartDate,
		"profile":                   profile,
		"purchase_approval_status":  purchaseApprovalStatus,
		"region":                    types.StringValue(region.Region),
		"requires_approval":         types.BoolPointerValue(region.RequiresApproval),
		"status":                    types.StringValue(string(region.Status)),
		"steps":                     stepsList,
		"term":                      term,
		"weeks_to_target":           weeksToTarget,
		"wow_violation":             types.BoolPointerValue(region.WowViolation),
	}, diags
}

func mapGcpPurchasePlanSteps(ctx context.Context, steps *[]models.GcpPurchasePlanStep) (types.List, diag.Diagnostics) {
	var stepItems []models.GcpPurchasePlanStep
	if steps != nil {
		stepItems = *steps
	}

	elemType := datasource_ps4c_gcp_planned_purchases.StepsValue{}.Type(ctx)
	attrTypes := datasource_ps4c_gcp_planned_purchases.StepsValue{}.AttributeTypes(ctx)

	return buildGcpPlannedPurchasesObjectList(elemType, attrTypes, len(stepItems),
		func(i int) (map[string]attr.Value, diag.Diagnostics) {
			return gcpPurchasePlanStepAttrs(ctx, stepItems[i])
		},
		datasource_ps4c_gcp_planned_purchases.NewStepsValue,
	)
}

func gcpPurchasePlanStepAttrs(ctx context.Context, step models.GcpPurchasePlanStep) (map[string]attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics

	cumulativeCommitment, d := buildGcpMoneyVal(
		step.CumulativeCommitment,
		datasource_ps4c_gcp_planned_purchases.CumulativeCommitmentValue{}.AttributeTypes(ctx),
		datasource_ps4c_gcp_planned_purchases.NewCumulativeCommitmentValue,
	)
	diags.Append(d...)

	estimatedSavings, d := buildGcpMoneyVal(
		step.EstimatedSavings,
		datasource_ps4c_gcp_planned_purchases.EstimatedSavingsValue{}.AttributeTypes(ctx),
		datasource_ps4c_gcp_planned_purchases.NewEstimatedSavingsValue,
	)
	diags.Append(d...)

	purchaseAmount, d := buildGcpMoneyVal(
		step.PurchaseAmount,
		datasource_ps4c_gcp_planned_purchases.PurchaseAmountValue{}.AttributeTypes(ctx),
		datasource_ps4c_gcp_planned_purchases.NewPurchaseAmountValue,
	)
	diags.Append(d...)

	return map[string]attr.Value{
		"cumulative_commitment": cumulativeCommitment,
		"estimated_savings":     estimatedSavings,
		"is_bootstrap":          types.BoolValue(step.IsBootstrap),
		"is_final":              types.BoolValue(step.IsFinal),
		"order":                 types.Int64Value(int64(step.Order)),
		"purchase_amount":       purchaseAmount,
		"requires_approval":     types.BoolValue(step.RequiresApproval),
		"scheduled_date":        types.StringValue(step.ScheduledDate.String()),
	}, diags
}

func moneyAttrsPlannedPurchases(amount, currency string) map[string]attr.Value {
	return map[string]attr.Value{
		"amount":   types.StringValue(amount),
		"currency": types.StringValue(currency),
	}
}

func buildGcpMoneyPtr[V attr.Value](
	m *models.Money,
	attrTypes map[string]attr.Type,
	newFn func(map[string]attr.Type, map[string]attr.Value) (V, diag.Diagnostics),
	newNullFn func() V,
) (V, diag.Diagnostics) {
	if m == nil {
		return newNullFn(), nil
	}
	return newFn(attrTypes, moneyAttrsPlannedPurchases(m.Amount, m.Currency))
}

func buildGcpMoneyVal[V attr.Value](
	m models.Money,
	attrTypes map[string]attr.Type,
	newFn func(map[string]attr.Type, map[string]attr.Value) (V, diag.Diagnostics),
) (V, diag.Diagnostics) {
	return newFn(attrTypes, moneyAttrsPlannedPurchases(m.Amount, m.Currency))
}

func buildGcpPlannedPurchasesObjectList[V attr.Value](
	elemType attr.Type,
	attrTypes map[string]attr.Type,
	n int,
	attrsAt func(i int) (map[string]attr.Value, diag.Diagnostics),
	newFn func(map[string]attr.Type, map[string]attr.Value) (V, diag.Diagnostics),
) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics

	if n == 0 {
		return types.ListValueMust(elemType, []attr.Value{}), diags
	}

	vals := make([]attr.Value, 0, n)
	for i := range n {
		a, d := attrsAt(i)
		diags.Append(d...)
		v, d := newFn(attrTypes, a)
		diags.Append(d...)
		vals = append(vals, v)
	}

	list, d := types.ListValue(elemType, vals)
	diags.Append(d...)
	return list, diags
}
