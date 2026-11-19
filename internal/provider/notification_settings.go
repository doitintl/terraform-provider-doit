package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// The settings of a user notification (doit_user_notification) and of an
// organization notification (doit_customer_notification) share their
// notification types. This file holds the code that does not depend on the
// generated types of either resource.

// notificationType is one property of settings: a notification type that can
// be subscribed to.
type notificationType struct {
	attr        string // attribute name under settings
	permission  string // permission the target user of a user notification must hold; empty for none
	description string
	noTeams     bool // the type cannot be delivered to Microsoft Teams
	orgOnly     bool // the type exists only on organization notifications
}

const (
	userNotificationInvoiceViewer      = "Invoice Viewer"
	userNotificationCloudAnalyticsUser = "Cloud Analytics User"
	userNotificationIssuesViewer       = "Issues Viewer"
	userNotificationInsightsManager    = "Insights Manager"
	userNotificationAssetsManager      = "Assets Manager"
)

const notificationEmptyTypeHint = " Set it to `{}` to subscribe."

const notificationBillingProfileHint = " Set it to `{}` to subscribe for every billing profile, or list `billing_profile_ids`."

var notificationTypes = []notificationType{
	{attr: "new_invoices", permission: userNotificationInvoiceViewer, description: "New invoices." + notificationBillingProfileHint},
	{attr: "new_proforma_invoices", permission: userNotificationInvoiceViewer, description: "New proforma invoices." + notificationBillingProfileHint},
	{attr: "payment_reminders", permission: userNotificationInvoiceViewer, description: "Due or overdue payments." + notificationBillingProfileHint},
	{attr: "credit_utilization", permission: userNotificationInvoiceViewer, description: "Credit utilization." + notificationEmptyTypeHint},
	{attr: "quota_utilization", permission: userNotificationCloudAnalyticsUser, description: "Quota utilization." + notificationEmptyTypeHint},
	{attr: "cloud_incidents", permission: userNotificationIssuesViewer, description: "Cloud incidents." + notificationEmptyTypeHint},
	{attr: "new_sku_usage", permission: userNotificationCloudAnalyticsUser, description: "Usage of new SKUs." + notificationEmptyTypeHint},
	{attr: "daily_digest", permission: userNotificationCloudAnalyticsUser, description: "Daily cost digest of the listed allocations."},
	{attr: "weekly_digest", permission: userNotificationCloudAnalyticsUser, description: "Weekly cost digest of the listed allocations."},
	{attr: "monthly_digest", permission: userNotificationCloudAnalyticsUser, description: "Monthly digest." + notificationEmptyTypeHint},
	{attr: "expert_insights", permission: userNotificationInsightsManager, description: "New custom insights from the DoiT expert team." + notificationEmptyTypeHint, noTeams: true},
	{attr: "insights", permission: userNotificationInsightsManager, description: "New DoiT insights. Set it to `{}` to subscribe to every insight, or narrow it with `pillars`, `severities` and `providers`; an insight must match all three."},
	{attr: "cost_anomalies", description: "Cloud cost anomalies."},
	{attr: "savings_plan_approval", permission: userNotificationCloudAnalyticsUser, description: "PerfectScale for Commitments: an AWS Savings Plan purchase requires approval." + notificationEmptyTypeHint},
	{attr: "savings_plan_purchased", permission: userNotificationCloudAnalyticsUser, description: "PerfectScale for Commitments: an AWS Savings Plan was purchased." + notificationEmptyTypeHint},
	{attr: "cud_approval", permission: userNotificationCloudAnalyticsUser, description: "PerfectScale for Commitments: a Google Cloud committed use discount purchase requires approval." + notificationEmptyTypeHint},
	{attr: "cud_purchased", permission: userNotificationCloudAnalyticsUser, description: "PerfectScale for Commitments: a Google Cloud committed use discount was purchased." + notificationEmptyTypeHint},
	{attr: "data_integration_sync_failure", permission: userNotificationAssetsManager, description: "Data integration sync failures." + notificationEmptyTypeHint, noTeams: true},
	{attr: "aws_billing_data_import", description: "Problems importing AWS billing data." + notificationBillingProfileHint, orgOnly: true},
}

// userNotificationTypes returns the types of a user notification.
func userNotificationTypes() []notificationType {
	var out []notificationType
	for _, t := range notificationTypes {
		if !t.orgOnly {
			out = append(out, t)
		}
	}
	return out
}

// customerNotificationNoTeamsTypes returns the attribute names of the types
// that cannot be delivered to Microsoft Teams.
func customerNotificationNoTeamsTypes() []string {
	var out []string
	for _, t := range notificationTypes {
		if t.noTeams {
			out = append(out, t.attr)
		}
	}
	return out
}

