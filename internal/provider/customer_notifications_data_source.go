package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_customer_notifications"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*customerNotificationsDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*customerNotificationsDataSource)(nil)

func NewCustomerNotificationsDataSource() datasource.DataSource {
	return &customerNotificationsDataSource{}
}

type customerNotificationsDataSource struct{ client *models.ClientWithResponses }

type customerNotificationsDataSourceModel struct {
	datasource_customer_notifications.CustomerNotificationsModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *customerNotificationsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_customer_notifications"
}

func (d *customerNotificationsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *customerNotificationsDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_customer_notifications.CustomerNotificationsDataSourceSchema(ctx)

	s.Description = "Organization-wide cost-anomaly notification configs, delivered by email, Slack and Microsoft Teams. Requires the Manage settings permission."
	s.MarkdownDescription = s.Description

	s.Attributes["timeouts"] = timeouts.Attributes(ctx)
	resp.Schema = s
}

func (d *customerNotificationsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data customerNotificationsDataSourceModel
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

	// There are no inputs besides the timeouts; this guards direct protocol
	// clients that pass an unknown configuration.
	if !req.Config.Raw.IsFullyKnown() {
		data.Items = types.ListUnknown(datasource_customer_notifications.ItemsValue{}.Type(ctx))
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	// The endpoint is not paginated: it returns every notification of the tenant.
	apiResp, err := d.client.ListCustomerNotificationsWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Customer Notifications", fmt.Sprintf("Could not list customer notifications: %v", err))
		return
	}
	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error Reading Customer Notifications", fmt.Sprintf("status %d, body: %s", apiResp.StatusCode(), string(apiResp.Body)))
		return
	}

	resp.Diagnostics.Append(mapCustomerNotificationsToDataSource(ctx, apiResp.JSON200.Items, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
