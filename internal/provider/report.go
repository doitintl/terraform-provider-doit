package provider

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_report"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// populateStateFromAPI fetches the report from the API and populates the Terraform state.
//
// # 404 Handling Strategy
//
// The allowNotFound parameter controls how 404 responses are handled:
//
//   - allowNotFound=true (used by Read):
//     404 means the resource was deleted externally (outside Terraform).
//     We set state.Id to null, which signals Terraform to remove the resource
//     from state. On next plan, Terraform will propose recreating it.
//     This is the standard Terraform pattern for "externally deleted" resources.
//
//   - allowNotFound=false (used by Create and Update):
//     404 is unexpected and indicates an error. After a successful Create or
//     Update API call, the resource MUST exist. A 404 here could indicate:
//
//   - A transient API issue (rare, but possible)
//
//   - An eventual consistency problem
//
//   - A bug in the provider or API
//     In these cases, we return an error so the user knows something went wrong
//     and can retry. This prevents silent resource orphaning.
//
// # Why This Matters
//
// Without this distinction, a transient 404 during Create would:
//  1. Create the resource successfully (API returns 201 with ID)
//  2. GET returns 404 (transient issue)
//  3. populateStateFromAPI sets state.Id = null (no error!)
//  4. Terraform "succeeds" but loses track of the resource
//  5. Resource is orphaned - exists in API but not in Terraform state
//
// With allowNotFound=false for Create/Update, step 3 returns an error,
// the user sees the failure, and can retry or investigate.
func (r *reportResource) populateStateFromAPI(ctx context.Context, id string, state *reportResourceModel, allowNotFound bool) diag.Diagnostics {
	reportResp, err := r.client.GetReportConfigWithResponse(ctx, id)
	if err != nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error reading report", "Could not read report config, unexpected error: "+err.Error()),
		}
	}

	// Handle 404 based on context
	if reportResp.StatusCode() == 404 {
		if allowNotFound {
			// Read context: Resource was deleted externally, mark for removal from state
			state.Id = types.StringNull()
			return nil
		}
		// Create/Update context: Resource should exist, 404 is an error
		return diag.Diagnostics{
			diag.NewErrorDiagnostic(
				"Resource not found after operation",
				"The report was successfully created/updated but could not be read back (404). "+
					"This may indicate a transient API issue. Please retry the operation. "+
					"If the problem persists, the resource may need to be imported manually. "+
					"Report ID: "+id,
			),
		}
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

	return r.populateState(ctx, state, reportResp.JSON200)
}

func (plan *reportResourceModel) toCreateRequest(ctx context.Context) (req models.CreateReportJSONRequestBody, diags diag.Diagnostics) {
	req.Name = plan.Name.ValueStringPointer()
	req.Description = plan.Description.ValueStringPointer()

	config, d := toExternalConfig(ctx, plan.Config)
	diags.Append(d...)
	if diags.HasError() {
		return req, diags
	}
	req.Config = config

	return req, diags
}

