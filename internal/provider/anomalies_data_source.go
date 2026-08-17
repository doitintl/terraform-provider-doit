package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_anomalies"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*anomaliesDataSource)(nil)

func NewAnomaliesDataSource() datasource.DataSource {
	return &anomaliesDataSource{}
}

type anomaliesDataSource struct {
	client *models.ClientWithResponses
}

type anomaliesDataSourceModel struct {
	datasource_anomalies.AnomaliesModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *anomaliesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_anomalies"
}

func (d *anomaliesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_anomalies.AnomaliesDataSourceSchema(ctx)

	s.Attributes["timeouts"] = timeouts.Attributes(ctx)

	resp.Schema = s
}

func (d *anomaliesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *anomaliesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data anomaliesDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	readTimeout, diags := data.Timeouts.Read(ctx, DefaultReadTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	// If any filter/pagination input is unknown, return unknown list and unknown computed attributes
	if data.Filter.IsUnknown() || data.MinCreationTime.IsUnknown() || data.MaxCreationTime.IsUnknown() || data.MaxResults.IsUnknown() || data.PageToken.IsUnknown() || data.IncludeNotifications.IsUnknown() || data.SortBy.IsUnknown() || data.SortOrder.IsUnknown() {
		data.Anomalies = types.ListUnknown(datasource_anomalies.AnomaliesValue{}.Type(ctx))
		data.AnomalySummary = datasource_anomalies.NewAnomalySummaryValueUnknown()
		data.RowCount = types.Int64Unknown()
		data.TotalCount = types.Int64Unknown()
		data.TotalCountExact = types.BoolUnknown()
		data.Truncated = types.BoolUnknown()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}
	// Build query parameters
	params := &models.ListAnomaliesParams{}
	if !data.Filter.IsNull() {
		params.Filter = new(data.Filter.ValueString())
	}
	if !data.MinCreationTime.IsNull() {
		params.MinCreationTime = new(data.MinCreationTime.ValueInt64())
	}
	if !data.MaxCreationTime.IsNull() {
		params.MaxCreationTime = new(data.MaxCreationTime.ValueInt64())
	}
	if !data.IncludeNotifications.IsNull() {
		params.IncludeNotifications = data.IncludeNotifications.ValueBoolPointer()
	}
	if !data.SortBy.IsNull() {
		params.SortBy = new(models.ListAnomaliesParamsSortBy(data.SortBy.ValueString()))
	}
	if !data.SortOrder.IsNull() {
		params.SortOrder = new(models.ListAnomaliesParamsSortOrder(data.SortOrder.ValueString()))
	}

	// Smart pagination: honor user-provided values, otherwise auto-paginate
	userControlsPagination := !data.MaxResults.IsNull()

	var allAnomalies []models.AnomalyItem
	var finalSummary models.AnomaliesResponseAnomalySummary
	var totalCount int64
	var totalCountExact bool
	var truncated bool

	if userControlsPagination {
		// Manual mode: single API call with user's params
		params.MaxResults = new(data.MaxResults.ValueInt64())
		if !data.PageToken.IsNull() {
			params.PageToken = new(data.PageToken.ValueString())
		}

		apiResp, err := d.client.ListAnomaliesWithResponse(ctx, params)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Reading Anomalies",
				fmt.Sprintf("Unable to read anomalies: %v", err),
			)
			return
		}

		if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
			resp.Diagnostics.AddError(
				"Error Reading Anomalies",
				fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
			)
			return
		}

		result := apiResp.JSON200
		allAnomalies = result.Anomalies
		finalSummary = result.AnomalySummary
		totalCount = result.TotalCount
		totalCountExact = result.TotalCountExact
		truncated = result.Truncated

		// Preserve API's page_token for user to fetch next page
		if result.PageToken != nil && *result.PageToken != "" {
			data.PageToken = types.StringValue(*result.PageToken)
		} else {
			data.PageToken = types.StringNull()
		}
		data.RowCount = types.Int64Value(result.RowCount)
		data.TotalCount = types.Int64Value(totalCount)
		data.TotalCountExact = types.BoolValue(totalCountExact)
		data.Truncated = types.BoolValue(truncated)
		// max_results is already set by user, no change needed
	} else {
		// Auto mode: fetch all pages, honoring user-provided page_token as starting point
		if !data.PageToken.IsNull() {
			params.PageToken = new(data.PageToken.ValueString())
		}
		for {
			apiResp, err := d.client.ListAnomaliesWithResponse(ctx, params)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error Reading Anomalies",
					fmt.Sprintf("Unable to read anomalies: %v", err),
				)
				return
			}

			if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
				resp.Diagnostics.AddError(
					"Error Reading Anomalies",
					fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
				)
				return
			}

			result := apiResp.JSON200
			allAnomalies = append(allAnomalies, result.Anomalies...)
			finalSummary = result.AnomalySummary
			totalCount = result.TotalCount
			totalCountExact = result.TotalCountExact
			// In auto mode, by definition all matching pages are fetched
			truncated = false

			if result.PageToken == nil || *result.PageToken == "" {
				break
			}
			params.PageToken = result.PageToken
		}

		// Auto mode: set counts based on what we fetched
		data.RowCount = types.Int64Value(int64(len(allAnomalies)))
		data.PageToken = types.StringNull()
		data.TotalCount = types.Int64Value(totalCount)
		data.TotalCountExact = types.BoolValue(totalCountExact)
		data.Truncated = types.BoolValue(truncated)
		// max_results was not set; preserve null
	}

	data.AnomalySummary = mapAnomalySummary(ctx, finalSummary, &resp.Diagnostics)

	// Map anomalies list
	if len(allAnomalies) > 0 {
		anomalyVals := make([]datasource_anomalies.AnomaliesValue, 0, len(allAnomalies))
		for _, anomaly := range allAnomalies {
			// Handle EndTime nullable.Nullable[int] -> Int64
			var endTimeVal types.Int64
			if endTime := nullableToPointer(anomaly.EndTime); endTime != nil {
				endTimeVal = types.Int64Value(int64(*endTime))
			} else {
				endTimeVal = types.Int64Null()
			}

			// Handle Status enum
			var statusVal types.String
			if status := nullableToPointer(anomaly.Status); status != nil {
				statusVal = types.StringValue(string(*status))
			} else {
				statusVal = types.StringNull()
			}

			// Handle DeactivationReason nullable enum
			var deactivationReasonVal types.String
			if deactivationReason := nullableToPointer(anomaly.DeactivationReason); deactivationReason != nil {
				deactivationReasonVal = types.StringValue(string(*deactivationReason))
			} else {
				deactivationReasonVal = types.StringNull()
			}

			// Map AcknowledgedAt (nullable.Nullable[time.Time])
			var acknowledgedAtVal types.String
			if acknowledgedAt := nullableToPointer(anomaly.AcknowledgedAt); acknowledgedAt != nil {
				acknowledgedAtVal = types.StringValue(acknowledgedAt.UTC().Format(time.RFC3339))
			} else {
				acknowledgedAtVal = types.StringNull()
			}

			// Map ResourceData nested list
			resourceDataList := mapAnomalyResourceData(ctx, anomaly.ResourceData, &resp.Diagnostics)

			// Map Top3SKUs nested list
			top3skusList := mapAnomalyTop3SKUs(ctx, anomaly.Top3SKUs, &resp.Diagnostics)

			// Map Notifications nested list
			notificationsList := mapAnomalyNotifications(ctx, anomaly.Notifications, &resp.Diagnostics)

			anomalyVal, diags := datasource_anomalies.NewAnomaliesValue(
				datasource_anomalies.AnomaliesValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"id":                  types.StringPointerValue(anomaly.Id),
					"acknowledged":        types.BoolPointerValue(anomaly.Acknowledged),
					"acknowledged_at":     acknowledgedAtVal,
					"acknowledged_by":     types.StringPointerValue(nullableToPointer(anomaly.AcknowledgedBy)),
					"actual_cost":         types.Float64PointerValue(nullableToPointer(anomaly.ActualCost)),
					"attribution":         types.StringValue(anomaly.Attribution),
					"billing_account":     types.StringValue(anomaly.BillingAccount),
					"cost_of_anomaly":     types.Float64Value(anomaly.CostOfAnomaly),
					"deactivation_reason": deactivationReasonVal,
					"end_time":            endTimeVal,
					"expected_max_cost":   types.Float64PointerValue(nullableToPointer(anomaly.ExpectedMaxCost)),
					"notifications":       notificationsList,
					"platform":            types.StringValue(anomaly.Platform),
					"scope":               types.StringValue(anomaly.Scope),
					"service_name":        types.StringValue(anomaly.ServiceName),
					"severity_level":      types.StringValue(anomaly.SeverityLevel),
					"start_time":          types.Int64Value(anomaly.StartTime),
					"status":              statusVal,
					"time_frame":          types.StringValue(anomaly.TimeFrame),
					"resource_data":       resourceDataList,
					"top3skus":            top3skusList,
				},
			)
			resp.Diagnostics.Append(diags...)
			anomalyVals = append(anomalyVals, anomalyVal)
		}

		anomalyList, diags := types.ListValueFrom(ctx, datasource_anomalies.AnomaliesValue{}.Type(ctx), anomalyVals)
		resp.Diagnostics.Append(diags...)
		data.Anomalies = anomalyList
	} else {
		emptyList, diags := types.ListValueFrom(ctx, datasource_anomalies.AnomaliesValue{}.Type(ctx), []datasource_anomalies.AnomaliesValue{})
		resp.Diagnostics.Append(diags...)
		data.Anomalies = emptyList
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// mapAnomalyResourceData maps API AnomalyResourceArray to Terraform list.
func mapAnomalyResourceData(ctx context.Context, resourceData *models.AnomalyResourceArray, diagnostics *diag.Diagnostics) types.List {
	if resourceData == nil || len(*resourceData) == 0 {
		emptyRD, d := types.ListValueFrom(ctx, datasource_anomalies.ResourceDataValue{}.Type(ctx), []datasource_anomalies.ResourceDataValue{})
		diagnostics.Append(d...)
		return emptyRD
	}

	vals := make([]datasource_anomalies.ResourceDataValue, 0, len(*resourceData))
	for _, rd := range *resourceData {
		// Map labels nested list for this resource
		labelsList := mapAnomalyResourceLabels(ctx, rd.Labels, diagnostics)

		rdVal, diags := datasource_anomalies.NewResourceDataValue(
			datasource_anomalies.ResourceDataValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"cost":            types.Float64PointerValue(rd.Cost),
				"labels":          labelsList,
				"operation":       types.StringPointerValue(rd.Operation),
				"resource_id":     types.StringPointerValue(rd.ResourceId),
				"sku_description": types.StringPointerValue(rd.SkuDescription),
			},
		)
		diagnostics.Append(diags...)
		vals = append(vals, rdVal)
	}

	list, diags := types.ListValueFrom(ctx, datasource_anomalies.ResourceDataValue{}.Type(ctx), vals)
	diagnostics.Append(diags...)
	return list
}

