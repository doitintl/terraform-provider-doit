package provider

import (
	"context"
	"fmt"
	"maps"
	"math/big"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// listDataSourceCase describes one paged list data source for the shared
// pagination tests. Every data source with max_results and page_token needs an
// entry; TestListDataSourcePaginationCoverage enforces it.
type listDataSourceCase struct {
	newDataSource func() datasource.DataSource
	// path is the request path the fake server answers.
	path string
	// itemsKey is the JSON key of the items in the list response.
	itemsKey string
	// itemsAttr is the attribute that holds the items.
	itemsAttr string
	// item returns the JSON of the i-th item.
	item func(i int) string
	// config holds the required inputs.
	config map[string]tftypes.Value
	kind   listRowCountKind
	// countRequired is true when the response's rowCount is a required field,
	// so the API never omits it.
	countRequired bool
	// singlePageOnly is true for data sources that never auto-paginate.
	singlePageOnly bool
}

func listItemWithID(i int) string { return fmt.Sprintf(`{"id":"item-%d"}`, i) }

func listItemWithNumericID(i int) string { return fmt.Sprintf(`{"id":%d}`, i+1) }

var listDataSourceCases = map[string]listDataSourceCase{
	"alert_slack_channels": {
		newDataSource: NewAlertSlackChannelsDataSource, path: "/analytics/v1/alerts/slack-channels", itemsKey: "items", itemsAttr: "items", item: listItemWithID,
		kind: listRowCountTotalUnlessPaged,
	},
	"alerts":           {newDataSource: NewAlertsDataSource, path: "/analytics/v1/alerts", itemsKey: "alerts", itemsAttr: "alerts", item: listItemWithID, kind: listRowCountPage},
	"allocations":      {newDataSource: NewAllocationsDataSource, path: "/analytics/v1/allocations", itemsKey: "allocations", itemsAttr: "allocations", item: listItemWithID, kind: listRowCountPage},
	"annotations":      {newDataSource: NewAnnotationsDataSource, path: "/analytics/v1/annotations", itemsKey: "annotations", itemsAttr: "annotations", item: listItemWithID, kind: listRowCountPage},
	"anomalies":        {newDataSource: NewAnomaliesDataSource, path: "/anomalies/v1", itemsKey: "anomalies", itemsAttr: "anomalies", item: listItemWithID, kind: listRowCountPage, countRequired: true},
	"assets":           {newDataSource: NewAssetsDataSource, path: "/billing/v1/assets", itemsKey: "assets", itemsAttr: "assets", item: listItemWithID, kind: listRowCountPage},
	"budgets":          {newDataSource: NewBudgetsDataSource, path: "/analytics/v1/budgets", itemsKey: "budgets", itemsAttr: "budgets", item: listItemWithID, kind: listRowCountPage},
	"cloud_incidents":  {newDataSource: NewCloudIncidentsDataSource, path: "/core/v1/cloudincidents", itemsKey: "incidents", itemsAttr: "incidents", item: listItemWithID, kind: listRowCountPage, singlePageOnly: true},
	"commitments":      {newDataSource: NewCommitmentsDataSource, path: "/analytics/v1/commitment-manager", itemsKey: "commitments", itemsAttr: "commitments", item: listItemWithID, kind: listRowCountPage},
	"composer_recipes": {newDataSource: NewComposerRecipesDataSource, path: "/insights/v2/recipes", itemsKey: "items", itemsAttr: "items", item: listItemWithNumericID, kind: listRowCountTotal, countRequired: true},
	"composer_recipe_results": {
		newDataSource: NewComposerRecipeResultsDataSource, path: "/insights/v2/recipes/1/results", itemsKey: "items", itemsAttr: "items", item: listItemWithNumericID,
		config: map[string]tftypes.Value{"recipe_id": tftypes.NewValue(tftypes.Number, 1)}, kind: listRowCountTotal, countRequired: true,
	},
	"custom_metrics":         {newDataSource: NewCustomMetricsDataSource, path: "/analytics/v1/custom-metrics", itemsKey: "items", itemsAttr: "items", item: listItemWithID, kind: listRowCountTotal},
	"customer_subscriptions": {newDataSource: NewCustomerSubscriptionsDataSource, path: "/customers/v1/subscriptions", itemsKey: "items", itemsAttr: "items", item: listItemWithID, kind: listRowCountTotal, countRequired: true},
	"dimensions":             {newDataSource: NewDimensionsDataSource, path: "/analytics/v1/dimensions", itemsKey: "dimensions", itemsAttr: "dimensions", item: listItemWithID, kind: listRowCountPage},
	"extended_metrics": {
		newDataSource: NewExtendedMetricsDataSource, path: "/analytics/v1/extended-metrics", itemsKey: "items", itemsAttr: "items",
		item: func(i int) string { return fmt.Sprintf(`{"key":"k%d","label":"L%d","dataSource":"billing"}`, i, i) }, kind: listRowCountTotal,
	},
	"folders":                {newDataSource: NewFoldersDataSource, path: "/analytics/v1/folders", itemsKey: "folders", itemsAttr: "folders", item: listItemWithID, kind: listRowCountPage},
	"invoices":               {newDataSource: NewInvoicesDataSource, path: "/billing/v1/invoices", itemsKey: "invoices", itemsAttr: "invoices", item: listItemWithID, kind: listRowCountPage},
	"labels":                 {newDataSource: NewLabelsDataSource, path: "/analytics/v1/labels", itemsKey: "labels", itemsAttr: "labels", item: listItemWithID, kind: listRowCountPage},
	"ps4c_aws_organizations": {newDataSource: NewPs4cAwsOrganizationsDataSource, path: "/ps4commitments/v1/aws/organizations", itemsKey: "items", itemsAttr: "items", item: listItemWithID, kind: listRowCountTotal},
	"ps4c_commitment_policies": {
		newDataSource: NewPs4cCommitmentPoliciesDataSource, path: "/ps4commitments/v1/general/policies", itemsKey: "items", itemsAttr: "items", item: listItemWithID,
		kind: listRowCountTotal, countRequired: true,
	},
	"ps4c_gcp_billing_accounts": {newDataSource: NewPs4cGcpBillingAccountsDataSource, path: "/ps4commitments/v1/gcp/billing-accounts", itemsKey: "items", itemsAttr: "items", item: listItemWithID, kind: listRowCountTotal},
	"ps4c_gcp_planned_purchases": {
		newDataSource: NewPs4cGcpPlannedPurchasesDataSource, path: "/ps4commitments/v1/gcp/billing-accounts/ba1/planned-purchases", itemsKey: "items", itemsAttr: "items", item: listItemWithID,
		config: map[string]tftypes.Value{"billing_account_id": tftypes.NewValue(tftypes.String, "ba1")}, kind: listRowCountPage, countRequired: true,
	},
	"ps4c_gcp_resource_cuds": {
		newDataSource: NewPs4cGcpResourceCudsDataSource, path: "/ps4commitments/v1/gcp/billing-accounts/ba1/resource-based", itemsKey: "items", itemsAttr: "items", item: listItemWithID,
		config: map[string]tftypes.Value{"billing_account_id": tftypes.NewValue(tftypes.String, "ba1")}, kind: listRowCountTotal,
	},
	"ps4c_gcp_settings": {newDataSource: NewPs4cGcpSettingsDataSource, path: "/ps4commitments/v1/gcp/settings", itemsKey: "items", itemsAttr: "items", item: listItemWithID, kind: listRowCountPage},
	"ps4c_gcp_spend_cuds": {
		newDataSource: NewPs4cGcpSpendCudsDataSource, path: "/ps4commitments/v1/gcp/billing-accounts/ba1/spend-based", itemsKey: "items", itemsAttr: "items", item: listItemWithID,
		config: map[string]tftypes.Value{"billing_account_id": tftypes.NewValue(tftypes.String, "ba1")}, kind: listRowCountTotal,
	},
	"reports":          {newDataSource: NewReportsDataSource, path: "/analytics/v1/reports", itemsKey: "reports", itemsAttr: "reports", item: listItemWithID, kind: listRowCountPage},
	"service_quotas":   {newDataSource: NewServiceQuotasDataSource, path: "/core/v1/service-quotas", itemsKey: "items", itemsAttr: "items", item: listItemWithID, kind: listRowCountTotal, countRequired: true},
	"support_requests": {newDataSource: NewSupportRequestsDataSource, path: "/support/v1/tickets", itemsKey: "tickets", itemsAttr: "tickets", item: listItemWithNumericID, kind: listRowCountPage},
}

// listPage is one page the fake server returns. rowCount is the JSON of the
// rowCount field ("" omits it).
type listPage struct {
	items    int
	rowCount string
}

// listFakeServer serves pages keyed by the incoming pageToken: "" is the first
// page, "t1" the second, and so on. It records each request's query.
//
// A numeric rowCount grows by one with every request after the first, so only
// the count of the read's first response gives the expected row_count. A
// data source that takes the count from a later page fails.
type listFakeServer struct {
	*httptest.Server
	mu       sync.Mutex
	requests []string
}

func newListFakeServer(t *testing.T, c listDataSourceCase, pages []listPage) *listFakeServer {
	t.Helper()
	s := &listFakeServer{}
	s.Server = httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.requests = append(s.requests, r.URL.RawQuery)
		request := len(s.requests) - 1
		s.mu.Unlock()
		if r.URL.Path != c.path {
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
			return
		}
		index := 0
		if token := r.URL.Query().Get("pageToken"); token != "" {
			n, err := strconv.Atoi(strings.TrimPrefix(token, "t"))
			if err != nil || n >= len(pages) {
				http.Error(w, "unexpected pageToken "+token, http.StatusNotFound)
				return
			}
			index = n
		}
		first := 0
		for _, p := range pages[:index] {
			first += p.items
		}
		items := make([]string, 0, pages[index].items)
		for i := range pages[index].items {
			items = append(items, c.item(first+i))
		}
		body := fmt.Sprintf(`{%q:[%s]`, c.itemsKey, strings.Join(items, ","))
		if index+1 < len(pages) {
			body += fmt.Sprintf(`,"pageToken":"t%d"`, index+1)
		}
		if rowCount := pages[index].rowCount; rowCount != "" {
			if n, err := strconv.Atoi(rowCount); err == nil {
				rowCount = strconv.Itoa(n + request)
			}
			body += `,"rowCount":` + rowCount
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body + "}"))
	}))
	return s
}

