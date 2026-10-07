package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_ps4c_gcp_recommendations"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource                     = (*ps4cGcpRecommendationsDataSource)(nil)
	_ datasource.DataSourceWithConfigure        = (*ps4cGcpRecommendationsDataSource)(nil)
	_ datasource.DataSourceWithConfigValidators = (*ps4cGcpRecommendationsDataSource)(nil)
)

func NewPs4cGcpRecommendationsDataSource() datasource.DataSource {
	return &ps4cGcpRecommendationsDataSource{}
}

type ps4cGcpRecommendationsDataSource struct {
	client *models.ClientWithResponses
}

type ps4cGcpRecommendationsDataSourceModel struct {
	datasource_ps4c_gcp_recommendations.Ps4cGcpRecommendationsModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *ps4cGcpRecommendationsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ps4c_gcp_recommendations"
}

func (d *ps4cGcpRecommendationsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*models.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *models.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *ps4cGcpRecommendationsDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_ps4c_gcp_recommendations.Ps4cGcpRecommendationsDataSourceSchema(ctx)
	s.Description = "Lists Google Cloud Platform (GCP) PerfectScale for Commitments purchase recommendations for a billing account."
	s.MarkdownDescription = "Lists Google Cloud Platform (GCP) PerfectScale for Commitments purchase recommendations for a billing account."
	s.Attributes["timeouts"] = timeouts.Attributes(ctx)
	resp.Schema = s
}

func (d *ps4cGcpRecommendationsDataSource) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		ps4cGcpRecommendationsConfigValidator{},
	}
}

func (d *ps4cGcpRecommendationsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ps4cGcpRecommendationsDataSourceModel

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

	if data.BillingAccountId.IsUnknown() || data.GcpService.IsUnknown() || data.Region.IsUnknown() {
		data.Items = types.ListUnknown(datasource_ps4c_gcp_recommendations.ItemsValue{}.Type(ctx))
		data.RowCount = types.Int64Unknown()
		data.PageToken = types.StringUnknown()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	billingAccountId := data.BillingAccountId.ValueString()
	params := &models.ListGcpRecommendationsParams{}
	if !data.GcpService.IsNull() {
		params.GcpService = (*models.ListGcpRecommendationsParamsGcpService)(data.GcpService.ValueStringPointer())
	}
	if !data.Region.IsNull() {
		params.Region = data.Region.ValueStringPointer()
	}

	apiResp, err := d.client.ListGcpRecommendationsWithResponse(ctx, billingAccountId, params)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading PS4C GCP Recommendations", fmt.Sprintf("Could not read PS4C GCP recommendations: %v", err))
		return
	}

	if apiResp.StatusCode() == 404 {
		resp.Diagnostics.AddError(
			"PS4C GCP Billing Account Not Found or Inaccessible",
			fmt.Sprintf("GCP Billing Account with ID %s not found or caller cannot access it, status: 404, body: %s",
				billingAccountId, string(apiResp.Body)),
		)
		return
	}

	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading PS4C GCP Recommendations",
			fmt.Sprintf("Could not read PS4C GCP recommendations for billing account %s, status: %d, body: %s",
				billingAccountId, apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	itemsList, itemsDiags := mapGcpRecommendationsToItemsList(ctx, apiResp.JSON200.Items)
	resp.Diagnostics.Append(itemsDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Items = itemsList

	data.PageToken = types.StringPointerValue(nullableToPointer(apiResp.JSON200.PageToken))
	if rowCount := apiResp.JSON200.RowCount; rowCount != nil {
		data.RowCount = types.Int64Value(*rowCount)
	} else {
		data.RowCount = types.Int64Value(int64(len(apiResp.JSON200.Items)))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
