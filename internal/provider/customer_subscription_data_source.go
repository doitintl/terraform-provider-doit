package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_customer_subscription"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*customerSubscriptionDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*customerSubscriptionDataSource)(nil)

func NewCustomerSubscriptionDataSource() datasource.DataSource {
	return &customerSubscriptionDataSource{}
}

type customerSubscriptionDataSource struct{ client *models.ClientWithResponses }

type customerSubscriptionDataSourceModel struct {
	datasource_customer_subscription.CustomerSubscriptionModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *customerSubscriptionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_customer_subscription"
}

func (d *customerSubscriptionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *customerSubscriptionDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_customer_subscription.CustomerSubscriptionDataSourceSchema(ctx)

	s.Description = "An organization-wide subscription: scheduled email or Slack delivery of a dashboard, a report or a commitment to one set of recipients. The ID of an alert or a budget is also accepted; its entry has type `alert` or `budget` and no schedule."
	s.MarkdownDescription = s.Description

	s.Attributes["timeouts"] = timeouts.Attributes(ctx)
	resp.Schema = s
}

func (d *customerSubscriptionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data customerSubscriptionDataSourceModel
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

	if data.Id.IsUnknown() {
		data.Name = types.StringUnknown()
		data.Etag = types.StringUnknown()
		data.CreatedBy = types.StringUnknown()
		data.Recipients = datasource_customer_subscription.NewRecipientsValueUnknown()
		data.Subscriptions = types.SetUnknown(datasource_customer_subscription.SubscriptionsValue{}.Type(ctx))
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	id := data.Id.ValueString()

	apiResp, err := d.client.GetCustomerSubscriptionWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Customer Subscription", fmt.Sprintf("Could not read subscription %s: %v", id, err))
		return
	}
	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error Reading Customer Subscription", fmt.Sprintf("Subscription %s: status %d, body: %s", id, apiResp.StatusCode(), string(apiResp.Body)))
		return
	}

	if got := nullableToPointer(apiResp.JSON200.Id); got == nil || *got != id {
		returned := ""
		if got != nil {
			returned = *got
		}
		resp.Diagnostics.AddError("Invalid Customer Subscription Response", fmt.Sprintf("Requested subscription %s but API returned ID %q", id, returned))
		return
	}

	resp.Diagnostics.Append(mapCustomerSubscriptionToDataSource(ctx, apiResp.JSON200, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
