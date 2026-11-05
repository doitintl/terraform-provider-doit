package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_customer_subscriptions"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*customerSubscriptionsDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*customerSubscriptionsDataSource)(nil)

func NewCustomerSubscriptionsDataSource() datasource.DataSource {
	return &customerSubscriptionsDataSource{}
}

type customerSubscriptionsDataSource struct{ client *models.ClientWithResponses }

type customerSubscriptionsDataSourceModel struct {
	datasource_customer_subscriptions.CustomerSubscriptionsModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *customerSubscriptionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_customer_subscriptions"
}

func (d *customerSubscriptionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *customerSubscriptionsDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_customer_subscriptions.CustomerSubscriptionsDataSourceSchema(ctx)

	s.Description = "Organization-wide subscriptions visible to the caller. Alerts and budgets are listed too; their entries have type `alert` or `budget` and no schedule. Without `max_results` every page is fetched; with it, a single page is returned together with the `page_token` of the next one."
	s.MarkdownDescription = s.Description

	s.Attributes["timeouts"] = timeouts.Attributes(ctx)
	resp.Schema = s
}

func (d *customerSubscriptionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data customerSubscriptionsDataSourceModel
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

	if !req.Config.Raw.IsFullyKnown() {
		data.Items = types.ListUnknown(datasource_customer_subscriptions.ItemsValue{}.Type(ctx))
		data.RowCount = types.Int64Unknown()
		// max_results and page_token are inputs: a known one stays as configured.
		if data.MaxResults.IsNull() {
			data.MaxResults = types.Int64Unknown()
		}
		if data.PageToken.IsNull() {
			data.PageToken = types.StringUnknown()
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	params := &models.ListCustomerSubscriptionsParams{}
	if !data.PageToken.IsNull() {
		params.PageToken = new(data.PageToken.ValueString())
	}

	var items []models.CustomerSubscription

	// Smart pagination: honor a user-provided max_results with a single page,
	// otherwise fetch every page.
	if !data.MaxResults.IsNull() {
		params.MaxResults = new(data.MaxResults.ValueInt64())

		page, ok := d.listPage(ctx, params, &resp.Diagnostics)
		if !ok {
			return
		}
		items = page.Items
		// Keep the next page's token for the user.
		data.PageToken = types.StringPointerValue(page.PageToken)
		data.RowCount = types.Int64Value(page.RowCount)
	} else {
		// row_count is the API's total across all pages, even when a page_token
		// starts the read after the first item, so it is taken from the first page.
		first := true
		for {
			page, ok := d.listPage(ctx, params, &resp.Diagnostics)
			if !ok {
				return
			}
			if first {
				data.RowCount = types.Int64Value(page.RowCount)
				first = false
			}
			items = append(items, page.Items...)
			if page.PageToken == nil || *page.PageToken == "" {
				break
			}
			params.PageToken = page.PageToken
		}
		// A configured page_token stays as configured.
		data.MaxResults = types.Int64Null()
	}

	resp.Diagnostics.Append(mapCustomerSubscriptionsToDataSource(ctx, items, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// listPage fetches one page and validates the response.
func (d *customerSubscriptionsDataSource) listPage(ctx context.Context, params *models.ListCustomerSubscriptionsParams, diags *diag.Diagnostics) (*models.CustomerSubscriptionListResponse, bool) {
	apiResp, err := d.client.ListCustomerSubscriptionsWithResponse(ctx, params)
	if err != nil {
		diags.AddError("Error Reading Customer Subscriptions", fmt.Sprintf("Could not list customer subscriptions: %v", err))
		return nil, false
	}
	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		diags.AddError("Error Reading Customer Subscriptions", fmt.Sprintf("status %d, body: %s", apiResp.StatusCode(), string(apiResp.Body)))
		return nil, false
	}
	return apiResp.JSON200, true
}
