package provider

import (
	"context"
	"maps"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_customer_subscription"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// testCustomerSubscriptionResponse returns an API response with a dashboard
// and a report entry, ordered by type like the API does.
func testCustomerSubscriptionResponse() *models.CustomerSubscription {
	createdBy := openapi_types.Email("owner@example.com")
	nextAt := time.Date(2026, 11, 10, 8, 0, 0, 0, time.UTC)
	permissions := &models.CustomerSubscriptionPermissions{CanDelete: true, CanEditRecipients: true}

	return &models.CustomerSubscription{
		Id:        valueToNullable("sub-1"),
		Name:      new("Executive costs (Dashboard Subscription)"),
		CreatedBy: &createdBy,
		Etag:      valueToNullable("etag-1"),
		Recipients: models.CustomerSubscriptionRecipients{
			Emails:        []openapi_types.Email{"finops@example.com"},
			SlackChannels: []string{"C0123ABCD"},
		},
		Subscriptions: []models.CustomerSubscriptionEntry{
			{
				Type:   models.CustomerSubscriptionTypeDashboardSubscription,
				Source: models.CustomerSubscriptionSource{Id: "dash-1", Name: new("Executive costs")},
				Schedule: &models.CustomerSubscriptionSchedule{
					Frequency: models.CustomerSubscriptionFrequencyWeekly,
					StartDate: time.Date(2026, 11, 3, 8, 0, 0, 0, time.UTC),
					Time:      time.Date(2026, 11, 3, 8, 0, 0, 0, time.UTC),
					TimeZone:  "America/Los_Angeles",
					NextAt:    &nextAt,
				},
				Permissions: permissions,
			},
			{
				Type:   models.CustomerSubscriptionTypeReportSubscription,
				Source: models.CustomerSubscriptionSource{Id: "report-1", Name: new("Monthly costs")},
				Schedule: &models.CustomerSubscriptionSchedule{
					Frequency:          models.CustomerSubscriptionFrequencyCustom,
					CustomPeriodAmount: new(2),
					CustomPeriodUnit:   new(models.CustomerSubscriptionCustomPeriodUnitWeek),
					StartDate:          time.Date(2026, 11, 3, 8, 0, 0, 0, time.UTC),
					Time:               time.Date(2026, 11, 3, 8, 0, 0, 0, time.UTC),
					TimeZone:           "America/Los_Angeles",
					NextAt:             &nextAt,
				},
				Attachments: &[]models.CustomerSubscriptionAttachment{models.CustomerSubscriptionAttachmentPdf},
				PdfSettings: &models.CustomerSubscriptionPdfSettings{ShowFilters: true, IncludeTable: false, IncludeAnnotations: true},
				Permissions: permissions,
			},
		},
	}
}

func mustMapCustomerSubscription(t *testing.T, resp *models.CustomerSubscription, state *customerSubscriptionResourceModel) {
	t.Helper()
	if diags := mapCustomerSubscriptionToModel(t.Context(), resp, state); diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
}

// customerSubscriptionPlanFromResponse builds a fully typed plan model the same way Read does.
func customerSubscriptionPlanFromResponse(t *testing.T, resp *models.CustomerSubscription) customerSubscriptionResourceModel {
	t.Helper()
	var plan customerSubscriptionResourceModel
	mustMapCustomerSubscription(t, resp, &plan)
	return plan
}

// customerSubscriptionEntries returns the set elements keyed by type.
func customerSubscriptionEntries(t *testing.T, set types.Set) map[string]resource_customer_subscription.SubscriptionsValue {
	t.Helper()
	var entries []resource_customer_subscription.SubscriptionsValue
	if diags := set.ElementsAs(t.Context(), &entries, false); diags.HasError() {
		t.Fatalf("subscriptions: %v", diags)
	}
	out := map[string]resource_customer_subscription.SubscriptionsValue{}
	for _, entry := range entries {
		out[entry.SubscriptionsType.ValueString()] = entry
	}
	return out
}

// objectAttributes returns the attributes of a generated object value.
func objectAttributes(t *testing.T, v interface {
	ToObjectValue(context.Context) (types.Object, diag.Diagnostics)
}) map[string]attr.Value {
	t.Helper()
	obj, diags := v.ToObjectValue(t.Context())
	if diags.HasError() {
		t.Fatalf("object: %v", diags)
	}
	return obj.Attributes()
}

// withCustomerSubscriptionEntry replaces the entry of the given type, after
// applying the overrides to its attributes and to those of its schedule and
// source. The other entries keep their position: request builders send the
// entries in set order.
func withCustomerSubscriptionEntry(t *testing.T, set types.Set, entryType string, entry, schedule, source map[string]attr.Value) types.Set {
	t.Helper()
	ctx := t.Context()
	var values []resource_customer_subscription.SubscriptionsValue
	if diags := set.ElementsAs(ctx, &values, false); diags.HasError() {
		t.Fatalf("subscriptions: %v", diags)
	}

	found := false
	for i, current := range values {
		if current.SubscriptionsType.ValueString() != entryType {
			continue
		}
		found = true
		attrs := objectAttributes(t, current)
		if len(schedule) > 0 {
			scheduleAttrs := objectAttributes(t, current.Schedule)
			maps.Copy(scheduleAttrs, schedule)
			attrs["schedule"] = resource_customer_subscription.NewScheduleValueMust(resource_customer_subscription.ScheduleValue{}.AttributeTypes(ctx), scheduleAttrs)
		}
		if len(source) > 0 {
			sourceAttrs := objectAttributes(t, current.Source)
			maps.Copy(sourceAttrs, source)
			attrs["source"] = resource_customer_subscription.NewSourceValueMust(resource_customer_subscription.SourceValue{}.AttributeTypes(ctx), sourceAttrs)
		}
		maps.Copy(attrs, entry)
		values[i] = resource_customer_subscription.NewSubscriptionsValueMust(resource_customer_subscription.SubscriptionsValue{}.AttributeTypes(ctx), attrs)
	}
	if !found {
		t.Fatalf("no %s entry in %v", entryType, set)
	}

	out, diags := types.SetValueFrom(ctx, customerSubscriptionEntriesType(ctx), values)
	if diags.HasError() {
		t.Fatalf("set: %v", diags)
	}
	return out
}

func TestMapCustomerSubscriptionToModel(t *testing.T) {
	var state customerSubscriptionResourceModel
	mustMapCustomerSubscription(t, testCustomerSubscriptionResponse(), &state)

	if state.Id.ValueString() != "sub-1" || state.Etag.ValueString() != "etag-1" || state.CreatedBy.ValueString() != "owner@example.com" ||
		state.Name.ValueString() != "Executive costs (Dashboard Subscription)" {
		t.Errorf("scalars not mapped: %+v", state.CustomerSubscriptionModel)
	}
	if got := stringListValues(t, state.Recipients.Emails); len(got) != 1 || got[0] != "finops@example.com" {
		t.Errorf("emails = %v", got)
	}
	if got := stringListValues(t, state.Recipients.SlackChannels); len(got) != 1 || got[0] != "C0123ABCD" {
		t.Errorf("slack = %v", got)
	}

	entries := customerSubscriptionEntries(t, state.Subscriptions)
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(entries))
	}

	dashboard := entries["dashboard-subscription"]
	if dashboard.Source.Id.ValueString() != "dash-1" || dashboard.Source.Name.ValueString() != "Executive costs" {
		t.Errorf("dashboard source = %+v", dashboard.Source)
	}
	if s := dashboard.Schedule; s.Frequency.ValueString() != "Weekly" || s.Time.ValueString() != "2026-11-03T08:00:00Z" ||
		s.StartDate.ValueString() != "2026-11-03T08:00:00Z" || s.NextAt.ValueString() != "2026-11-10T08:00:00Z" ||
		!s.CustomPeriodAmount.IsNull() || !s.CustomPeriodUnit.IsNull() {
		t.Errorf("dashboard schedule = %+v", s)
	}
	// The API omits report options on other types: attachments is a clearable
	// list, so it maps to [] like an empty one; pdf_settings is null.
	if got := stringListValues(t, dashboard.Attachments); dashboard.Attachments.IsNull() || len(got) != 0 {
		t.Errorf("dashboard attachments = %v, want []", dashboard.Attachments)
	}
	if !dashboard.PdfSettings.IsNull() {
		t.Errorf("dashboard pdf_settings = %v, want null", dashboard.PdfSettings)
	}
	if !dashboard.Permissions.CanDelete.ValueBool() || dashboard.Permissions.CanUnsubscribe.ValueBool() {
		t.Errorf("dashboard permissions = %+v", dashboard.Permissions)
	}

	report := entries["report-subscription"]
	if s := report.Schedule; s.CustomPeriodAmount.ValueInt64() != 2 || s.CustomPeriodUnit.ValueString() != "Week" {
		t.Errorf("report custom period = %v %v", s.CustomPeriodAmount, s.CustomPeriodUnit)
	}
	if got := stringListValues(t, report.Attachments); len(got) != 1 || got[0] != "pdf" {
		t.Errorf("report attachments = %v", got)
	}
	if p := report.PdfSettings; !p.ShowFilters.ValueBool() || p.IncludeTable.ValueBool() || !p.IncludeAnnotations.ValueBool() {
		t.Errorf("report pdf settings = %+v", p)
	}
}

