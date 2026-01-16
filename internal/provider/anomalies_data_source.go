package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_anomalies"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

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
}

func (d *anomaliesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_anomalies"
}

func (d *anomaliesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_anomalies.AnomaliesDataSourceSchema(ctx)
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

	// Build query parameters
	params := &models.ListAnomaliesParams{}
	if !data.Filter.IsNull() && !data.Filter.IsUnknown() {
		filter := data.Filter.ValueString()
		params.Filter = &filter
	}
	if !data.MinCreationTime.IsNull() && !data.MinCreationTime.IsUnknown() {
		minCreationTime := data.MinCreationTime.ValueString()
		params.MinCreationTime = &minCreationTime
	}
	if !data.MaxCreationTime.IsNull() && !data.MaxCreationTime.IsUnknown() {
		maxCreationTime := data.MaxCreationTime.ValueString()
		params.MaxCreationTime = &maxCreationTime
	}

	// Auto-paginate: fetch all pages
	var allAnomalies []models.AnomalyItem

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

		// Accumulate anomalies
		if result.Anomalies != nil {
			allAnomalies = append(allAnomalies, *result.Anomalies...)
		}

		// Check for next page
		if result.PageToken == nil || *result.PageToken == "" {
			break
		}
		params.PageToken = result.PageToken
	}

	// Set row count to total accumulated
	data.RowCount = types.Int64Value(int64(len(allAnomalies)))

	// Page token is always null after auto-pagination
	data.PageToken = types.StringNull()

	// Ignore max_results input
	data.MaxResults = types.Int64Null()

	// Map anomalies list
	if len(allAnomalies) > 0 {
		anomalyVals := make([]datasource_anomalies.AnomaliesValue, 0, len(allAnomalies))
		for _, anomaly := range allAnomalies {
			// Handle EndTime *int -> Int64
			var endTimeVal types.Int64
			if anomaly.EndTime != nil {
				endTimeVal = types.Int64Value(int64(*anomaly.EndTime))
			} else {
				endTimeVal = types.Int64Null()
			}

			// Handle Status enum
			var statusVal types.String
			if anomaly.Status != nil {
				statusVal = types.StringValue(string(*anomaly.Status))
			} else {
				statusVal = types.StringNull()
			}

			// Map ResourceData nested list
			resourceDataList := mapAnomalyResourceData(ctx, anomaly.ResourceData, &resp.Diagnostics)

			// Map Top3SKUs nested list
			top3skusList := mapAnomalyTop3SKUs(ctx, anomaly.Top3SKUs, &resp.Diagnostics)

			anomalyVal, diags := datasource_anomalies.NewAnomaliesValue(
				datasource_anomalies.AnomaliesValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"id":              types.StringPointerValue(anomaly.Id),
					"acknowledged":    types.BoolPointerValue(anomaly.Acknowledged),
					"attribution":     types.StringValue(anomaly.Attribution),
					"billing_account": types.StringValue(anomaly.BillingAccount),
					"cost_of_anomaly": types.Float64Value(anomaly.CostOfAnomaly),
					"end_time":        endTimeVal,
					"platform":        types.StringValue(anomaly.Platform),
					"scope":           types.StringValue(anomaly.Scope),
					"service_name":    types.StringValue(anomaly.ServiceName),
					"severity_level":  types.StringValue(anomaly.SeverityLevel),
					"start_time":      types.Int64Value(anomaly.StartTime),
					"status":          statusVal,
					"time_frame":      types.StringValue(anomaly.TimeFrame),
					"resource_data":   resourceDataList,
					"top3skus":        top3skusList,
				},
			)
			resp.Diagnostics.Append(diags...)
			anomalyVals = append(anomalyVals, anomalyVal)
		}

		anomalyList, diags := types.ListValueFrom(ctx, datasource_anomalies.AnomaliesValue{}.Type(ctx), anomalyVals)
		resp.Diagnostics.Append(diags...)
		data.Anomalies = anomalyList
	} else {
		data.Anomalies = types.ListNull(datasource_anomalies.AnomaliesValue{}.Type(ctx))
	}

	// Set optional filter params to null if they were unknown
	if data.Filter.IsUnknown() {
		data.Filter = types.StringNull()
	}
	if data.MinCreationTime.IsUnknown() {
		data.MinCreationTime = types.StringNull()
	}
	if data.MaxCreationTime.IsUnknown() {
		data.MaxCreationTime = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// mapAnomalyResourceData maps API AnomalyResourceArray to Terraform list.
func mapAnomalyResourceData(ctx context.Context, resourceData *models.AnomalyResourceArray, diagnostics *diag.Diagnostics) types.List {
	if resourceData == nil || len(*resourceData) == 0 {
		return types.ListNull(datasource_anomalies.ResourceDataValue{}.Type(ctx))
	}

	vals := make([]datasource_anomalies.ResourceDataValue, 0, len(*resourceData))
	for _, rd := range *resourceData {
		rdVal, diags := datasource_anomalies.NewResourceDataValue(
			datasource_anomalies.ResourceDataValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"cost":            types.Float64PointerValue(rd.Cost),
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

// mapAnomalyTop3SKUs maps API AnomalySKUArray to Terraform list.
func mapAnomalyTop3SKUs(ctx context.Context, skus models.AnomalySKUArray, diagnostics *diag.Diagnostics) types.List {
	if len(skus) == 0 {
		return types.ListNull(datasource_anomalies.Top3skusValue{}.Type(ctx))
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
