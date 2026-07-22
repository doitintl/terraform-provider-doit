package provider

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_report"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// overlayReportComputedFields preserves all user-configured values from the Terraform
// plan and selectively resolves only Unknown (Optional+Computed) fields from the API response.
//
// Strategy: Two-phase approach
//  1. Use mapReportToModel to build a fully-resolved state from the API response.
//     This resolves all Optional+Computed fields that the user omitted (Unknown).
//  2. Walk the plan and overlay all Known values on top of the resolved state.
//     This ensures user-configured values are preserved exactly as specified,
//     immune to API normalization (sentinel stripping, alias renaming, timestamp
//     reformatting, etc.).
//
// This eliminates the entire class of "Provider produced inconsistent result" errors
// for Create/Update operations. Sentinel restoration, alias normalization, and timestamp
// preservation in mapReportToModel are harmless here — they only affect Unknown values
// that the user didn't configure, where the API's normalized form is acceptable.
//
// Used by: Create, Update
// NOT used by: Read, ImportState (which use populateState → mapReportToModel).
func overlayReportComputedFields(ctx context.Context, apiResp *models.ExternalReport, plan *reportResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	// Phase 1: Build fully-resolved state from API response.
	// This gives us known values for every Optional+Computed field the user omitted.
	var resolved reportResourceModel
	diags.Append(mapReportToModel(ctx, apiResp, &resolved)...)
	if diags.HasError() {
		return diags
	}

	// Phase 2: Overlay known plan values on top of resolved state.
	// ── Top-level fields ──

	// id: always from API (Computed-only)
	plan.Id = resolved.Id

	// type: Computed-only — always from API
	plan.Type = resolved.Type

	// name: use plan if known, otherwise resolved
	if plan.Name.IsUnknown() {
		plan.Name = resolved.Name
	}

	// description: use plan if known, otherwise resolved
	if plan.Description.IsUnknown() {
		plan.Description = resolved.Description
	}

	// labels: use plan if known, otherwise resolved
	if plan.Labels.IsUnknown() {
		plan.Labels = resolved.Labels
	}

	// ── Config block ──
	// Walk each field: if the plan value is Known, keep it (user's source of truth).
	// If Unknown, use the API-resolved value.
	if !plan.Config.IsNull() && !plan.Config.IsUnknown() {
		diags.Append(overlayConfigFields(ctx, &resolved.Config, &plan.Config)...)
	} else if plan.Config.IsUnknown() {
		plan.Config = resolved.Config
	}

	return diags
}

// overlayConfigFields walks every field in the config block and replaces
// Unknown plan values with API-resolved values. Known plan values are preserved.
// For nested objects and list elements, it also walks into subfields.
func overlayConfigFields(ctx context.Context, resolved *resource_report.ConfigValue, plan *resource_report.ConfigValue) diag.Diagnostics {
	var diags diag.Diagnostics

	// ── Scalar fields ──
	if plan.Aggregation.IsUnknown() {
		plan.Aggregation = resolved.Aggregation
	}
	if plan.Currency.IsUnknown() {
		plan.Currency = resolved.Currency
	}
	if plan.DataSource.IsUnknown() {
		plan.DataSource = resolved.DataSource
	}
	if plan.DisplayValues.IsUnknown() {
		plan.DisplayValues = resolved.DisplayValues
	}
	if plan.IncludePromotionalCredits.IsUnknown() {
		plan.IncludePromotionalCredits = resolved.IncludePromotionalCredits
	}
	if plan.Layout.IsUnknown() {
		plan.Layout = resolved.Layout
	}

	if plan.TimeInterval.IsUnknown() {
		plan.TimeInterval = resolved.TimeInterval
	}

	// ── Nested object: DisplaySettings ──
	if plan.DisplaySettings.IsUnknown() {
		plan.DisplaySettings = resolved.DisplaySettings
	} else if !plan.DisplaySettings.IsNull() {
		overlayDisplaySettings(&resolved.DisplaySettings, &plan.DisplaySettings)
	}

	// ── Nested objects: resolve entire object when Unknown, or walk subfields ──
	if plan.AdvancedAnalysis.IsUnknown() {
		plan.AdvancedAnalysis = resolved.AdvancedAnalysis
	} else if !plan.AdvancedAnalysis.IsNull() {
		overlayAdvancedAnalysis(&resolved.AdvancedAnalysis, &plan.AdvancedAnalysis)
	}

	if plan.CustomTimeRange.IsUnknown() {
		plan.CustomTimeRange = resolved.CustomTimeRange
	} else if !plan.CustomTimeRange.IsNull() {
		overlayCustomTimeRange(&resolved.CustomTimeRange, &plan.CustomTimeRange)
	}

	if plan.Metric.IsUnknown() {
		plan.Metric = resolved.Metric
	} else if !plan.Metric.IsNull() {
		overlayMetric(&resolved.Metric, &plan.Metric)
	}

	if plan.MetricFilter.IsUnknown() {
		plan.MetricFilter = resolved.MetricFilter
	} else if !plan.MetricFilter.IsNull() {
		overlayMetricFilter(&resolved.MetricFilter, &plan.MetricFilter)
	}

	if plan.TimeRange.IsUnknown() {
		plan.TimeRange = resolved.TimeRange
	} else if !plan.TimeRange.IsNull() {
		overlayTimeRange(&resolved.TimeRange, &plan.TimeRange)
	}

	if plan.SecondaryTimeRange.IsUnknown() {
		plan.SecondaryTimeRange = resolved.SecondaryTimeRange
	} else if !plan.SecondaryTimeRange.IsNull() {
		overlaySecondaryTimeRange(&resolved.SecondaryTimeRange, &plan.SecondaryTimeRange)
	}

	if plan.ForecastSettings.IsUnknown() {
		plan.ForecastSettings = resolved.ForecastSettings
	} else if !plan.ForecastSettings.IsNull() {
		overlayForecastSettings(&resolved.ForecastSettings, &plan.ForecastSettings)
	}

	// ── List fields: use resolved when plan is Unknown ──
	// When known, the list elements are user-configured, so we keep them.
	// Subfield unknowns inside list elements (filters, splits, group) are
	// resolved element-by-element.
	if plan.Dimensions.IsUnknown() {
		plan.Dimensions = resolved.Dimensions
	} else if !plan.Dimensions.IsNull() {
		diags.Append(overlayListElements(ctx, &resolved.Dimensions, &plan.Dimensions, overlayDimension)...)
	}

	if plan.Filters.IsUnknown() {
		plan.Filters = resolved.Filters
	} else if !plan.Filters.IsNull() {
		diags.Append(overlayListElements(ctx, &resolved.Filters, &plan.Filters, overlayFilter)...)
	}

	if plan.Group.IsUnknown() {
		plan.Group = resolved.Group
	} else if !plan.Group.IsNull() {
		diags.Append(overlayListElements(ctx, &resolved.Group, &plan.Group, overlayGroup)...)
	}

	if plan.Metrics.IsUnknown() {
		plan.Metrics = resolved.Metrics
	} else if !plan.Metrics.IsNull() {
		diags.Append(overlayListElements(ctx, &resolved.Metrics, &plan.Metrics, overlayMetricsElement)...)
	}

	if plan.Splits.IsUnknown() {
		plan.Splits = resolved.Splits
	} else if !plan.Splits.IsNull() {
		diags.Append(overlayListElements(ctx, &resolved.Splits, &plan.Splits, overlaySplit)...)
	}

	return diags
}

// ── Nested object overlay helpers ──

func overlayAdvancedAnalysis(resolved, plan *resource_report.AdvancedAnalysisValue) {
	if plan.Forecast.IsUnknown() {
		plan.Forecast = resolved.Forecast
	}
	if plan.NotTrending.IsUnknown() {
		plan.NotTrending = resolved.NotTrending
	}
	if plan.TrendingDown.IsUnknown() {
		plan.TrendingDown = resolved.TrendingDown
	}
	if plan.TrendingUp.IsUnknown() {
		plan.TrendingUp = resolved.TrendingUp
	}
}

func overlayDisplaySettings(resolved, plan *resource_report.DisplaySettingsValue) {
	if plan.AxisLabelFontSize.IsUnknown() {
		plan.AxisLabelFontSize = resolved.AxisLabelFontSize
	}
	if plan.DataLabelFontSize.IsUnknown() {
		plan.DataLabelFontSize = resolved.DataLabelFontSize
	}
	if plan.DecimalPrecision.IsUnknown() {
		plan.DecimalPrecision = resolved.DecimalPrecision
	}
	if plan.NumberScale.IsUnknown() {
		plan.NumberScale = resolved.NumberScale
	}

	// theme_id: Optional+Computed with Default "default" — never Unknown at plan time.
}

func overlayCustomTimeRange(resolved, plan *resource_report.CustomTimeRangeValue) {
	if plan.From.IsUnknown() {
		plan.From = resolved.From
	}
	if plan.To.IsUnknown() {
		plan.To = resolved.To
	}
}

func overlayMetric(resolved, plan *resource_report.MetricValue) {
	if plan.MetricType.IsUnknown() {
		plan.MetricType = resolved.MetricType
	}
	if plan.Value.IsUnknown() {
		plan.Value = resolved.Value
	}
}

func overlayMetricFilter(resolved, plan *resource_report.MetricFilterValue) {
	if plan.Operator.IsUnknown() {
		plan.Operator = resolved.Operator
	}
	if plan.Values.IsUnknown() {
		plan.Values = resolved.Values
	}
	if plan.Metric.IsUnknown() {
		plan.Metric = resolved.Metric
	} else if !plan.Metric.IsNull() {
		overlayMetric(&resolved.Metric, &plan.Metric)
	}
}

func overlayTimeRange(resolved, plan *resource_report.TimeRangeValue) {
	if plan.Amount.IsUnknown() {
		plan.Amount = resolved.Amount
	}
	if plan.IncludeCurrent.IsUnknown() {
		plan.IncludeCurrent = resolved.IncludeCurrent
	}
	if plan.Mode.IsUnknown() {
		plan.Mode = resolved.Mode
	}
	if plan.Unit.IsUnknown() {
		plan.Unit = resolved.Unit
	}
}

