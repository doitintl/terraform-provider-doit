package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_user_notification"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func userNotificationTestTimeouts(t *testing.T, sch schema.Schema) timeouts.Value {
	t.Helper()
	timeoutsAttrTypes := make(map[string]attr.Type)
	if timeoutsSingle, ok := sch.Attributes["timeouts"].(schema.SingleNestedAttribute); ok {
		for k, v := range timeoutsSingle.Attributes {
			timeoutsAttrTypes[k] = v.GetType()
		}
	}
	return timeouts.Value{
		Object: types.ObjectNull(timeoutsAttrTypes),
	}
}

func userNotificationTestSettings(t *testing.T, amount string, mode string, ids []attr.Value) resource_user_notification.SettingsValue {
	t.Helper()
	ctx := t.Context()
	return resource_user_notification.NewSettingsValueMust(
		resource_user_notification.SettingsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"cost_anomalies": resource_user_notification.NewCostAnomaliesValueMust(
				resource_user_notification.CostAnomaliesValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"allocations": resource_user_notification.NewAllocationsValueMust(
						resource_user_notification.AllocationsValue{}.AttributeTypes(ctx),
						map[string]attr.Value{
							"ids":  types.ListValueMust(types.StringType, ids),
							"mode": types.StringValue(mode),
						},
					),
					"min_excess_cost": resource_user_notification.NewMinExcessCostValueMust(
						resource_user_notification.MinExcessCostValue{}.AttributeTypes(ctx),
						map[string]attr.Value{
							"amount":   types.StringValue(amount),
							"currency": types.StringValue("USD"),
						},
					),
					"min_severity": types.StringValue("warning"),
					"repeat":       resource_user_notification.NewRepeatValueNull(),
				},
			),
		},
	)
}

func TestNormalizeUSDWholeDollarAmount(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"100.00":  "100.00",
		"100.99":  "100.00",
		"0.01":    "0.00",
		"150":     "150.00",
		" 10.10 ": "10.00",
	}
	for in, want := range cases {
		if got := normalizeUSDWholeDollarAmount(in); got != want {
			t.Errorf("normalizeUSDWholeDollarAmount(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestUserNotificationImportState_CompositeID(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	r := &userNotificationResource{}

	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", schemaResp.Diagnostics)
	}

	state := tfsdk.State{
		Schema: schemaResp.Schema,
		Raw:    tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), nil),
	}

	resp := &resource.ImportStateResponse{State: state}
	r.ImportState(ctx, resource.ImportStateRequest{ID: "user-abc/notif-xyz"}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("import diagnostics: %v", resp.Diagnostics)
	}

	var userID, id, notificationID types.String
	resp.Diagnostics.Append(resp.State.GetAttribute(ctx, path.Root("user_id"), &userID)...)
	resp.Diagnostics.Append(resp.State.GetAttribute(ctx, path.Root("id"), &id)...)
	resp.Diagnostics.Append(resp.State.GetAttribute(ctx, path.Root("notification_id"), &notificationID)...)
	if resp.Diagnostics.HasError() {
		t.Fatalf("get attribute diagnostics: %v", resp.Diagnostics)
	}

	if userID.ValueString() != "user-abc" {
		t.Errorf("user_id = %q, want user-abc", userID.ValueString())
	}
	if id.ValueString() != "notif-xyz" {
		t.Errorf("id = %q, want notif-xyz", id.ValueString())
	}
	if notificationID.ValueString() != "notif-xyz" {
		t.Errorf("notification_id = %q, want notif-xyz", notificationID.ValueString())
	}
}

func TestUserNotificationImportState_InvalidID(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	r := &userNotificationResource{}
	resp := &resource.ImportStateResponse{}
	r.ImportState(ctx, resource.ImportStateRequest{ID: "not-composite"}, resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for invalid import id")
	}
}

