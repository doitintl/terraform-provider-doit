package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_role"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*roleDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*roleDataSource)(nil)

func NewRoleDataSource() datasource.DataSource {
	return &roleDataSource{}
}

type roleDataSource struct {
	client *models.ClientWithResponses
}

type roleDataSourceModel struct {
	datasource_role.RoleModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *roleDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (d *roleDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*models.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *models.ClientWithResponses, got: %T", req.ProviderData),
		)
		return
	}
	d.client = client
}

func (d *roleDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_role.RoleDataSourceSchema(ctx)
	s.Attributes["timeouts"] = timeouts.Attributes(ctx)
	resp.Schema = s
}

func (d *roleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data roleDataSourceModel
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

	if data.Id.IsUnknown() {
		data.ChildTenantEligible = types.BoolUnknown()
		data.Customer = types.StringUnknown()
		data.Description = types.StringUnknown()
		data.Name = types.StringUnknown()
		data.Permissions = types.ListUnknown(types.StringType)
		data.Type = types.StringUnknown()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	apiResp, err := d.client.GetRoleWithResponse(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Role", fmt.Sprintf("Could not read role %s: %v", data.Id.ValueString(), err))
		return
	}
	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error Reading Role", fmt.Sprintf("Role %s: status %d, body: %s", data.Id.ValueString(), apiResp.StatusCode(), string(apiResp.Body)))
		return
	}

	var mapped roleResourceModel
	resp.Diagnostics.Append(mapRoleToModel(ctx, apiResp.JSON200, &mapped)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if mapped.Id.IsNull() || mapped.Id.ValueString() != data.Id.ValueString() {
		resp.Diagnostics.AddError("Invalid Role Response", fmt.Sprintf("Requested role %s but API returned ID %q", data.Id.ValueString(), mapped.Id.ValueString()))
		return
	}

	data.ChildTenantEligible = mapped.ChildTenantEligible
	data.Customer = mapped.Customer
	data.Description = mapped.Description
	data.Name = mapped.Name
	data.Permissions = mapped.Permissions
	data.Type = mapped.Type
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