// mapAnomalyResourceLabels maps API AnomalyResourceLabel slice to Terraform list.
func mapAnomalyResourceLabels(ctx context.Context, labels *[]models.AnomalyResourceLabel, diagnostics *diag.Diagnostics) types.List {
	if labels == nil || len(*labels) == 0 {
		emptyLabels, d := types.ListValueFrom(ctx, datasource_anomalies.LabelsValue{}.Type(ctx), []datasource_anomalies.LabelsValue{})
		diagnostics.Append(d...)
		return emptyLabels
	}

	vals := make([]datasource_anomalies.LabelsValue, 0, len(*labels))
	for _, l := range *labels {
		labelVal, diags := datasource_anomalies.NewLabelsValue(
			datasource_anomalies.LabelsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"cost":  types.Float64PointerValue(l.Cost),
				"key":   types.StringPointerValue(l.Key),
				"value": types.StringPointerValue(l.Value),
			},
		)
		diagnostics.Append(diags...)
		vals = append(vals, labelVal)
	}

	list, diags := types.ListValueFrom(ctx, datasource_anomalies.LabelsValue{}.Type(ctx), vals)
	diagnostics.Append(diags...)
	return list
}

// mapAnomalyTop3SKUs maps API AnomalySKUArray to Terraform list.
func mapAnomalyTop3SKUs(ctx context.Context, skus models.AnomalySKUArray, diagnostics *diag.Diagnostics) types.List {
	if len(skus) == 0 {
		emptySkus, d := types.ListValueFrom(ctx, datasource_anomalies.Top3skusValue{}.Type(ctx), []datasource_anomalies.Top3skusValue{})
		diagnostics.Append(d...)
		return emptySkus
	}

	vals := make([]datasource_anomalies.Top3skusValue, 0, len(skus))
	for _, sku := range skus {
		skuVal, diags := datasource_anomalies.NewTop3skusValue(
			datasource_anomalies.Top3skusValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"cost": types.Float64PointerValue(sku.Cost),
				"name": types.StringPointerValue(sku.Name),
			},
		)
		diagnostics.Append(diags...)
		vals = append(vals, skuVal)
	}

	list, diags := types.ListValueFrom(ctx, datasource_anomalies.Top3skusValue{}.Type(ctx), vals)
	diagnostics.Append(diags...)
	return list
}

