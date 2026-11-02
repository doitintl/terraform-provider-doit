package provider

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_customer_notifications"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// readCustomerNotificationDataSourceForTest runs Read on the singular data
// source when id is non-nil, and on the plural data source otherwise.
func readCustomerNotificationDataSourceForTest(t *testing.T, server *httptest.Server, id *tftypes.Value) datasource.ReadResponse {
	t.Helper()
	httpClient := server.Client()
	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}
	var ds datasource.DataSource
	if id == nil {
		ds = &customerNotificationsDataSource{client: client}
	} else {
		ds = &customerNotificationDataSource{client: client}
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
	if id != nil {
		values["id"] = *id
	}
	resp := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	ds.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(tftypes.Object{AttributeTypes: attrTypes}, values)}}, &resp)
	return resp
}

func customerNotificationDataSourceTestServer(t *testing.T, status int, body string, gotPath *atomic.Value) *httptest.Server {
	t.Helper()
	return httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath.Store(r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

// customerNotificationFullJSON has every optional member set and all three
// destination kinds.
const customerNotificationFullJSON = `{
	"id": "notif-1", "name": "Anomaly alerts",
	"createTime": "2026-10-01T08:00:00Z", "updateTime": "2026-10-02T09:30:00Z", "etag": "etag-1",
	"destinations": {
		"emails": ["finops@example.com", "ops@example.com"],
		"slackChannels": ["C0123ABCD"],
		"teamsChannels": ["19:abc@thread.tacv2"]
	},
	"settings": {"costAnomalies": {
		"minSeverity": "critical",
		"minExcessCost": {"amount": "100.00", "currency": "USD"},
		"allocations": {"mode": "include", "ids": ["alloc-b", "alloc-a"]},
		"repeat": {"everyHours": 24, "whenCostGrows": {
			"match": "or", "percentIncrease": 10, "withinHours": 6,
			"minIncrease": {"amount": "150.00", "currency": "USD"}}}
	}}
}`

// customerNotificationMinimalJSON omits repeat, the unused destination
// arrays and the optional timestamps.
const customerNotificationMinimalJSON = `{
	"id": "notif-2", "name": "Minimal", "createTime": null, "updateTime": null, "etag": null,
	"destinations": {"emails": ["finops@example.com"]},
	"settings": {"costAnomalies": {
		"minSeverity": "warning",
		"minExcessCost": {"amount": "1000000.00", "currency": "USD"},
		"allocations": {"mode": "all", "ids": []}
	}}
}`

func TestCustomerNotificationDataSourceUnknownID(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewTestServer(t, http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { calls.Add(1) }))
	unknown := tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	resp := readCustomerNotificationDataSourceForTest(t, server, &unknown)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if calls.Load() != 0 {
		t.Fatalf("made %d API requests for unknown input", calls.Load())
	}
	var data customerNotificationDataSourceModel
	if diags := resp.State.Get(t.Context(), &data); diags.HasError() {
		t.Fatal(diags)
	}
	if !data.Name.IsUnknown() || !data.Etag.IsUnknown() || !data.CreateTime.IsUnknown() || !data.UpdateTime.IsUnknown() ||
		!data.Destinations.IsUnknown() || !data.Settings.IsUnknown() {
		t.Fatalf("outputs must remain unknown: %+v", data)
	}
}

func TestCustomerNotificationsDataSourceUnknownConfig(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewTestServer(t, http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { calls.Add(1) }))
	httpClient := server.Client()
	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}
	ds := &customerNotificationsDataSource{client: client}
	ctx := t.Context()
	var schemaResp datasource.SchemaResponse
	ds.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)

	// The only configurable attribute is timeouts; make it unknown.
	attrTypes := map[string]tftypes.Type{}
	values := map[string]tftypes.Value{}
	for name, attribute := range schemaResp.Schema.Attributes {
		tfType := attribute.GetType().TerraformType(ctx)
		attrTypes[name] = tfType
		if name == "timeouts" {
			values[name] = tftypes.NewValue(tfType, tftypes.UnknownValue)
		} else {
			values[name] = tftypes.NewValue(tfType, nil)
		}
	}
	resp := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	ds.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(tftypes.Object{AttributeTypes: attrTypes}, values)}}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if calls.Load() != 0 {
		t.Fatalf("made %d API requests for unknown input", calls.Load())
	}
	var data customerNotificationsDataSourceModel
	if diags := resp.State.Get(ctx, &data); diags.HasError() {
		t.Fatal(diags)
	}
	if !data.Items.IsUnknown() {
		t.Fatalf("items must remain unknown: %+v", data.Items)
	}
}

