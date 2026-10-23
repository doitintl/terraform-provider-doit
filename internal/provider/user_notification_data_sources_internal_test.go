package provider

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_user_notifications"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// readUserNotificationDataSourceForTest runs Read on the singular data source
// when id is non-nil, and on the plural data source otherwise.
func readUserNotificationDataSourceForTest(t *testing.T, server *httptest.Server, userID, id *tftypes.Value) datasource.ReadResponse {
	t.Helper()
	httpClient := server.Client()
	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}
	var ds datasource.DataSource
	if id == nil {
		ds = &userNotificationsDataSource{client: client}
	} else {
		ds = &userNotificationDataSource{client: client}
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
	if userID != nil {
		values["user_id"] = *userID
	}
	if id != nil {
		values["id"] = *id
	}
	resp := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	ds.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(tftypes.Object{AttributeTypes: attrTypes}, values)}}, &resp)
	return resp
}

func userNotificationDataSourceTestServer(t *testing.T, status int, body string, gotPath *atomic.Value) *httptest.Server {
	t.Helper()
	return httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath.Store(r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

// userNotificationFullJSON has every optional member set.
const userNotificationFullJSON = `{
	"id": "notif-1", "name": "Anomaly alerts",
	"createTime": "2026-10-01T08:00:00Z", "updateTime": "2026-10-02T09:30:00Z", "etag": "etag-1",
	"destinations": [{"type": "email", "address": "jane.doe@example.com"}],
	"settings": {"costAnomalies": {
		"minSeverity": "critical",
		"minExcessCost": {"amount": "100.00", "currency": "USD"},
		"allocations": {"mode": "include", "ids": ["alloc-b", "alloc-a"]},
		"repeat": {"everyHours": 24, "whenCostGrows": {
			"match": "or", "percentIncrease": 10, "withinHours": 6,
			"minIncrease": {"amount": "150.00", "currency": "USD"}}}
	}}
}`

// userNotificationMinimalJSON omits repeat, destinations and the optional timestamps.
const userNotificationMinimalJSON = `{
	"id": "notif-2", "name": "Minimal", "createTime": null, "updateTime": null, "etag": null,
	"settings": {"costAnomalies": {
		"minSeverity": "warning",
		"minExcessCost": {"amount": "1000000.00", "currency": "USD"},
		"allocations": {"mode": "all", "ids": []}
	}}
}`

func TestUserNotificationDataSourceUnknownInputs(t *testing.T) {
	unknown := tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	known := tftypes.NewValue(tftypes.String, "known")
	for _, tc := range []struct {
		name   string
		userID tftypes.Value
		id     tftypes.Value
	}{
		{"unknown id", known, unknown},
		{"unknown user_id", unknown, known},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewTestServer(t, http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { calls.Add(1) }))
			resp := readUserNotificationDataSourceForTest(t, server, &tc.userID, &tc.id)
			if resp.Diagnostics.HasError() {
				t.Fatal(resp.Diagnostics)
			}
			if calls.Load() != 0 {
				t.Fatalf("made %d API requests for unknown input", calls.Load())
			}
			var data userNotificationDataSourceModel
			if diags := resp.State.Get(t.Context(), &data); diags.HasError() {
				t.Fatal(diags)
			}
			if !data.Name.IsUnknown() || !data.Etag.IsUnknown() || !data.CreateTime.IsUnknown() ||
				!data.Destinations.IsUnknown() || !data.Settings.IsUnknown() {
				t.Fatalf("outputs must remain unknown: %+v", data)
			}
		})
	}
}

func TestUserNotificationsDataSourceUnknownUserID(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewTestServer(t, http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { calls.Add(1) }))
	unknown := tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	resp := readUserNotificationDataSourceForTest(t, server, &unknown, nil)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if calls.Load() != 0 {
		t.Fatalf("made %d API requests for unknown input", calls.Load())
	}
	var data userNotificationsDataSourceModel
	if diags := resp.State.Get(t.Context(), &data); diags.HasError() {
		t.Fatal(diags)
	}
	if !data.Items.IsUnknown() {
		t.Fatalf("items must remain unknown: %+v", data.Items)
	}
}

