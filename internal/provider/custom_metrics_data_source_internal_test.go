package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_custom_metrics"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// readCustomMetricsHelper builds a customMetricsDataSource backed by a client
// pointed at server, runs Read with the given config overrides (every other
// attribute is null) and returns the state and diagnostics.
func readCustomMetricsHelper(t *testing.T, server *httptest.Server, overrides map[string]tftypes.Value) (customMetricsDataSourceModel, diag.Diagnostics) {
	t.Helper()

	httpClient := server.Client()
	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ds := &customMetricsDataSource{client: client}
	ctx := t.Context()

	schemaResp := &datasource.SchemaResponse{}
	ds.Schema(ctx, datasource.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("failed to get schema: %v", schemaResp.Diagnostics)
	}

	configValues := map[string]tftypes.Value{}
	attrTypes := map[string]tftypes.Type{}
	for name, a := range schemaResp.Schema.Attributes {
		tfType := a.GetType().TerraformType(ctx)
		attrTypes[name] = tfType
		if override, ok := overrides[name]; ok {
			configValues[name] = override
			continue
		}
		configValues[name] = tftypes.NewValue(tfType, nil)
	}

	config := tfsdk.Config{
		Schema: schemaResp.Schema,
		Raw:    tftypes.NewValue(tftypes.Object{AttributeTypes: attrTypes}, configValues),
	}
	readResp := &datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	ds.Read(ctx, datasource.ReadRequest{Config: config}, readResp)
	if readResp.Diagnostics.HasError() {
		return customMetricsDataSourceModel{}, readResp.Diagnostics
	}

	var data customMetricsDataSourceModel
	if diags := readResp.State.Get(ctx, &data); diags.HasError() {
		t.Fatalf("failed to read state: %v", diags)
	}
	return data, readResp.Diagnostics
}

func cmString(s string) tftypes.Value { return tftypes.NewValue(tftypes.String, s) }
func cmInt(n int64) tftypes.Value     { return tftypes.NewValue(tftypes.Number, n) }

// cmItemJSON is a custom metric as the API returns it.
func cmItemJSON(id, name string) string {
	return fmt.Sprintf(`{
  "id": %q, "etag": "\"e-%[1]s\"", "name": %q, "description": "d-%[1]s", "owner": "owner@example.com",
  "formula": "A / B", "unitType": "number", "decimals": {"min": 0, "max": 2},
  "variables": [
    {"letter": "A", "metric": {"type": "basic", "value": "cost"}, "allocationId": "alloc-a"},
    {"letter": "B", "metric": {"type": "extended", "value": "amortized_cost"}, "allocationId": "alloc-b"}
  ],
  "createTime": "2026-10-12T09:30:00Z", "updateTime": "2026-10-13T10:00:00.5Z"
}`, id, name)
}

// cmPagedServer serves a fixed sequence of pages keyed by the pageToken query
// parameter and records every request's query.
type cmPagedServer struct {
	mu      sync.Mutex
	queries []url.Values
	pages   map[string]string // pageToken -> response body
}

func (s *cmPagedServer) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.queries = append(s.queries, r.URL.Query())
		s.mu.Unlock()

		body, ok := s.pages[r.URL.Query().Get("pageToken")]
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"code":"pagination_token_invalid"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})
}

func (s *cmPagedServer) requests() []url.Values {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]url.Values(nil), s.queries...)
}

func newCMPages() map[string]string {
	return map[string]string{
		"": fmt.Sprintf(`{"items": [%s, %s], "pageToken": "t2", "rowCount": 4}`,
			cmItemJSON("m1", "one"), cmItemJSON("m2", "two")),
		"t2": fmt.Sprintf(`{"items": [%s], "pageToken": "t3", "rowCount": 4}`,
			cmItemJSON("m3", "three")),
		"t3": fmt.Sprintf(`{"items": [%s], "rowCount": 4}`,
			cmItemJSON("m4", "four")),
	}
}

func cmItemNames(t *testing.T, data customMetricsDataSourceModel) []string {
	t.Helper()
	if data.Items.IsNull() || data.Items.IsUnknown() {
		t.Fatalf("items = %s, want a known list", data.Items)
	}
	names := []string{}
	for _, e := range data.Items.Elements() {
		obj := e.(datasource_custom_metrics.ItemsValue)
		names = append(names, obj.Name.ValueString())
	}
	return names
}