// The API lowercases emails and returns timestamps in UTC. State keeps the
// configured spelling while the value is the same; entries are matched to
// their prior state by type, not by position.
func TestMapCustomerSubscriptionToModel_PreservesConfiguredSpelling(t *testing.T) {
	state := customerSubscriptionPlanFromResponse(t, testCustomerSubscriptionResponse())
	emails, diags := types.ListValueFrom(t.Context(), types.StringType, []string{"FinOps@Example.com"})
	if diags.HasError() {
		t.Fatal(diags)
	}
	state.Recipients.Emails = emails
	state.Subscriptions = withCustomerSubscriptionEntry(t, state.Subscriptions, "report-subscription", nil, map[string]attr.Value{
		"start_date": types.StringValue("2026-11-03T00:00:00-08:00"),
		"time":       types.StringValue("2026-11-03T10:00:00+02:00"),
	}, nil)
	state.Subscriptions = withCustomerSubscriptionEntry(t, state.Subscriptions, "dashboard-subscription", nil, map[string]attr.Value{
		"time": types.StringValue("2026-11-03T09:00:00+02:00"), // a different instant
	}, nil)

	resp := testCustomerSubscriptionResponse()
	resp.Subscriptions[0], resp.Subscriptions[1] = resp.Subscriptions[1], resp.Subscriptions[0]
	mustMapCustomerSubscription(t, resp, &state)

	if got := stringListValues(t, state.Recipients.Emails); got[0] != "FinOps@Example.com" {
		t.Errorf("email = %q, want the configured spelling", got[0])
	}
	entries := customerSubscriptionEntries(t, state.Subscriptions)
	if got := entries["report-subscription"].Schedule.StartDate.ValueString(); got != "2026-11-03T00:00:00-08:00" {
		t.Errorf("report start_date = %q, want the configured spelling", got)
	}
	if got := entries["report-subscription"].Schedule.Time.ValueString(); got != "2026-11-03T10:00:00+02:00" {
		t.Errorf("report time = %q, want the configured spelling", got)
	}
	if got := entries["dashboard-subscription"].Schedule.Time.ValueString(); got != "2026-11-03T08:00:00Z" {
		t.Errorf("dashboard time = %q, want the API value for a changed instant", got)
	}
	if got := entries["dashboard-subscription"].Schedule.StartDate.ValueString(); got != "2026-11-03T08:00:00Z" {
		t.Errorf("dashboard start_date = %q", got)
	}
}