func overlaySecondaryTimeRange(resolved, plan *resource_report.SecondaryTimeRangeValue) {
	if plan.Amount.IsUnknown() {
		plan.Amount = resolved.Amount
	}
	if plan.IncludeCurrent.IsUnknown() {
		plan.IncludeCurrent = resolved.IncludeCurrent
	}
	if plan.Unit.IsUnknown() {
		plan.Unit = resolved.Unit
	}
	if plan.CustomTimeRange.IsUnknown() {
		plan.CustomTimeRange = resolved.CustomTimeRange
	} else if !plan.CustomTimeRange.IsNull() {
		overlayCustomTimeRange(&resolved.CustomTimeRange, &plan.CustomTimeRange)
	}
}

func overlayForecastSettings(resolved, plan *resource_report.ForecastSettingsValue) {
	if plan.FutureCustomDateRange.IsUnknown() {
		plan.FutureCustomDateRange = resolved.FutureCustomDateRange
	} else if !plan.FutureCustomDateRange.IsNull() {
		overlayFutureCustomDateRange(&resolved.FutureCustomDateRange, &plan.FutureCustomDateRange)
	}
	if plan.FutureTimeIntervals.IsUnknown() {
		plan.FutureTimeIntervals = resolved.FutureTimeIntervals
	}
	if plan.HistoricalCustomDateRange.IsUnknown() {
		plan.HistoricalCustomDateRange = resolved.HistoricalCustomDateRange
	} else if !plan.HistoricalCustomDateRange.IsNull() {
		overlayHistoricalCustomDateRange(&resolved.HistoricalCustomDateRange, &plan.HistoricalCustomDateRange)
	}
	if plan.HistoricalTimeIntervals.IsUnknown() {
		plan.HistoricalTimeIntervals = resolved.HistoricalTimeIntervals
	}
}

func overlayFutureCustomDateRange(resolved, plan *resource_report.FutureCustomDateRangeValue) {
	if plan.From.IsUnknown() {
		plan.From = resolved.From
	}
	if plan.To.IsUnknown() {
		plan.To = resolved.To
	}
}

func overlayHistoricalCustomDateRange(resolved, plan *resource_report.HistoricalCustomDateRangeValue) {
	if plan.From.IsUnknown() {
		plan.From = resolved.From
	}
	if plan.To.IsUnknown() {
		plan.To = resolved.To
	}
}

// ── List element overlay helpers ──

// isUnknownOverlayElement checks whether a list element is Unknown as a whole.
func isUnknownOverlayElement[T any](v T) bool {
	if unknownable, ok := any(v).(interface{ IsUnknown() bool }); ok {
		return unknownable.IsUnknown()
	}
	return false
}

// isNullOverlayElement checks whether a list element is Null as a whole.
func isNullOverlayElement[T any](v T) bool {
	if nullable, ok := any(v).(interface{ IsNull() bool }); ok {
		return nullable.IsNull()
	}
	return false
}

// overlayListElements is a generic helper that walks two lists element-by-element,
// invoking the overlay function for each matching index. If a plan element is
// Unknown as a whole, it is replaced with the corresponding resolved element.
// If a resolved element is Null/Unknown, the overlay is skipped for that element.
// Diagnostics from element decoding, overlay functions, and list rebuilding are
// returned to the caller.
func overlayListElements[T any](ctx context.Context, resolved, plan *types.List, overlayFn func(context.Context, *T, *T) diag.Diagnostics) diag.Diagnostics {
	var diags diag.Diagnostics
	var planElems []T
	var resolvedElems []T

	planDiags := plan.ElementsAs(ctx, &planElems, true)
	diags.Append(planDiags...)
	if planDiags.HasError() {
		return diags
	}
	resolvedDiags := resolved.ElementsAs(ctx, &resolvedElems, true)
	diags.Append(resolvedDiags...)
	if resolvedDiags.HasError() {
		return diags
	}

	for i := range planElems {
		if i >= len(resolvedElems) {
			continue
		}
		// If the plan element is Unknown as a whole, replace it with the resolved value.
		if isUnknownOverlayElement(planElems[i]) {
			planElems[i] = resolvedElems[i]
			continue
		}
		// If the resolved element is Null/Unknown (API didn't return it), skip
		// the subfield overlay — the plan element keeps its values as-is.
		// This guards all list element helpers uniformly without requiring
		// individual null checks in each helper.
		if isNullOverlayElement(resolvedElems[i]) || isUnknownOverlayElement(resolvedElems[i]) {
			continue
		}
		diags.Append(overlayFn(ctx, &resolvedElems[i], &planElems[i])...)
	}

	// Rebuild the list with overlaid elements.
	newList, rebuildDiags := types.ListValueFrom(ctx, plan.ElementType(ctx), planElems)
	diags.Append(rebuildDiags...)
	if !rebuildDiags.HasError() {
		*plan = newList
	}
	return diags
}

func overlayDimension(_ context.Context, resolved, plan *resource_report.DimensionsValue) diag.Diagnostics {
	if plan.Id.IsUnknown() {
		plan.Id = resolved.Id
	}
	if plan.DimensionsType.IsUnknown() {
		plan.DimensionsType = resolved.DimensionsType
	}
	return nil
}

func overlayFilter(_ context.Context, resolved, plan *resource_report.FiltersValue) diag.Diagnostics {

	if plan.Inverse.IsUnknown() {
		plan.Inverse = resolved.Inverse
	}

	if plan.Mode.IsUnknown() {
		plan.Mode = resolved.Mode
	}

	if plan.Values.IsUnknown() {
		plan.Values = resolved.Values
	}
	return nil
}

func overlayGroup(_ context.Context, resolved, plan *resource_report.GroupValue) diag.Diagnostics {
	if plan.Id.IsUnknown() {
		plan.Id = resolved.Id
	}
	if plan.GroupType.IsUnknown() {
		plan.GroupType = resolved.GroupType
	}
	if plan.Limit.IsUnknown() {
		plan.Limit = resolved.Limit
	} else if !plan.Limit.IsNull() {
		overlayLimit(&resolved.Limit, &plan.Limit)
	}
	return nil
}

func overlayLimit(resolved, plan *resource_report.LimitValue) {
	if plan.Sort.IsUnknown() {
		plan.Sort = resolved.Sort
	}
	if plan.Value.IsUnknown() {
		plan.Value = resolved.Value
	}
	if plan.Metric.IsUnknown() {
		plan.Metric = resolved.Metric
	} else if !plan.Metric.IsNull() {
		overlayMetric(&resolved.Metric, &plan.Metric)
	}
}

func overlayMetricsElement(_ context.Context, resolved, plan *resource_report.MetricsValue) diag.Diagnostics {
	if plan.MetricsType.IsUnknown() {
		plan.MetricsType = resolved.MetricsType
	}
	if plan.Value.IsUnknown() {
		plan.Value = resolved.Value
	}
	return nil
}

func overlaySplit(ctx context.Context, resolved, plan *resource_report.SplitsValue) diag.Diagnostics {
	var diags diag.Diagnostics
	if plan.Id.IsUnknown() {
		plan.Id = resolved.Id
	}
	if plan.IncludeOrigin.IsUnknown() {
		plan.IncludeOrigin = resolved.IncludeOrigin
	}
	if plan.Mode.IsUnknown() {
		plan.Mode = resolved.Mode
	}
	if plan.SplitsType.IsUnknown() {
		plan.SplitsType = resolved.SplitsType
	}
	if plan.Origin.IsUnknown() {
		plan.Origin = resolved.Origin
	} else if !plan.Origin.IsNull() {
		overlayOrigin(&resolved.Origin, &plan.Origin)
	}
	if plan.Targets.IsUnknown() {
		plan.Targets = resolved.Targets
	} else if !plan.Targets.IsNull() {
		diags.Append(overlayListElements(ctx, &resolved.Targets, &plan.Targets, overlayTarget)...)
	}
	return diags
}

func overlayTarget(_ context.Context, resolved, plan *resource_report.TargetsValue) diag.Diagnostics {
	if plan.Id.IsUnknown() {
		plan.Id = resolved.Id
	}
	if plan.TargetsType.IsUnknown() {
		plan.TargetsType = resolved.TargetsType
	}
	if plan.Value.IsUnknown() {
		plan.Value = resolved.Value
	}
	return nil
}

func overlayOrigin(resolved, plan *resource_report.OriginValue) {
	if plan.Id.IsUnknown() {
		plan.Id = resolved.Id
	}
	if plan.OriginType.IsUnknown() {
		plan.OriginType = resolved.OriginType
	}
}

// isNAFallback reports whether v is a NullFallback sentinel of the form "[... N/A]"
// as defined in the Cloud Analytics metadata keymap (metadata_keymap.go).
// The API (null_fallback.go StripNullFallback) silently removes these sentinels from
// filter values and converts them to includeNull=true. We use this check to restore
// them on read, preserving round-trip fidelity without importing the internal keymap.
func isNAFallback(v string) bool {
	return strings.HasPrefix(v, "[") && strings.HasSuffix(v, " N/A]")
}

// mergeSentinelValues merges API-returned values with the prior state to restore
// any NullFallback sentinels that the API strips during normalization.
//
// apiValues are the values returned by the API for a single filter/scope.
// stateVals are the values from the prior Terraform state for the same filter/scope.
// apiIncludeNull is the include_null flag returned by the API.
//
// Algorithm:
//  1. Build a set of apiValues for O(1) lookup.
//  2. Walk stateVals in order (preserving prior-state ordering):
//     - If the value is in apiValues: keep it and mark it as placed.
//     - Else if apiIncludeNull=true and it is a sentinel: restore it in-place.
//     - Otherwise: flag that a non-sentinel value disappeared.
//  3. If a non-sentinel disappeared AND API returned nothing: fall back to
//     the full state list (legacy behaviour for filters the API wiped entirely).
//  4. Append any apiValues not already placed (genuinely new values from the API),
//     in their API-returned order.
//
// Walking stateVals first (step 2) ensures sentinels are placed at their original
// indices, preventing order-sensitive drift in Terraform ListAttributes.
func mergeSentinelValues(apiValues []string, stateVals []string, apiIncludeNull bool) []string {
	apiValueSet := make(map[string]bool, len(apiValues))
	for _, v := range apiValues {
		apiValueSet[v] = true
	}

	// Walk prior state in order: keep API-present values and restore stripped sentinels.
	result := make([]string, 0, len(stateVals)+len(apiValues))
	usedAPIValues := make(map[string]bool, len(apiValues))
	hasLostNonSentinel := false

	for _, sv := range stateVals {
		if apiValueSet[sv] {
			// Value still returned by API: keep it at its original position.
			result = append(result, sv)
			usedAPIValues[sv] = true
		} else if apiIncludeNull && isNAFallback(sv) {
			// Sentinel was stripped by API normalization: restore it in-place.
			result = append(result, sv)
		} else if !isNAFallback(sv) {
			// A non-sentinel value disappeared according to the API.
			// (A sentinel that was not restored — because apiIncludeNull=false —
			// is intentionally dropped and does NOT trigger the full-state fallback.)
			hasLostNonSentinel = true
		}
	}

	if hasLostNonSentinel && len(apiValues) == 0 {
		// API returned nothing at all and a non-sentinel is missing:
		// fall back to preserving the full state list (legacy behaviour).
		return stateVals
	}

	// Append any API values not already placed (genuinely new entries),
	// preserving the API's ordering for these new entries.
	for _, v := range apiValues {
		if !usedAPIValues[v] {
			result = append(result, v)
		}
	}

	return result
}