const userNotificationDataSourceSettingsDescription = "The notification types the user is subscribed to, one attribute per type. " +
	"A type the user is not subscribed to is null; an empty object (`{}`) means subscribed without options. " +
	"An empty filter list means every value of that filter."

const customerNotificationDataSourceSettingsDescription = "The notification types the notification subscribes to, one attribute per type. " +
	"A type that is not subscribed is null; an empty object (`{}`) means subscribed without options. " +
	"An empty filter list means every value of that filter."

// userNotificationTypeDescription is the description of a settings property
// of a user notification, including the permission the target user needs.
func userNotificationTypeDescription(t notificationType) string {
	if t.permission == "" {
		return t.description + " The user needs no extra permission for this type."
	}
	return fmt.Sprintf("%s The user must hold the %s permission.", t.description, t.permission)
}

// customerNotificationTypeDescription is the description of a settings
// property of an organization notification. The API checks no permission per
// type there, but some types cannot be delivered to Microsoft Teams.
func customerNotificationTypeDescription(t notificationType) string {
	if t.noTeams {
		return t.description + " Delivered by email and Slack only: a notification that subscribes to it must have empty `destinations.teams_channels`."
	}
	return t.description
}

// Values of the insight filters. The API spec lists them as enums of an
// array schema, which the generated models do not turn into constants.
var (
	notificationInsightPillars    = []string{"cost", "operations", "performance", "security", "compliance"}
	notificationInsightSeverities = []string{"critical", "high", "medium", "low"}
)

const (
	notificationBillingProfileIDsDescription   = "IDs of the billing profiles to notify about, such as `ent_01HEXAMPLEBILLING0001`. Omit it or set `[]` for every billing profile."
	notificationDigestAllocationIDsDescription = "IDs of custom single allocations to include in the digest, kept in the configured order. At least one; preset and group allocations are rejected."
	notificationInsightPillarsDescription      = "Insight pillars to notify about: `cost`, `operations`, `performance`, `security` or `compliance`. Omit it or set `[]` for every pillar."
	notificationInsightSeveritiesDescription   = "Insight severities to notify about: `critical`, `high`, `medium` or `low`. Omit it or set `[]` for every severity."
	notificationInsightProvidersDescription    = "Insights provider IDs to notify about, such as `aws`, `gcp`, `azure`, `k8s` or `google-workspace` (not invoice platform IDs such as `amazon-web-services`). Omit it or set `[]` for every provider."
)

// withNotificationListRules requires the elements of a settings list to be
// non-null, unique and valid, and sets its description.
func withNotificationListRules(list schema.ListAttribute, description string, elementValidator validator.String) schema.ListAttribute {
	list.Validators = append(list.Validators,
		listvalidator.UniqueValues(),
		listvalidator.ValueStringsAre(elementValidator),
	)
	list.Description = description
	list.MarkdownDescription = description
	return list
}

// notificationSettingsSource holds the API settings of every type except
// cost anomalies, keyed by attribute name. The settings models of user and
// organization notifications differ in type but share these fields.
type notificationSettingsSource struct {
	empty           map[string]*models.NotificationEmptySettings
	billingProfiles map[string]*models.BillingProfileNotificationSettings
	digests         map[string]*models.DigestNotificationSettings
	insights        *models.InsightsNotificationSettings
}

func userNotificationSettingsSource(s *models.UserNotificationSettings) notificationSettingsSource {
	return notificationSettingsSource{
		empty: map[string]*models.NotificationEmptySettings{
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
		},
		billingProfiles: map[string]*models.BillingProfileNotificationSettings{
			"new_invoices":          s.NewInvoices,
			"new_proforma_invoices": s.NewProformaInvoices,
			"payment_reminders":     s.PaymentReminders,
		},
		digests: map[string]*models.DigestNotificationSettings{
			"daily_digest":  s.DailyDigest,
			"weekly_digest": s.WeeklyDigest,
		},
		insights: s.Insights,
	}
}

func customerNotificationSettingsSource(s *models.CustomerNotificationSettings) notificationSettingsSource {
	return notificationSettingsSource{
		empty: map[string]*models.NotificationEmptySettings{
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
		},
		billingProfiles: map[string]*models.BillingProfileNotificationSettings{
			"new_invoices":            s.NewInvoices,
			"new_proforma_invoices":   s.NewProformaInvoices,
			"payment_reminders":       s.PaymentReminders,
			"aws_billing_data_import": s.AwsBillingDataImport,
		},
		digests: map[string]*models.DigestNotificationSettings{
			"daily_digest":  s.DailyDigest,
			"weekly_digest": s.WeeklyDigest,
		},
		insights: s.Insights,
	}
}