func TestCustomerSubscriptionPopulateState(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      string
		wantNull  bool
		wantError string
	}{
		{name: "not found", status: 404, body: `{"message":"Not Found"}`, wantNull: true},
		{
			name:      "alert",
			status:    200,
			body:      `{"id":"alert-1","name":"Spend","createdBy":"a@example.com","etag":"e","recipients":{"emails":[],"slackChannels":[]},"subscriptions":[{"type":"alert","source":{"id":"alert-1","name":"Spend"},"permissions":{"canDelete":false,"canEditRecipients":true,"canUnsubscribe":false}}]}`,
			wantError: "Unsupported Subscription Type",
		},
		{
			name:      "budget",
			status:    200,
			body:      `{"id":"budget-1","name":"Budget","createdBy":"a@example.com","etag":"e","recipients":{"emails":[],"slackChannels":[]},"subscriptions":[{"type":"budget","source":{"id":"budget-1","name":"Budget"},"permissions":{"canDelete":false,"canEditRecipients":true,"canUnsubscribe":false}}]}`,
			wantError: "Unsupported Subscription Type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/customers/v1/subscriptions/id-1" {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))

			res := &customerSubscriptionResource{client: newTestAPIClient(t, srv)}
			var state customerSubscriptionResourceModel
			state.Id = types.StringValue("id-1")

			diags := res.populateState(t.Context(), &state)
			if tt.wantError != "" {
				if diags.ErrorsCount() != 1 || diags.Errors()[0].Summary() != tt.wantError {
					t.Fatalf("diagnostics = %v, want one %q error", diags, tt.wantError)
				}
				return
			}
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			if got := state.Id.IsNull(); got != tt.wantNull {
				t.Errorf("id null = %v, want %v", got, tt.wantNull)
			}
		})
	}
}

