package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_user_notification"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// testUserNotificationResponse returns an API response with every optional
// member set. Callers trim it per test.
func testUserNotificationResponse() *models.UserNotification {
	createTime := time.Date(2026, 10, 1, 8, 0, 0, 123000, time.UTC)
	updateTime := time.Date(2026, 10, 2, 9, 30, 0, 0, time.UTC)
	address := openapi_types.Email("jane.doe@example.com")

	return &models.UserNotification{
		Id:         valueToNullable("notif-1"),
		Name:       "Anomaly alerts",
		CreateTime: valueToNullable(createTime),
		UpdateTime: valueToNullable(updateTime),
		Etag:       valueToNullable("etag-1"),
		Destinations: &[]models.UserNotificationEmailDestination{
			{Type: "email", Address: &address},
		},
		Settings: models.UserNotificationSettings{
			CostAnomalies: models.CostAnomalySettings{
				MinSeverity:   "critical",
				MinExcessCost: models.Money{Amount: "100.00", Currency: "USD"},
				Allocations: models.CostAnomalyAllocations{
					Mode: "include",
					Ids:  []string{"alloc-b", "alloc-a"},
				},
				Repeat: &models.CostAnomalyRepeat{
					EveryHours: new(24),
					WhenCostGrows: &models.CostAnomalyWhenCostGrows{
						Match:           new(models.CostAnomalyWhenCostGrowsMatch("or")),
						PercentIncrease: new(10),
						MinIncrease:     &models.Money{Amount: "150.00", Currency: "USD"},
						WithinHours:     new(6),
					},
				},
			},
		},
	}
}

func mustMapUserNotification(t *testing.T, resp *models.UserNotification, state *userNotificationResourceModel) {
	t.Helper()
	if diags := mapUserNotificationToModel(t.Context(), resp, state); diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
}

func TestMapUserNotificationToModel(t *testing.T) {
	state := userNotificationResourceModel{UserId: types.StringValue("user-1")}
	mustMapUserNotification(t, testUserNotificationResponse(), &state)

	if got := state.Id.ValueString(); got != "notif-1" {
		t.Errorf("id = %q, want notif-1", got)
	}
	if got := state.UserId.ValueString(); got != "user-1" {
		t.Errorf("user_id = %q, want user-1 (kept from path)", got)
	}
	if got := state.CreateTime.ValueString(); got != "2026-10-01T08:00:00Z" {
		t.Errorf("create_time = %q", got)
	}
	if got := state.UpdateTime.ValueString(); got != "2026-10-02T09:30:00Z" {
		t.Errorf("update_time = %q", got)
	}
	if got := state.Etag.ValueString(); got != "etag-1" {
		t.Errorf("etag = %q", got)
	}

	var destinations []resource_user_notification.DestinationsValue
	if diags := state.Destinations.ElementsAs(t.Context(), &destinations, false); diags.HasError() {
		t.Fatalf("destinations: %v", diags)
	}
	if len(destinations) != 1 || destinations[0].Address.ValueString() != "jane.doe@example.com" || destinations[0].DestinationsType.ValueString() != "email" {
		t.Errorf("destinations = %+v", destinations)
	}

	ca := state.Settings.CostAnomalies
	if got := ca.MinSeverity.ValueString(); got != "critical" {
		t.Errorf("min_severity = %q", got)
	}
	if got := ca.MinExcessCost.Amount.ValueString(); got != "100.00" {
		t.Errorf("min_excess_cost.amount = %q", got)
	}
	var ids []string
	if diags := ca.Allocations.Ids.ElementsAs(t.Context(), &ids, false); diags.HasError() {
		t.Fatalf("ids: %v", diags)
	}
	if !reflect.DeepEqual(ids, []string{"alloc-b", "alloc-a"}) {
		t.Errorf("ids = %v, want API order", ids)
	}

	wcg := ca.Repeat.WhenCostGrows
	if ca.Repeat.EveryHours.ValueInt64() != 24 || wcg.Match.ValueString() != "or" || wcg.PercentIncrease.ValueInt64() != 10 ||
		wcg.WithinHours.ValueInt64() != 6 || wcg.MinIncrease.Amount.ValueString() != "150.00" {
		t.Errorf("repeat = %+v", ca.Repeat)
	}
}