func TestMapUserNotificationToModel(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	createTime := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	updateTime := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	email := openapi_types.Email("jane.doe@example.com")
	everyHours := 24

	api := &models.UserNotification{
		Id:         valueToNullable("notif-1"),
		Name:       "Anomaly alerts",
		CreateTime: valueToNullable(createTime),
		UpdateTime: valueToNullable(updateTime),
		Etag:       valueToNullable(`"etag-1"`),
		Destinations: &[]models.UserNotificationEmailDestination{
			{Type: "email", Address: &email},
		},
		Settings: models.UserNotificationResourceSettings{
			CostAnomalies: models.UserNotificationResourceSettingsCostAnomalies{
				MinSeverity: models.UserNotificationResourceSettingsCostAnomaliesMinSeverityWarning,
				MinExcessCost: models.Money{
					Amount:   "100.00",
					Currency: "USD",
				},
				Allocations: models.CostAnomalyAllocations{
					Mode: models.CostAnomalyAllocationsModeInclude,
					Ids:  []string{"alloc_abc"},
				},
				Repeat: &models.CostAnomalyRepeat{EveryHours: &everyHours},
			},
		},
	}

	state := userNotificationResourceModel{
		UserNotificationModel: resource_user_notification.UserNotificationModel{
			UserId: types.StringValue("user-1"),
		},
	}
	diags := mapUserNotificationToModel(ctx, api, &state)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if state.Id.ValueString() != "notif-1" {
		t.Errorf("id = %q", state.Id.ValueString())
	}
	if state.NotificationId.ValueString() != "notif-1" {
		t.Errorf("notification_id = %q", state.NotificationId.ValueString())
	}
	if state.Etag.ValueString() != `"etag-1"` {
		t.Errorf("etag = %q", state.Etag.ValueString())
	}
	if state.Settings.CostAnomalies.MinExcessCost.Amount.ValueString() != "100.00" {
		t.Errorf("amount = %q", state.Settings.CostAnomalies.MinExcessCost.Amount.ValueString())
	}
	if state.Settings.CostAnomalies.Repeat.EveryHours.ValueInt64() != 24 {
		t.Errorf("every_hours = %d", state.Settings.CostAnomalies.Repeat.EveryHours.ValueInt64())
	}
}

func TestUserNotification_PopulateState_NotFound(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	srv := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/iam/v1/users/user-1/notifications/missing" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))

	res := &userNotificationResource{client: newTestAPIClient(t, srv)}
	state := userNotificationResourceModel{
		UserNotificationModel: resource_user_notification.UserNotificationModel{
			UserId: types.StringValue("user-1"),
			Id:     types.StringValue("missing"),
		},
	}

	diags := res.populateState(ctx, &state)
	if diags.HasError() {
		t.Fatalf("populateState should not error on 404: %v", diags)
	}
	if !state.Id.IsNull() {
		t.Errorf("expected Id null on 404, got %q", state.Id.ValueString())
	}
}

func TestUserNotification_Delete_SendsIfMatch(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	var gotIfMatch string
	srv := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete && r.URL.Path == "/iam/v1/users/user-1/notifications/notif-1" {
			gotIfMatch = r.Header.Get("If-Match")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))

	res := &userNotificationResource{client: newTestAPIClient(t, srv)}

	schemaResp := &resource.SchemaResponse{}
	res.Schema(ctx, resource.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("schema: %v", schemaResp.Diagnostics)
	}

	state := tfsdk.State{Schema: schemaResp.Schema}
	diags := state.Set(ctx, userNotificationResourceModel{
		UserNotificationModel: resource_user_notification.UserNotificationModel{
			UserId:   types.StringValue("user-1"),
			Id:       types.StringValue("notif-1"),
			Etag:     types.StringValue(`"etag-abc"`),
			Name:     types.StringValue("n"),
			Settings: userNotificationTestSettings(t, "100.00", "all", []attr.Value{}),
			Destinations: types.ListNull(resource_user_notification.DestinationsType{
				ObjectType: types.ObjectType{AttrTypes: resource_user_notification.DestinationsValue{}.AttributeTypes(ctx)},
			}),
			CreateTime:     types.StringNull(),
			UpdateTime:     types.StringNull(),
			NotificationId: types.StringValue("notif-1"),
		},
		Timeouts: userNotificationTestTimeouts(t, schemaResp.Schema),
	})
	if diags.HasError() {
		t.Fatalf("set state: %v", diags)
	}

	deleteResp := &resource.DeleteResponse{}
	res.Delete(ctx, resource.DeleteRequest{State: state}, deleteResp)
	if deleteResp.Diagnostics.HasError() {
		t.Fatalf("delete diagnostics: %v", deleteResp.Diagnostics)
	}
	if gotIfMatch != `"etag-abc"` {
		t.Fatalf("If-Match = %q, want %q", gotIfMatch, `"etag-abc"`)
	}
}

