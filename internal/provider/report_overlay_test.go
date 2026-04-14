package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	resource_report "github.com/doitintl/terraform-provider-doit/internal/provider/resource_report"
)

// TestOverlayAdvancedAnalysis_NullResolved verifies that when the API doesn't
// return an advanced_analysis object (resolved is null), Unknown plan subfields
// are resolved to explicit null values instead of zero-valued Go struct fields.
func TestOverlayAdvancedAnalysis_NullResolved(t *testing.T) {
	ctx := context.Background()

	resolved := resource_report.NewAdvancedAnalysisValueNull()

	// Plan where the user set forecast=true but omitted the other booleans (Unknown).
	plan := resource_report.NewAdvancedAnalysisValueMust(
		resource_report.AdvancedAnalysisValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"forecast":      types.BoolValue(true),
			"not_trending":  types.BoolUnknown(),
			"trending_down": types.BoolUnknown(),
			"trending_up":   types.BoolUnknown(),
		},
	)

	overlayAdvancedAnalysis(&resolved, &plan)

	// Known plan values must be preserved.
	if !plan.Forecast.ValueBool() {
		t.Errorf("expected Forecast to remain true, got %v", plan.Forecast)
	}

	// Unknown values must be resolved to explicit null, not remain Unknown.
	for name, val := range map[string]types.Bool{
		"NotTrending":  plan.NotTrending,
		"TrendingDown": plan.TrendingDown,
		"TrendingUp":   plan.TrendingUp,
	} {
		if val.IsUnknown() {
			t.Errorf("%s should not be Unknown after overlay with null resolved", name)
		}
		if !val.IsNull() {
			t.Errorf("%s should be null when resolved is null, got %v", name, val)
		}
	}
}

// TestOverlayAdvancedAnalysis_KnownResolved verifies that when the API returns
// a valid advanced_analysis object, Unknown plan subfields pick up the API values.
func TestOverlayAdvancedAnalysis_KnownResolved(t *testing.T) {
	ctx := context.Background()

	resolved := resource_report.NewAdvancedAnalysisValueMust(
		resource_report.AdvancedAnalysisValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"forecast":      types.BoolValue(false),
			"not_trending":  types.BoolValue(true),
			"trending_down": types.BoolValue(false),
			"trending_up":   types.BoolValue(true),
		},
	)

	plan := resource_report.NewAdvancedAnalysisValueMust(
		resource_report.AdvancedAnalysisValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"forecast":      types.BoolValue(true), // user set this — must be preserved
			"not_trending":  types.BoolUnknown(),   // user omitted — pick up API value
			"trending_down": types.BoolUnknown(),
			"trending_up":   types.BoolUnknown(),
		},
	)

	overlayAdvancedAnalysis(&resolved, &plan)

	if !plan.Forecast.ValueBool() {
		t.Errorf("expected Forecast to remain true (user-configured), got %v", plan.Forecast)
	}
	if !plan.NotTrending.ValueBool() {
		t.Errorf("expected NotTrending to be true (from API), got %v", plan.NotTrending)
	}
	if plan.TrendingDown.ValueBool() {
		t.Errorf("expected TrendingDown to be false (from API), got %v", plan.TrendingDown)
	}
	if !plan.TrendingUp.ValueBool() {
		t.Errorf("expected TrendingUp to be true (from API), got %v", plan.TrendingUp)
	}
}

// TestOverlayTimeRange_NullResolved verifies the null guard for time_range.
func TestOverlayTimeRange_NullResolved(t *testing.T) {
	ctx := context.Background()

	resolved := resource_report.NewTimeRangeValueNull()

	plan := resource_report.NewTimeRangeValueMust(
		resource_report.TimeRangeValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"mode":            types.StringValue("last"),
			"unit":            types.StringValue("month"),
			"amount":          types.Int64Unknown(),
			"include_current": types.BoolUnknown(),
		},
	)

	overlayTimeRange(&resolved, &plan)

	// Known values preserved.
	if plan.Mode.ValueString() != "last" {
		t.Errorf("expected Mode to remain 'last', got %q", plan.Mode.ValueString())
	}
	if plan.Unit.ValueString() != "month" {
		t.Errorf("expected Unit to remain 'month', got %q", plan.Unit.ValueString())
	}

	// Unknown values resolved to null, not left Unknown.
	if plan.Amount.IsUnknown() {
		t.Error("Amount should not be Unknown after overlay with null resolved")
	}
	if !plan.Amount.IsNull() {
		t.Errorf("Amount should be null when resolved is null, got %v", plan.Amount)
	}
	if plan.IncludeCurrent.IsUnknown() {
		t.Error("IncludeCurrent should not be Unknown after overlay with null resolved")
	}
	if !plan.IncludeCurrent.IsNull() {
		t.Errorf("IncludeCurrent should be null when resolved is null, got %v", plan.IncludeCurrent)
	}
}