func TestCustomerNotificationDataSourceReadErrors(t *testing.T) {
	for _, tc := range []struct {
		name        string
		status      int
		contentType string
		body        string
		want        string
	}{
		{"not found", 404, "application/json", `{"error":"missing"}`, "status 404"},
		{"forbidden", 403, "application/json", `{"error":"no settings permission"}`, "status 403"},
		{"server error", 500, "application/json", `{"error":"boom"}`, "boom"},
		{"non-JSON", 200, "text/plain", "wrong content type", "status 200"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			id := tftypes.NewValue(tftypes.String, "notif-1")
			for name, notificationID := range map[string]*tftypes.Value{"singular": &id, "plural": nil} {
				resp := readCustomerNotificationDataSourceForTest(t, server, notificationID)
				if !resp.Diagnostics.HasError() || !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), tc.want) {
					t.Fatalf("%s: diagnostics = %v, want %q", name, resp.Diagnostics, tc.want)
				}
			}
		})
	}
}

func TestCustomerNotificationDataSourceRead(t *testing.T) {
	var gotPath atomic.Value
	server := customerNotificationDataSourceTestServer(t, 200, customerNotificationFullJSON, &gotPath)
	id := tftypes.NewValue(tftypes.String, "notif-1")
	resp := readCustomerNotificationDataSourceForTest(t, server, &id)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if want := "/customers/v1/notifications/notif-1"; gotPath.Load() != want {
		t.Fatalf("path = %v, want %s", gotPath.Load(), want)
	}

	var data customerNotificationDataSourceModel
	if diags := resp.State.Get(t.Context(), &data); diags.HasError() {
		t.Fatal(diags)
	}
	if data.Id.ValueString() != "notif-1" || data.Name.ValueString() != "Anomaly alerts" || data.Etag.ValueString() != "etag-1" ||
		data.CreateTime.ValueString() != "2026-10-01T08:00:00Z" || data.UpdateTime.ValueString() != "2026-10-02T09:30:00Z" {
		t.Fatalf("mapped state = %+v", data)
	}

	// The API order and spelling of the destinations is kept.
	dest := data.Destinations
	emails := dest.Emails.Elements()
	if len(emails) != 2 || emails[0].String() != `"finops@example.com"` || emails[1].String() != `"ops@example.com"` {
		t.Fatalf("emails = %v", emails)
	}
	if got := dest.SlackChannels.Elements(); len(got) != 1 || got[0].String() != `"C0123ABCD"` {
		t.Fatalf("slack_channels = %v", got)
	}
	if got := dest.TeamsChannels.Elements(); len(got) != 1 || got[0].String() != `"19:abc@thread.tacv2"` {
		t.Fatalf("teams_channels = %v", got)
	}

	ca := data.Settings.CostAnomalies
	if ca.MinSeverity.ValueString() != "critical" ||
		ca.MinExcessCost.Amount.ValueString() != "100.00" || ca.MinExcessCost.Currency.ValueString() != "USD" ||
		ca.Allocations.Mode.ValueString() != "include" {
		t.Fatalf("cost_anomalies = %+v", ca)
	}
	ids := ca.Allocations.Ids.Elements()
	if len(ids) != 2 || ids[0].String() != `"alloc-b"` || ids[1].String() != `"alloc-a"` {
		t.Fatalf("allocation ids = %v", ids)
	}
	wcg := ca.Repeat.WhenCostGrows
	if ca.Repeat.EveryHours.ValueInt64() != 24 || wcg.Match.ValueString() != "or" ||
		wcg.PercentIncrease.ValueInt64() != 10 || wcg.WithinHours.ValueInt64() != 6 ||
		wcg.MinIncrease.Amount.ValueString() != "150.00" || wcg.MinIncrease.Currency.ValueString() != "USD" {
		t.Fatalf("repeat = %+v", ca.Repeat)
	}
}

