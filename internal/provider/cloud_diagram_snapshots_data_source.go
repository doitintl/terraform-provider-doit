package provider

import (
	"context"
	"fmt"
	"time"

	ds "github.com/doitintl/terraform-provider-doit/internal/provider/datasource_cloud_diagram_snapshots"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Compile-time interface checks.
var _ datasource.DataSource = (*cloudDiagramSnapshotsDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*cloudDiagramSnapshotsDataSource)(nil)

// NewCloudDiagramSnapshotsDataSource creates a new instance of the data source.
func NewCloudDiagramSnapshotsDataSource() datasource.DataSource {
	return &cloudDiagramSnapshotsDataSource{}
}

// cloudDiagramSnapshotsDataSource implements datasource.DataSource for cloud diagram snapshot lookups.
type cloudDiagramSnapshotsDataSource struct {
	client *models.ClientWithResponses
}

// cloudDiagramSnapshotsDataSourceModel is the Terraform state model.
type cloudDiagramSnapshotsDataSourceModel struct {
	ds.CloudDiagramSnapshotsModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *cloudDiagramSnapshotsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_diagram_snapshots"
}

func (d *cloudDiagramSnapshotsDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := ds.CloudDiagramSnapshotsDataSourceSchema(ctx)
	s.Attributes["timeouts"] = timeouts.Attributes(ctx)
	resp.Schema = s
}

func (d *cloudDiagramSnapshotsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *cloudDiagramSnapshotsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data cloudDiagramSnapshotsDataSourceModel

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

	// If the config contains any unknown values (e.g., id is unknown during plan),
	// we cannot make a complete API query. Return all computed attributes as unknown.
	if data.Id.IsUnknown() {
		data.CloudDiagramSnapshots = types.SetUnknown(ds.CloudDiagramSnapshotsValue{}.Type(ctx))
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	// Prepare query parameters
	var params models.ListCloudDiagramLayerSnapshotsParams

	if !data.Offset.IsNull() && !data.Offset.IsUnknown() {
		offsetVal := int(data.Offset.ValueInt64())
		params.Offset = &offsetVal
	}

	if !data.Limit.IsNull() && !data.Limit.IsUnknown() {
		limitVal := int(data.Limit.ValueInt64())
		params.Limit = &limitVal
	}

	if !data.Sort.IsNull() && !data.Sort.IsUnknown() {
		sortVal := data.Sort.ValueString()
		params.Sort = &sortVal
	}

	// Call the API
	apiResp, err := d.client.ListCloudDiagramLayerSnapshotsWithResponse(ctx, data.Id.ValueString(), &params)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Cloud Diagram Snapshots",
			fmt.Sprintf("Unable to list cloud diagram layer snapshots: %v", err),
		)
		return
	}

	if apiResp.StatusCode() == 404 {
		resp.Diagnostics.AddError(
			"Cloud Diagram Layer Not Found",
			fmt.Sprintf("Cloud Diagram layer (statussheet) with ID %q not found.", data.Id.ValueString()),
		)
		return
	}

	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Cloud Diagram Snapshots",
			fmt.Sprintf("Cloud Diagrams API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	// Map API response to Terraform state
	var snapshotVals []ds.CloudDiagramSnapshotsValue
	if apiResp.JSON200 != nil && len(*apiResp.JSON200) > 0 {
		snapshotVals = make([]ds.CloudDiagramSnapshotsValue, 0, len(*apiResp.JSON200))
		for _, s := range *apiResp.JSON200 {
			createdAtStr := s.CreatedAt.UTC().Format(time.RFC3339)

			snapshotVal, valDiags := ds.NewCloudDiagramSnapshotsValue(
				ds.CloudDiagramSnapshotsValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"_id":        types.StringValue(s.UnderscoreId),
					"created_at": types.StringValue(createdAtStr),
					"name":       types.StringPointerValue(s.Name),
					"prev_state": types.StringPointerValue(s.PrevState),
				},
			)
			resp.Diagnostics.Append(valDiags...)
			if resp.Diagnostics.HasError() {
				return
			}
			snapshotVals = append(snapshotVals, snapshotVal)
		}
	} else {
		// Empty set representation instead of nil or null
		snapshotVals = []ds.CloudDiagramSnapshotsValue{}
	}

	snapshotsSet, setDiags := types.SetValueFrom(ctx, ds.CloudDiagramSnapshotsValue{}.Type(ctx), snapshotVals)
	resp.Diagnostics.Append(setDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.CloudDiagramSnapshots = snapshotsSet

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
