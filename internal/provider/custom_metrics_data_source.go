package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_custom_metrics"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*customMetricsDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*customMetricsDataSource)(nil)

func NewCustomMetricsDataSource() datasource.DataSource {
	return &customMetricsDataSource{}
}

type customMetricsDataSource struct{ client *models.ClientWithResponses }

type customMetricsDataSourceModel struct {
	datasource_custom_metrics.CustomMetricsModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *customMetricsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_custom_metrics"
}

func (d *customMetricsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *customMetricsDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_custom_metrics.CustomMetricsDataSourceSchema(ctx)

	s.Description = "Custom metrics of the customer. Preset metrics are not listed. Without `max_results` every page is fetched; with it, a single page is returned together with the `page_token` of the next one. A `page_token` is bound to `name_contains`, `filter`, `sort_by` and `sort_order`: reuse it only with the same values."
	s.MarkdownDescription = s.Description

	resp.Diagnostics.Append(setListPaginationDescriptions(&s, listRowCountTotal, "")...)
	s.Attributes["timeouts"] = timeouts.Attributes(ctx)
	resp.Schema = s
}

func (d *customMetricsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data customMetricsDataSourceModel
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
		data.Items = types.ListUnknown(datasource_custom_metrics.ItemsValue{}.Type(ctx))
		data.RowCount = types.Int64Unknown()
		// page_token is a result in every mode: a read replaces it with the
		// next page's token, or null. The inputs stay as configured.
		data.PageToken = types.StringUnknown()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	params := &models.ListCustomMetricsParams{}
	if !data.Filter.IsNull() {
		params.Filter = new(data.Filter.ValueString())
	}
	if !data.NameContains.IsNull() {
		params.NameContains = new(data.NameContains.ValueString())
	}
	if !data.SortBy.IsNull() {
		params.SortBy = new(models.ListCustomMetricsParamsSortBy(data.SortBy.ValueString()))
	}
	if !data.SortOrder.IsNull() {
		params.SortOrder = new(models.ListCustomMetricsParamsSortOrder(data.SortOrder.ValueString()))
	}
	if !data.PageToken.IsNull() {
		params.PageToken = new(data.PageToken.ValueString())
	}

	var items []models.CustomMetric

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
		data.RowCount = listRowCount(nullableToPointer(page.RowCount), len(items))
	} else {
		// row_count is the API's total across all pages, so it is taken from the
		// first page even when a page_token starts the read after the first item.
		// Without a count from the API, the number of items fetched over all
		// pages stands in for it.
		var apiRowCount *int64
		first := true
		for {
			page, ok := d.listPage(ctx, params, &resp.Diagnostics)
			if !ok {
				return
			}
			if first {
				apiRowCount = nullableToPointer(page.RowCount)
				first = false
			}
			items = append(items, page.Items...)
			if page.PageToken == nil || *page.PageToken == "" {
				break
			}
			params.PageToken = page.PageToken
		}
		data.RowCount = listRowCount(apiRowCount, len(items))
		data.PageToken = types.StringNull()
		data.MaxResults = types.Int64Null()
	}

	resp.Diagnostics.Append(mapCustomMetricsToDataSource(ctx, items, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// listPage fetches one page and validates the response.
func (d *customMetricsDataSource) listPage(ctx context.Context, params *models.ListCustomMetricsParams, diags *diag.Diagnostics) (*models.CustomMetricListResponse, bool) {
	apiResp, err := d.client.ListCustomMetricsWithResponse(ctx, params)
	if err != nil {
		diags.AddError("Error Reading Custom Metrics", fmt.Sprintf("Could not list custom metrics: %v", err))
		return nil, false
	}
	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		diags.AddError("Error Reading Custom Metrics", fmt.Sprintf("status %d, body: %s", apiResp.StatusCode(), string(apiResp.Body)))
		return nil, false
	}
	return apiResp.JSON200, true
}