// populateState fetches the report from the API and populates the Terraform state.
// This is used by Read and ImportState. Create and Update use overlayReportComputedFields
// with the API response instead.
//
// On 404, state.Id is set to null to signal Terraform to remove the resource from state.
func (r *reportResource) populateState(ctx context.Context, state *reportResourceModel) diag.Diagnostics {
	reportResp, err := r.client.GetReportConfigWithResponse(ctx, state.Id.ValueString())
	if err != nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error reading report", "Could not read report config, unexpected error: "+err.Error()),
		}
	}

	// Handle externally deleted resource
	if reportResp.StatusCode() == 404 {
		state.Id = types.StringNull()
		return nil
	}

	if reportResp.StatusCode() != 200 {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error reading report", fmt.Sprintf("Could not read report config, status: %d", reportResp.StatusCode())),
		}
	}

	if reportResp.JSON200 == nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error reading report", "Could not read report config, empty response"),
		}
	}

	return mapReportToModel(ctx, reportResp.JSON200, state)
}

func (plan *reportResourceModel) toCreateRequest(ctx context.Context) (req models.CreateReportJSONRequestBody, diags diag.Diagnostics) {
	if !plan.Name.IsNull() && !plan.Name.IsUnknown() {
		req.Name = plan.Name.ValueStringPointer()
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		req.Description = plan.Description.ValueStringPointer()
	}
	req.FolderId = plan.FolderId.ValueStringPointer()

	if !plan.Labels.IsNull() && !plan.Labels.IsUnknown() {
		var labels []string
		diags.Append(plan.Labels.ElementsAs(ctx, &labels, false)...)
		if diags.HasError() {
			return req, diags
		}
		req.Labels = &labels
	}

	config, d := toExternalConfig(ctx, plan.Config)
	diags.Append(d...)
	if diags.HasError() {
		return req, diags
	}
	req.Config = config

	return req, diags
}

func (plan *reportResourceModel) toUpdateRequest(ctx context.Context) (req models.UpdateReportJSONRequestBody, diags diag.Diagnostics) {
	if !plan.Name.IsNull() && !plan.Name.IsUnknown() {
		req.Name = plan.Name.ValueStringPointer()
	}
	req.FolderId = plan.FolderId.ValueStringPointer()

	if !plan.Labels.IsNull() && !plan.Labels.IsUnknown() {
		var labels []string
		diags.Append(plan.Labels.ElementsAs(ctx, &labels, false)...)
		if diags.HasError() {
			return req, diags
		}
		req.Labels = &labels
	}

	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		req.Description = plan.Description.ValueStringPointer()
	}

	config, d := toExternalConfig(ctx, plan.Config)
	diags.Append(d...)
	if diags.HasError() {
		return req, diags
	}
	req.Config = config

	return req, diags
}

