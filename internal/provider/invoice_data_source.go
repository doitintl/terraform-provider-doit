package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_invoice"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*invoiceDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*invoiceDataSource)(nil)

func NewInvoiceDataSource() datasource.DataSource {
	return &invoiceDataSource{}
}

type invoiceDataSource struct {
	client *models.ClientWithResponses
}

func (ds *invoiceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_invoice"
}

func (ds *invoiceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (ds *invoiceDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_invoice.InvoiceDataSourceSchema(ctx)
}

func (ds *invoiceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state datasource_invoice.InvoiceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.Id.ValueString()
	invoiceResp, err := ds.client.GetInvoiceWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Error reading invoice", err.Error())
		return
	}
	if invoiceResp.StatusCode() == 404 {
		resp.Diagnostics.AddError("Invoice not found", fmt.Sprintf("Invoice with ID %s not found", id))
		return
	}
	if invoiceResp.StatusCode() != 200 || invoiceResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error reading invoice",
			fmt.Sprintf("Unexpected status: %d, body: %s", invoiceResp.StatusCode(), string(invoiceResp.Body)),
		)
		return
	}

	invoice := invoiceResp.JSON200

	// Map fields from the anonymous struct response
	state.Id = types.StringPointerValue(invoice.Id)
	state.BalanceAmount = types.Float64PointerValue(invoice.BalanceAmount)
	state.DueDate = types.Int64PointerValue(invoice.DueDate)
	state.InvoiceDate = types.Int64PointerValue(invoice.InvoiceDate)
	state.TotalAmount = types.Float64PointerValue(invoice.TotalAmount)
	state.Url = types.StringPointerValue(invoice.Url)

	if invoice.Currency != nil {
		state.Currency = types.StringValue(string(*invoice.Currency))
	} else {
		state.Currency = types.StringNull()
	}

	if invoice.Platform != nil {
		state.Platform = types.StringValue(string(*invoice.Platform))
	} else {
		state.Platform = types.StringNull()
	}

	if invoice.Status != nil {
		state.Status = types.StringValue(string(*invoice.Status))
	} else {
		state.Status = types.StringNull()
	}

	// Map line_items
	if invoice.LineItems != nil && len(*invoice.LineItems) > 0 {
		lineItemVals := make([]datasource_invoice.LineItemsValue, 0, len(*invoice.LineItems))
		for _, li := range *invoice.LineItems {
			var liType types.String
			if li.Type != nil {
				liType = types.StringValue(*li.Type)
			} else {
				liType = types.StringNull()
			}

			liVal, d := datasource_invoice.NewLineItemsValue(
				datasource_invoice.LineItemsValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"currency":    types.StringPointerValue(li.Currency),
					"description": types.StringPointerValue(li.Description),
					"details":     types.StringPointerValue(li.Details),
					"price":       types.Float64PointerValue(li.Price),
					"qty":         types.Float64PointerValue(li.Qty),
					"type":        liType,
				},
			)
			resp.Diagnostics.Append(d...)
			lineItemVals = append(lineItemVals, liVal)
		}
		lineItemsList, d := types.ListValueFrom(ctx, datasource_invoice.LineItemsValue{}.Type(ctx), lineItemVals)
		resp.Diagnostics.Append(d...)
		state.LineItems = lineItemsList
	} else {
		emptyList, d := types.ListValueFrom(ctx, datasource_invoice.LineItemsValue{}.Type(ctx), []datasource_invoice.LineItemsValue{})
		resp.Diagnostics.Append(d...)
		state.LineItems = emptyList
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