// TestOverlayCustomTimeRange_NullResolved verifies the null guard for custom_time_range.
func TestOverlayCustomTimeRange_NullResolved(t *testing.T) {
	ctx := context.Background()

	resolved := resource_report.NewCustomTimeRangeValueNull()

	plan := resource_report.NewCustomTimeRangeValueMust(
		resource_report.CustomTimeRangeValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"from": types.StringUnknown(),
			"to":   types.StringUnknown(),
		},
	)

	overlayCustomTimeRange(&resolved, &plan)

	if plan.From.IsUnknown() {
		t.Error("From should not be Unknown after overlay with null resolved")
	}
	if !plan.From.IsNull() {
		t.Errorf("From should be null when resolved is null, got %v", plan.From)
	}
	if plan.To.IsUnknown() {
		t.Error("To should not be Unknown after overlay with null resolved")
	}
	if !plan.To.IsNull() {
		t.Errorf("To should be null when resolved is null, got %v", plan.To)
	}
}

// TestOverlayMetric_NullResolved verifies the null guard for metric.
func TestOverlayMetric_NullResolved(t *testing.T) {
	ctx := context.Background()

	resolved := resource_report.NewMetricValueNull()

	plan := resource_report.NewMetricValueMust(
		resource_report.MetricValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"type":  types.StringValue("basic"),
			"value": types.StringUnknown(),
		},
	)

	overlayMetric(&resolved, &plan)

	if plan.MetricType.ValueString() != "basic" {
		t.Errorf("expected MetricType to remain 'basic', got %q", plan.MetricType.ValueString())
	}
	if plan.Value.IsUnknown() {
		t.Error("Value should not be Unknown after overlay with null resolved")
	}
	if !plan.Value.IsNull() {
		t.Errorf("Value should be null when resolved is null, got %v", plan.Value)
	}
}

// TestOverlayLimit_NullResolved verifies the null guard for limit.
func TestOverlayLimit_NullResolved(t *testing.T) {
	ctx := context.Background()

	resolved := resource_report.NewLimitValueNull()

	plan := resource_report.NewLimitValueMust(
		resource_report.LimitValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"sort":   types.StringUnknown(),
			"value":  types.Int64Unknown(),
			"metric": resource_report.NewMetricValueNull(),
		},
	)

	overlayLimit(&resolved, &plan)

	if plan.Sort.IsUnknown() {
		t.Error("Sort should not be Unknown after overlay with null resolved")
	}
	if !plan.Sort.IsNull() {
		t.Errorf("Sort should be null when resolved is null, got %v", plan.Sort)
	}
	if plan.Value.IsUnknown() {
		t.Error("Value should not be Unknown after overlay with null resolved")
	}
	if !plan.Value.IsNull() {
		t.Errorf("Value should be null when resolved is null, got %v", plan.Value)
	}
}

// TestOverlayListElements_NullResolvedElement verifies that when a resolved list
// element is null (API returned null in the list), the overlay skips that element
// rather than walking into zero-valued struct fields.
func TestOverlayListElements_NullResolvedElement(t *testing.T) {
	ctx := context.Background()

	// Build a plan list with one filter element that has an Unknown subfield.
	planFilter := resource_report.NewFiltersValueMust(
		resource_report.FiltersValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"id":               types.StringValue("sku_description"),
			"type":             types.StringValue("fixed"),
			"mode":             types.StringValue("include"),
			"case_insensitive": types.BoolUnknown(), // user omitted
			"include_null":     types.BoolValue(false),
			"inverse":          types.BoolValue(false),
			"values":           types.ListValueMust(types.StringType, []attr.Value{types.StringValue("test")}),
		},
	)

	planList, diags := types.ListValueFrom(ctx, planFilter.Type(ctx), []resource_report.FiltersValue{planFilter})
	if diags.HasError() {
		t.Fatalf("failed to build plan list: %v", diags)
	}

	// Build a resolved list with a null element at the same index.
	resolvedFilter := resource_report.NewFiltersValueNull()
	resolvedList, diags := types.ListValueFrom(ctx, resolvedFilter.Type(ctx), []resource_report.FiltersValue{resolvedFilter})
	if diags.HasError() {
		t.Fatalf("failed to build resolved list: %v", diags)
	}

	// Track whether overlayFn was called — it should NOT be for a null resolved element.
	called := false
	overlayDiags := overlayListElements(ctx, &resolvedList, &planList, func(_ context.Context, resolved, plan *resource_report.FiltersValue) diag.Diagnostics {
		called = true
		return nil
	})
	if overlayDiags.HasError() {
		t.Fatalf("overlayListElements returned errors: %v", overlayDiags)
	}
	if called {
		t.Error("overlayFn should not be called when resolved element is null")
	}

	// Verify the plan element is untouched — CaseInsensitive should still be Unknown.
	var result []resource_report.FiltersValue
	resultDiags := planList.ElementsAs(ctx, &result, true)
	if resultDiags.HasError() {
		t.Fatalf("failed to decode plan list elements: %v", resultDiags)
	}
	if !result[0].CaseInsensitive.IsUnknown() {
		t.Errorf("CaseInsensitive should remain Unknown when resolved element is null, got %v", result[0].CaseInsensitive)
	}
	// Known values should be preserved.
	if result[0].Id.ValueString() != "sku_description" {
		t.Errorf("Id should remain 'sku_description', got %q", result[0].Id.ValueString())
	}
}
