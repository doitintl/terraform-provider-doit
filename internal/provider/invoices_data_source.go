package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_invoices"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*invoicesDataSource)(nil)

func NewInvoicesDataSource() datasource.DataSource {
	return &invoicesDataSource{}
}

type invoicesDataSource struct {
	client *models.ClientWithResponses
}

type invoicesDataSourceModel struct {
	datasource_invoices.InvoicesModel
}

func (d *invoicesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_invoices"
}

func (d *invoicesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_invoices.InvoicesDataSourceSchema(ctx)
}

func (d *invoicesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *invoicesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data invoicesDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build query parameters
	params := &models.ListInvoicesParams{}
	if !data.Filter.IsNull() && !data.Filter.IsUnknown() {
		filter := data.Filter.ValueString()
		params.Filter = &filter
	}
	if !data.MinCreationTime.IsNull() && !data.MinCreationTime.IsUnknown() {
		minCreationTime := data.MinCreationTime.ValueInt64()
		params.MinCreationTime = &minCreationTime
	}
	if !data.MaxCreationTime.IsNull() && !data.MaxCreationTime.IsUnknown() {
		maxCreationTime := data.MaxCreationTime.ValueInt64()
		params.MaxCreationTime = &maxCreationTime
	}

	// Smart pagination: honor user-provided values, otherwise auto-paginate
	userControlsPagination := !data.MaxResults.IsNull() && !data.MaxResults.IsUnknown()

	var allInvoices []models.InvoiceListItem

	if userControlsPagination {
		// Manual mode: single API call with user's params
		maxResultsVal := data.MaxResults.ValueInt64()
		params.MaxResults = &maxResultsVal
		if !data.PageToken.IsNull() && !data.PageToken.IsUnknown() {
			pageTokenVal := data.PageToken.ValueString()
			params.PageToken = &pageTokenVal
		}

		apiResp, err := d.client.ListInvoicesWithResponse(ctx, params)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Reading Invoices",
				fmt.Sprintf("Unable to read invoices: %v", err),
			)
			return
		}

		if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
			resp.Diagnostics.AddError(
				"Error Reading Invoices",
				fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
			)
			return
		}

		result := apiResp.JSON200
		if result.Invoices != nil {
			allInvoices = *result.Invoices
		}

		// Preserve API's page_token for user to fetch next page
		data.PageToken = types.StringPointerValue(result.PageToken)
		if result.RowCount != nil {
			data.RowCount = types.Int64Value(*result.RowCount)
		} else {
			data.RowCount = types.Int64Value(int64(len(allInvoices)))
		}
		// max_results is already set by user, no change needed
	} else {
		// Auto mode: fetch all pages
		for {
			apiResp, err := d.client.ListInvoicesWithResponse(ctx, params)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error Reading Invoices",
					fmt.Sprintf("Unable to read invoices: %v", err),
				)
				return
			}

			if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
				resp.Diagnostics.AddError(
					"Error Reading Invoices",
					fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
				)
				return
			}

			result := apiResp.JSON200
			if result.Invoices != nil {
				allInvoices = append(allInvoices, *result.Invoices...)
			}

			if result.PageToken == nil || *result.PageToken == "" {
				break
			}
			params.PageToken = result.PageToken
		}

		// Auto mode: set counts based on what we fetched
		data.RowCount = types.Int64Value(int64(len(allInvoices)))
		data.PageToken = types.StringNull()
		// max_results was not set; preserve null/unknown handling below
		if data.MaxResults.IsUnknown() {
			data.MaxResults = types.Int64Null()
		}
	}

	// Map invoices list
	if len(allInvoices) > 0 {
		invoiceVals := make([]datasource_invoices.InvoicesValue, 0, len(allInvoices))
		for _, inv := range allInvoices {
			// Handle optional Currency enum
			var currencyVal types.String
			if inv.Currency != nil {
				currencyVal = types.StringValue(string(*inv.Currency))
			} else {
				currencyVal = types.StringNull()
			}

			// Handle optional Platform enum
			var platformVal types.String
			if inv.Platform != nil {
				platformVal = types.StringValue(string(*inv.Platform))
			} else {
				platformVal = types.StringNull()
			}

			// Handle optional Status enum
			var statusVal types.String
			if inv.Status != nil {
				statusVal = types.StringValue(string(*inv.Status))
			} else {
				statusVal = types.StringNull()
			}

			invVal, diags := datasource_invoices.NewInvoicesValue(
				datasource_invoices.InvoicesValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"id":             types.StringPointerValue(inv.Id),
					"balance_amount": types.Float64PointerValue(inv.BalanceAmount),
					"total_amount":   types.Float64PointerValue(inv.TotalAmount),
					"currency":       currencyVal,
					"due_date":       types.Int64PointerValue(inv.DueDate),
					"invoice_date":   types.Int64PointerValue(inv.InvoiceDate),
					"platform":       platformVal,
					"status":         statusVal,
					"url":            types.StringPointerValue(inv.Url),
				},
			)
			resp.Diagnostics.Append(diags...)
			invoiceVals = append(invoiceVals, invVal)
		}

		invoiceList, diags := types.ListValueFrom(ctx, datasource_invoices.InvoicesValue{}.Type(ctx), invoiceVals)
		resp.Diagnostics.Append(diags...)
		data.Invoices = invoiceList
	} else {
		data.Invoices = types.ListNull(datasource_invoices.InvoicesValue{}.Type(ctx))
	}

	// Set optional filter params to null if they were unknown
	if data.Filter.IsUnknown() {
		data.Filter = types.StringNull()
	}
	if data.MinCreationTime.IsUnknown() {
		data.MinCreationTime = types.Int64Null()
	}
	if data.MaxCreationTime.IsUnknown() {
		data.MaxCreationTime = types.Int64Null()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
