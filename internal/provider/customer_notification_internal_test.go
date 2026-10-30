package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_customer_notification"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// testCustomerNotificationResponse returns an API response with every optional
// member set. Callers trim it per test.
func testCustomerNotificationResponse() *models.CustomerNotification {
	createTime := time.Date(2026, 10, 1, 8, 0, 0, 123000, time.UTC)
	updateTime := time.Date(2026, 10, 2, 9, 30, 0, 0, time.UTC)

	return &models.CustomerNotification{
		Id:         valueToNullable("notif-1"),
		Name:       "Anomaly alerts",
		CreateTime: valueToNullable(createTime),
		UpdateTime: valueToNullable(updateTime),
		Etag:       valueToNullable("etag-1"),
		Destinations: models.CustomerNotificationDestinations{
			Emails:        []openapi_types.Email{"finops@example.com", "ops@example.com"},
			SlackChannels: []string{"C0123ABCD"},
			TeamsChannels: []string{},
		},
		Settings: models.CustomerNotificationSettings{
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

func mustMapCustomerNotification(t *testing.T, resp *models.CustomerNotification, state *customerNotificationResourceModel) {
	t.Helper()
	if diags := mapCustomerNotificationToModel(t.Context(), resp, state); diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
}

// customerNotificationPlanFromResponse builds a fully typed plan model the same way Read does.
func customerNotificationPlanFromResponse(t *testing.T, resp *models.CustomerNotification) customerNotificationResourceModel {
	t.Helper()
	var plan customerNotificationResourceModel
	mustMapCustomerNotification(t, resp, &plan)
	return plan
}

func stringListValues(t *testing.T, list types.List) []string {
	t.Helper()
	var out []string
	if diags := list.ElementsAs(t.Context(), &out, false); diags.HasError() {
		t.Fatalf("list: %v", diags)
	}
	return out
}

func TestMapCustomerNotificationToModel(t *testing.T) {
	var state customerNotificationResourceModel
	mustMapCustomerNotification(t, testCustomerNotificationResponse(), &state)

	if state.Id.ValueString() != "notif-1" || state.Name.ValueString() != "Anomaly alerts" || state.Etag.ValueString() != "etag-1" {
		t.Errorf("scalars not mapped: %+v", state.CustomerNotificationModel)
	}
	if state.CreateTime.IsNull() || state.UpdateTime.IsNull() {
		t.Errorf("timestamps not mapped: %v, %v", state.CreateTime, state.UpdateTime)
	}
	d := state.Destinations
	if got := stringListValues(t, d.Emails); len(got) != 2 || got[0] != "finops@example.com" || got[1] != "ops@example.com" {
		t.Errorf("emails = %v", got)
	}
	if got := stringListValues(t, d.SlackChannels); len(got) != 1 || got[0] != "C0123ABCD" {
		t.Errorf("slack = %v", got)
	}
	ca := state.Settings.CostAnomalies
	if ca.MinSeverity.ValueString() != "critical" || ca.MinExcessCost.Amount.ValueString() != "100.00" || ca.Allocations.Mode.ValueString() != "include" {
		t.Errorf("cost anomalies = %+v", ca)
	}
	if ca.Repeat.EveryHours.ValueInt64() != 24 || ca.Repeat.WhenCostGrows.WithinHours.ValueInt64() != 6 || ca.Repeat.WhenCostGrows.MinIncrease.Amount.ValueString() != "150.00" {
		t.Errorf("repeat = %+v", ca.Repeat)
	}
}

// Unused destination types arrive as [] (or, defensively, nil) and must be an
// empty list in state, never null; an absent repeat is null.
func TestMapCustomerNotificationToModel_EmptyDestinationsAndNoRepeat(t *testing.T) {
	resp := testCustomerNotificationResponse()
	resp.Destinations = models.CustomerNotificationDestinations{Emails: []openapi_types.Email{"a@example.com"}}
	resp.Settings.CostAnomalies.Repeat = nil

	var state customerNotificationResourceModel
	mustMapCustomerNotification(t, resp, &state)

	for name, list := range map[string]types.List{"slack": state.Destinations.SlackChannels, "teams": state.Destinations.TeamsChannels} {
		if list.IsNull() || list.IsUnknown() || len(list.Elements()) != 0 {
			t.Errorf("%s = %v, want empty list", name, list)
		}
	}
	if !state.Settings.CostAnomalies.Repeat.IsNull() {
		t.Errorf("repeat = %v, want null", state.Settings.CostAnomalies.Repeat)
	}
}

// The API lowercases emails. A prior spelling that differs only in case is kept
// per position; a different address, or an import without prior state, takes the API's.
func TestMapCustomerNotificationToModel_EmailCase(t *testing.T) {
	prior := customerNotificationPlanFromResponse(t, testCustomerNotificationResponse())
	emails, diags := types.ListValueFrom(t.Context(), types.StringType, []string{"FinOps@Example.com", "other@example.com"})
	if diags.HasError() {
		t.Fatal(diags)
	}
	prior.Destinations.Emails = emails

	mustMapCustomerNotification(t, testCustomerNotificationResponse(), &prior)
	got := stringListValues(t, prior.Destinations.Emails)
	if got[0] != "FinOps@Example.com" {
		t.Errorf("email 0 = %q, want the configured spelling", got[0])
	}
	if got[1] != "ops@example.com" {
		t.Errorf("email 1 = %q, want the API value for a different address", got[1])
	}

	// A longer API list than the prior one must not panic and takes the API values.
	var fresh customerNotificationResourceModel
	mustMapCustomerNotification(t, testCustomerNotificationResponse(), &fresh)
	if got := stringListValues(t, fresh.Destinations.Emails); got[0] != "finops@example.com" {
		t.Errorf("import email = %q", got[0])
	}
}

func TestMapCustomerNotificationToModel_AmountSpelling(t *testing.T) {
	var state customerNotificationResourceModel
	mustMapCustomerNotification(t, testCustomerNotificationResponse(), &state)
	state.Settings.CostAnomalies.MinExcessCost.Amount = types.StringValue("100")

	mustMapCustomerNotification(t, testCustomerNotificationResponse(), &state)
	if got := state.Settings.CostAnomalies.MinExcessCost.Amount.ValueString(); got != "100" {
		t.Errorf("amount = %q, want the configured spelling", got)
	}
}

func TestOverlayCustomerNotificationComputedFields(t *testing.T) {
	// The plan carries the configured values; the response the API's.
	var plan customerNotificationResourceModel
	mustMapCustomerNotification(t, testCustomerNotificationResponse(), &plan)
	plan.Settings.CostAnomalies.MinExcessCost.Amount = types.StringValue("100")
	emails, diags := types.ListValueFrom(t.Context(), types.StringType, []string{"FinOps@Example.com", "ops@example.com"})
	if diags.HasError() {
		t.Fatal(diags)
	}
	plan.Destinations.Emails = emails
	plan.Id = types.StringUnknown()
	plan.CreateTime = types.StringUnknown()
	plan.UpdateTime = types.StringUnknown()
	plan.Etag = types.StringUnknown()

	if diags := overlayCustomerNotificationComputedFields(t.Context(), testCustomerNotificationResponse(), &plan); diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if plan.Id.ValueString() != "notif-1" || plan.Etag.ValueString() != "etag-1" || plan.UpdateTime.IsUnknown() || plan.CreateTime.IsUnknown() {
		t.Errorf("computed fields not resolved: %+v", plan.CustomerNotificationModel)
	}
	if got := plan.Settings.CostAnomalies.MinExcessCost.Amount.ValueString(); got != "100" {
		t.Errorf("amount = %q, want the planned spelling 100", got)
	}
	if got := stringListValues(t, plan.Destinations.Emails); got[0] != "FinOps@Example.com" {
		t.Errorf("email = %q, want the planned spelling", got[0])
	}

	// On Update, known update_time and etag are kept (the next Read refreshes them).
	plan.UpdateTime = types.StringValue("2026-01-01T00:00:00Z")
	plan.Etag = types.StringValue("etag-old")
	if diags := overlayCustomerNotificationComputedFields(t.Context(), testCustomerNotificationResponse(), &plan); diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if plan.UpdateTime.ValueString() != "2026-01-01T00:00:00Z" || plan.Etag.ValueString() != "etag-old" {
		t.Errorf("known update_time/etag overwritten: %q, %q", plan.UpdateTime.ValueString(), plan.Etag.ValueString())
	}
}

func TestCustomerNotificationToCreateRequest(t *testing.T) {
	t.Run("full", func(t *testing.T) {
		plan := customerNotificationPlanFromResponse(t, testCustomerNotificationResponse())
		req, diags := plan.toCreateRequest(t.Context())
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		assertJSONEqual(t, req, `{
			"name": "Anomaly alerts",
			"destinations": {"emails": ["finops@example.com", "ops@example.com"], "slackChannels": ["C0123ABCD"], "teamsChannels": []},
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

	// Unused destination lists and ids are sent as [], never omitted: the API requires them.
	t.Run("minimal", func(t *testing.T) {
		resp := testCustomerNotificationResponse()
		resp.Destinations = models.CustomerNotificationDestinations{TeamsChannels: []string{"19:abc@thread.tacv2"}}
		resp.Settings.CostAnomalies.Repeat = nil
		resp.Settings.CostAnomalies.Allocations = models.CostAnomalyAllocations{Mode: "all"}
		plan := customerNotificationPlanFromResponse(t, resp)

		req, diags := plan.toCreateRequest(t.Context())
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		assertJSONEqual(t, req, `{
			"name": "Anomaly alerts",
			"destinations": {"emails": [], "slackChannels": [], "teamsChannels": ["19:abc@thread.tacv2"]},
			"settings": {"costAnomalies": {
				"minSeverity": "critical",
				"minExcessCost": {"amount": "100.00", "currency": "USD"},
				"allocations": {"mode": "all", "ids": []}
			}}
		}`)
	})
}

func TestCustomerNotificationToUpdateRequest(t *testing.T) {
	// The patch carries the full desired state: destinations replace the stored
	// object, and every optional repeat member is explicit.
	t.Run("full", func(t *testing.T) {
		plan := customerNotificationPlanFromResponse(t, testCustomerNotificationResponse())
		req, diags := plan.toUpdateRequest(t.Context())
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		assertJSONEqual(t, req, `{
			"name": "Anomaly alerts",
			"destinations": {"emails": ["finops@example.com", "ops@example.com"], "slackChannels": ["C0123ABCD"], "teamsChannels": []},
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

	t.Run("repeat removed", func(t *testing.T) {
		resp := testCustomerNotificationResponse()
		resp.Settings.CostAnomalies.Repeat = nil
		plan := customerNotificationPlanFromResponse(t, resp)
		req, diags := plan.toUpdateRequest(t.Context())
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		assertJSONEqual(t, req, `{
			"name": "Anomaly alerts",
			"destinations": {"emails": ["finops@example.com", "ops@example.com"], "slackChannels": ["C0123ABCD"], "teamsChannels": []},
			"settings": {"costAnomalies": {
				"minSeverity": "critical",
				"minExcessCost": {"amount": "100.00", "currency": "USD"},
				"allocations": {"mode": "include", "ids": ["alloc-b", "alloc-a"]},
				"repeat": null
			}}
		}`)
	})

	t.Run("leaves cleared", func(t *testing.T) {
		resp := testCustomerNotificationResponse()
		resp.Settings.CostAnomalies.Repeat = &models.CostAnomalyRepeat{
			WhenCostGrows: &models.CostAnomalyWhenCostGrows{PercentIncrease: new(5)},
		}
		plan := customerNotificationPlanFromResponse(t, resp)
		req, diags := plan.toUpdateRequest(t.Context())
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		assertJSONEqual(t, req, `{
			"name": "Anomaly alerts",
			"destinations": {"emails": ["finops@example.com", "ops@example.com"], "slackChannels": ["C0123ABCD"], "teamsChannels": []},
			"settings": {"costAnomalies": {
				"minSeverity": "critical",
				"minExcessCost": {"amount": "100.00", "currency": "USD"},
				"allocations": {"mode": "include", "ids": ["alloc-b", "alloc-a"]},
				"repeat": {
					"everyHours": null,
					"whenCostGrows": {"match": null, "percentIncrease": 5, "minIncrease": null, "withinHours": null}
				}
			}}
		}`)
	})
}

func TestCustomerNotificationImportState(t *testing.T) {
	r := &customerNotificationResource{}
	schemaResp := &resource.SchemaResponse{}
	r.Schema(t.Context(), resource.SchemaRequest{}, schemaResp)

	state := newTestState(t.Context(), t, r, nil)
	resp := &resource.ImportStateResponse{State: state}
	r.ImportState(t.Context(), resource.ImportStateRequest{ID: "notif-1"}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}
	var id types.String
	if diags := resp.State.GetAttribute(t.Context(), path.Root("id"), &id); diags.HasError() || id.ValueString() != "notif-1" {
		t.Errorf("id = %v, diags = %v", id, diags)
	}
}

func TestCustomerNotification_PopulateState_NotFound(t *testing.T) {
	srv := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/customers/v1/notifications/missing" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message": "Not Found"}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))

	res := &customerNotificationResource{client: newTestAPIClient(t, srv)}
	state := customerNotificationResourceModel{
		Id: types.StringValue("missing"),
	}

	if diags := res.populateState(t.Context(), &state); diags.HasError() {
		t.Fatalf("populateState should not return error on 404, got: %v", diags)
	}
	if !state.Id.IsNull() {
		t.Errorf("expected state.Id to be null on 404, got %q", state.Id.ValueString())
	}
}

// TestCustomerNotificationResourceDelete runs every case through the plain test
// client and through DCIRetryClient, which production uses: a status the retry
// client turns into an error never reaches the resource's status handling.
func TestCustomerNotificationResourceDelete(t *testing.T) {
	const path = "/customers/v1/notifications/notif-1"

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
				r := &customerNotificationResource{client: newClient(t, srv)}
				resp := &resource.DeleteResponse{}
				values := map[string]string{"id": "notif-1"}
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

func testCustomerAllocationsValue(t *testing.T, mode attr.Value, ids attr.Value) resource_customer_notification.AllocationsValue {
	t.Helper()
	return resource_customer_notification.NewAllocationsValueMust(
		resource_customer_notification.AllocationsValue{}.AttributeTypes(t.Context()),
		map[string]attr.Value{"mode": mode, "ids": ids},
	)
}

func TestValidateCustomerNotificationAllocations(t *testing.T) {
	a, b := types.StringValue("a"), types.StringValue("b")
	unknown := types.StringUnknown()

	tests := []struct {
		name    string
		value   resource_customer_notification.AllocationsValue
		summary string
	}{
		{"null object", resource_customer_notification.NewAllocationsValueNull(), ""},
		{"unknown object", resource_customer_notification.NewAllocationsValueUnknown(), ""},
		{"unknown ids", testCustomerAllocationsValue(t, types.StringValue("include"), types.ListUnknown(types.StringType)), ""},
		{"unknown mode with ids", testCustomerAllocationsValue(t, types.StringUnknown(), stringList(t, a)), ""},
		{"unknown mode with duplicate ids", testCustomerAllocationsValue(t, types.StringUnknown(), stringList(t, a, a)), "Duplicate Allocation ID"},
		{"all empty", testCustomerAllocationsValue(t, types.StringValue("all"), stringList(t)), ""},
		{"all with ids", testCustomerAllocationsValue(t, types.StringValue("all"), stringList(t, a)), "Invalid Allocation IDs"},
		{"all with unknown id", testCustomerAllocationsValue(t, types.StringValue("all"), stringList(t, unknown)), "Invalid Allocation IDs"},
		{"include empty", testCustomerAllocationsValue(t, types.StringValue("include"), stringList(t)), "Missing Allocation IDs"},
		{"exclude empty", testCustomerAllocationsValue(t, types.StringValue("exclude"), stringList(t)), "Missing Allocation IDs"},
		{"include unknown id", testCustomerAllocationsValue(t, types.StringValue("include"), stringList(t, unknown)), ""},
		{"include ids", testCustomerAllocationsValue(t, types.StringValue("include"), stringList(t, a, b)), ""},
		{"duplicate before unknown", testCustomerAllocationsValue(t, types.StringValue("include"), stringList(t, a, a, unknown)), "Duplicate Allocation ID"},
		{"duplicate after unknown", testCustomerAllocationsValue(t, types.StringValue("include"), stringList(t, unknown, a, a)), "Duplicate Allocation ID"},
		// Null and empty IDs are rejected by the API, and a null element cannot
		// be converted to the request's []string.
		{"null id", testCustomerAllocationsValue(t, types.StringValue("include"), stringList(t, types.StringNull())), "Invalid Allocation ID"},
		{"empty id", testCustomerAllocationsValue(t, types.StringValue("include"), stringList(t, a, types.StringValue(""))), "Invalid Allocation ID"},
		{"null id after unknown", testCustomerAllocationsValue(t, types.StringValue("include"), stringList(t, unknown, types.StringNull())), "Invalid Allocation ID"},
		{"empty id before unknown", testCustomerAllocationsValue(t, types.StringValue("exclude"), stringList(t, types.StringValue(""), unknown)), "Invalid Allocation ID"},
		{"null id with unknown mode", testCustomerAllocationsValue(t, types.StringUnknown(), stringList(t, types.StringNull())), "Invalid Allocation ID"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var diags diag.Diagnostics
			validateCustomerNotificationAllocations(tt.value, &diags)
			assertSingleError(t, diags, tt.summary, "allocations.ids")
		})
	}
}

func testCustomerWhenCostGrowsValue(t *testing.T, match attr.Value, percent attr.Value, minIncrease attr.Value) resource_customer_notification.WhenCostGrowsValue {
	t.Helper()
	return resource_customer_notification.NewWhenCostGrowsValueMust(
		resource_customer_notification.WhenCostGrowsValue{}.AttributeTypes(t.Context()),
		map[string]attr.Value{
			"match":            match,
			"percent_increase": percent,
			"min_increase":     minIncrease,
			"within_hours":     types.Int64Null(),
		},
	)
}

func testCustomerMinIncreaseValue(t *testing.T) resource_customer_notification.MinIncreaseValue {
	t.Helper()
	return resource_customer_notification.NewMinIncreaseValueMust(
		resource_customer_notification.MinIncreaseValue{}.AttributeTypes(t.Context()),
		map[string]attr.Value{"amount": types.StringValue("10"), "currency": types.StringValue("USD")},
	)
}

func TestValidateCustomerNotificationRepeat(t *testing.T) {
	repeat := func(everyHours attr.Value, wcg attr.Value) resource_customer_notification.RepeatValue {
		return resource_customer_notification.NewRepeatValueMust(
			resource_customer_notification.RepeatValue{}.AttributeTypes(t.Context()),
			map[string]attr.Value{"every_hours": everyHours, "when_cost_grows": wcg},
		)
	}
	wcg := testCustomerWhenCostGrowsValue(t, types.StringNull(), types.Int64Value(5), resource_customer_notification.NewMinIncreaseValueNull())

	tests := []struct {
		name    string
		value   resource_customer_notification.RepeatValue
		summary string
	}{
		{"null", resource_customer_notification.NewRepeatValueNull(), ""},
		{"unknown", resource_customer_notification.NewRepeatValueUnknown(), ""},
		{"empty", repeat(types.Int64Null(), resource_customer_notification.NewWhenCostGrowsValueNull()), "Empty Repeat Configuration"},
		{"unknown every_hours", repeat(types.Int64Unknown(), resource_customer_notification.NewWhenCostGrowsValueNull()), ""},
		{"unknown when_cost_grows", repeat(types.Int64Null(), resource_customer_notification.NewWhenCostGrowsValueUnknown()), ""},
		{"every_hours only", repeat(types.Int64Value(12), resource_customer_notification.NewWhenCostGrowsValueNull()), ""},
		{"when_cost_grows only", repeat(types.Int64Null(), wcg), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var diags diag.Diagnostics
			validateCustomerNotificationRepeat(tt.value, &diags)
			assertSingleError(t, diags, tt.summary, "repeat")
		})
	}
}

func TestValidateCustomerNotificationWhenCostGrows(t *testing.T) {
	noMin := resource_customer_notification.NewMinIncreaseValueNull()
	minInc := testCustomerMinIncreaseValue(t)

	tests := []struct {
		name    string
		value   resource_customer_notification.WhenCostGrowsValue
		summary string
		path    string
	}{
		{"null", resource_customer_notification.NewWhenCostGrowsValueNull(), "", ""},
		{"unknown", resource_customer_notification.NewWhenCostGrowsValueUnknown(), "", ""},
		{"no threshold", testCustomerWhenCostGrowsValue(t, types.StringNull(), types.Int64Null(), noMin), "Missing Cost Growth Threshold", "when_cost_grows"},
		{"unknown percent", testCustomerWhenCostGrowsValue(t, types.StringNull(), types.Int64Unknown(), noMin), "", ""},
		{"unknown min_increase", testCustomerWhenCostGrowsValue(t, types.StringNull(), types.Int64Null(), resource_customer_notification.NewMinIncreaseValueUnknown()), "", ""},
		{"percent only", testCustomerWhenCostGrowsValue(t, types.StringNull(), types.Int64Value(5), noMin), "", ""},
		{"min_increase only", testCustomerWhenCostGrowsValue(t, types.StringNull(), types.Int64Null(), minInc), "", ""},
		// The API accepts and echoes match with a single threshold.
		{"one threshold with match", testCustomerWhenCostGrowsValue(t, types.StringValue("or"), types.Int64Value(5), noMin), "", ""},
		{"both without match", testCustomerWhenCostGrowsValue(t, types.StringNull(), types.Int64Value(5), minInc), "Missing Match", "when_cost_grows.match"},
		{"both with unknown match", testCustomerWhenCostGrowsValue(t, types.StringUnknown(), types.Int64Value(5), minInc), "", ""},
		// No value of match can make a rule without thresholds valid.
		{"no threshold with unknown match", testCustomerWhenCostGrowsValue(t, types.StringUnknown(), types.Int64Null(), noMin), "Missing Cost Growth Threshold", "when_cost_grows"},
		{"one threshold with unknown match", testCustomerWhenCostGrowsValue(t, types.StringUnknown(), types.Int64Value(5), noMin), "", ""},
		{"unknown percent with null match", testCustomerWhenCostGrowsValue(t, types.StringNull(), types.Int64Unknown(), minInc), "", ""},
		{"both with match", testCustomerWhenCostGrowsValue(t, types.StringValue("and"), types.Int64Value(5), minInc), "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var diags diag.Diagnostics
			validateCustomerNotificationWhenCostGrows(tt.value, &diags)
			assertSingleError(t, diags, tt.summary, tt.path)
		})
	}
}

func testCustomerDestinationsValue(t *testing.T, emails, slack, teams attr.Value) resource_customer_notification.DestinationsValue {
	t.Helper()
	return resource_customer_notification.NewDestinationsValueMust(
		resource_customer_notification.DestinationsValue{}.AttributeTypes(t.Context()),
		map[string]attr.Value{"emails": emails, "slack_channels": slack, "teams_channels": teams},
	)
}

func TestValidateCustomerNotificationDestinations(t *testing.T) {
	empty := stringList(t)
	a, b := types.StringValue("a@example.com"), types.StringValue("A@Example.com")
	c := types.StringValue("C0123")
	unknown := types.StringUnknown()
	unknownList := types.ListUnknown(types.StringType)

	tests := []struct {
		name    string
		value   resource_customer_notification.DestinationsValue
		summary string
		path    string
	}{
		{"null object", resource_customer_notification.NewDestinationsValueNull(), "", ""},
		{"unknown object", resource_customer_notification.NewDestinationsValueUnknown(), "", ""},
		{"all empty", testCustomerDestinationsValue(t, empty, empty, empty), "No Destinations", "destinations"},
		{"email only", testCustomerDestinationsValue(t, stringList(t, a), empty, empty), "", ""},
		{"slack only", testCustomerDestinationsValue(t, empty, stringList(t, c), empty), "", ""},
		{"teams only", testCustomerDestinationsValue(t, empty, empty, stringList(t, c)), "", ""},
		{"unknown list may be non-empty", testCustomerDestinationsValue(t, empty, unknownList, empty), "", ""},
		{"unknown element counts as an entry", testCustomerDestinationsValue(t, stringList(t, unknown), empty, empty), "", ""},
		{"duplicate email ignoring case", testCustomerDestinationsValue(t, stringList(t, a, b), empty, empty), "Duplicate Destination", "destinations.emails"},
		{"duplicate slack channel", testCustomerDestinationsValue(t, empty, stringList(t, c, c), empty), "Duplicate Destination", "destinations.slack_channels"},
		{"channel ids compare exactly", testCustomerDestinationsValue(t, empty, stringList(t, c, types.StringValue("c0123")), empty), "", ""},
		{"same value in different lists", testCustomerDestinationsValue(t, empty, stringList(t, c), stringList(t, c)), "", ""},
		{"null entry", testCustomerDestinationsValue(t, stringList(t, types.StringNull()), empty, empty), "Invalid Destination", "destinations.emails"},
		{"blank entry", testCustomerDestinationsValue(t, empty, stringList(t, types.StringValue("  ")), empty), "Invalid Destination", "destinations.slack_channels"},
		{"duplicate next to unknown", testCustomerDestinationsValue(t, stringList(t, a, unknown, a), empty, empty), "Duplicate Destination", "destinations.emails"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var diags diag.Diagnostics
			validateCustomerNotificationDestinations(tt.value, &diags)
			assertSingleError(t, diags, tt.summary, tt.path)
		})
	}
}

func TestCustomerNotificationTrimmedValidator(t *testing.T) {
	tests := []struct {
		name    string
		value   types.String
		wantErr bool
	}{
		{"null", types.StringNull(), false},
		{"unknown", types.StringUnknown(), false},
		{"plain", types.StringValue("C0123ABCD"), false},
		{"inner space", types.StringValue("My notification"), false},
		{"leading space", types.StringValue(" a"), true},
		{"trailing newline", types.StringValue("a\n"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &validator.StringResponse{}
			customerNotificationTrimmedValidator{}.ValidateString(t.Context(), validator.StringRequest{Path: path.Root("name"), ConfigValue: tt.value}, resp)
			if got := resp.Diagnostics.HasError(); got != tt.wantErr {
				t.Errorf("hasError = %v, want %v: %v", got, tt.wantErr, resp.Diagnostics)
			}
		})
	}
}
