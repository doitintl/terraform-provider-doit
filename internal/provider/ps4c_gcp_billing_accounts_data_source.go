package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_ps4c_gcp_billing_accounts"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*ps4cGcpBillingAccountsDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*ps4cGcpBillingAccountsDataSource)(nil)

func NewPs4cGcpBillingAccountsDataSource() datasource.DataSource {
	return &ps4cGcpBillingAccountsDataSource{}
}

type ps4cGcpBillingAccountsDataSource struct {
	client *models.ClientWithResponses
}

type ps4cGcpBillingAccountsDataSourceModel struct {
	datasource_ps4c_gcp_billing_accounts.Ps4cGcpBillingAccountsModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *ps4cGcpBillingAccountsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ps4c_gcp_billing_accounts"
}

func (d *ps4cGcpBillingAccountsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ps4cGcpBillingAccountsDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_ps4c_gcp_billing_accounts.Ps4cGcpBillingAccountsDataSourceSchema(ctx)

	s.MarkdownDescription = "List GCP Billing Accounts tracked by PerfectScale for Commitments (PS4C)."
	s.Description = "List GCP Billing Accounts tracked by PerfectScale for Commitments (PS4C)."

	resp.Diagnostics.Append(setListPaginationDescriptions(&s, listRowCountTotal, "A `page_token` expires. An invalid or expired one fails the read; read again without `page_token` to start over.")...)
	s.Attributes["timeouts"] = timeouts.Attributes(ctx)

	resp.Schema = s
}

func (d *ps4cGcpBillingAccountsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ps4cGcpBillingAccountsDataSourceModel

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

	// The results are not yet known. page_token is one of them in every mode:
	// a read replaces it with the next page's token, or null. The inputs stay
	// as configured.
	if !req.Config.Raw.IsFullyKnown() {
		data.Items = types.ListUnknown(datasource_ps4c_gcp_billing_accounts.ItemsValue{}.Type(ctx))
		data.RowCount = types.Int64Unknown()
		data.PageToken = types.StringUnknown()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	params := &models.ListGcpBillingAccountsParams{}

	userControlsPagination := !data.MaxResults.IsNull()

	var allAccounts []models.GcpBillingAccount

	if userControlsPagination {
		params.MaxResults = new(int(data.MaxResults.ValueInt64()))
		if !data.PageToken.IsNull() {
			params.PageToken = new(data.PageToken.ValueString())
		}

		apiResp, err := d.client.ListGcpBillingAccountsWithResponse(ctx, params)
		if err != nil {
			resp.Diagnostics.AddError("Error Reading PS4C GCP Billing Accounts", fmt.Sprintf("Unable to read PS4C GCP billing accounts: %v", err))
			return
		}
		if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
			resp.Diagnostics.AddError("Error Reading PS4C GCP Billing Accounts", fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)))
			return
		}

		result := apiResp.JSON200
		allAccounts = result.Items

		data.PageToken = types.StringPointerValue(nullableToPointer(result.PageToken))
		data.RowCount = listRowCount(nullableToPointer(result.RowCount), len(allAccounts))
	} else {
		if !data.PageToken.IsNull() {
			params.PageToken = new(data.PageToken.ValueString())
		}
		// rowCount is the total across all pages, the same on every page, so it
		// is taken from the first page even when a page_token starts the read
		// after the first item.
		var apiRowCount *int64
		for first := true; ; first = false {
			apiResp, err := d.client.ListGcpBillingAccountsWithResponse(ctx, params)
			if err != nil {
				resp.Diagnostics.AddError("Error Reading PS4C GCP Billing Accounts", fmt.Sprintf("Unable to read PS4C GCP billing accounts: %v", err))
				return
			}
			if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
				resp.Diagnostics.AddError("Error Reading PS4C GCP Billing Accounts", fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)))
				return
			}

			result := apiResp.JSON200
			if first {
				apiRowCount = nullableToPointer(result.RowCount)
			}
			allAccounts = append(allAccounts, result.Items...)

			pageToken := nullableToPointer(result.PageToken)
			if pageToken == nil || *pageToken == "" {
				break
			}
			params.PageToken = pageToken
		}

		// Without a count from the API, the items fetched over all pages stand in for it.
		data.RowCount = listRowCount(apiRowCount, len(allAccounts))
		data.PageToken = types.StringNull()
		data.MaxResults = types.Int64Null()
	}

	itemsList, itemsDiags := mapGcpBillingAccountsToItemsList(ctx, allAccounts)
	resp.Diagnostics.Append(itemsDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Items = itemsList

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
