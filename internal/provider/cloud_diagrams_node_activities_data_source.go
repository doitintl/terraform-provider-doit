package provider

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_cloud_diagrams_node_activities"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Compile-time interface checks.
var _ datasource.DataSource = (*cloudDiagramsNodeActivitiesDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*cloudDiagramsNodeActivitiesDataSource)(nil)

// NewCloudDiagramsNodeActivitiesDataSource creates a new instance of the data source.
func NewCloudDiagramsNodeActivitiesDataSource() datasource.DataSource {
	return &cloudDiagramsNodeActivitiesDataSource{}
}

type cloudDiagramsNodeActivitiesDataSource struct {
	client *models.ClientWithResponses
}

type cloudDiagramsNodeActivitiesDataSourceModel struct {
	Id                          types.String   `tfsdk:"id"`
	SsId                        types.String   `tfsdk:"ss_id"`
	NodeId                      types.String   `tfsdk:"node_id"`
	Limit                       types.Int64    `tfsdk:"limit"`
	Offset                      types.Int64    `tfsdk:"offset"`
	CloudDiagramsNodeActivities types.Set      `tfsdk:"cloud_diagrams_node_activities"`
	Timeouts                    timeouts.Value `tfsdk:"timeouts"`
}

func (d *cloudDiagramsNodeActivitiesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_diagrams_node_activities"
}

func (d *cloudDiagramsNodeActivitiesDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	genSchema := datasource_cloud_diagrams_node_activities.CloudDiagramsNodeActivitiesDataSourceSchema(ctx)

	// Add computed id.
	genSchema.Attributes["id"] = schema.StringAttribute{
		Computed:            true,
		Description:         "A deterministic hash of the query parameters, used as the data source identifier.",
		MarkdownDescription: "A deterministic hash of the query parameters, used as the data source identifier.",
	}

	genSchema.Attributes["timeouts"] = timeouts.Attributes(ctx)

	genSchema.Description = "Retrieves individual activity records for a specific Cloud Diagram component node."
	genSchema.MarkdownDescription = "Retrieves individual activity records for a specific Cloud Diagram component node."

	resp.Schema = genSchema
}

func (d *cloudDiagramsNodeActivitiesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *cloudDiagramsNodeActivitiesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data cloudDiagramsNodeActivitiesDataSourceModel

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

	// If the config contains any unknown values, return all computed attributes as unknown.
	if !req.Config.Raw.IsFullyKnown() {
		data.Id = types.StringUnknown()
		data.CloudDiagramsNodeActivities = types.SetUnknown(datasource_cloud_diagrams_node_activities.CloudDiagramsNodeActivitiesValue{}.Type(ctx))
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	// Build query parameters.
	params := &models.ListCloudDiagramNodeActivitiesParams{
		SsId:   data.SsId.ValueString(),
		NodeId: data.NodeId.ValueString(),
	}
	if !data.Limit.IsNull() && !data.Limit.IsUnknown() {
		params.Limit = new(int(data.Limit.ValueInt64()))
	}
	if !data.Offset.IsNull() && !data.Offset.IsUnknown() {
		params.Offset = new(int(data.Offset.ValueInt64()))
	}

	apiResp, err := d.client.ListCloudDiagramNodeActivitiesWithResponse(ctx, params)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Cloud Diagram Node Activities",
			fmt.Sprintf("Unable to read cloud diagram node activities: %v", err),
		)
		return
	}

	if apiResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Reading Cloud Diagram Node Activities",
			fmt.Sprintf("Cloud Diagram Node Activities API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Cloud Diagram Node Activities",
			fmt.Sprintf("Cloud Diagram Node Activities API returned status 200 but response body could not be parsed: %s", string(apiResp.Body)),
		)
		return
	}

	// Map API response to Terraform state.
	resp.Diagnostics.Append(mapNodeActivitiesToState(ctx, &data, *apiResp.JSON200)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Set a deterministic ID based on query parameters.
	// Only include optional fields when set, matching the pattern in
	// cloud_diagrams_search_data_source.go.
	idInput := fmt.Sprintf("cloud_diagrams_node_activities\nss_id:%s\nnode_id:%s",
		data.SsId.ValueString(), data.NodeId.ValueString())
	if !data.Limit.IsNull() && !data.Limit.IsUnknown() {
		idInput += fmt.Sprintf("\nlimit:%d", data.Limit.ValueInt64())
	}
	if !data.Offset.IsNull() && !data.Offset.IsUnknown() {
		idInput += fmt.Sprintf("\noffset:%d", data.Offset.ValueInt64())
	}
	hash := sha256.Sum256([]byte(idInput))
	data.Id = types.StringValue(fmt.Sprintf("%x", hash))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// mapNodeActivitiesToState maps the API response to the Terraform state model.
func mapNodeActivitiesToState(
	ctx context.Context,
	data *cloudDiagramsNodeActivitiesDataSourceModel,
	activities []models.CloudDiagramNodeActivity,
) diag.Diagnostics {
	var diags diag.Diagnostics

	activityVals := make([]datasource_cloud_diagrams_node_activities.CloudDiagramsNodeActivitiesValue, 0, len(activities))
	for _, a := range activities {
		val, valDiags := datasource_cloud_diagrams_node_activities.NewCloudDiagramsNodeActivitiesValue(
			datasource_cloud_diagrams_node_activities.CloudDiagramsNodeActivitiesValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"_id":         types.StringValue(a.UnderscoreId),
				"activity":    types.StringValue(string(a.Activity)),
				"metadata":    mapFreeformJSON(a.Metadata),
				"statussheet": types.StringValue(a.Statussheet),
				"timestamp":   types.StringValue(a.Timestamp.Format(time.RFC3339)),
				"user":        types.StringValue(a.User),
			},
		)
		diags.Append(valDiags...)
		if diags.HasError() {
			return diags
		}
		activityVals = append(activityVals, val)
	}

	activitySet, setDiags := types.SetValueFrom(ctx, datasource_cloud_diagrams_node_activities.CloudDiagramsNodeActivitiesValue{}.Type(ctx), activityVals)
	diags.Append(setDiags...)
	if diags.HasError() {
		return diags
	}
	data.CloudDiagramsNodeActivities = activitySet

	return diags
}