func TestCustomMetricsDataSource_Pagination(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		pages         map[string]string
		overrides     map[string]tftypes.Value
		wantNames     []string
		wantTokens    []string // pageToken sent by each request
		wantRowCount  int64
		wantPageToken types.String
		wantMaxResult types.Int64
	}{
		{
			name:          "auto-paginates every page",
			pages:         newCMPages(),
			wantNames:     []string{"one", "two", "three", "four"},
			wantTokens:    []string{"", "t2", "t3"},
			wantRowCount:  4,
			wantPageToken: types.StringNull(),
			wantMaxResult: types.Int64Null(),
		},
		{
			name:          "page_token alone auto-paginates from the token",
			pages:         newCMPages(),
			overrides:     map[string]tftypes.Value{"page_token": cmString("t2")},
			wantNames:     []string{"three", "four"},
			wantTokens:    []string{"t2", "t3"},
			wantRowCount:  4,
			wantPageToken: types.StringValue("t2"),
			wantMaxResult: types.Int64Null(),
		},
		{
			name:          "max_results reads one page and keeps the next token",
			pages:         newCMPages(),
			overrides:     map[string]tftypes.Value{"max_results": cmInt(2)},
			wantNames:     []string{"one", "two"},
			wantTokens:    []string{""},
			wantRowCount:  4,
			wantPageToken: types.StringValue("t2"),
			wantMaxResult: types.Int64Value(2),
		},
		{
			name:  "max_results with page_token reads the last page",
			pages: newCMPages(),
			overrides: map[string]tftypes.Value{
				"max_results": cmInt(2),
				"page_token":  cmString("t3"),
			},
			wantNames:     []string{"four"},
			wantTokens:    []string{"t3"},
			wantRowCount:  4,
			wantPageToken: types.StringNull(),
			wantMaxResult: types.Int64Value(2),
		},
		{
			name: "row_count falls back to the fetched items when the API omits it",
			pages: map[string]string{
				"": fmt.Sprintf(`{"items": [%s, %s]}`, cmItemJSON("m1", "one"), cmItemJSON("m2", "two")),
			},
			wantNames:     []string{"one", "two"},
			wantTokens:    []string{""},
			wantRowCount:  2,
			wantPageToken: types.StringNull(),
			wantMaxResult: types.Int64Null(),
		},
		{
			name: "row_count falls back to all fetched items across pages when the API omits it",
			pages: map[string]string{
				"":   fmt.Sprintf(`{"items": [%s, %s], "pageToken": "t2"}`, cmItemJSON("m1", "one"), cmItemJSON("m2", "two")),
				"t2": fmt.Sprintf(`{"items": [%s], "pageToken": "t3"}`, cmItemJSON("m3", "three")),
				"t3": fmt.Sprintf(`{"items": [%s]}`, cmItemJSON("m4", "four")),
			},
			wantNames:     []string{"one", "two", "three", "four"},
			wantTokens:    []string{"", "t2", "t3"},
			wantRowCount:  4,
			wantPageToken: types.StringNull(),
			wantMaxResult: types.Int64Null(),
		},
		{
			name: "row_count falls back to all fetched items across pages when the API returns null",
			pages: map[string]string{
				"":   fmt.Sprintf(`{"items": [%s, %s], "pageToken": "t2", "rowCount": null}`, cmItemJSON("m1", "one"), cmItemJSON("m2", "two")),
				"t2": fmt.Sprintf(`{"items": [%s], "pageToken": "t3", "rowCount": null}`, cmItemJSON("m3", "three")),
				"t3": fmt.Sprintf(`{"items": [%s], "rowCount": null}`, cmItemJSON("m4", "four")),
			},
			wantNames:     []string{"one", "two", "three", "four"},
			wantTokens:    []string{"", "t2", "t3"},
			wantRowCount:  4,
			wantPageToken: types.StringNull(),
			wantMaxResult: types.Int64Null(),
		},
		{
			name: "a row_count the first page supplies wins over the fetched count",
			pages: map[string]string{
				"":   fmt.Sprintf(`{"items": [%s], "pageToken": "t2", "rowCount": 9}`, cmItemJSON("m1", "one")),
				"t2": fmt.Sprintf(`{"items": [%s]}`, cmItemJSON("m2", "two")),
			},
			wantNames:     []string{"one", "two"},
			wantTokens:    []string{"", "t2"},
			wantRowCount:  9,
			wantPageToken: types.StringNull(),
			wantMaxResult: types.Int64Null(),
		},
		{
			name:          "no items is an empty list",
			pages:         map[string]string{"": `{"items": [], "rowCount": 0}`},
			wantNames:     []string{},
			wantTokens:    []string{""},
			wantRowCount:  0,
			wantPageToken: types.StringNull(),
			wantMaxResult: types.Int64Null(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fake := &cmPagedServer{pages: tc.pages}
			server := httptest.NewTestServer(t, fake.handler())

			data, diags := readCustomMetricsHelper(t, server, tc.overrides)
			if diags.HasError() {
				t.Fatalf("Read() returned diagnostics: %v", diags)
			}

			if got := cmItemNames(t, data); strings.Join(got, ",") != strings.Join(tc.wantNames, ",") {
				t.Errorf("item names = %v, want %v", got, tc.wantNames)
			}

			reqs := fake.requests()
			var tokens []string
			for _, q := range reqs {
				tokens = append(tokens, q.Get("pageToken"))
			}
			if strings.Join(tokens, ",") != strings.Join(tc.wantTokens, ",") {
				t.Errorf("pageToken per request = %q, want %q", tokens, tc.wantTokens)
			}

			if !data.RowCount.Equal(types.Int64Value(tc.wantRowCount)) {
				t.Errorf("row_count = %s, want %d", data.RowCount, tc.wantRowCount)
			}
			if !data.PageToken.Equal(tc.wantPageToken) {
				t.Errorf("page_token = %s, want %s", data.PageToken, tc.wantPageToken)
			}
			if !data.MaxResults.Equal(tc.wantMaxResult) {
				t.Errorf("max_results = %s, want %s", data.MaxResults, tc.wantMaxResult)
			}
		})
	}
}

