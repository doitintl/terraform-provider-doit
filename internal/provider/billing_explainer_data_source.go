package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_billing_explainer"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*billingExplainerDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*billingExplainerDataSource)(nil)

func NewBillingExplainerDataSource() datasource.DataSource {
	return &billingExplainerDataSource{}
}

type billingExplainerDataSource struct {
	client *models.ClientWithResponses
}

type billingExplainerDataSourceModel struct {
	datasource_billing_explainer.BillingExplainerModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (ds *billingExplainerDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_billing_explainer"
}

func (ds *billingExplainerDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
	ds.client = client
}

func (ds *billingExplainerDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_billing_explainer.BillingExplainerDataSourceSchema(ctx)

	s.Attributes["timeouts"] = timeouts.Attributes(ctx)

	resp.Schema = s
}

func (ds *billingExplainerDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state billingExplainerDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	readTimeout, diags := state.Timeouts.Read(ctx, 2*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	// If invoice_month is unknown (depends on a resource not yet created), set
	// every computed attribute to unknown so consumers don't treat null as a
	// real value during planning.
	if state.InvoiceMonth.IsUnknown() {
		state.CustomerId = types.StringUnknown()
		state.DoitCredits = datasource_billing_explainer.NewDoitCreditsValueUnknown()
		state.InvoiceAdjustments = datasource_billing_explainer.NewInvoiceAdjustmentsValueUnknown()
		state.Payers = types.MapUnknown(datasource_billing_explainer.PayersValue{}.Type(ctx))
		state.UpdateTime = types.StringUnknown()
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	invoiceMonth := state.InvoiceMonth.ValueString()
	apiResp, err := ds.client.GetBillingExplainerPerPayerWithResponse(ctx, invoiceMonth, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error reading billing explainer", err.Error())
		return
	}
	if apiResp.StatusCode() == 404 {
		resp.Diagnostics.AddError(
			"Billing Explainer Not Found",
			fmt.Sprintf("No billing explainer found for invoice month %s", invoiceMonth),
		)
		return
	}
	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Billing Explainer",
			fmt.Sprintf("status: %d, body: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	resp.Diagnostics.Append(mapBillingExplainerToModel(ctx, apiResp.JSON200, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
