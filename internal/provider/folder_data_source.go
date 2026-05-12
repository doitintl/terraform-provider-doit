package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_folder"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*folderDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*folderDataSource)(nil)

func NewFolderDataSource() datasource.DataSource {
	return &folderDataSource{}
}

type (
	folderDataSource struct {
		client *models.ClientWithResponses
	}
	folderDataSourceModel struct {
		datasource_folder.FolderModel
		Timeouts timeouts.Value `tfsdk:"timeouts"`
	}
)

func (d *folderDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_folder"
}

func (d *folderDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_folder.FolderDataSourceSchema(ctx)

	s.MarkdownDescription = "Retrieve a Cloud Analytics folder by its ID."
	s.Description = "Retrieve a Cloud Analytics folder by its ID."

	s.Attributes["timeouts"] = timeouts.Attributes(ctx)

	resp.Schema = s
}

func (d *folderDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*models.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *models.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *folderDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data folderDataSourceModel

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
		data.Name = types.StringUnknown()
		data.Description = types.StringUnknown()
		data.ParentFolderId = types.StringUnknown()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	// Call API to get folder
	folderResp, err := d.client.GetFolderWithResponse(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Folder",
			"Could not read folder ID "+data.Id.ValueString()+": "+err.Error(),
		)
		return
	}

	if folderResp.StatusCode() != 200 || folderResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Folder",
			fmt.Sprintf("Could not read folder ID %s, status: %d, body: %s", data.Id.ValueString(), folderResp.StatusCode(), string(folderResp.Body)),
		)
		return
	}

	folder := folderResp.JSON200

	// Map API response to model
	data.Id = types.StringPointerValue(folder.Id)
	data.Name = types.StringPointerValue(folder.Name)
	data.Description = types.StringPointerValue(folder.Description)
	data.ParentFolderId = types.StringPointerValue(folder.ParentFolderId)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