func TestOverlayCustomerSubscriptionComputedFields(t *testing.T) {
	// The plan carries the configured values; the response the API's.
	plan := customerSubscriptionPlanFromResponse(t, testCustomerSubscriptionResponse())
	plan.Id = types.StringUnknown()
	plan.Name = types.StringUnknown()
	plan.CreatedBy = types.StringUnknown()
	plan.Etag = types.StringUnknown()
	permissionsUnknown := resource_customer_subscription.NewPermissionsValueUnknown()
	plan.Subscriptions = withCustomerSubscriptionEntry(t, plan.Subscriptions, "report-subscription",
		map[string]attr.Value{
			"attachments":  types.ListUnknown(types.StringType),
			"pdf_settings": resource_customer_subscription.NewPdfSettingsValueUnknown(),
			"permissions":  permissionsUnknown,
		},
		map[string]attr.Value{
			"next_at": types.StringUnknown(),
			"time":    types.StringValue("2026-11-03T10:00:00+02:00"),
		},
		map[string]attr.Value{"name": types.StringUnknown()},
	)
	plan.Subscriptions = withCustomerSubscriptionEntry(t, plan.Subscriptions, "dashboard-subscription",
		map[string]attr.Value{
			"attachments":  types.ListUnknown(types.StringType),
			"pdf_settings": resource_customer_subscription.NewPdfSettingsValueUnknown(),
			"permissions":  permissionsUnknown,
		},
		map[string]attr.Value{"next_at": types.StringUnknown()},
		map[string]attr.Value{"name": types.StringUnknown()},
	)

	// The API returns the entries in its own order.
	resp := testCustomerSubscriptionResponse()
	resp.Subscriptions[0], resp.Subscriptions[1] = resp.Subscriptions[1], resp.Subscriptions[0]
	if diags := overlayCustomerSubscriptionComputedFields(t.Context(), resp, &plan); diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if plan.Id.ValueString() != "sub-1" || plan.Etag.ValueString() != "etag-1" || plan.CreatedBy.ValueString() != "owner@example.com" ||
		plan.Name.ValueString() != "Executive costs (Dashboard Subscription)" {
		t.Errorf("computed fields not resolved: %+v", plan.CustomerSubscriptionModel)
	}
	entries := customerSubscriptionEntries(t, plan.Subscriptions)
	report := entries["report-subscription"]
	if report.Source.Name.ValueString() != "Monthly costs" || report.Schedule.NextAt.ValueString() != "2026-11-10T08:00:00Z" ||
		!report.Permissions.CanDelete.ValueBool() {
		t.Errorf("report computed fields not resolved: %+v", report)
	}
	if got := stringListValues(t, report.Attachments); len(got) != 1 || got[0] != "pdf" {
		t.Errorf("report attachments = %v", got)
	}
	if report.PdfSettings.IsUnknown() || report.PdfSettings.IncludeTable.ValueBool() {
		t.Errorf("report pdf settings = %+v", report.PdfSettings)
	}
	if got := report.Schedule.Time.ValueString(); got != "2026-11-03T10:00:00+02:00" {
		t.Errorf("report time = %q, want the planned spelling", got)
	}
	dashboard := entries["dashboard-subscription"]
	if dashboard.Source.Name.ValueString() != "Executive costs" || dashboard.Schedule.NextAt.IsUnknown() {
		t.Errorf("dashboard computed fields not resolved: %+v", dashboard)
	}
	if got := stringListValues(t, dashboard.Attachments); dashboard.Attachments.IsNull() || len(got) != 0 || !dashboard.PdfSettings.IsNull() {
		t.Errorf("dashboard report options = %v, %v; want [] and null", dashboard.Attachments, dashboard.PdfSettings)
	}

	// On Update, known name and etag are kept (the next Read refreshes them).
	plan.Name = types.StringValue("Old name")
	plan.Etag = types.StringValue("etag-old")
	if diags := overlayCustomerSubscriptionComputedFields(t.Context(), resp, &plan); diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if plan.Name.ValueString() != "Old name" || plan.Etag.ValueString() != "etag-old" {
		t.Errorf("known name/etag overwritten: %q, %q", plan.Name.ValueString(), plan.Etag.ValueString())
	}
}

