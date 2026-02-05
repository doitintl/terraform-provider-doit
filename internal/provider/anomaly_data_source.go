package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_anomaly"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*anomalyDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*anomalyDataSource)(nil)

func NewAnomalyDataSource() datasource.DataSource {
	return &anomalyDataSource{}
}

type anomalyDataSource struct {
	client *models.ClientWithResponses
}

func (ds *anomalyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_anomaly"
}

func (ds *anomalyDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*models.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *models.ClientWithResponses, got: %T", req.ProviderData),
		)
		return
	}
	ds.client = client
}

func (ds *anomalyDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_anomaly.AnomalyDataSourceSchema(ctx)
}

func (ds *anomalyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state datasource_anomaly.AnomalyModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.Id.ValueString()
	anomalyResp, err := ds.client.GetAnomalyWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Error reading anomaly", err.Error())
		return
	}
	if anomalyResp.StatusCode() == 404 {
		resp.Diagnostics.AddError("Anomaly not found", fmt.Sprintf("Anomaly with ID %s not found", id))
		return
	}
	if anomalyResp.StatusCode() != 200 || anomalyResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error reading anomaly",
			fmt.Sprintf("Unexpected status: %d, body: %s", anomalyResp.StatusCode(), string(anomalyResp.Body)),
		)
		return
	}

	anomaly := anomalyResp.JSON200

	// Map fields - the API returns an anonymous struct
	state.Id = types.StringValue(id) // ID is not in the response, use the requested ID
	state.Acknowledged = types.BoolPointerValue(anomaly.Acknowledged)
	state.Attribution = types.StringValue(anomaly.Attribution)
	state.BillingAccount = types.StringValue(anomaly.BillingAccount)
	state.CostOfAnomaly = types.Float64Value(anomaly.CostOfAnomaly)
	state.Scope = types.StringValue(anomaly.Scope)
	state.ServiceName = types.StringValue(anomaly.ServiceName)
	state.StartTime = types.Int64Value(anomaly.StartTime)
	state.Platform = types.StringValue(anomaly.Platform)
	state.SeverityLevel = types.StringValue(anomaly.SeverityLevel)
	state.TimeFrame = types.StringValue(anomaly.TimeFrame)

	// EndTime is *int in the API
	if anomaly.EndTime != nil {
		state.EndTime = types.Int64Value(int64(*anomaly.EndTime))
	} else {
		state.EndTime = types.Int64Null()
	}

	// Status is a pointer
	if anomaly.Status != nil {
		state.Status = types.StringValue(string(*anomaly.Status))
	} else {
		state.Status = types.StringNull()
	}

	// Map resource_data
	if anomaly.ResourceData != nil && len(*anomaly.ResourceData) > 0 {
		resourceDataVals := make([]datasource_anomaly.ResourceDataValue, 0, len(*anomaly.ResourceData))
		for _, rd := range *anomaly.ResourceData {
			rdVal, d := datasource_anomaly.NewResourceDataValue(
				datasource_anomaly.ResourceDataValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"cost":            types.Float64PointerValue(rd.Cost),
					"operation":       types.StringPointerValue(rd.Operation),
					"resource_id":     types.StringPointerValue(rd.ResourceId),
					"sku_description": types.StringPointerValue(rd.SkuDescription),
				},
			)
			resp.Diagnostics.Append(d...)
			resourceDataVals = append(resourceDataVals, rdVal)
		}
		resourceDataList, d := types.ListValueFrom(ctx, datasource_anomaly.ResourceDataValue{}.Type(ctx), resourceDataVals)
		resp.Diagnostics.Append(d...)
		state.ResourceData = resourceDataList
	} else {
		state.ResourceData = types.ListNull(datasource_anomaly.ResourceDataValue{}.Type(ctx))
	}

	// Map top3skus
	if len(anomaly.Top3SKUs) > 0 {
		top3Vals := make([]datasource_anomaly.Top3skusValue, 0, len(anomaly.Top3SKUs))
		for _, sku := range anomaly.Top3SKUs {
			skuVal, d := datasource_anomaly.NewTop3skusValue(
				datasource_anomaly.Top3skusValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"cost": types.Float64PointerValue(sku.Cost),
					"name": types.StringPointerValue(sku.Name),
				},
			)
			resp.Diagnostics.Append(d...)
			top3Vals = append(top3Vals, skuVal)
		}
		top3List, d := types.ListValueFrom(ctx, datasource_anomaly.Top3skusValue{}.Type(ctx), top3Vals)
		resp.Diagnostics.Append(d...)
		state.Top3skus = top3List
	} else {
		state.Top3skus = types.ListNull(datasource_anomaly.Top3skusValue{}.Type(ctx))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
