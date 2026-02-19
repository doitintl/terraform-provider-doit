package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_roles"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*rolesDataSource)(nil)

func NewRolesDataSource() datasource.DataSource {
	return &rolesDataSource{}
}

type rolesDataSource struct {
	client *models.ClientWithResponses
}

type rolesDataSourceModel struct {
	datasource_roles.RolesModel
}

func (d *rolesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_roles"
}

func (d *rolesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_roles.RolesDataSourceSchema(ctx)
}

func (d *rolesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *rolesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data rolesDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Call the API to list all roles
	apiResp, err := d.client.ListRolesWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Roles",
			fmt.Sprintf("Unable to read roles: %v", err),
		)
		return
	}

	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Roles",
			fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	// Map API response to data source model
	if apiResp.JSON200.Roles != nil && len(*apiResp.JSON200.Roles) > 0 {
		roleVals := make([]datasource_roles.RolesValue, 0, len(*apiResp.JSON200.Roles))
		for _, role := range *apiResp.JSON200.Roles {
			// Map permissions list
			var permissionsList types.List
			if role.Permissions != nil && len(*role.Permissions) > 0 {
				permVals := make([]attr.Value, 0, len(*role.Permissions))
				for _, p := range *role.Permissions {
					permVals = append(permVals, types.StringValue(p))
				}
				permsListVal, diags := types.ListValue(types.StringType, permVals)
				resp.Diagnostics.Append(diags...)
				permissionsList = permsListVal
			} else {
				permsList, d := types.ListValueFrom(ctx, types.StringType, []string{})
				resp.Diagnostics.Append(d...)
				permissionsList = permsList
			}

			roleVal, diags := datasource_roles.NewRolesValue(
				datasource_roles.RolesValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"id":          types.StringPointerValue(role.Id),
					"name":        types.StringPointerValue(role.Name),
					"type":        types.StringPointerValue(role.Type),
					"customer":    types.StringPointerValue(role.Customer),
					"permissions": permissionsList,
				},
			)
			resp.Diagnostics.Append(diags...)
			roleVals = append(roleVals, roleVal)
		}

		rolesList, diags := types.ListValueFrom(ctx, datasource_roles.RolesValue{}.Type(ctx), roleVals)
		resp.Diagnostics.Append(diags...)
		data.Roles = rolesList
	} else {
		emptyList, diags := types.ListValueFrom(ctx, datasource_roles.RolesValue{}.Type(ctx), []datasource_roles.RolesValue{})
		resp.Diagnostics.Append(diags...)
		data.Roles = emptyList
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