func TestUserNotification_Update_SendsIfMatch(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	var gotIfMatch string
	srv := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPatch && r.URL.Path == "/iam/v1/users/user-1/notifications/notif-1":
			gotIfMatch = r.Header.Get("If-Match")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"id":"notif-1",
				"name":"alerts",
				"etag":"\"etag-new\"",
				"createTime":"2026-10-01T08:00:00Z",
				"updateTime":"2026-10-01T09:00:00Z",
				"destinations":[{"type":"email","address":"jane.doe@example.com"}],
				"settings":{
					"costAnomalies":{
						"minSeverity":"warning",
						"minExcessCost":{"amount":"100.00","currency":"USD"},
						"allocations":{"mode":"include","ids":["alloc_a"]}
					}
				}
			}`))
			return
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))

	res := &userNotificationResource{client: newTestAPIClient(t, srv)}

	schemaResp := &resource.SchemaResponse{}
	res.Schema(ctx, resource.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("schema: %v", schemaResp.Diagnostics)
	}

	model := userNotificationResourceModel{
		UserNotificationModel: resource_user_notification.UserNotificationModel{
			UserId: types.StringValue("user-1"),
			Id:     types.StringValue("notif-1"),
			Etag:   types.StringValue(`"etag-abc"`),
			Name:   types.StringValue("alerts"),
			Settings: userNotificationTestSettings(t, "100.00", "include", []attr.Value{
				types.StringValue("alloc_a"),
			}),
			Destinations: types.ListNull(resource_user_notification.DestinationsType{
				ObjectType: types.ObjectType{AttrTypes: resource_user_notification.DestinationsValue{}.AttributeTypes(ctx)},
			}),
			CreateTime:     types.StringNull(),
			UpdateTime:     types.StringNull(),
			NotificationId: types.StringValue("notif-1"),
		},
		Timeouts: userNotificationTestTimeouts(t, schemaResp.Schema),
	}

	state := tfsdk.State{Schema: schemaResp.Schema}
	diags := state.Set(ctx, model)
	if diags.HasError() {
		t.Fatalf("set state: %v", diags)
	}
	plan := tfsdk.Plan{Schema: schemaResp.Schema}
	diags = plan.Set(ctx, model)
	if diags.HasError() {
		t.Fatalf("set plan: %v", diags)
	}

	updateResp := &resource.UpdateResponse{State: state}
	res.Update(ctx, resource.UpdateRequest{State: state, Plan: plan}, updateResp)
	if updateResp.Diagnostics.HasError() {
		t.Fatalf("update diagnostics: %v", updateResp.Diagnostics)
	}
	if gotIfMatch != `"etag-abc"` {
		t.Fatalf("If-Match = %q, want %q", gotIfMatch, `"etag-abc"`)
	}
}

func TestUserNotification_UpdateBodyJSON_ClearsRepeat(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	ids, diags := types.ListValueFrom(ctx, types.StringType, []string{"alloc_a"})
	if diags.HasError() {
		t.Fatalf("ids: %v", diags)
	}

	plan := userNotificationResourceModel{
		UserNotificationModel: resource_user_notification.UserNotificationModel{
			Name:     types.StringValue("alerts"),
			Settings: userNotificationTestSettings(t, "100.99", "include", ids.Elements()),
		},
	}

	body, diags := plan.updateBodyJSON(ctx)
	if diags.HasError() {
		t.Fatalf("updateBodyJSON: %v", diags)
	}

	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	settings := parsed["settings"].(map[string]any)
	ca := settings["costAnomalies"].(map[string]any)
	if ca["repeat"] != nil {
		t.Fatalf("expected repeat null, got %#v", ca["repeat"])
	}
	mec := ca["minExcessCost"].(map[string]any)
	if mec["amount"] != "100.00" {
		t.Fatalf("expected rounded amount 100.00, got %#v", mec["amount"])
	}
}