// The API omits repeat (and each unset member) when no resend control is set.
func TestMapUserNotificationToModel_AbsentOptionals(t *testing.T) {
	t.Run("no repeat, nil ids", func(t *testing.T) {
		resp := testUserNotificationResponse()
		resp.Settings.CostAnomalies.Repeat = nil
		resp.Settings.CostAnomalies.Allocations = models.CostAnomalyAllocations{Mode: "all"}

		var state userNotificationResourceModel
		mustMapUserNotification(t, resp, &state)

		if !state.Settings.CostAnomalies.Repeat.IsNull() {
			t.Errorf("repeat = %v, want null", state.Settings.CostAnomalies.Repeat)
		}
		ids := state.Settings.CostAnomalies.Allocations.Ids
		if ids.IsNull() || len(ids.Elements()) != 0 {
			t.Errorf("ids = %v, want empty list", ids)
		}
	})

	t.Run("partial repeat", func(t *testing.T) {
		resp := testUserNotificationResponse()
		resp.Settings.CostAnomalies.Repeat = &models.CostAnomalyRepeat{
			WhenCostGrows: &models.CostAnomalyWhenCostGrows{PercentIncrease: new(10)},
		}

		var state userNotificationResourceModel
		mustMapUserNotification(t, resp, &state)

		repeat := state.Settings.CostAnomalies.Repeat
		if !repeat.EveryHours.IsNull() {
			t.Errorf("every_hours = %v, want null", repeat.EveryHours)
		}
		wcg := repeat.WhenCostGrows
		if !wcg.Match.IsNull() || !wcg.MinIncrease.IsNull() || !wcg.WithinHours.IsNull() || wcg.PercentIncrease.ValueInt64() != 10 {
			t.Errorf("when_cost_grows = %+v", wcg)
		}
	})
}

// The API echoes amounts with two decimals. A refresh must keep the spelling in
// state when the value is unchanged, and take the API value when it changed.
func TestMapUserNotificationToModel_AmountSpelling(t *testing.T) {
	withAmounts := func(minExcess, minIncrease string) *models.UserNotification {
		resp := testUserNotificationResponse()
		resp.Settings.CostAnomalies.MinExcessCost.Amount = minExcess
		resp.Settings.CostAnomalies.Repeat.WhenCostGrows.MinIncrease.Amount = minIncrease
		return resp
	}

	// Prior state as written in config, without decimals.
	var state userNotificationResourceModel
	mustMapUserNotification(t, withAmounts("100", "150"), &state)

	mustMapUserNotification(t, withAmounts("100.00", "150.00"), &state)
	ca := state.Settings.CostAnomalies
	if got := ca.MinExcessCost.Amount.ValueString(); got != "100" {
		t.Errorf("unchanged min_excess_cost.amount = %q, want state spelling 100", got)
	}
	if got := ca.Repeat.WhenCostGrows.MinIncrease.Amount.ValueString(); got != "150" {
		t.Errorf("unchanged min_increase.amount = %q, want state spelling 150", got)
	}

	mustMapUserNotification(t, withAmounts("200.00", "150.00"), &state)
	if got := state.Settings.CostAnomalies.MinExcessCost.Amount.ValueString(); got != "200.00" {
		t.Errorf("changed min_excess_cost.amount = %q, want API value 200.00", got)
	}

	// Import: no prior state, so the API spelling is taken.
	var imported userNotificationResourceModel
	mustMapUserNotification(t, withAmounts("100.00", "150.00"), &imported)
	if got := imported.Settings.CostAnomalies.MinExcessCost.Amount.ValueString(); got != "100.00" {
		t.Errorf("imported min_excess_cost.amount = %q, want 100.00", got)
	}
}