func (s *listFakeServer) requestCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.requests)
}

// readListDataSource reads the data source with config on top of an all-null
// configuration and returns the state's attributes.
func readListDataSource(t *testing.T, c listDataSourceCase, server *listFakeServer, config map[string]tftypes.Value) (map[string]tftypes.Value, schema.Schema) {
	t.Helper()
	ctx := t.Context()
	httpClient := server.Client()
	client, err := newClientWithHTTPClient(server.URL, "token", "", "test", "test", httpClient)
	if err != nil {
		t.Fatal(err)
	}
	ds := c.newDataSource()
	configurable, ok := ds.(datasource.DataSourceWithConfigure)
	if !ok {
		t.Fatal("data source does not implement Configure")
	}
	var configureResp datasource.ConfigureResponse
	configurable.Configure(ctx, datasource.ConfigureRequest{ProviderData: client}, &configureResp)
	if configureResp.Diagnostics.HasError() {
		t.Fatal(configureResp.Diagnostics)
	}

	s := listDataSourceSchema(ctx, t, ds)
	attrTypes := map[string]tftypes.Type{}
	values := map[string]tftypes.Value{}
	for name, attribute := range s.Attributes {
		attrTypes[name] = attribute.GetType().TerraformType(ctx)
		values[name] = tftypes.NewValue(attrTypes[name], nil)
	}
	maps.Copy(values, c.config)
	maps.Copy(values, config)
	objectType := tftypes.Object{AttributeTypes: attrTypes}
	resp := datasource.ReadResponse{State: tfsdk.State{Schema: s, Raw: tftypes.NewValue(objectType, nil)}}
	ds.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: s, Raw: tftypes.NewValue(objectType, values)}}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("read failed: %v", resp.Diagnostics)
	}
	var state map[string]tftypes.Value
	if err := resp.State.Raw.As(&state); err != nil {
		t.Fatal(err)
	}
	return state, s
}

