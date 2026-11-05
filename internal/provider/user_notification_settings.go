package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_user_notification"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// userNotificationType is one property of settings: a notification type the
// user can subscribe to.
type userNotificationType struct {
	attr        string // attribute name under settings
	api         string // API property name, as a 403 problem names it
	permission  string // permission the target user must hold; empty for none
	description string
}

const (
	userNotificationInvoiceViewer      = "Invoice Viewer"
	userNotificationCloudAnalyticsUser = "Cloud Analytics User"
	userNotificationIssuesViewer       = "Issues Viewer"
	userNotificationInsightsManager    = "Insights Manager"
	userNotificationAssetsManager      = "Assets Manager"
)

const userNotificationEmptyTypeHint = " Set it to `{}` to subscribe."

const userNotificationBillingProfileHint = " Set it to `{}` to subscribe for every billing profile, or list `billing_profile_ids`."

var userNotificationTypes = []userNotificationType{
	{"new_invoices", "newInvoices", userNotificationInvoiceViewer, "New invoices." + userNotificationBillingProfileHint},
	{"new_proforma_invoices", "newProformaInvoices", userNotificationInvoiceViewer, "New proforma invoices." + userNotificationBillingProfileHint},
	{"payment_reminders", "paymentReminders", userNotificationInvoiceViewer, "Due or overdue payments." + userNotificationBillingProfileHint},
	{"credit_utilization", "creditUtilization", userNotificationInvoiceViewer, "Credit utilization." + userNotificationEmptyTypeHint},
	{"quota_utilization", "quotaUtilization", userNotificationCloudAnalyticsUser, "Quota utilization." + userNotificationEmptyTypeHint},
	{"cloud_incidents", "cloudIncidents", userNotificationIssuesViewer, "Cloud incidents." + userNotificationEmptyTypeHint},
	{"new_sku_usage", "newSkuUsage", userNotificationCloudAnalyticsUser, "Usage of new SKUs." + userNotificationEmptyTypeHint},
	{"daily_digest", "dailyDigest", userNotificationCloudAnalyticsUser, "Daily cost digest of the listed allocations."},
	{"weekly_digest", "weeklyDigest", userNotificationCloudAnalyticsUser, "Weekly cost digest of the listed allocations."},
	{"monthly_digest", "monthlyDigest", userNotificationCloudAnalyticsUser, "Monthly digest." + userNotificationEmptyTypeHint},
	{"expert_insights", "expertInsights", userNotificationInsightsManager, "New custom insights from the DoiT expert team." + userNotificationEmptyTypeHint},
	{"insights", "insights", userNotificationInsightsManager, "New DoiT insights. Set it to `{}` to subscribe to every insight, or narrow it with `pillars`, `severities` and `providers`; an insight must match all three."},
	{"cost_anomalies", "costAnomalies", "", "Cloud cost anomalies."},
	{"savings_plan_approval", "savingsPlanApproval", userNotificationCloudAnalyticsUser, "PerfectScale for Commitments: an AWS Savings Plan purchase requires approval." + userNotificationEmptyTypeHint},
	{"savings_plan_purchased", "savingsPlanPurchased", userNotificationCloudAnalyticsUser, "PerfectScale for Commitments: an AWS Savings Plan was purchased." + userNotificationEmptyTypeHint},
	{"cud_approval", "cudApproval", userNotificationCloudAnalyticsUser, "PerfectScale for Commitments: a Google Cloud committed use discount purchase requires approval." + userNotificationEmptyTypeHint},
	{"cud_purchased", "cudPurchased", userNotificationCloudAnalyticsUser, "PerfectScale for Commitments: a Google Cloud committed use discount was purchased." + userNotificationEmptyTypeHint},
	{"data_integration_sync_failure", "dataIntegrationSyncFailure", userNotificationAssetsManager, "Data integration sync failures." + userNotificationEmptyTypeHint},
}

const userNotificationDataSourceSettingsDescription = "The notification types the user is subscribed to, one attribute per type. " +
	"A type the user is not subscribed to is null; an empty object (`{}`) means subscribed without options. " +
	"An empty filter list means every value of that filter."

