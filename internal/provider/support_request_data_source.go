package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_support_request"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*supportRequestDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*supportRequestDataSource)(nil)

func NewSupportRequestDataSource() datasource.DataSource {
	return &supportRequestDataSource{}
}

type supportRequestDataSource struct {
	client *models.ClientWithResponses
}

func (ds *supportRequestDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_support_request"
}

func (ds *supportRequestDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (ds *supportRequestDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_support_request.SupportRequestDataSourceSchema(ctx)
}

func (ds *supportRequestDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state datasource_support_request.SupportRequestModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If ticket_id is unknown (depends on a resource not yet created), return early
	if state.TicketId.IsUnknown() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	ticketId := state.TicketId.ValueInt64()
	ticketResp, err := ds.client.IdOfTicketGetWithResponse(ctx, ticketId)
	if err != nil {
		resp.Diagnostics.AddError("Error reading support request", err.Error())
		return
	}
	if ticketResp.StatusCode() == 404 {
		resp.Diagnostics.AddError("Support request not found", fmt.Sprintf("Support request with ID %d not found", ticketId))
		return
	}
	if ticketResp.StatusCode() != 200 || ticketResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error reading support request",
			fmt.Sprintf("Unexpected status: %d, body: %s", ticketResp.StatusCode(), string(ticketResp.Body)),
		)
		return
	}

	ticket := ticketResp.JSON200

	// Map API response to state model
	state.Id = types.Int64PointerValue(ticket.Id)
	state.CreateTime = types.Int64PointerValue(ticket.CreateTime)
	state.Description = types.StringPointerValue(ticket.Description)
	state.IsPublic = types.BoolPointerValue(ticket.IsPublic)
	state.Product = types.StringPointerValue(ticket.Product)
	state.Requester = types.StringPointerValue(ticket.Requester)
	state.Status = types.StringPointerValue(ticket.Status)
	state.Subject = types.StringPointerValue(ticket.Subject)
	state.UpdateTime = types.Int64PointerValue(ticket.UpdateTime)
	state.UrlUi = types.StringPointerValue(ticket.UrlUI)

	// Handle enum types — convert to string
	if ticket.Platform != nil {
		state.Platform = types.StringValue(string(*ticket.Platform))
	} else {
		state.Platform = types.StringNull()
	}

	if ticket.Severity != nil {
		state.Severity = types.StringValue(string(*ticket.Severity))
	} else {
		state.Severity = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