func TestUserNotificationAmountsEqual(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"100", "100.00", true},
		{"100.0", "100.00", true},
		{"0", "0.00", true},
		{"100", "100.01", false},
		{"100", "1000.00", false},
		{"not-a-number", "100.00", false},
		{"100", "", false},
	}
	for _, tt := range tests {
		if got := costAnomalyAmountsEqual(tt.a, tt.b); got != tt.want {
			t.Errorf("costAnomalyAmountsEqual(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestUserNotificationAmountPattern(t *testing.T) {
	tests := []struct {
		value string
		valid bool
	}{
		{"0", true},
		{"100", true},
		{"100.0", true},
		{"100.00", true},
		{"1000000000", true},
		{"100.000", false},
		{"100.50", false},
		{"0150", false},
		{"-100", false},
		{"1e3", false},
		{"+100", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := costAnomalyAmountPattern.MatchString(tt.value); got != tt.valid {
			t.Errorf("pattern match for %q = %v, want %v", tt.value, got, tt.valid)
		}
	}
}

func TestOverlayUserNotificationComputedFields(t *testing.T) {
	// The plan carries the configured spelling; the response the API spelling.
	var plan userNotificationResourceModel
	mustMapUserNotification(t, testUserNotificationResponse(), &plan)
	plan.Settings.CostAnomalies.MinExcessCost.Amount = types.StringValue("100")
	plan.Id = types.StringUnknown()
	plan.CreateTime = types.StringUnknown()
	plan.UpdateTime = types.StringUnknown()
	plan.Etag = types.StringUnknown()
	plan.Destinations = types.ListUnknown(resource_user_notification.DestinationsValue{}.Type(t.Context()))

	if diags := overlayUserNotificationComputedFields(t.Context(), testUserNotificationResponse(), &plan); diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if plan.Id.ValueString() != "notif-1" || plan.Etag.ValueString() != "etag-1" || plan.UpdateTime.IsUnknown() || plan.CreateTime.IsUnknown() {
		t.Errorf("computed fields not resolved: %+v", plan.UserNotificationModel)
	}
	if plan.Destinations.IsUnknown() || len(plan.Destinations.Elements()) != 1 {
		t.Errorf("destinations = %v", plan.Destinations)
	}
	if got := plan.Settings.CostAnomalies.MinExcessCost.Amount.ValueString(); got != "100" {
		t.Errorf("amount = %q, want the planned spelling 100", got)
	}

	// On Update, known update_time and etag are kept (the next Read refreshes them).
	plan.UpdateTime = types.StringValue("2026-01-01T00:00:00Z")
	plan.Etag = types.StringValue("etag-old")
	if diags := overlayUserNotificationComputedFields(t.Context(), testUserNotificationResponse(), &plan); diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if plan.UpdateTime.ValueString() != "2026-01-01T00:00:00Z" || plan.Etag.ValueString() != "etag-old" {
		t.Errorf("known update_time/etag overwritten: %q, %q", plan.UpdateTime.ValueString(), plan.Etag.ValueString())
	}
}

// assertJSONEqual compares v's JSON encoding with want, ignoring key order.
func assertJSONEqual(t *testing.T, v any, want string) {
	t.Helper()
	gotBytes, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got, exp any
	if err := json.Unmarshal(gotBytes, &got); err != nil {
		t.Fatalf("unmarshal got: %v", err)
	}
	if err := json.Unmarshal([]byte(want), &exp); err != nil {
		t.Fatalf("unmarshal want: %v", err)
	}
	if !reflect.DeepEqual(got, exp) {
		t.Errorf("JSON mismatch\n got: %s\nwant: %s", gotBytes, want)
	}
}

// planFromResponse builds a fully typed plan model the same way Read does.
func planFromResponse(t *testing.T, resp *models.UserNotification) userNotificationResourceModel {
	t.Helper()
	var plan userNotificationResourceModel
	mustMapUserNotification(t, resp, &plan)
	return plan
}

func TestUserNotificationToCreateRequest(t *testing.T) {
	t.Run("full", func(t *testing.T) {
		plan := planFromResponse(t, testUserNotificationResponse())
		req, diags := plan.toCreateRequest(t.Context())
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		assertJSONEqual(t, req, `{
			"name": "Anomaly alerts",
			"settings": {"costAnomalies": {
				"minSeverity": "critical",
				"minExcessCost": {"amount": "100.00", "currency": "USD"},
				"allocations": {"mode": "include", "ids": ["alloc-b", "alloc-a"]},
				"repeat": {
					"everyHours": 24,
					"whenCostGrows": {"match": "or", "percentIncrease": 10, "minIncrease": {"amount": "150.00", "currency": "USD"}, "withinHours": 6}
				}
			}}
		}`)
	})

	// Omitted optionals are left out; ids is still sent as [] because the API requires it.
	t.Run("minimal", func(t *testing.T) {
		resp := testUserNotificationResponse()
		resp.Settings.CostAnomalies.Repeat = nil
		resp.Settings.CostAnomalies.Allocations = models.CostAnomalyAllocations{Mode: "all"}
		plan := planFromResponse(t, resp)

		req, diags := plan.toCreateRequest(t.Context())
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		assertJSONEqual(t, req, `{
			"name": "Anomaly alerts",
			"settings": {"costAnomalies": {
				"minSeverity": "critical",
				"minExcessCost": {"amount": "100.00", "currency": "USD"},
				"allocations": {"mode": "all", "ids": []}
			}}
		}`)
	})
}

// The merge patch must carry explicit nulls for every unset optional member:
// an omitted member would keep its stored value.
func TestUserNotificationToUpdateRequest(t *testing.T) {
	const head = `"name": "Anomaly alerts", "settings": {"costAnomalies": {
		"minSeverity": "critical",
		"minExcessCost": {"amount": "100.00", "currency": "USD"},
		"allocations": {"mode": "include", "ids": ["alloc-b", "alloc-a"]},`

	tests := []struct {
		name   string
		repeat *models.CostAnomalyRepeat
		want   string
	}{
		{
			name:   "repeat cleared",
			repeat: nil,
			want:   `{` + head + `"repeat": null}}}`,
		},
		{
			name:   "when_cost_grows cleared",
			repeat: &models.CostAnomalyRepeat{EveryHours: new(12)},
			want:   `{` + head + `"repeat": {"everyHours": 12, "whenCostGrows": null}}}}`,
		},
		{
			name: "single leaves cleared",
			repeat: &models.CostAnomalyRepeat{
				WhenCostGrows: &models.CostAnomalyWhenCostGrows{PercentIncrease: new(10)},
			},
			want: `{` + head + `"repeat": {"everyHours": null, "whenCostGrows": {"match": null, "percentIncrease": 10, "minIncrease": null, "withinHours": null}}}}}`,
		},
		{
			name:   "everything set",
			repeat: testUserNotificationResponse().Settings.CostAnomalies.Repeat,
			want: `{` + head + `"repeat": {"everyHours": 24, "whenCostGrows": {"match": "or", "percentIncrease": 10, ` +
				`"minIncrease": {"amount": "150.00", "currency": "USD"}, "withinHours": 6}}}}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := testUserNotificationResponse()
			resp.Settings.CostAnomalies.Repeat = tt.repeat
			plan := planFromResponse(t, resp)

			req, diags := plan.toUpdateRequest(t.Context())
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			assertJSONEqual(t, req, tt.want)
		})
	}
}

func TestUserNotificationImportState_InvalidID(t *testing.T) {
	r := &userNotificationResource{}
	for _, id := range []string{"notif-only", "/notif", "user/", "a/b/c", ""} {
		resp := &resource.ImportStateResponse{}
		r.ImportState(t.Context(), resource.ImportStateRequest{ID: id}, resp)
		if !resp.Diagnostics.HasError() {
			t.Errorf("import ID %q: expected an error", id)
		}
	}
}

func TestUserNotification_PopulateState_NotFound(t *testing.T) {
	srv := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/iam/v1/users/user-1/notifications/missing" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message": "Not Found"}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))

	res := &userNotificationResource{client: newTestAPIClient(t, srv)}
	state := userNotificationResourceModel{
		UserId: types.StringValue("user-1"),
		Id:     types.StringValue("missing")}

	if diags := res.populateState(t.Context(), &state); diags.HasError() {
		t.Fatalf("populateState should not return error on 404, got: %v", diags)
	}
	if !state.Id.IsNull() {
		t.Errorf("expected state.Id to be null on 404, got %q", state.Id.ValueString())
	}
}

// TestUserNotificationResourceDelete runs every case through the plain test
// client and through DCIRetryClient, which production uses: a status the retry
// client turns into an error never reaches the resource's status handling.
func TestUserNotificationResourceDelete(t *testing.T) {
	const path = "/iam/v1/users/user-1/notifications/notif-1"

	clients := map[string]func(*testing.T, *httptest.Server) *models.ClientWithResponses{
		"plain": newTestAPIClient,
		"retry": func(t *testing.T, srv *httptest.Server) *models.ClientWithResponses {
			return newTestRetryAPIClient(t, srv, DefaultRequestTimeout, constantBackOff(time.Millisecond))
		},
	}

	tests := []struct {
		name        string
		stateEtag   string
		currentEtag string // etag the server accepts; GET returns it
		getStatus   int
		deleteFinal int // status once the etag matches
		wantDeletes int32
		wantGets    int32
		expectError bool
	}{
		{name: "etag from state", stateEtag: "e1", currentEtag: "e1", getStatus: 200, deleteFinal: 204, wantDeletes: 1, wantGets: 0},
		{name: "stale etag retried once", stateEtag: "old", currentEtag: "e2", getStatus: 200, deleteFinal: 204, wantDeletes: 2, wantGets: 1},
		{name: "no etag in state fetched first", stateEtag: "", currentEtag: "e3", getStatus: 200, deleteFinal: 204, wantDeletes: 1, wantGets: 1},
		{name: "already gone", stateEtag: "e1", currentEtag: "e1", getStatus: 404, deleteFinal: 404, wantDeletes: 1, wantGets: 0},
		{name: "gone before retry", stateEtag: "old", currentEtag: "e2", getStatus: 404, deleteFinal: 204, wantDeletes: 1, wantGets: 1},
		{name: "server error", stateEtag: "e1", currentEtag: "e1", getStatus: 200, deleteFinal: 500, wantDeletes: 1, wantGets: 0, expectError: true},
	}

	for clientName, newClient := range clients {
		for _, tt := range tests {
			t.Run(clientName+"/"+tt.name, func(t *testing.T) {
				var deletes, gets atomic.Int32
				srv := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != path {
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					switch r.Method {
					case http.MethodGet:
						gets.Add(1)
						if tt.getStatus != 200 {
							w.Header().Set("Content-Type", "application/problem+json")
							w.WriteHeader(tt.getStatus)
							_, _ = w.Write([]byte(`{"code":"not_found","status":404,"title":"Not found"}`))
							return
						}
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(tt.getStatus)
						_, _ = w.Write([]byte(`{"id":"notif-1","name":"n","etag":"` + tt.currentEtag + `","settings":{"costAnomalies":{"minSeverity":"warning","minExcessCost":{"amount":"1.00","currency":"USD"},"allocations":{"mode":"all","ids":[]}}}}`))
					case http.MethodDelete:
						deletes.Add(1)
						if r.Header.Get("Idempotency-Key") == "" {
							w.WriteHeader(http.StatusBadRequest)
							return
						}
						if r.Header.Get("If-Match") != tt.currentEtag {
							w.WriteHeader(http.StatusPreconditionFailed)
							return
						}
						w.WriteHeader(tt.deleteFinal)
					default:
						w.WriteHeader(http.StatusMethodNotAllowed)
					}
				}))

				ctx, cancel := context.WithTimeout(t.Context(), retryTestTimeout)
				defer cancel()
				r := &userNotificationResource{client: newClient(t, srv)}
				resp := &resource.DeleteResponse{}
				values := map[string]string{"user_id": "user-1", "id": "notif-1"}
				if tt.stateEtag != "" {
					values["etag"] = tt.stateEtag
				}
				r.Delete(ctx, resource.DeleteRequest{State: newTestState(ctx, t, r, values)}, resp)

				if got := resp.Diagnostics.HasError(); got != tt.expectError {
					t.Errorf("hasError = %v, want %v; diagnostics: %v", got, tt.expectError, resp.Diagnostics)
				}
				if got := deletes.Load(); got != tt.wantDeletes {
					t.Errorf("DELETE calls = %d, want %d", got, tt.wantDeletes)
				}
				if got := gets.Load(); got != tt.wantGets {
					t.Errorf("GET calls = %d, want %d", got, tt.wantGets)
				}
			})
		}
	}
}

func testAllocationsValue(t *testing.T, mode attr.Value, ids attr.Value) resource_user_notification.AllocationsValue {
	t.Helper()
	return resource_user_notification.NewAllocationsValueMust(
		resource_user_notification.AllocationsValue{}.AttributeTypes(t.Context()),
		map[string]attr.Value{"mode": mode, "ids": ids},
	)
}

func stringList(t *testing.T, elements ...attr.Value) types.List {
	t.Helper()
	l, diags := types.ListValue(types.StringType, elements)
	if diags.HasError() {
		t.Fatalf("list: %v", diags)
	}
	return l
}

func assertSingleError(t *testing.T, diags diag.Diagnostics, summary, pathSuffix string) {
	t.Helper()
	if summary == "" {
		if diags.HasError() {
			t.Errorf("unexpected diagnostics: %v", diags)
		}
		return
	}
	if diags.ErrorsCount() != 1 {
		t.Fatalf("errors = %d, want 1: %v", diags.ErrorsCount(), diags)
	}
	d := diags.Errors()[0]
	if d.Summary() != summary {
		t.Errorf("summary = %q, want %q", d.Summary(), summary)
	}
	withPath, ok := d.(diag.DiagnosticWithPath)
	if !ok {
		t.Fatalf("diagnostic has no path: %v", d)
	}
	if got := withPath.Path().String(); !strings.HasSuffix(got, pathSuffix) {
		t.Errorf("path = %q, want suffix %q", got, pathSuffix)
	}
}

func TestValidateUserNotificationAllocations(t *testing.T) {
	a, b := types.StringValue("a"), types.StringValue("b")
	unknown := types.StringUnknown()

	tests := []struct {
		name    string
		value   resource_user_notification.AllocationsValue
		summary string
	}{
		{"null object", resource_user_notification.NewAllocationsValueNull(), ""},
		{"unknown object", resource_user_notification.NewAllocationsValueUnknown(), ""},
		{"unknown ids", testAllocationsValue(t, types.StringValue("include"), types.ListUnknown(types.StringType)), ""},
		{"unknown mode with ids", testAllocationsValue(t, types.StringUnknown(), stringList(t, a)), ""},
		{"unknown mode with duplicate ids", testAllocationsValue(t, types.StringUnknown(), stringList(t, a, a)), "Duplicate Allocation ID"},
		{"all empty", testAllocationsValue(t, types.StringValue("all"), stringList(t)), ""},
		{"all with ids", testAllocationsValue(t, types.StringValue("all"), stringList(t, a)), "Invalid Allocation IDs"},
		{"all with unknown id", testAllocationsValue(t, types.StringValue("all"), stringList(t, unknown)), "Invalid Allocation IDs"},
		{"include empty", testAllocationsValue(t, types.StringValue("include"), stringList(t)), "Missing Allocation IDs"},
		{"exclude empty", testAllocationsValue(t, types.StringValue("exclude"), stringList(t)), "Missing Allocation IDs"},
		{"include unknown id", testAllocationsValue(t, types.StringValue("include"), stringList(t, unknown)), ""},
		{"include ids", testAllocationsValue(t, types.StringValue("include"), stringList(t, a, b)), ""},
		{"duplicate before unknown", testAllocationsValue(t, types.StringValue("include"), stringList(t, a, a, unknown)), "Duplicate Allocation ID"},
		{"duplicate after unknown", testAllocationsValue(t, types.StringValue("include"), stringList(t, unknown, a, a)), "Duplicate Allocation ID"},
		// Null and empty IDs are rejected by the API, and a null element cannot
		// be converted to the request's []string.
		{"null id", testAllocationsValue(t, types.StringValue("include"), stringList(t, types.StringNull())), "Invalid Allocation ID"},
		{"empty id", testAllocationsValue(t, types.StringValue("include"), stringList(t, a, types.StringValue(""))), "Invalid Allocation ID"},
		{"null id after unknown", testAllocationsValue(t, types.StringValue("include"), stringList(t, unknown, types.StringNull())), "Invalid Allocation ID"},
		{"empty id before unknown", testAllocationsValue(t, types.StringValue("exclude"), stringList(t, types.StringValue(""), unknown)), "Invalid Allocation ID"},
		{"null id with unknown mode", testAllocationsValue(t, types.StringUnknown(), stringList(t, types.StringNull())), "Invalid Allocation ID"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var diags diag.Diagnostics
			validateUserNotificationAllocations(tt.value, &diags)
			assertSingleError(t, diags, tt.summary, "allocations.ids")
		})
	}
}

func testWhenCostGrowsValue(t *testing.T, match attr.Value, percent attr.Value, minIncrease attr.Value) resource_user_notification.WhenCostGrowsValue {
	t.Helper()
	return resource_user_notification.NewWhenCostGrowsValueMust(
		resource_user_notification.WhenCostGrowsValue{}.AttributeTypes(t.Context()),
		map[string]attr.Value{
			"match":            match,
			"percent_increase": percent,
			"min_increase":     minIncrease,
			"within_hours":     types.Int64Null(),
		},
	)
}

func testMinIncreaseValue(t *testing.T) resource_user_notification.MinIncreaseValue {
	t.Helper()
	return resource_user_notification.NewMinIncreaseValueMust(
		resource_user_notification.MinIncreaseValue{}.AttributeTypes(t.Context()),
		map[string]attr.Value{"amount": types.StringValue("10"), "currency": types.StringValue("USD")},
	)
}

func TestValidateUserNotificationRepeat(t *testing.T) {
	repeat := func(everyHours attr.Value, wcg attr.Value) resource_user_notification.RepeatValue {
		return resource_user_notification.NewRepeatValueMust(
			resource_user_notification.RepeatValue{}.AttributeTypes(t.Context()),
			map[string]attr.Value{"every_hours": everyHours, "when_cost_grows": wcg},
		)
	}
	wcg := testWhenCostGrowsValue(t, types.StringNull(), types.Int64Value(5), resource_user_notification.NewMinIncreaseValueNull())

	tests := []struct {
		name    string
		value   resource_user_notification.RepeatValue
		summary string
	}{
		{"null", resource_user_notification.NewRepeatValueNull(), ""},
		{"unknown", resource_user_notification.NewRepeatValueUnknown(), ""},
		{"empty", repeat(types.Int64Null(), resource_user_notification.NewWhenCostGrowsValueNull()), "Empty Repeat Configuration"},
		{"unknown every_hours", repeat(types.Int64Unknown(), resource_user_notification.NewWhenCostGrowsValueNull()), ""},
		{"unknown when_cost_grows", repeat(types.Int64Null(), resource_user_notification.NewWhenCostGrowsValueUnknown()), ""},
		{"every_hours only", repeat(types.Int64Value(12), resource_user_notification.NewWhenCostGrowsValueNull()), ""},
		{"when_cost_grows only", repeat(types.Int64Null(), wcg), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var diags diag.Diagnostics
			validateUserNotificationRepeat(tt.value, &diags)
			assertSingleError(t, diags, tt.summary, "repeat")
		})
	}
}

func TestValidateUserNotificationWhenCostGrows(t *testing.T) {
	noMin := resource_user_notification.NewMinIncreaseValueNull()
	minInc := testMinIncreaseValue(t)

	tests := []struct {
		name    string
		value   resource_user_notification.WhenCostGrowsValue
		summary string
		path    string
	}{
		{"null", resource_user_notification.NewWhenCostGrowsValueNull(), "", ""},
		{"unknown", resource_user_notification.NewWhenCostGrowsValueUnknown(), "", ""},
		{"no threshold", testWhenCostGrowsValue(t, types.StringNull(), types.Int64Null(), noMin), "Missing Cost Growth Threshold", "when_cost_grows"},
		{"unknown percent", testWhenCostGrowsValue(t, types.StringNull(), types.Int64Unknown(), noMin), "", ""},
		{"unknown min_increase", testWhenCostGrowsValue(t, types.StringNull(), types.Int64Null(), resource_user_notification.NewMinIncreaseValueUnknown()), "", ""},
		{"percent only", testWhenCostGrowsValue(t, types.StringNull(), types.Int64Value(5), noMin), "", ""},
		{"min_increase only", testWhenCostGrowsValue(t, types.StringNull(), types.Int64Null(), minInc), "", ""},
		// The API accepts and echoes match with a single threshold.
		{"one threshold with match", testWhenCostGrowsValue(t, types.StringValue("or"), types.Int64Value(5), noMin), "", ""},
		{"both without match", testWhenCostGrowsValue(t, types.StringNull(), types.Int64Value(5), minInc), "Missing Match", "when_cost_grows.match"},
		{"both with unknown match", testWhenCostGrowsValue(t, types.StringUnknown(), types.Int64Value(5), minInc), "", ""},
		// No value of match can make a rule without thresholds valid.
		{"no threshold with unknown match", testWhenCostGrowsValue(t, types.StringUnknown(), types.Int64Null(), noMin), "Missing Cost Growth Threshold", "when_cost_grows"},
		{"one threshold with unknown match", testWhenCostGrowsValue(t, types.StringUnknown(), types.Int64Value(5), noMin), "", ""},
		{"unknown percent with null match", testWhenCostGrowsValue(t, types.StringNull(), types.Int64Unknown(), minInc), "", ""},
		{"both with match", testWhenCostGrowsValue(t, types.StringValue("and"), types.Int64Value(5), minInc), "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var diags diag.Diagnostics
			validateUserNotificationWhenCostGrows(tt.value, &diags)
			assertSingleError(t, diags, tt.summary, tt.path)
		})
	}
}

func TestUserNotificationAmountRangeValidator(t *testing.T) {
	tests := []struct {
		name     string
		value    types.String
		positive bool
		wantErr  string
	}{
		{"null", types.StringNull(), true, ""},
		{"unknown", types.StringUnknown(), true, ""},
		{"malformed is left to the pattern validator", types.StringValue("abc"), true, ""},
		{"zero excess cost", types.StringValue("0"), false, ""},
		{"zero excess cost with decimals", types.StringValue("0.00"), false, ""},
		{"zero increase", types.StringValue("0"), true, "must be greater than 0"},
		{"zero increase with decimals", types.StringValue("0.00"), true, "must be greater than 0"},
		{"smallest increase", types.StringValue("1"), true, ""},
		{"maximum", types.StringValue("1000000000"), false, ""},
		{"maximum with decimals", types.StringValue("1000000000.00"), true, ""},
		{"above maximum", types.StringValue("1000000001"), false, "must be at most 1000000000 US dollars"},
		{"above maximum increase", types.StringValue("1000000001"), true, "must be at most 1000000000 US dollars"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := path.Root("settings").AtName("cost_anomalies").AtName("min_excess_cost").AtName("amount")
			resp := &validator.StringResponse{}
			costAnomalyAmountRangeValidator{positive: tt.positive}.ValidateString(t.Context(), validator.StringRequest{Path: p, ConfigValue: tt.value}, resp)

			if tt.wantErr == "" {
				if resp.Diagnostics.HasError() {
					t.Errorf("unexpected diagnostics: %v", resp.Diagnostics)
				}
				return
			}
			if resp.Diagnostics.ErrorsCount() != 1 {
				t.Fatalf("errors = %d, want 1: %v", resp.Diagnostics.ErrorsCount(), resp.Diagnostics)
			}
			d := resp.Diagnostics.Errors()[0]
			if d.Summary() != "Invalid Amount" || !strings.Contains(d.Detail(), tt.wantErr) {
				t.Errorf("diagnostic = %q / %q, want detail containing %q", d.Summary(), d.Detail(), tt.wantErr)
			}
			if withPath, ok := d.(diag.DiagnosticWithPath); !ok || !withPath.Path().Equal(p) {
				t.Errorf("diagnostic path mismatch: %v", d)
			}
		})
	}
}
