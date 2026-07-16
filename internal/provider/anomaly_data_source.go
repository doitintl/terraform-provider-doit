package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_anomaly"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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

type anomalyDataSourceModel struct {
	datasource_anomaly.AnomalyModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
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
	s := datasource_anomaly.AnomalyDataSourceSchema(ctx)

	s.Attributes["timeouts"] = timeouts.Attributes(ctx)

	resp.Schema = s
}

func (ds *anomalyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data anomalyDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	readTimeout, diags := data.Timeouts.Read(ctx, 2*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	// If ID is unknown (depends on a resource not yet created), set all computed
	// attributes to unknown so consumers don't treat null as a real value during planning.
	if data.Id.IsUnknown() {
		data.Acknowledged = types.BoolUnknown()
		data.AcknowledgedAt = types.StringUnknown()
		data.AcknowledgedBy = types.StringUnknown()
		data.Attribution = types.StringUnknown()
		data.BillingAccount = types.StringUnknown()
		data.CostOfAnomaly = types.Float64Unknown()
		data.EndTime = types.Int64Unknown()
		data.Notifications = types.ListUnknown(datasource_anomaly.NotificationsValue{}.Type(ctx))
		data.Platform = types.StringUnknown()
		data.ResourceData = types.ListUnknown(datasource_anomaly.ResourceDataValue{}.Type(ctx))
		data.Scope = types.StringUnknown()
		data.ServiceName = types.StringUnknown()
		data.SeverityLevel = types.StringUnknown()
		data.StartTime = types.Int64Unknown()
		data.Status = types.StringUnknown()
		data.TimeFrame = types.StringUnknown()
		data.Top3skus = types.ListUnknown(datasource_anomaly.Top3skusValue{}.Type(ctx))
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	id := data.Id.ValueString()
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
	data.Id = types.StringValue(id) // ID is not in the response, use the requested ID
	data.Acknowledged = types.BoolPointerValue(anomaly.Acknowledged)
	data.Attribution = types.StringValue(anomaly.Attribution)
	data.BillingAccount = types.StringValue(anomaly.BillingAccount)
	data.CostOfAnomaly = types.Float64Value(anomaly.CostOfAnomaly)
	data.Scope = types.StringValue(anomaly.Scope)
	data.ServiceName = types.StringValue(anomaly.ServiceName)
	data.StartTime = types.Int64Value(anomaly.StartTime)
	data.Platform = types.StringValue(anomaly.Platform)
	data.SeverityLevel = types.StringValue(anomaly.SeverityLevel)
	data.TimeFrame = types.StringValue(anomaly.TimeFrame)

	// AcknowledgedAt is nullable.Nullable[time.Time]
	if acknowledgedAt := nullableToPointer(anomaly.AcknowledgedAt); acknowledgedAt != nil {
		data.AcknowledgedAt = types.StringValue(acknowledgedAt.UTC().Format(time.RFC3339))
	} else {
		data.AcknowledgedAt = types.StringNull()
	}

	// AcknowledgedBy is nullable.Nullable[string]
	data.AcknowledgedBy = types.StringPointerValue(nullableToPointer(anomaly.AcknowledgedBy))

	// EndTime is nullable.Nullable[int] in the API
	if endTime := nullableToPointer(anomaly.EndTime); endTime != nil {
		data.EndTime = types.Int64Value(int64(*endTime))
	} else {
		data.EndTime = types.Int64Null()
	}

	// Status is nullable.Nullable[models.GetAnomaly200ResponseStatus]
	if status := nullableToPointer(anomaly.Status); status != nil {
		data.Status = types.StringValue(string(*status))
	} else {
		data.Status = types.StringNull()
	}

	// Map resource_data
	if anomaly.ResourceData != nil && len(*anomaly.ResourceData) > 0 {
		resourceDataVals := make([]datasource_anomaly.ResourceDataValue, 0, len(*anomaly.ResourceData))
		for _, rd := range *anomaly.ResourceData {
			// Map labels nested list for this resource
			labelsList := mapAnomalyResourceLabelsForAnomaly(ctx, rd.Labels, &resp.Diagnostics)

			rdVal, d := datasource_anomaly.NewResourceDataValue(
				datasource_anomaly.ResourceDataValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"cost":            types.Float64PointerValue(rd.Cost),
					"labels":          labelsList,
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
		data.ResourceData = resourceDataList
	} else {
		emptyList, d := types.ListValueFrom(ctx, datasource_anomaly.ResourceDataValue{}.Type(ctx), []datasource_anomaly.ResourceDataValue{})
		resp.Diagnostics.Append(d...)
		data.ResourceData = emptyList
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
		data.Top3skus = top3List
	} else {
		emptyList, d := types.ListValueFrom(ctx, datasource_anomaly.Top3skusValue{}.Type(ctx), []datasource_anomaly.Top3skusValue{})
		resp.Diagnostics.Append(d...)
		data.Top3skus = emptyList
	}

	// Map notifications
	data.Notifications = mapAnomalyNotificationsForAnomaly(ctx, anomaly.Notifications, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// mapAnomalyResourceLabelsForAnomaly maps API AnomalyResourceLabel slice to Terraform list
// for the singular anomaly data source.
func mapAnomalyResourceLabelsForAnomaly(ctx context.Context, labels *[]models.AnomalyResourceLabel, diagnostics *diag.Diagnostics) types.List {
	if labels == nil || len(*labels) == 0 {
		emptyLabels, d := types.ListValueFrom(ctx, datasource_anomaly.LabelsValue{}.Type(ctx), []datasource_anomaly.LabelsValue{})
		diagnostics.Append(d...)
		return emptyLabels
	}

	vals := make([]datasource_anomaly.LabelsValue, 0, len(*labels))
	for _, l := range *labels {
		labelVal, diags := datasource_anomaly.NewLabelsValue(
			datasource_anomaly.LabelsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"cost":  types.Float64PointerValue(l.Cost),
				"key":   types.StringPointerValue(l.Key),
				"value": types.StringPointerValue(l.Value),
			},
		)
		diagnostics.Append(diags...)
		vals = append(vals, labelVal)
	}

	list, diags := types.ListValueFrom(ctx, datasource_anomaly.LabelsValue{}.Type(ctx), vals)
	diagnostics.Append(diags...)
	return list
}

// mapAnomalyNotificationsForAnomaly maps API NotificationEvent slice to Terraform list
// for the singular anomaly data source.
func mapAnomalyNotificationsForAnomaly(ctx context.Context, notifications []models.NotificationEvent, diagnostics *diag.Diagnostics) types.List {
	if len(notifications) == 0 {
		emptyNotifications, d := types.ListValueFrom(ctx, datasource_anomaly.NotificationsValue{}.Type(ctx), []datasource_anomaly.NotificationsValue{})
		diagnostics.Append(d...)
		return emptyNotifications
	}

	vals := make([]datasource_anomaly.NotificationsValue, 0, len(notifications))
	for _, n := range notifications {
		notificationVal, diags := datasource_anomaly.NewNotificationsValue(
			datasource_anomaly.NotificationsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"channel":   types.StringValue(string(n.Channel)),
				"timestamp": types.StringValue(n.Timestamp.UTC().Format(time.RFC3339)),
			},
		)
		diagnostics.Append(diags...)
		vals = append(vals, notificationVal)
	}

	list, diags := types.ListValueFrom(ctx, datasource_anomaly.NotificationsValue{}.Type(ctx), vals)
	diagnostics.Append(diags...)
	return list
}
