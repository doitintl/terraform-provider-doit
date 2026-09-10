package provider

import (
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	ds "github.com/doitintl/terraform-provider-doit/internal/provider/datasource_support_request_comments"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func readSupportRequestCommentsHelper(t *testing.T, server *httptest.Server, overrides map[string]tftypes.Value) (supportRequestCommentsDataSourceModel, tfsdk.State, diag.Diagnostics) {
	t.Helper()

	httpClient := server.Client()
	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	d := &supportRequestCommentsDataSource{client: client}
	ctx := t.Context()

	schemaResp := &datasource.SchemaResponse{}
	d.Schema(ctx, datasource.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("failed to get schema: %v", schemaResp.Diagnostics)
	}

	configValues := map[string]tftypes.Value{}
	attrTypes := map[string]tftypes.Type{}
	for name, attr := range schemaResp.Schema.Attributes {
		tfType := attr.GetType().TerraformType(ctx)
		attrTypes[name] = tfType
		if override, ok := overrides[name]; ok {
			configValues[name] = override
			continue
		}
		configValues[name] = tftypes.NewValue(tfType, nil)
	}

	configVal := tftypes.NewValue(tftypes.Object{AttributeTypes: attrTypes}, configValues)
	config := tfsdk.Config{
		Schema: schemaResp.Schema,
		Raw:    configVal,
	}

	readResp := &datasource.ReadResponse{
		State: tfsdk.State{Schema: schemaResp.Schema},
	}
	d.Read(ctx, datasource.ReadRequest{Config: config}, readResp)

	var data supportRequestCommentsDataSourceModel
	if !readResp.Diagnostics.HasError() {
		if diags := readResp.State.Get(ctx, &data); diags.HasError() {
			t.Fatalf("failed to read state: %v", diags)
		}
	}

	return data, readResp.State, readResp.Diagnostics
}

func TestSupportRequestCommentsDataSource_Read_PublicMapping(t *testing.T) {
	t.Parallel()

	responseJSON := `{
		"comments": [
			{
				"id": 1001,
				"author": "requester@example.com",
				"body": "Customer public comment",
				"created": 1788912000000,
				"public": true,
				"attachments": [
					{
						"id": 2001,
						"file_name": "screenshot.png",
						"content_url": "https://example.com/screenshot.png"
					}
				]
			},
			{
				"id": 1002,
				"author": "agent@example.com",
				"body": "Internal private note",
				"created": 1788912050000,
				"public": false,
				"attachments": []
			}
		]
	}`

	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/support/v1/tickets/123/comments" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, responseJSON)
	}))

	overrides := map[string]tftypes.Value{
		"ticket_id": tftypes.NewValue(tftypes.Number, new(big.Float).SetInt64(123)),
	}

	data, _, diags := readSupportRequestCommentsHelper(t, server, overrides)
	if diags.HasError() {
		t.Fatalf("Read() returned unexpected diagnostics: %v", diags)
	}

	ctx := t.Context()
	var comments []ds.CommentsValue
	if d := data.Comments.ElementsAs(ctx, &comments, false); d.HasError() {
		t.Fatalf("failed to deserialize comments: %v", d)
	}

	if len(comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(comments))
	}

	// Verify public: true comment
	c1 := comments[0]
	if c1.Id.ValueInt64() != 1001 {
		t.Errorf("comment[0].Id = %d, want 1001", c1.Id.ValueInt64())
	}
	if c1.Author.ValueString() != "requester@example.com" {
		t.Errorf("comment[0].Author = %q, want requester@example.com", c1.Author.ValueString())
	}
	if c1.Body.ValueString() != "Customer public comment" {
		t.Errorf("comment[0].Body = %q, want Customer public comment", c1.Body.ValueString())
	}
	if c1.Created.ValueInt64() != 1788912000000 {
		t.Errorf("comment[0].Created = %d, want 1788912000000", c1.Created.ValueInt64())
	}
	if !c1.Public.ValueBool() {
		t.Errorf("comment[0].Public = %v, want true", c1.Public.ValueBool())
	}
	if c1.Public.IsNull() || c1.Public.IsUnknown() {
		t.Errorf("comment[0].Public must be known and not null")
	}

	var attachments []ds.AttachmentsValue
	if d := c1.Attachments.ElementsAs(ctx, &attachments, false); d.HasError() {
		t.Fatalf("failed to deserialize attachments: %v", d)
	}
	if len(attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(attachments))
	}
	if attachments[0].FileName.ValueString() != "screenshot.png" {
		t.Errorf("attachment[0].FileName = %q, want screenshot.png", attachments[0].FileName.ValueString())
	}

	// Verify public: false comment (internal note)
	c2 := comments[1]
	if c2.Id.ValueInt64() != 1002 {
		t.Errorf("comment[1].Id = %d, want 1002", c2.Id.ValueInt64())
	}
	if c2.Author.ValueString() != "agent@example.com" {
		t.Errorf("comment[1].Author = %q, want agent@example.com", c2.Author.ValueString())
	}
	if c2.Body.ValueString() != "Internal private note" {
		t.Errorf("comment[1].Body = %q, want Internal private note", c2.Body.ValueString())
	}
	if c2.Created.ValueInt64() != 1788912050000 {
		t.Errorf("comment[1].Created = %d, want 1788912050000", c2.Created.ValueInt64())
	}
	if c2.Public.ValueBool() {
		t.Errorf("comment[1].Public = %v, want false", c2.Public.ValueBool())
	}
	if c2.Public.IsNull() || c2.Public.IsUnknown() {
		t.Errorf("comment[1].Public must be known and not null")
	}
}