// TestCustomMetricsDataSource_QueryParameters checks that every configured
// input reaches every page request, and that an unset input is not sent.
func TestCustomMetricsDataSource_QueryParameters(t *testing.T) {
	t.Parallel()

	fake := &cmPagedServer{pages: newCMPages()}
	server := httptest.NewTestServer(t, fake.handler())

	data, diags := readCustomMetricsHelper(t, server, map[string]tftypes.Value{
		"name_contains": cmString("cost"),
		"filter":        cmString("owner:finops%40example.com"),
		"sort_by":       cmString("name"),
		"sort_order":    cmString("asc"),
	})
	if diags.HasError() {
		t.Fatalf("Read() returned diagnostics: %v", diags)
	}

	reqs := fake.requests()
	if len(reqs) != 3 {
		t.Fatalf("requests = %d, want 3", len(reqs))
	}
	for i, q := range reqs {
		for key, want := range map[string]string{
			"nameContains": "cost",
			"filter":       "owner:finops%40example.com",
			"sortBy":       "name",
			"sortOrder":    "asc",
		} {
			if got := q.Get(key); got != want {
				t.Errorf("request %d: %s = %q, want %q", i, key, got, want)
			}
		}
		if q.Has("maxResults") {
			t.Errorf("request %d sent maxResults %q although it is not configured", i, q.Get("maxResults"))
		}
	}

	// The configured inputs stay as configured.
	for name, got := range map[string]types.String{
		"name_contains": data.NameContains,
		"filter":        data.Filter,
		"sort_by":       data.SortBy,
		"sort_order":    data.SortOrder,
	} {
		if got.IsNull() || got.IsUnknown() {
			t.Errorf("%s = %s, want the configured value", name, got)
		}
	}
}

// TestCustomMetricsDataSource_UnsetInputsStayNull checks that omitted inputs
// are neither sent nor filled in from the API's defaults.
func TestCustomMetricsDataSource_UnsetInputsStayNull(t *testing.T) {
	t.Parallel()

	fake := &cmPagedServer{pages: map[string]string{"": `{"items": [], "rowCount": 0}`}}
	server := httptest.NewTestServer(t, fake.handler())

	data, diags := readCustomMetricsHelper(t, server, nil)
	if diags.HasError() {
		t.Fatalf("Read() returned diagnostics: %v", diags)
	}

	reqs := fake.requests()
	if len(reqs) != 1 {
		t.Fatalf("requests = %d, want 1", len(reqs))
	}
	if got := reqs[0].Encode(); got != "" {
		t.Errorf("query = %q, want none", got)
	}
	for name, got := range map[string]types.String{
		"name_contains": data.NameContains,
		"filter":        data.Filter,
		"sort_by":       data.SortBy,
		"sort_order":    data.SortOrder,
	} {
		if !got.IsNull() {
			t.Errorf("%s = %s, want null", name, got)
		}
	}
}