func TestUserNotificationDataSourceReadErrors(t *testing.T) {
	for _, tc := range []struct {
		name        string
		status      int
		contentType string
		body        string
		want        string
	}{
		{"not found", 404, "application/json", `{"error":"missing"}`, "status 404"},
		{"server error", 500, "application/json", `{"error":"boom"}`, "boom"},
		{"non-JSON", 200, "text/plain", "wrong content type", "status 200"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			userID := tftypes.NewValue(tftypes.String, "user-1")
			id := tftypes.NewValue(tftypes.String, "notif-1")
			for name, notificationID := range map[string]*tftypes.Value{"singular": &id, "plural": nil} {
				resp := readUserNotificationDataSourceForTest(t, server, &userID, notificationID)
				if !resp.Diagnostics.HasError() || !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), tc.want) {
					t.Fatalf("%s: diagnostics = %v, want %q", name, resp.Diagnostics, tc.want)
				}
			}
		})
	}
}

func TestUserNotificationDataSourceRead(t *testing.T) {
	var gotPath atomic.Value
	server := userNotificationDataSourceTestServer(t, 200, userNotificationFullJSON, &gotPath)
	userID := tftypes.NewValue(tftypes.String, "user-1")
	id := tftypes.NewValue(tftypes.String, "notif-1")
	resp := readUserNotificationDataSourceForTest(t, server, &userID, &id)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if want := "/iam/v1/users/user-1/notifications/notif-1"; gotPath.Load() != want {
		t.Fatalf("path = %v, want %s", gotPath.Load(), want)
	}

	var data userNotificationDataSourceModel
	if diags := resp.State.Get(t.Context(), &data); diags.HasError() {
		t.Fatal(diags)
	}
	if data.Id.ValueString() != "notif-1" || data.UserId.ValueString() != "user-1" ||
		data.Name.ValueString() != "Anomaly alerts" || data.Etag.ValueString() != "etag-1" ||
		data.CreateTime.ValueString() != "2026-10-01T08:00:00Z" || data.UpdateTime.ValueString() != "2026-10-02T09:30:00Z" {
		t.Fatalf("mapped state = %+v", data)
	}
	if n := len(data.Destinations.Elements()); n != 1 {
		t.Fatalf("destinations = %d, want 1", n)
	}

	ca := data.Settings.CostAnomalies
	if ca.MinSeverity.ValueString() != "critical" ||
		ca.MinExcessCost.Amount.ValueString() != "100.00" || ca.MinExcessCost.Currency.ValueString() != "USD" ||
		ca.Allocations.Mode.ValueString() != "include" {
		t.Fatalf("cost_anomalies = %+v", ca)
	}
	// The order of the IDs is the API's.
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

func TestUserNotificationDataSourceReadMinimal(t *testing.T) {
	var gotPath atomic.Value
	server := userNotificationDataSourceTestServer(t, 200, userNotificationMinimalJSON, &gotPath)
	userID := tftypes.NewValue(tftypes.String, "user-1")
	id := tftypes.NewValue(tftypes.String, "notif-2")
	resp := readUserNotificationDataSourceForTest(t, server, &userID, &id)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}

	var data userNotificationDataSourceModel
	if diags := resp.State.Get(t.Context(), &data); diags.HasError() {
		t.Fatal(diags)
	}
	if !data.CreateTime.IsNull() || !data.UpdateTime.IsNull() || !data.Etag.IsNull() {
		t.Fatalf("absent timestamps and etag must be null: %+v", data)
	}
	// Computed-only lists are empty, never null, so consumers can iterate them.
	if data.Destinations.IsNull() || len(data.Destinations.Elements()) != 0 {
		t.Fatalf("destinations = %v, want empty list", data.Destinations)
	}
	ca := data.Settings.CostAnomalies
	if ca.Allocations.Ids.IsNull() || len(ca.Allocations.Ids.Elements()) != 0 {
		t.Fatalf("allocations.ids = %v, want empty list", ca.Allocations.Ids)
	}
	if !ca.Repeat.IsNull() {
		t.Fatalf("repeat = %v, want null when the API omits it", ca.Repeat)
	}
}

