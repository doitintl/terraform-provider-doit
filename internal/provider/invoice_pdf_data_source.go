package provider

import (
	"context"
	"encoding/base64"
	"fmt"
	"mime"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_invoice_pdf"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*invoicePdfDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*invoicePdfDataSource)(nil)

func NewInvoicePdfDataSource() datasource.DataSource {
	return &invoicePdfDataSource{}
}

type invoicePdfDataSource struct {
	client *models.ClientWithResponses
}

type invoicePdfDataSourceModel struct {
	datasource_invoice_pdf.InvoicePdfModel
	Filename types.String   `tfsdk:"filename"`
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (ds *invoicePdfDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_invoice_pdf"
}

func (ds *invoicePdfDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (ds *invoicePdfDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_invoice_pdf.InvoicePdfDataSourceSchema(ctx)

	s.Attributes["filename"] = schema.StringAttribute{
		Computed:            true,
		Description:         "Suggested filename for the invoice PDF from the Content-Disposition response header (for example, `<id>.pdf`).",
		MarkdownDescription: "Suggested filename for the invoice PDF from the Content-Disposition response header (for example, `<id>.pdf`).",
	}
	s.Attributes["timeouts"] = timeouts.Attributes(ctx)
	s.Description = "Downloads the PDF document for an invoice as a base64-encoded string."
	s.MarkdownDescription = "Downloads the PDF document for an invoice as a base64-encoded string."

	resp.Schema = s
}

func (ds *invoicePdfDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state invoicePdfDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	readTimeout, diags := state.Timeouts.Read(ctx, DefaultReadTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	// If ID is unknown (depends on a resource not yet created), set all computed
	// attributes to unknown so consumers don't treat null as a real value during planning.
	if state.Id.IsUnknown() {
		state.ContentBase64 = types.StringUnknown()
		state.Filename = types.StringUnknown()
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	id := state.Id.ValueString()
	invoicePdfResp, err := ds.client.GetInvoicePdfWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Error reading invoice PDF", err.Error())
		return
	}
	if invoicePdfResp.StatusCode() == 404 {
		resp.Diagnostics.AddError("Invoice PDF not found", fmt.Sprintf("Invoice PDF with ID %s not found", id))
		return
	}
	if invoicePdfResp.StatusCode() != 200 || len(invoicePdfResp.Body) == 0 {
		resp.Diagnostics.AddError(
			"Error reading invoice PDF",
			fmt.Sprintf("Unexpected status: %d, body: %s", invoicePdfResp.StatusCode(), string(invoicePdfResp.Body)),
		)
		return
	}

	state.ContentBase64 = types.StringValue(base64.StdEncoding.EncodeToString(invoicePdfResp.Body))
	state.Filename = parseContentDispositionFilename(invoicePdfResp.Headers200)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func parseContentDispositionFilename(headers *models.GetInvoicePdfResp200Headers) types.String {
	if headers == nil || headers.ContentDisposition == nil || *headers.ContentDisposition == "" {
		return types.StringNull()
	}
	_, params, err := mime.ParseMediaType(*headers.ContentDisposition)
	if err != nil {
		return types.StringNull()
	}
	if filename := params["filename"]; filename != "" {
		return types.StringValue(filename)
	}
	return types.StringNull()
}