func toExternalConfig(ctx context.Context, config resource_report.ConfigValue) (externalConfig *models.ExternalConfig, diags diag.Diagnostics) {
	if config.IsNull() || config.IsUnknown() {
		return nil, diags
	}
	externalConfig = &models.ExternalConfig{}

	// All top-level scalar fields are guarded with null/unknown checks so this
	// function works both in the resource context (where defaults populate
	// these fields) and in the data source context (where they may be omitted).
	if !config.Aggregation.IsNull() && !config.Aggregation.IsUnknown() {
		externalConfig.Aggregation = new(models.ExternalConfigAggregation(config.Aggregation.ValueString()))
	}
	if !config.Currency.IsNull() && !config.Currency.IsUnknown() {
		externalConfig.Currency = new(models.Currency(config.Currency.ValueString()))
	}
	if !config.DisplayValues.IsNull() && !config.DisplayValues.IsUnknown() {
		externalConfig.DisplayValues = new(models.ExternalConfigDisplayValues(config.DisplayValues.ValueString()))
	}
	if !config.IncludePromotionalCredits.IsNull() && !config.IncludePromotionalCredits.IsUnknown() {
		externalConfig.IncludePromotionalCredits = new(config.IncludePromotionalCredits.ValueBool())
	}
	if !config.Layout.IsNull() && !config.Layout.IsUnknown() {
		externalConfig.Layout = new(models.ExternalRenderer(config.Layout.ValueString()))
	}
	if !config.SortDimensions.IsNull() {
		externalConfig.SortDimensions = new(models.ExternalConfigSortDimensions(config.SortDimensions.ValueString()))
	}
	if !config.SortGroups.IsNull() {
		externalConfig.SortGroups = new(models.ExternalConfigSortGroups(config.SortGroups.ValueString()))
	}
	if !config.TimeInterval.IsNull() && !config.TimeInterval.IsUnknown() {
		externalConfig.TimeInterval = new(models.ExternalConfigTimeInterval(config.TimeInterval.ValueString()))
	}

	if !config.DataSource.IsNull() && !config.DataSource.IsUnknown() {
		externalConfig.DataSource = new(models.ExternalConfigDataSource(config.DataSource.ValueString()))
	}

	if !config.AdvancedAnalysis.IsNull() && !config.AdvancedAnalysis.IsUnknown() {
		aa := &models.AdvancedAnalysis{}
		if !config.AdvancedAnalysis.Forecast.IsNull() && !config.AdvancedAnalysis.Forecast.IsUnknown() {
			aa.Forecast = config.AdvancedAnalysis.Forecast.ValueBoolPointer()
		}
		if !config.AdvancedAnalysis.NotTrending.IsNull() && !config.AdvancedAnalysis.NotTrending.IsUnknown() {
			aa.NotTrending = config.AdvancedAnalysis.NotTrending.ValueBoolPointer()
		}
		if !config.AdvancedAnalysis.TrendingDown.IsNull() && !config.AdvancedAnalysis.TrendingDown.IsUnknown() {
			aa.TrendingDown = config.AdvancedAnalysis.TrendingDown.ValueBoolPointer()
		}
		if !config.AdvancedAnalysis.TrendingUp.IsNull() && !config.AdvancedAnalysis.TrendingUp.IsUnknown() {
			aa.TrendingUp = config.AdvancedAnalysis.TrendingUp.ValueBoolPointer()
		}
		externalConfig.AdvancedAnalysis = aa
	}

	if !config.CustomTimeRange.IsNull() && !config.CustomTimeRange.IsUnknown() {
		customTimeRange := models.ExternalConfigCustomTimeRange{}
		if !config.CustomTimeRange.From.IsNull() && !config.CustomTimeRange.From.IsUnknown() {
			fromTime, err := time.Parse(time.RFC3339, config.CustomTimeRange.From.ValueString())
			if err != nil {
				diags.AddError("Invalid From Time", "Could not parse CustomTimeRange.From as RFC3339: "+err.Error())
			}
			customTimeRange.From = &fromTime
		}
		if !config.CustomTimeRange.To.IsNull() && !config.CustomTimeRange.To.IsUnknown() {
			toTime, err := time.Parse(time.RFC3339, config.CustomTimeRange.To.ValueString())
			if err != nil {
				diags.AddError("Invalid To Time", "Could not parse CustomTimeRange.To as RFC3339: "+err.Error())
			}
			customTimeRange.To = &toTime
		}
		if !diags.HasError() {
			externalConfig.CustomTimeRange = &customTimeRange
		}
	}

	if !config.Dimensions.IsNull() && !config.Dimensions.IsUnknown() {
		var dimensions []resource_report.DimensionsValue
		diags.Append(config.Dimensions.ElementsAs(ctx, &dimensions, false)...)
		if !diags.HasError() {
			externalDimensions := make([]models.Dimension, len(dimensions))
			for i, d := range dimensions {
				externalDimensions[i] = models.Dimension{
					Id:   d.Id.ValueStringPointer(),
					Type: new(models.DimensionsTypes(d.DimensionsType.ValueString())),
				}
			}
			externalConfig.Dimensions = &externalDimensions
		}
	}

	if !config.Filters.IsNull() && !config.Filters.IsUnknown() {
		var filters []resource_report.FiltersValue
		diags.Append(config.Filters.ElementsAs(ctx, &filters, false)...)
		if !diags.HasError() {
			externalFilters := make([]models.ExternalConfigFilter, len(filters))
			for i, f := range filters {
				filterType := models.DimensionsTypes(f.FiltersType.ValueString())
				externalFilters[i] = models.ExternalConfigFilter{
					CaseInsensitive: f.CaseInsensitive.ValueBoolPointer(),
					Id:              f.Id.ValueString(),
					IncludeNull:     f.IncludeNull.ValueBoolPointer(),
					Inverse:         f.Inverse.ValueBoolPointer(),
					Type:            filterType,
				}
				if !f.Values.IsNull() && !f.Values.IsUnknown() {
					var values []string
					diags.Append(f.Values.ElementsAs(ctx, &values, false)...)
					if diags.HasError() {
						continue
					}
					externalFilters[i].Values = &values
				}
				if !f.Mode.IsNull() && !f.Mode.IsUnknown() {
					externalFilters[i].Mode = new(models.ExternalConfigFilterMode(f.Mode.ValueString()))
				}
			}
			externalConfig.Filters = &externalFilters
		}
	}

	if !config.Group.IsNull() && !config.Group.IsUnknown() {
		var groups []resource_report.GroupValue
		diags.Append(config.Group.ElementsAs(ctx, &groups, false)...)
		if !diags.HasError() {
			externalGroups := make([]models.Group, len(groups))
			for i, g := range groups {
				externalGroups[i] = models.Group{
					Id:   g.Id.ValueStringPointer(),
					Type: new(models.DimensionsTypes(g.GroupType.ValueString())),
				}
				if !g.Limit.IsNull() && !g.Limit.IsUnknown() {
					limit := models.Limit{}
					if !g.Limit.Sort.IsNull() && !g.Limit.Sort.IsUnknown() {
						limit.Sort = new(models.LimitSort(g.Limit.Sort.ValueString()))
					}
					if !g.Limit.Value.IsNull() && !g.Limit.Value.IsUnknown() {
						limit.Value = g.Limit.Value.ValueInt64Pointer()
					}
					if !g.Limit.Metric.IsNull() && !g.Limit.Metric.IsUnknown() {
						metric := baseTypeObjectValueToExternalMetric(g.Limit.Metric)
						limit.Metric = metric
					}
					externalGroups[i].Limit = &limit
				}
			}
			externalConfig.Group = &externalGroups
		}
	}

	// metric and metrics are mutually exclusive in the API. On Update, prior state
	// may carry both (the API response populates both). Prefer metrics (non-deprecated).
	hasMetrics := !config.Metrics.IsNull() && !config.Metrics.IsUnknown()
	if !config.Metric.IsNull() && !config.Metric.IsUnknown() && !hasMetrics {
		metric := baseTypeObjectValueToExternalMetric(config.Metric)
		externalConfig.Metric = metric
	}

	// Handle metrics list (new multi-metric support, replaces deprecated singular metric)
	// - If user sets a non-empty list: send it to API
	// - If user sets an empty list []: send nothing (omit from request), API preserves existing
	// - If user omits (null): send nothing (omit from request), API uses defaults
	// Note: We only send metrics if the list has elements to avoid accidental state inconsistency.
	if !config.Metrics.IsNull() && !config.Metrics.IsUnknown() {
		var metricsValues []resource_report.MetricsValue
		diags.Append(config.Metrics.ElementsAs(ctx, &metricsValues, false)...)
		if !diags.HasError() {
			externalMetrics := make([]models.ExternalMetric, len(metricsValues))
			for i, m := range metricsValues {
				externalMetrics[i] = models.ExternalMetric{}
				if !m.MetricsType.IsNull() && !m.MetricsType.IsUnknown() {
					externalMetrics[i].Type = new(models.ExternalMetricType(m.MetricsType.ValueString()))
				}
				if !m.Value.IsNull() && !m.Value.IsUnknown() {
					externalMetrics[i].Value = m.Value.ValueStringPointer()
				}
			}
			// Only send metrics to API if list is non-empty
			// Empty list means user wants to clear, but API doesn't support this - omit from request
			if len(externalMetrics) > 0 {
				externalConfig.Metrics = &externalMetrics
			}
		}
	}

	if !config.MetricFilter.IsNull() && !config.MetricFilter.IsUnknown() {
		externalConfig.MetricFilter = &models.ExternalConfigMetricFilter{}
		if !config.MetricFilter.Metric.IsNull() && !config.MetricFilter.Metric.IsUnknown() {
			metric := baseTypeObjectValueToExternalMetric(config.MetricFilter.Metric)
			externalConfig.MetricFilter.Metric = metric
		}
		if !config.MetricFilter.Values.IsNull() && !config.MetricFilter.Values.IsUnknown() {
			var values []float64
			diags.Append(config.MetricFilter.Values.ElementsAs(ctx, &values, false)...)
			externalConfig.MetricFilter.Values = &values
		}
		if !config.MetricFilter.Operator.IsNull() && !config.MetricFilter.Operator.IsUnknown() {
			externalConfig.MetricFilter.Operator = new(models.ExternalConfigMetricFilterOperator(config.MetricFilter.Operator.ValueString()))
		}
	}

	if !config.Splits.IsNull() && !config.Splits.IsUnknown() {
		var splits []resource_report.SplitsValue
		diags.Append(config.Splits.ElementsAs(ctx, &splits, false)...)
		if !diags.HasError() {
			externalSplits := make([]models.ExternalSplit, len(splits))
			for i, s := range splits {
				externalSplits[i] = models.ExternalSplit{
					Id:            s.Id.ValueStringPointer(),
					IncludeOrigin: s.IncludeOrigin.ValueBoolPointer(),
					Mode:          new(models.ExternalSplitMode(s.Mode.ValueString())),
					Type:          new(models.ExternalSplitType(s.SplitsType.ValueString())),
				}
				if !s.Origin.IsNull() && !s.Origin.IsUnknown() {
					externalSplits[i].Origin = &models.ExternalOrigin{
						Id:   s.Origin.Id.ValueStringPointer(),
						Type: new(models.ExternalOriginType(s.Origin.OriginType.ValueString())),
					}
				}
				if !s.Targets.IsNull() && !s.Targets.IsUnknown() {
					var targets []resource_report.TargetsValue
					diags.Append(s.Targets.ElementsAs(ctx, &targets, false)...)
					if diags.HasError() {
						continue
					}
					externalTargets := make([]models.ExternalSplitTarget, len(targets))
					for j, t := range targets {
						externalTargets[j] = models.ExternalSplitTarget{
							Id:    t.Id.ValueStringPointer(),
							Type:  new(models.ExternalSplitTargetType(t.TargetsType.ValueString())),
							Value: t.Value.ValueFloat64Pointer(),
						}
					}
					externalSplits[i].Targets = &externalTargets
				}
			}
			externalConfig.Splits = &externalSplits
		}
	}

	if !config.TimeRange.IsNull() && !config.TimeRange.IsUnknown() {
		ts := &models.TimeSettings{}
		if !config.TimeRange.Amount.IsNull() && !config.TimeRange.Amount.IsUnknown() {
			ts.Amount = config.TimeRange.Amount.ValueInt64Pointer()
		}
		if !config.TimeRange.IncludeCurrent.IsNull() && !config.TimeRange.IncludeCurrent.IsUnknown() {
			ts.IncludeCurrent = config.TimeRange.IncludeCurrent.ValueBoolPointer()
		}
		if !config.TimeRange.Mode.IsNull() && !config.TimeRange.Mode.IsUnknown() {
			ts.Mode = new(models.TimeSettingsMode(config.TimeRange.Mode.ValueString()))
		}
		if !config.TimeRange.Unit.IsNull() && !config.TimeRange.Unit.IsUnknown() {
			ts.Unit = new(models.TimeSettingsUnit(config.TimeRange.Unit.ValueString()))
		}
		externalConfig.TimeRange = ts
	}

	if !config.SecondaryTimeRange.IsNull() && !config.SecondaryTimeRange.IsUnknown() {
		secondaryTimeRange := &models.TimeSettingsSecondary{}
		if !config.SecondaryTimeRange.Amount.IsNull() && !config.SecondaryTimeRange.Amount.IsUnknown() {
			secondaryTimeRange.Amount = config.SecondaryTimeRange.Amount.ValueInt64Pointer()
		}
		if !config.SecondaryTimeRange.IncludeCurrent.IsNull() && !config.SecondaryTimeRange.IncludeCurrent.IsUnknown() {
			secondaryTimeRange.IncludeCurrent = config.SecondaryTimeRange.IncludeCurrent.ValueBoolPointer()
		}
		if !config.SecondaryTimeRange.Unit.IsNull() && !config.SecondaryTimeRange.Unit.IsUnknown() {
			secondaryTimeRange.Unit = new(models.TimeSettingsSecondaryUnit(config.SecondaryTimeRange.Unit.ValueString()))
		}
		if !config.SecondaryTimeRange.CustomTimeRange.IsNull() && !config.SecondaryTimeRange.CustomTimeRange.IsUnknown() {
			ctr := models.TimeSettingsSecondaryCustomTimeRange{}
			if !config.SecondaryTimeRange.CustomTimeRange.From.IsNull() && !config.SecondaryTimeRange.CustomTimeRange.From.IsUnknown() {
				fromTime, err := time.Parse(time.RFC3339, config.SecondaryTimeRange.CustomTimeRange.From.ValueString())
				if err != nil {
					diags.AddError("Invalid From Time", "Could not parse SecondaryTimeRange.CustomTimeRange.From as RFC3339: "+err.Error())
				}
				ctr.From = &fromTime
			}
			if !config.SecondaryTimeRange.CustomTimeRange.To.IsNull() && !config.SecondaryTimeRange.CustomTimeRange.To.IsUnknown() {
				toTime, err := time.Parse(time.RFC3339, config.SecondaryTimeRange.CustomTimeRange.To.ValueString())
				if err != nil {
					diags.AddError("Invalid To Time", "Could not parse SecondaryTimeRange.CustomTimeRange.To as RFC3339: "+err.Error())
				}
				ctr.To = &toTime
			}
			if !diags.HasError() {
				secondaryTimeRange.CustomTimeRange = &ctr
			}
		}
		externalConfig.SecondaryTimeRange = secondaryTimeRange
	}

	if !config.DisplaySettings.IsNull() && !config.DisplaySettings.IsUnknown() {
		ds := &models.ExternalDisplaySettings{}
		if !config.DisplaySettings.AxisLabelFontSize.IsNull() && !config.DisplaySettings.AxisLabelFontSize.IsUnknown() {
			ds.AxisLabelFontSize = new(models.ExternalDisplaySettingsAxisLabelFontSize(config.DisplaySettings.AxisLabelFontSize.ValueString()))
		}
		if !config.DisplaySettings.DataLabelFontSize.IsNull() && !config.DisplaySettings.DataLabelFontSize.IsUnknown() {
			ds.DataLabelFontSize = new(models.ExternalDisplaySettingsDataLabelFontSize(config.DisplaySettings.DataLabelFontSize.ValueString()))
		}
		if !config.DisplaySettings.DecimalPrecision.IsNull() && !config.DisplaySettings.DecimalPrecision.IsUnknown() {
			ds.DecimalPrecision = new(int(config.DisplaySettings.DecimalPrecision.ValueInt64()))
		}
		if !config.DisplaySettings.NumberScale.IsNull() && !config.DisplaySettings.NumberScale.IsUnknown() {
			ds.NumberScale = new(models.ExternalDisplaySettingsNumberScale(config.DisplaySettings.NumberScale.ValueString()))
		}
		if !config.DisplaySettings.ThemeId.IsNull() {
			ds.ThemeId = new(config.DisplaySettings.ThemeId.ValueString())
		}
		externalConfig.DisplaySettings = ds
	}

	if !config.ForecastSettings.IsNull() && !config.ForecastSettings.IsUnknown() {
		isModeDefault := config.ForecastSettings.Mode.IsNull() ||
			config.ForecastSettings.Mode.ValueString() == "totals"

		if config.ForecastSettings.FutureTimeIntervals.IsNull() &&
			config.ForecastSettings.HistoricalTimeIntervals.IsNull() &&
			config.ForecastSettings.FutureCustomDateRange.IsNull() &&
			config.ForecastSettings.HistoricalCustomDateRange.IsNull() &&
			isModeDefault {
			externalConfig.ForecastSettings.Set(models.ExternalForecastSettings{})
		} else {
			fs := models.ExternalForecastSettings{}
			if !config.ForecastSettings.FutureTimeIntervals.IsNull() && !config.ForecastSettings.FutureTimeIntervals.IsUnknown() {
				fs.FutureTimeIntervals = config.ForecastSettings.FutureTimeIntervals.ValueInt64Pointer()
			}
			if !config.ForecastSettings.HistoricalTimeIntervals.IsNull() && !config.ForecastSettings.HistoricalTimeIntervals.IsUnknown() {
				fs.HistoricalTimeIntervals = config.ForecastSettings.HistoricalTimeIntervals.ValueInt64Pointer()
			}
			if !config.ForecastSettings.Mode.IsNull() {
				fs.Mode = new(models.ExternalForecastSettingsMode)
				*fs.Mode = models.ExternalForecastSettingsMode(config.ForecastSettings.Mode.ValueString())
			}
			if !config.ForecastSettings.FutureCustomDateRange.IsNull() && !config.ForecastSettings.FutureCustomDateRange.IsUnknown() {
				fcdr := models.ExternalForecastDateRange{}
				if !config.ForecastSettings.FutureCustomDateRange.From.IsNull() && !config.ForecastSettings.FutureCustomDateRange.From.IsUnknown() {
					fromTime, err := time.Parse(time.RFC3339, config.ForecastSettings.FutureCustomDateRange.From.ValueString())
					if err != nil {
						diags.AddError("Invalid From Time", "Could not parse ForecastSettings.FutureCustomDateRange.From as RFC3339: "+err.Error())
					}
					fcdr.From = &fromTime
				}
				if !config.ForecastSettings.FutureCustomDateRange.To.IsNull() && !config.ForecastSettings.FutureCustomDateRange.To.IsUnknown() {
					toTime, err := time.Parse(time.RFC3339, config.ForecastSettings.FutureCustomDateRange.To.ValueString())
					if err != nil {
						diags.AddError("Invalid To Time", "Could not parse ForecastSettings.FutureCustomDateRange.To as RFC3339: "+err.Error())
					}
					fcdr.To = &toTime
				}
				if !diags.HasError() {
					fs.FutureCustomDateRange = &fcdr
				}
			}
			if !config.ForecastSettings.HistoricalCustomDateRange.IsNull() && !config.ForecastSettings.HistoricalCustomDateRange.IsUnknown() {
				hcdr := models.ExternalForecastDateRange{}
				if !config.ForecastSettings.HistoricalCustomDateRange.From.IsNull() && !config.ForecastSettings.HistoricalCustomDateRange.From.IsUnknown() {
					fromTime, err := time.Parse(time.RFC3339, config.ForecastSettings.HistoricalCustomDateRange.From.ValueString())
					if err != nil {
						diags.AddError("Invalid From Time", "Could not parse ForecastSettings.HistoricalCustomDateRange.From as RFC3339: "+err.Error())
					}
					hcdr.From = &fromTime
				}
				if !config.ForecastSettings.HistoricalCustomDateRange.To.IsNull() && !config.ForecastSettings.HistoricalCustomDateRange.To.IsUnknown() {
					toTime, err := time.Parse(time.RFC3339, config.ForecastSettings.HistoricalCustomDateRange.To.ValueString())
					if err != nil {
						diags.AddError("Invalid To Time", "Could not parse ForecastSettings.HistoricalCustomDateRange.To as RFC3339: "+err.Error())
					}
					hcdr.To = &toTime
				}
				if !diags.HasError() {
					fs.HistoricalCustomDateRange = &hcdr
				}
			}
			if !diags.HasError() {
				externalConfig.ForecastSettings.Set(fs)
			}
		}
	} else if config.ForecastSettings.IsNull() {
		externalConfig.ForecastSettings.SetNull()
	}

	return externalConfig, diags
}

