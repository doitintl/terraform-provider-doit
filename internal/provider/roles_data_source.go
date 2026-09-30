package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_roles"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*rolesDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*rolesDataSource)(nil)

func NewRolesDataSource() datasource.DataSource {
	return &rolesDataSource{}
}

type rolesDataSource struct {
	client *models.ClientWithResponses
}

type rolesDataSourceModel struct {
	datasource_roles.RolesModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *rolesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_roles"
}

func (d *rolesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_roles.RolesDataSourceSchema(ctx)

	s.Attributes["timeouts"] = timeouts.Attributes(ctx)

	resp.Schema = s
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

	readTimeout, diags := data.Timeouts.Read(ctx, DefaultReadTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

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

	result := apiResp.JSON200

	roleCount := int64(0)
	if result.Roles != nil {
		roleCount = int64(len(*result.Roles))
	}

	// Map row count with deterministic fallback
	if result.RowCount != nil {
		data.RowCount = types.Int64Value(int64(*result.RowCount))
	} else {
		data.RowCount = types.Int64Value(roleCount)
	}

	// Map API response to data source model
	if result.Roles != nil && len(*result.Roles) > 0 {
		roleVals := make([]datasource_roles.RolesValue, 0, len(*result.Roles))
		for _, role := range *result.Roles {
			// Map permissions list
			permissions := role.Permissions
			if permissions == nil {
				permissions = []string{}
			}
			permissionsList, diags := types.ListValueFrom(ctx, types.StringType, permissions)
			resp.Diagnostics.Append(diags...)

			roleVal, diags := datasource_roles.NewRolesValue(
				datasource_roles.RolesValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"child_tenant_eligible": types.BoolValue(role.ChildTenantEligible),
					"customer":              types.StringValue(role.Customer),
					"description":           types.StringValue(role.Description),
					"id":                    types.StringValue(role.Id),
					"name":                  types.StringValue(role.Name),
					"permissions":           permissionsList,
					"type":                  types.StringValue(string(role.Type)),
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