func TestSupportRequestCommentsDataSource_UnknownTicketId(t *testing.T) {
	t.Parallel()

	var reqCount atomic.Int32
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))

	overrides := map[string]tftypes.Value{
		"ticket_id": tftypes.NewValue(tftypes.Number, tftypes.UnknownValue),
	}

	data, _, diags := readSupportRequestCommentsHelper(t, server, overrides)
	if diags.HasError() {
		t.Fatalf("Read() returned unexpected diagnostics: %v", diags)
	}

	if reqCount.Load() != 0 {
		t.Errorf("expected 0 HTTP requests when ticket_id is unknown, got %d", reqCount.Load())
	}

	if !data.Comments.IsUnknown() {
		t.Errorf("expected Comments to be unknown, got %v", data.Comments)
	}
}

func TestSupportRequestCommentsDataSource_EmptyComments(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		responseJSON string
	}{
		{
			name:         "empty array",
			responseJSON: `{"comments": []}`,
		},
		{
			name:         "null comments",
			responseJSON: `{"comments": null}`,
		},
		{
			name:         "omitted comments",
			responseJSON: `{}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprint(w, tc.responseJSON)
			}))

			overrides := map[string]tftypes.Value{
				"ticket_id": tftypes.NewValue(tftypes.Number, new(big.Float).SetInt64(456)),
			}

			data, _, diags := readSupportRequestCommentsHelper(t, server, overrides)
			if diags.HasError() {
				t.Fatalf("Read() returned unexpected diagnostics: %v", diags)
			}

			if data.Comments.IsNull() {
				t.Fatal("expected Comments to be non-null empty list, got null")
			}
			if data.Comments.IsUnknown() {
				t.Fatal("expected Comments to be known, got unknown")
			}
			if len(data.Comments.Elements()) != 0 {
				t.Fatalf("expected 0 elements in Comments, got %d", len(data.Comments.Elements()))
			}
		})
	}
}

func TestSupportRequestCommentsDataSource_NotFound(t *testing.T) {
	t.Parallel()

	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"error": "Ticket not found"}`)
	}))

	overrides := map[string]tftypes.Value{
		"ticket_id": tftypes.NewValue(tftypes.Number, new(big.Float).SetInt64(999)),
	}

	_, _, diags := readSupportRequestCommentsHelper(t, server, overrides)
	if !diags.HasError() {
		t.Fatal("expected diagnostics error for 404, got none")
	}

	foundSummary := false
	for _, d := range diags.Errors() {
		if d.Summary() == "Support request not found" {
			foundSummary = true
			break
		}
	}
	if !foundSummary {
		t.Errorf("expected error summary 'Support request not found', got: %v", diags)
	}
}

func TestSupportRequestCommentsDataSource_ApiError(t *testing.T) {
	t.Parallel()

	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"error": "Internal Server Error"}`)
	}))

	overrides := map[string]tftypes.Value{
		"ticket_id": tftypes.NewValue(tftypes.Number, new(big.Float).SetInt64(100)),
	}

	_, _, diags := readSupportRequestCommentsHelper(t, server, overrides)
	if !diags.HasError() {
		t.Fatal("expected diagnostics error for 500, got none")
	}

	foundSummary := false
	for _, d := range diags.Errors() {
		if d.Summary() == "Error reading support request comments" {
			foundSummary = true
			break
		}
	}
	if !foundSummary {
		t.Errorf("expected error summary 'Error reading support request comments', got: %v", diags)
	}
}