// userNotificationTypeDescription is the description of a settings property,
// including the permission the target user needs.
func userNotificationTypeDescription(t userNotificationType) string {
	if t.permission == "" {
		return t.description + " The user needs no extra permission for this type."
	}
	return fmt.Sprintf("%s The user must hold the %s permission.", t.description, t.permission)
}

// userNotificationSettingsObjects returns the attributes of every subscribed
// type except cost_anomalies, keyed by attribute name. A type that is not
// subscribed is absent. The API omits an empty filter list (empty means all),
// so it is mapped to [].
func userNotificationSettingsObjects(ctx context.Context, s *models.UserNotificationSettings) (map[string]map[string]attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics
	objects := map[string]map[string]attr.Value{}

	for name, v := range map[string]*models.NotificationEmptySettings{
		"credit_utilization":            s.CreditUtilization,
		"quota_utilization":             s.QuotaUtilization,
		"cloud_incidents":               s.CloudIncidents,
		"new_sku_usage":                 s.NewSkuUsage,
		"monthly_digest":                s.MonthlyDigest,
		"expert_insights":               s.ExpertInsights,
		"savings_plan_approval":         s.SavingsPlanApproval,
		"savings_plan_purchased":        s.SavingsPlanPurchased,
		"cud_approval":                  s.CudApproval,
		"cud_purchased":                 s.CudPurchased,
		"data_integration_sync_failure": s.DataIntegrationSyncFailure,
	} {
		if v != nil {
			objects[name] = map[string]attr.Value{}
		}
	}

	for name, v := range map[string]*models.BillingProfileNotificationSettings{
		"new_invoices":          s.NewInvoices,
		"new_proforma_invoices": s.NewProformaInvoices,
		"payment_reminders":     s.PaymentReminders,
	} {
		if v == nil {
			continue
		}
		ids, d := userNotificationListValue(ctx, v.BillingProfileIds)
		diags.Append(d...)
		objects[name] = map[string]attr.Value{"billing_profile_ids": ids}
	}

	for name, v := range map[string]*models.DigestNotificationSettings{
		"daily_digest":  s.DailyDigest,
		"weekly_digest": s.WeeklyDigest,
	} {
		if v == nil {
			continue
		}
		ids, d := userNotificationListValue(ctx, &v.AllocationIds)
		diags.Append(d...)
		objects[name] = map[string]attr.Value{"allocation_ids": ids}
	}

	if v := s.Insights; v != nil {
		pillars, d := userNotificationListValue(ctx, v.Pillars)
		diags.Append(d...)
		severities, d := userNotificationListValue(ctx, v.Severities)
		diags.Append(d...)
		providers, d := userNotificationListValue(ctx, v.Providers)
		diags.Append(d...)
		objects["insights"] = map[string]attr.Value{
			"pillars":    pillars,
			"severities": severities,
			"providers":  providers,
		}
	}

	return objects, diags
}

// userNotificationSettingsValues converts objects into values of the settings
// attribute types of the resource or a data source schema; a type missing from
// objects is null. cost_anomalies is left to the caller, because its nested
// types differ per schema.
func userNotificationSettingsValues(ctx context.Context, attrTypes map[string]attr.Type, objects map[string]map[string]attr.Value) (map[string]attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics
	values := make(map[string]attr.Value, len(attrTypes))

	for name, t := range attrTypes {
		if name == "cost_anomalies" {
			continue
		}
		objectType, ok := t.(basetypes.ObjectTypable)
		if !ok {
			diags.AddError("Unexpected Settings Type", fmt.Sprintf("settings.%s has type %T, expected an object type.", name, t))
			continue
		}
		withAttributeTypes, ok := t.(attr.TypeWithAttributeTypes)
		if !ok {
			diags.AddError("Unexpected Settings Type", fmt.Sprintf("settings.%s has type %T without attribute types.", name, t))
			continue
		}

		object := types.ObjectNull(withAttributeTypes.AttributeTypes())
		if attributes, ok := objects[name]; ok {
			var d diag.Diagnostics
			object, d = types.ObjectValue(withAttributeTypes.AttributeTypes(), attributes)
			diags.Append(d...)
			if d.HasError() {
				continue
			}
		}

		v, d := objectType.ValueFromObject(ctx, object)
		diags.Append(d...)
		values[name] = v
	}

	return values, diags
}

