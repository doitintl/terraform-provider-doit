package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_ps4c_gcp_recommendation"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource                     = (*ps4cGcpRecommendationDataSource)(nil)
	_ datasource.DataSourceWithConfigure        = (*ps4cGcpRecommendationDataSource)(nil)
	_ datasource.DataSourceWithConfigValidators = (*ps4cGcpRecommendationDataSource)(nil)
)

func NewPs4cGcpRecommendationDataSource() datasource.DataSource {
	return &ps4cGcpRecommendationDataSource{}
}

type ps4cGcpRecommendationDataSource struct {
	client *models.ClientWithResponses
}

type ps4cGcpRecommendationDataSourceModel struct {
	datasource_ps4c_gcp_recommendation.Ps4cGcpRecommendationModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *ps4cGcpRecommendationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ps4c_gcp_recommendation"
}

func (d *ps4cGcpRecommendationDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ps4cGcpRecommendationDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_ps4c_gcp_recommendation.Ps4cGcpRecommendationDataSourceSchema(ctx)
	s.Description = "Retrieves a Google Cloud Platform (GCP) PerfectScale for Commitments purchase recommendation and eligible usage history."
	s.MarkdownDescription = "Retrieves a Google Cloud Platform (GCP) PerfectScale for Commitments purchase recommendation and eligible usage history."
	s.Attributes["timeouts"] = timeouts.Attributes(ctx)
	resp.Schema = s
}

func (d *ps4cGcpRecommendationDataSource) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		ps4cGcpRecommendationConfigValidator{},
	}
}

func (d *ps4cGcpRecommendationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ps4cGcpRecommendationDataSourceModel

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

	if data.BillingAccountId.IsUnknown() || data.GcpService.IsUnknown() || data.Region.IsUnknown() || data.Granularity.IsUnknown() {
		data.EligibleUsage = types.ListUnknown(datasource_ps4c_gcp_recommendation.EligibleUsageValue{}.Type(ctx))
		data.EstimatedEquivalentRecommendedCommitment = types.Float64Unknown()
		data.Recommendation = datasource_ps4c_gcp_recommendation.NewRecommendationValueUnknown()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	billingAccountId := data.BillingAccountId.ValueString()
	gcpService := models.GetGcpRecommendationParamsGcpService(data.GcpService.ValueString())

	params := &models.GetGcpRecommendationParams{}
	if !data.Region.IsNull() {
		params.Region = data.Region.ValueStringPointer()
	}
	if !data.Granularity.IsNull() {
		params.Granularity = (*models.GetGcpRecommendationParamsGranularity)(data.Granularity.ValueStringPointer())
	}

	apiResp, err := d.client.GetGcpRecommendationWithResponse(ctx, billingAccountId, gcpService, params)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading PS4C GCP Recommendation", fmt.Sprintf("Unable to read PS4C GCP recommendation: %v", err))
		return
	}

	if apiResp.StatusCode() == 404 {
		resp.Diagnostics.AddError(
			"PS4C GCP Recommendation Not Found",
			fmt.Sprintf("GCP recommendation not found for billing account %s, service %s, status: 404, body: %s",
				billingAccountId, data.GcpService.ValueString(), string(apiResp.Body)),
		)
		return
	}

	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading PS4C GCP Recommendation",
			fmt.Sprintf("Could not read PS4C GCP recommendation, status: %d, body: %s",
				apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	mapDiags := mapGcpRecommendationToModel(ctx, apiResp.JSON200, &data)
	resp.Diagnostics.Append(mapDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
