package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_alert_slack_channels"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*alertSlackChannelsDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*alertSlackChannelsDataSource)(nil)

func NewAlertSlackChannelsDataSource() datasource.DataSource {
	return &alertSlackChannelsDataSource{}
}

type alertSlackChannelsDataSource struct {
	client *models.ClientWithResponses
}

type alertSlackChannelsDataSourceModel struct {
	datasource_alert_slack_channels.AlertSlackChannelsModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *alertSlackChannelsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_alert_slack_channels"
}

func (d *alertSlackChannelsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_alert_slack_channels.AlertSlackChannelsDataSourceSchema(ctx)
	s.Description = "Retrieves Slack channels eligible for Alert notifications."
	s.MarkdownDescription = "Retrieves Slack channels eligible for Alert notifications."
	s.Attributes["timeouts"] = timeouts.Attributes(ctx)

	resp.Schema = s
}

func (d *alertSlackChannelsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*models.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *models.ClientWithResponses, got: %T.", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *alertSlackChannelsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data alertSlackChannelsDataSourceModel

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

	// If any filter/pagination input is unknown, return unknown list and computed attributes
	if data.NameContains.IsUnknown() || data.MaxResults.IsUnknown() || data.PageToken.IsUnknown() {
		data.Items = types.ListUnknown(datasource_alert_slack_channels.ItemsValue{}.Type(ctx))
		data.RowCount = types.Int64Unknown()
		data.WorkspaceStatus = types.StringUnknown()
		data.IsWorkspaceConnected = types.BoolUnknown()
		data.HasSharedChannel = types.BoolUnknown()
		data.PageToken = types.StringUnknown()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	// Build query parameters
	params := &models.ListAlertSlackChannelsParams{}
	if !data.NameContains.IsNull() {
		params.NameContains = new(data.NameContains.ValueString())
	}

	// Smart pagination: honor user-provided values, otherwise auto-paginate
	userControlsPagination := !data.MaxResults.IsNull()

	var allChannels []models.AlertSlackChannel
	var workspaceStatus string
	var isWorkspaceConnected bool
	var hasSharedChannel bool

	if userControlsPagination {
		// Manual mode: single API call with user's params
		params.MaxResults = new(int(data.MaxResults.ValueInt64()))
		if !data.PageToken.IsNull() {
			params.PageToken = new(data.PageToken.ValueString())
		}

		apiResp, err := d.client.ListAlertSlackChannelsWithResponse(ctx, params)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Reading Alert Slack Channels",
				fmt.Sprintf("Unable to read alert slack channels: %v", err),
			)
			return
		}

		if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
			resp.Diagnostics.AddError(
				"Error Reading Alert Slack Channels",
				fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
			)
			return
		}

		result := apiResp.JSON200
		allChannels = result.Items
		workspaceStatus = string(result.WorkspaceStatus)
		isWorkspaceConnected = result.IsWorkspaceConnected
		hasSharedChannel = result.HasSharedChannel

		// Preserve API's page_token for user to fetch next page
		data.PageToken = types.StringPointerValue(result.PageToken)
		data.RowCount = types.Int64Value(int64(result.RowCount))
	} else {
		// Auto mode: fetch all pages, honoring user-provided page_token as starting point
		if !data.PageToken.IsNull() {
			params.PageToken = new(data.PageToken.ValueString())
		}
		for {
			apiResp, err := d.client.ListAlertSlackChannelsWithResponse(ctx, params)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error Reading Alert Slack Channels",
					fmt.Sprintf("Unable to read alert slack channels: %v", err),
				)
				return
			}

			if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
				resp.Diagnostics.AddError(
					"Error Reading Alert Slack Channels",
					fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
				)
				return
			}

			result := apiResp.JSON200
			allChannels = append(allChannels, result.Items...)
			workspaceStatus = string(result.WorkspaceStatus)
			isWorkspaceConnected = result.IsWorkspaceConnected
			hasSharedChannel = result.HasSharedChannel

			if result.PageToken == nil || *result.PageToken == "" {
				break
			}
			params.PageToken = result.PageToken
		}

		// Auto mode: set counts based on what we fetched
		data.RowCount = types.Int64Value(int64(len(allChannels)))
		data.PageToken = types.StringNull()
		data.MaxResults = types.Int64Null()
	}

	data.WorkspaceStatus = types.StringValue(workspaceStatus)
	data.IsWorkspaceConnected = types.BoolValue(isWorkspaceConnected)
	data.HasSharedChannel = types.BoolValue(hasSharedChannel)

	// Map items list
	if len(allChannels) > 0 {
		channelVals := make([]datasource_alert_slack_channels.ItemsValue, 0, len(allChannels))
		for _, ch := range allChannels {
			var typeVal types.String
			if ch.Type != nil {
				typeVal = types.StringValue(string(*ch.Type))
			} else {
				typeVal = types.StringNull()
			}

			chVal, diags := datasource_alert_slack_channels.NewItemsValue(
				datasource_alert_slack_channels.ItemsValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"customer_id": types.StringPointerValue(ch.CustomerId),
					"id":          types.StringValue(ch.Id),
					"name":        types.StringPointerValue(ch.Name),
					"shared":      normalizeSlackChannelShared(ch.Shared),
					"type":        typeVal,
					"workspace":   normalizeSlackChannelWorkspace(ch.Workspace),
				},
			)
			resp.Diagnostics.Append(diags...)
			channelVals = append(channelVals, chVal)
		}
		channelsList, diags := types.ListValueFrom(ctx, datasource_alert_slack_channels.ItemsValue{}.Type(ctx), channelVals)
		resp.Diagnostics.Append(diags...)
		data.Items = channelsList
	} else {
		emptyList, diags := types.ListValueFrom(ctx, datasource_alert_slack_channels.ItemsValue{}.Type(ctx), []datasource_alert_slack_channels.ItemsValue{})
		resp.Diagnostics.Append(diags...)
		data.Items = emptyList
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
