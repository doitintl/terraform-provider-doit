package provider

import (
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_extended_metrics"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// extendedMetricsPages is a three-page result of one item per page, keyed by
// the incoming pageToken. The API's total is 3.
var extendedMetricsPages = map[string]struct{ body, next string }{
	"":   {`{"key":"a_cost","label":"A cost","dataSource":"billing"}`, "p2"},
	"p2": {`{"key":"b_rides","label":"B rides","dataSource":"billing-datahub"}`, "p3"},
	"p3": {`{"key":"c_gpu","label":"C gpu","dataSource":"tokenomics"}`, ""},
}

func extendedMetricsPageJSON(items string, next string, rowCount string) string {
	body := fmt.Sprintf(`{"items": [%s]`, items)
	if next != "" {
		body += fmt.Sprintf(`, "pageToken": %q`, next)
	}
	if rowCount != "" {
		body += `, "rowCount": ` + rowCount
	}
	return body + "}"
}

func extendedMetricsPagedHandler(w http.ResponseWriter, q url.Values) {
	page, ok := extendedMetricsPages[q.Get("pageToken")]
	if !ok {
		writeJSON(w, 400, `{"code":"pagination_token_invalid"}`)
		return
	}
	writeJSON(w, 200, extendedMetricsPageJSON(page.body, page.next, "3"))
}

func readExtendedMetricsForTest(t *testing.T, server *recordingServer, config map[string]tftypes.Value) (extendedMetricsDataSourceModel, datasource.ReadResponse) {
	t.Helper()
	httpClient := server.Client()
	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}
	ds := &extendedMetricsDataSource{client: client}
	ctx := t.Context()
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
	maps.Copy(values, config)
	resp := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	ds.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(tftypes.Object{AttributeTypes: attrTypes}, values)}}, &resp)

	var data extendedMetricsDataSourceModel
	if !resp.Diagnostics.HasError() {
		if diags := resp.State.Get(ctx, &data); diags.HasError() {
			t.Fatal(diags)
		}
	}
	return data, resp
}

func extendedMetricsKeys(t *testing.T, data extendedMetricsDataSourceModel) string {
	t.Helper()
	var items []datasource_extended_metrics.ItemsValue
	if diags := data.Items.ElementsAs(t.Context(), &items, false); diags.HasError() {
		t.Fatal(diags)
	}
	keys := make([]string, 0, len(items))
	for _, item := range items {
		keys = append(keys, item.Key.ValueString()+"@"+item.DataSource.ValueString())
	}
	return strings.Join(keys, ",")
}

func TestExtendedMetricsDataSource_MetadataAndConfigure(t *testing.T) {
	ds := NewExtendedMetricsDataSource()

	var metaResp datasource.MetadataResponse
	ds.Metadata(t.Context(), datasource.MetadataRequest{ProviderTypeName: "doit"}, &metaResp)
	if metaResp.TypeName != "doit_extended_metrics" {
		t.Fatalf("TypeName = %q, want %q", metaResp.TypeName, "doit_extended_metrics")
	}

	var configResp datasource.ConfigureResponse
	ds.(datasource.DataSourceWithConfigure).Configure(t.Context(), datasource.ConfigureRequest{ProviderData: nil}, &configResp)
	if configResp.Diagnostics.HasError() {
		t.Fatalf("unexpected error on nil ProviderData: %v", configResp.Diagnostics)
	}

	ds.(datasource.DataSourceWithConfigure).Configure(t.Context(), datasource.ConfigureRequest{ProviderData: "wrong-type"}, &configResp)
	if !configResp.Diagnostics.HasError() {
		t.Fatal("expected error on wrong ProviderData type, got none")
	}
}

func TestExtendedMetricsDataSourceAutoPagination(t *testing.T) {
	server := newRecordingServer(t, extendedMetricsPagedHandler)
	config := map[string]tftypes.Value{"filter": tftypes.NewValue(tftypes.String, "dataSource:billing,billing-datahub,tokenomics")}
	data, resp := readExtendedMetricsForTest(t, server, config)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}

	calls := server.calls()
	if len(calls) != 3 {
		t.Fatalf("made %d requests, want 3", len(calls))
	}
	for i, wantToken := range []string{"", "p2", "p3"} {
		if got := calls[i].Get("pageToken"); got != wantToken {
			t.Errorf("request %d pageToken = %q, want %q", i, got, wantToken)
		}
		if calls[i].Has("maxResults") {
			t.Errorf("request %d sent maxResults without configuration", i)
		}
		if got := calls[i].Get("filter"); got != "dataSource:billing,billing-datahub,tokenomics" {
			t.Errorf("request %d filter = %q", i, got)
		}
	}
	if got, want := extendedMetricsKeys(t, data), "a_cost@billing,b_rides@billing-datahub,c_gpu@tokenomics"; got != want {
		t.Errorf("items = %s, want %s", got, want)
	}
	if data.RowCount.ValueInt64() != 3 {
		t.Errorf("row_count = %v", data.RowCount)
	}
	if !data.MaxResults.IsNull() || !data.PageToken.IsNull() {
		t.Errorf("max_results and page_token must stay null: %v %v", data.MaxResults, data.PageToken)
	}
	if data.Filter.ValueString() != "dataSource:billing,billing-datahub,tokenomics" {
		t.Errorf("configured filter must be kept, got %v", data.Filter)
	}
}

