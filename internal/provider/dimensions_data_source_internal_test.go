package provider

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_dimensions"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func readDimensionsHelper(t *testing.T, server *httptest.Server, overrides map[string]tftypes.Value) (dimensionsDataSourceModel, datasource.ReadResponse) {
	t.Helper()
	ctx := t.Context()

	httpClient := server.Client()
	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ds := NewDimensionsDataSource()
	var configureResp datasource.ConfigureResponse
	ds.(datasource.DataSourceWithConfigure).Configure(ctx, datasource.ConfigureRequest{ProviderData: client}, &configureResp)
	if configureResp.Diagnostics.HasError() {
		t.Fatalf("failed to configure data source: %v", configureResp.Diagnostics)
	}

	var schemaResp datasource.SchemaResponse
	ds.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
	if diags := schemaResp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("schema validation failed: %v", diags)
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

	readResp := datasource.ReadResponse{
		State: tfsdk.State{Schema: schemaResp.Schema},
	}
	ds.Read(ctx, datasource.ReadRequest{Config: config}, &readResp)

	var model dimensionsDataSourceModel
	if !readResp.Diagnostics.HasError() && !readResp.State.Raw.IsNull() {
		if diags := readResp.State.Get(ctx, &model); diags.HasError() {
			t.Fatalf("failed to get state: %v", diags)
		}
	}
	return model, readResp
}