func TestCustomerNotificationDataSourceReadMinimal(t *testing.T) {
	var gotPath atomic.Value
	server := customerNotificationDataSourceTestServer(t, 200, customerNotificationMinimalJSON, &gotPath)
	id := tftypes.NewValue(tftypes.String, "notif-2")
	resp := readCustomerNotificationDataSourceForTest(t, server, &id)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}

	var data customerNotificationDataSourceModel
	if diags := resp.State.Get(t.Context(), &data); diags.HasError() {
		t.Fatal(diags)
	}
	if !data.CreateTime.IsNull() || !data.UpdateTime.IsNull() || !data.Etag.IsNull() {
		t.Fatalf("absent timestamps and etag must be null: %+v", data)
	}
	// Computed-only lists are empty, never null, so consumers can iterate them.
	dest := data.Destinations
	if len(dest.Emails.Elements()) != 1 {
		t.Fatalf("emails = %v", dest.Emails)
	}
	if dest.SlackChannels.IsNull() || len(dest.SlackChannels.Elements()) != 0 ||
		dest.TeamsChannels.IsNull() || len(dest.TeamsChannels.Elements()) != 0 {
		t.Fatalf("unused destination lists must be empty, got %v / %v", dest.SlackChannels, dest.TeamsChannels)
	}
	ca := data.Settings.CostAnomalies
	if ca.Allocations.Ids.IsNull() || len(ca.Allocations.Ids.Elements()) != 0 {
		t.Fatalf("allocations.ids = %v, want empty list", ca.Allocations.Ids)
	}
	if !ca.Repeat.IsNull() {
		t.Fatalf("repeat = %v, want null when the API omits it", ca.Repeat)
	}
}

func TestCustomerNotificationDataSourceReadPartialRepeat(t *testing.T) {
	for _, tc := range []struct {
		name   string
		repeat string
		check  func(t *testing.T, data customerNotificationDataSourceModel)
	}{
		{
			"growth without min_increase",
			`{"whenCostGrows": {"percentIncrease": 20}}`,
			func(t *testing.T, data customerNotificationDataSourceModel) {
				repeat := data.Settings.CostAnomalies.Repeat
				wcg := repeat.WhenCostGrows
				if !repeat.EveryHours.IsNull() || wcg.IsNull() || wcg.PercentIncrease.ValueInt64() != 20 ||
					!wcg.Match.IsNull() || !wcg.MinIncrease.IsNull() || !wcg.WithinHours.IsNull() {
					t.Fatalf("repeat = %+v", repeat)
				}
			},
		},
		{
			"every_hours only",
			`{"everyHours": 12}`,
			func(t *testing.T, data customerNotificationDataSourceModel) {
				repeat := data.Settings.CostAnomalies.Repeat
				if repeat.IsNull() || repeat.EveryHours.ValueInt64() != 12 || !repeat.WhenCostGrows.IsNull() {
					t.Fatalf("repeat = %+v", repeat)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotPath atomic.Value
			server := customerNotificationDataSourceTestServer(t, 200, `{
				"id": "notif-3", "name": "Partial",
				"destinations": {"emails": [], "slackChannels": ["C1"], "teamsChannels": []},
				"settings": {"costAnomalies": {
					"minSeverity": "warning",
					"minExcessCost": {"amount": "5.00", "currency": "USD"},
					"allocations": {"mode": "all", "ids": []},
					"repeat": `+tc.repeat+`
				}}
			}`, &gotPath)
			id := tftypes.NewValue(tftypes.String, "notif-3")
			resp := readCustomerNotificationDataSourceForTest(t, server, &id)
			if resp.Diagnostics.HasError() {
				t.Fatal(resp.Diagnostics)
			}
			var data customerNotificationDataSourceModel
			if diags := resp.State.Get(t.Context(), &data); diags.HasError() {
				t.Fatal(diags)
			}
			tc.check(t, data)
		})
	}
}

func TestCustomerNotificationDataSourceIDMismatch(t *testing.T) {
	var gotPath atomic.Value
	server := customerNotificationDataSourceTestServer(t, 200, customerNotificationFullJSON, &gotPath)
	id := tftypes.NewValue(tftypes.String, "requested")
	resp := readCustomerNotificationDataSourceForTest(t, server, &id)
	if !resp.Diagnostics.HasError() || !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "notif-1") {
		t.Fatalf("diagnostics = %v, want ID mismatch error", resp.Diagnostics)
	}
}

