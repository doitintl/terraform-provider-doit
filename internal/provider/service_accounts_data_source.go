package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_service_accounts"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*serviceAccountsDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*serviceAccountsDataSource)(nil)

func NewServiceAccountsDataSource() datasource.DataSource { return &serviceAccountsDataSource{} }

type serviceAccountsDataSource struct{ client *models.ClientWithResponses }

type serviceAccountsDataSourceModel struct {
	datasource_service_accounts.ServiceAccountsModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *serviceAccountsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_accounts"
}

func (d *serviceAccountsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *serviceAccountsDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_service_accounts.ServiceAccountsDataSourceSchema(ctx)
	s.Attributes["timeouts"] = timeouts.Attributes(ctx)
	resp.Schema = s
}

func (d *serviceAccountsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data serviceAccountsDataSourceModel
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

	apiResp, err := d.client.ListServiceAccountsWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Service Accounts", fmt.Sprintf("Could not list service accounts: %v", err))
		return
	}
	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error Reading Service Accounts", fmt.Sprintf("status %d, body: %s", apiResp.StatusCode(), string(apiResp.Body)))
		return
	}

	resp.Diagnostics.Append(mapServiceAccountsToModel(ctx, apiResp.JSON200.Items, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func mapServiceAccountsToModel(ctx context.Context, items []models.ServiceAccount, data *serviceAccountsDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	values := make([]datasource_service_accounts.ItemsValue, 0, len(items))
	for _, item := range items {
		var mapped serviceAccountResourceModel
		diags.Append(mapServiceAccountToModel(ctx, &item, &mapped)...)
		if diags.HasError() {
			return diags
		}
		if mapped.Id.IsNull() {
			diags.AddError("Invalid Service Accounts Response", "A service account in the list has no ID")
			return diags
		}
		value, valueDiags := datasource_service_accounts.NewItemsValue(
			datasource_service_accounts.ItemsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"id":                 mapped.Id,
				"customer_id":        mapped.CustomerId,
				"name":               mapped.Name,
				"description":        mapped.Description,
				"permissions":        mapped.Permissions,
				"created_by":         mapped.CreatedBy,
				"created_by_email":   mapped.CreatedByEmail,
				"created_by_user_id": mapped.CreatedByUserId,
				"create_time":        mapped.CreateTime,
				"update_time":        mapped.UpdateTime,
				"etag":               mapped.Etag,
			},
		)
		diags.Append(valueDiags...)
		if diags.HasError() {
			return diags
		}
		values = append(values, value)
	}
	list, listDiags := types.ListValueFrom(ctx, datasource_service_accounts.ItemsValue{}.Type(ctx), values)
	diags.Append(listDiags...)
	data.Items = list
	return diags
}
