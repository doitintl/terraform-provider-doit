package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_alerts"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*alertsDataSource)(nil)

func NewAlertsDataSource() datasource.DataSource {
	return &alertsDataSource{}
}

type alertsDataSource struct {
	client *models.ClientWithResponses
}

type alertsDataSourceModel struct {
	datasource_alerts.AlertsModel
}

func (d *alertsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_alerts"
}

func (d *alertsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_alerts.AlertsDataSourceSchema(ctx)
}

func (d *alertsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*models.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *models.ClientWithResponses, got: %T.", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *alertsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data alertsDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If any filter/pagination input is unknown, return unknown list
	if data.Filter.IsUnknown() || data.SortBy.IsUnknown() || data.SortOrder.IsUnknown() || data.MaxResults.IsUnknown() || data.PageToken.IsUnknown() {
		data.Alerts = types.ListUnknown(datasource_alerts.AlertsValue{}.Type(ctx))
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}
	// Build query parameters
	params := &models.ListAlertsParams{}
	if !data.Filter.IsNull() {
		filter := data.Filter.ValueString()
		params.Filter = &filter
	}
	if !data.SortBy.IsNull() {
		sortBy := models.ListAlertsParamsSortBy(data.SortBy.ValueString())
		params.SortBy = &sortBy
	}
	if !data.SortOrder.IsNull() {
		sortOrder := models.ListAlertsParamsSortOrder(data.SortOrder.ValueString())
		params.SortOrder = &sortOrder
	}

	// Smart pagination: honor user-provided values, otherwise auto-paginate
	userControlsPagination := !data.MaxResults.IsNull()

	var allAlerts []models.Alert

	if userControlsPagination {
		// Manual mode: single API call with user's params
		maxResultsVal := data.MaxResults.ValueString()
		params.MaxResults = &maxResultsVal
		if !data.PageToken.IsNull() {
			pageTokenVal := data.PageToken.ValueString()
			params.PageToken = &pageTokenVal
		}

		apiResp, err := d.client.ListAlertsWithResponse(ctx, params)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Reading Alerts",
				fmt.Sprintf("Unable to read alerts: %v", err),
			)
			return
		}

		if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
			resp.Diagnostics.AddError(
				"Error Reading Alerts",
				fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
			)
			return
		}

		result := apiResp.JSON200
		if result.Alerts != nil {
			allAlerts = *result.Alerts
		}

		// Preserve API's page_token for user to fetch next page
		data.PageToken = types.StringPointerValue(result.PageToken)
		if result.RowCount != nil {
			data.RowCount = types.Int64Value(*result.RowCount)
		} else {
			data.RowCount = types.Int64Value(int64(len(allAlerts)))
		}
		// max_results is already set by user, no change needed
	} else {
		// Auto mode: fetch all pages, honoring user-provided page_token as starting point
		if !data.PageToken.IsNull() {
			pageTokenVal := data.PageToken.ValueString()
			params.PageToken = &pageTokenVal
		}
		for {
			apiResp, err := d.client.ListAlertsWithResponse(ctx, params)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error Reading Alerts",
					fmt.Sprintf("Unable to read alerts: %v", err),
				)
				return
			}

			if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
				resp.Diagnostics.AddError(
					"Error Reading Alerts",
					fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
				)
				return
			}

			result := apiResp.JSON200
			if result.Alerts != nil {
				allAlerts = append(allAlerts, *result.Alerts...)
			}

			if result.PageToken == nil || *result.PageToken == "" {
				break
			}
			params.PageToken = result.PageToken
		}

		// Auto mode: set counts based on what we fetched
		data.RowCount = types.Int64Value(int64(len(allAlerts)))
		data.PageToken = types.StringNull()
		// max_results was not set; preserve null
	}

	// Map alerts list
	if len(allAlerts) > 0 {
		alertVals := make([]datasource_alerts.AlertsValue, 0, len(allAlerts))
		for _, alert := range allAlerts {
			// Map nested config
			configVal := mapAlertConfig(ctx, alert.Config, &resp.Diagnostics)

			// Handle recipients list
			var recipientsList types.List
			if alert.Recipients != nil {
				recipientVals := make([]attr.Value, 0, len(*alert.Recipients))
				for _, r := range *alert.Recipients {
					recipientVals = append(recipientVals, types.StringValue(r))
				}
				var diags diag.Diagnostics
				recipientsList, diags = types.ListValue(types.StringType, recipientVals)
				resp.Diagnostics.Append(diags...)
			} else {
				emptyList1, d := types.ListValueFrom(ctx, types.StringType, []string{})
				resp.Diagnostics.Append(d...)
				recipientsList = emptyList1
			}

			alertVal, diags := datasource_alerts.NewAlertsValue(
				datasource_alerts.AlertsValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"id":           types.StringPointerValue(alert.Id),
					"name":         types.StringValue(alert.Name),
					"create_time":  types.Int64PointerValue(alert.CreateTime),
					"update_time":  types.Int64PointerValue(alert.UpdateTime),
					"last_alerted": types.Int64PointerValue(alert.LastAlerted),
					"recipients":   recipientsList,
					"config":       configVal,
				},
			)
			resp.Diagnostics.Append(diags...)
			alertVals = append(alertVals, alertVal)
		}

		alertList, diags := types.ListValueFrom(ctx, datasource_alerts.AlertsValue{}.Type(ctx), alertVals)
		resp.Diagnostics.Append(diags...)
		data.Alerts = alertList
	} else {
		emptyList, diags := types.ListValueFrom(ctx, datasource_alerts.AlertsValue{}.Type(ctx), []datasource_alerts.AlertsValue{})
		resp.Diagnostics.Append(diags...)
		data.Alerts = emptyList
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// mapAlertConfig maps API AlertConfig to Terraform ConfigValue.
func mapAlertConfig(ctx context.Context, config *models.AlertConfig, diagnostics *diag.Diagnostics) datasource_alerts.ConfigValue {
	if config == nil {
		return datasource_alerts.NewConfigValueNull()
	}

	// Map attributions list
	var attributionsList types.List
	if config.Attributions != nil {
		attrVals := make([]attr.Value, 0, len(*config.Attributions))
		for _, a := range *config.Attributions {
			attrVals = append(attrVals, types.StringValue(a))
		}
		var diags diag.Diagnostics
		attributionsList, diags = types.ListValue(types.StringType, attrVals)
		diagnostics.Append(diags...)
	} else {
		emptyAttrs, d := types.ListValueFrom(ctx, types.StringType, []string{})
		diagnostics.Append(d...)
		attributionsList = emptyAttrs
	}

	// Map scopes list
	scopesList := mapAlertScopes(ctx, config.Scopes, diagnostics)

	// Map metric
	metricVal := mapAlertMetric(ctx, config.Metric, diagnostics)

	// Map condition (Condition type to string)
	var conditionVal types.String
	if config.Condition != nil {
		conditionVal = types.StringValue(*config.Condition)
	} else {
		conditionVal = types.StringNull()
	}

	// Map currency enum
	var currencyVal types.String
	if config.Currency != nil {
		currencyVal = types.StringValue(string(*config.Currency))
	} else {
		currencyVal = types.StringNull()
	}

	configVal, diags := datasource_alerts.NewConfigValue(
		datasource_alerts.ConfigValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"attributions":      attributionsList,
			"condition":         conditionVal,
			"currency":          currencyVal,
			"data_source":       types.StringPointerValue(config.DataSource),
			"evaluate_for_each": types.StringPointerValue(config.EvaluateForEach),
			"metric":            metricVal,
			"operator": func() types.String {
				if config.Operator != nil {
					return types.StringValue(string(*config.Operator))
				}
				return types.StringNull()
			}(),
			"scopes":        scopesList,
			"time_interval": types.StringValue(string(config.TimeInterval)),
			"value":         types.Float64Value(config.Value),
		},
	)
	diagnostics.Append(diags...)
	return configVal
}