func TestExtendedMetricsDataSourceOmittedFilterIsNotSent(t *testing.T) {
	server := newRecordingServer(t, extendedMetricsPagedHandler)
	data, resp := readExtendedMetricsForTest(t, server, nil)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	for i, call := range server.calls() {
		if call.Has("filter") {
			t.Errorf("request %d sent a filter without configuration: %q", i, call.Get("filter"))
		}
	}
	if !data.Filter.IsNull() {
		t.Errorf("an omitted filter must be null in state, got %v", data.Filter)
	}
}

func TestExtendedMetricsDataSourceMaxResultsIsSinglePage(t *testing.T) {
	server := newRecordingServer(t, extendedMetricsPagedHandler)
	config := map[string]tftypes.Value{"max_results": tftypes.NewValue(tftypes.Number, 1)}
	data, resp := readExtendedMetricsForTest(t, server, config)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}

	calls := server.calls()
	if len(calls) != 1 {
		t.Fatalf("made %d requests, want 1", len(calls))
	}
	if got := calls[0].Get("maxResults"); got != "1" {
		t.Errorf("maxResults = %q", got)
	}
	if got, want := extendedMetricsKeys(t, data), "a_cost@billing"; got != want {
		t.Errorf("items = %s, want %s", got, want)
	}
	if data.MaxResults.ValueInt64() != 1 || data.PageToken.ValueString() != "p2" {
		t.Errorf("max_results/page_token = %v / %v", data.MaxResults, data.PageToken)
	}
	// row_count is the total across pages, not the size of this page.
	if data.RowCount.ValueInt64() != 3 {
		t.Errorf("row_count = %v", data.RowCount)
	}
}

func TestExtendedMetricsDataSourceLastPageHasNoToken(t *testing.T) {
	server := newRecordingServer(t, extendedMetricsPagedHandler)
	config := map[string]tftypes.Value{
		"max_results": tftypes.NewValue(tftypes.Number, 1),
		"page_token":  tftypes.NewValue(tftypes.String, "p3"),
	}
	data, resp := readExtendedMetricsForTest(t, server, config)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if got := server.calls()[0].Get("pageToken"); got != "p3" {
		t.Errorf("pageToken = %q", got)
	}
	if !data.PageToken.IsNull() {
		t.Errorf("page_token after the last page must be null, got %v", data.PageToken)
	}
}

func TestExtendedMetricsDataSourcePageTokenStartsAutoPagination(t *testing.T) {
	server := newRecordingServer(t, extendedMetricsPagedHandler)
	config := map[string]tftypes.Value{"page_token": tftypes.NewValue(tftypes.String, "p2")}
	data, resp := readExtendedMetricsForTest(t, server, config)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}

	if n := len(server.calls()); n != 2 {
		t.Fatalf("made %d requests, want 2", n)
	}
	if got, want := extendedMetricsKeys(t, data), "b_rides@billing-datahub,c_gpu@tokenomics"; got != want {
		t.Errorf("items = %s, want %s", got, want)
	}
	if data.PageToken.ValueString() != "p2" {
		t.Errorf("configured page_token must be kept, got %v", data.PageToken)
	}
	// row_count is the API's total across all pages, not the size of the suffix
	// that was fetched.
	if data.RowCount.ValueInt64() != 3 {
		t.Errorf("row_count = %v, want the API's total of 3", data.RowCount)
	}
}

func TestExtendedMetricsDataSourceRowCountFallsBackToItemCount(t *testing.T) {
	server := newRecordingServer(t, func(w http.ResponseWriter, q url.Values) {
		page := extendedMetricsPages[q.Get("pageToken")]
		next := page.next
		if next == "p3" {
			next = ""
		}
		writeJSON(w, 200, extendedMetricsPageJSON(page.body, next, ""))
	})
	data, resp := readExtendedMetricsForTest(t, server, nil)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if data.RowCount.IsNull() || data.RowCount.ValueInt64() != 2 {
		t.Errorf("row_count = %v, want the 2 items read", data.RowCount)
	}
}