// userNotificationListValue maps an API list to a string list; nil maps to [].
func userNotificationListValue[T ~string](ctx context.Context, values *[]T) (types.List, diag.Diagnostics) {
	elements := []string{}
	if values != nil {
		for _, v := range *values {
			elements = append(elements, string(v))
		}
	}
	return types.ListValueFrom(ctx, types.StringType, elements)
}

// userNotificationListPointer returns the planned list, or nil while it is
// unknown so the request leaves it out and the API applies its default (all).
func userNotificationListPointer[T ~string](ctx context.Context, list types.List) (*[]T, diag.Diagnostics) {
	if list.IsNull() || list.IsUnknown() {
		return nil, nil
	}
	var elements []string
	diags := list.ElementsAs(ctx, &elements, false)
	values := make([]T, 0, len(elements))
	for _, e := range elements {
		values = append(values, T(e))
	}
	return &values, diags
}

func userNotificationBillingProfiles(ctx context.Context, ids types.List) (models.BillingProfileNotificationSettings, diag.Diagnostics) {
	list, diags := userNotificationListPointer[string](ctx, ids)
	return models.BillingProfileNotificationSettings{BillingProfileIds: list}, diags
}

func userNotificationDigest(ctx context.Context, ids types.List) (models.DigestNotificationSettings, diag.Diagnostics) {
	allocationIDs := []string{}
	diags := ids.ElementsAs(ctx, &allocationIDs, false)
	return models.DigestNotificationSettings{AllocationIds: allocationIDs}, diags
}

func userNotificationInsights(ctx context.Context, v resource_user_notification.InsightsValue) (models.InsightsNotificationSettings, diag.Diagnostics) {
	var diags diag.Diagnostics
	pillars, d := userNotificationListPointer[models.InsightsNotificationSettingsPillars](ctx, v.Pillars)
	diags.Append(d...)
	severities, d := userNotificationListPointer[models.InsightsNotificationSettingsSeverities](ctx, v.Severities)
	diags.Append(d...)
	providers, d := userNotificationListPointer[string](ctx, v.Providers)
	diags.Append(d...)
	return models.InsightsNotificationSettings{Pillars: pillars, Severities: severities, Providers: providers}, diags
}

// userNotificationSubscribed reports whether the plan subscribes to a type.
// A type is unknown only while its configuration is, which is never the case
// at apply; it is then treated like an omitted type.
func userNotificationSubscribed(v attr.Value) bool {
	return !v.IsNull() && !v.IsUnknown()
}

