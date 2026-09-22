package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_ps4c_gcp_planned_purchases"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*ps4cGcpPlannedPurchasesDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*ps4cGcpPlannedPurchasesDataSource)(nil)

func NewPs4cGcpPlannedPurchasesDataSource() datasource.DataSource {
	return &ps4cGcpPlannedPurchasesDataSource{}
}

type ps4cGcpPlannedPurchasesDataSource struct {
	client *models.ClientWithResponses
}

type ps4cGcpPlannedPurchasesDataSourceModel struct {
	datasource_ps4c_gcp_planned_purchases.Ps4cGcpPlannedPurchasesModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *ps4cGcpPlannedPurchasesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ps4c_gcp_planned_purchases"
}

func (d *ps4cGcpPlannedPurchasesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ps4cGcpPlannedPurchasesDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_ps4c_gcp_planned_purchases.Ps4cGcpPlannedPurchasesDataSourceSchema(ctx)

	s.MarkdownDescription = "Retrieve planned commitment purchase projections for a GCP billing account tracked by PerfectScale for Commitments (PS4C)."
	s.Description = "Retrieve planned commitment purchase projections for a GCP billing account tracked by PerfectScale for Commitments (PS4C)."

	s.Attributes["timeouts"] = timeouts.Attributes(ctx)

	resp.Schema = s
}

func (d *ps4cGcpPlannedPurchasesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ps4cGcpPlannedPurchasesDataSourceModel

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

	if data.BillingAccountId.IsUnknown() || data.GcpService.IsUnknown() || data.Region.IsUnknown() || data.MaxResults.IsUnknown() || data.PageToken.IsUnknown() {
		data.Items = types.ListUnknown(datasource_ps4c_gcp_planned_purchases.ItemsValue{}.Type(ctx))
		data.RowCount = types.Int64Unknown()
		data.PageToken = types.StringUnknown()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	billingAccountId := data.BillingAccountId.ValueString()
	params := &models.ListGcpPlannedPurchasesParams{}

	if !data.GcpService.IsNull() {
		params.GcpService = new(models.ListGcpPlannedPurchasesParamsGcpService(data.GcpService.ValueString()))
	}

	if !data.Region.IsNull() {
		params.Region = new(data.Region.ValueString())
	}

	userControlsPagination := !data.MaxResults.IsNull()

	var allPurchases []models.GcpPlannedPurchaseByService

	if userControlsPagination {
		params.MaxResults = new(int(data.MaxResults.ValueInt64()))
		if !data.PageToken.IsNull() {
			params.PageToken = new(data.PageToken.ValueString())
		}

		apiResp, err := d.client.ListGcpPlannedPurchasesWithResponse(ctx, billingAccountId, params)
		if err != nil {
			resp.Diagnostics.AddError("Error Reading PS4C GCP Planned Purchases", fmt.Sprintf("Unable to read PS4C GCP planned purchases: %v", err))
			return
		}
		if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
			resp.Diagnostics.AddError("Error Reading PS4C GCP Planned Purchases", fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)))
			return
		}

		result := apiResp.JSON200
		allPurchases = result.Items

		data.PageToken = types.StringPointerValue(nullableToPointer(result.PageToken))
		data.RowCount = types.Int64Value(result.RowCount)
	} else {
		if !data.PageToken.IsNull() {
			params.PageToken = new(data.PageToken.ValueString())
		}
		for {
			apiResp, err := d.client.ListGcpPlannedPurchasesWithResponse(ctx, billingAccountId, params)
			if err != nil {
				resp.Diagnostics.AddError("Error Reading PS4C GCP Planned Purchases", fmt.Sprintf("Unable to read PS4C GCP planned purchases: %v", err))
				return
			}
			if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
				resp.Diagnostics.AddError("Error Reading PS4C GCP Planned Purchases", fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)))
				return
			}

			result := apiResp.JSON200
			allPurchases = append(allPurchases, result.Items...)

			pageToken := nullableToPointer(result.PageToken)
			if pageToken == nil || *pageToken == "" {
				break
			}
			params.PageToken = pageToken
		}

		data.RowCount = types.Int64Value(int64(len(allPurchases)))
		data.PageToken = types.StringNull()
	}

	itemsList, itemsDiags := mapGcpPlannedPurchasesToItemsList(ctx, allPurchases)
	resp.Diagnostics.Append(itemsDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Items = itemsList

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