func mapReportToModel(ctx context.Context, resp *models.ExternalReport, state *reportResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	state.Id = types.StringPointerValue(resp.Id)
	state.Name = types.StringValue(resp.Name)
	state.Description = types.StringPointerValue(resp.Description)
	// Defend against API returning nil: fall back to "root" to match the
	// schema default and prevent perpetual plan drift.
	if resp.FolderId != nil {
		state.FolderId = types.StringValue(*resp.FolderId)
	} else {
		state.FolderId = types.StringValue("root")
	}
	if resp.Type != nil {
		state.Type = types.StringValue(string(*resp.Type))
	} else {
		state.Type = types.StringNull()
	}

	// Labels: user-configurable list — always return empty list instead of null
	if resp.Labels != nil && len(*resp.Labels) > 0 {
		var labelsDiags diag.Diagnostics
		state.Labels, labelsDiags = types.ListValueFrom(ctx, types.StringType, *resp.Labels)
		diags.Append(labelsDiags...)
	} else {
		var emptyLabelsDiags diag.Diagnostics
		state.Labels, emptyLabelsDiags = types.ListValue(types.StringType, []attr.Value{})
		diags.Append(emptyLabelsDiags...)
	}

	if resp.Config == nil {
		state.Config = resource_report.NewConfigValueNull()
		return diags
	}

	config := resp.Config

	// Helper to create simple map for ConfigValue
	configMap := map[string]attr.Value{
		"aggregation":                 types.StringNull(),
		"currency":                    types.StringNull(),
		"data_source":                 types.StringNull(),
		"display_values":              types.StringNull(),
		"include_promotional_credits": types.BoolNull(),
		"include_subtotals":           types.BoolNull(),
		"layout":                      types.StringNull(),
		"sort_dimensions":             types.StringNull(),
		"sort_groups":                 types.StringNull(),
		"time_interval":               types.StringNull(),
		"forecast_settings":           resource_report.NewForecastSettingsValueNull(),
	}

	if config.Aggregation != nil {
		configMap["aggregation"] = types.StringValue(string(*config.Aggregation))
	}
	if config.Currency != nil {
		configMap["currency"] = types.StringValue(string(*config.Currency))
	}
	if config.DataSource != nil {
		configMap["data_source"] = types.StringValue(string(*config.DataSource))
	}
	if config.DisplayValues != nil {
		configMap["display_values"] = types.StringValue(string(*config.DisplayValues))
	}
	if config.IncludePromotionalCredits != nil {
		configMap["include_promotional_credits"] = types.BoolValue(*config.IncludePromotionalCredits)
	}
	if config.IncludeSubtotals != nil {
		configMap["include_subtotals"] = types.BoolValue(*config.IncludeSubtotals)
	}
	if config.Layout != nil {
		configMap["layout"] = types.StringValue(string(*config.Layout))
	}
	if config.SortDimensions != nil {
		configMap["sort_dimensions"] = types.StringValue(string(*config.SortDimensions))
	}
	if config.SortGroups != nil {
		configMap["sort_groups"] = types.StringValue(string(*config.SortGroups))
	}
	if config.TimeInterval != nil {
		configMap["time_interval"] = types.StringValue(string(*config.TimeInterval))
	}

	// Nested Object: AdvancedAnalysis
	if config.AdvancedAnalysis != nil {
		advMap := map[string]attr.Value{
			"forecast":      types.BoolPointerValue(config.AdvancedAnalysis.Forecast),
			"not_trending":  types.BoolPointerValue(config.AdvancedAnalysis.NotTrending),
			"trending_down": types.BoolPointerValue(config.AdvancedAnalysis.TrendingDown),
			"trending_up":   types.BoolPointerValue(config.AdvancedAnalysis.TrendingUp),
		}
		advVal, d := resource_report.NewAdvancedAnalysisValue(resource_report.AdvancedAnalysisValue{}.AttributeTypes(ctx), advMap)
		diags.Append(d...)
		configMap["advanced_analysis"] = advVal
	} else {
		configMap["advanced_analysis"] = resource_report.NewAdvancedAnalysisValueNull()
	}

	// Nested Object: CustomTimeRange
	configMap["custom_time_range"] = resource_report.NewCustomTimeRangeValueNull()

	if config.CustomTimeRange != nil {
		ctrMap := map[string]attr.Value{
			"from": types.StringNull(),
			"to":   types.StringNull(),
		}

		// Get existing custom_time_range values to preserve user's timestamp format
		var existingFrom, existingTo string
		if !state.Config.IsNull() && !state.Config.IsUnknown() &&
			!state.Config.CustomTimeRange.IsNull() && !state.Config.CustomTimeRange.IsUnknown() {
			existingFrom = state.Config.CustomTimeRange.From.ValueString()
			existingTo = state.Config.CustomTimeRange.To.ValueString()
		}

		if config.CustomTimeRange.From != nil {
			// Preserve user's timestamp format if semantically equal
			existingTime, err := time.Parse(time.RFC3339, existingFrom)
			if err == nil && datesEqualUTC(existingTime, *config.CustomTimeRange.From) {
				ctrMap["from"] = types.StringValue(existingFrom)
			} else {
				ctrMap["from"] = types.StringValue(config.CustomTimeRange.From.Format(time.RFC3339))
			}
		}
		if config.CustomTimeRange.To != nil {
			// Preserve user's timestamp format if semantically equal
			existingTime, err := time.Parse(time.RFC3339, existingTo)
			if err == nil && datesEqualUTC(existingTime, *config.CustomTimeRange.To) {
				ctrMap["to"] = types.StringValue(existingTo)
			} else {
				ctrMap["to"] = types.StringValue(config.CustomTimeRange.To.Format(time.RFC3339))
			}
		}
		ctrVal, d := resource_report.NewCustomTimeRangeValue(resource_report.CustomTimeRangeValue{}.AttributeTypes(ctx), ctrMap)
		diags.Append(d...)
		configMap["custom_time_range"] = ctrVal
	}

	// Nested List: Dimensions
	// Note: For user-configurable lists, API treats empty list and nil as equivalent.
	if config.Dimensions != nil && len(*config.Dimensions) > 0 {
		// Get existing dimension types from state for alias normalization
		var existingDimTypes []string
		if !state.Config.IsNull() && !state.Config.IsUnknown() &&
			!state.Config.Dimensions.IsNull() && !state.Config.Dimensions.IsUnknown() {
			var existingDims []resource_report.DimensionsValue
			if d := state.Config.Dimensions.ElementsAs(ctx, &existingDims, false); !d.HasError() {
				for _, ed := range existingDims {
					existingDimTypes = append(existingDimTypes, ed.DimensionsType.ValueString())
				}
			}
		}

		dims := make([]attr.Value, len(*config.Dimensions))
		for i, d := range *config.Dimensions {
			dType := string(*d.Type)
			// Normalize alias types to preserve user's configured value
			if i < len(existingDimTypes) {
				dType = normalizeDimensionsType(dType, existingDimTypes[i])
			}
			m := map[string]attr.Value{
				"id":   types.StringPointerValue(d.Id),
				"type": types.StringPointerValue(&dType),
			}
			dimVal, d := resource_report.NewDimensionsValue(resource_report.DimensionsValue{}.AttributeTypes(ctx), m)
			diags.Append(d...)
			dims[i] = dimVal
		}
		dimList, d := types.ListValueFrom(ctx, resource_report.DimensionsValue{}.Type(ctx), dims)
		diags.Append(d...)
		configMap["dimensions"] = dimList
	} else {
		// API returned nil or empty slice - return empty list.
		// For user-configurable lists, the API treats empty list and nil as equivalent,
		// so we return empty list to match user's potential config of field = [].
		var emptyDimsDiags diag.Diagnostics
		configMap["dimensions"], emptyDimsDiags = types.ListValueFrom(ctx, resource_report.DimensionsValue{}.Type(ctx), []resource_report.DimensionsValue{})
		diags.Append(emptyDimsDiags...)
	}

	// Nested List: Filters
	// Note: For user-configurable lists, API treats empty list and nil as equivalent.
	if config.Filters != nil && len(*config.Filters) > 0 {
		// Get existing filter types, IDs, includeNull, and inverse from state for alias
		// normalization and preserving user-configured values the API does not echo back.
		var existingFilterTypes []string
		var existingFilterIDs []string
		var existingFilterIncludeNull []*bool
		var existingFilterInverse []*bool
		var existingFilterCaseInsensitive []*bool
		var existingFilterValues []types.List
		if !state.Config.IsNull() && !state.Config.IsUnknown() &&
			!state.Config.Filters.IsNull() && !state.Config.Filters.IsUnknown() {
			var existingFilters []resource_report.FiltersValue
			if d := state.Config.Filters.ElementsAs(ctx, &existingFilters, false); !d.HasError() {
				for _, ef := range existingFilters {
					existingFilterTypes = append(existingFilterTypes, ef.FiltersType.ValueString())
					existingFilterIDs = append(existingFilterIDs, ef.Id.ValueString())
					existingFilterIncludeNull = append(existingFilterIncludeNull, ef.IncludeNull.ValueBoolPointer())
					existingFilterInverse = append(existingFilterInverse, ef.Inverse.ValueBoolPointer())
					existingFilterCaseInsensitive = append(existingFilterCaseInsensitive, ef.CaseInsensitive.ValueBoolPointer())
					existingFilterValues = append(existingFilterValues, ef.Values)
				}
			}
		}
		filters := make([]attr.Value, len(*config.Filters))
		for i, f := range *config.Filters {
			fType := string(f.Type)
			fID := f.Id
			// Normalize alias types and IDs to preserve user's configured value.
			// The filter ID can also be an alias (e.g. "allocation_rule" vs "attribution")
			// when it references a dimension type directly.
			if i < len(existingFilterTypes) {
				fType = normalizeDimensionsType(fType, existingFilterTypes[i])
			}
			if i < len(existingFilterIDs) {
				fID = normalizeDimensionsType(fID, existingFilterIDs[i])
			}
			// Prefer the plan/state value for includeNull when available, falling back to the
			// API response. The API does echo includeNull correctly (e.g. it returns true when
			// a NullFallback sentinel was stripped — see the sentinel restoration logic below),
			// but keeping state-first ensures round-trip stability if the field is ever
			// omitted from a response.
			includeNullVal := types.BoolValue(false)
			if i < len(existingFilterIncludeNull) && existingFilterIncludeNull[i] != nil {
				includeNullVal = types.BoolValue(*existingFilterIncludeNull[i])
			} else if f.IncludeNull != nil {
				includeNullVal = types.BoolValue(*f.IncludeNull)
			}
			// When the API doesn't echo inverse, preserve the plan/state value.
			var inverseVal attr.Value
			if f.Inverse != nil {
				inverseVal = types.BoolPointerValue(f.Inverse)
			} else if i < len(existingFilterInverse) {
				inverseVal = types.BoolPointerValue(existingFilterInverse[i])
			} else {
				inverseVal = types.BoolPointerValue(nil)
			}

			// The API may not reliably echo caseInsensitive — always prefer the plan/state
			// value when available.
			caseInsensitiveVal := types.BoolValue(false)
			if i < len(existingFilterCaseInsensitive) && existingFilterCaseInsensitive[i] != nil {
				caseInsensitiveVal = types.BoolValue(*existingFilterCaseInsensitive[i])
			} else if f.CaseInsensitive != nil {
				caseInsensitiveVal = types.BoolValue(*f.CaseInsensitive)
			}

			m := map[string]attr.Value{
				"case_insensitive": caseInsensitiveVal,
				"id":               types.StringValue(fID),
				"include_null":     includeNullVal,
				"inverse":          inverseVal,
				// filters type enum cast
				"type": types.StringValue(fType),
				"mode": types.StringPointerValue((*string)(f.Mode)),
			}

			// The API silently strips legacy "[... N/A]" NullFallback sentinels from filter
			// values and converts them to includeNull=true instead. This happens for both
			// pure-NA filters (values=[] + includeNull=true) and mixed filters where real
			// values remain (values=["AA"] + includeNull=true when "[X N/A]" was stripped).
			//
			// Strategy:
			//   1. Start with what the API returned.
			//   2. For each value that was in the prior state but is now missing from the API
			//      response, check if it is a NullFallback sentinel and includeNull flipped true.
			//      If so, the API stripped it — restore it to prevent drift.
			//   3. If the API returned nothing AND state had non-sentinel values, preserve them
			//      unchanged (handles other non-echoed fields that aren't NullFallback-related).
			//
			// This correctly distinguishes:
			//   • values stripped by API normalization  → restore sentinel
			//   • values genuinely cleared by user in UI → keep empty (includeNull stays false)
			// See: https://doitintl.atlassian.net/browse/CMP-38116
			apiIncludeNull := f.IncludeNull != nil && *f.IncludeNull
			var apiValues []string
			if f.Values != nil {
				apiValues = *f.Values
			}
			mergedValues := apiValues
			// Scan state values and restore any NullFallback sentinels the API stripped.
			if i < len(existingFilterValues) && !existingFilterValues[i].IsNull() && !existingFilterValues[i].IsUnknown() {
				var stateVals []string
				if d := existingFilterValues[i].ElementsAs(ctx, &stateVals, false); !d.HasError() {
					mergedValues = mergeSentinelValues(apiValues, stateVals, apiIncludeNull)
				}
			}
			if len(mergedValues) > 0 {
				values, d := types.ListValueFrom(ctx, types.StringType, mergedValues)
				diags.Append(d...)
				m["values"] = values
			} else {
				var emptyDiags diag.Diagnostics
				m["values"], emptyDiags = types.ListValueFrom(ctx, types.StringType, []string{})
				diags.Append(emptyDiags...)
			}
			filterVal, d := resource_report.NewFiltersValue(resource_report.FiltersValue{}.AttributeTypes(ctx), m)
			diags.Append(d...)
			filters[i] = filterVal
		}
		filterList, d := types.ListValueFrom(ctx, resource_report.FiltersValue{}.Type(ctx), filters)
		diags.Append(d...)
		configMap["filters"] = filterList
	} else {
		// API returned nil or empty slice - return empty list.
		// For user-configurable lists, the API treats empty list and nil as equivalent.
		var emptyFiltersDiags diag.Diagnostics
		configMap["filters"], emptyFiltersDiags = types.ListValueFrom(ctx, resource_report.FiltersValue{}.Type(ctx), []resource_report.FiltersValue{})
		diags.Append(emptyFiltersDiags...)
	}

	// Nested List: Group
	// Note: For user-configurable lists, API treats empty list and nil as equivalent.
	if config.Group != nil && len(*config.Group) > 0 {
		// Get existing group types and IDs from state for alias normalization
		var existingGroupTypes []string
		var existingGroupIDs []string
		if !state.Config.IsNull() && !state.Config.IsUnknown() &&
			!state.Config.Group.IsNull() && !state.Config.Group.IsUnknown() {
			var existingGroups []resource_report.GroupValue
			if d := state.Config.Group.ElementsAs(ctx, &existingGroups, false); !d.HasError() {
				for _, eg := range existingGroups {
					existingGroupTypes = append(existingGroupTypes, eg.GroupType.ValueString())
					existingGroupIDs = append(existingGroupIDs, eg.Id.ValueString())
				}
			}
		}

		groups := make([]attr.Value, len(*config.Group))
		for i, g := range *config.Group {
			groupType := string(*g.Type)
			groupID := g.Id
			// Normalize alias types and IDs to preserve user's configured value
			if i < len(existingGroupTypes) {
				groupType = normalizeDimensionsType(groupType, existingGroupTypes[i])
			}
			if groupID != nil && i < len(existingGroupIDs) {
				groupID = new(normalizeDimensionsType(*groupID, existingGroupIDs[i]))
			}
			m := map[string]attr.Value{
				"id":   types.StringPointerValue(groupID),
				"type": types.StringPointerValue(&groupType),
			}
			m["limit"] = resource_report.NewLimitValueNull()

			if g.Limit != nil {
				metricVal, d := externalMetricToBaseTypeObjectValue(ctx, g.Limit.Metric)
				diags.Append(d...)
				if diags.HasError() {
					return diags
				}
				lMap := map[string]attr.Value{
					"sort":   types.StringValue(string(*g.Limit.Sort)),
					"value":  types.Int64PointerValue(g.Limit.Value),
					"metric": metricVal,
				}
				// Nested Object in Group
				limitVal, d := resource_report.NewLimitValue(resource_report.LimitValue{}.AttributeTypes(ctx), lMap)
				diags.Append(d...)
				if diags.HasError() {
					return diags
				}
				m["limit"] = limitVal
			}
			groupVal, d := resource_report.NewGroupValue(resource_report.GroupValue{}.AttributeTypes(ctx), m)
			diags.Append(d...)
			groups[i] = groupVal
		}
		groupList, d := types.ListValueFrom(ctx, resource_report.GroupValue{}.Type(ctx), groups)
		diags.Append(d...)
		configMap["group"] = groupList
	} else {
		// API returned nil or empty slice - return empty list.
		// For user-configurable lists, the API treats empty list and nil as equivalent.
		var emptyGroupDiags diag.Diagnostics
		configMap["group"], emptyGroupDiags = types.ListValueFrom(ctx, resource_report.GroupValue{}.Type(ctx), []resource_report.GroupValue{})
		diags.Append(emptyGroupDiags...)
	}

	// Nested Object: Metric
	metricVal, d := externalMetricToBaseTypeObjectValue(ctx, config.Metric)
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}
	configMap["metric"] = metricVal

	// Nested List: Metrics (new multi-metric support)
	if config.Metrics != nil && len(*config.Metrics) > 0 {
		metricsVals := make([]attr.Value, len(*config.Metrics))
		for i, m := range *config.Metrics {
			mMap := map[string]attr.Value{
				"value": types.StringPointerValue(m.Value),
			}
			if m.Type != nil {
				mMap["type"] = types.StringValue(string(*m.Type))
			} else {
				mMap["type"] = types.StringNull()
			}
			mv, mvDiags := resource_report.NewMetricsValue(resource_report.MetricsValue{}.AttributeTypes(ctx), mMap)
			diags.Append(mvDiags...)
			if diags.HasError() {
				return diags
			}
			metricsVals[i] = mv
		}
		metricsList, metricsListDiags := types.ListValueFrom(ctx, resource_report.MetricsValue{}.Type(ctx), metricsVals)
		diags.Append(metricsListDiags...)
		if diags.HasError() {
			return diags
		}
		configMap["metrics"] = metricsList
	} else {
		// API returned no metrics - return empty list
		// (user-configurable attributes must always return empty list, not null)
		var emptyMetricsDiags diag.Diagnostics
		configMap["metrics"], emptyMetricsDiags = types.ListValueFrom(ctx, resource_report.MetricsValue{}.Type(ctx), []resource_report.MetricsValue{})
		diags.Append(emptyMetricsDiags...)
	}

	// Nested Object: MetricFilter
	if config.MetricFilter != nil {
		mfMap := map[string]attr.Value{
			"operator": types.StringValue(string(*config.MetricFilter.Operator)),
		}

		metricFilterMetricVal, mfMetricDiags := externalMetricToBaseTypeObjectValue(ctx, config.MetricFilter.Metric)
		diags.Append(mfMetricDiags...)
		if diags.HasError() {
			return diags
		}
		mfMap["metric"] = metricFilterMetricVal
		if config.MetricFilter.Values != nil {
			var mfValueDiags diag.Diagnostics
			mfMap["values"], mfValueDiags = types.ListValueFrom(ctx, types.Float64Type, *config.MetricFilter.Values)
			diags.Append(mfValueDiags...)
			if diags.HasError() {
				return diags
			}
		} else {
			var emptyDiags diag.Diagnostics
			mfMap["values"], emptyDiags = types.ListValueFrom(ctx, types.Float64Type, []float64{})
			diags.Append(emptyDiags...)
		}
		mfv, mfvDiags := resource_report.NewMetricFilterValue(resource_report.MetricFilterValue{}.AttributeTypes(ctx), mfMap)
		diags.Append(mfvDiags...)
		if diags.HasError() {
			return diags
		}
		configMap["metric_filter"] = mfv
	} else {
		configMap["metric_filter"] = resource_report.NewMetricFilterValueNull()
	}

	// Nested List: Splits
	// Note: For user-configurable lists, API treats empty list and nil as equivalent.
	if config.Splits != nil && len(*config.Splits) > 0 {
		splits := make([]attr.Value, len(*config.Splits))
		for i, s := range *config.Splits {
			m := map[string]attr.Value{
				"id":             types.StringPointerValue(s.Id),
				"include_origin": types.BoolPointerValue(s.IncludeOrigin),
				"mode":           types.StringValue(string(*s.Mode)),
				"type":           types.StringValue(string(*s.Type)),
			}

			m["origin"] = resource_report.NewOriginValueNull()

			if s.Origin != nil {
				oMap := map[string]attr.Value{
					"id":   types.StringPointerValue(s.Origin.Id),
					"type": types.StringValue(string(*s.Origin.Type)),
				}
				originVal, originDiags := resource_report.NewOriginValue(resource_report.OriginValue{}.AttributeTypes(ctx), oMap)
				diags.Append(originDiags...)
				m["origin"] = originVal
			}

			if s.Targets != nil && len(*s.Targets) > 0 {
				targets := make([]attr.Value, len(*s.Targets))
				for j, t := range *s.Targets {
					tMap := map[string]attr.Value{
						"id":    types.StringPointerValue(t.Id),
						"type":  types.StringValue(string(*t.Type)),
						"value": types.Float64PointerValue(t.Value),
					}
					targetVal, targetDiags := resource_report.NewTargetsValue(resource_report.TargetsValue{}.AttributeTypes(ctx), tMap)
					diags.Append(targetDiags...)
					targets[j] = targetVal
				}
				targetList, targetListDiags := types.ListValueFrom(ctx, resource_report.TargetsValue{}.Type(ctx), targets)
				diags.Append(targetListDiags...)
				m["targets"] = targetList
			} else {
				// Return empty list instead of null to match user config when targets = []
				emptyTargets, emptyTargetsDiags := types.ListValueFrom(ctx, resource_report.TargetsValue{}.Type(ctx), []resource_report.TargetsValue{})
				diags.Append(emptyTargetsDiags...)
				m["targets"] = emptyTargets
			}
			splitVal, splitDiags := resource_report.NewSplitsValue(resource_report.SplitsValue{}.AttributeTypes(ctx), m)
			diags.Append(splitDiags...)
			splits[i] = splitVal
		}
		splitList, splitListDiags := types.ListValueFrom(ctx, resource_report.SplitsValue{}.Type(ctx), splits)
		diags.Append(splitListDiags...)
		configMap["splits"] = splitList
	} else {
		// API returned nil or empty slice - return empty list.
		// For user-configurable lists, the API treats empty list and nil as equivalent.
		var emptySplitsDiags diag.Diagnostics
		configMap["splits"], emptySplitsDiags = types.ListValueFrom(ctx, resource_report.SplitsValue{}.Type(ctx), []resource_report.SplitsValue{})
		diags.Append(emptySplitsDiags...)
	}

	// Nested Object: TimeRange
	if config.TimeRange != nil {
		trMap := map[string]attr.Value{
			"amount":          types.Int64PointerValue(config.TimeRange.Amount),
			"include_current": types.BoolPointerValue(config.TimeRange.IncludeCurrent),
			"mode":            types.StringValue(string(*config.TimeRange.Mode)),
			"unit":            types.StringValue(string(*config.TimeRange.Unit)),
		}
		trv, trvDiags := resource_report.NewTimeRangeValue(resource_report.TimeRangeValue{}.AttributeTypes(ctx), trMap)
		diags.Append(trvDiags...)
		configMap["time_range"] = trv
	} else {
		configMap["time_range"] = resource_report.NewTimeRangeValueNull()
	}

	// Nested Object: SecondaryTimeRange
	if config.SecondaryTimeRange != nil {
		strMap := map[string]attr.Value{
			"amount":          types.Int64PointerValue(config.SecondaryTimeRange.Amount),
			"include_current": types.BoolPointerValue(config.SecondaryTimeRange.IncludeCurrent),
			"unit":            types.StringNull(),
		}
		if config.SecondaryTimeRange.Unit != nil {
			strMap["unit"] = types.StringValue(string(*config.SecondaryTimeRange.Unit))
		}

		// Nested Object: SecondaryTimeRange.CustomTimeRange
		if config.SecondaryTimeRange.CustomTimeRange != nil {
			ctrMap := map[string]attr.Value{
				"from": types.StringNull(),
				"to":   types.StringNull(),
			}

			// Get existing secondary custom_time_range values to preserve user's timestamp format
			var existingFrom, existingTo string
			if !state.Config.IsNull() && !state.Config.IsUnknown() &&
				!state.Config.SecondaryTimeRange.IsNull() && !state.Config.SecondaryTimeRange.IsUnknown() &&
				!state.Config.SecondaryTimeRange.CustomTimeRange.IsNull() && !state.Config.SecondaryTimeRange.CustomTimeRange.IsUnknown() {
				existingFrom = state.Config.SecondaryTimeRange.CustomTimeRange.From.ValueString()
				existingTo = state.Config.SecondaryTimeRange.CustomTimeRange.To.ValueString()
			}

			if config.SecondaryTimeRange.CustomTimeRange.From != nil {
				existingTime, err := time.Parse(time.RFC3339, existingFrom)
				if err == nil && datesEqualUTC(existingTime, *config.SecondaryTimeRange.CustomTimeRange.From) {
					ctrMap["from"] = types.StringValue(existingFrom)
				} else {
					ctrMap["from"] = types.StringValue(config.SecondaryTimeRange.CustomTimeRange.From.Format(time.RFC3339))
				}
			}
			if config.SecondaryTimeRange.CustomTimeRange.To != nil {
				existingTime, err := time.Parse(time.RFC3339, existingTo)
				if err == nil && datesEqualUTC(existingTime, *config.SecondaryTimeRange.CustomTimeRange.To) {
					ctrMap["to"] = types.StringValue(existingTo)
				} else {
					ctrMap["to"] = types.StringValue(config.SecondaryTimeRange.CustomTimeRange.To.Format(time.RFC3339))
				}
			}
			ctrVal, ctrDiags := resource_report.NewCustomTimeRangeValue(resource_report.CustomTimeRangeValue{}.AttributeTypes(ctx), ctrMap)
			diags.Append(ctrDiags...)
			strMap["custom_time_range"] = ctrVal
		} else {
			strMap["custom_time_range"] = resource_report.NewCustomTimeRangeValueNull()
		}

		strVal, strDiags := resource_report.NewSecondaryTimeRangeValue(resource_report.SecondaryTimeRangeValue{}.AttributeTypes(ctx), strMap)
		diags.Append(strDiags...)
		configMap["secondary_time_range"] = strVal
	} else {
		configMap["secondary_time_range"] = resource_report.NewSecondaryTimeRangeValueNull()
	}

	// Nested Object: DisplaySettings
	if config.DisplaySettings != nil {
		dsMap := map[string]attr.Value{
			"axis_label_font_size": types.StringNull(),
			"data_label_font_size": types.StringNull(),
			"decimal_precision":    types.Int64Null(),
			"number_scale":         types.StringNull(),
			// Defend against API returning nil: fall back to "default" to match
			// the schema default and prevent perpetual plan drift.
			"theme_id": types.StringValue("default"),
		}
		if config.DisplaySettings.AxisLabelFontSize != nil {
			dsMap["axis_label_font_size"] = types.StringValue(string(*config.DisplaySettings.AxisLabelFontSize))
		}
		if config.DisplaySettings.DataLabelFontSize != nil {
			dsMap["data_label_font_size"] = types.StringValue(string(*config.DisplaySettings.DataLabelFontSize))
		}
		if config.DisplaySettings.DecimalPrecision != nil {
			dsMap["decimal_precision"] = types.Int64Value(int64(*config.DisplaySettings.DecimalPrecision))
		}
		if config.DisplaySettings.NumberScale != nil {
			dsMap["number_scale"] = types.StringValue(string(*config.DisplaySettings.NumberScale))
		}
		if config.DisplaySettings.ThemeId != nil {
			dsMap["theme_id"] = types.StringValue(*config.DisplaySettings.ThemeId)
		}
		dsVal, dsDiags := resource_report.NewDisplaySettingsValue(resource_report.DisplaySettingsValue{}.AttributeTypes(ctx), dsMap)
		diags.Append(dsDiags...)
		configMap["display_settings"] = dsVal
	} else {
		configMap["display_settings"] = resource_report.NewDisplaySettingsValueNull()
	}

	// Nested Object: ForecastSettings
	fs := nullableToPointer(config.ForecastSettings)
	if fs != nil &&
		(fs.FutureTimeIntervals != nil ||
			fs.HistoricalTimeIntervals != nil ||
			fs.FutureCustomDateRange != nil ||
			fs.HistoricalCustomDateRange != nil ||
			(fs.Mode != nil && *fs.Mode == models.Grouping)) {
		fsMap := map[string]attr.Value{
			"future_custom_date_range":     resource_report.NewFutureCustomDateRangeValueNull(),
			"future_time_intervals":        types.Int64Null(),
			"historical_custom_date_range": resource_report.NewHistoricalCustomDateRangeValueNull(),
			"historical_time_intervals":    types.Int64Null(),
			"mode":                         types.StringValue("totals"),
		}

		if fs.FutureTimeIntervals != nil && fs.FutureCustomDateRange == nil {
			fsMap["future_time_intervals"] = types.Int64Value(*fs.FutureTimeIntervals)
		}
		if fs.HistoricalTimeIntervals != nil && fs.HistoricalCustomDateRange == nil {
			fsMap["historical_time_intervals"] = types.Int64Value(*fs.HistoricalTimeIntervals)
		}
		if fs.Mode != nil {
			fsMap["mode"] = types.StringValue(string(*fs.Mode))
		}

		if fs.FutureCustomDateRange != nil {
			fcdrMap := map[string]attr.Value{
				"from": types.StringNull(),
				"to":   types.StringNull(),
			}
			var existingFutureFrom, existingFutureTo string
			if !state.Config.IsNull() && !state.Config.IsUnknown() &&
				!state.Config.ForecastSettings.IsNull() && !state.Config.ForecastSettings.IsUnknown() &&
				!state.Config.ForecastSettings.FutureCustomDateRange.IsNull() && !state.Config.ForecastSettings.FutureCustomDateRange.IsUnknown() {
				existingFutureFrom = state.Config.ForecastSettings.FutureCustomDateRange.From.ValueString()
				existingFutureTo = state.Config.ForecastSettings.FutureCustomDateRange.To.ValueString()
			}

			if fs.FutureCustomDateRange.From != nil {
				existingTime, err := time.Parse(time.RFC3339, existingFutureFrom)
				if err == nil && datesEqualUTC(existingTime, *fs.FutureCustomDateRange.From) {
					fcdrMap["from"] = types.StringValue(existingFutureFrom)
				} else {
					fcdrMap["from"] = types.StringValue(fs.FutureCustomDateRange.From.UTC().Format(time.RFC3339))
				}
			}
			if fs.FutureCustomDateRange.To != nil {
				existingTime, err := time.Parse(time.RFC3339, existingFutureTo)
				if err == nil && datesEqualUTC(existingTime, *fs.FutureCustomDateRange.To) {
					fcdrMap["to"] = types.StringValue(existingFutureTo)
				} else {
					fcdrMap["to"] = types.StringValue(fs.FutureCustomDateRange.To.UTC().Format(time.RFC3339))
				}
			}
			fcdrVal, fcdrDiags := resource_report.NewFutureCustomDateRangeValue(resource_report.FutureCustomDateRangeValue{}.AttributeTypes(ctx), fcdrMap)
			diags.Append(fcdrDiags...)
			fsMap["future_custom_date_range"] = fcdrVal
		}

		if fs.HistoricalCustomDateRange != nil {
			hcdrMap := map[string]attr.Value{
				"from": types.StringNull(),
				"to":   types.StringNull(),
			}
			var existingHistFrom, existingHistTo string
			if !state.Config.IsNull() && !state.Config.IsUnknown() &&
				!state.Config.ForecastSettings.IsNull() && !state.Config.ForecastSettings.IsUnknown() &&
				!state.Config.ForecastSettings.HistoricalCustomDateRange.IsNull() && !state.Config.ForecastSettings.HistoricalCustomDateRange.IsUnknown() {
				existingHistFrom = state.Config.ForecastSettings.HistoricalCustomDateRange.From.ValueString()
				existingHistTo = state.Config.ForecastSettings.HistoricalCustomDateRange.To.ValueString()
			}

			if fs.HistoricalCustomDateRange.From != nil {
				existingTime, err := time.Parse(time.RFC3339, existingHistFrom)
				if err == nil && datesEqualUTC(existingTime, *fs.HistoricalCustomDateRange.From) {
					hcdrMap["from"] = types.StringValue(existingHistFrom)
				} else {
					hcdrMap["from"] = types.StringValue(fs.HistoricalCustomDateRange.From.UTC().Format(time.RFC3339))
				}
			}
			if fs.HistoricalCustomDateRange.To != nil {
				existingTime, err := time.Parse(time.RFC3339, existingHistTo)
				if err == nil && datesEqualUTC(existingTime, *fs.HistoricalCustomDateRange.To) {
					hcdrMap["to"] = types.StringValue(existingHistTo)
				} else {
					hcdrMap["to"] = types.StringValue(fs.HistoricalCustomDateRange.To.UTC().Format(time.RFC3339))
				}
			}
			hcdrVal, hcdrDiags := resource_report.NewHistoricalCustomDateRangeValue(resource_report.HistoricalCustomDateRangeValue{}.AttributeTypes(ctx), hcdrMap)
			diags.Append(hcdrDiags...)
			fsMap["historical_custom_date_range"] = hcdrVal
		}

		fsVal, fsDiags := resource_report.NewForecastSettingsValue(resource_report.ForecastSettingsValue{}.AttributeTypes(ctx), fsMap)
		diags.Append(fsDiags...)
		configMap["forecast_settings"] = fsVal
	} else if config.AdvancedAnalysis != nil && config.AdvancedAnalysis.Forecast != nil && *config.AdvancedAnalysis.Forecast {
		if !state.Config.IsNull() && !state.Config.IsUnknown() &&
			!state.Config.ForecastSettings.IsNull() && !state.Config.ForecastSettings.IsUnknown() {
			configMap["forecast_settings"] = state.Config.ForecastSettings
		} else {
			attrs := map[string]attr.Value{
				"future_custom_date_range":     resource_report.NewFutureCustomDateRangeValueNull(),
				"future_time_intervals":        types.Int64Null(),
				"historical_custom_date_range": resource_report.NewHistoricalCustomDateRangeValueNull(),
				"historical_time_intervals":    types.Int64Null(),
				"mode":                         types.StringValue("totals"),
			}
			fsVal, fsDiags := resource_report.NewForecastSettingsValue(resource_report.ForecastSettingsValue{}.AttributeTypes(ctx), attrs)
			diags.Append(fsDiags...)
			if !fsDiags.HasError() {
				configMap["forecast_settings"] = fsVal
			}
		}
	} else {
		configMap["forecast_settings"] = resource_report.NewForecastSettingsValueNull()
	}

	state.Config, d = resource_report.NewConfigValue(resource_report.ConfigValue{}.AttributeTypes(ctx), configMap)
	diags.Append(d...)

	return diags
}

