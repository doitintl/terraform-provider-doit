package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_ps4c_gcp_resource_cuds"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*ps4cGcpResourceCudsDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*ps4cGcpResourceCudsDataSource)(nil)

func NewPs4cGcpResourceCudsDataSource() datasource.DataSource {
	return &ps4cGcpResourceCudsDataSource{}
}

type ps4cGcpResourceCudsDataSource struct {
	client *models.ClientWithResponses
}

type ps4cGcpResourceCudsDataSourceModel struct {
	datasource_ps4c_gcp_resource_cuds.Ps4cGcpResourceCudsModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *ps4cGcpResourceCudsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ps4c_gcp_resource_cuds"
}

func (d *ps4cGcpResourceCudsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ps4cGcpResourceCudsDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_ps4c_gcp_resource_cuds.Ps4cGcpResourceCudsDataSourceSchema(ctx)

	s.MarkdownDescription = "List resource-based commitments (CUDs) for a GCP billing account tracked by PerfectScale for Commitments (PS4C)."
	s.Description = "List resource-based commitments (CUDs) for a GCP billing account tracked by PerfectScale for Commitments (PS4C)."

	resp.Diagnostics.Append(setListPaginationDescriptions(&s, listRowCountTotal, "A `page_token` expires. An invalid or expired one fails the read; read again without `page_token` to start over.")...)
	s.Attributes["timeouts"] = timeouts.Attributes(ctx)

	resp.Schema = s
}

func (d *ps4cGcpResourceCudsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ps4cGcpResourceCudsDataSourceModel

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
		data.Items = types.ListUnknown(datasource_ps4c_gcp_resource_cuds.ItemsValue{}.Type(ctx))
		data.RowCount = types.Int64Unknown()
		data.PageToken = types.StringUnknown()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	billingAccountId := data.BillingAccountId.ValueString()
	params := &models.ListGcpResourceCudsParams{}

	if !data.Status.IsNull() {
		params.Status = new(models.ListGcpResourceCudsParamsStatus(data.Status.ValueString()))
	}

	userControlsPagination := !data.MaxResults.IsNull()

	var allCuds []models.GcpResourceCud

	if userControlsPagination {
		params.MaxResults = new(int(data.MaxResults.ValueInt64()))
		if !data.PageToken.IsNull() {
			params.PageToken = new(data.PageToken.ValueString())
		}

		apiResp, err := d.client.ListGcpResourceCudsWithResponse(ctx, billingAccountId, params)
		if err != nil {
			resp.Diagnostics.AddError("Error Reading PS4C GCP Resource CUDs", fmt.Sprintf("Unable to read PS4C GCP resource CUDs: %v", err))
			return
		}
		if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
			resp.Diagnostics.AddError("Error Reading PS4C GCP Resource CUDs", fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)))
			return
		}

		result := apiResp.JSON200
		allCuds = result.Items

		data.PageToken = types.StringPointerValue(nullableToPointer(result.PageToken))
		data.RowCount = listRowCount(nullableToPointer(result.RowCount), len(allCuds))
	} else {
		if !data.PageToken.IsNull() {
			params.PageToken = new(data.PageToken.ValueString())
		}
		// rowCount is the total across all pages, the same on every page, so it
		// is taken from the first page even when a page_token starts the read
		// after the first item.
		var apiRowCount *int64
		for first := true; ; first = false {
			apiResp, err := d.client.ListGcpResourceCudsWithResponse(ctx, billingAccountId, params)
			if err != nil {
				resp.Diagnostics.AddError("Error Reading PS4C GCP Resource CUDs", fmt.Sprintf("Unable to read PS4C GCP resource CUDs: %v", err))
				return
			}
			if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
				resp.Diagnostics.AddError("Error Reading PS4C GCP Resource CUDs", fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)))
				return
			}

			result := apiResp.JSON200
			if first {
				apiRowCount = nullableToPointer(result.RowCount)
			}
			allCuds = append(allCuds, result.Items...)

			pageToken := nullableToPointer(result.PageToken)
			if pageToken == nil || *pageToken == "" {
				break
			}
			params.PageToken = pageToken
		}

		// Without a count from the API, the items fetched over all pages stand in for it.
		data.RowCount = listRowCount(apiRowCount, len(allCuds))
		data.PageToken = types.StringNull()
		data.MaxResults = types.Int64Null()
	}

	itemsList, itemsDiags := mapGcpResourceCudsToItemsList(ctx, allCuds)
	resp.Diagnostics.Append(itemsDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Items = itemsList

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
