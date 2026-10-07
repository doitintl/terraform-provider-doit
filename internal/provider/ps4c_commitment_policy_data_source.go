package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_ps4c_commitment_policy"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*ps4cCommitmentPolicyDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*ps4cCommitmentPolicyDataSource)(nil)

func NewPs4cCommitmentPolicyDataSource() datasource.DataSource {
	return &ps4cCommitmentPolicyDataSource{}
}

type ps4cCommitmentPolicyDataSource struct {
	client *models.ClientWithResponses
}

type ps4cCommitmentPolicyDataSourceModel struct {
	datasource_ps4c_commitment_policy.Ps4cCommitmentPolicyModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *ps4cCommitmentPolicyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ps4c_commitment_policy"
}

func (d *ps4cCommitmentPolicyDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ps4cCommitmentPolicyDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_ps4c_commitment_policy.Ps4cCommitmentPolicyDataSourceSchema(ctx)

	s.MarkdownDescription = "Retrieve a single PerfectScale for Commitments (PS4C) commitment policy by its ID."
	s.Description = "Retrieve a single PerfectScale for Commitments (PS4C) commitment policy by its ID."

	s.Attributes["timeouts"] = timeouts.Attributes(ctx)

	resp.Schema = s
}

func (d *ps4cCommitmentPolicyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ps4cCommitmentPolicyDataSourceModel
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

	// If the policy ID is unknown (depends on a resource not yet created), set
	// every computed attribute to unknown so consumers don't treat null as a
	// real value during planning.
	if data.PolicyId.IsUnknown() {
		data.Id = types.StringUnknown()
		data.Name = types.StringUnknown()
		data.IsBuiltIn = types.BoolUnknown()
		data.TargetCoverage = types.Float64Unknown()
		data.LookbackDays = types.Int64Unknown()
		data.LadderStepPercent = types.Float64Unknown()
		data.LadderIntervalDays = types.Int64Unknown()
		data.BootstrapPercent = types.Float64Unknown()
		data.CreatedTime = types.StringUnknown()
		data.UpdatedTime = types.StringUnknown()
		data.Assignments = types.ListUnknown(datasource_ps4c_commitment_policy.AssignmentsValue{}.Type(ctx))
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	policyId := data.PolicyId.ValueString()
	apiResp, err := d.client.GetCommitmentPolicyWithResponse(ctx, policyId, &models.GetCommitmentPolicyParams{})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading PS4C Commitment Policy",
			"Could not read PS4C Commitment Policy "+policyId+": "+err.Error(),
		)
		return
	}

	if apiResp.StatusCode() == 404 {
		resp.Diagnostics.AddError(
			"PS4C Commitment Policy Not Found",
			fmt.Sprintf("Commitment policy with ID %s not found", policyId),
		)
		return
	}

	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading PS4C Commitment Policy",
			fmt.Sprintf("Could not read PS4C Commitment Policy %s, status: %d, body: %s",
				policyId, apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	resp.Diagnostics.Append(mapCommitmentPolicyDetailToModel(ctx, apiResp.JSON200, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
