package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_service_account"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*serviceAccountDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*serviceAccountDataSource)(nil)

func NewServiceAccountDataSource() datasource.DataSource { return &serviceAccountDataSource{} }

type serviceAccountDataSource struct{ client *models.ClientWithResponses }

type serviceAccountDataSourceModel struct {
	datasource_service_account.ServiceAccountModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *serviceAccountDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_account"
}

func (d *serviceAccountDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*models.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("Expected *models.ClientWithResponses, got: %T", req.ProviderData))
		return
	}
	d.client = client
}

func (d *serviceAccountDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_service_account.ServiceAccountDataSourceSchema(ctx)
	s.Attributes["timeouts"] = timeouts.Attributes(ctx)
	resp.Schema = s
}

func (d *serviceAccountDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data serviceAccountDataSourceModel
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
		data.CustomerId = types.StringUnknown()
		data.Name = types.StringUnknown()
		data.Description = types.StringUnknown()
		data.Permissions = types.ListUnknown(types.StringType)
		data.CreatedBy = types.StringUnknown()
		data.CreatedByEmail = types.StringUnknown()
		data.CreatedByUserId = types.StringUnknown()
		data.CreateTime = types.StringUnknown()
		data.UpdateTime = types.StringUnknown()
		data.Etag = types.StringUnknown()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	apiResp, err := d.client.GetServiceAccountWithResponse(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Service Account", fmt.Sprintf("Could not read service account %s: %v", data.Id.ValueString(), err))
		return
	}
	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error Reading Service Account", fmt.Sprintf("Service account %s: status %d, body: %s", data.Id.ValueString(), apiResp.StatusCode(), string(apiResp.Body)))
		return
	}

	var mapped serviceAccountResourceModel
	resp.Diagnostics.Append(mapServiceAccountToModel(ctx, apiResp.JSON200, &mapped)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if mapped.Id.IsNull() || mapped.Id.ValueString() != data.Id.ValueString() {
		resp.Diagnostics.AddError("Invalid Service Account Response", fmt.Sprintf("Requested service account %s but API returned ID %q", data.Id.ValueString(), mapped.Id.ValueString()))
		return
	}
	data.CustomerId = mapped.CustomerId
	data.Name = mapped.Name
	data.Description = mapped.Description
	data.Permissions = mapped.Permissions
	data.CreatedBy = mapped.CreatedBy
	data.CreatedByEmail = mapped.CreatedByEmail
	data.CreatedByUserId = mapped.CreatedByUserId
	data.CreateTime = mapped.CreateTime
	data.UpdateTime = mapped.UpdateTime
	data.Etag = mapped.Etag
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