func (plan *reportResourceModel) toUpdateRequest(ctx context.Context) (req models.UpdateReportJSONRequestBody, diags diag.Diagnostics) {
	req.Name = plan.Name.ValueStringPointer()
	req.Description = plan.Description.ValueStringPointer()

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
	aggregation := models.ExternalConfigAggregation(config.Aggregation.ValueString())
	currency := models.Currency(config.Currency.ValueString())
	displayValues := models.ExternalConfigDisplayValues(config.DisplayValues.ValueString())
	includePromotionalCredits := config.IncludePromotionalCredits.ValueBool()
	layout := models.ExternalRenderer(config.Layout.ValueString())
	sortDimensions := models.ExternalConfigSortDimensions(config.SortDimensions.ValueString())
	sortGroups := models.ExternalConfigSortGroups(config.SortGroups.ValueString())
	timeInterval := models.ExternalConfigTimeInterval(config.TimeInterval.ValueString())
	externalConfig = &models.ExternalConfig{
		Aggregation:               &aggregation,
		Currency:                  &currency,
		DisplayValues:             &displayValues,
		IncludePromotionalCredits: &includePromotionalCredits,
		Layout:                    &layout,
		SortDimensions:            &sortDimensions,
		SortGroups:                &sortGroups,
		TimeInterval:              &timeInterval,
	}

	if !config.DataSource.IsNull() && !config.DataSource.IsUnknown() {
		dataSource := models.ExternalConfigDataSource(config.DataSource.ValueString())
		externalConfig.DataSource = &dataSource
	}

	if !config.AdvancedAnalysis.IsNull() && !config.AdvancedAnalysis.IsUnknown() {
		externalConfig.AdvancedAnalysis = &models.AdvancedAnalysis{
			Forecast:     config.AdvancedAnalysis.Forecast.ValueBoolPointer(),
			NotTrending:  config.AdvancedAnalysis.NotTrending.ValueBoolPointer(),
			TrendingDown: config.AdvancedAnalysis.TrendingDown.ValueBoolPointer(),
			TrendingUp:   config.AdvancedAnalysis.TrendingUp.ValueBoolPointer(),
		}
	}

	if !config.CustomTimeRange.IsNull() && !config.CustomTimeRange.IsUnknown() {
		fromTime, err := time.Parse(time.RFC3339, config.CustomTimeRange.From.ValueString())
		if err != nil {
			diags.AddError("Invalid From Time", "Could not parse CustomTimeRange.From as RFC3339: "+err.Error())
		}
		toTime, err := time.Parse(time.RFC3339, config.CustomTimeRange.To.ValueString())
		if err != nil {
			diags.AddError("Invalid To Time", "Could not parse CustomTimeRange.To as RFC3339: "+err.Error())
		}

		if !diags.HasError() {
			customTimeRange := models.ExternalConfigCustomTimeRange{
				From: &fromTime,
				To:   &toTime,
			}
			externalConfig.CustomTimeRange = &customTimeRange
		}
	}

	if !config.Dimensions.IsNull() && !config.Dimensions.IsUnknown() {
		var dimensions []resource_report.DimensionsValue
		diags.Append(config.Dimensions.ElementsAs(ctx, &dimensions, false)...)
		if !diags.HasError() {
			externalDimensions := make([]models.Dimension, len(dimensions))
			for i, d := range dimensions {
				dimType := models.DimensionsTypes(d.DimensionsType.ValueString())
				externalDimensions[i] = models.Dimension{
					Id:   d.Id.ValueStringPointer(),
					Type: &dimType,
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
				inverse := f.Inverse.ValueBool()
				filterType := models.DimensionsTypes(f.FiltersType.ValueString())
				externalFilters[i] = models.ExternalConfigFilter{
					Id:      f.Id.ValueString(),
					Inverse: &inverse,
					Type:    filterType,
				}
				if !f.Values.IsNull() && !f.Values.IsUnknown() {
					var values []string
					diags.Append(f.Values.ElementsAs(ctx, &values, false)...)
					if diags.HasError() {
						continue
					}
					externalFilters[i].Values = &values
				}
				externalFilters[i].Mode = models.ExternalConfigFilterMode(f.Mode.ValueString())
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
				groupType := models.DimensionsTypes(g.GroupType.ValueString())
				externalGroups[i] = models.Group{
					Id:   g.Id.ValueStringPointer(),
					Type: &groupType,
				}
				if !g.Limit.IsNull() && !g.Limit.IsUnknown() {
					limit := models.Limit{}
					if !g.Limit.Sort.IsNull() && !g.Limit.Sort.IsUnknown() {
						sort := models.LimitSort(g.Limit.Sort.ValueString())
						limit.Sort = &sort
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

	if !config.Metric.IsNull() && !config.Metric.IsUnknown() {
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
					mType := models.ExternalMetricType(m.MetricsType.ValueString())
					externalMetrics[i].Type = &mType
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
			operator := models.ExternalConfigMetricFilterOperator(config.MetricFilter.Operator.ValueString())
			externalConfig.MetricFilter.Operator = &operator
		}
	}

	if !config.Splits.IsNull() && !config.Splits.IsUnknown() {
		var splits []resource_report.SplitsValue
		diags.Append(config.Splits.ElementsAs(ctx, &splits, false)...)
		if !diags.HasError() {
			externalSplits := make([]models.ExternalSplit, len(splits))
			for i, s := range splits {
				splitMode := models.ExternalSplitMode(s.Mode.ValueString())
				splitType := models.ExternalSplitType(s.SplitsType.ValueString())
				externalSplits[i] = models.ExternalSplit{
					Id:            s.Id.ValueStringPointer(),
					IncludeOrigin: s.IncludeOrigin.ValueBoolPointer(),
					Mode:          &splitMode,
					Type:          &splitType,
				}
				if !s.Origin.IsNull() && !s.Origin.IsUnknown() {
					originType := models.ExternalOriginType(s.Origin.OriginType.ValueString())
					externalSplits[i].Origin = &models.ExternalOrigin{
						Id:   s.Origin.Id.ValueStringPointer(),
						Type: &originType,
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
						targetType := models.ExternalSplitTargetType(t.TargetsType.ValueString())
						externalTargets[j] = models.ExternalSplitTarget{
							Id:    t.Id.ValueStringPointer(),
							Type:  &targetType,
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
		timeSettingsMode := models.TimeSettingsMode(config.TimeRange.Mode.ValueString())
		timeSettingsUnit := models.TimeSettingsUnit(config.TimeRange.Unit.ValueString())
		externalConfig.TimeRange = &models.TimeSettings{
			Amount:         config.TimeRange.Amount.ValueInt64Pointer(),
			IncludeCurrent: config.TimeRange.IncludeCurrent.ValueBoolPointer(),
			Mode:           &timeSettingsMode,
			Unit:           &timeSettingsUnit,
		}
	}

	if !config.SecondaryTimeRange.IsNull() && !config.SecondaryTimeRange.IsUnknown() {
		secondaryTimeRange := &models.TimeSettingsSecondary{
			Amount:         config.SecondaryTimeRange.Amount.ValueInt64Pointer(),
			IncludeCurrent: config.SecondaryTimeRange.IncludeCurrent.ValueBoolPointer(),
		}
		if !config.SecondaryTimeRange.Unit.IsNull() && !config.SecondaryTimeRange.Unit.IsUnknown() {
			unit := models.TimeSettingsSecondaryUnit(config.SecondaryTimeRange.Unit.ValueString())
			secondaryTimeRange.Unit = &unit
		}
		if !config.SecondaryTimeRange.CustomTimeRange.IsNull() && !config.SecondaryTimeRange.CustomTimeRange.IsUnknown() {
			fromTime, err := time.Parse(time.RFC3339, config.SecondaryTimeRange.CustomTimeRange.From.ValueString())
			if err != nil {
				diags.AddError("Invalid From Time", "Could not parse SecondaryTimeRange.CustomTimeRange.From as RFC3339: "+err.Error())
			}
			toTime, err := time.Parse(time.RFC3339, config.SecondaryTimeRange.CustomTimeRange.To.ValueString())
			if err != nil {
				diags.AddError("Invalid To Time", "Could not parse SecondaryTimeRange.CustomTimeRange.To as RFC3339: "+err.Error())
			}
			if !diags.HasError() {
				ctr := models.TimeSettingsSecondaryCustomTimeRange{
					From: &fromTime,
					To:   &toTime,
				}
				secondaryTimeRange.CustomTimeRange = &ctr
			}
		}
		externalConfig.SecondaryTimeRange = secondaryTimeRange
	}

	return externalConfig, diags
}

func (r *reportResource) populateState(ctx context.Context, state *reportResourceModel, resp *models.ExternalReport) diag.Diagnostics {
	var diags diag.Diagnostics

	state.Id = types.StringPointerValue(resp.Id)
	state.Name = types.StringValue(resp.Name)
	state.Description = types.StringPointerValue(resp.Description)
	if resp.Type != nil {
		state.Type = types.StringValue(string(*resp.Type))
	} else {
		state.Type = types.StringNull()
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
			if err == nil && existingTime.Equal(*config.CustomTimeRange.From) {
				ctrMap["from"] = types.StringValue(existingFrom)
			} else {
				ctrMap["from"] = types.StringValue(config.CustomTimeRange.From.Format(time.RFC3339))
			}
		}
		if config.CustomTimeRange.To != nil {
			// Preserve user's timestamp format if semantically equal
			existingTime, err := time.Parse(time.RFC3339, existingTo)
			if err == nil && existingTime.Equal(*config.CustomTimeRange.To) {
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
		dims := make([]attr.Value, len(*config.Dimensions))
		for i, d := range *config.Dimensions {
			dType := string(*d.Type)
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
		filters := make([]attr.Value, len(*config.Filters))
		for i, f := range *config.Filters {
			fType := string(f.Type)
			m := map[string]attr.Value{
				"id":      types.StringValue(f.Id),
				"inverse": types.BoolPointerValue(f.Inverse),
				// filters type enum cast
				"type": types.StringValue(fType),
				"mode": types.StringValue(string(f.Mode)),
			}

			if f.Values != nil {
				values, d := types.ListValueFrom(ctx, types.StringType, *f.Values)
				diags.Append(d...)
				m["values"] = values
			} else {
				m["values"] = types.ListNull(types.StringType)
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
		groups := make([]attr.Value, len(*config.Group))
		for i, g := range *config.Group {
			groupType := string(*g.Type)
			m := map[string]attr.Value{
				"id":   types.StringPointerValue(g.Id),
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
			mfMap["values"] = types.ListNull(types.Float64Type)
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
				if err == nil && existingTime.Equal(*config.SecondaryTimeRange.CustomTimeRange.From) {
					ctrMap["from"] = types.StringValue(existingFrom)
				} else {
					ctrMap["from"] = types.StringValue(config.SecondaryTimeRange.CustomTimeRange.From.Format(time.RFC3339))
				}
			}
			if config.SecondaryTimeRange.CustomTimeRange.To != nil {
				existingTime, err := time.Parse(time.RFC3339, existingTo)
				if err == nil && existingTime.Equal(*config.SecondaryTimeRange.CustomTimeRange.To) {
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

	state.Config, d = resource_report.NewConfigValue(resource_report.ConfigValue{}.AttributeTypes(ctx), configMap)
	diags.Append(d...)

	return diags
}

func baseTypeObjectValueToExternalMetric(metricValue resource_report.MetricValue) (metric *models.ExternalMetric) {
	metric = &models.ExternalMetric{}
	if !metricValue.MetricType.IsNull() {
		tString := models.ExternalMetricType(metricValue.MetricType.ValueString())
		metric.Type = &tString
	}
	if !metricValue.Value.IsNull() {
		vString := metricValue.Value.ValueString()
		metric.Value = &vString
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