func TestExtendedMetricsDataSourceKeepsSameKeyOfTwoDataSources(t *testing.T) {
	server := newRecordingServer(t, func(w http.ResponseWriter, _ url.Values) {
		writeJSON(w, 200, extendedMetricsPageJSON(
			`{"key":"cost","label":"Cost","dataSource":"billing"},{"key":"cost","label":"Cost","dataSource":"bqlens"}`, "", "2"))
	})
	data, resp := readExtendedMetricsForTest(t, server, nil)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if got, want := extendedMetricsKeys(t, data), "cost@billing,cost@bqlens"; got != want {
		t.Errorf("items = %s, want %s", got, want)
	}
}

func TestExtendedMetricsDataSourceMapsEveryDataSource(t *testing.T) {
	sources := []models.ExtendedMetricDataSource{"billing", "billing-datahub", "bqlens", "genai-collection", "kubernetes-utilization", "tokenomics"}
	items := make([]models.ExtendedMetric, 0, len(sources))
	var want []string
	for _, source := range sources {
		items = append(items, models.ExtendedMetric{Key: "k_" + string(source), Label: "L " + string(source), DataSource: source})
		want = append(want, "k_"+string(source)+"@"+string(source))
	}

	var data extendedMetricsDataSourceModel
	if diags := mapExtendedMetricsToDataSource(t.Context(), items, &data); diags.HasError() {
		t.Fatal(diags)
	}
	if got := extendedMetricsKeys(t, data); got != strings.Join(want, ",") {
		t.Errorf("items = %s, want %s", got, strings.Join(want, ","))
	}
}

func TestExtendedMetricsDataSourceEmpty(t *testing.T) {
	server := newRecordingServer(t, func(w http.ResponseWriter, _ url.Values) { writeJSON(w, 200, `{"items": [], "rowCount": 0}`) })
	data, resp := readExtendedMetricsForTest(t, server, nil)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if data.Items.IsNull() || len(data.Items.Elements()) != 0 {
		t.Fatalf("items must be [], got %s", data.Items)
	}
	if data.RowCount.ValueInt64() != 0 {
		t.Errorf("row_count = %v", data.RowCount)
	}
}

// With unknown configuration the read makes no request and writes a state in
// which every output is unknown. Each input keeps its configured value; an
// omitted or unknown one is not yet determined. page_token is the exception: it
// is the next page's token whenever max_results is set or not yet known, and
// only in auto-pagination mode (max_results omitted) is a configured one a
// starting point that stays as configured.
func TestExtendedMetricsDataSourceUnknownConfig(t *testing.T) {
	unknownString := tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	unknownNumber := tftypes.NewValue(tftypes.Number, tftypes.UnknownValue)
	str := func(v string) tftypes.Value { return tftypes.NewValue(tftypes.String, v) }
	num := func(v int) tftypes.Value { return tftypes.NewValue(tftypes.Number, v) }

	const (
		unknown = "<unknown>"
	)

	for _, tc := range []struct {
		name   string
		config map[string]tftypes.Value
		// Expected state of the inputs, as types.*.String() renders them.
		wantFilter, wantMaxResults, wantPageToken string
	}{
		{
			name:       "unknown filter",
			config:     map[string]tftypes.Value{"filter": unknownString},
			wantFilter: unknown, wantMaxResults: unknown, wantPageToken: unknown,
		},
		{
			name:       "unknown max_results",
			config:     map[string]tftypes.Value{"max_results": unknownNumber},
			wantFilter: unknown, wantMaxResults: unknown, wantPageToken: unknown,
		},
		{
			name:       "unknown page_token",
			config:     map[string]tftypes.Value{"page_token": unknownString},
			wantFilter: unknown, wantMaxResults: unknown, wantPageToken: unknown,
		},
		{
			name: "max_results and a starting page_token next to an unknown filter",
			config: map[string]tftypes.Value{
				"max_results": num(1), "page_token": str("p2"), "filter": unknownString,
			},
			wantFilter: unknown, wantMaxResults: "1", wantPageToken: unknown,
		},
		{
			name: "unknown max_results next to a known page_token",
			config: map[string]tftypes.Value{
				"max_results": unknownNumber, "page_token": str("p2"),
			},
			wantFilter: unknown, wantMaxResults: unknown, wantPageToken: unknown,
		},
		{
			name: "a starting page_token in auto-pagination mode next to an unknown filter",
			config: map[string]tftypes.Value{
				"page_token": str("p2"), "filter": unknownString,
			},
			wantFilter: unknown, wantMaxResults: unknown, wantPageToken: `"p2"`,
		},
		{
			name: "max_results without a page_token next to an unknown filter",
			config: map[string]tftypes.Value{
				"max_results": num(1), "filter": unknownString,
			},
			wantFilter: unknown, wantMaxResults: "1", wantPageToken: unknown,
		},
		{
			name: "a known filter next to an unknown page_token",
			config: map[string]tftypes.Value{
				"filter": str("dataSource:billing"), "page_token": unknownString,
			},
			wantFilter: `"dataSource:billing"`, wantMaxResults: unknown, wantPageToken: unknown,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := newRecordingServer(t, extendedMetricsPagedHandler)
			data, resp := readExtendedMetricsForTest(t, server, tc.config)
			if resp.Diagnostics.HasError() {
				t.Fatal(resp.Diagnostics)
			}
			if n := len(server.calls()); n != 0 {
				t.Fatalf("made %d API requests for unknown input", n)
			}
			if !data.Items.IsUnknown() || !data.RowCount.IsUnknown() {
				t.Errorf("outputs must remain unknown: items %s, row_count %s", data.Items, data.RowCount)
			}
			if got := data.Filter.String(); got != tc.wantFilter {
				t.Errorf("filter = %s, want %s", got, tc.wantFilter)
			}
			if got := data.MaxResults.String(); got != tc.wantMaxResults {
				t.Errorf("max_results = %s, want %s", got, tc.wantMaxResults)
			}
			if got := data.PageToken.String(); got != tc.wantPageToken {
				t.Errorf("page_token = %s, want %s", got, tc.wantPageToken)
			}
		})
	}
}