func TestUserNotificationDataSourceReadPartialRepeat(t *testing.T) {
	var gotPath atomic.Value
	server := userNotificationDataSourceTestServer(t, 200, `{
		"id": "notif-3", "name": "Partial",
		"settings": {"costAnomalies": {
			"minSeverity": "warning",
			"minExcessCost": {"amount": "5.00", "currency": "USD"},
			"allocations": {"mode": "all", "ids": []},
			"repeat": {"whenCostGrows": {"percentIncrease": 20}}
		}}
	}`, &gotPath)
	userID := tftypes.NewValue(tftypes.String, "user-1")
	id := tftypes.NewValue(tftypes.String, "notif-3")
	resp := readUserNotificationDataSourceForTest(t, server, &userID, &id)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}

	var data userNotificationDataSourceModel
	if diags := resp.State.Get(t.Context(), &data); diags.HasError() {
		t.Fatal(diags)
	}
	repeat := data.Settings.CostAnomalies.Repeat
	wcg := repeat.WhenCostGrows
	if !repeat.EveryHours.IsNull() || wcg.IsNull() || wcg.PercentIncrease.ValueInt64() != 20 ||
		!wcg.Match.IsNull() || !wcg.MinIncrease.IsNull() || !wcg.WithinHours.IsNull() {
		t.Fatalf("repeat = %+v", repeat)
	}
}

func TestUserNotificationDataSourceIDMismatch(t *testing.T) {
	var gotPath atomic.Value
	server := userNotificationDataSourceTestServer(t, 200, userNotificationFullJSON, &gotPath)
	userID := tftypes.NewValue(tftypes.String, "user-1")
	id := tftypes.NewValue(tftypes.String, "requested")
	resp := readUserNotificationDataSourceForTest(t, server, &userID, &id)
	if !resp.Diagnostics.HasError() || !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "notif-1") {
		t.Fatalf("diagnostics = %v, want ID mismatch error", resp.Diagnostics)
	}
}

func TestUserNotificationsDataSourceRead(t *testing.T) {
	var gotPath atomic.Value
	server := userNotificationDataSourceTestServer(t, 200, `{"items": [`+userNotificationFullJSON+`,`+userNotificationMinimalJSON+`]}`, &gotPath)
	userID := tftypes.NewValue(tftypes.String, "user-1")
	resp := readUserNotificationDataSourceForTest(t, server, &userID, nil)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if want := "/iam/v1/users/user-1/notifications"; gotPath.Load() != want {
		t.Fatalf("path = %v, want %s", gotPath.Load(), want)
	}

	var data userNotificationsDataSourceModel
	if diags := resp.State.Get(t.Context(), &data); diags.HasError() {
		t.Fatal(diags)
	}
	if data.UserId.ValueString() != "user-1" {
		t.Fatalf("user_id = %q", data.UserId.ValueString())
	}
	var items []datasource_user_notifications.ItemsValue
	diags := data.Items.ElementsAs(t.Context(), &items, false)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if len(items) != 2 || items[0].Id.ValueString() != "notif-1" || items[1].Id.ValueString() != "notif-2" {
		t.Fatalf("items must keep the API order: %+v", items)
	}
	// The first item carries the full repeat configuration, the second has none.
	repeat := items[0].Settings.CostAnomalies.Repeat
	if repeat.EveryHours.ValueInt64() != 24 || repeat.WhenCostGrows.MinIncrease.Amount.ValueString() != "150.00" {
		t.Fatalf("first item lost nested settings: %+v", repeat)
	}
	if len(items[0].Destinations.Elements()) != 1 || items[0].Etag.ValueString() != "etag-1" {
		t.Fatalf("first item = %+v", items[0])
	}
	if !items[1].Settings.CostAnomalies.Repeat.IsNull() || !items[1].Etag.IsNull() || items[1].Destinations.IsNull() {
		t.Fatalf("second item = %+v", items[1])
	}
}

func TestUserNotificationsDataSourceReadEmpty(t *testing.T) {
	var gotPath atomic.Value
	server := userNotificationDataSourceTestServer(t, 200, `{"items": []}`, &gotPath)
	userID := tftypes.NewValue(tftypes.String, "user-1")
	resp := readUserNotificationDataSourceForTest(t, server, &userID, nil)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	var data userNotificationsDataSourceModel
	if diags := resp.State.Get(t.Context(), &data); diags.HasError() {
		t.Fatal(diags)
	}
	// An empty list, never null, so for_each and for expressions work.
	if data.Items.IsNull() || data.Items.IsUnknown() || len(data.Items.Elements()) != 0 {
		t.Fatalf("items = %v, want empty list", data.Items)
	}
}