// toUserNotificationCreateSettings sets every type the plan subscribes to.
func toUserNotificationCreateSettings(ctx context.Context, s resource_user_notification.SettingsValue, out *models.UserNotificationSettings) diag.Diagnostics {
	var diags diag.Diagnostics

	if userNotificationSubscribed(s.CreditUtilization) {
		out.CreditUtilization = &models.NotificationEmptySettings{}
	}
	if userNotificationSubscribed(s.QuotaUtilization) {
		out.QuotaUtilization = &models.NotificationEmptySettings{}
	}
	if userNotificationSubscribed(s.CloudIncidents) {
		out.CloudIncidents = &models.NotificationEmptySettings{}
	}
	if userNotificationSubscribed(s.NewSkuUsage) {
		out.NewSkuUsage = &models.NotificationEmptySettings{}
	}
	if userNotificationSubscribed(s.MonthlyDigest) {
		out.MonthlyDigest = &models.NotificationEmptySettings{}
	}
	if userNotificationSubscribed(s.ExpertInsights) {
		out.ExpertInsights = &models.NotificationEmptySettings{}
	}
	if userNotificationSubscribed(s.SavingsPlanApproval) {
		out.SavingsPlanApproval = &models.NotificationEmptySettings{}
	}
	if userNotificationSubscribed(s.SavingsPlanPurchased) {
		out.SavingsPlanPurchased = &models.NotificationEmptySettings{}
	}
	if userNotificationSubscribed(s.CudApproval) {
		out.CudApproval = &models.NotificationEmptySettings{}
	}
	if userNotificationSubscribed(s.CudPurchased) {
		out.CudPurchased = &models.NotificationEmptySettings{}
	}
	if userNotificationSubscribed(s.DataIntegrationSyncFailure) {
		out.DataIntegrationSyncFailure = &models.NotificationEmptySettings{}
	}
	if userNotificationSubscribed(s.NewInvoices) {
		v, d := userNotificationBillingProfiles(ctx, s.NewInvoices.BillingProfileIds)
		diags.Append(d...)
		out.NewInvoices = &v
	}
	if userNotificationSubscribed(s.NewProformaInvoices) {
		v, d := userNotificationBillingProfiles(ctx, s.NewProformaInvoices.BillingProfileIds)
		diags.Append(d...)
		out.NewProformaInvoices = &v
	}
	if userNotificationSubscribed(s.PaymentReminders) {
		v, d := userNotificationBillingProfiles(ctx, s.PaymentReminders.BillingProfileIds)
		diags.Append(d...)
		out.PaymentReminders = &v
	}
	if userNotificationSubscribed(s.DailyDigest) {
		v, d := userNotificationDigest(ctx, s.DailyDigest.AllocationIds)
		diags.Append(d...)
		out.DailyDigest = &v
	}
	if userNotificationSubscribed(s.WeeklyDigest) {
		v, d := userNotificationDigest(ctx, s.WeeklyDigest.AllocationIds)
		diags.Append(d...)
		out.WeeklyDigest = &v
	}
	if userNotificationSubscribed(s.Insights) {
		v, d := userNotificationInsights(ctx, s.Insights)
		diags.Append(d...)
		out.Insights = &v
	}
	if userNotificationSubscribed(s.CostAnomalies) {
		v, d := toCreateCostAnomalies(ctx, s.CostAnomalies)
		diags.Append(d...)
		out.CostAnomalies = &v
	}

	return diags
}

