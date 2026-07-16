package provider

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_cloud_diagrams_cost_snapshot"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

var _ datasource.DataSource = (*cloudDiagramsCostSnapshotDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*cloudDiagramsCostSnapshotDataSource)(nil)

func NewCloudDiagramsCostSnapshotDataSource() datasource.DataSource {
	return &cloudDiagramsCostSnapshotDataSource{}
}

type cloudDiagramsCostSnapshotDataSource struct {
	client *models.ClientWithResponses
}

type cloudDiagramsCostSnapshotDataSourceModel struct {
	datasource_cloud_diagrams_cost_snapshot.CloudDiagramsCostSnapshotModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *cloudDiagramsCostSnapshotDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_diagrams_cost_snapshot"
}

func (d *cloudDiagramsCostSnapshotDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	genSchema := datasource_cloud_diagrams_cost_snapshot.CloudDiagramsCostSnapshotDataSourceSchema(ctx)

	genSchema.Attributes["timeouts"] = timeouts.Attributes(ctx)

	genSchema.Description = "Returns a bounded cost snapshot for a Cloud Diagram layer over a date window."
	genSchema.MarkdownDescription = "Returns a bounded cost snapshot for a Cloud Diagram layer over a date window."

	resp.Schema = genSchema
}

func (d *cloudDiagramsCostSnapshotDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *cloudDiagramsCostSnapshotDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data cloudDiagramsCostSnapshotDataSourceModel

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

	if !req.Config.Raw.IsFullyKnown() {
		data.DiagramId = types.StringUnknown()
		data.Currency = types.StringUnknown()
		data.Total = types.NumberUnknown()
		data.TrendingPct = types.NumberUnknown()
		data.Interval = types.StringUnknown()
		data.TimeRange = datasource_cloud_diagrams_cost_snapshot.NewTimeRangeValueUnknown()
		data.TopResources = types.ListUnknown(datasource_cloud_diagrams_cost_snapshot.TopResourcesValue{}.Type(ctx))
		data.ByService = types.ListUnknown(datasource_cloud_diagrams_cost_snapshot.ByServiceValue{}.Type(ctx))
		data.Trend = types.ListUnknown(datasource_cloud_diagrams_cost_snapshot.TrendValue{}.Type(ctx))
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	layerID := data.Id.ValueString()

	startDate, err := time.Parse("2006-01-02", data.StartDate.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Start Date",
			fmt.Sprintf("Unable to parse start_date: %v", err),
		)
		return
	}

	endDate, err := time.Parse("2006-01-02", data.EndDate.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid End Date",
			fmt.Sprintf("Unable to parse end_date: %v", err),
		)
		return
	}

	params := &models.GetCloudDiagramCostSnapshotParams{
		StartDate: openapi_types.Date{Time: startDate},
		EndDate:   openapi_types.Date{Time: endDate},
	}

	if !data.Interval.IsNull() && !data.Interval.IsUnknown() {
		params.Interval = new(models.GetCloudDiagramCostSnapshotParamsInterval(data.Interval.ValueString()))
	}

	apiResp, err := d.client.GetCloudDiagramCostSnapshotWithResponse(ctx, layerID, params)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Cloud Diagram Cost Snapshot",
			fmt.Sprintf("Unable to read cost snapshot: %v", err),
		)
		return
	}

	if apiResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Reading Cloud Diagram Cost Snapshot",
			fmt.Sprintf("Cloud Diagram Cost Snapshot API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Cloud Diagram Cost Snapshot",
			fmt.Sprintf("Cloud Diagram Cost Snapshot API returned status 200 but response body could not be parsed: %s", string(apiResp.Body)),
		)
		return
	}

	snapshot := apiResp.JSON200

	data.DiagramId = types.StringValue(snapshot.DiagramId)
	data.Currency = types.StringValue(snapshot.Currency)
	data.Total = types.NumberValue(big.NewFloat(float64(snapshot.Total)))

	if trendingPct := nullableToPointer(snapshot.TrendingPct); trendingPct != nil {
		data.TrendingPct = types.NumberValue(big.NewFloat(float64(*trendingPct)))
	} else {
		data.TrendingPct = types.NumberNull()
	}

	data.TimeRange, diags = datasource_cloud_diagrams_cost_snapshot.NewTimeRangeValue(
		datasource_cloud_diagrams_cost_snapshot.TimeRangeValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"start_date": types.StringValue(snapshot.TimeRange.StartDate.Format("2006-01-02")),
			"end_date":   types.StringValue(snapshot.TimeRange.EndDate.Format("2006-01-02")),
			"interval":   types.StringValue(string(snapshot.TimeRange.Interval)),
		},
	)
	resp.Diagnostics.Append(diags...)

	data.Interval = types.StringValue(string(snapshot.TimeRange.Interval))

	topResources := make([]datasource_cloud_diagrams_cost_snapshot.TopResourcesValue, len(snapshot.TopResources))
	for i, r := range snapshot.TopResources {
		topResources[i], diags = datasource_cloud_diagrams_cost_snapshot.NewTopResourcesValue(
			datasource_cloud_diagrams_cost_snapshot.TopResourcesValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"id":     types.StringValue(r.Id),
				"name":   types.StringValue(r.Name),
				"type":   types.StringValue(r.Type),
				"amount": types.NumberValue(big.NewFloat(float64(r.Amount))),
			},
		)
		resp.Diagnostics.Append(diags...)
	}
	data.TopResources, diags = types.ListValueFrom(ctx, datasource_cloud_diagrams_cost_snapshot.TopResourcesValue{}.Type(ctx), topResources)
	resp.Diagnostics.Append(diags...)

	byService := make([]datasource_cloud_diagrams_cost_snapshot.ByServiceValue, len(snapshot.ByService))
	for i, s := range snapshot.ByService {
		byService[i], diags = datasource_cloud_diagrams_cost_snapshot.NewByServiceValue(
			datasource_cloud_diagrams_cost_snapshot.ByServiceValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"service": types.StringValue(s.Service),
				"amount":  types.NumberValue(big.NewFloat(float64(s.Amount))),
			},
		)
		resp.Diagnostics.Append(diags...)
	}
	data.ByService, diags = types.ListValueFrom(ctx, datasource_cloud_diagrams_cost_snapshot.ByServiceValue{}.Type(ctx), byService)
	resp.Diagnostics.Append(diags...)

	trend := make([]datasource_cloud_diagrams_cost_snapshot.TrendValue, len(snapshot.Trend))
	for i, t := range snapshot.Trend {
		trend[i], diags = datasource_cloud_diagrams_cost_snapshot.NewTrendValue(
			datasource_cloud_diagrams_cost_snapshot.TrendValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"bucket_start": types.StringValue(t.BucketStart),
				"amount":       types.NumberValue(big.NewFloat(float64(t.Amount))),
			},
		)
		resp.Diagnostics.Append(diags...)
	}
	data.Trend, diags = types.ListValueFrom(ctx, datasource_cloud_diagrams_cost_snapshot.TrendValue{}.Type(ctx), trend)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
