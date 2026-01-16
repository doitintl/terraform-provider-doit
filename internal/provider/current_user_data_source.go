package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_current_user"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*currentUserDataSource)(nil)

func NewCurrentUserDataSource() datasource.DataSource {
	return &currentUserDataSource{}
}

type currentUserDataSource struct {
	client *models.ClientWithResponses
}

type currentUserDataSourceModel struct {
	datasource_current_user.CurrentUserModel
}

func (d *currentUserDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_current_user"
}

func (d *currentUserDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_current_user.CurrentUserDataSourceSchema(ctx)
}

func (d *currentUserDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *currentUserDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data currentUserDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Call the API to validate the token and get current user info
	apiResp, err := d.client.ValidateWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Current User",
			fmt.Sprintf("Unable to read current user: %v", err),
		)
		return
	}

	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Current User",
			fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	// Map API response to data source model
	user := apiResp.JSON200
	data.Email = types.StringPointerValue(user.Email)
	data.Domain = types.StringPointerValue(user.Domain)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
