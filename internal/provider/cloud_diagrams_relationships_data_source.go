package provider

import (
	"context"
	"fmt"
	"time"

	ds "github.com/doitintl/terraform-provider-doit/internal/provider/datasource_cloud_diagrams_relationships"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Compile-time interface checks.
var _ datasource.DataSource = (*cloudDiagramsRelationshipsDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*cloudDiagramsRelationshipsDataSource)(nil)

// NewCloudDiagramsRelationshipsDataSource creates a new instance of the data source.
func NewCloudDiagramsRelationshipsDataSource() datasource.DataSource {
	return &cloudDiagramsRelationshipsDataSource{}
}

// cloudDiagramsRelationshipsDataSource implements datasource.DataSource.
type cloudDiagramsRelationshipsDataSource struct {
	client *models.ClientWithResponses
}

// cloudDiagramsRelationshipsDataSourceModel is the Terraform state model.
type cloudDiagramsRelationshipsDataSourceModel struct {
	ds.CloudDiagramsRelationshipsModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *cloudDiagramsRelationshipsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_diagrams_relationships"
}

func (d *cloudDiagramsRelationshipsDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := ds.CloudDiagramsRelationshipsDataSourceSchema(ctx)
	s.Attributes["timeouts"] = timeouts.Attributes(ctx)
	s.Description = "Retrieves resource relationships for a Cloud Diagram component by traversing the diagram graph."
	s.MarkdownDescription = "Retrieves resource relationships for a Cloud Diagram component by traversing the diagram graph."
	resp.Schema = s
}

func (d *cloudDiagramsRelationshipsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *cloudDiagramsRelationshipsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data cloudDiagramsRelationshipsDataSourceModel

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

	// Handle unknown inputs during planning.
	if !req.Config.Raw.IsFullyKnown() {
		data.Anchor = ds.NewAnchorValueUnknown()
		data.Relations = types.ListUnknown(ds.RelationsValue{}.Type(ctx))
		data.Direction = types.StringUnknown()
		data.Depth = types.StringUnknown()
		data.Kind = types.StringUnknown()
		data.Truncated = types.BoolUnknown()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	// Prepare query parameters.
	params := models.GetCloudDiagramResourceRelationshipsParams{}

	if !data.Direction.IsNull() {
		params.Direction = new(models.GetCloudDiagramResourceRelationshipsParamsDirection(data.Direction.ValueString()))
	}

	if !data.Depth.IsNull() {
		params.Depth = new(models.GetCloudDiagramResourceRelationshipsParamsDepth(data.Depth.ValueString()))
	}

	if !data.Kind.IsNull() {
		params.Kind = new(models.GetCloudDiagramResourceRelationshipsParamsKind(data.Kind.ValueString()))
	}

	// Call the API.
	apiResp, err := d.client.GetCloudDiagramResourceRelationshipsWithResponse(
		ctx,
		data.Id.ValueString(),
		data.Rid.ValueString(),
		&params,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Cloud Diagram Resource Relationships",
			fmt.Sprintf("Unable to get resource relationships: %v", err),
		)
		return
	}

	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Cloud Diagram Resource Relationships",
			fmt.Sprintf("Cloud Diagrams API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	result := apiResp.JSON200

	// Map the anchor object.
	anchorVal, anchorDiags := ds.NewAnchorValue(
		ds.AnchorValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"id":           types.StringValue(result.Anchor.Id),
			"name":         types.StringValue(result.Anchor.Name),
			"service_type": types.StringPointerValue(result.Anchor.ServiceType),
			"type":         types.StringValue(string(result.Anchor.Type)),
		},
	)
	resp.Diagnostics.Append(anchorDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Anchor = anchorVal

	// Map the relations list.
	var relationVals []ds.RelationsValue
	if len(result.Relations) > 0 {
		relationVals = make([]ds.RelationsValue, 0, len(result.Relations))
		for _, r := range result.Relations {
			relVal, relDiags := ds.NewRelationsValue(
				ds.RelationsValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"id":           types.StringValue(r.Id),
					"name":         types.StringValue(r.Name),
					"type":         types.StringValue(string(r.Type)),
					"relation":     types.StringValue(string(r.Relation)),
					"hops":         types.Int64Value(int64(r.Hops)),
					"service_type": types.StringPointerValue(r.ServiceType),
				},
			)
			resp.Diagnostics.Append(relDiags...)
			if resp.Diagnostics.HasError() {
				return
			}
			relationVals = append(relationVals, relVal)
		}
	} else {
		relationVals = []ds.RelationsValue{}
	}

	relationsList, listDiags := types.ListValueFrom(ctx, ds.RelationsValue{}.Type(ctx), relationVals)
	resp.Diagnostics.Append(listDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Relations = relationsList

	// Map remaining scalar fields.
	data.Direction = types.StringValue(string(result.Direction))
	data.Depth = types.StringValue(string(result.Depth))
	data.Kind = types.StringValue(string(result.Kind))
	data.Truncated = types.BoolValue(result.Truncated)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