func TestCustomerSubscriptionToCreateRequest(t *testing.T) {
	t.Run("full", func(t *testing.T) {
		plan := customerSubscriptionPlanFromResponse(t, testCustomerSubscriptionResponse())
		plan.Subscriptions = withCustomerSubscriptionEntry(t, plan.Subscriptions, "report-subscription", nil, map[string]attr.Value{
			"start_date": types.StringValue("2026-11-03T00:00:00-08:00"),
		}, nil)
		req, diags := plan.toCreateRequest(t.Context())
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		assertJSONEqual(t, req, `{
			"recipients": {"emails": ["finops@example.com"], "slackChannels": ["C0123ABCD"]},
			"subscriptions": [
				{"type": "dashboard-subscription", "source": {"id": "dash-1"},
				 "schedule": {"frequency": "Weekly", "startDate": "2026-11-03T08:00:00Z", "time": "2026-11-03T08:00:00Z", "timeZone": "America/Los_Angeles"}},
				{"type": "report-subscription", "source": {"id": "report-1"},
				 "schedule": {"frequency": "Custom", "customPeriodAmount": 2, "customPeriodUnit": "Week", "startDate": "2026-11-03T00:00:00-08:00", "time": "2026-11-03T08:00:00Z", "timeZone": "America/Los_Angeles"},
				 "attachments": ["pdf"], "pdfSettings": {"showFilters": true, "includeTable": false, "includeAnnotations": true}}
			]
		}`)
	})

	// Omitted Optional+Computed fields are unknown at create and left to the API.
	t.Run("unknown optional computed", func(t *testing.T) {
		plan := customerSubscriptionPlanFromResponse(t, testCustomerSubscriptionResponse())
		plan.Subscriptions = withCustomerSubscriptionEntry(t, plan.Subscriptions, "report-subscription",
			map[string]attr.Value{
				"attachments":  types.ListUnknown(types.StringType),
				"pdf_settings": resource_customer_subscription.NewPdfSettingsValueUnknown(),
			},
			map[string]attr.Value{
				"frequency":            types.StringValue("Daily"),
				"custom_period_amount": types.Int64Unknown(),
				"custom_period_unit":   types.StringUnknown(),
			}, nil)
		plan.Subscriptions = withCustomerSubscriptionEntry(t, plan.Subscriptions, "dashboard-subscription",
			map[string]attr.Value{
				"attachments":  types.ListUnknown(types.StringType),
				"pdf_settings": resource_customer_subscription.NewPdfSettingsValueUnknown(),
			}, nil, nil)
		req, diags := plan.toCreateRequest(t.Context())
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		assertJSONEqual(t, req, `{
			"recipients": {"emails": ["finops@example.com"], "slackChannels": ["C0123ABCD"]},
			"subscriptions": [
				{"type": "dashboard-subscription", "source": {"id": "dash-1"},
				 "schedule": {"frequency": "Weekly", "startDate": "2026-11-03T08:00:00Z", "time": "2026-11-03T08:00:00Z", "timeZone": "America/Los_Angeles"}},
				{"type": "report-subscription", "source": {"id": "report-1"},
				 "schedule": {"frequency": "Daily", "startDate": "2026-11-03T08:00:00Z", "time": "2026-11-03T08:00:00Z", "timeZone": "America/Los_Angeles"}}
			]
		}`)
	})

	// Report options are never sent for other types, which the API rejects.
	t.Run("report options only on reports", func(t *testing.T) {
		attachments, diags := types.ListValueFrom(t.Context(), types.StringType, []string{"csv"})
		if diags.HasError() {
			t.Fatal(diags)
		}
		plan := customerSubscriptionPlanFromResponse(t, testCustomerSubscriptionResponse())
		plan.Subscriptions = withCustomerSubscriptionEntry(t, plan.Subscriptions, "dashboard-subscription",
			map[string]attr.Value{"attachments": attachments}, nil, nil)
		req, diags := plan.toCreateRequest(t.Context())
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		for _, entry := range req.Subscriptions {
			if entry.Type == models.CustomerSubscriptionScheduledTypeDashboardSubscription && (entry.Attachments != nil || entry.PdfSettings != nil) {
				t.Errorf("dashboard entry carries report options: %+v", entry)
			}
		}
	})
}

