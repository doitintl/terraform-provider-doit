package provider

import (
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_customer_subscription"
	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_customer_subscriptions"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// recordingServer answers every request through handler and records the query
// of each one. It uses the in-memory test network: only a client built from
// server.Client() can reach it, so the recorded calls are complete.
type recordingServer struct {
	*httptest.Server
	mu      sync.Mutex
	queries []url.Values
	paths   []string
}

func newRecordingServer(t *testing.T, handler func(w http.ResponseWriter, q url.Values)) *recordingServer {
	t.Helper()
	rs := &recordingServer{}
	rs.Server = httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rs.mu.Lock()
		rs.queries = append(rs.queries, r.URL.Query())
		rs.paths = append(rs.paths, r.URL.Path)
		rs.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		handler(w, r.URL.Query())
	}))
	return rs
}

func (rs *recordingServer) calls() []url.Values {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return append([]url.Values(nil), rs.queries...)
}

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

// readCustomerSubscriptionDataSourceForTest runs Read on the singular data
// source when plural is false, and on the plural one otherwise. Every
// attribute is null unless config overrides it.
func readCustomerSubscriptionDataSourceForTest(t *testing.T, server *recordingServer, plural bool, config map[string]tftypes.Value) datasource.ReadResponse {
	t.Helper()
	httpClient := server.Client()
	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}
	var ds datasource.DataSource
	if plural {
		ds = &customerSubscriptionsDataSource{client: client}
	} else {
		ds = &customerSubscriptionDataSource{client: client}
	}
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
	return resp
}

func subscriptionIDConfig(id string) map[string]tftypes.Value {
	return map[string]tftypes.Value{"id": tftypes.NewValue(tftypes.String, id)}
}

const customerSubscriptionScheduledJSON = `{
	"id": "sub-1", "name": "Monthly costs (Report Subscription)", "createdBy": "owner@example.com",
	"etag": "etag-1",
	"recipients": {"emails": ["finops@example.com"], "slackChannels": ["C0123ABCD"]},
	"subscriptions": [
		{"type": "dashboard-subscription",
		 "source": {"id": "dash-1", "name": "Executive costs"},
		 "schedule": {"timeZone": "Europe/Warsaw", "startDate": "2026-10-23T00:00:00Z",
		              "time": "2026-10-23T08:00:00Z", "frequency": "Weekly", "nextAt": "2026-10-30T08:00:00Z"},
		 "permissions": {"canEditRecipients": true, "canDelete": true, "canUnsubscribe": false}},
		{"type": "report-subscription",
		 "source": {"id": "rep-1", "name": "Monthly costs"},
		 "schedule": {"timeZone": "Europe/Warsaw", "startDate": "2026-11-01T09:00:00.5+01:00",
		              "time": "2026-11-01T08:00:00Z", "frequency": "Custom",
		              "customPeriodAmount": 2, "customPeriodUnit": "week", "nextAt": "2026-11-15T08:00:00Z"},
		 "attachments": ["pdf", "csv"],
		 "pdfSettings": {"includeAnnotations": false, "includeTable": true, "showFilters": true},
		 "permissions": {"canEditRecipients": true, "canDelete": false, "canUnsubscribe": true}}
	]
}`

// customerSubscriptionAlertJSON is an alert: no schedule, attachments or PDF
// settings, and no recipients on Slack.
const customerSubscriptionAlertJSON = `{
	"id": "alert-1", "name": "Production spend alert", "createdBy": "owner@example.com",
	"etag": "etag-2",
	"recipients": {"emails": ["finops@example.com"], "slackChannels": []},
	"subscriptions": [
		{"type": "alert", "source": {"id": "alert-1", "name": "Production spend alert"},
		 "permissions": {"canEditRecipients": true, "canDelete": false, "canUnsubscribe": false}}
	]
}`

