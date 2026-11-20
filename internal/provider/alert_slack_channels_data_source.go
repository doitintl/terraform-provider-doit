package provider

import (
	"context"
	"fmt"
	"net/http"
	"strings"

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
	const description = "Retrieves Slack channels eligible for Alert notifications. " +
		"When `max_results` is omitted, the provider fetches all pages using the API's maximum page size of 500; " +
		"set `max_results` to control the page size yourself and fetch a single page. " +
		"Channels are returned in workspace order (the shared channel, then public, then private channels), not sorted by name or ID. " +
		"With `max_results` a page can hold fewer channels than requested, even none, while `page_token` is set " +
		"because `name_contains` filtered out most of the scanned channels; keep following `page_token` until it is null. " +
		"`row_count` is then null as well, because the total is only known once every channel has been listed."
	s.Description = description
	s.MarkdownDescription = description
	resp.Diagnostics.Append(setListPaginationDescriptions(ctx, &s, listRowCountTotalUnlessPaged, "")...)
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

// invalidCursorMarker is how the retry client reports a 400: it surfaces every
// non-retryable 4xx as an error whose text starts with the status code.
const invalidCursorMarker = "non-retryable error: 400"

// slackChannelsRestartHint explains a 400 on a request that carried a page token:
// Slack no longer accepts the cursor, and the API resolves that by restarting from
// the first page. It returns "" when the hint does not apply.
func slackChannelsRestartHint(badRequest, hadPageToken bool) string {
	if !badRequest || !hadPageToken {
		return ""
	}
	return "\nThe page_token may be invalid or expired; read again without page_token to restart from the first page."
}

// slackChannelsRequestError describes a failed request. The retry client returns a
// 400 as an error, a plain client as a response, so Read passes whichever it got.
func slackChannelsRequestError(err error, hadPageToken bool) string {
	return fmt.Sprintf("Unable to read alert slack channels: %v", err) +
		slackChannelsRestartHint(strings.Contains(err.Error(), invalidCursorMarker), hadPageToken)
}

func slackChannelsStatusError(status int, body []byte, hadPageToken bool) string {
	return fmt.Sprintf("API returned status %d: %s", status, string(body)) +
		slackChannelsRestartHint(status == http.StatusBadRequest, hadPageToken) //nolint:retrystatus // plain-client fallback: the retry client reports a 400 as an error (see slackChannelsRequestError)
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

	// The results are not yet known. page_token is one of them in every mode:
	// a read replaces it with the next page's token, or null. The inputs stay
	// as configured.
	if !req.Config.Raw.IsFullyKnown() {
		data.Items = types.ListUnknown(datasource_alert_slack_channels.ItemsValue{}.Type(ctx))
		data.RowCount = types.Int64Unknown()
		data.WorkspaceStatus = types.StringUnknown()
		data.IsWorkspaceConnected = types.BoolUnknown()
		data.HasSharedChannel = types.BoolUnknown()
		data.ChannelsComplete = types.BoolUnknown()
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
	channelsComplete := true

	if userControlsPagination {
		// Manual mode: single API call with user's params
		params.MaxResults = new(int(data.MaxResults.ValueInt64()))
		if !data.PageToken.IsNull() {
			params.PageToken = new(data.PageToken.ValueString())
		}

		apiResp, err := d.client.ListAlertSlackChannelsWithResponse(ctx, params)
		if err != nil {
			resp.Diagnostics.AddError("Error Reading Alert Slack Channels", slackChannelsRequestError(err, params.PageToken != nil))
			return
		}

		if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
			resp.Diagnostics.AddError("Error Reading Alert Slack Channels", slackChannelsStatusError(apiResp.StatusCode(), apiResp.Body, params.PageToken != nil))
			return
		}

		result := apiResp.JSON200
		allChannels = result.Items
		workspaceStatus = string(result.WorkspaceStatus)
		isWorkspaceConnected = result.IsWorkspaceConnected
		hasSharedChannel = result.HasSharedChannel
		channelsComplete = result.ChannelsComplete

		// Preserve API's page_token for user to fetch next page
		data.PageToken = types.StringPointerValue(result.PageToken)
		// The API returns rowCount only when this single request saw the whole list;
		// otherwise the total is unknown, so row_count stays null instead of
		// reporting the size of the page as a total.
		data.RowCount = types.Int64Null()
		if result.RowCount != nil {
			data.RowCount = types.Int64Value(int64(*result.RowCount))
		}
	} else {
		// Auto mode: fetch all pages, honoring user-provided page_token as starting point
		if !data.PageToken.IsNull() {
			params.PageToken = new(data.PageToken.ValueString())
		}
		// Use the API's maximum page size to keep the number of requests low.
		params.MaxResults = new(500)
		for {
			apiResp, err := d.client.ListAlertSlackChannelsWithResponse(ctx, params)
			if err != nil {
				resp.Diagnostics.AddError("Error Reading Alert Slack Channels", slackChannelsRequestError(err, params.PageToken != nil))
				return
			}

			if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
				resp.Diagnostics.AddError("Error Reading Alert Slack Channels", slackChannelsStatusError(apiResp.StatusCode(), apiResp.Body, params.PageToken != nil))
				return
			}

			result := apiResp.JSON200
			allChannels = append(allChannels, result.Items...)
			workspaceStatus = string(result.WorkspaceStatus)
			isWorkspaceConnected = result.IsWorkspaceConnected
			hasSharedChannel = result.HasSharedChannel
			// A single partial page makes the whole listing non-authoritative.
			channelsComplete = channelsComplete && result.ChannelsComplete

			if result.PageToken == nil || *result.PageToken == "" {
				break
			}
			params.PageToken = result.PageToken
		}

		// Auto mode: the API omits rowCount on follow-up pages, so count what we fetched
		data.RowCount = types.Int64Value(int64(len(allChannels)))
		data.PageToken = types.StringNull()
		data.MaxResults = types.Int64Null()
	}

	data.WorkspaceStatus = types.StringValue(workspaceStatus)
	data.IsWorkspaceConnected = types.BoolValue(isWorkspaceConnected)
	data.HasSharedChannel = types.BoolValue(hasSharedChannel)
	data.ChannelsComplete = types.BoolValue(channelsComplete)
	if !channelsComplete {
		resp.Diagnostics.AddWarning(
			"Incomplete Slack Channel List",
			"The API reported that the Slack channel list may be incomplete (channels_complete = false). "+
				"Causes are a Slack rate limit or transient failure (re-read later), a missing private-channel scope "+
				"(see workspace_status), or a service-account caller, for which private channels are never listed. "+
				"Channels may be missing from the result; do not treat it as authoritative.",
		)
	}

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