func datesEqualUTC(t1, t2 time.Time) bool {
	u1 := t1.UTC()
	u2 := t2.UTC()
	return u1.Year() == u2.Year() && u1.Month() == u2.Month() && u1.Day() == u2.Day()
}

func baseTypeObjectValueToExternalMetric(metricValue resource_report.MetricValue) (metric *models.ExternalMetric) {
	metric = &models.ExternalMetric{}
	if !metricValue.MetricType.IsNull() {
		metric.Type = new(models.ExternalMetricType(metricValue.MetricType.ValueString()))
	}
	if !metricValue.Value.IsNull() && !metricValue.Value.IsUnknown() {
		metric.Value = new(metricValue.Value.ValueString())
	}
	return metric
}

func externalMetricToBaseTypeObjectValue(ctx context.Context, metric *models.ExternalMetric) (metricValue resource_report.MetricValue, diags diag.Diagnostics) {
	if metric == nil {
		return resource_report.NewMetricValueNull(), diags
	}
	mMap := map[string]attr.Value{
		"type":  types.StringValue(string(*metric.Type)),
		"value": types.StringPointerValue(metric.Value),
	}
	mv, d := resource_report.NewMetricValue(resource_report.MetricValue{}.AttributeTypes(ctx), mMap)
	diags = append(diags, d...)
	if diags.HasError() {
		log.Println("Error creating metric value")
		return metricValue, diags
	}
	return mv, diags
}
