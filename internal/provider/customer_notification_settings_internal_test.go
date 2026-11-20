package provider

import (
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_customer_notifications"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_customer_notification"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// testCustomerNotificationSettings builds a resource settings value in which
// every type is null except the given ones.
func testCustomerNotificationSettings(t *testing.T, set map[string]attr.Value) resource_customer_notification.SettingsValue {
	t.Helper()
	values := map[string]attr.Value{
		"new_invoices":                  resource_customer_notification.NewNewInvoicesValueNull(),
		"new_proforma_invoices":         resource_customer_notification.NewNewProformaInvoicesValueNull(),
		"payment_reminders":             resource_customer_notification.NewPaymentRemindersValueNull(),
		"credit_utilization":            resource_customer_notification.NewCreditUtilizationValueNull(),
		"quota_utilization":             resource_customer_notification.NewQuotaUtilizationValueNull(),
		"cloud_incidents":               resource_customer_notification.NewCloudIncidentsValueNull(),
		"new_sku_usage":                 resource_customer_notification.NewNewSkuUsageValueNull(),
		"daily_digest":                  resource_customer_notification.NewDailyDigestValueNull(),
		"weekly_digest":                 resource_customer_notification.NewWeeklyDigestValueNull(),
		"monthly_digest":                resource_customer_notification.NewMonthlyDigestValueNull(),
		"expert_insights":               resource_customer_notification.NewExpertInsightsValueNull(),
		"insights":                      resource_customer_notification.NewInsightsValueNull(),
		"cost_anomalies":                resource_customer_notification.NewCostAnomaliesValueNull(),
		"savings_plan_approval":         resource_customer_notification.NewSavingsPlanApprovalValueNull(),
		"savings_plan_purchased":        resource_customer_notification.NewSavingsPlanPurchasedValueNull(),
		"cud_approval":                  resource_customer_notification.NewCudApprovalValueNull(),
		"cud_purchased":                 resource_customer_notification.NewCudPurchasedValueNull(),
		"data_integration_sync_failure": resource_customer_notification.NewDataIntegrationSyncFailureValueNull(),
		"aws_billing_data_import":       resource_customer_notification.NewAwsBillingDataImportValueNull(),
	}
	for name, v := range set {
		if _, ok := values[name]; !ok {
			t.Fatalf("unknown settings type %q", name)
		}
		values[name] = v
	}
	return resource_customer_notification.NewSettingsValueMust(resource_customer_notification.SettingsValue{}.AttributeTypes(t.Context()), values)
}

func testCNCreditUtilization(t *testing.T) resource_customer_notification.CreditUtilizationValue {
	t.Helper()
	return resource_customer_notification.NewCreditUtilizationValueMust(map[string]attr.Type{}, map[string]attr.Value{})
}

func testCNExpertInsights(t *testing.T) resource_customer_notification.ExpertInsightsValue {
	t.Helper()
	return resource_customer_notification.NewExpertInsightsValueMust(map[string]attr.Type{}, map[string]attr.Value{})
}

func testCNDataIntegrationSyncFailure(t *testing.T) resource_customer_notification.DataIntegrationSyncFailureValue {
	t.Helper()
	return resource_customer_notification.NewDataIntegrationSyncFailureValueMust(map[string]attr.Type{}, map[string]attr.Value{})
}

func testCNNewInvoices(t *testing.T, ids types.List) resource_customer_notification.NewInvoicesValue {
	t.Helper()
	return resource_customer_notification.NewNewInvoicesValueMust(
		resource_customer_notification.NewInvoicesValue{}.AttributeTypes(t.Context()),
		map[string]attr.Value{"billing_profile_ids": ids},
	)
}

func testCNAwsBillingDataImport(t *testing.T, ids types.List) resource_customer_notification.AwsBillingDataImportValue {
	t.Helper()
	return resource_customer_notification.NewAwsBillingDataImportValueMust(
		resource_customer_notification.AwsBillingDataImportValue{}.AttributeTypes(t.Context()),
		map[string]attr.Value{"billing_profile_ids": ids},
	)
}

func testCNDailyDigest(t *testing.T, ids types.List) resource_customer_notification.DailyDigestValue {
	t.Helper()
	return resource_customer_notification.NewDailyDigestValueMust(
		resource_customer_notification.DailyDigestValue{}.AttributeTypes(t.Context()),
		map[string]attr.Value{"allocation_ids": ids},
	)
}

func testCNInsights(t *testing.T, pillars, severities, providers types.List) resource_customer_notification.InsightsValue {
	t.Helper()
	return resource_customer_notification.NewInsightsValueMust(
		resource_customer_notification.InsightsValue{}.AttributeTypes(t.Context()),
		map[string]attr.Value{"pillars": pillars, "severities": severities, "providers": providers},
	)
}

// testCustomerNotificationMultiTypeResponse subscribes to one type of every
// shape, including the organization-only aws_billing_data_import, and not to
// cost anomalies.
func testCustomerNotificationMultiTypeResponse() *models.CustomerNotification {
	resp := testCustomerNotificationResponse()
	resp.Settings = models.CustomerNotificationSettings{
		CreditUtilization:    &models.NotificationEmptySettings{},
		ExpertInsights:       &models.NotificationEmptySettings{},
		NewInvoices:          &models.BillingProfileNotificationSettings{},
		AwsBillingDataImport: &models.BillingProfileNotificationSettings{BillingProfileIds: &[]string{"ent_2", "ent_1"}},
		DailyDigest:          &models.DigestNotificationSettings{AllocationIds: []string{"alloc-b", "alloc-a"}},
		Insights:             &models.InsightsNotificationSettings{Pillars: &[]string{"security", "cost"}},
	}
	return resp
}

func TestMapCustomerNotificationToModel_SettingsTypes(t *testing.T) {
	var state customerNotificationResourceModel
	mustMapCustomerNotification(t, testCustomerNotificationMultiTypeResponse(), &state)

	want := testCustomerNotificationSettings(t, map[string]attr.Value{
		"credit_utilization":      testCNCreditUtilization(t),
		"expert_insights":         testCNExpertInsights(t),
		"new_invoices":            testCNNewInvoices(t, testStrings(t)), // absent → []
		"aws_billing_data_import": testCNAwsBillingDataImport(t, testStrings(t, "ent_2", "ent_1")),
		"daily_digest":            testCNDailyDigest(t, testStrings(t, "alloc-b", "alloc-a")),
		"insights":                testCNInsights(t, testStrings(t, "security", "cost"), testStrings(t), testStrings(t)),
	})

	if !state.Settings.Equal(want) {
		t.Errorf("settings =\n%s\nwant\n%s", state.Settings, want)
	}
}

// assertNotificationDataSourceSettings checks every attribute of a data
// source settings value: the types in want hold exactly the given attributes,
// every other type is null.
func assertNotificationDataSourceSettings(t *testing.T, settings basetypes.ObjectValuable, want map[string]map[string]attr.Value) {
	t.Helper()
	object, diags := settings.ToObjectValue(t.Context())
	if diags.HasError() {
		t.Fatalf("settings: %v", diags)
	}
	for name, got := range object.Attributes() {
		wantAttrs, subscribed := want[name]
		if !subscribed {
			if !got.IsNull() {
				t.Errorf("settings.%s = %s, want null", name, got)
			}
			continue
		}
		gotObject, ok := got.(basetypes.ObjectValuable)
		if !ok {
			t.Fatalf("settings.%s is %T, not an object", name, got)
		}
		gotValue, diags := gotObject.ToObjectValue(t.Context())
		if diags.HasError() {
			t.Fatalf("settings.%s: %v", name, diags)
		}
		if gotValue.IsNull() || gotValue.IsUnknown() {
			t.Errorf("settings.%s = %s, want subscribed", name, gotValue)
			continue
		}
		gotAttrs := gotValue.Attributes()
		if len(gotAttrs) != len(wantAttrs) {
			t.Errorf("settings.%s has %d attributes, want %d", name, len(gotAttrs), len(wantAttrs))
		}
		for key, wantValue := range wantAttrs {
			if !gotAttrs[key].Equal(wantValue) {
				t.Errorf("settings.%s.%s = %s, want %s", name, key, gotAttrs[key], wantValue)
			}
		}
	}
	for name := range want {
		if _, ok := object.Attributes()[name]; !ok {
			t.Errorf("settings.%s is missing", name)
		}
	}
}

func TestMapCustomerNotificationToDataSources_SettingsTypes(t *testing.T) {
	want := map[string]map[string]attr.Value{
		"credit_utilization":      {},
		"expert_insights":         {},
		"new_invoices":            {"billing_profile_ids": testStrings(t)},
		"aws_billing_data_import": {"billing_profile_ids": testStrings(t, "ent_2", "ent_1")},
		"daily_digest":            {"allocation_ids": testStrings(t, "alloc-b", "alloc-a")},
		"insights": {
			"pillars":    testStrings(t, "security", "cost"),
			"severities": testStrings(t),
			"providers":  testStrings(t),
		},
	}

	t.Run("singular", func(t *testing.T) {
		var data customerNotificationDataSourceModel
		if diags := mapCustomerNotificationToDataSource(t.Context(), testCustomerNotificationMultiTypeResponse(), &data); diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		assertNotificationDataSourceSettings(t, data.Settings, want)
	})

	t.Run("plural", func(t *testing.T) {
		var data customerNotificationsDataSourceModel
		items := []models.CustomerNotification{*testCustomerNotificationMultiTypeResponse()}
		if diags := mapCustomerNotificationsToDataSource(t.Context(), items, &data); diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		elements := data.Items.Elements()
		if len(elements) != 1 {
			t.Fatalf("items has %d elements, want 1", len(elements))
		}
		item, ok := elements[0].(datasource_customer_notifications.ItemsValue)
		if !ok {
			t.Fatalf("item is %T", elements[0])
		}
		assertNotificationDataSourceSettings(t, item.Settings, want)
	})
}

func TestCustomerNotificationToCreateRequest_SettingsTypes(t *testing.T) {
	plan := customerNotificationPlanFromResponse(t, testCustomerNotificationMultiTypeResponse())
	// An omitted filter list of a new type is unknown in the plan; the
	// request leaves it out so the API applies its default (all).
	plan.Settings.NewInvoices = testCNNewInvoices(t, types.ListUnknown(types.StringType))
	plan.Settings.Insights = testCNInsights(t, testStrings(t, "cost"), types.ListUnknown(types.StringType), testStrings(t))

	req, diags := plan.toCreateRequest(t.Context())
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	assertJSONEqual(t, req, `{
		"name": "Anomaly alerts",
		"destinations": {"emails": ["finops@example.com", "ops@example.com"], "slackChannels": ["C0123ABCD"], "teamsChannels": []},
		"settings": {
			"creditUtilization": {},
			"expertInsights": {},
			"newInvoices": {},
			"awsBillingDataImport": {"billingProfileIds": ["ent_2", "ent_1"]},
			"dailyDigest": {"allocationIds": ["alloc-b", "alloc-a"]},
			"insights": {"pillars": ["cost"], "providers": []}
		}
	}`)
}

// Subscribed types are sent in full, an empty type as {} (never null), and
// every other type as an explicit null. The API merges a sent type into the
// stored one, so a cleared filter list must be sent as [] to take effect.
func TestCustomerNotificationToUpdateRequest_SettingsTypes(t *testing.T) {
	t.Run("known lists", func(t *testing.T) {
		plan := customerNotificationPlanFromResponse(t, testCustomerNotificationMultiTypeResponse())
		req, diags := plan.toUpdateRequest(t.Context())
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		assertJSONEqual(t, req.Settings, `{
			"creditUtilization": {},
			"expertInsights": {},
			"newInvoices": {"billingProfileIds": []},
			"awsBillingDataImport": {"billingProfileIds": ["ent_2", "ent_1"]},
			"dailyDigest": {"allocationIds": ["alloc-b", "alloc-a"]},
			"insights": {"pillars": ["security", "cost"], "severities": [], "providers": []},
			"costAnomalies": null,
			"newProformaInvoices": null, "paymentReminders": null, "quotaUtilization": null, "cloudIncidents": null,
			"newSkuUsage": null, "weeklyDigest": null, "monthlyDigest": null,
			"savingsPlanApproval": null, "savingsPlanPurchased": null, "cudApproval": null, "cudPurchased": null,
			"dataIntegrationSyncFailure": null
		}`)
	})

	// A filter list is unknown only for a type that is new in this update:
	// nothing is stored for it, so leaving the list out applies the default.
	t.Run("unknown lists of a new type", func(t *testing.T) {
		plan := customerNotificationPlanFromResponse(t, testCustomerNotificationMultiTypeResponse())
		plan.Settings.AwsBillingDataImport = testCNAwsBillingDataImport(t, types.ListUnknown(types.StringType))
		plan.Settings.Insights = testCNInsights(t, testStrings(t, "cost"), types.ListUnknown(types.StringType), types.ListUnknown(types.StringType))
		req, diags := plan.toUpdateRequest(t.Context())
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		aws, err := req.Settings.AwsBillingDataImport.Get()
		if err != nil {
			t.Fatalf("awsBillingDataImport: %v", err)
		}
		assertJSONEqual(t, aws, `{}`)
		insights, err := req.Settings.Insights.Get()
		if err != nil {
			t.Fatalf("insights: %v", err)
		}
		assertJSONEqual(t, insights, `{"pillars": ["cost"]}`)
	})
}

func TestOverlayCustomerNotificationSettings(t *testing.T) {
	resolved := testCustomerNotificationSettings(t, map[string]attr.Value{
		"credit_utilization":      testCNCreditUtilization(t),
		"new_invoices":            testCNNewInvoices(t, testStrings(t)),
		"aws_billing_data_import": testCNAwsBillingDataImport(t, testStrings(t)),
		"insights":                testCNInsights(t, testStrings(t, "cost"), testStrings(t), testStrings(t)),
	})

	unknown := types.ListUnknown(types.StringType)
	tests := []struct {
		name string
		plan resource_customer_notification.SettingsValue
		want resource_customer_notification.SettingsValue
	}{
		{
			name: "unknown lists of a new type are resolved",
			plan: testCustomerNotificationSettings(t, map[string]attr.Value{
				"new_invoices":            testCNNewInvoices(t, unknown),
				"aws_billing_data_import": testCNAwsBillingDataImport(t, unknown),
				"insights":                testCNInsights(t, testStrings(t, "cost"), unknown, unknown),
			}),
			want: testCustomerNotificationSettings(t, map[string]attr.Value{
				"new_invoices":            testCNNewInvoices(t, testStrings(t)),
				"aws_billing_data_import": testCNAwsBillingDataImport(t, testStrings(t)),
				"insights":                testCNInsights(t, testStrings(t, "cost"), testStrings(t), testStrings(t)),
			}),
		},
		{
			name: "known lists are kept",
			plan: testCustomerNotificationSettings(t, map[string]attr.Value{
				"new_invoices":            testCNNewInvoices(t, testStrings(t, "ent_1")),
				"aws_billing_data_import": testCNAwsBillingDataImport(t, testStrings(t, "ent_3")),
				"insights":                testCNInsights(t, testStrings(t, "security"), testStrings(t, "high"), testStrings(t, "aws")),
			}),
			want: testCustomerNotificationSettings(t, map[string]attr.Value{
				"new_invoices":            testCNNewInvoices(t, testStrings(t, "ent_1")),
				"aws_billing_data_import": testCNAwsBillingDataImport(t, testStrings(t, "ent_3")),
				"insights":                testCNInsights(t, testStrings(t, "security"), testStrings(t, "high"), testStrings(t, "aws")),
			}),
		},
		{
			name: "unknown type is resolved",
			plan: testCustomerNotificationSettings(t, map[string]attr.Value{
				"credit_utilization": resource_customer_notification.NewCreditUtilizationValueUnknown(),
			}),
			want: testCustomerNotificationSettings(t, map[string]attr.Value{"credit_utilization": testCNCreditUtilization(t)}),
		},
		{
			name: "null types stay null",
			plan: testCustomerNotificationSettings(t, map[string]attr.Value{"credit_utilization": testCNCreditUtilization(t)}),
			want: testCustomerNotificationSettings(t, map[string]attr.Value{"credit_utilization": testCNCreditUtilization(t)}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := tt.plan
			resolvedCopy := resolved
			overlayCustomerNotificationSettings(t.Context(), &resolvedCopy, &plan)
			if !plan.Equal(tt.want) {
				t.Errorf("plan =\n%s\nwant\n%s", plan, tt.want)
			}
		})
	}
}

func TestValidateCustomerNotificationSubscribesAType(t *testing.T) {
	tests := []struct {
		name     string
		settings resource_customer_notification.SettingsValue
		wantErr  bool
	}{
		{"no type", testCustomerNotificationSettings(t, nil), true},
		{"organization-only type set", testCustomerNotificationSettings(t, map[string]attr.Value{
			"aws_billing_data_import": testCNAwsBillingDataImport(t, testStrings(t)),
		}), false},
		{"empty type set", testCustomerNotificationSettings(t, map[string]attr.Value{"credit_utilization": testCNCreditUtilization(t)}), false},
		{"unknown type, others null", testCustomerNotificationSettings(t, map[string]attr.Value{
			"daily_digest": resource_customer_notification.NewDailyDigestValueUnknown(),
		}), false},
		{"settings unknown", resource_customer_notification.NewSettingsValueUnknown(), false},
		{"settings null", resource_customer_notification.NewSettingsValueNull(), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var diags diag.Diagnostics
			validateNotificationSubscribesAType(t.Context(), tt.settings, customerNotificationSettingsPath, &diags)
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

func testCNDestinations(t *testing.T, teams types.List) resource_customer_notification.DestinationsValue {
	t.Helper()
	return resource_customer_notification.NewDestinationsValueMust(
		resource_customer_notification.DestinationsValue{}.AttributeTypes(t.Context()),
		map[string]attr.Value{
			"emails":         testStrings(t, "finops@example.com"),
			"slack_channels": testStrings(t),
			"teams_channels": teams,
		},
	)
}

func TestValidateCustomerNotificationChannels(t *testing.T) {
	teams := testStrings(t, "19:abc@thread.tacv2")
	expert := testCustomerNotificationSettings(t, map[string]attr.Value{"expert_insights": testCNExpertInsights(t)})

	tests := []struct {
		name         string
		destinations resource_customer_notification.DestinationsValue
		settings     resource_customer_notification.SettingsValue
		wantPaths    []string // attribute paths of the expected errors, in order
	}{
		{"no teams", testCNDestinations(t, testStrings(t)), expert, nil},
		{"teams with expert_insights", testCNDestinations(t, teams), expert, []string{"settings.expert_insights"}},
		{"teams with both unsupported types", testCNDestinations(t, teams), testCustomerNotificationSettings(t, map[string]attr.Value{
			"expert_insights":               testCNExpertInsights(t),
			"data_integration_sync_failure": testCNDataIntegrationSyncFailure(t),
			"credit_utilization":            testCNCreditUtilization(t),
		}), []string{"settings.expert_insights", "settings.data_integration_sync_failure"}},
		{"unknown teams entry still counts", testCNDestinations(t, stringList(t, types.StringUnknown())), expert, []string{"settings.expert_insights"}},
		{"teams with supported types only", testCNDestinations(t, teams), testCustomerNotificationSettings(t, map[string]attr.Value{
			"credit_utilization": testCNCreditUtilization(t),
			"insights":           testCNInsights(t, testStrings(t), testStrings(t), testStrings(t)),
		}), nil},
		{"teams unknown", testCNDestinations(t, types.ListUnknown(types.StringType)), expert, nil},
		{"unsupported type unknown", testCNDestinations(t, teams), testCustomerNotificationSettings(t, map[string]attr.Value{
			"expert_insights": resource_customer_notification.NewExpertInsightsValueUnknown(),
		}), nil},
		{"settings unknown", testCNDestinations(t, teams), resource_customer_notification.NewSettingsValueUnknown(), nil},
		{"destinations unknown", resource_customer_notification.NewDestinationsValueUnknown(), expert, nil},
		{"destinations null", resource_customer_notification.NewDestinationsValueNull(), expert, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var diags diag.Diagnostics
			validateCustomerNotificationChannels(t.Context(), tt.destinations, tt.settings, &diags)
			if len(diags) != len(tt.wantPaths) {
				t.Fatalf("got %d diagnostics, want %d: %v", len(diags), len(tt.wantPaths), diags)
			}
			for i, want := range tt.wantPaths {
				d := diags[i]
				if d.Severity() != diag.SeverityError || d.Summary() != "Channel Not Supported" {
					t.Errorf("diagnostic %d = %s %q, want error %q", i, d.Severity(), d.Summary(), "Channel Not Supported")
				}
				withPath, ok := d.(diag.DiagnosticWithPath)
				if !ok || withPath.Path().String() != want {
					t.Errorf("diagnostic %d path = %v, want %s", i, d, want)
				}
			}
		})
	}
}

// buildCustomerNotificationValidatorConfig builds a configuration of the
// generated resource schema with the given top-level values; every other
// attribute is null.
func buildCustomerNotificationValidatorConfig(t *testing.T, values map[string]attr.Value) tfsdk.Config {
	t.Helper()
	ctx := t.Context()
	s := resource_customer_notification.CustomerNotificationResourceSchema(ctx)
	schemaType := s.Type().TerraformType(ctx)
	objectType, ok := schemaType.(tftypes.Object)
	if !ok {
		t.Fatalf("expected schema to be tftypes.Object, got %T", schemaType)
	}
	terraformValues := make(map[string]tftypes.Value, len(objectType.AttributeTypes))
	for name, attributeType := range objectType.AttributeTypes {
		terraformValues[name] = tftypes.NewValue(attributeType, nil)
	}
	for name, value := range values {
		terraformValue, err := value.ToTerraformValue(ctx)
		if err != nil {
			t.Fatalf("convert %s to Terraform value: %v", name, err)
		}
		terraformValues[name] = terraformValue
	}
	return tfsdk.Config{Schema: s, Raw: tftypes.NewValue(schemaType, terraformValues)}
}

// The settings validator reads the cost-anomaly children by path; with
// cost_anomalies omitted it reports nothing for them, and only the missing
// type when settings is empty.
func TestCustomerNotificationSettingsValidator_WithoutCostAnomalies(t *testing.T) {
	tests := []struct {
		name     string
		settings resource_customer_notification.SettingsValue
		wantErr  bool
	}{
		{"another type", testCustomerNotificationSettings(t, map[string]attr.Value{"credit_utilization": testCNCreditUtilization(t)}), false},
		{"no type", testCustomerNotificationSettings(t, nil), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := buildCustomerNotificationValidatorConfig(t, map[string]attr.Value{
				"settings":     tt.settings,
				"destinations": testCNDestinations(t, testStrings(t)),
			})
			var resp resource.ValidateConfigResponse
			customerNotificationSettingsValidator{}.ValidateResource(t.Context(), resource.ValidateConfigRequest{Config: config}, &resp)
			if tt.wantErr {
				assertSingleError(t, resp.Diagnostics, "No Notification Type", "settings")
				return
			}
			if len(resp.Diagnostics) != 0 {
				t.Errorf("unexpected diagnostics: %v", resp.Diagnostics)
			}
		})
	}
}

// customerNotificationSettingsAttribute returns the settings attribute of the
// resource schema.
func customerNotificationSettingsAttribute(t *testing.T) schema.SingleNestedAttribute {
	t.Helper()
	var resp resource.SchemaResponse
	NewCustomerNotificationResource().Schema(t.Context(), resource.SchemaRequest{}, &resp)
	settings, ok := resp.Schema.Attributes["settings"].(schema.SingleNestedAttribute)
	if !ok {
		t.Fatal("settings is not a single nested attribute")
	}
	return settings
}

// Every settings type, cost_anomalies included, keeps the generated
// Optional+Computed flags and plans null when omitted, so omitting it
// unsubscribes the type.
func TestCustomerNotificationSettingsTypesClearOnOmit(t *testing.T) {
	settings := customerNotificationSettingsAttribute(t)
	if len(settings.Attributes) != len(customerNotificationTypes()) {
		t.Errorf("settings has %d types, want %d", len(settings.Attributes), len(customerNotificationTypes()))
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

func TestCustomerNotificationSettingsListValidators(t *testing.T) {
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
		{"aws_billing_data_import", "billing_profile_ids", testStrings(t, "ent_1"), false},
		{"aws_billing_data_import", "billing_profile_ids", testStrings(t, "ent_1", "ent_1"), true},
		{"aws_billing_data_import", "billing_profile_ids", stringList(t, types.StringNull()), true},
		{"daily_digest", "allocation_ids", testStrings(t, "alloc-a"), false},
		{"weekly_digest", "allocation_ids", testStrings(t, "alloc-a", "alloc-a"), true},
		{"daily_digest", "allocation_ids", testStrings(t), true},
		{"insights", "pillars", types.ListUnknown(types.StringType), false},
		{"daily_digest", "allocation_ids", stringList(t, types.StringNull()), true},
		{"daily_digest", "allocation_ids", stringList(t, types.StringUnknown()), false},
		{"insights", "severities", stringList(t, types.StringNull()), true},
	}

	settings := customerNotificationSettingsAttribute(t)
	for _, tt := range tests {
		t.Run(tt.typeName+"."+tt.listName+" "+tt.value.String(), func(t *testing.T) {
			typeAttr, ok := settings.Attributes[tt.typeName].(schema.SingleNestedAttribute)
			if !ok {
				t.Fatalf("settings.%s is not a single nested attribute", tt.typeName)
			}
			list, ok := typeAttr.Attributes[tt.listName].(schema.ListAttribute)
			if !ok {
				t.Fatalf("settings.%s.%s is not a list attribute", tt.typeName, tt.listName)
			}
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

// Every settings type of the organization schema is in customerNotificationTypes and
// described without a user permission.
func TestCustomerNotificationTypesCoverSchema(t *testing.T) {
	attrTypes := resource_customer_notification.SettingsValue{}.AttributeTypes(t.Context())
	described := map[string]bool{}
	for _, nt := range customerNotificationTypes() {
		if _, ok := attrTypes[nt.attr]; !ok {
			t.Errorf("customerNotificationTypes lists %q, which is not in the organization schema", nt.attr)
		}
		described[nt.attr] = true
	}
	for name := range attrTypes {
		if !described[name] {
			t.Errorf("settings.%s is missing from customerNotificationTypes", name)
		}
	}

	settings := customerNotificationSettingsAttribute(t)
	for _, nt := range customerNotificationTypes() {
		if got, want := settings.Attributes[nt.attr].GetDescription(), customerNotificationTypeDescription(nt); got != want {
			t.Errorf("settings.%s description = %q, want %q", nt.attr, got, want)
		}
	}
}
