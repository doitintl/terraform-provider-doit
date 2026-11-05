package provider

import (
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_user_notification"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_user_notification"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// testUserNotificationSettings builds a resource settings value in which every
// type is null except the given ones.
func testUserNotificationSettings(t *testing.T, set map[string]attr.Value) resource_user_notification.SettingsValue {
	t.Helper()
	values := map[string]attr.Value{
		"new_invoices":                  resource_user_notification.NewNewInvoicesValueNull(),
		"new_proforma_invoices":         resource_user_notification.NewNewProformaInvoicesValueNull(),
		"payment_reminders":             resource_user_notification.NewPaymentRemindersValueNull(),
		"credit_utilization":            resource_user_notification.NewCreditUtilizationValueNull(),
		"quota_utilization":             resource_user_notification.NewQuotaUtilizationValueNull(),
		"cloud_incidents":               resource_user_notification.NewCloudIncidentsValueNull(),
		"new_sku_usage":                 resource_user_notification.NewNewSkuUsageValueNull(),
		"daily_digest":                  resource_user_notification.NewDailyDigestValueNull(),
		"weekly_digest":                 resource_user_notification.NewWeeklyDigestValueNull(),
		"monthly_digest":                resource_user_notification.NewMonthlyDigestValueNull(),
		"expert_insights":               resource_user_notification.NewExpertInsightsValueNull(),
		"insights":                      resource_user_notification.NewInsightsValueNull(),
		"cost_anomalies":                resource_user_notification.NewCostAnomaliesValueNull(),
		"savings_plan_approval":         resource_user_notification.NewSavingsPlanApprovalValueNull(),
		"savings_plan_purchased":        resource_user_notification.NewSavingsPlanPurchasedValueNull(),
		"cud_approval":                  resource_user_notification.NewCudApprovalValueNull(),
		"cud_purchased":                 resource_user_notification.NewCudPurchasedValueNull(),
		"data_integration_sync_failure": resource_user_notification.NewDataIntegrationSyncFailureValueNull(),
	}
	for name, v := range set {
		if _, ok := values[name]; !ok {
			t.Fatalf("unknown settings type %q", name)
		}
		values[name] = v
	}
	return resource_user_notification.NewSettingsValueMust(resource_user_notification.SettingsValue{}.AttributeTypes(t.Context()), values)
}

func testCreditUtilization(t *testing.T) resource_user_notification.CreditUtilizationValue {
	t.Helper()
	return resource_user_notification.NewCreditUtilizationValueMust(map[string]attr.Type{}, map[string]attr.Value{})
}

func testNewInvoices(t *testing.T, ids types.List) resource_user_notification.NewInvoicesValue {
	t.Helper()
	return resource_user_notification.NewNewInvoicesValueMust(
		resource_user_notification.NewInvoicesValue{}.AttributeTypes(t.Context()),
		map[string]attr.Value{"billing_profile_ids": ids},
	)
}

func testDailyDigest(t *testing.T, ids types.List) resource_user_notification.DailyDigestValue {
	t.Helper()
	return resource_user_notification.NewDailyDigestValueMust(
		resource_user_notification.DailyDigestValue{}.AttributeTypes(t.Context()),
		map[string]attr.Value{"allocation_ids": ids},
	)
}

func testInsights(t *testing.T, pillars, severities, providers types.List) resource_user_notification.InsightsValue {
	t.Helper()
	return resource_user_notification.NewInsightsValueMust(
		resource_user_notification.InsightsValue{}.AttributeTypes(t.Context()),
		map[string]attr.Value{"pillars": pillars, "severities": severities, "providers": providers},
	)
}

func testStrings(t *testing.T, values ...string) types.List {
	t.Helper()
	elements := make([]attr.Value, 0, len(values))
	for _, v := range values {
		elements = append(elements, types.StringValue(v))
	}
	return stringList(t, elements...)
}

// testMultiTypeResponse subscribes to one type of every shape and not to cost
// anomalies.
func testMultiTypeResponse() *models.UserNotification {
	resp := testUserNotificationResponse()
	resp.Settings = models.UserNotificationSettings{
		CreditUtilization:   &models.NotificationEmptySettings{},
		NewInvoices:         &models.BillingProfileNotificationSettings{},
		NewProformaInvoices: &models.BillingProfileNotificationSettings{BillingProfileIds: &[]string{"ent_2", "ent_1"}},
		DailyDigest:         &models.DigestNotificationSettings{AllocationIds: []string{"alloc-b", "alloc-a"}},
		Insights: &models.InsightsNotificationSettings{
			Pillars: &[]models.InsightsNotificationSettingsPillars{"security", "cost"},
		},
	}
	return resp
}

func TestMapUserNotificationToModel_SettingsTypes(t *testing.T) {
	var state userNotificationResourceModel
	mustMapUserNotification(t, testMultiTypeResponse(), &state)

	proforma := resource_user_notification.NewNewProformaInvoicesValueMust(
		resource_user_notification.NewProformaInvoicesValue{}.AttributeTypes(t.Context()),
		map[string]attr.Value{"billing_profile_ids": testStrings(t, "ent_2", "ent_1")},
	)
	want := testUserNotificationSettings(t, map[string]attr.Value{
		"credit_utilization":    testCreditUtilization(t),
		"new_invoices":          testNewInvoices(t, testStrings(t)), // absent → []
		"new_proforma_invoices": proforma,
		"daily_digest":          testDailyDigest(t, testStrings(t, "alloc-b", "alloc-a")),
		"insights":              testInsights(t, testStrings(t, "security", "cost"), testStrings(t), testStrings(t)),
	})

	if !state.Settings.Equal(want) {
		t.Errorf("settings =\n%s\nwant\n%s", state.Settings, want)
	}
}

func TestMapUserNotificationToDataSource_SettingsTypes(t *testing.T) {
	var data userNotificationDataSourceModel
	if diags := mapUserNotificationToDataSource(t.Context(), testMultiTypeResponse(), &data); diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if data.Settings.CostAnomalies.IsNull() != true {
		t.Errorf("cost_anomalies = %s, want null", data.Settings.CostAnomalies)
	}
	if data.Settings.CreditUtilization.IsNull() || data.Settings.CreditUtilization.IsUnknown() {
		t.Errorf("credit_utilization = %s, want {}", data.Settings.CreditUtilization)
	}
	if !data.Settings.CloudIncidents.IsNull() {
		t.Errorf("cloud_incidents = %s, want null", data.Settings.CloudIncidents)
	}
	wantDigest := datasource_user_notification.NewDailyDigestValueMust(
		datasource_user_notification.DailyDigestValue{}.AttributeTypes(t.Context()),
		map[string]attr.Value{"allocation_ids": testStrings(t, "alloc-b", "alloc-a")},
	)
	if !data.Settings.DailyDigest.Equal(wantDigest) {
		t.Errorf("daily_digest = %s, want %s", data.Settings.DailyDigest, wantDigest)
	}
	if !data.Settings.NewInvoices.BillingProfileIds.Equal(testStrings(t)) {
		t.Errorf("new_invoices.billing_profile_ids = %s, want []", data.Settings.NewInvoices.BillingProfileIds)
	}
}

func TestUserNotificationToCreateRequest_SettingsTypes(t *testing.T) {
	plan := planFromResponse(t, testMultiTypeResponse())
	// An omitted filter list of a new type is unknown in the plan; the
	// request leaves it out so the API applies its default (all).
	plan.Settings.NewInvoices = testNewInvoices(t, types.ListUnknown(types.StringType))
	plan.Settings.Insights = testInsights(t, testStrings(t, "cost"), types.ListUnknown(types.StringType), testStrings(t))

	req, diags := plan.toCreateRequest(t.Context())
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	assertJSONEqual(t, req, `{
		"name": "Anomaly alerts",
		"settings": {
			"creditUtilization": {},
			"newInvoices": {},
			"newProformaInvoices": {"billingProfileIds": ["ent_2", "ent_1"]},
			"dailyDigest": {"allocationIds": ["alloc-b", "alloc-a"]},
			"insights": {"pillars": ["cost"], "providers": []}
		}
	}`)
}

// Subscribed types are sent in full, an empty type as {} (never null), a
// cleared filter list as [], and every other type as an explicit null.
func TestUserNotificationToUpdateRequest_SettingsTypes(t *testing.T) {
	plan := planFromResponse(t, testMultiTypeResponse())

	req, diags := plan.toUpdateRequest(t.Context())
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	assertJSONEqual(t, req, `{
		"name": "Anomaly alerts",
		"settings": {
			"creditUtilization": {},
			"newInvoices": {"billingProfileIds": []},
			"newProformaInvoices": {"billingProfileIds": ["ent_2", "ent_1"]},
			"dailyDigest": {"allocationIds": ["alloc-b", "alloc-a"]},
			"insights": {"pillars": ["security", "cost"], "severities": [], "providers": []},
			"costAnomalies": null,
			"paymentReminders": null, "quotaUtilization": null, "cloudIncidents": null, "newSkuUsage": null,
			"weeklyDigest": null, "monthlyDigest": null, "expertInsights": null,
			"savingsPlanApproval": null, "savingsPlanPurchased": null, "cudApproval": null, "cudPurchased": null,
			"dataIntegrationSyncFailure": null
		}
	}`)
}

func TestOverlayUserNotificationSettings(t *testing.T) {
	resolved := testUserNotificationSettings(t, map[string]attr.Value{
		"credit_utilization": testCreditUtilization(t),
		"new_invoices":       testNewInvoices(t, testStrings(t)),
		"insights":           testInsights(t, testStrings(t, "cost"), testStrings(t), testStrings(t)),
	})

	unknown := types.ListUnknown(types.StringType)
	tests := []struct {
		name string
		plan resource_user_notification.SettingsValue
		want resource_user_notification.SettingsValue
	}{
		{
			name: "unknown lists of a new type are resolved",
			plan: testUserNotificationSettings(t, map[string]attr.Value{
				"new_invoices": testNewInvoices(t, unknown),
				"insights":     testInsights(t, testStrings(t, "cost"), unknown, unknown),
			}),
			want: testUserNotificationSettings(t, map[string]attr.Value{
				"new_invoices": testNewInvoices(t, testStrings(t)),
				"insights":     testInsights(t, testStrings(t, "cost"), testStrings(t), testStrings(t)),
			}),
		},
		{
			name: "known lists are kept",
			plan: testUserNotificationSettings(t, map[string]attr.Value{
				"new_invoices": testNewInvoices(t, testStrings(t, "ent_1")),
				"insights":     testInsights(t, testStrings(t, "security"), testStrings(t, "high"), testStrings(t, "aws")),
			}),
			want: testUserNotificationSettings(t, map[string]attr.Value{
				"new_invoices": testNewInvoices(t, testStrings(t, "ent_1")),
				"insights":     testInsights(t, testStrings(t, "security"), testStrings(t, "high"), testStrings(t, "aws")),
			}),
		},
		{
			name: "unknown type is resolved",
			plan: testUserNotificationSettings(t, map[string]attr.Value{
				"credit_utilization": resource_user_notification.NewCreditUtilizationValueUnknown(),
			}),
			want: testUserNotificationSettings(t, map[string]attr.Value{"credit_utilization": testCreditUtilization(t)}),
		},
		{
			name: "null types stay null",
			plan: testUserNotificationSettings(t, map[string]attr.Value{"credit_utilization": testCreditUtilization(t)}),
			want: testUserNotificationSettings(t, map[string]attr.Value{"credit_utilization": testCreditUtilization(t)}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := tt.plan
			resolvedCopy := resolved
			overlayUserNotificationSettings(t.Context(), &resolvedCopy, &plan)
			if !plan.Equal(tt.want) {
				t.Errorf("plan =\n%s\nwant\n%s", plan, tt.want)
			}
		})
	}
}

func TestValidateUserNotificationSubscribesAType(t *testing.T) {
	tests := []struct {
		name     string
		settings resource_user_notification.SettingsValue
		wantErr  bool
	}{
		{"no type", testUserNotificationSettings(t, nil), true},
		{"empty type set", testUserNotificationSettings(t, map[string]attr.Value{"credit_utilization": testCreditUtilization(t)}), false},
		{"unknown type, others null", testUserNotificationSettings(t, map[string]attr.Value{
			"daily_digest": resource_user_notification.NewDailyDigestValueUnknown(),
		}), false},
		{"settings unknown", resource_user_notification.NewSettingsValueUnknown(), false},
		{"settings null", resource_user_notification.NewSettingsValueNull(), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var diags diag.Diagnostics
			validateUserNotificationSubscribesAType(t.Context(), tt.settings, &diags)
			if tt.wantErr {
				assertSingleError(t, diags, "No Notification Type", "settings")
				return
			}
			if len(diags) != 0 {
				t.Errorf("unexpected diagnostics: %v", diags)
			}
		})
	}
}