func TestCustomerNotificationsDataSourceRead(t *testing.T) {
	var gotPath atomic.Value
	server := customerNotificationDataSourceTestServer(t, 200, `{"items": [`+customerNotificationFullJSON+`,`+customerNotificationMinimalJSON+`]}`, &gotPath)
	resp := readCustomerNotificationDataSourceForTest(t, server, nil)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if want := "/customers/v1/notifications"; gotPath.Load() != want {
		t.Fatalf("path = %v, want %s", gotPath.Load(), want)
	}

	var data customerNotificationsDataSourceModel
	if diags := resp.State.Get(t.Context(), &data); diags.HasError() {
		t.Fatal(diags)
	}
	var items []datasource_customer_notifications.ItemsValue
	if diags := data.Items.ElementsAs(t.Context(), &items, false); diags.HasError() {
		t.Fatal(diags)
	}
	if len(items) != 2 || items[0].Id.ValueString() != "notif-1" || items[1].Id.ValueString() != "notif-2" {
		t.Fatalf("items must keep the API order: %+v", items)
	}
	// The first item carries the full configuration, the second only the required members.
	repeat := items[0].Settings.CostAnomalies.Repeat
	if repeat.EveryHours.ValueInt64() != 24 || repeat.WhenCostGrows.MinIncrease.Amount.ValueString() != "150.00" {
		t.Fatalf("first item lost nested settings: %+v", repeat)
	}
	dest := items[0].Destinations
	if len(dest.Emails.Elements()) != 2 || len(dest.SlackChannels.Elements()) != 1 || len(dest.TeamsChannels.Elements()) != 1 ||
		items[0].Etag.ValueString() != "etag-1" {
		t.Fatalf("first item = %+v", items[0])
	}
	second := items[1]
	if !second.Settings.CostAnomalies.Repeat.IsNull() || !second.Etag.IsNull() ||
		second.Destinations.SlackChannels.IsNull() || len(second.Destinations.SlackChannels.Elements()) != 0 {
		t.Fatalf("second item = %+v", second)
	}
}

func TestCustomerNotificationsDataSourceReadEmpty(t *testing.T) {
	var gotPath atomic.Value
	server := customerNotificationDataSourceTestServer(t, 200, `{"items": []}`, &gotPath)
	resp := readCustomerNotificationDataSourceForTest(t, server, nil)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	var data customerNotificationsDataSourceModel
	if diags := resp.State.Get(t.Context(), &data); diags.HasError() {
		t.Fatal(diags)
	}
	// An empty list, never null, so for_each and for expressions work.
	if data.Items.IsNull() || data.Items.IsUnknown() || len(data.Items.Elements()) != 0 {
		t.Fatalf("items = %v, want empty list", data.Items)
	}
}