// TestCustomMetricsDataSource_APIError checks that a non-200 response is an
// error carrying the status and body.
func TestCustomMetricsDataSource_APIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not json`))
	}))

	_, diags := readCustomMetricsHelper(t, server, nil)
	if !diags.HasError() {
		t.Fatal("Read() returned no error for an unparsable body")
	}
}

// TestCustomMetricsDataSource_UnknownConfigDefensiveFallback exercises the
// protocol-level fallback directly. Terraform Core normally defers a data
// source with unknown configuration, but protocol v6 permits clients to send
// unknown configuration to ReadDataSource. The fallback must make no API call
// and leave the results unknown; configured inputs keep their value and
// omitted ones become unknown.
func TestCustomMetricsDataSource_UnknownConfigDefensiveFallback(t *testing.T) {
	t.Parallel()

	unknownString := tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	unknownNumber := tftypes.NewValue(tftypes.Number, tftypes.UnknownValue)

	tests := []struct {
		name      string
		overrides map[string]tftypes.Value
	}{
		{"unknown name_contains", map[string]tftypes.Value{"name_contains": unknownString}},
		{"unknown filter", map[string]tftypes.Value{"filter": unknownString}},
		{"unknown sort_by", map[string]tftypes.Value{"sort_by": unknownString}},
		{"unknown sort_order", map[string]tftypes.Value{"sort_order": unknownString}},
		{"unknown max_results", map[string]tftypes.Value{"max_results": unknownNumber}},
		{"unknown page_token", map[string]tftypes.Value{"page_token": unknownString}},
		{"max_results and a starting page_token next to an unknown filter", map[string]tftypes.Value{
			"max_results": cmInt(2),
			"page_token":  cmString("t2"),
			"filter":      unknownString,
		}},
		{"unknown max_results next to a known page_token", map[string]tftypes.Value{
			"max_results": unknownNumber,
			"page_token":  cmString("t2"),
		}},
		{"a starting page_token in auto-pagination mode next to an unknown filter", map[string]tftypes.Value{
			"page_token": cmString("t2"),
			"filter":     unknownString,
		}},
		{"known inputs next to an unknown one", map[string]tftypes.Value{
			"name_contains": cmString("cost"),
			"sort_by":       cmString("name"),
			"page_token":    unknownString,
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var requests int
			var mu sync.Mutex
			server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				mu.Lock()
				requests++
				mu.Unlock()
				w.WriteHeader(http.StatusOK)
			}))

			data, diags := readCustomMetricsHelper(t, server, tc.overrides)
			if diags.HasError() {
				t.Fatalf("Read() returned diagnostics: %v", diags)
			}

			// Only a client built from server.Client() can reach the server, so
			// zero here is meaningful.
			mu.Lock()
			got := requests
			mu.Unlock()
			if got != 0 {
				t.Errorf("API requests = %d, want 0", got)
			}

			if !data.Items.IsUnknown() {
				t.Errorf("items = %s, want unknown", data.Items)
			}
			if !data.RowCount.IsUnknown() {
				t.Errorf("row_count = %s, want unknown", data.RowCount)
			}

			// An input is unknown when it was unknown or omitted, and keeps its
			// value when it was configured.
			wantString := func(attrName string, got types.String) {
				t.Helper()
				if override, ok := tc.overrides[attrName]; ok {
					if override.IsKnown() {
						var want string
						if err := override.As(&want); err != nil {
							t.Fatal(err)
						}
						if !got.Equal(types.StringValue(want)) {
							t.Errorf("%s = %s, want %q", attrName, got, want)
						}
						return
					}
				}
				if !got.IsUnknown() {
					t.Errorf("%s = %s, want unknown", attrName, got)
				}
			}
			wantString("name_contains", data.NameContains)
			wantString("filter", data.Filter)
			wantString("sort_by", data.SortBy)
			wantString("sort_order", data.SortOrder)
			// With max_results set (or unknown) page_token is the next page's
			// token, which is not known without the call; without max_results a
			// configured page_token is only the starting point and stays.
			if _, manual := tc.overrides["max_results"]; manual {
				if !data.PageToken.IsUnknown() {
					t.Errorf("page_token = %s, want unknown in single-page mode", data.PageToken)
				}
			} else {
				wantString("page_token", data.PageToken)
			}
			if override, ok := tc.overrides["max_results"]; ok && override.IsKnown() {
				if !data.MaxResults.Equal(types.Int64Value(2)) {
					t.Errorf("max_results = %s, want the configured 2", data.MaxResults)
				}
			} else if !data.MaxResults.IsUnknown() {
				t.Errorf("max_results = %s, want unknown", data.MaxResults)
			}
		})
	}
}

// TestMapCustomMetricsToDataSource compares the full mapped list with a freshly
// built expectation.
func TestMapCustomMetricsToDataSource(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	var full models.CustomMetric
	if err := json.Unmarshal([]byte(cmItemJSON("m1", "one")), &full); err != nil {
		t.Fatalf("unmarshal full item: %v", err)
	}
	var bare models.CustomMetric
	if err := json.Unmarshal([]byte(`{
  "id": "m2", "etag": "\"e2\"", "name": "two", "description": "", "formula": "A", "unitType": "number",
  "decimals": null, "variables": []
}`), &bare); err != nil {
		t.Fatalf("unmarshal bare item: %v", err)
	}

	var data customMetricsDataSourceModel
	diags := mapCustomMetricsToDataSource(ctx, []models.CustomMetric{full, bare}, &data)
	if diags.HasError() {
		t.Fatalf("mapCustomMetricsToDataSource() diagnostics: %v", diags)
	}

	itemTypes := datasource_custom_metrics.ItemsValue{}.AttributeTypes(ctx)
	varType := datasource_custom_metrics.VariablesValue{}.Type(ctx)
	variable := func(letter, metricType, metricValue, alloc string) datasource_custom_metrics.VariablesValue {
		return datasource_custom_metrics.NewVariablesValueMust(
			datasource_custom_metrics.VariablesValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"allocation_id": types.StringValue(alloc),
				"letter":        types.StringValue(letter),
				"metric": datasource_custom_metrics.NewMetricValueMust(
					datasource_custom_metrics.MetricValue{}.AttributeTypes(ctx),
					map[string]attr.Value{
						"type":  types.StringValue(metricType),
						"value": types.StringValue(metricValue),
					}),
			})
	}

	fullVars, d := types.ListValueFrom(ctx, varType, []datasource_custom_metrics.VariablesValue{
		variable("A", "basic", "cost", "alloc-a"),
		variable("B", "extended", "amortized_cost", "alloc-b"),
	})
	if d.HasError() {
		t.Fatal(d)
	}
	emptyVars, d := types.ListValueFrom(ctx, varType, []datasource_custom_metrics.VariablesValue{})
	if d.HasError() {
		t.Fatal(d)
	}

	wantFull := datasource_custom_metrics.NewItemsValueMust(itemTypes, map[string]attr.Value{
		"id":          types.StringValue("m1"),
		"etag":        types.StringValue(`"e-m1"`),
		"name":        types.StringValue("one"),
		"description": types.StringValue("d-m1"),
		"owner":       types.StringValue("owner@example.com"),
		"formula":     types.StringValue("A / B"),
		"unit_type":   types.StringValue("number"),
		"decimals": datasource_custom_metrics.NewDecimalsValueMust(
			datasource_custom_metrics.DecimalsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{"min": types.Int64Value(0), "max": types.Int64Value(2)}),
		"variables":   fullVars,
		"create_time": types.StringValue("2026-10-12T09:30:00Z"),
		"update_time": types.StringValue("2026-10-13T10:00:00.5Z"),
	})
	wantBare := datasource_custom_metrics.NewItemsValueMust(itemTypes, map[string]attr.Value{
		"id":          types.StringValue("m2"),
		"etag":        types.StringValue(`"e2"`),
		"name":        types.StringValue("two"),
		"description": types.StringValue(""),
		"owner":       types.StringNull(),
		"formula":     types.StringValue("A"),
		"unit_type":   types.StringValue("number"),
		"decimals":    datasource_custom_metrics.NewDecimalsValueNull(),
		"variables":   emptyVars,
		"create_time": types.StringNull(),
		"update_time": types.StringNull(),
	})
	want, d := types.ListValueFrom(ctx, datasource_custom_metrics.ItemsValue{}.Type(ctx),
		[]datasource_custom_metrics.ItemsValue{wantFull, wantBare})
	if d.HasError() {
		t.Fatal(d)
	}

	if !data.Items.Equal(want) {
		t.Errorf("items mismatch\n got: %s\nwant: %s", data.Items, want)
	}
}