// userNotificationSettingsListAttribute returns a list attribute of a settings
// type from the resource schema.
func userNotificationSettingsListAttribute(t *testing.T, typeName, listName string) schema.ListAttribute {
	t.Helper()
	var resp resource.SchemaResponse
	NewUserNotificationResource().Schema(t.Context(), resource.SchemaRequest{}, &resp)
	settings, ok := resp.Schema.Attributes["settings"].(schema.SingleNestedAttribute)
	if !ok {
		t.Fatal("settings is not a single nested attribute")
	}
	typeAttr, ok := settings.Attributes[typeName].(schema.SingleNestedAttribute)
	if !ok {
		t.Fatalf("settings.%s is not a single nested attribute", typeName)
	}
	list, ok := typeAttr.Attributes[listName].(schema.ListAttribute)
	if !ok {
		t.Fatalf("settings.%s.%s is not a list attribute", typeName, listName)
	}
	return list
}

// Every settings type keeps the generated Optional+Computed flags and plans
// null when omitted, so omitting it unsubscribes the type.
func TestUserNotificationSettingsTypesClearOnOmit(t *testing.T) {
	var resp resource.SchemaResponse
	NewUserNotificationResource().Schema(t.Context(), resource.SchemaRequest{}, &resp)
	settings, ok := resp.Schema.Attributes["settings"].(schema.SingleNestedAttribute)
	if !ok {
		t.Fatal("settings is not a single nested attribute")
	}
	for name, a := range settings.Attributes {
		typeAttr, ok := a.(schema.SingleNestedAttribute)
		if !ok {
			t.Errorf("settings.%s is %T, want a single nested attribute", name, a)
			continue
		}
		if !typeAttr.Optional || !typeAttr.Computed {
			t.Errorf("settings.%s: Optional=%v Computed=%v, want the generated Optional+Computed", name, typeAttr.Optional, typeAttr.Computed)
		}
		found := false
		for _, m := range typeAttr.PlanModifiers {
			if _, ok := m.(useNullForOmittedNotificationSettingModifier); ok {
				found = true
			}
		}
		if !found {
			t.Errorf("settings.%s lacks useNullForOmittedNotificationSetting", name)
		}
	}
}

