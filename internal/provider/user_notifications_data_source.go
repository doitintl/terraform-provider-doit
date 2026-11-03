package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_user_notifications"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*userNotificationsDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*userNotificationsDataSource)(nil)

func NewUserNotificationsDataSource() datasource.DataSource {
	return &userNotificationsDataSource{}
}

type userNotificationsDataSource struct{ client *models.ClientWithResponses }

type userNotificationsDataSourceModel struct {
	datasource_user_notifications.UserNotificationsModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *userNotificationsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_notifications"
}

func (d *userNotificationsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *userNotificationsDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_user_notifications.UserNotificationsDataSourceSchema(ctx)
	s.Attributes["timeouts"] = timeouts.Attributes(ctx)
	resp.Schema = s
}

func (d *userNotificationsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data userNotificationsDataSourceModel
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

	if data.UserId.IsUnknown() {
		data.Items = types.ListUnknown(datasource_user_notifications.ItemsValue{}.Type(ctx))
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	userID := data.UserId.ValueString()

	// The endpoint is not paginated: it returns every notification of the user.
	apiResp, err := d.client.ListUserNotificationsWithResponse(ctx, userID)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading User Notifications", fmt.Sprintf("Could not list notifications of user %s: %v", userID, err))
		return
	}
	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error Reading User Notifications", fmt.Sprintf("User %s: status %d, body: %s", userID, apiResp.StatusCode(), string(apiResp.Body)))
		return
	}

	resp.Diagnostics.Append(mapUserNotificationsToDataSource(ctx, apiResp.JSON200.Items, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
