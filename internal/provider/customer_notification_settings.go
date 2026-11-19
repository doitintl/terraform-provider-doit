package provider

import (
	"context"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_customer_notification"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// toCustomerNotificationCreateSettings sets every type the plan subscribes to.
func toCustomerNotificationCreateSettings(ctx context.Context, s resource_customer_notification.SettingsValue, out *models.CustomerNotificationSettings) diag.Diagnostics {
	var diags diag.Diagnostics

	if notificationSubscribed(s.CreditUtilization) {
		out.CreditUtilization = &models.NotificationEmptySettings{}
	}
	if notificationSubscribed(s.QuotaUtilization) {
		out.QuotaUtilization = &models.NotificationEmptySettings{}
	}
	if notificationSubscribed(s.CloudIncidents) {
		out.CloudIncidents = &models.NotificationEmptySettings{}
	}
	if notificationSubscribed(s.NewSkuUsage) {
		out.NewSkuUsage = &models.NotificationEmptySettings{}
	}
	if notificationSubscribed(s.MonthlyDigest) {
		out.MonthlyDigest = &models.NotificationEmptySettings{}
	}
	if notificationSubscribed(s.ExpertInsights) {
		out.ExpertInsights = &models.NotificationEmptySettings{}
	}
	if notificationSubscribed(s.SavingsPlanApproval) {
		out.SavingsPlanApproval = &models.NotificationEmptySettings{}
	}
	if notificationSubscribed(s.SavingsPlanPurchased) {
		out.SavingsPlanPurchased = &models.NotificationEmptySettings{}
	}
	if notificationSubscribed(s.CudApproval) {
		out.CudApproval = &models.NotificationEmptySettings{}
	}
	if notificationSubscribed(s.CudPurchased) {
		out.CudPurchased = &models.NotificationEmptySettings{}
	}
	if notificationSubscribed(s.DataIntegrationSyncFailure) {
		out.DataIntegrationSyncFailure = &models.NotificationEmptySettings{}
	}
	if notificationSubscribed(s.NewInvoices) {
		v, d := notificationBillingProfiles(ctx, s.NewInvoices.BillingProfileIds)
		diags.Append(d...)
		out.NewInvoices = &v
	}
	if notificationSubscribed(s.NewProformaInvoices) {
		v, d := notificationBillingProfiles(ctx, s.NewProformaInvoices.BillingProfileIds)
		diags.Append(d...)
		out.NewProformaInvoices = &v
	}
	if notificationSubscribed(s.PaymentReminders) {
		v, d := notificationBillingProfiles(ctx, s.PaymentReminders.BillingProfileIds)
		diags.Append(d...)
		out.PaymentReminders = &v
	}
	if notificationSubscribed(s.AwsBillingDataImport) {
		v, d := notificationBillingProfiles(ctx, s.AwsBillingDataImport.BillingProfileIds)
		diags.Append(d...)
		out.AwsBillingDataImport = &v
	}
	if notificationSubscribed(s.DailyDigest) {
		v, d := notificationDigest(ctx, s.DailyDigest.AllocationIds)
		diags.Append(d...)
		out.DailyDigest = &v
	}
	if notificationSubscribed(s.WeeklyDigest) {
		v, d := notificationDigest(ctx, s.WeeklyDigest.AllocationIds)
		diags.Append(d...)
		out.WeeklyDigest = &v
	}
	if notificationSubscribed(s.Insights) {
		v, d := notificationInsights(ctx, s.Insights.Pillars, s.Insights.Severities, s.Insights.Providers)
		diags.Append(d...)
		out.Insights = &v
	}
	if notificationSubscribed(s.CostAnomalies) {
		v, d := toCreateCustomerNotificationCostAnomalies(ctx, s.CostAnomalies)
		diags.Append(d...)
		out.CostAnomalies = &v
	}

	return diags
}

// toCustomerNotificationPatchSettings sets every type in the merge patch: the
// planned value for a subscribed type, and an explicit null for every other
// one. A type the patch leaves out stays as stored, so null is what
// unsubscribes it. The API merges a sent type into the stored one, so every
// known filter list is sent, [] included: a list left out would keep its stored
// value. An empty type is sent as a non-nil empty map, because Set(nil) would
// marshal to null.
func toCustomerNotificationPatchSettings(ctx context.Context, s resource_customer_notification.SettingsValue, out *models.PatchCustomerNotificationSettings) diag.Diagnostics {
	var diags diag.Diagnostics

	if notificationSubscribed(s.CreditUtilization) {
		out.CreditUtilization.Set(models.PatchNotificationEmptySettings{})
	} else {
		out.CreditUtilization.SetNull()
	}
	if notificationSubscribed(s.QuotaUtilization) {
		out.QuotaUtilization.Set(models.PatchNotificationEmptySettings{})
	} else {
		out.QuotaUtilization.SetNull()
	}
	if notificationSubscribed(s.CloudIncidents) {
		out.CloudIncidents.Set(models.PatchNotificationEmptySettings{})
	} else {
		out.CloudIncidents.SetNull()
	}
	if notificationSubscribed(s.NewSkuUsage) {
		out.NewSkuUsage.Set(models.PatchNotificationEmptySettings{})
	} else {
		out.NewSkuUsage.SetNull()
	}
	if notificationSubscribed(s.MonthlyDigest) {
		out.MonthlyDigest.Set(models.PatchNotificationEmptySettings{})
	} else {
		out.MonthlyDigest.SetNull()
	}
	if notificationSubscribed(s.ExpertInsights) {
		out.ExpertInsights.Set(models.PatchNotificationEmptySettings{})
	} else {
		out.ExpertInsights.SetNull()
	}
	if notificationSubscribed(s.SavingsPlanApproval) {
		out.SavingsPlanApproval.Set(models.PatchNotificationEmptySettings{})
	} else {
		out.SavingsPlanApproval.SetNull()
	}
	if notificationSubscribed(s.SavingsPlanPurchased) {
		out.SavingsPlanPurchased.Set(models.PatchNotificationEmptySettings{})
	} else {
		out.SavingsPlanPurchased.SetNull()
	}
	if notificationSubscribed(s.CudApproval) {
		out.CudApproval.Set(models.PatchNotificationEmptySettings{})
	} else {
		out.CudApproval.SetNull()
	}
	if notificationSubscribed(s.CudPurchased) {
		out.CudPurchased.Set(models.PatchNotificationEmptySettings{})
	} else {
		out.CudPurchased.SetNull()
	}
	if notificationSubscribed(s.DataIntegrationSyncFailure) {
		out.DataIntegrationSyncFailure.Set(models.PatchNotificationEmptySettings{})
	} else {
		out.DataIntegrationSyncFailure.SetNull()
	}
	if notificationSubscribed(s.NewInvoices) {
		v, d := patchCustomerNotificationBillingProfiles(ctx, s.NewInvoices.BillingProfileIds)
		diags.Append(d...)
		out.NewInvoices.Set(v)
	} else {
		out.NewInvoices.SetNull()
	}
	if notificationSubscribed(s.NewProformaInvoices) {
		v, d := patchCustomerNotificationBillingProfiles(ctx, s.NewProformaInvoices.BillingProfileIds)
		diags.Append(d...)
		out.NewProformaInvoices.Set(v)
	} else {
		out.NewProformaInvoices.SetNull()
	}
	if notificationSubscribed(s.PaymentReminders) {
		v, d := patchCustomerNotificationBillingProfiles(ctx, s.PaymentReminders.BillingProfileIds)
		diags.Append(d...)
		out.PaymentReminders.Set(v)
	} else {
		out.PaymentReminders.SetNull()
	}
	if notificationSubscribed(s.AwsBillingDataImport) {
		v, d := patchCustomerNotificationBillingProfiles(ctx, s.AwsBillingDataImport.BillingProfileIds)
		diags.Append(d...)
		out.AwsBillingDataImport.Set(v)
	} else {
		out.AwsBillingDataImport.SetNull()
	}
	if notificationSubscribed(s.DailyDigest) {
		v, d := patchCustomerNotificationDigest(ctx, s.DailyDigest.AllocationIds)
		diags.Append(d...)
		out.DailyDigest.Set(v)
	} else {
		out.DailyDigest.SetNull()
	}
	if notificationSubscribed(s.WeeklyDigest) {
		v, d := patchCustomerNotificationDigest(ctx, s.WeeklyDigest.AllocationIds)
		diags.Append(d...)
		out.WeeklyDigest.Set(v)
	} else {
		out.WeeklyDigest.SetNull()
	}
	if notificationSubscribed(s.Insights) {
		v, d := patchCustomerNotificationInsights(ctx, s.Insights)
		diags.Append(d...)
		out.Insights.Set(v)
	} else {
		out.Insights.SetNull()
	}
	if notificationSubscribed(s.CostAnomalies) {
		v, d := toPatchCustomerNotificationCostAnomalies(ctx, s.CostAnomalies)
		diags.Append(d...)
		out.CostAnomalies.Set(v)
	} else {
		out.CostAnomalies.SetNull()
	}

	return diags
}

func patchCustomerNotificationBillingProfiles(ctx context.Context, ids types.List) (models.PatchBillingProfileNotificationSettings, diag.Diagnostics) {
	list, diags := notificationListPointer(ctx, ids)
	return models.PatchBillingProfileNotificationSettings{BillingProfileIds: list}, diags
}

func patchCustomerNotificationDigest(ctx context.Context, ids types.List) (models.PatchDigestNotificationSettings, diag.Diagnostics) {
	allocationIDs := []string{}
	diags := ids.ElementsAs(ctx, &allocationIDs, false)
	return models.PatchDigestNotificationSettings{AllocationIds: &allocationIDs}, diags
}

func patchCustomerNotificationInsights(ctx context.Context, v resource_customer_notification.InsightsValue) (models.PatchInsightsNotificationSettings, diag.Diagnostics) {
	var diags diag.Diagnostics
	pillars, d := notificationListPointer(ctx, v.Pillars)
	diags.Append(d...)
	severities, d := notificationListPointer(ctx, v.Severities)
	diags.Append(d...)
	providers, d := notificationListPointer(ctx, v.Providers)
	diags.Append(d...)
	return models.PatchInsightsNotificationSettings{Pillars: pillars, Severities: severities, Providers: providers}, diags
}