func TestUserNotificationSettingsListValidators(t *testing.T) {
	tests := []struct {
		typeName, listName string
		value              types.List
		wantErr            bool
	}{
		{"insights", "pillars", testStrings(t, "cost", "security"), false},
		{"insights", "pillars", testStrings(t, "costs"), true},
		{"insights", "pillars", testStrings(t, "cost", "cost"), true},
		{"insights", "severities", testStrings(t, "critical", "low"), false},
		{"insights", "severities", testStrings(t, "warning"), true},
		{"insights", "providers", testStrings(t, "aws", "gcp"), false},
		{"insights", "providers", testStrings(t, ""), true},
		{"new_invoices", "billing_profile_ids", testStrings(t), false},
		{"new_invoices", "billing_profile_ids", testStrings(t, "ent_1", "ent_1"), true},
		{"payment_reminders", "billing_profile_ids", testStrings(t, ""), true},
		{"daily_digest", "allocation_ids", testStrings(t, "alloc-a"), false},
		{"weekly_digest", "allocation_ids", testStrings(t, "alloc-a", "alloc-a"), true},
		{"daily_digest", "allocation_ids", testStrings(t), true},
		{"insights", "pillars", types.ListUnknown(types.StringType), false},
		{"daily_digest", "allocation_ids", stringList(t, types.StringNull()), true},
		{"weekly_digest", "allocation_ids", stringList(t, types.StringValue("alloc-a"), types.StringNull()), true},
		{"daily_digest", "allocation_ids", stringList(t, types.StringUnknown()), false},
		{"new_invoices", "billing_profile_ids", stringList(t, types.StringNull()), true},
		{"insights", "pillars", stringList(t, types.StringNull()), true},
		{"insights", "severities", stringList(t, types.StringNull()), true},
		{"insights", "providers", stringList(t, types.StringNull()), true},
	}

	for _, tt := range tests {
		t.Run(tt.typeName+"."+tt.listName+" "+tt.value.String(), func(t *testing.T) {
			list := userNotificationSettingsListAttribute(t, tt.typeName, tt.listName)
			var diags diag.Diagnostics
			for _, v := range list.Validators {
				var resp validator.ListResponse
				v.ValidateList(t.Context(), validator.ListRequest{ConfigValue: tt.value}, &resp)
				diags.Append(resp.Diagnostics...)
			}
			if got := diags.HasError(); got != tt.wantErr {
				t.Errorf("error = %v, want %v: %v", got, tt.wantErr, diags)
			}
		})
	}
}

