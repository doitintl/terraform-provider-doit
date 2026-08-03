package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_ps4c_aws_organization"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*ps4cAwsOrganizationDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*ps4cAwsOrganizationDataSource)(nil)

func NewPs4cAwsOrganizationDataSource() datasource.DataSource {
	return &ps4cAwsOrganizationDataSource{}
}

type ps4cAwsOrganizationDataSource struct {
	client *models.ClientWithResponses
}

type ps4cAwsOrganizationDataSourceModel struct {
	datasource_ps4c_aws_organization.Ps4cAwsOrganizationModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *ps4cAwsOrganizationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ps4c_aws_organization"
}

func (d *ps4cAwsOrganizationDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ps4cAwsOrganizationDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_ps4c_aws_organization.Ps4cAwsOrganizationDataSourceSchema(ctx)

	s.MarkdownDescription = "Retrieve a single AWS Organization tracked by PerfectScale for Commitments (PS4C), by its management (payer) account ID."
	s.Description = "Retrieve a single AWS Organization tracked by PerfectScale for Commitments (PS4C), by its management (payer) account ID."

	s.Attributes["timeouts"] = timeouts.Attributes(ctx)

	resp.Schema = s
}

func (d *ps4cAwsOrganizationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ps4cAwsOrganizationDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	readTimeout, diags := data.Timeouts.Read(ctx, 2*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	// If the account ID is unknown (depends on a resource not yet created), set
	// every computed attribute to unknown so consumers don't treat null as a
	// real value during planning.
	if data.ManagementAccountId.IsUnknown() {
		data.DisplayName = types.StringUnknown()
		data.SavingsPlansSyncTime = types.StringUnknown()
		data.OnboardingStatus = datasource_ps4c_aws_organization.NewOnboardingStatusValueUnknown()
		data.Stats30d = datasource_ps4c_aws_organization.NewStats30dValueUnknown()
		data.SavingsTotals = datasource_ps4c_aws_organization.NewSavingsTotalsValueUnknown()
		data.MonthlyPotentialSavings = datasource_ps4c_aws_organization.NewMonthlyPotentialSavingsValueUnknown()
		data.MonthlyStats = datasource_ps4c_aws_organization.NewMonthlyStatsValueUnknown()
		data.DailyCoverage = datasource_ps4c_aws_organization.NewDailyCoverageValueUnknown()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	managementAccountId := data.ManagementAccountId.ValueString()
	orgResp, err := d.client.GetAwsOrganizationWithResponse(ctx, managementAccountId, &models.GetAwsOrganizationParams{})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading PS4C AWS Organization",
			"Could not read PS4C AWS Organization "+managementAccountId+": "+err.Error(),
		)
		return
	}

	if orgResp.StatusCode() == 404 {
		resp.Diagnostics.AddError(
			"PS4C AWS Organization Not Found",
			fmt.Sprintf("AWS Organization with management account ID %s not found", managementAccountId),
		)
		return
	}

	if orgResp.StatusCode() != 200 || orgResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading PS4C AWS Organization",
			fmt.Sprintf("Could not read PS4C AWS Organization %s, status: %d, body: %s",
				managementAccountId, orgResp.StatusCode(), string(orgResp.Body)),
		)
		return
	}

	resp.Diagnostics.Append(mapAwsOrganizationDetailToModel(ctx, orgResp.JSON200, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