func subscriptionEntries(t *testing.T, set types.Set) map[string]datasource_customer_subscription.SubscriptionsValue {
	t.Helper()
	var entries []datasource_customer_subscription.SubscriptionsValue
	if diags := set.ElementsAs(t.Context(), &entries, false); diags.HasError() {
		t.Fatal(diags)
	}
	byType := map[string]datasource_customer_subscription.SubscriptionsValue{}
	for _, e := range entries {
		byType[e.SubscriptionsType.ValueString()] = e
	}
	return byType
}

func TestCustomerSubscriptionDataSourceUnknownID(t *testing.T) {
	server := newRecordingServer(t, func(w http.ResponseWriter, _ url.Values) { writeJSON(w, 500, "{}") })
	config := map[string]tftypes.Value{"id": tftypes.NewValue(tftypes.String, tftypes.UnknownValue)}
	resp := readCustomerSubscriptionDataSourceForTest(t, server, false, config)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if n := len(server.calls()); n != 0 {
		t.Fatalf("made %d API requests for unknown input", n)
	}
	var data customerSubscriptionDataSourceModel
	if diags := resp.State.Get(t.Context(), &data); diags.HasError() {
		t.Fatal(diags)
	}
	if !data.Name.IsUnknown() || !data.Etag.IsUnknown() || !data.CreatedBy.IsUnknown() ||
		!data.Recipients.IsUnknown() || !data.Subscriptions.IsUnknown() {
		t.Fatalf("outputs must remain unknown: %+v", data)
	}
}

func TestCustomerSubscriptionDataSourceRead(t *testing.T) {
	server := newRecordingServer(t, func(w http.ResponseWriter, _ url.Values) { writeJSON(w, 200, customerSubscriptionScheduledJSON) })
	resp := readCustomerSubscriptionDataSourceForTest(t, server, false, subscriptionIDConfig("sub-1"))
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if got := server.paths[0]; !strings.HasSuffix(got, "/customers/v1/subscriptions/sub-1") {
		t.Fatalf("path = %q", got)
	}

	var data customerSubscriptionDataSourceModel
	if diags := resp.State.Get(t.Context(), &data); diags.HasError() {
		t.Fatal(diags)
	}
	if data.Id.ValueString() != "sub-1" || data.Name.ValueString() != "Monthly costs (Report Subscription)" ||
		data.Etag.ValueString() != "etag-1" || data.CreatedBy.ValueString() != "owner@example.com" {
		t.Fatalf("unexpected scalars: %+v", data)
	}
	if got := data.Recipients.Emails.String(); got != `["finops@example.com"]` {
		t.Errorf("emails = %s", got)
	}
	if got := data.Recipients.SlackChannels.String(); got != `["C0123ABCD"]` {
		t.Errorf("slack_channels = %s", got)
	}

	entries := subscriptionEntries(t, data.Subscriptions)
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}

	dashboard := entries["dashboard-subscription"]
	if dashboard.Source.Id.ValueString() != "dash-1" || dashboard.Source.Name.ValueString() != "Executive costs" {
		t.Errorf("dashboard source = %+v", dashboard.Source)
	}
	if dashboard.Schedule.Frequency.ValueString() != "Weekly" || dashboard.Schedule.NextAt.ValueString() != "2026-10-30T08:00:00Z" {
		t.Errorf("dashboard schedule = %+v", dashboard.Schedule)
	}
	if !dashboard.Schedule.CustomPeriodAmount.IsNull() || !dashboard.Schedule.CustomPeriodUnit.IsNull() {
		t.Errorf("custom period must be null for Weekly: %+v", dashboard.Schedule)
	}
	if dashboard.Attachments.IsNull() || len(dashboard.Attachments.Elements()) != 0 {
		t.Errorf("attachments of a dashboard must be [], got %s", dashboard.Attachments)
	}
	if !dashboard.PdfSettings.IsNull() {
		t.Errorf("pdf_settings of a dashboard must be null, got %s", dashboard.PdfSettings)
	}
	if !dashboard.Permissions.CanDelete.ValueBool() || dashboard.Permissions.CanUnsubscribe.ValueBool() {
		t.Errorf("dashboard permissions = %+v", dashboard.Permissions)
	}

	report := entries["report-subscription"]
	// The API's offset spelling is normalized to UTC and keeps the fraction.
	if got := report.Schedule.StartDate.ValueString(); got != "2026-11-01T08:00:00.5Z" {
		t.Errorf("start_date = %q", got)
	}
	if report.Schedule.CustomPeriodAmount.ValueInt64() != 2 || report.Schedule.CustomPeriodUnit.ValueString() != "week" {
		t.Errorf("custom period = %+v", report.Schedule)
	}
	if got := report.Attachments.String(); got != `["pdf","csv"]` {
		t.Errorf("attachments = %s", got)
	}
	if report.PdfSettings.IsNull() || report.PdfSettings.IncludeAnnotations.ValueBool() ||
		!report.PdfSettings.IncludeTable.ValueBool() || !report.PdfSettings.ShowFilters.ValueBool() {
		t.Errorf("pdf_settings = %+v", report.PdfSettings)
	}
}