// mapAlertMetric maps API MetricConfig to Terraform MetricValue.
func mapAlertMetric(ctx context.Context, metric models.MetricConfig, diagnostics *diag.Diagnostics) datasource_alerts.MetricValue {
	metricVal, diags := datasource_alerts.NewMetricValue(
		datasource_alerts.MetricValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"type":  types.StringValue(metric.Type),
			"value": types.StringValue(metric.Value),
		},
	)
	diagnostics.Append(diags...)
	return metricVal
}

// mapAlertScopes maps API ExternalConfigFilter slice to Terraform scopes list.
func mapAlertScopes(ctx context.Context, scopes *[]models.ExternalConfigFilter, diagnostics *diag.Diagnostics) types.List {
	if scopes == nil || len(*scopes) == 0 {
		emptyScopesList, d := types.ListValueFrom(ctx, datasource_alerts.ScopesValue{}.Type(ctx), []datasource_alerts.ScopesValue{})
		diagnostics.Append(d...)
		return emptyScopesList
	}

	vals := make([]datasource_alerts.ScopesValue, 0, len(*scopes))
	for _, s := range *scopes {
		// Handle values list
		var valuesList types.List
		if s.Values != nil {
			valStrs := make([]attr.Value, 0, len(*s.Values))
			for _, v := range *s.Values {
				valStrs = append(valStrs, types.StringValue(v))
			}
			var diags diag.Diagnostics
			valuesList, diags = types.ListValue(types.StringType, valStrs)
			diagnostics.Append(diags...)
		} else {
			emptyVals, d := types.ListValueFrom(ctx, types.StringType, []string{})
			diagnostics.Append(d...)
			valuesList = emptyVals
		}

		scopeVal, diags := datasource_alerts.NewScopesValue(
			datasource_alerts.ScopesValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"id":      types.StringValue(s.Id),
				"inverse": types.BoolPointerValue(s.Inverse),
				"mode":    types.StringValue(string(s.Mode)),
				"type":    types.StringValue(string(s.Type)),
				"values":  valuesList,
			},
		)
		diagnostics.Append(diags...)
		vals = append(vals, scopeVal)
	}

	list, diags := types.ListValueFrom(ctx, datasource_alerts.ScopesValue{}.Type(ctx), vals)
	diagnostics.Append(diags...)
	return list
}
