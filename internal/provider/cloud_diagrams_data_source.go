package provider

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_cloud_diagrams"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Compile-time interface checks.
var _ datasource.DataSource = (*cloudDiagramsDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*cloudDiagramsDataSource)(nil)

// NewCloudDiagramsDataSource creates a new instance of the data source.
func NewCloudDiagramsDataSource() datasource.DataSource {
	return &cloudDiagramsDataSource{}
}

// cloudDiagramsDataSource implements datasource.DataSource for cloud diagram lookups.
type cloudDiagramsDataSource struct {
	client *models.ClientWithResponses
}

// cloudDiagramsDataSourceModel is the Terraform state model.
// The CloudDiagrams field uses the generated tfsdk tag "cloud_diagrams" to match the generated schema.
type cloudDiagramsDataSourceModel struct {
	Id            types.String   `tfsdk:"id"`
	Resources     types.List     `tfsdk:"resources"`
	CloudDiagrams types.Set      `tfsdk:"cloud_diagrams"`
	Timeouts      timeouts.Value `tfsdk:"timeouts"`
}

func (d *cloudDiagramsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_diagrams"
}

func (d *cloudDiagramsDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	// Start from the generated schema (contains the "cloud_diagrams" output attribute).
	genSchema := datasource_cloud_diagrams.CloudDiagramsDataSourceSchema(ctx)

	// Add the input attributes that the generator cannot produce from the POST
	// request body, and the computed id.
	genSchema.Attributes["id"] = schema.StringAttribute{
		Computed:            true,
		Description:         "A deterministic hash of the resource IDs, used as the data source identifier.",
		MarkdownDescription: "A deterministic hash of the resource IDs, used as the data source identifier.",
	}
	genSchema.Attributes["resources"] = schema.ListAttribute{
		ElementType:         types.StringType,
		Required:            true,
		Description:         "Resource IDs to find diagrams for.",
		MarkdownDescription: "Resource IDs to find diagrams for.",
	}

	genSchema.Attributes["timeouts"] = timeouts.Attributes(ctx)

	resp.Schema = genSchema
}

func (d *cloudDiagramsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *cloudDiagramsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data cloudDiagramsDataSourceModel

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

	// If the config contains any unknown values (e.g., a list element like
	// [some_resource.id] during plan), we cannot make a complete API query.
	// Return all computed attributes as unknown.
	if !req.Config.Raw.IsFullyKnown() {
		data.Id = types.StringUnknown()
		data.CloudDiagrams = types.SetUnknown(datasource_cloud_diagrams.CloudDiagramsValue{}.Type(ctx))
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	// Extract resource IDs from the list.
	var resourceIDs []string
	resp.Diagnostics.Append(data.Resources.ElementsAs(ctx, &resourceIDs, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Call the API.
	apiResp, err := d.client.FindCloudDiagramsWithResponse(ctx, models.FindCloudDiagramsJSONRequestBody{
		Resources: resourceIDs,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Finding Cloud Diagrams",
			fmt.Sprintf("Unable to find cloud diagrams: %v", err),
		)
		return
	}

	if apiResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Finding Cloud Diagrams",
			fmt.Sprintf("Cloud Diagrams API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Finding Cloud Diagrams",
			fmt.Sprintf("Cloud Diagrams API returned status 200 but response body could not be parsed: %s", string(apiResp.Body)),
		)
		return
	}

	// Set a deterministic ID based on sorted resource IDs.
	sorted := make([]string, len(resourceIDs))
	copy(sorted, resourceIDs)
	sort.Strings(sorted)
	hash := sha256.Sum256([]byte(strings.Join(sorted, "\n")))
	data.Id = types.StringValue(fmt.Sprintf("%x", hash))

	// Map API response to Terraform state.
	diagramVals := make([]datasource_cloud_diagrams.CloudDiagramsValue, 0, len(*apiResp.JSON200))
	for _, item := range *apiResp.JSON200 {
		diagVal, newValDiags := datasource_cloud_diagrams.NewCloudDiagramsValue(
			datasource_cloud_diagrams.CloudDiagramsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"diagram_url": types.StringValue(item.DiagramUrl),
				"image_url":   types.StringValue(item.ImageUrl),
			},
		)
		resp.Diagnostics.Append(newValDiags...)
		if resp.Diagnostics.HasError() {
			return
		}
		diagramVals = append(diagramVals, diagVal)
	}

	diagramsSet, setDiags := types.SetValueFrom(ctx, datasource_cloud_diagrams.CloudDiagramsValue{}.Type(ctx), diagramVals)
	resp.Diagnostics.Append(setDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.CloudDiagrams = diagramsSet

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
