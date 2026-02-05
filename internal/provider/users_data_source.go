package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_users"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*usersDataSource)(nil)

func NewUsersDataSource() datasource.DataSource {
	return &usersDataSource{}
}

type usersDataSource struct {
	client *models.ClientWithResponses
}

type usersDataSourceModel struct {
	datasource_users.UsersModel
}

func (d *usersDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_users"
}

func (d *usersDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_users.UsersDataSourceSchema(ctx)
}

func (d *usersDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *usersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data usersDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := d.client.ListUsersWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Users",
			fmt.Sprintf("Unable to read users: %v", err),
		)
		return
	}

	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Users",
			fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	result := apiResp.JSON200

	// Map row count
	if result.RowCount != nil {
		data.RowCount = types.Int64Value(*result.RowCount)
	} else {
		data.RowCount = types.Int64Null()
	}

	// Map users list
	if result.Users != nil && len(*result.Users) > 0 {
		userVals := make([]datasource_users.UsersValue, 0, len(*result.Users))
		for _, user := range *result.Users {
			// Handle enum types that need conversion
			var jobFunctionVal types.String
			if user.JobFunction != nil {
				jobFunctionVal = types.StringValue(string(*user.JobFunction))
			} else {
				jobFunctionVal = types.StringNull()
			}

			var languageVal types.String
			if user.Language != nil {
				languageVal = types.StringValue(string(*user.Language))
			} else {
				languageVal = types.StringNull()
			}

			var statusVal types.String
			if user.Status != nil {
				statusVal = types.StringValue(string(*user.Status))
			} else {
				statusVal = types.StringNull()
			}

			userVal, diags := datasource_users.NewUsersValue(
				datasource_users.UsersValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"id":              types.StringPointerValue(user.Id),
					"email":           types.StringPointerValue(user.Email),
					"display_name":    types.StringPointerValue(user.DisplayName),
					"first_name":      types.StringPointerValue(user.FirstName),
					"last_name":       types.StringPointerValue(user.LastName),
					"job_function":    jobFunctionVal,
					"language":        languageVal,
					"phone":           types.StringPointerValue(user.Phone),
					"phone_extension": types.StringPointerValue(user.PhoneExtension),
					"organization_id": types.StringPointerValue(user.OrganizationId),
					"role_id":         types.StringPointerValue(user.RoleId),
					"status":          statusVal,
				},
			)
			resp.Diagnostics.Append(diags...)
			userVals = append(userVals, userVal)
		}

		usersList, diags := types.ListValueFrom(ctx, datasource_users.UsersValue{}.Type(ctx), userVals)
		resp.Diagnostics.Append(diags...)
		data.Users = usersList
	} else {
		data.Users = types.ListNull(datasource_users.UsersValue{}.Type(ctx))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
