package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_account_team"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*accountTeamDataSource)(nil)

func NewAccountTeamDataSource() datasource.DataSource {
	return &accountTeamDataSource{}
}

type accountTeamDataSource struct {
	client *models.ClientWithResponses
}

type accountTeamDataSourceModel struct {
	datasource_account_team.AccountTeamModel
}

func (d *accountTeamDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_account_team"
}

func (d *accountTeamDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_account_team.AccountTeamDataSourceSchema(ctx)
}

func (d *accountTeamDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *accountTeamDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data accountTeamDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := d.client.ListAccountTeamWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Account Team",
			fmt.Sprintf("Unable to read account team: %v", err),
		)
		return
	}

	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Account Team",
			fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	if apiResp.JSON200.AccountManagers != nil && len(*apiResp.JSON200.AccountManagers) > 0 {
		amVals := make([]datasource_account_team.AccountManagersValue, 0, len(*apiResp.JSON200.AccountManagers))
		for _, am := range *apiResp.JSON200.AccountManagers {
			amVal, diags := datasource_account_team.NewAccountManagersValue(
				datasource_account_team.AccountManagersValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"id":            types.StringPointerValue(am.Id),
					"name":          types.StringPointerValue(am.Name),
					"email":         types.StringPointerValue(am.Email),
					"role":          types.StringPointerValue(am.Role),
					"calendly_link": types.StringPointerValue(am.CalendlyLink),
				},
			)
			resp.Diagnostics.Append(diags...)
			amVals = append(amVals, amVal)
		}

		amList, diags := types.ListValueFrom(ctx, datasource_account_team.AccountManagersValue{}.Type(ctx), amVals)
		resp.Diagnostics.Append(diags...)
		data.AccountManagers = amList
	} else {
		data.AccountManagers = types.ListNull(datasource_account_team.AccountManagersValue{}.Type(ctx))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
