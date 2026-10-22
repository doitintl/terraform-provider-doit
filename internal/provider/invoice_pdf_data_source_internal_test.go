package provider

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_invoice_pdf"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func newInvoicePdfReadRequest(t *testing.T, id tftypes.Value) (datasource.ReadRequest, datasource.ReadResponse) {
	t.Helper()
	ctx := t.Context()

	ds := &invoicePdfDataSource{}
	var schemaResp datasource.SchemaResponse
	ds.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)

	schemaType := schemaResp.Schema.Type().TerraformType(ctx)
	rawVal := tftypes.NewValue(schemaType, map[string]tftypes.Value{
		"id":             id,
		"content_base64": tftypes.NewValue(tftypes.String, nil),
		"timeouts": tftypes.NewValue(
			timeouts.Attributes(ctx).GetType().TerraformType(ctx),
			nil,
		),
	})

	req := datasource.ReadRequest{
		Config: tfsdk.Config{
			Schema: schemaResp.Schema,
			Raw:    rawVal,
		},
	}
	resp := datasource.ReadResponse{
		State: tfsdk.State{
			Schema: schemaResp.Schema,
			Raw:    rawVal,
		},
	}
	return req, resp
}

func TestInvoicePdfDataSource_Read_Success(t *testing.T) {
	t.Parallel()

	pdfBytes := []byte("%PDF-1.4\n%mock invoice pdf content\n%%EOF\n")
	expectedBase64 := base64.StdEncoding.EncodeToString(pdfBytes)

	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/billing/v1/invoices/INV-12345/pdf" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(pdfBytes)
	}))
	httpClient := server.Client()
	client, err := newClientWithHTTPClient(server.URL, "test-token", "", "test", "dev", httpClient)
	if err != nil {
		t.Fatalf("newClientWithHTTPClient() error = %v", err)
	}

	ds := &invoicePdfDataSource{client: client}
	req, resp := newInvoicePdfReadRequest(t, tftypes.NewValue(tftypes.String, "INV-12345"))
	ds.Read(t.Context(), req, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	var state struct {
		datasource_invoice_pdf.InvoicePdfModel
		Timeouts timeouts.Value `tfsdk:"timeouts"`
	}
	if diags := resp.State.Get(t.Context(), &state); diags.HasError() {
		t.Fatalf("State.Get() error: %v", diags)
	}
	if state.Id != types.StringValue("INV-12345") {
		t.Errorf("state.Id = %v, want INV-12345", state.Id)
	}
	if state.ContentBase64 != types.StringValue(expectedBase64) {
		t.Errorf("state.ContentBase64 = %v, want %q", state.ContentBase64, expectedBase64)
	}
}

func TestInvoicePdfDataSource_Read_NotFound(t *testing.T) {
	t.Parallel()

	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	httpClient := server.Client()
	client, err := newClientWithHTTPClient(server.URL, "test-token", "", "test", "dev", httpClient)
	if err != nil {
		t.Fatalf("newClientWithHTTPClient() error = %v", err)
	}

	ds := &invoicePdfDataSource{client: client}
	req, resp := newInvoicePdfReadRequest(t, tftypes.NewValue(tftypes.String, "INV-MISSING"))
	ds.Read(t.Context(), req, &resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic on 404 response, got none")
	}
	detail := resp.Diagnostics.Errors()[0].Detail()
	if !strings.Contains(detail, "Invoice PDF with ID INV-MISSING not found") {
		t.Errorf("unexpected error detail: %q", detail)
	}
}