func TestCustomerSubscriptionDataSourceReadAlert(t *testing.T) {
	server := newRecordingServer(t, func(w http.ResponseWriter, _ url.Values) { writeJSON(w, 200, customerSubscriptionAlertJSON) })
	resp := readCustomerSubscriptionDataSourceForTest(t, server, false, subscriptionIDConfig("alert-1"))
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	var data customerSubscriptionDataSourceModel
	if diags := resp.State.Get(t.Context(), &data); diags.HasError() {
		t.Fatal(diags)
	}
	if got := data.Recipients.SlackChannels; got.IsNull() || len(got.Elements()) != 0 {
		t.Errorf("slack_channels must be [], got %s", got)
	}
	alert, ok := subscriptionEntries(t, data.Subscriptions)["alert"]
	if !ok {
		t.Fatalf("no alert entry in %s", data.Subscriptions)
	}
	if !alert.Schedule.IsNull() || !alert.PdfSettings.IsNull() {
		t.Errorf("alert must have no schedule or PDF settings: %+v", alert)
	}
	if alert.Attachments.IsNull() || len(alert.Attachments.Elements()) != 0 {
		t.Errorf("attachments must be [], got %s", alert.Attachments)
	}
	if alert.Permissions.IsNull() || !alert.Permissions.CanEditRecipients.ValueBool() {
		t.Errorf("permissions = %+v", alert.Permissions)
	}
}