func TestDimensionsDataSource_QueryParams(t *testing.T) {
	for _, tc := range []struct {
		name        string
		overrides   map[string]tftypes.Value
		wantSortBy  string
		wantSortDir string
		wantFilter  string
		wantMaxRes  string
		wantPageTok string
	}{
		{
			name: "sort_by id and sort_order desc",
			overrides: map[string]tftypes.Value{
				"sort_by":    tftypes.NewValue(tftypes.String, "id"),
				"sort_order": tftypes.NewValue(tftypes.String, "desc"),
			},
			wantSortBy:  "id",
			wantSortDir: "desc",
		},
		{
			name: "sort_by id and sort_order asc",
			overrides: map[string]tftypes.Value{
				"sort_by":    tftypes.NewValue(tftypes.String, "id"),
				"sort_order": tftypes.NewValue(tftypes.String, "asc"),
			},
			wantSortBy:  "id",
			wantSortDir: "asc",
		},
		{
			name: "sort_by key",
			overrides: map[string]tftypes.Value{
				"sort_by": tftypes.NewValue(tftypes.String, "key"),
			},
			wantSortBy: "key",
		},
		{
			name: "sort_by label",
			overrides: map[string]tftypes.Value{
				"sort_by": tftypes.NewValue(tftypes.String, "label"),
			},
			wantSortBy: "label",
		},
		{
			name: "sort_by type",
			overrides: map[string]tftypes.Value{
				"sort_by": tftypes.NewValue(tftypes.String, "type"),
			},
			wantSortBy: "type",
		},
		{
			name:      "omitted sort_by and sort_order",
			overrides: map[string]tftypes.Value{},
		},
		{
			name: "combined filter, pagination, and sorting",
			overrides: map[string]tftypes.Value{
				"filter":      tftypes.NewValue(tftypes.String, "type:google-cloud"),
				"max_results": tftypes.NewValue(tftypes.Number, int64(25)),
				"page_token":  tftypes.NewValue(tftypes.String, "tok-xyz"),
				"sort_by":     tftypes.NewValue(tftypes.String, "id"),
				"sort_order":  tftypes.NewValue(tftypes.String, "asc"),
			},
			wantFilter:  "type:google-cloud",
			wantMaxRes:  "25",
			wantPageTok: "tok-xyz",
			wantSortBy:  "id",
			wantSortDir: "asc",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var capturedReq *http.Request
			server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedReq = r.Clone(t.Context())
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"dimensions": []}`))
			}))

			_, resp := readDimensionsHelper(t, server, tc.overrides)
			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected error: %v", resp.Diagnostics)
			}
			if capturedReq == nil {
				t.Fatal("expected request to be made")
			}

			q := capturedReq.URL.Query()
			if got := q.Get("sortBy"); got != tc.wantSortBy {
				t.Errorf("sortBy query param = %q, want %q", got, tc.wantSortBy)
			}
			if got := q.Get("sortOrder"); got != tc.wantSortDir {
				t.Errorf("sortOrder query param = %q, want %q", got, tc.wantSortDir)
			}
			if got := q.Get("filter"); got != tc.wantFilter {
				t.Errorf("filter query param = %q, want %q", got, tc.wantFilter)
			}
			if got := q.Get("maxResults"); got != tc.wantMaxRes {
				t.Errorf("maxResults query param = %q, want %q", got, tc.wantMaxRes)
			}
			if got := q.Get("pageToken"); got != tc.wantPageTok {
				t.Errorf("pageToken query param = %q, want %q", got, tc.wantPageTok)
			}
		})
	}
}

func TestDimensionsDataSource_UnknownGuards(t *testing.T) {
	for _, tc := range []struct {
		name      string
		overrides map[string]tftypes.Value
	}{
		{
			name: "unknown filter",
			overrides: map[string]tftypes.Value{
				"filter": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
			},
		},
		{
			name: "unknown sort_by",
			overrides: map[string]tftypes.Value{
				"sort_by": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
			},
		},
		{
			name: "unknown sort_order",
			overrides: map[string]tftypes.Value{
				"sort_order": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
			},
		},
		{
			name: "unknown max_results",
			overrides: map[string]tftypes.Value{
				"max_results": tftypes.NewValue(tftypes.Number, tftypes.UnknownValue),
			},
		},
		{
			name: "unknown page_token",
			overrides: map[string]tftypes.Value{
				"page_token": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var requestCount atomic.Int32
			server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestCount.Add(1)
				w.WriteHeader(http.StatusOK)
			}))

			model, resp := readDimensionsHelper(t, server, tc.overrides)
			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
			}
			if count := requestCount.Load(); count != 0 {
				t.Errorf("expected 0 HTTP requests, got %d", count)
			}
			if !model.Dimensions.IsUnknown() {
				t.Errorf("expected Dimensions to be unknown, got %v", model.Dimensions)
			}
			if !model.RowCount.IsUnknown() {
				t.Errorf("expected RowCount to be unknown, got %v", model.RowCount)
			}
		})
	}
}

func TestDimensionsDataSource_ResponseMapping(t *testing.T) {
	for _, tc := range []struct {
		name             string
		body             string
		overrides        map[string]tftypes.Value
		wantRowCount     int64
		wantDimsCount    int
		wantPageToken    string
		wantPageTokenSet bool
	}{
		{
			name: "canonical dimensions with rowCount and pageToken in manual mode",
			overrides: map[string]tftypes.Value{
				"max_results": tftypes.NewValue(tftypes.Number, int64(2)),
			},
			body: `{
				"rowCount": 2,
				"pageToken": "next-page-tok",
				"dimensions": [
					{
						"id": "dimension-1",
						"label": "Project ID",
						"type": "fixed"
					},
					{
						"id": "dimension-2",
						"label": "Service",
						"type": "google-cloud"
					}
				]
			}`,
			wantRowCount:     2,
			wantDimsCount:    2,
			wantPageToken:    "next-page-tok",
			wantPageTokenSet: true,
		},
		{
			name: "fallback rowCount when omitted by API in manual mode",
			overrides: map[string]tftypes.Value{
				"max_results": tftypes.NewValue(tftypes.Number, int64(2)),
			},
			body: `{
				"dimensions": [
					{
						"id": "dimension-1",
						"label": "Project ID",
						"type": "fixed"
					}
				]
			}`,
			wantRowCount:  1,
			wantDimsCount: 1,
		},
		{
			name:      "empty dimensions array in auto mode",
			overrides: map[string]tftypes.Value{},
			body: `{
				"dimensions": []
			}`,
			wantRowCount:  0,
			wantDimsCount: 0,
		},
		{
			name:          "null dimensions array in auto mode",
			overrides:     map[string]tftypes.Value{},
			body:          `{}`,
			wantRowCount:  0,
			wantDimsCount: 0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			}))

			model, resp := readDimensionsHelper(t, server, tc.overrides)
			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected error: %v", resp.Diagnostics)
			}

			if model.RowCount.IsNull() || model.RowCount.IsUnknown() || model.RowCount.ValueInt64() != tc.wantRowCount {
				t.Errorf("RowCount = %v, want %d", model.RowCount, tc.wantRowCount)
			}

			if model.Dimensions.IsNull() || model.Dimensions.IsUnknown() {
				t.Fatalf("Dimensions should not be null or unknown, got %v", model.Dimensions)
			}
			if len(model.Dimensions.Elements()) != tc.wantDimsCount {
				t.Errorf("Dimensions length = %d, want %d", len(model.Dimensions.Elements()), tc.wantDimsCount)
			}

			if tc.wantPageTokenSet {
				if model.PageToken.IsNull() || model.PageToken.ValueString() != tc.wantPageToken {
					t.Errorf("PageToken = %v, want %s", model.PageToken, tc.wantPageToken)
				}
			} else {
				if !model.PageToken.IsNull() {
					t.Errorf("PageToken = %v, want null", model.PageToken)
				}
			}

			if tc.wantDimsCount > 0 {
				var dims []datasource_dimensions.DimensionsValue
				if diags := model.Dimensions.ElementsAs(t.Context(), &dims, false); diags.HasError() {
					t.Fatalf("failed to decode dimensions: %v", diags)
				}
				if !dims[0].Id.Equal(types.StringValue("dimension-1")) {
					t.Errorf("dims[0].Id = %v, want dimension-1", dims[0].Id)
				}
				if !dims[0].Label.Equal(types.StringValue("Project ID")) {
					t.Errorf("dims[0].Label = %v, want Project ID", dims[0].Label)
				}
				if !dims[0].DimensionsType.Equal(types.StringValue("fixed")) {
					t.Errorf("dims[0].Type = %v, want fixed", dims[0].DimensionsType)
				}
			}
		})
	}
}

func TestDimensionsDataSource_ErrorHandling(t *testing.T) {
	for _, tc := range []struct {
		name        string
		status      int
		body        string
		contentType string
		wantError   string
	}{
		{
			name:        "bad request",
			status:      400,
			body:        `{"error": "invalid parameter"}`,
			contentType: "application/json",
			wantError:   "API returned status 400: {\"error\": \"invalid parameter\"}",
		},
		{
			name:        "forbidden",
			status:      403,
			body:        `{"error": "access denied"}`,
			contentType: "application/json",
			wantError:   "API returned status 403: {\"error\": \"access denied\"}",
		},
		{
			name:        "server error",
			status:      500,
			body:        `{"error": "internal error"}`,
			contentType: "application/json",
			wantError:   "API returned status 500: {\"error\": \"internal error\"}",
		},
		{
			name:        "non-JSON response",
			status:      200,
			body:        "plain text response",
			contentType: "text/plain",
			wantError:   "API returned status 200: plain text response",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))

			_, resp := readDimensionsHelper(t, server, map[string]tftypes.Value{})
			if !resp.Diagnostics.HasError() {
				t.Fatal("expected error diagnostic, got none")
			}
			if !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), tc.wantError) {
				t.Errorf("got error %q, want error containing %q", resp.Diagnostics.Errors()[0].Detail(), tc.wantError)
			}
			if !resp.State.Raw.IsNull() {
				t.Fatal("failed read must not set state")
			}
		})
	}
}
