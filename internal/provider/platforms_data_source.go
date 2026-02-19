package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_platforms"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*platformsDataSource)(nil)

func NewPlatformsDataSource() datasource.DataSource {
	return &platformsDataSource{}
}

type platformsDataSource struct {
	client *models.ClientWithResponses
}

type platformsDataSourceModel struct {
	datasource_platforms.PlatformsModel
}

func (d *platformsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_platforms"
}

func (d *platformsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_platforms.PlatformsDataSourceSchema(ctx)
}

func (d *platformsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *platformsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data platformsDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := d.client.ListPlatformsWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Platforms",
			fmt.Sprintf("Unable to read platforms: %v", err),
		)
		return
	}

	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Platforms",
			fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	if apiResp.JSON200.Platforms != nil && len(*apiResp.JSON200.Platforms) > 0 {
		platVals := make([]datasource_platforms.PlatformsValue, 0, len(*apiResp.JSON200.Platforms))
		for _, p := range *apiResp.JSON200.Platforms {
			platVal, diags := datasource_platforms.NewPlatformsValue(
				datasource_platforms.PlatformsValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"display_name": types.StringPointerValue(p.DisplayName),
					"id":           types.StringPointerValue(p.Id),
				},
			)
			resp.Diagnostics.Append(diags...)
			platVals = append(platVals, platVal)
		}

		platList, diags := types.ListValueFrom(ctx, datasource_platforms.PlatformsValue{}.Type(ctx), platVals)
		resp.Diagnostics.Append(diags...)
		data.Platforms = platList
	} else {
		data.Platforms = types.ListNull(datasource_platforms.PlatformsValue{}.Type(ctx))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
