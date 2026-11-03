package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_user_notification"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*userNotificationDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*userNotificationDataSource)(nil)

func NewUserNotificationDataSource() datasource.DataSource {
	return &userNotificationDataSource{}
}

type userNotificationDataSource struct{ client *models.ClientWithResponses }

type userNotificationDataSourceModel struct {
	datasource_user_notification.UserNotificationModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *userNotificationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_notification"
}

func (d *userNotificationDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *userNotificationDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_user_notification.UserNotificationDataSourceSchema(ctx)
	s.Attributes["timeouts"] = timeouts.Attributes(ctx)
	resp.Schema = s
}

func (d *userNotificationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data userNotificationDataSourceModel
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

	if data.UserId.IsUnknown() || data.Id.IsUnknown() {
		data.Name = types.StringUnknown()
		data.CreateTime = types.StringUnknown()
		data.UpdateTime = types.StringUnknown()
		data.Etag = types.StringUnknown()
		data.Destinations = types.ListUnknown(datasource_user_notification.DestinationsValue{}.Type(ctx))
		data.Settings = datasource_user_notification.NewSettingsValueUnknown()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	userID := data.UserId.ValueString()
	id := data.Id.ValueString()

	apiResp, err := d.client.GetUserNotificationWithResponse(ctx, userID, id)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading User Notification", fmt.Sprintf("Could not read notification %s of user %s: %v", id, userID, err))
		return
	}
	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error Reading User Notification", fmt.Sprintf("Notification %s of user %s: status %d, body: %s", id, userID, apiResp.StatusCode(), string(apiResp.Body)))
		return
	}

	if got := nullableToPointer(apiResp.JSON200.Id); got == nil || *got != id {
		returned := ""
		if got != nil {
			returned = *got
		}
		resp.Diagnostics.AddError("Invalid User Notification Response", fmt.Sprintf("Requested notification %s but API returned ID %q", id, returned))
		return
	}

	resp.Diagnostics.Append(mapUserNotificationToDataSource(ctx, apiResp.JSON200, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