func TestCustomerSubscriptionToUpdateRequest(t *testing.T) {
	// The patch carries both recipient lists and the full set: the API removes
	// entry types that are missing from it.
	resp := testCustomerSubscriptionResponse()
	resp.Recipients.SlackChannels = nil
	plan := customerSubscriptionPlanFromResponse(t, resp)
	plan.Subscriptions = withCustomerSubscriptionEntry(t, plan.Subscriptions, "report-subscription",
		map[string]attr.Value{"pdf_settings": resource_customer_subscription.NewPdfSettingsValueUnknown()}, nil, nil)

	req, diags := plan.toUpdateRequest(t.Context())
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	assertJSONEqual(t, req, `{
		"recipients": {"emails": ["finops@example.com"], "slackChannels": []},
		"subscriptions": [
			{"type": "dashboard-subscription", "source": {"id": "dash-1"},
			 "schedule": {"frequency": "Weekly", "startDate": "2026-11-03T08:00:00Z", "time": "2026-11-03T08:00:00Z", "timeZone": "America/Los_Angeles"}},
			{"type": "report-subscription", "source": {"id": "report-1"},
			 "schedule": {"frequency": "Custom", "customPeriodAmount": 2, "customPeriodUnit": "Week", "startDate": "2026-11-03T08:00:00Z", "time": "2026-11-03T08:00:00Z", "timeZone": "America/Los_Angeles"},
			 "attachments": ["pdf"]}
		]
	}`)
}

// TestCustomerSubscriptionResourceDelete runs every case through the plain test
// client and through DCIRetryClient, which production uses: a status the retry
// client turns into an error never reaches the resource's status handling.
func TestCustomerSubscriptionResourceDelete(t *testing.T) {
	const path = "/customers/v1/subscriptions/sub-1"

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
		// The API answers 204 for a subscription that is already gone.
		{name: "already gone", stateEtag: "e1", currentEtag: "e1", getStatus: 404, deleteFinal: 204, wantDeletes: 1, wantGets: 0},
		{name: "no etag and gone", stateEtag: "", currentEtag: "e1", getStatus: 404, deleteFinal: 204, wantDeletes: 0, wantGets: 1},
		{name: "gone before retry", stateEtag: "old", currentEtag: "e2", getStatus: 404, deleteFinal: 204, wantDeletes: 1, wantGets: 1},
		{name: "alert or budget", stateEtag: "e1", currentEtag: "e1", getStatus: 200, deleteFinal: 409, wantDeletes: 1, wantGets: 0, expectError: true},
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
						_, _ = w.Write([]byte(`{"id":"sub-1","name":"n","createdBy":"a@example.com","etag":"` + tt.currentEtag + `","recipients":{"emails":["a@example.com"],"slackChannels":[]},"subscriptions":[]}`))
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
				r := &customerSubscriptionResource{client: newClient(t, srv)}
				resp := &resource.DeleteResponse{}
				values := map[string]string{"id": "sub-1"}
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

func testCustomerSubscriptionRecipientsValue(t *testing.T, emails, slack attr.Value) resource_customer_subscription.RecipientsValue {
	t.Helper()
	return resource_customer_subscription.NewRecipientsValueMust(
		resource_customer_subscription.RecipientsValue{}.AttributeTypes(t.Context()),
		map[string]attr.Value{"emails": emails, "slack_channels": slack},
	)
}

func TestValidateCustomerSubscriptionRecipients(t *testing.T) {
	empty := stringList(t)
	a, b := types.StringValue("a@example.com"), types.StringValue("A@Example.com")
	c := types.StringValue("C0123")

	tests := []struct {
		name    string
		value   resource_customer_subscription.RecipientsValue
		summary string
		path    string
	}{
		{"null object", resource_customer_subscription.NewRecipientsValueNull(), "", ""},
		{"unknown object", resource_customer_subscription.NewRecipientsValueUnknown(), "", ""},
		{"both empty", testCustomerSubscriptionRecipientsValue(t, empty, empty), "No Destinations", "recipients"},
		{"email only", testCustomerSubscriptionRecipientsValue(t, stringList(t, a), empty), "", ""},
		{"slack only", testCustomerSubscriptionRecipientsValue(t, empty, stringList(t, c)), "", ""},
		{"unknown list may be non-empty", testCustomerSubscriptionRecipientsValue(t, empty, types.ListUnknown(types.StringType)), "", ""},
		{"duplicate email ignoring case", testCustomerSubscriptionRecipientsValue(t, stringList(t, a, b), empty), "Duplicate Destination", "recipients.emails"},
		{"duplicate slack channel", testCustomerSubscriptionRecipientsValue(t, empty, stringList(t, c, c)), "Duplicate Destination", "recipients.slack_channels"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var diags diag.Diagnostics
			validateCustomerSubscriptionRecipients(tt.value, &diags)
			assertSingleError(t, diags, tt.summary, tt.path)
		})
	}
}