// mapAnomalyNotifications maps API NotificationEvent slice to Terraform list.
func mapAnomalyNotifications(ctx context.Context, notifications []models.NotificationEvent, diagnostics *diag.Diagnostics) types.List {
	if len(notifications) == 0 {
		emptyNotifications, d := types.ListValueFrom(ctx, datasource_anomalies.NotificationsValue{}.Type(ctx), []datasource_anomalies.NotificationsValue{})
		diagnostics.Append(d...)
		return emptyNotifications
	}

	vals := make([]datasource_anomalies.NotificationsValue, 0, len(notifications))
	for _, n := range notifications {
		notificationVal, diags := datasource_anomalies.NewNotificationsValue(
			datasource_anomalies.NotificationsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"channel":   types.StringValue(string(n.Channel)),
				"timestamp": types.StringValue(n.Timestamp.UTC().Format(time.RFC3339)),
			},
		)
		diagnostics.Append(diags...)
		vals = append(vals, notificationVal)
	}

	list, diags := types.ListValueFrom(ctx, datasource_anomalies.NotificationsValue{}.Type(ctx), vals)
	diagnostics.Append(diags...)
	return list
}

// mapAnomalySummary maps API AnomaliesResponseAnomalySummary to Terraform AnomalySummaryValue.
func mapAnomalySummary(ctx context.Context, summary models.AnomaliesResponseAnomalySummary, diagnostics *diag.Diagnostics) datasource_anomalies.AnomalySummaryValue {
	countBySeverityVal, diags := datasource_anomalies.NewCountBySeverityValue(
		datasource_anomalies.CountBySeverityValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"critical":    types.Int64Value(summary.CountBySeverity.Critical),
			"information": types.Int64Value(summary.CountBySeverity.Information),
			"warning":     types.Int64Value(summary.CountBySeverity.Warning),
		},
	)
	diagnostics.Append(diags...)

	summaryVal, diags := datasource_anomalies.NewAnomalySummaryValue(
		datasource_anomalies.AnomalySummaryValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"count_by_severity":     countBySeverityVal,
			"total_cost_of_anomaly": types.Float64Value(summary.TotalCostOfAnomaly),
		},
	)
	diagnostics.Append(diags...)

	return summaryVal
}
