package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_cloud_diagram_snapshot"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Compile-time interface checks.
var _ datasource.DataSource = (*cloudDiagramSnapshotDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*cloudDiagramSnapshotDataSource)(nil)

// NewCloudDiagramSnapshotDataSource creates a new instance of the data source.
func NewCloudDiagramSnapshotDataSource() datasource.DataSource {
	return &cloudDiagramSnapshotDataSource{}
}

// cloudDiagramSnapshotDataSource implements datasource.DataSource.
type cloudDiagramSnapshotDataSource struct {
	client *models.ClientWithResponses
}

// cloudDiagramSnapshotDataSourceModel is the Terraform state model.
type cloudDiagramSnapshotDataSourceModel struct {
	Id         types.String   `tfsdk:"id"`
	SnapshotId types.String   `tfsdk:"snapshot_id"`
	Name       types.String   `tfsdk:"name"`
	CreatedAt  types.String   `tfsdk:"created_at"`
	PrevState  types.String   `tfsdk:"prev_state"`
	Timeouts   timeouts.Value `tfsdk:"timeouts"`
}

func (d *cloudDiagramSnapshotDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_diagram_snapshot"
}

func (d *cloudDiagramSnapshotDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	genSchema := datasource_cloud_diagram_snapshot.CloudDiagramSnapshotDataSourceSchema(ctx)

	// Override id to be required input (layer ID).
	genSchema.Attributes["id"] = schema.StringAttribute{
		Required:            true,
		Description:         "Layer ID.",
		MarkdownDescription: "Layer ID.",
	}

	// Override snapshot_id to be required input.
	genSchema.Attributes["snapshot_id"] = schema.StringAttribute{
		Required:            true,
		Description:         "Snapshot ID.",
		MarkdownDescription: "Snapshot ID.",
	}

	// Remove _id — it equals snapshot_id and its leading underscore is
	// invalid as a top-level tfsdk struct tag.
	delete(genSchema.Attributes, "_id")

	genSchema.Attributes["timeouts"] = timeouts.Attributes(ctx)

	genSchema.Description = "Retrieves a single snapshot of a Cloud Diagram layer."
	genSchema.MarkdownDescription = "Retrieves a single snapshot of a Cloud Diagram layer."

	resp.Schema = genSchema
}

func (d *cloudDiagramSnapshotDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *cloudDiagramSnapshotDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data cloudDiagramSnapshotDataSourceModel

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
		data.Name = types.StringUnknown()
		data.CreatedAt = types.StringUnknown()
		data.PrevState = types.StringUnknown()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	layerID := data.Id.ValueString()
	snapshotID := data.SnapshotId.ValueString()

	params := &models.GetCloudDiagramLayerSnapshotParams{
		SnapshotId: snapshotID,
	}

	apiResp, err := d.client.GetCloudDiagramLayerSnapshotWithResponse(ctx, layerID, params)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Cloud Diagram Snapshot",
			fmt.Sprintf("Unable to read snapshot: %v", err),
		)
		return
	}

	if apiResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Reading Cloud Diagram Snapshot",
			fmt.Sprintf("Cloud Diagram Snapshot API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Cloud Diagram Snapshot",
			fmt.Sprintf("Cloud Diagram Snapshot API returned status 200 but response body could not be parsed: %s", string(apiResp.Body)),
		)
		return
	}

	snapshot := apiResp.JSON200

	// Map API response to Terraform state.
	data.CreatedAt = types.StringValue(snapshot.CreatedAt.Format(time.RFC3339))
	data.Name = types.StringPointerValue(snapshot.Name)
	data.PrevState = types.StringPointerValue(snapshot.PrevState)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
