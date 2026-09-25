package provider

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_widgets"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestWidgetsDataSourceSchema_ValidateImplementation(t *testing.T) {
	ctx := t.Context()
	ds := NewWidgetsDataSource()

	var schemaResp datasource.SchemaResponse
	ds.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
	if diags := schemaResp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("Widgets schema validation failed: %v", diags)
	}
}

func TestMapWidgetsToModel_EmptyReturnsNonNullList(t *testing.T) {
	ctx := t.Context()

	for _, name := range []string{"nil slice", "empty slice"} {
		t.Run(name, func(t *testing.T) {
			var items []models.WidgetItem
			if name == "empty slice" {
				items = []models.WidgetItem{}
			}

			var data widgetsDataSourceModel
			diags := mapWidgetsToModel(ctx, items, &data)
			if diags.HasError() {
				t.Fatalf("mapWidgetsToModel returned errors: %v", diags)
			}

			if data.Items.IsNull() {
				t.Fatal("expected Items to be a non-null empty list, got null")
			}
			if data.Items.IsUnknown() {
				t.Fatal("expected Items to be known, got unknown")
			}
			if got := len(data.Items.Elements()); got != 0 {
				t.Fatalf("expected 0 elements, got %d", got)
			}
			if data.RowCount.ValueInt64() != 0 {
				t.Fatalf("expected row_count 0, got %d", data.RowCount.ValueInt64())
			}
		})
	}
}

func TestWidgetsDataSource_Read(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     int
		body       string
		wantErr    bool
		errPattern string
		wantCount  int
	}{
		{
			name:   "successful list with two widgets",
			status: http.StatusOK,
			body: `{
				"items": [
					{
						"id": "spend-widget-123",
						"alias": "current-month-cloud-spend",
						"name": "Current month cloud spend",
						"description": "Total cloud spend accumulated during the current calendar month.",
						"type": "preset",
						"kind": "monetary_metric"
					},
					{
						"id": "forecast-widget-456",
						"alias": "current-month-cloud-forecast",
						"name": "Current month cloud forecast",
						"description": "Projected total cloud spend for the entire current calendar month.",
						"type": "preset",
						"kind": "monetary_metric"
					}
				],
				"rowCount": 2
			}`,
			wantErr:   false,
			wantCount: 2,
		},
		{
			name:       "forbidden 403",
			status:     http.StatusForbidden,
			body:       `{"status": 403, "title": "Forbidden", "detail": "access denied"}`,
			wantErr:    true,
			errPattern: "status 403",
		},
		{
			name:       "server error 500",
			status:     http.StatusInternalServerError,
			body:       `{"status": 500, "title": "Internal Server Error"}`,
			wantErr:    true,
			errPattern: "status 500",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/analytics/v1/widgets" {
					t.Errorf("unexpected request: %s %s, want GET /analytics/v1/widgets", r.Method, r.URL)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))

			resp := readWidgets(t, server)
			if tc.wantErr {
				if !resp.Diagnostics.HasError() {
					t.Fatalf("expected error containing %q, got none", tc.errPattern)
				}
				found := false
				for _, diag := range resp.Diagnostics.Errors() {
					if diag.Summary() == tc.errPattern || strings.Contains(diag.Detail(), tc.errPattern) {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("expected error containing %q, got %v", tc.errPattern, resp.Diagnostics)
				}
				return
			}

			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
			}

			var state widgetsDataSourceModel
			if diags := resp.State.Get(t.Context(), &state); diags.HasError() {
				t.Fatal(diags)
			}

			if state.RowCount.ValueInt64() != int64(tc.wantCount) {
				t.Errorf("expected row_count %d, got %d", tc.wantCount, state.RowCount.ValueInt64())
			}

			var items []datasource_widgets.ItemsValue
			if diags := state.Items.ElementsAs(t.Context(), &items, false); diags.HasError() {
				t.Fatal(diags)
			}

			if len(items) != tc.wantCount {
				t.Fatalf("expected %d items, got %d", tc.wantCount, len(items))
			}

			if tc.wantCount > 0 {
				if items[0].Alias.ValueString() != "current-month-cloud-spend" {
					t.Errorf("unexpected item 0 alias: %s", items[0].Alias.ValueString())
				}
				if items[0].Name.ValueString() != "Current month cloud spend" {
					t.Errorf("unexpected item 0 name: %s", items[0].Name.ValueString())
				}
			}
		})
	}
}

func readWidgets(t *testing.T, server *httptest.Server) datasource.ReadResponse {
	t.Helper()
	ctx := t.Context()
	httpClient := server.Client()
	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}

	ds := NewWidgetsDataSource()
	var configureResp datasource.ConfigureResponse
	ds.(datasource.DataSourceWithConfigure).Configure(ctx, datasource.ConfigureRequest{ProviderData: client}, &configureResp)
	if configureResp.Diagnostics.HasError() {
		t.Fatal(configureResp.Diagnostics)
	}

	var schemaResp datasource.SchemaResponse
	ds.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
	if diags := schemaResp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatal(diags)
	}

	attrTypes := map[string]tftypes.Type{}
	values := map[string]tftypes.Value{}
	for name, attribute := range schemaResp.Schema.Attributes {
		tfType := attribute.GetType().TerraformType(ctx)
		attrTypes[name] = tfType
		values[name] = tftypes.NewValue(tfType, nil)
	}

	resp := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	ds.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(tftypes.Object{AttributeTypes: attrTypes}, values)}}, &resp)
	return resp
}