// testCustomerSubscriptionEntry builds a set element. schedule and the report
// options default to a valid Weekly schedule and null options.
func testCustomerSubscriptionEntry(t *testing.T, entryType attr.Value, overrides map[string]attr.Value, scheduleOverrides map[string]attr.Value) resource_customer_subscription.SubscriptionsValue {
	t.Helper()
	ctx := t.Context()

	scheduleAttrs := map[string]attr.Value{
		"custom_period_amount": types.Int64Null(),
		"custom_period_unit":   types.StringNull(),
		"frequency":            types.StringValue("Weekly"),
		"next_at":              types.StringNull(),
		"start_date":           types.StringValue("2026-11-03T08:00:00Z"),
		"time":                 types.StringValue("2026-11-03T08:00:00Z"),
		"time_zone":            types.StringValue("UTC"),
	}
	maps.Copy(scheduleAttrs, scheduleOverrides)

	attrs := map[string]attr.Value{
		"attachments":  types.ListNull(types.StringType),
		"pdf_settings": resource_customer_subscription.NewPdfSettingsValueNull(),
		"permissions":  resource_customer_subscription.NewPermissionsValueNull(),
		"schedule":     resource_customer_subscription.NewScheduleValueMust(resource_customer_subscription.ScheduleValue{}.AttributeTypes(ctx), scheduleAttrs),
		"source": resource_customer_subscription.NewSourceValueMust(resource_customer_subscription.SourceValue{}.AttributeTypes(ctx), map[string]attr.Value{
			"id":   types.StringValue("src-1"),
			"name": types.StringNull(),
		}),
		"type": entryType,
	}
	maps.Copy(attrs, overrides)
	return resource_customer_subscription.NewSubscriptionsValueMust(resource_customer_subscription.SubscriptionsValue{}.AttributeTypes(ctx), attrs)
}

func testCustomerSubscriptionEntrySet(t *testing.T, entries ...attr.Value) types.Set {
	t.Helper()
	set, diags := types.SetValue(customerSubscriptionEntriesType(t.Context()), entries)
	if diags.HasError() {
		t.Fatalf("set: %v", diags)
	}
	return set
}

