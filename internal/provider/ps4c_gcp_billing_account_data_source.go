package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_ps4c_gcp_billing_account"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*ps4cGcpBillingAccountDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*ps4cGcpBillingAccountDataSource)(nil)

func NewPs4cGcpBillingAccountDataSource() datasource.DataSource {
	return &ps4cGcpBillingAccountDataSource{}
}

type ps4cGcpBillingAccountDataSource struct {
	client *models.ClientWithResponses
}

type ps4cGcpBillingAccountDataSourceModel struct {
	datasource_ps4c_gcp_billing_account.Ps4cGcpBillingAccountModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *ps4cGcpBillingAccountDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ps4c_gcp_billing_account"
}

func (d *ps4cGcpBillingAccountDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ps4cGcpBillingAccountDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_ps4c_gcp_billing_account.Ps4cGcpBillingAccountDataSourceSchema(ctx)

	s.MarkdownDescription = "Retrieve a single GCP Billing Account tracked by PerfectScale for Commitments (PS4C), by its billing account ID."
	s.Description = "Retrieve a single GCP Billing Account tracked by PerfectScale for Commitments (PS4C), by its billing account ID."

	s.Attributes["timeouts"] = timeouts.Attributes(ctx)

	resp.Schema = s
}

func (d *ps4cGcpBillingAccountDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ps4cGcpBillingAccountDataSourceModel

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

	if data.BillingAccountId.IsUnknown() {
		data.CommitmentsSyncTime = types.StringUnknown()
		data.CudExportHealthy = types.BoolUnknown()
		data.Currency = types.StringUnknown()
		data.DailyCoverage = datasource_ps4c_gcp_billing_account.NewDailyCoverageValueUnknown()
		data.DisplayName = types.StringUnknown()
		data.MonthlyStats = datasource_ps4c_gcp_billing_account.NewMonthlyStatsValueUnknown()
		data.OnboardingStatus = datasource_ps4c_gcp_billing_account.NewOnboardingStatusValueUnknown()
		data.SavingsTotals = datasource_ps4c_gcp_billing_account.NewSavingsTotalsValueUnknown()
		data.Stats30d = datasource_ps4c_gcp_billing_account.NewStats30dValueUnknown()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	billingAccountId := data.BillingAccountId.ValueString()
	params := &models.GetGcpBillingAccountParams{}

	apiResp, err := d.client.GetGcpBillingAccountWithResponse(ctx, billingAccountId, params)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading PS4C GCP Billing Account",
			"Could not read PS4C GCP Billing Account "+billingAccountId+": "+err.Error(),
		)
		return
	}

	if apiResp.StatusCode() == 404 {
		resp.Diagnostics.AddError(
			"PS4C GCP Billing Account Not Found",
			fmt.Sprintf("GCP Billing Account with ID %s not found", billingAccountId),
		)
		return
	}

	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading PS4C GCP Billing Account",
			fmt.Sprintf("Could not read PS4C GCP Billing Account %s, status: %d, body: %s",
				billingAccountId, apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	resp.Diagnostics.Append(mapGcpBillingAccountDetailToModel(ctx, apiResp.JSON200, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
