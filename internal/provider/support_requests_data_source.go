package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_support_requests"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*supportRequestsDataSource)(nil)

func NewSupportRequestsDataSource() datasource.DataSource {
	return &supportRequestsDataSource{}
}

type supportRequestsDataSource struct {
	client *models.ClientWithResponses
}

type supportRequestsDataSourceModel struct {
	datasource_support_requests.SupportRequestsModel
}

func (d *supportRequestsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_support_requests"
}

func (d *supportRequestsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_support_requests.SupportRequestsDataSourceSchema(ctx)
}

func (d *supportRequestsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *supportRequestsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data supportRequestsDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If any filter/pagination input is unknown, return unknown list
	if data.Filter.IsUnknown() || data.MinCreationTime.IsUnknown() || data.MaxCreationTime.IsUnknown() || data.MaxResults.IsUnknown() || data.PageToken.IsUnknown() {
		data.Tickets = types.ListUnknown(datasource_support_requests.TicketsValue{}.Type(ctx))
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}
	// Build query parameters
	params := &models.IdOfTicketsParams{}
	if !data.Filter.IsNull() {
		filter := data.Filter.ValueString()
		params.Filter = &filter
	}
	if !data.MinCreationTime.IsNull() {
		minCreationTime := data.MinCreationTime.ValueString()
		params.MinCreationTime = &minCreationTime
	}
	if !data.MaxCreationTime.IsNull() {
		maxCreationTime := data.MaxCreationTime.ValueString()
		params.MaxCreationTime = &maxCreationTime
	}

	// Smart pagination: honor user-provided values, otherwise auto-paginate
	userControlsPagination := !data.MaxResults.IsNull()

	var allTickets []models.TicketListItem

	if userControlsPagination {
		// Manual mode: single API call with user's params
		maxResultsVal := data.MaxResults.ValueInt64()
		params.MaxResults = &maxResultsVal
		if !data.PageToken.IsNull() {
			pageTokenVal := data.PageToken.ValueString()
			params.PageToken = &pageTokenVal
		}

		apiResp, err := d.client.IdOfTicketsWithResponse(ctx, params)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Reading Support Requests",
				fmt.Sprintf("Unable to read support requests: %v", err),
			)
			return
		}

		if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
			resp.Diagnostics.AddError(
				"Error Reading Support Requests",
				fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
			)
			return
		}

		result := apiResp.JSON200
		if result.Tickets != nil {
			allTickets = *result.Tickets
		}

		// Preserve API's page_token for user to fetch next page
		data.PageToken = types.StringPointerValue(result.PageToken)
		if result.RowCount != nil {
			data.RowCount = types.Int64Value(*result.RowCount)
		} else {
			data.RowCount = types.Int64Value(int64(len(allTickets)))
		}
		// max_results is already set by user, no change needed
	} else {
		// Auto mode: fetch all pages, honoring user-provided page_token as starting point
		if !data.PageToken.IsNull() {
			pageTokenVal := data.PageToken.ValueString()
			params.PageToken = &pageTokenVal
		}
		for {
			apiResp, err := d.client.IdOfTicketsWithResponse(ctx, params)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error Reading Support Requests",
					fmt.Sprintf("Unable to read support requests: %v", err),
				)
				return
			}

			if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
				resp.Diagnostics.AddError(
					"Error Reading Support Requests",
					fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
				)
				return
			}

			result := apiResp.JSON200
			if result.Tickets != nil {
				allTickets = append(allTickets, *result.Tickets...)
			}

			if result.PageToken == nil || *result.PageToken == "" {
				break
			}
			params.PageToken = result.PageToken
		}

		// Auto mode: set counts based on what we fetched
		data.RowCount = types.Int64Value(int64(len(allTickets)))
		data.PageToken = types.StringNull()
		// max_results was not set; preserve null
	}

	// Map tickets list
	if len(allTickets) > 0 {
		ticketVals := make([]datasource_support_requests.TicketsValue, 0, len(allTickets))
		for _, ticket := range allTickets {
			// Handle optional Platform enum
			var platformVal types.String
			if ticket.Platform != nil {
				platformVal = types.StringValue(string(*ticket.Platform))
			} else {
				platformVal = types.StringNull()
			}

			ticketVal, diags := datasource_support_requests.NewTicketsValue(
				datasource_support_requests.TicketsValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"id":          types.Int64PointerValue(ticket.Id),
					"create_time": types.Int64PointerValue(ticket.CreateTime),
					"update_time": types.Int64PointerValue(ticket.UpdateTime),
					"is_public":   types.BoolPointerValue(ticket.IsPublic),
					"platform":    platformVal,
					"product":     types.StringPointerValue(ticket.Product),
					"requester":   types.StringPointerValue(ticket.Requester),
					"severity":    types.StringPointerValue(ticket.Severity),
					"status":      types.StringPointerValue(ticket.Status),
					"subject":     types.StringPointerValue(ticket.Subject),
					"url_ui":      types.StringPointerValue(ticket.UrlUI),
				},
			)
			resp.Diagnostics.Append(diags...)
			ticketVals = append(ticketVals, ticketVal)
		}

		ticketList, diags := types.ListValueFrom(ctx, datasource_support_requests.TicketsValue{}.Type(ctx), ticketVals)
		resp.Diagnostics.Append(diags...)
		data.Tickets = ticketList
	} else {
		emptyList, diags := types.ListValueFrom(ctx, datasource_support_requests.TicketsValue{}.Type(ctx), []datasource_support_requests.TicketsValue{})
		resp.Diagnostics.Append(diags...)
		data.Tickets = emptyList
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