// notificationSettingsObjects returns the attributes of every subscribed
// type except cost_anomalies, keyed by attribute name. A type that is not
// subscribed is absent. The API omits an empty filter list (empty means all),
// so it is mapped to [].
func notificationSettingsObjects(ctx context.Context, s notificationSettingsSource) (map[string]map[string]attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics
	objects := map[string]map[string]attr.Value{}

	for name, v := range s.empty {
		if v != nil {
			objects[name] = map[string]attr.Value{}
		}
	}

	for name, v := range s.billingProfiles {
		if v == nil {
			continue
		}
		ids, d := notificationListValue(ctx, v.BillingProfileIds)
		diags.Append(d...)
		objects[name] = map[string]attr.Value{"billing_profile_ids": ids}
	}

	for name, v := range s.digests {
		if v == nil {
			continue
		}
		ids, d := notificationListValue(ctx, &v.AllocationIds)
		diags.Append(d...)
		objects[name] = map[string]attr.Value{"allocation_ids": ids}
	}

	if v := s.insights; v != nil {
		pillars, d := notificationListValue(ctx, v.Pillars)
		diags.Append(d...)
		severities, d := notificationListValue(ctx, v.Severities)
		diags.Append(d...)
		providers, d := notificationListValue(ctx, v.Providers)
		diags.Append(d...)
		objects["insights"] = map[string]attr.Value{
			"pillars":    pillars,
			"severities": severities,
			"providers":  providers,
		}
	}

	return objects, diags
}

// notificationSettingsValues converts objects into values of the settings
// attribute types of a resource or data source schema; a type missing from
// objects is null. cost_anomalies is left to the caller, because its nested
// types differ per schema.
func notificationSettingsValues(ctx context.Context, attrTypes map[string]attr.Type, objects map[string]map[string]attr.Value) (map[string]attr.Value, diag.Diagnostics) {
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

// notificationListValue maps an API list to a string list; nil maps to [].
func notificationListValue(ctx context.Context, values *[]string) (types.List, diag.Diagnostics) {
	elements := []string{}
	if values != nil {
		elements = append(elements, *values...)
	}
	return types.ListValueFrom(ctx, types.StringType, elements)
}

// notificationListPointer returns the planned list, or nil while it is
// unknown so the request leaves it out and the API applies its default (all).
func notificationListPointer(ctx context.Context, list types.List) (*[]string, diag.Diagnostics) {
	if list.IsNull() || list.IsUnknown() {
		return nil, nil
	}
	values := []string{}
	diags := list.ElementsAs(ctx, &values, false)
	return &values, diags
}

func notificationBillingProfiles(ctx context.Context, ids types.List) (models.BillingProfileNotificationSettings, diag.Diagnostics) {
	list, diags := notificationListPointer(ctx, ids)
	return models.BillingProfileNotificationSettings{BillingProfileIds: list}, diags
}

func notificationDigest(ctx context.Context, ids types.List) (models.DigestNotificationSettings, diag.Diagnostics) {
	allocationIDs := []string{}
	diags := ids.ElementsAs(ctx, &allocationIDs, false)
	return models.DigestNotificationSettings{AllocationIds: allocationIDs}, diags
}

func notificationInsights(ctx context.Context, pillars, severities, providers types.List) (models.InsightsNotificationSettings, diag.Diagnostics) {
	var diags diag.Diagnostics
	pillarList, d := notificationListPointer(ctx, pillars)
	diags.Append(d...)
	severityList, d := notificationListPointer(ctx, severities)
	diags.Append(d...)
	providerList, d := notificationListPointer(ctx, providers)
	diags.Append(d...)
	return models.InsightsNotificationSettings{Pillars: pillarList, Severities: severityList, Providers: providerList}, diags
}

// notificationSubscribed reports whether the plan subscribes to a type.
// A type is unknown only while its configuration is, which is never the case
// at apply; it is then treated like an omitted type.
func notificationSubscribed(v attr.Value) bool {
	return !v.IsNull() && !v.IsUnknown()
}

// validateNotificationSubscribesAType requires at least one type in
// settings: the API rejects a create without one, and an update that removes
// the last one, with 422.
func validateNotificationSubscribesAType(ctx context.Context, settings basetypes.ObjectValuable, settingsPath path.Path, diags *diag.Diagnostics) {
	if settings.IsNull() || settings.IsUnknown() {
		return
	}

	object, d := settings.ToObjectValue(ctx)
	diags.Append(d...)
	if d.HasError() {
		return
	}
	if object.IsNull() || object.IsUnknown() {
		return
	}

	for _, value := range object.Attributes() {
		if value.IsUnknown() {
			return
		}
		if !value.IsNull() {
			return
		}
	}

	diags.AddAttributeError(
		settingsPath,
		"No Notification Type",
		"`settings` must subscribe to at least one notification type, for example `credit_utilization = {}`. "+
			"To stop every notification, remove the resource instead.",
	)
}