func TestCustomerSubscriptionDataSourceReadErrors(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr string
	}{
		{"not found", 404, `{"error":"not found"}`, "status 404"},
		{"server error", 500, `{}`, "status 500"},
		{"empty body", 200, `null`, `API returned ID ""`},
		{"id mismatch", 200, customerSubscriptionAlertJSON, `API returned ID "alert-1"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newRecordingServer(t, func(w http.ResponseWriter, _ url.Values) { writeJSON(w, tt.status, tt.body) })
			resp := readCustomerSubscriptionDataSourceForTest(t, server, false, subscriptionIDConfig("sub-1"))
			if !resp.Diagnostics.HasError() {
				t.Fatal("expected an error")
			}
			if got := fmt.Sprint(resp.Diagnostics); !strings.Contains(got, tt.wantErr) {
				t.Fatalf("diagnostics %q do not contain %q", got, tt.wantErr)
			}
		})
	}
}

// listPageJSON builds a list response holding n alerts.
func listPageJSON(ids []string, pageToken string, rowCount int) string {
	items := make([]string, len(ids))
	for i, id := range ids {
		items[i] = strings.ReplaceAll(customerSubscriptionAlertJSON, "alert-1", id)
	}
	token := ""
	if pageToken != "" {
		token = fmt.Sprintf(`"pageToken": %q,`, pageToken)
	}
	return fmt.Sprintf(`{"items": [%s], %s "rowCount": %d}`, strings.Join(items, ","), token, rowCount)
}

// pagedListHandler serves three pages of one item each, keyed by pageToken.
func pagedListHandler(w http.ResponseWriter, q url.Values) {
	switch q.Get("pageToken") {
	case "":
		writeJSON(w, 200, listPageJSON([]string{"a1"}, "p2", 3))
	case "p2":
		writeJSON(w, 200, listPageJSON([]string{"a2"}, "p3", 3))
	case "p3":
		writeJSON(w, 200, listPageJSON([]string{"a3"}, "", 3))
	default:
		writeJSON(w, 400, `{"error":"bad token"}`)
	}
}

func itemIDs(t *testing.T, data customerSubscriptionsDataSourceModel) []string {
	t.Helper()
	var items []datasource_customer_subscriptions.ItemsValue
	if diags := data.Items.ElementsAs(t.Context(), &items, false); diags.HasError() {
		t.Fatal(diags)
	}
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.Id.ValueString()
	}
	return ids
}

func readPluralForTest(t *testing.T, server *recordingServer, config map[string]tftypes.Value) (customerSubscriptionsDataSourceModel, datasource.ReadResponse) {
	t.Helper()
	resp := readCustomerSubscriptionDataSourceForTest(t, server, true, config)
	var data customerSubscriptionsDataSourceModel
	if !resp.Diagnostics.HasError() {
		if diags := resp.State.Get(t.Context(), &data); diags.HasError() {
			t.Fatal(diags)
		}
	}
	return data, resp
}

func TestCustomerSubscriptionsDataSourceAutoPagination(t *testing.T) {
	server := newRecordingServer(t, pagedListHandler)
	data, resp := readPluralForTest(t, server, nil)
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
	}
	if got := strings.Join(itemIDs(t, data), ","); got != "a1,a2,a3" {
		t.Errorf("items = %s", got)
	}
	if data.RowCount.ValueInt64() != 3 {
		t.Errorf("row_count = %v", data.RowCount)
	}
	if !data.MaxResults.IsNull() || !data.PageToken.IsNull() {
		t.Errorf("max_results and page_token must stay null: %v %v", data.MaxResults, data.PageToken)
	}
}

func TestCustomerSubscriptionsDataSourceMaxResultsIsSinglePage(t *testing.T) {
	server := newRecordingServer(t, pagedListHandler)
	config := map[string]tftypes.Value{"max_results": tftypes.NewValue(tftypes.Number, 1)}
	data, resp := readPluralForTest(t, server, config)
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
	if got := strings.Join(itemIDs(t, data), ","); got != "a1" {
		t.Errorf("items = %s", got)
	}
	if data.MaxResults.ValueInt64() != 1 || data.PageToken.ValueString() != "p2" {
		t.Errorf("max_results/page_token = %v / %v", data.MaxResults, data.PageToken)
	}
	// row_count is the total across pages, not the size of this page.
	if data.RowCount.ValueInt64() != 3 {
		t.Errorf("row_count = %v", data.RowCount)
	}
}

func TestCustomerSubscriptionsDataSourcePageTokenStartsAutoPagination(t *testing.T) {
	server := newRecordingServer(t, pagedListHandler)
	config := map[string]tftypes.Value{"page_token": tftypes.NewValue(tftypes.String, "p2")}
	data, resp := readPluralForTest(t, server, config)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}

	if n := len(server.calls()); n != 2 {
		t.Fatalf("made %d requests, want 2", n)
	}
	if got := strings.Join(itemIDs(t, data), ","); got != "a2,a3" {
		t.Errorf("items = %s", got)
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

func TestCustomerSubscriptionsDataSourceEmpty(t *testing.T) {
	server := newRecordingServer(t, func(w http.ResponseWriter, _ url.Values) { writeJSON(w, 200, `{"items": [], "rowCount": 0}`) })
	data, resp := readPluralForTest(t, server, nil)
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

func TestCustomerSubscriptionsDataSourceMapsItems(t *testing.T) {
	server := newRecordingServer(t, func(w http.ResponseWriter, _ url.Values) {
		writeJSON(w, 200, fmt.Sprintf(`{"items": [%s, %s], "rowCount": 2}`, customerSubscriptionScheduledJSON, customerSubscriptionAlertJSON))
	})
	data, resp := readPluralForTest(t, server, nil)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	var items []datasource_customer_subscriptions.ItemsValue
	if diags := data.Items.ElementsAs(t.Context(), &items, false); diags.HasError() {
		t.Fatal(diags)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items", len(items))
	}
	scheduled := items[0]
	if scheduled.Id.ValueString() != "sub-1" || scheduled.CreatedBy.ValueString() != "owner@example.com" ||
		scheduled.Etag.ValueString() != "etag-1" || scheduled.Recipients.Emails.String() != `["finops@example.com"]` {
		t.Errorf("scheduled item = %+v", scheduled)
	}
	var entries []datasource_customer_subscriptions.SubscriptionsValue
	if diags := scheduled.Subscriptions.ElementsAs(t.Context(), &entries, false); diags.HasError() {
		t.Fatal(diags)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries", len(entries))
	}
	for _, e := range entries {
		if e.SubscriptionsType.ValueString() != "report-subscription" {
			continue
		}
		if e.Attachments.String() != `["pdf","csv"]` || e.PdfSettings.IsNull() ||
			e.Schedule.StartDate.ValueString() != "2026-11-01T08:00:00.5Z" || e.Schedule.CustomPeriodAmount.ValueInt64() != 2 {
			t.Errorf("report entry = %+v", e)
		}
	}

	var alertEntries []datasource_customer_subscriptions.SubscriptionsValue
	if diags := items[1].Subscriptions.ElementsAs(t.Context(), &alertEntries, false); diags.HasError() {
		t.Fatal(diags)
	}
	if len(alertEntries) != 1 || !alertEntries[0].Schedule.IsNull() || alertEntries[0].Attachments.IsNull() {
		t.Errorf("alert entries = %+v", alertEntries)
	}
}

func TestCustomerSubscriptionsDataSourceUnknownConfig(t *testing.T) {
	server := newRecordingServer(t, pagedListHandler)
	config := map[string]tftypes.Value{"page_token": tftypes.NewValue(tftypes.String, tftypes.UnknownValue)}
	resp := readCustomerSubscriptionDataSourceForTest(t, server, true, config)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if n := len(server.calls()); n != 0 {
		t.Fatalf("made %d API requests for unknown input", n)
	}
	var data customerSubscriptionsDataSourceModel
	if diags := resp.State.Get(t.Context(), &data); diags.HasError() {
		t.Fatal(diags)
	}
	if !data.Items.IsUnknown() || !data.RowCount.IsUnknown() {
		t.Fatalf("outputs must remain unknown: %+v", data)
	}
}

func TestCustomerSubscriptionsDataSourceErrors(t *testing.T) {
	t.Run("error on a later page fails the read", func(t *testing.T) {
		server := newRecordingServer(t, func(w http.ResponseWriter, q url.Values) {
			if q.Get("pageToken") == "" {
				writeJSON(w, 200, listPageJSON([]string{"a1"}, "p2", 2))
				return
			}
			writeJSON(w, 500, `{"error":"boom"}`)
		})
		resp := readCustomerSubscriptionDataSourceForTest(t, server, true, nil)
		if !resp.Diagnostics.HasError() || !strings.Contains(fmt.Sprint(resp.Diagnostics), "status 500") {
			t.Fatalf("diagnostics = %v", resp.Diagnostics)
		}
		if n := len(server.calls()); n != 2 {
			t.Errorf("made %d requests, want 2", n)
		}
	})
}
