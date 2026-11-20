package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_extended_metrics"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*extendedMetricsDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*extendedMetricsDataSource)(nil)

func NewExtendedMetricsDataSource() datasource.DataSource {
	return &extendedMetricsDataSource{}
}

type extendedMetricsDataSource struct{ client *models.ClientWithResponses }

type extendedMetricsDataSourceModel struct {
	datasource_extended_metrics.ExtendedMetricsModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *extendedMetricsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_extended_metrics"
}

func (d *extendedMetricsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *extendedMetricsDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_extended_metrics.ExtendedMetricsDataSourceSchema(ctx)

	s.Description = "Extended metrics available to the customer: the global extended metrics and the customer's DataHub metrics. " +
		"The `key` of an item is the `value` of `metric = { type = \"extended\", value = <key> }` in `doit_custom_metric` variables, reports and alerts. " +
		"Where a key is accepted depends on its `data_source`: a custom metric variable accepts only `billing` and `billing-datahub` keys " +
		"(use `filter = \"dataSource:billing,billing-datahub\"` to list them), and a report or alert accepts the keys of its own data source. " +
		"A key appears once per data source, so `key` alone is not unique across items. " +
		"Items are sorted by `label`, then `key`, then `data_source`. " +
		"Without `max_results` every page is fetched; with it, a single page of at most 500 items is returned together with the `page_token` of the next one."
	s.MarkdownDescription = s.Description

	filter, okFilter := s.Attributes["filter"].(schema.StringAttribute)
	if !okFilter {
		resp.Diagnostics.AddError("Unexpected Schema", "Expected filter to be a string attribute.")
		return
	}

	filter.Description = "Restricts the result by data source, as `dataSource:<value>[,<value>...]`. " +
		"The only supported key is `dataSource`; its values are `billing`, `billing-datahub`, `bqlens`, `genai-collection`, `kubernetes-utilization` and `tokenomics`. " +
		"For example `dataSource:billing,billing-datahub` lists the keys a custom metric variable accepts. " +
		"A `page_token` is bound to the `filter` it was returned for; reusing it with another `filter` fails."
	filter.MarkdownDescription = filter.Description
	s.Attributes["filter"] = filter

	resp.Diagnostics.Append(setListPaginationDescriptions(ctx, &s, listRowCountTotal, "")...)

	s.Attributes["timeouts"] = timeouts.Attributes(ctx)
	resp.Schema = s
}

func (d *extendedMetricsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data extendedMetricsDataSourceModel
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
		data.Items = types.ListUnknown(datasource_extended_metrics.ItemsValue{}.Type(ctx))
		data.RowCount = types.Int64Unknown()
		// page_token is a result in every mode: a read replaces it with the
		// next page's token, or null. The inputs stay as configured.
		data.PageToken = types.StringUnknown()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	params := &models.ListExtendedMetricsParams{}
	if !data.Filter.IsNull() {
		params.Filter = new(data.Filter.ValueString())
	}
	if !data.PageToken.IsNull() {
		params.PageToken = new(data.PageToken.ValueString())
	}

	var items []models.ExtendedMetric

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
		// row_count is the API's total across all pages, even when a page_token
		// starts the read after the first item, so it is taken from the first page.
		var firstPage *models.ExtendedMetricListResponse
		for {
			page, ok := d.listPage(ctx, params, &resp.Diagnostics)
			if !ok {
				return
			}
			items = append(items, page.Items...)
			if firstPage == nil {
				firstPage = page
			}
			if page.PageToken == nil || *page.PageToken == "" {
				break
			}
			params.PageToken = page.PageToken
		}
		// Without a count from the API, the items read are the best total there is.
		data.RowCount = listRowCount(nullableToPointer(firstPage.RowCount), len(items))
		data.PageToken = types.StringNull()
		data.MaxResults = types.Int64Null()
	}

	resp.Diagnostics.Append(mapExtendedMetricsToDataSource(ctx, items, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// listPage fetches one page and validates the response.
func (d *extendedMetricsDataSource) listPage(ctx context.Context, params *models.ListExtendedMetricsParams, diags *diag.Diagnostics) (*models.ExtendedMetricListResponse, bool) {
	apiResp, err := d.client.ListExtendedMetricsWithResponse(ctx, params)
	if err != nil {
		diags.AddError("Error Reading Extended Metrics", fmt.Sprintf("Could not list extended metrics: %v", err))
		return nil, false
	}
	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		diags.AddError("Error Reading Extended Metrics", fmt.Sprintf("status %d, body: %s", apiResp.StatusCode(), string(apiResp.Body)))
		return nil, false
	}
	return apiResp.JSON200, true
}