func TestExtendedMetricsDataSourceErrors(t *testing.T) {
	t.Run("error on a later page fails the read", func(t *testing.T) {
		server := newRecordingServer(t, func(w http.ResponseWriter, q url.Values) {
			if q.Get("pageToken") == "" {
				writeJSON(w, 200, extendedMetricsPageJSON(extendedMetricsPages[""].body, "p2", "3"))
				return
			}
			writeJSON(w, 500, `{"error":"boom"}`)
		})
		_, resp := readExtendedMetricsForTest(t, server, nil)
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected an error")
		}
		if n := len(server.calls()); n != 2 {
			t.Errorf("made %d requests, want 2", n)
		}
	})

	t.Run("a 200 without a JSON body is an error with status and body", func(t *testing.T) {
		server := newRecordingServer(t, func(w http.ResponseWriter, _ url.Values) {
			w.Header().Set("Content-Type", "text/plain")
			writeJSON(w, 200, `not json`)
		})
		_, resp := readExtendedMetricsForTest(t, server, nil)
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected an error")
		}
		got := fmt.Sprint(resp.Diagnostics)
		if !strings.Contains(got, "status 200") || !strings.Contains(got, "not json") {
			t.Errorf("diagnostics = %v", got)
		}
	})

	t.Run("a rejected page token surfaces the status", func(t *testing.T) {
		server := newRecordingServer(t, extendedMetricsPagedHandler)
		config := map[string]tftypes.Value{"page_token": tftypes.NewValue(tftypes.String, "bogus")}
		_, resp := readExtendedMetricsForTest(t, server, config)
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected an error")
		}
	})
}

// Without a rowCount from the API, row_count is the number of items a read
// fetched, so two reads of the same collection report different values: a
// single page of one item reads 1, and a read from a page token reads what is
// left. Only an API-supplied total is the same for both.
func TestExtendedMetricsDataSourceRowCountFallbackDiffersBetweenReads(t *testing.T) {
	server := newRecordingServer(t, func(w http.ResponseWriter, q url.Values) {
		page := extendedMetricsPages[q.Get("pageToken")]
		writeJSON(w, 200, extendedMetricsPageJSON(page.body, page.next, ""))
	})

	firstPage, resp := readExtendedMetricsForTest(t, server, map[string]tftypes.Value{"max_results": tftypes.NewValue(tftypes.Number, 1)})
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	fromToken, resp := readExtendedMetricsForTest(t, server, map[string]tftypes.Value{"page_token": tftypes.NewValue(tftypes.String, "p2")})
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}

	if got := firstPage.RowCount.ValueInt64(); got != 1 {
		t.Errorf("first page row_count = %d, want the 1 item read", got)
	}
	if got := fromToken.RowCount.ValueInt64(); got != 2 {
		t.Errorf("from-token row_count = %d, want the 2 items read", got)
	}
}
