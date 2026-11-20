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
		"Without `max_results` every page is fetched; with it, a single page is returned together with the `page_token` of the next one."
	s.MarkdownDescription = s.Description

	filter, okFilter := s.Attributes["filter"].(schema.StringAttribute)
	maxResults, okMaxResults := s.Attributes["max_results"].(schema.Int64Attribute)
	pageToken, okPageToken := s.Attributes["page_token"].(schema.StringAttribute)
	rowCount, okRowCount := s.Attributes["row_count"].(schema.Int64Attribute)
	if !okFilter || !okMaxResults || !okPageToken || !okRowCount {
		resp.Diagnostics.AddError("Unexpected Schema", "Expected filter and page_token to be string attributes and max_results and row_count to be number attributes.")
		return
	}

	filter.Description = "Restricts the result by data source, as `dataSource:<value>[,<value>...]`. " +
		"The only supported key is `dataSource`; its values are `billing`, `billing-datahub`, `bqlens`, `genai-collection`, `kubernetes-utilization` and `tokenomics`. " +
		"For example `dataSource:billing,billing-datahub` lists the keys a custom metric variable accepts. " +
		"A `page_token` is bound to the `filter` it was returned for; reusing it with another `filter` fails."
	filter.MarkdownDescription = filter.Description
	s.Attributes["filter"] = filter

	maxResults.Description = "The maximum number of items to return, at most 500. " +
		"Setting it returns a single page together with the `page_token` of the next page; omit it to fetch every page."
	maxResults.MarkdownDescription = maxResults.Description
	s.Attributes["max_results"] = maxResults

	pageToken.Description = "Token of the page to start at. " +
		"With `max_results` set, the page is read once and `page_token` holds the token of the next page, which is null after the last page. " +
		"Without `max_results`, every page from this token on is fetched and the configured token is kept."
	pageToken.MarkdownDescription = pageToken.Description
	s.Attributes["page_token"] = pageToken

	rowCount.Description = "The number of extended metrics that match `filter`, across all pages, as reported by the API. " +
		"It is the same for every read, whatever `max_results` and `page_token` are, so with either set it is larger than the number of `items`. " +
		"If the API reports no count, `row_count` is instead the number of items this read fetched, which is a partial count when `max_results` or `page_token` is set."
	rowCount.MarkdownDescription = rowCount.Description
	s.Attributes["row_count"] = rowCount

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
		// The inputs are Optional+Computed: a configured value stays as
		// configured, an omitted one is not yet determined.
		if data.Filter.IsNull() {
			data.Filter = types.StringUnknown()
		}
		// With max_results set, or not yet known, page_token is the next page's
		// token. Only in auto-pagination mode (max_results omitted) is a
		// configured one just the starting point, which stays as configured.
		autoPagination := data.MaxResults.IsNull()
		if data.MaxResults.IsNull() {
			data.MaxResults = types.Int64Unknown()
		}
		if !autoPagination || data.PageToken.IsNull() {
			data.PageToken = types.StringUnknown()
		}
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
		data.RowCount = extendedMetricsRowCount(page, len(items))
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
		data.RowCount = extendedMetricsRowCount(firstPage, len(items))
		// A configured page_token stays as configured.
		data.MaxResults = types.Int64Null()
	}

	// An omitted filter reads back as null, not unknown.
	if data.Filter.IsUnknown() {
		data.Filter = types.StringNull()
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
