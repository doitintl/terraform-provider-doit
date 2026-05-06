package provider

import (
	"context"
	"fmt"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_users"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*usersDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*usersDataSource)(nil)

func NewUsersDataSource() datasource.DataSource {
	return &usersDataSource{}
}

type usersDataSource struct {
	client *models.ClientWithResponses
}

type usersDataSourceModel struct {
	datasource_users.UsersModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *usersDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_users"
}

func (d *usersDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_users.UsersDataSourceSchema(ctx)

	s.Attributes["timeouts"] = timeouts.Attributes(ctx)

	// Override email: it's an input-only filter, not a computed output.
	s.Attributes["email"] = schema.StringAttribute{
		Optional:            true,
		Description:         "Filter by exact email address. When provided, returns at most one user matching this email. The email is matched case-insensitively.",
		MarkdownDescription: "Filter by exact email address. When provided, returns at most one user matching this email. The email is matched case-insensitively.",
	}

	resp.Schema = s
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

	readTimeout, diags := data.Timeouts.Read(ctx, 2*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	// If email is unknown (depends on an unresolved resource), defer the API call
	// and return unknown outputs to prevent Terraform from using incorrect data.
	if data.Email.IsUnknown() {
		data.RowCount = types.Int64Unknown()
		data.Users = types.ListUnknown(datasource_users.UsersValue{}.Type(ctx))
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	// Build query params, passing email filter if provided
	var params *models.ListUsersParams
	if !data.Email.IsNull() && !data.Email.IsUnknown() {
		email := openapi_types.Email(data.Email.ValueString())
		params = &models.ListUsersParams{
			Email: &email,
		}
	}

	apiResp, err := d.client.ListUsersWithResponse(ctx, params)
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

	userCount := int64(0)
	if result.Users != nil {
		userCount = int64(len(*result.Users))
	}

	// Map row count
	if result.RowCount != nil {
		data.RowCount = types.Int64Value(*result.RowCount)
	} else {
		data.RowCount = types.Int64Value(userCount)
	}

	// Map users list
	if result.Users != nil && len(*result.Users) > 0 {
		userVals := make([]datasource_users.UsersValue, 0, len(*result.Users))
		for _, user := range *result.Users {
			// Handle enum types that need conversion
			var jobTitleVal types.String
			if user.JobTitle != nil {
				jobTitleVal = types.StringValue(string(*user.JobTitle))
			} else {
				jobTitleVal = types.StringNull()
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
					"job_title":       jobTitleVal,
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
		emptyList, diags := types.ListValueFrom(ctx, datasource_users.UsersValue{}.Type(ctx), []datasource_users.UsersValue{})
		resp.Diagnostics.Append(diags...)
		data.Users = emptyList
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
