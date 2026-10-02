package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_service_account_tokens"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*serviceAccountTokensDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*serviceAccountTokensDataSource)(nil)

func NewServiceAccountTokensDataSource() datasource.DataSource {
	return &serviceAccountTokensDataSource{}
}

type serviceAccountTokensDataSource struct{ client *models.ClientWithResponses }

type serviceAccountTokensDataSourceModel struct {
	datasource_service_account_tokens.ServiceAccountTokensModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *serviceAccountTokensDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_account_tokens"
}

func (d *serviceAccountTokensDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *serviceAccountTokensDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_service_account_tokens.ServiceAccountTokensDataSourceSchema(ctx)
	s.Attributes["timeouts"] = timeouts.Attributes(ctx)
	resp.Schema = s
}

func (d *serviceAccountTokensDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data serviceAccountTokensDataSourceModel
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

	if data.ServiceAccountId.IsUnknown() {
		data.Items = types.ListUnknown(datasource_service_account_tokens.ItemsValue{}.Type(ctx))
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	serviceAccountID := data.ServiceAccountId.ValueString()

	// The endpoint is not paginated: it returns every token of the service account (at most 10).
	apiResp, err := d.client.ListServiceAccountTokensWithResponse(ctx, serviceAccountID)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Service Account Tokens", fmt.Sprintf("Could not list tokens of service account %s: %v", serviceAccountID, err))
		return
	}
	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error Reading Service Account Tokens", fmt.Sprintf("Service account %s: status %d, body: %s", serviceAccountID, apiResp.StatusCode(), string(apiResp.Body)))
		return
	}

	resp.Diagnostics.Append(mapServiceAccountTokensToModel(ctx, apiResp.JSON200.Items, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// mapServiceAccountTokensToModel maps the API tokens to data.Items. The result is
// an empty list, never null, when there are no tokens. data.ServiceAccountId is
// the requested service account and is used when an item omits its own.
func mapServiceAccountTokensToModel(ctx context.Context, items []models.ServiceAccountToken, data *serviceAccountTokensDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	values := make([]datasource_service_account_tokens.ItemsValue, 0, len(items))
	for _, item := range items {
		var mapped serviceAccountTokenResourceModel
		mapped.ServiceAccountId = data.ServiceAccountId
		mapServiceAccountTokenToModel(&item, &mapped)
		if mapped.Id.IsNull() {
			diags.AddError("Invalid Service Account Tokens Response", "A service account token in the list has no ID")
			return diags
		}
		value, valueDiags := datasource_service_account_tokens.NewItemsValue(
			datasource_service_account_tokens.ItemsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"id":                 mapped.Id,
				"service_account_id": mapped.ServiceAccountId,
				"customer_id":        mapped.CustomerId,
				"name":               mapped.Name,
				"state":              mapped.State,
				"create_time":        mapped.CreateTime,
				"expires_time":       mapped.ExpiresTime,
				"last_used_time":     mapped.LastUsedTime,
			},
		)
		diags.Append(valueDiags...)
		if diags.HasError() {
			return diags
		}
		values = append(values, value)
	}
	list, listDiags := types.ListValueFrom(ctx, datasource_service_account_tokens.ItemsValue{}.Type(ctx), values)
	diags.Append(listDiags...)
	data.Items = list
	return diags
}