// toUserNotificationPatchSettings sets every type in the merge patch: the planned value for a subscribed type, and an explicit null
// for every other one. A type the patch leaves out stays as stored, so null is
// what unsubscribes it. An empty type is sent as a non-nil empty map, because
// Set(nil) would marshal to null.
func toUserNotificationPatchSettings(ctx context.Context, s resource_user_notification.SettingsValue, out *models.PatchUserNotificationSettings) diag.Diagnostics {
	var diags diag.Diagnostics

	if userNotificationSubscribed(s.CreditUtilization) {
		out.CreditUtilization.Set(models.NotificationEmptySettings{})
	} else {
		out.CreditUtilization.SetNull()
	}
	if userNotificationSubscribed(s.QuotaUtilization) {
		out.QuotaUtilization.Set(models.NotificationEmptySettings{})
	} else {
		out.QuotaUtilization.SetNull()
	}
	if userNotificationSubscribed(s.CloudIncidents) {
		out.CloudIncidents.Set(models.NotificationEmptySettings{})
	} else {
		out.CloudIncidents.SetNull()
	}
	if userNotificationSubscribed(s.NewSkuUsage) {
		out.NewSkuUsage.Set(models.NotificationEmptySettings{})
	} else {
		out.NewSkuUsage.SetNull()
	}
	if userNotificationSubscribed(s.MonthlyDigest) {
		out.MonthlyDigest.Set(models.NotificationEmptySettings{})
	} else {
		out.MonthlyDigest.SetNull()
	}
	if userNotificationSubscribed(s.ExpertInsights) {
		out.ExpertInsights.Set(models.NotificationEmptySettings{})
	} else {
		out.ExpertInsights.SetNull()
	}
	if userNotificationSubscribed(s.SavingsPlanApproval) {
		out.SavingsPlanApproval.Set(models.NotificationEmptySettings{})
	} else {
		out.SavingsPlanApproval.SetNull()
	}
	if userNotificationSubscribed(s.SavingsPlanPurchased) {
		out.SavingsPlanPurchased.Set(models.NotificationEmptySettings{})
	} else {
		out.SavingsPlanPurchased.SetNull()
	}
	if userNotificationSubscribed(s.CudApproval) {
		out.CudApproval.Set(models.NotificationEmptySettings{})
	} else {
		out.CudApproval.SetNull()
	}
	if userNotificationSubscribed(s.CudPurchased) {
		out.CudPurchased.Set(models.NotificationEmptySettings{})
	} else {
		out.CudPurchased.SetNull()
	}
	if userNotificationSubscribed(s.DataIntegrationSyncFailure) {
		out.DataIntegrationSyncFailure.Set(models.NotificationEmptySettings{})
	} else {
		out.DataIntegrationSyncFailure.SetNull()
	}
	if userNotificationSubscribed(s.NewInvoices) {
		v, d := userNotificationBillingProfiles(ctx, s.NewInvoices.BillingProfileIds)
		diags.Append(d...)
		out.NewInvoices.Set(v)
	} else {
		out.NewInvoices.SetNull()
	}
	if userNotificationSubscribed(s.NewProformaInvoices) {
		v, d := userNotificationBillingProfiles(ctx, s.NewProformaInvoices.BillingProfileIds)
		diags.Append(d...)
		out.NewProformaInvoices.Set(v)
	} else {
		out.NewProformaInvoices.SetNull()
	}
	if userNotificationSubscribed(s.PaymentReminders) {
		v, d := userNotificationBillingProfiles(ctx, s.PaymentReminders.BillingProfileIds)
		diags.Append(d...)
		out.PaymentReminders.Set(v)
	} else {
		out.PaymentReminders.SetNull()
	}
	if userNotificationSubscribed(s.DailyDigest) {
		v, d := userNotificationDigest(ctx, s.DailyDigest.AllocationIds)
		diags.Append(d...)
		out.DailyDigest.Set(v)
	} else {
		out.DailyDigest.SetNull()
	}
	if userNotificationSubscribed(s.WeeklyDigest) {
		v, d := userNotificationDigest(ctx, s.WeeklyDigest.AllocationIds)
		diags.Append(d...)
		out.WeeklyDigest.Set(v)
	} else {
		out.WeeklyDigest.SetNull()
	}
	if userNotificationSubscribed(s.Insights) {
		v, d := userNotificationInsights(ctx, s.Insights)
		diags.Append(d...)
		out.Insights.Set(v)
	} else {
		out.Insights.SetNull()
	}
	if userNotificationSubscribed(s.CostAnomalies) {
		v, d := toPatchCostAnomalies(ctx, s.CostAnomalies)
		diags.Append(d...)
		out.CostAnomalies.Set(v)
	} else {
		out.CostAnomalies.SetNull()
	}

	return diags
}

// userNotificationPermissionHint explains a 403 that names the notification
// type the target user lacks the permission for. It returns "" for any other
// response.
func userNotificationPermissionHint(statusCode int, body []byte) string {
	if statusCode != 403 {
		return ""
	}
	var problem models.UserNotificationForbidden
	if err := json.Unmarshal(body, &problem); err != nil || problem.NotificationType == nil {
		return ""
	}
	for _, t := range userNotificationTypes {
		if t.api != *problem.NotificationType {
			continue
		}
		hint := fmt.Sprintf("The user does not hold the permission that settings.%s requires", t.attr)
		if t.permission != "" {
			hint += fmt.Sprintf(" (%s)", t.permission)
		}
		if problem.Permission != nil {
			hint += fmt.Sprintf(", permission ID %q", *problem.Permission)
		}
		return hint + ". Grant it to the user or remove the type from settings.\n\n"
	}
	return ""
}
