package provider

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_report"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*reportDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*reportDataSource)(nil)

func NewReportDataSource() datasource.DataSource {
	return &reportDataSource{}
}

type (
	reportDataSource struct {
		client *models.ClientWithResponses
	}
	reportDataSourceModel struct {
		datasource_report.ReportModel
	}
)

func (ds *reportDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_report"
}

func (ds *reportDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_report.ReportDataSourceSchema(ctx)
}

func (ds *reportDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*models.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *models.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	ds.client = client
}

func (ds *reportDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data reportDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If ID is unknown (depends on a resource not yet created), return early
	if data.Id.IsUnknown() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	// Call API to get report config
	reportResp, err := ds.client.GetReportConfigWithResponse(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Report",
			"Could not read report ID "+data.Id.ValueString()+": "+err.Error(),
		)
		return
	}

	if reportResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Reading Report",
			fmt.Sprintf("Could not read report ID %s, status: %d, body: %s", data.Id.ValueString(), reportResp.StatusCode(), string(reportResp.Body)),
		)
		return
	}

	if reportResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Report",
			"Received empty response body for report ID "+data.Id.ValueString(),
		)
		return
	}

	report := reportResp.JSON200

	// Map API response to model
	resp.Diagnostics.Append(ds.populateState(ctx, &data, report)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// populateState maps the ExternalReport API response to the reportDataSourceModel.
// This is adapted from report.go for use with the data source schema types.
func (ds *reportDataSource) populateState(ctx context.Context, state *reportDataSourceModel, resp *models.ExternalReport) diag.Diagnostics {
	var diags diag.Diagnostics

	state.Id = types.StringPointerValue(resp.Id)
	state.Name = types.StringValue(resp.Name)
	state.Description = types.StringPointerValue(resp.Description)
	if resp.Type != nil {
		state.Type = types.StringValue(string(*resp.Type))
	} else {
		state.Type = types.StringNull()
	}

	// Labels: always return empty list instead of null
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
		state.Config = datasource_report.NewConfigValueNull()
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
		advVal, advDiags := datasource_report.NewAdvancedAnalysisValue(datasource_report.AdvancedAnalysisValue{}.AttributeTypes(ctx), advMap)
		diags.Append(advDiags...)
		configMap["advanced_analysis"] = advVal
	} else {
		configMap["advanced_analysis"] = datasource_report.NewAdvancedAnalysisValueNull()
	}

	// Nested Object: CustomTimeRange
	configMap["custom_time_range"] = datasource_report.NewCustomTimeRangeValueNull()

	if config.CustomTimeRange != nil {
		ctrMap := map[string]attr.Value{
			"from": types.StringNull(),
			"to":   types.StringNull(),
		}
		if config.CustomTimeRange.From != nil {
			ctrMap["from"] = types.StringValue(config.CustomTimeRange.From.Format(time.RFC3339))
		}
		if config.CustomTimeRange.To != nil {
			ctrMap["to"] = types.StringValue(config.CustomTimeRange.To.Format(time.RFC3339))
		}
		ctrVal, ctrDiags := datasource_report.NewCustomTimeRangeValue(datasource_report.CustomTimeRangeValue{}.AttributeTypes(ctx), ctrMap)
		diags.Append(ctrDiags...)
		configMap["custom_time_range"] = ctrVal
	}

	// Nested List: Dimensions
	if config.Dimensions != nil {
		dims := make([]attr.Value, len(*config.Dimensions))
		for i, dim := range *config.Dimensions {
			dType := string(*dim.Type)
			m := map[string]attr.Value{
				"id":   types.StringPointerValue(dim.Id),
				"type": types.StringPointerValue(&dType),
			}
			dimVal, dimDiags := datasource_report.NewDimensionsValue(datasource_report.DimensionsValue{}.AttributeTypes(ctx), m)
			diags.Append(dimDiags...)
			dims[i] = dimVal
		}
		dimList, dimListDiags := types.ListValueFrom(ctx, datasource_report.DimensionsValue{}.Type(ctx), dims)
		diags.Append(dimListDiags...)
		configMap["dimensions"] = dimList
	} else {
		emptyDims, d := types.ListValueFrom(ctx, datasource_report.DimensionsValue{}.Type(ctx), []datasource_report.DimensionsValue{})
		diags.Append(d...)
		configMap["dimensions"] = emptyDims
	}

	// Nested List: Filters
	if config.Filters != nil {
		filters := make([]attr.Value, len(*config.Filters))
		for i, f := range *config.Filters {
			fType := string(f.Type)
			m := map[string]attr.Value{
				"id":      types.StringValue(f.Id),
				"inverse": types.BoolPointerValue(f.Inverse),
				"type":    types.StringValue(fType),
				"mode":    types.StringValue(string(f.Mode)),
			}

			if f.Values != nil {
				values, valuesDiags := types.ListValueFrom(ctx, types.StringType, *f.Values)
				diags.Append(valuesDiags...)
				m["values"] = values
			} else {
				emptyList1, d := types.ListValueFrom(ctx, types.StringType, []string{})
				diags.Append(d...)
				m["values"] = emptyList1
			}
			filterVal, filterDiags := datasource_report.NewFiltersValue(datasource_report.FiltersValue{}.AttributeTypes(ctx), m)
			diags.Append(filterDiags...)
			filters[i] = filterVal
		}
		filterList, filterListDiags := types.ListValueFrom(ctx, datasource_report.FiltersValue{}.Type(ctx), filters)
		diags.Append(filterListDiags...)
		configMap["filters"] = filterList
	} else {
		emptyFilters, d := types.ListValueFrom(ctx, datasource_report.FiltersValue{}.Type(ctx), []datasource_report.FiltersValue{})
		diags.Append(d...)
		configMap["filters"] = emptyFilters
	}

	// Nested List: Group
	if config.Group != nil {
		groups := make([]attr.Value, len(*config.Group))
		for i, g := range *config.Group {
			groupType := string(*g.Type)
			m := map[string]attr.Value{
				"id":   types.StringPointerValue(g.Id),
				"type": types.StringPointerValue(&groupType),
			}
			m["limit"] = datasource_report.NewLimitValueNull()

			if g.Limit != nil {
				metricVal, metricDiags := ds.externalMetricToValue(ctx, g.Limit.Metric)
				diags.Append(metricDiags...)
				if diags.HasError() {
					return diags
				}
				lMap := map[string]attr.Value{
					"sort":   types.StringValue(string(*g.Limit.Sort)),
					"value":  types.Int64PointerValue(g.Limit.Value),
					"metric": metricVal,
				}
				limitVal, limitDiags := datasource_report.NewLimitValue(datasource_report.LimitValue{}.AttributeTypes(ctx), lMap)
				diags.Append(limitDiags...)
				if diags.HasError() {
					log.Println("Error creating limit value")
					return diags
				}
				m["limit"] = limitVal
			}
			groupVal, groupDiags := datasource_report.NewGroupValue(datasource_report.GroupValue{}.AttributeTypes(ctx), m)
			diags.Append(groupDiags...)
			groups[i] = groupVal
		}
		groupList, groupListDiags := types.ListValueFrom(ctx, datasource_report.GroupValue{}.Type(ctx), groups)
		diags.Append(groupListDiags...)
		configMap["group"] = groupList
	} else {
		emptyGroup, d := types.ListValueFrom(ctx, datasource_report.GroupValue{}.Type(ctx), []datasource_report.GroupValue{})
		diags.Append(d...)
		configMap["group"] = emptyGroup
	}

	// Nested Object: Metric (deprecated, but still supported)
	metricVal, metricDiags := ds.externalMetricToValue(ctx, config.Metric)
	diags.Append(metricDiags...)
	if diags.HasError() {
		log.Println("Error creating metric value configMap")
		return diags
	}
	configMap["metric"] = metricVal

	// Nested List: Metrics (new - up to 4 metrics per report)
	if config.Metrics != nil && len(*config.Metrics) > 0 {
		metricsVals := make([]attr.Value, len(*config.Metrics))
		for i, m := range *config.Metrics {
			// Build MetricsValue directly (not MetricValue - different type!)
			mMap := map[string]attr.Value{
				"type":  types.StringNull(),
				"value": types.StringNull(),
			}
			if m.Type != nil {
				mMap["type"] = types.StringValue(string(*m.Type))
			}
			if m.Value != nil {
				mMap["value"] = types.StringValue(*m.Value)
			}
			mVal, mDiags := datasource_report.NewMetricsValue(datasource_report.MetricsValue{}.AttributeTypes(ctx), mMap)
			diags.Append(mDiags...)
			if diags.HasError() {
				log.Println("Error creating metrics list value")
				return diags
			}
			metricsVals[i] = mVal
		}
		metricsList, metricsListDiags := types.ListValueFrom(ctx, datasource_report.MetricsValue{}.Type(ctx), metricsVals)
		diags.Append(metricsListDiags...)
		configMap["metrics"] = metricsList
	} else {
		emptyMetrics, d := types.ListValueFrom(ctx, datasource_report.MetricsValue{}.Type(ctx), []datasource_report.MetricsValue{})
		diags.Append(d...)
		configMap["metrics"] = emptyMetrics
	}

	// Nested Object: MetricFilter
	if config.MetricFilter != nil {
		mfMap := map[string]attr.Value{
			"operator": types.StringValue(string(*config.MetricFilter.Operator)),
		}

		metricFilterMetricVal, mfMetricDiags := ds.externalMetricToValue(ctx, config.MetricFilter.Metric)
		diags.Append(mfMetricDiags...)
		if diags.HasError() {
			log.Println("Error creating metric value mfMap")
			return diags
		}
		mfMap["metric"] = metricFilterMetricVal
		if config.MetricFilter.Values != nil {
			mfValues, mfValueDiags := types.ListValueFrom(ctx, types.Float64Type, *config.MetricFilter.Values)
			diags.Append(mfValueDiags...)
			if diags.HasError() {
				log.Println("Error creating metric filter values mfMap")
				return diags
			}
			mfMap["values"] = mfValues
		} else {
			emptyList2, d := types.ListValueFrom(ctx, types.Float64Type, []float64{})
			diags.Append(d...)
			mfMap["values"] = emptyList2
		}
		mfv, mfvDiags := datasource_report.NewMetricFilterValue(datasource_report.MetricFilterValue{}.AttributeTypes(ctx), mfMap)
		diags.Append(mfvDiags...)
		if diags.HasError() {
			log.Println("Error creating metric filter value")
			return diags
		}
		configMap["metric_filter"] = mfv
	} else {
		configMap["metric_filter"] = datasource_report.NewMetricFilterValueNull()
	}

	// Nested List: Splits
	if config.Splits != nil {
		splits := make([]attr.Value, len(*config.Splits))
		for i, s := range *config.Splits {
			m := map[string]attr.Value{
				"id":             types.StringPointerValue(s.Id),
				"include_origin": types.BoolPointerValue(s.IncludeOrigin),
				"mode":           types.StringValue(string(*s.Mode)),
				"type":           types.StringValue(string(*s.Type)),
			}

			m["origin"] = datasource_report.NewOriginValueNull()

			if s.Origin != nil {
				oMap := map[string]attr.Value{
					"id":   types.StringPointerValue(s.Origin.Id),
					"type": types.StringValue(string(*s.Origin.Type)),
				}
				originVal, originDiags := datasource_report.NewOriginValue(datasource_report.OriginValue{}.AttributeTypes(ctx), oMap)
				diags.Append(originDiags...)
				m["origin"] = originVal
			}

			if s.Targets != nil {
				targets := make([]attr.Value, len(*s.Targets))
				for j, t := range *s.Targets {
					tMap := map[string]attr.Value{
						"id":    types.StringPointerValue(t.Id),
						"type":  types.StringValue(string(*t.Type)),
						"value": types.Float64PointerValue(t.Value),
					}
					targetVal, targetDiags := datasource_report.NewTargetsValue(datasource_report.TargetsValue{}.AttributeTypes(ctx), tMap)
					diags.Append(targetDiags...)
					targets[j] = targetVal
				}
				targetList, targetListDiags := types.ListValueFrom(ctx, datasource_report.TargetsValue{}.Type(ctx), targets)
				diags.Append(targetListDiags...)
				m["targets"] = targetList
			} else {
				emptyTargets, d := types.ListValueFrom(ctx, datasource_report.TargetsValue{}.Type(ctx), []datasource_report.TargetsValue{})
				diags.Append(d...)
				m["targets"] = emptyTargets
			}
			splitVal, splitDiags := datasource_report.NewSplitsValue(datasource_report.SplitsValue{}.AttributeTypes(ctx), m)
			diags.Append(splitDiags...)
			splits[i] = splitVal
		}
		splitList, splitListDiags := types.ListValueFrom(ctx, datasource_report.SplitsValue{}.Type(ctx), splits)
		diags.Append(splitListDiags...)
		configMap["splits"] = splitList
	} else {
		emptySplits, d := types.ListValueFrom(ctx, datasource_report.SplitsValue{}.Type(ctx), []datasource_report.SplitsValue{})
		diags.Append(d...)
		configMap["splits"] = emptySplits
	}

	// Nested Object: TimeRange
	if config.TimeRange != nil {
		trMap := map[string]attr.Value{
			"amount":          types.Int64PointerValue(config.TimeRange.Amount),
			"include_current": types.BoolPointerValue(config.TimeRange.IncludeCurrent),
			"mode":            types.StringValue(string(*config.TimeRange.Mode)),
			"unit":            types.StringValue(string(*config.TimeRange.Unit)),
		}
		trv, trvDiags := datasource_report.NewTimeRangeValue(datasource_report.TimeRangeValue{}.AttributeTypes(ctx), trMap)
		diags.Append(trvDiags...)
		configMap["time_range"] = trv
	} else {
		configMap["time_range"] = datasource_report.NewTimeRangeValueNull()
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

		if config.SecondaryTimeRange.CustomTimeRange != nil {
			ctrMap := map[string]attr.Value{
				"from": types.StringNull(),
				"to":   types.StringNull(),
			}
			if config.SecondaryTimeRange.CustomTimeRange.From != nil {
				ctrMap["from"] = types.StringValue(config.SecondaryTimeRange.CustomTimeRange.From.Format(time.RFC3339))
			}
			if config.SecondaryTimeRange.CustomTimeRange.To != nil {
				ctrMap["to"] = types.StringValue(config.SecondaryTimeRange.CustomTimeRange.To.Format(time.RFC3339))
			}
			ctrVal, ctrDiags := datasource_report.NewCustomTimeRangeValue(datasource_report.CustomTimeRangeValue{}.AttributeTypes(ctx), ctrMap)
			diags.Append(ctrDiags...)
			strMap["custom_time_range"] = ctrVal
		} else {
			strMap["custom_time_range"] = datasource_report.NewCustomTimeRangeValueNull()
		}

		strVal, strDiags := datasource_report.NewSecondaryTimeRangeValue(datasource_report.SecondaryTimeRangeValue{}.AttributeTypes(ctx), strMap)
		diags.Append(strDiags...)
		configMap["secondary_time_range"] = strVal
	} else {
		configMap["secondary_time_range"] = datasource_report.NewSecondaryTimeRangeValueNull()
	}

	var configDiags diag.Diagnostics
	state.Config, configDiags = datasource_report.NewConfigValue(datasource_report.ConfigValue{}.AttributeTypes(ctx), configMap)
	diags.Append(configDiags...)

	return diags
}

// externalMetricToValue converts an API ExternalMetric to a datasource_report MetricValue.
func (ds *reportDataSource) externalMetricToValue(ctx context.Context, metric *models.ExternalMetric) (datasource_report.MetricValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	if metric == nil {
		return datasource_report.NewMetricValueNull(), diags
	}
	mMap := map[string]attr.Value{
		"type":  types.StringValue(string(*metric.Type)),
		"value": types.StringPointerValue(metric.Value),
	}
	mv, mvDiags := datasource_report.NewMetricValue(datasource_report.MetricValue{}.AttributeTypes(ctx), mMap)
	diags.Append(mvDiags...)
	if diags.HasError() {
		log.Println("Error creating metric value")
		return datasource_report.MetricValue{}, diags
	}
	return mv, diags
}
