package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_service_account_token"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*serviceAccountTokenDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*serviceAccountTokenDataSource)(nil)

func NewServiceAccountTokenDataSource() datasource.DataSource {
	return &serviceAccountTokenDataSource{}
}

type serviceAccountTokenDataSource struct{ client *models.ClientWithResponses }

type serviceAccountTokenDataSourceModel struct {
	datasource_service_account_token.ServiceAccountTokenModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *serviceAccountTokenDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_account_token"
}

func (d *serviceAccountTokenDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *serviceAccountTokenDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_service_account_token.ServiceAccountTokenDataSourceSchema(ctx)
	s.Attributes["timeouts"] = timeouts.Attributes(ctx)
	resp.Schema = s
}

func (d *serviceAccountTokenDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data serviceAccountTokenDataSourceModel
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

	if data.ServiceAccountId.IsUnknown() || data.Id.IsUnknown() {
		data.CustomerId = types.StringUnknown()
		data.Name = types.StringUnknown()
		data.State = types.StringUnknown()
		data.CreateTime = types.StringUnknown()
		data.ExpiresTime = types.StringUnknown()
		data.LastUsedTime = types.StringUnknown()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	serviceAccountID := data.ServiceAccountId.ValueString()
	id := data.Id.ValueString()

	apiResp, err := d.client.GetServiceAccountTokenWithResponse(ctx, serviceAccountID, id)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Service Account Token", fmt.Sprintf("Could not read service account token %s of service account %s: %v", id, serviceAccountID, err))
		return
	}
	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error Reading Service Account Token", fmt.Sprintf("Service account token %s of service account %s: status %d, body: %s", id, serviceAccountID, apiResp.StatusCode(), string(apiResp.Body)))
		return
	}

	// The mapper keeps the existing service_account_id when the response omits it,
	// so seed it with the requested one.
	var mapped serviceAccountTokenResourceModel
	mapped.ServiceAccountId = data.ServiceAccountId
	mapServiceAccountTokenToModel(apiResp.JSON200, &mapped)
	if mapped.Id.IsNull() || mapped.Id.ValueString() != id {
		resp.Diagnostics.AddError("Invalid Service Account Token Response", fmt.Sprintf("Requested service account token %s but API returned ID %q", id, mapped.Id.ValueString()))
		return
	}

	data.CustomerId = mapped.CustomerId
	data.Name = mapped.Name
	data.State = mapped.State
	data.CreateTime = mapped.CreateTime
	data.ExpiresTime = mapped.ExpiresTime
	data.LastUsedTime = mapped.LastUsedTime
	data.ServiceAccountId = mapped.ServiceAccountId
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
