package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_billing_gcp_requirements"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	modelsv2 "github.com/doitintl/terraform-provider-doit/internal/provider/models/v2"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*billingGcpRequirementsDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*billingGcpRequirementsDataSource)(nil)

// NewBillingGcpRequirementsDataSource creates a new GCP billing requirements data source instance.
func NewBillingGcpRequirementsDataSource() datasource.DataSource {
	return &billingGcpRequirementsDataSource{}
}

type billingGcpRequirementsDataSource struct {
	client *modelsv2.ClientWithResponses
}

type billingGcpRequirementsDataSourceModel struct {
	datasource_billing_gcp_requirements.BillingGcpRequirementsModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *billingGcpRequirementsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_billing_gcp_requirements"
}

func (d *billingGcpRequirementsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	v2Client, err := newV2Client(client)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			"Could not create the v2 API client: "+err.Error()+". Please report this issue to the provider developers.",
		)
		return
	}

	d.client = v2Client
}

func (d *billingGcpRequirementsDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_billing_gcp_requirements.BillingGcpRequirementsDataSourceSchema(ctx)

	s.Description = "Returns what to set up before connecting a Google Cloud billing account with `doit_billing_gcp_account`: " +
		"the DoiT service account, the IAM roles to grant it and where, and the steps Google offers no API for. " +
		"The first read for a customer reserves its DoiT service account; later reads return the same account."
	s.MarkdownDescription = s.Description

	s.Attributes["timeouts"] = timeouts.Attributes(ctx)

	rejectNullDataSourceCollectionElements(s.Attributes)
	resp.Schema = s
}

func (d *billingGcpRequirementsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data billingGcpRequirementsDataSourceModel

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

	// The only input is timeouts; an unknown one defers the read.
	if !req.Config.Raw.IsFullyKnown() {
		data.ServiceAccountEmail = types.StringUnknown()
		data.Grants = types.ListUnknown(datasource_billing_gcp_requirements.GrantsValue{}.Type(ctx))
		data.Prerequisites = types.ListUnknown(types.StringType)
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	apiResp, err := d.client.ResolveGcpBillingRequirementsWithResponse(ctx, &modelsv2.ResolveGcpBillingRequirementsParams{})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading GCP Billing Requirements",
			fmt.Sprintf("Unable to resolve GCP billing requirements: %v", err),
		)
		return
	}

	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading GCP Billing Requirements",
			fmt.Sprintf("Unable to resolve GCP billing requirements, status: %d, body: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	resp.Diagnostics.Append(mapBillingGcpRequirementsToModel(ctx, apiResp.JSON200, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// mapBillingGcpRequirementsToModel maps the requirements response. Lists are
// empty, never null, when the API returns none.
func mapBillingGcpRequirementsToModel(ctx context.Context, resp *modelsv2.GcpBillingRequirementsResponse, data *billingGcpRequirementsDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	data.ServiceAccountEmail = types.StringValue(string(resp.ServiceAccountEmail))

	grants := make([]datasource_billing_gcp_requirements.GrantsValue, 0, len(resp.Grants))
	for _, grant := range resp.Grants {
		value, d := datasource_billing_gcp_requirements.NewGrantsValue(
			datasource_billing_gcp_requirements.GrantsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"role":   types.StringValue(grant.Role),
				"target": types.StringValue(string(grant.Target)),
			},
		)
		diags.Append(d...)
		grants = append(grants, value)
	}
	if diags.HasError() {
		return diags
	}

	var d diag.Diagnostics
	data.Grants, d = types.ListValueFrom(ctx, datasource_billing_gcp_requirements.GrantsValue{}.Type(ctx), grants)
	diags.Append(d...)

	prerequisites := resp.Prerequisites
	if prerequisites == nil {
		prerequisites = []string{}
	}
	data.Prerequisites, d = types.ListValueFrom(ctx, types.StringType, prerequisites)
	diags.Append(d...)

	return diags
}