func listDataSourceSchema(ctx context.Context, t *testing.T, ds datasource.DataSource) schema.Schema {
	t.Helper()
	var resp datasource.SchemaResponse
	ds.Schema(ctx, datasource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	return resp.Schema
}

func tfNumber(n int64) tftypes.Value { return tftypes.NewValue(tftypes.Number, n) }

// listInt64 returns the number in v, or -1 when v is null.
func listInt64(t *testing.T, v tftypes.Value) int64 {
	t.Helper()
	if v.IsNull() {
		return -1
	}
	var n big.Float
	if err := v.As(&n); err != nil {
		t.Fatal(err)
	}
	i, _ := n.Int64()
	return i
}

func listLen(t *testing.T, v tftypes.Value) int {
	t.Helper()
	var elems []tftypes.Value
	if err := v.As(&elems); err != nil {
		t.Fatal(err)
	}
	return len(elems)
}

// listRowCountJSON is the rowCount the fake server sends on every page: the
// total for total kinds (deliberately not the number of items served, so the
// tests can tell the two apart), the page size for page kinds.
func listRowCountJSON(c listDataSourceCase, pageItems int) string {
	if c.kind == listRowCountPage {
		return strconv.Itoa(pageItems)
	}
	return "10"
}

func TestListDataSourcePaginationCoverage(t *testing.T) {
	ctx := t.Context()
	p := &doitProvider{}
	for _, newDataSource := range p.DataSources(ctx) {
		ds := newDataSource()
		var metadata datasource.MetadataResponse
		ds.Metadata(ctx, datasource.MetadataRequest{ProviderTypeName: "doit"}, &metadata)
		s := listDataSourceSchema(ctx, t, ds)
		_, hasMaxResults := s.Attributes["max_results"]
		_, hasPageToken := s.Attributes["page_token"]
		name := strings.TrimPrefix(metadata.TypeName, "doit_")
		if !hasMaxResults || !hasPageToken {
			continue
		}
		if _, ok := listDataSourceCases[name]; !ok {
			t.Errorf("%s has max_results and page_token but no entry in listDataSourceCases", metadata.TypeName)
		}
	}
}

func TestListDataSourceSchemaDescriptions(t *testing.T) {
	for name, c := range listDataSourceCases {
		t.Run(name, func(t *testing.T) {
			s := listDataSourceSchema(t.Context(), t, c.newDataSource())
			want := schema.Schema{Attributes: map[string]schema.Attribute{
				"max_results": schema.Int64Attribute{},
				"page_token":  schema.StringAttribute{},
				"row_count":   schema.Int64Attribute{},
			}}
			setDescriptions := setListPaginationDescriptions
			if c.singlePageOnly {
				setDescriptions = setSinglePageListPaginationDescriptions
			}
			if diags := setDescriptions(&want, c.kind, ""); diags.HasError() {
				t.Fatal(diags)
			}
			for _, attribute := range []string{"max_results", "page_token", "row_count"} {
				// A page_token note for one endpoint may follow the shared text.
				got, wantDescription := s.Attributes[attribute].GetDescription(), want.Attributes[attribute].GetDescription()
				if got != wantDescription && (attribute != "page_token" || !strings.HasPrefix(got, wantDescription+" ")) {
					t.Errorf("%s description = %q, want %q", attribute, got, wantDescription)
				}
				if markdown := s.Attributes[attribute].GetMarkdownDescription(); markdown != got {
					t.Errorf("%s markdown description = %q, want %q", attribute, markdown, got)
				}
			}
		})
	}
}

// TestListDataSourceUnknownConfig checks the defensive guard for configuration
// that is not fully known: no request, every result unknown, and every input,
// max_results included, as configured. page_token is a result in every mode,
// because a real read replaces it with the next page's token or null.
func TestListDataSourceUnknownConfig(t *testing.T) {
	maxResultsValues := map[string]*tftypes.Value{"omitted": nil, "known": new(tfNumber(2)), "unknown": new(tftypes.NewValue(tftypes.Number, tftypes.UnknownValue))}
	pageTokenValues := map[string]*tftypes.Value{"omitted": nil, "known": new(tfString("t1"))}

	for name, c := range listDataSourceCases {
		for maxResultsName, maxResults := range maxResultsValues {
			for pageTokenName, pageToken := range pageTokenValues {
				t.Run(fmt.Sprintf("%s/max_results_%s/page_token_%s", name, maxResultsName, pageTokenName), func(t *testing.T) {
					// Pages exist so that a guard that reads anyway fails on the request
					// count rather than on the response.
					server := newListFakeServer(t, c, []listPage{{items: 1}, {items: 1}})
					ctx := t.Context()
					s := listDataSourceSchema(ctx, t, c.newDataSource())
					timeoutsType := s.Attributes["timeouts"].GetType().TerraformType(ctx)
					config := map[string]tftypes.Value{"timeouts": tftypes.NewValue(timeoutsType, tftypes.UnknownValue)}
					if maxResults != nil {
						config["max_results"] = *maxResults
					}
					if pageToken != nil {
						config["page_token"] = *pageToken
					}

					state, _ := readListDataSource(t, c, server, config)

					if n := server.requestCount(); n != 0 {
						t.Errorf("made %d requests, want none", n)
					}
					expected := map[string]tftypes.Value{}
					for attrName, attribute := range s.Attributes {
						attrType := attribute.GetType().TerraformType(ctx)
						switch {
						case attrName == "page_token" || !attribute.IsOptional() && !attribute.IsRequired():
							expected[attrName] = tftypes.NewValue(attrType, tftypes.UnknownValue)
						case config[attrName].Type() != nil:
							expected[attrName] = config[attrName]
						case c.config[attrName].Type() != nil:
							expected[attrName] = c.config[attrName]
						default:
							expected[attrName] = tftypes.NewValue(attrType, nil)
						}
					}
					for _, attrName := range slices.Sorted(maps.Keys(expected)) {
						if !state[attrName].Equal(expected[attrName]) {
							t.Errorf("%s = %v, want %v", attrName, state[attrName], expected[attrName])
						}
					}
				})
			}
		}
	}
}

func TestListDataSourceRead(t *testing.T) {
	type expectation struct {
		requests  int
		items     int
		rowCount  int64 // -1 means null
		pageToken string
	}
	rows := []struct {
		name   string
		config map[string]tftypes.Value
		pages  func(c listDataSourceCase) []listPage
		// skip reports why a row does not apply to a case.
		skip   func(c listDataSourceCase) string
		expect func(c listDataSourceCase) expectation
	}{
		{
			name: "auto mode reads every page, row_count omitted",
			pages: func(listDataSourceCase) []listPage {
				return []listPage{{items: 2}, {items: 1}, {items: 1}}
			},
			skip: func(c listDataSourceCase) string {
				if c.countRequired {
					return "the API always sends rowCount"
				}
				return ""
			},
			expect: func(listDataSourceCase) expectation { return expectation{requests: 3, items: 4, rowCount: 4} },
		},
		{
			name: "auto mode reads every page, row_count null",
			pages: func(listDataSourceCase) []listPage {
				return []listPage{{items: 2, rowCount: "null"}, {items: 1, rowCount: "null"}, {items: 1, rowCount: "null"}}
			},
			skip: func(c listDataSourceCase) string {
				if c.countRequired {
					return "the API always sends rowCount"
				}
				return ""
			},
			expect: func(listDataSourceCase) expectation { return expectation{requests: 3, items: 4, rowCount: 4} },
		},
		{
			name: "auto mode reads every page, row_count from the API",
			pages: func(c listDataSourceCase) []listPage {
				return []listPage{{2, listRowCountJSON(c, 2)}, {1, listRowCountJSON(c, 1)}, {1, listRowCountJSON(c, 1)}}
			},
			expect: func(c listDataSourceCase) expectation {
				if c.kind == listRowCountTotal {
					return expectation{requests: 3, items: 4, rowCount: 10}
				}
				return expectation{requests: 3, items: 4, rowCount: 4}
			},
		},
		{
			name:   "auto mode from a page_token reads the remaining pages",
			config: map[string]tftypes.Value{"page_token": tfString("t1")},
			pages: func(c listDataSourceCase) []listPage {
				return []listPage{{2, listRowCountJSON(c, 2)}, {1, listRowCountJSON(c, 1)}, {1, listRowCountJSON(c, 1)}}
			},
			expect: func(c listDataSourceCase) expectation {
				if c.kind == listRowCountTotal {
					return expectation{requests: 2, items: 2, rowCount: 10}
				}
				return expectation{requests: 2, items: 2, rowCount: 2}
			},
		},
		{
			name: "auto mode continues past empty pages",
			pages: func(c listDataSourceCase) []listPage {
				return []listPage{{0, listRowCountJSON(c, 0)}, {0, listRowCountJSON(c, 0)}, {3, listRowCountJSON(c, 3)}}
			},
			expect: func(c listDataSourceCase) expectation {
				if c.kind == listRowCountTotal {
					return expectation{requests: 3, items: 3, rowCount: 10}
				}
				return expectation{requests: 3, items: 3, rowCount: 3}
			},
		},
		{
			name:   "max_results reads one page and returns the next token",
			config: map[string]tftypes.Value{"max_results": tfNumber(2)},
			pages: func(c listDataSourceCase) []listPage {
				return []listPage{{2, listRowCountJSON(c, 2)}, {1, listRowCountJSON(c, 1)}, {1, listRowCountJSON(c, 1)}}
			},
			expect: func(c listDataSourceCase) expectation {
				if c.kind == listRowCountPage {
					return expectation{requests: 1, items: 2, rowCount: 2, pageToken: "t1"}
				}
				return expectation{requests: 1, items: 2, rowCount: 10, pageToken: "t1"}
			},
		},
		{
			name:   "max_results on the last page returns no token",
			config: map[string]tftypes.Value{"max_results": tfNumber(2), "page_token": tfString("t2")},
			pages: func(c listDataSourceCase) []listPage {
				return []listPage{{2, listRowCountJSON(c, 2)}, {1, listRowCountJSON(c, 1)}, {1, listRowCountJSON(c, 1)}}
			},
			expect: func(c listDataSourceCase) expectation {
				if c.kind == listRowCountPage {
					return expectation{requests: 1, items: 1, rowCount: 1}
				}
				return expectation{requests: 1, items: 1, rowCount: 10}
			},
		},
		{
			name:   "max_results without a count from the API",
			config: map[string]tftypes.Value{"max_results": tfNumber(2)},
			pages: func(listDataSourceCase) []listPage {
				return []listPage{{items: 2}, {items: 1}}
			},
			skip: func(c listDataSourceCase) string {
				if c.countRequired {
					return "the API always sends rowCount"
				}
				return ""
			},
			expect: func(c listDataSourceCase) expectation {
				if c.kind == listRowCountTotalUnlessPaged {
					return expectation{requests: 1, items: 2, rowCount: -1, pageToken: "t1"}
				}
				return expectation{requests: 1, items: 2, rowCount: 2, pageToken: "t1"}
			},
		},
	}

	for name, c := range listDataSourceCases {
		for _, row := range rows {
			t.Run(name+"/"+row.name, func(t *testing.T) {
				if row.skip != nil {
					if reason := row.skip(c); reason != "" {
						t.Skip(reason)
					}
				}
				_, singlePage := row.config["max_results"]
				if c.singlePageOnly && !singlePage {
					t.Skip("the data source reads a single page")
				}
				server := newListFakeServer(t, c, row.pages(c))
				want := row.expect(c)

				state, _ := readListDataSource(t, c, server, row.config)

				if got := server.requestCount(); got != want.requests {
					t.Errorf("made %d requests, want %d", got, want.requests)
				}
				if got := listLen(t, state[c.itemsAttr]); got != want.items {
					t.Errorf("%s has %d items, want %d", c.itemsAttr, got, want.items)
				}
				if got := listInt64(t, state["row_count"]); got != want.rowCount {
					t.Errorf("row_count = %d, want %d (-1 is null)", got, want.rowCount)
				}
				// Without a next page, page_token must be null, not "": a
				// configuration that chains on page_token != null would
				// otherwise read the first page again.
				if wantPageToken := tftypes.NewValue(tftypes.String, nil); want.pageToken == "" {
					if !state["page_token"].Equal(wantPageToken) {
						t.Errorf("page_token = %v, want null", state["page_token"])
					}
				} else if wantPageToken = tfString(want.pageToken); !state["page_token"].Equal(wantPageToken) {
					t.Errorf("page_token = %v, want %v", state["page_token"], wantPageToken)
				}
				if _, configured := row.config["max_results"]; !configured && !state["max_results"].IsNull() {
					t.Errorf("max_results = %v, want null", state["max_results"])
				}
			})
		}
	}
}