func TestValidateCustomerSubscriptionEntries(t *testing.T) {
	report, dashboard := types.StringValue("report-subscription"), types.StringValue("dashboard-subscription")
	attachments := stringList(t, types.StringValue("pdf"))
	pdfSettings := resource_customer_subscription.NewPdfSettingsValueMust(
		resource_customer_subscription.PdfSettingsValue{}.AttributeTypes(t.Context()),
		map[string]attr.Value{"include_annotations": types.BoolValue(true), "include_table": types.BoolValue(true), "show_filters": types.BoolValue(true)},
	)
	custom := map[string]attr.Value{"frequency": types.StringValue("Custom"), "custom_period_amount": types.Int64Value(2), "custom_period_unit": types.StringValue("Week")}

	tests := []struct {
		name    string
		value   types.Set
		summary string
		path    string
	}{
		{"null set", types.SetNull(customerSubscriptionEntriesType(t.Context())), "", ""},
		{"unknown set", types.SetUnknown(customerSubscriptionEntriesType(t.Context())), "", ""},
		{"one entry", testCustomerSubscriptionEntrySet(t, testCustomerSubscriptionEntry(t, dashboard, nil, nil)), "", ""},
		{"different types", testCustomerSubscriptionEntrySet(t,
			testCustomerSubscriptionEntry(t, dashboard, nil, nil),
			testCustomerSubscriptionEntry(t, report, nil, nil)), "", ""},
		{"duplicate type", testCustomerSubscriptionEntrySet(t,
			testCustomerSubscriptionEntry(t, dashboard, nil, nil),
			testCustomerSubscriptionEntry(t, dashboard, nil, map[string]attr.Value{"frequency": types.StringValue("Daily")})), "Duplicate Subscription Type", "subscriptions"},
		{"unknown type is deferred", testCustomerSubscriptionEntrySet(t,
			testCustomerSubscriptionEntry(t, dashboard, nil, nil),
			testCustomerSubscriptionEntry(t, types.StringUnknown(), map[string]attr.Value{"attachments": attachments}, nil)), "", ""},
		{"unknown entry is skipped", testCustomerSubscriptionEntrySet(t,
			testCustomerSubscriptionEntry(t, dashboard, nil, nil),
			resource_customer_subscription.NewSubscriptionsValueUnknown()), "", ""},
		{"report options on report", testCustomerSubscriptionEntrySet(t,
			testCustomerSubscriptionEntry(t, report, map[string]attr.Value{"attachments": attachments, "pdf_settings": pdfSettings}, nil)), "", ""},
		{"attachments on dashboard", testCustomerSubscriptionEntrySet(t,
			testCustomerSubscriptionEntry(t, dashboard, map[string]attr.Value{"attachments": attachments}, nil)), "Attachments Not Allowed", ".attachments"},
		{"empty attachments on dashboard", testCustomerSubscriptionEntrySet(t,
			testCustomerSubscriptionEntry(t, dashboard, map[string]attr.Value{"attachments": stringList(t)}, nil)), "Attachments Not Allowed", ".attachments"},
		{"pdf settings on dashboard", testCustomerSubscriptionEntrySet(t,
			testCustomerSubscriptionEntry(t, dashboard, map[string]attr.Value{"pdf_settings": pdfSettings}, nil)), "PDF Settings Not Allowed", ".pdf_settings"},
		{"unknown attachments on dashboard are deferred", testCustomerSubscriptionEntrySet(t,
			testCustomerSubscriptionEntry(t, dashboard, map[string]attr.Value{"attachments": types.ListUnknown(types.StringType)}, nil)), "", ""},
		{"custom period with Custom", testCustomerSubscriptionEntrySet(t,
			testCustomerSubscriptionEntry(t, dashboard, nil, custom)), "", ""},
		{"Custom without amount", testCustomerSubscriptionEntrySet(t,
			testCustomerSubscriptionEntry(t, dashboard, nil, map[string]attr.Value{"frequency": types.StringValue("Custom"), "custom_period_unit": types.StringValue("Week")})), "Missing Custom Period", ".schedule.custom_period_amount"},
		{"Custom without unit", testCustomerSubscriptionEntrySet(t,
			testCustomerSubscriptionEntry(t, dashboard, nil, map[string]attr.Value{"frequency": types.StringValue("Custom"), "custom_period_amount": types.Int64Value(2)})), "Missing Custom Period", ".schedule.custom_period_unit"},
		{"Custom with unknown period is deferred", testCustomerSubscriptionEntrySet(t,
			testCustomerSubscriptionEntry(t, dashboard, nil, map[string]attr.Value{"frequency": types.StringValue("Custom"), "custom_period_amount": types.Int64Unknown(), "custom_period_unit": types.StringUnknown()})), "", ""},
		{"custom amount without Custom", testCustomerSubscriptionEntrySet(t,
			testCustomerSubscriptionEntry(t, dashboard, nil, map[string]attr.Value{"custom_period_amount": types.Int64Value(2)})), "Custom Period Not Allowed", ".schedule.custom_period_amount"},
		{"custom unit without Custom", testCustomerSubscriptionEntrySet(t,
			testCustomerSubscriptionEntry(t, dashboard, nil, map[string]attr.Value{"custom_period_unit": types.StringValue("Week")})), "Custom Period Not Allowed", ".schedule.custom_period_unit"},
		{"unknown frequency is deferred", testCustomerSubscriptionEntrySet(t,
			testCustomerSubscriptionEntry(t, dashboard, nil, map[string]attr.Value{"frequency": types.StringUnknown(), "custom_period_amount": types.Int64Value(2)})), "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var diags diag.Diagnostics
			validateCustomerSubscriptionEntries(tt.value, &diags)
			assertSingleError(t, diags, tt.summary, tt.path)
		})
	}
}