// Every settings type of the schema is described with its permission.
func TestUserNotificationTypesCoverSchema(t *testing.T) {
	attrTypes := resource_user_notification.SettingsValue{}.AttributeTypes(t.Context())
	described := map[string]bool{}
	for _, nt := range userNotificationTypes {
		if _, ok := attrTypes[nt.attr]; !ok {
			t.Errorf("userNotificationTypes lists %q, which is not in the schema", nt.attr)
		}
		described[nt.attr] = true
	}
	for name := range attrTypes {
		if !described[name] {
			t.Errorf("settings.%s is missing from userNotificationTypes", name)
		}
	}
}

// Omitted repeat controls that are not stored plan unknown: the overlay
// resolves them from the API, create leaves them out, and the patch sends null.
func TestUserNotificationRepeatUnknownControls(t *testing.T) {
	unknownControls := func(t *testing.T) userNotificationResourceModel {
		t.Helper()
		plan := planFromResponse(t, testUserNotificationResponse())
		plan.Settings.CostAnomalies.Repeat.EveryHours = types.Int64Unknown()
		plan.Settings.CostAnomalies.Repeat.WhenCostGrows.Match = types.StringUnknown()
		plan.Settings.CostAnomalies.Repeat.WhenCostGrows.WithinHours = types.Int64Unknown()
		return plan
	}

	t.Run("overlay", func(t *testing.T) {
		plan := unknownControls(t)
		resp := testUserNotificationResponse()
		resp.Settings.CostAnomalies.Repeat.EveryHours = nil
		resp.Settings.CostAnomalies.Repeat.WhenCostGrows.Match = nil
		resp.Settings.CostAnomalies.Repeat.WhenCostGrows.WithinHours = new(3)
		if diags := overlayUserNotificationComputedFields(t.Context(), resp, &plan); diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		want := planFromResponse(t, resp).Settings.CostAnomalies.Repeat
		if got := plan.Settings.CostAnomalies.Repeat; !got.Equal(want) {
			t.Errorf("repeat =\n%s\nwant\n%s", got, want)
		}
	})

	t.Run("create", func(t *testing.T) {
		plan := unknownControls(t)
		req, diags := plan.toCreateRequest(t.Context())
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		assertJSONEqual(t, req.Settings.CostAnomalies.Repeat, `{
			"whenCostGrows": {"percentIncrease": 10, "minIncrease": {"amount": "150.00", "currency": "USD"}}
		}`)
	})

	t.Run("update", func(t *testing.T) {
		plan := unknownControls(t)
		req, diags := plan.toUpdateRequest(t.Context())
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		patch, err := req.Settings.CostAnomalies.Get()
		if err != nil {
			t.Fatalf("costAnomalies: %v", err)
		}
		assertJSONEqual(t, patch.Repeat, `{
			"everyHours": null,
			"whenCostGrows": {"match": null, "percentIncrease": 10, "minIncrease": {"amount": "150.00", "currency": "USD"}, "withinHours": null}
		}`)
	})
}
