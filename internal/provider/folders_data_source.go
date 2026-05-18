package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_folders"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*foldersDataSource)(nil)

func NewFoldersDataSource() datasource.DataSource {
	return &foldersDataSource{}
}

type foldersDataSource struct {
	client *models.ClientWithResponses
}

type foldersDataSourceModel struct {
	datasource_folders.FoldersModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *foldersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_folders"
}

func (d *foldersDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_folders.FoldersDataSourceSchema(ctx)

	s.MarkdownDescription = "List Cloud Analytics folders."
	s.Description = "List Cloud Analytics folders."

	s.Attributes["timeouts"] = timeouts.Attributes(ctx)

	resp.Schema = s
}

func (d *foldersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *foldersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data foldersDataSourceModel

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

	// If any pagination input is unknown, return unknown for all computed attributes
	if data.MaxResults.IsUnknown() || data.PageToken.IsUnknown() {
		data.Folders = types.ListUnknown(datasource_folders.FoldersValue{}.Type(ctx))
		data.RowCount = types.Int64Unknown()
		data.PageToken = types.StringUnknown()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	// Build query parameters
	params := &models.ListFoldersParams{}

	// Smart pagination: honor user-provided values, otherwise auto-paginate
	userControlsPagination := !data.MaxResults.IsNull()

	var allFolders []models.Folder

	if userControlsPagination {
		// Manual mode: single API call with user's params
		params.MaxResults = new(data.MaxResults.ValueString())
		if !data.PageToken.IsNull() {
			params.PageToken = new(data.PageToken.ValueString())
		}

		apiResp, err := d.client.ListFoldersWithResponse(ctx, params)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Reading Folders",
				fmt.Sprintf("Unable to read folders: %v", err),
			)
			return
		}

		if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
			resp.Diagnostics.AddError(
				"Error Reading Folders",
				fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
			)
			return
		}

		result := apiResp.JSON200
		if result.Folders != nil {
			allFolders = *result.Folders
		}

		// Preserve API's page_token for user to fetch next page
		data.PageToken = types.StringPointerValue(result.PageToken)
		if result.RowCount != nil {
			data.RowCount = types.Int64Value(int64(*result.RowCount))
		} else {
			data.RowCount = types.Int64Value(int64(len(allFolders)))
		}
	} else {
		// Auto mode: fetch all pages
		if !data.PageToken.IsNull() {
			params.PageToken = new(data.PageToken.ValueString())
		}
		for {
			apiResp, err := d.client.ListFoldersWithResponse(ctx, params)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error Reading Folders",
					fmt.Sprintf("Unable to read folders: %v", err),
				)
				return
			}

			if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
				resp.Diagnostics.AddError(
					"Error Reading Folders",
					fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
				)
				return
			}

			result := apiResp.JSON200
			if result.Folders != nil {
				allFolders = append(allFolders, *result.Folders...)
			}

			if result.PageToken == nil || *result.PageToken == "" {
				break
			}
			params.PageToken = result.PageToken
		}

		data.RowCount = types.Int64Value(int64(len(allFolders)))
		data.PageToken = types.StringNull()
	}

	// Map folders list
	if len(allFolders) > 0 {
		folderVals := make([]datasource_folders.FoldersValue, 0, len(allFolders))
		for _, folder := range allFolders {
			folderVal, diags := datasource_folders.NewFoldersValue(
				datasource_folders.FoldersValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"id":               types.StringPointerValue(folder.Id),
					"name":             types.StringPointerValue(folder.Name),
					"description":      types.StringPointerValue(folder.Description),
					"parent_folder_id": types.StringPointerValue(folder.ParentFolderId),
				},
			)
			resp.Diagnostics.Append(diags...)
			folderVals = append(folderVals, folderVal)
		}

		folderList, diags := types.ListValueFrom(ctx, datasource_folders.FoldersValue{}.Type(ctx), folderVals)
		resp.Diagnostics.Append(diags...)
		data.Folders = folderList
	} else {
		emptyList, diags := types.ListValueFrom(ctx, datasource_folders.FoldersValue{}.Type(ctx), []datasource_folders.FoldersValue{})
		resp.Diagnostics.Append(diags...)
		data.Folders = emptyList
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
