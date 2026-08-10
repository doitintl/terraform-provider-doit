package provider

import (
	"context"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_budget_suggestions"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// mapBudgetSuggestionsItems maps a ListBudgetSuggestions API response's items
// into the budget_suggestions data source's list attribute.
func mapBudgetSuggestionsItems(ctx context.Context, items []models.BudgetSuggestion) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics

	elemType := datasource_budget_suggestions.ItemsValue{}.Type(ctx)

	if len(items) == 0 {
		return types.ListValueMust(elemType, []attr.Value{}), diags
	}

	vals := make([]attr.Value, 0, len(items))
	for _, item := range items {
		itemAttrs, d := budgetSuggestionItemAttrs(ctx, item)
		diags.Append(d...)

		itemVal, d := datasource_budget_suggestions.NewItemsValue(
			datasource_budget_suggestions.ItemsValue{}.AttributeTypes(ctx),
			itemAttrs,
		)
		diags.Append(d...)
		vals = append(vals, itemVal)
	}

	list, d := types.ListValue(elemType, vals)
	diags.Append(d...)
	return list, diags
}

func budgetSuggestionItemAttrs(ctx context.Context, item models.BudgetSuggestion) (map[string]attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics

	alertThresholds, d := types.ListValueFrom(ctx, types.Float64Type, sliceFromPointer(item.AlertThresholds))
	diags.Append(d...)

	amount, d := mapBudgetSuggestionAmount(ctx, item.Amount)
	diags.Append(d...)

	scopeChips, d := mapBudgetSuggestionScopeChips(ctx, item.ScopeChips)
	diags.Append(d...)

	generatedTime := types.StringNull()
	if item.GeneratedTime != nil {
		generatedTime = types.StringValue(item.GeneratedTime.UTC().Format(time.RFC3339))
	}

	confidence := types.StringNull()
	if item.Confidence != nil {
		confidence = types.StringValue(string(*item.Confidence))
	}

	status := types.StringNull()
	if item.Status != nil {
		status = types.StringValue(string(*item.Status))
	}

	return map[string]attr.Value{
		"alert_thresholds": alertThresholds,
		"amount":           amount,
		"confidence":       confidence,
		"config":           mapFreeformJSON(item.Config),
		"description":      types.StringPointerValue(item.Description),
		"generated_time":   generatedTime,
		"id":               types.StringPointerValue(item.Id),
		"name":             types.StringPointerValue(item.Name),
		"rationale":        types.StringPointerValue(item.Rationale),
		"scope_chips":      scopeChips,
		"status":           status,
		"time_interval":    types.StringPointerValue(item.TimeInterval),
	}, diags
}

func mapBudgetSuggestionAmount(ctx context.Context, amount *models.BudgetSuggestionAmount) (datasource_budget_suggestions.AmountValue, diag.Diagnostics) {
	if amount == nil {
		return datasource_budget_suggestions.NewAmountValueNull(), nil
	}

	return datasource_budget_suggestions.NewAmountValue(
		datasource_budget_suggestions.AmountValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"amount":   types.StringPointerValue(amount.Amount),
			"currency": types.StringPointerValue(amount.Currency),
		},
	)
}

func mapBudgetSuggestionScopeChips(ctx context.Context, chips *[]models.BudgetSuggestionScopeChipsItem) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics

	elemType := datasource_budget_suggestions.ScopeChipsValue{}.Type(ctx)

	if chips == nil || len(*chips) == 0 {
		return types.ListValueMust(elemType, []attr.Value{}), diags
	}

	vals := make([]attr.Value, 0, len(*chips))
	for _, chip := range *chips {
		values, d := types.ListValueFrom(ctx, types.StringType, sliceFromPointer(chip.Values))
		diags.Append(d...)

		chipVal, d := datasource_budget_suggestions.NewScopeChipsValue(
			datasource_budget_suggestions.ScopeChipsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"key":    types.StringPointerValue(chip.Key),
				"values": values,
			},
		)
		diags.Append(d...)
		vals = append(vals, chipVal)
	}

	list, d := types.ListValue(elemType, vals)
	diags.Append(d...)
	return list, diags
}

func sliceFromPointer[T any](p *[]T) []T {
	if p == nil {
		return []T{}
	}
	return *p
}
