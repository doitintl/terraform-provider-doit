package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_user_notification"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/oapi-codegen/nullable"
)

type userNotificationResourceModel struct {
	resource_user_notification.UserNotificationModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

// populateState fetches the notification from the API and populates the Terraform state.
// On 404, state.Id is set to null to signal Terraform to remove the resource from state.
func (r *userNotificationResource) populateState(ctx context.Context, state *userNotificationResourceModel) diag.Diagnostics {
	userID := state.UserId.ValueString()
	id := state.Id.ValueString()

	getResp, err := r.client.GetUserNotificationWithResponse(ctx, userID, id)
	if err != nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading User Notification", fmt.Sprintf("Could not read notification %s of user %s: %s", id, userID, err)),
		}
	}

	// A notification ID under the wrong user also returns 404.
	if getResp.StatusCode() == 404 {
		state.Id = types.StringNull()
		return nil
	}

	if getResp.StatusCode() != 200 {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading User Notification", fmt.Sprintf("Unexpected status code %d for notification %s of user %s: %s", getResp.StatusCode(), id, userID, string(getResp.Body))),
		}
	}

	if getResp.JSON200 == nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading User Notification", fmt.Sprintf("Received empty response body for notification %s of user %s", id, userID)),
		}
	}

	return mapUserNotificationToModel(ctx, getResp.JSON200, state)
}

// mapUserNotificationToModel maps the API response to the Terraform model.
// user_id is not part of the response; the path parameter in state is kept.
func mapUserNotificationToModel(ctx context.Context, resp *models.UserNotification, state *userNotificationResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	state.Id = types.StringPointerValue(nullableToPointer(resp.Id))
	state.Name = types.StringValue(resp.Name)
	state.CreateTime = nullableTimeToString(resp.CreateTime)
	state.UpdateTime = nullableTimeToString(resp.UpdateTime)
	state.Etag = types.StringPointerValue(nullableToPointer(resp.Etag))

	destinations, d := buildUserNotificationDestinations(ctx, resp.Destinations)
	diags.Append(d...)
	state.Destinations = destinations

	settings, d := buildUserNotificationSettings(ctx, &resp.Settings, state.Settings.CostAnomalies)
	diags.Append(d...)
	state.Settings = settings

	return diags
}

func buildUserNotificationDestinations(ctx context.Context, destinations *[]models.UserNotificationEmailDestination) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics

	attrTypes := resource_user_notification.DestinationsValue{}.AttributeTypes(ctx)
	items := []resource_user_notification.DestinationsValue{}
	if destinations != nil {
		for _, dest := range *destinations {
			address := types.StringNull()
			if dest.Address != nil {
				address = types.StringValue(string(*dest.Address))
			}
			v, d := resource_user_notification.NewDestinationsValue(attrTypes, map[string]attr.Value{
				"address": address,
				"type":    types.StringValue(string(dest.Type)),
			})
			diags.Append(d...)
			items = append(items, v)
		}
	}

	list, d := types.ListValueFrom(ctx, resource_user_notification.DestinationsValue{}.Type(ctx), items)
	diags.Append(d...)
	return list, diags
}

// buildUserNotificationSettings maps the subscribed types. prior is the
// cost-anomaly value currently in state; it only decides how amounts are spelled.
func buildUserNotificationSettings(ctx context.Context, s *models.UserNotificationSettings, prior resource_user_notification.CostAnomaliesValue) (resource_user_notification.SettingsValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	attrTypes := resource_user_notification.SettingsValue{}.AttributeTypes(ctx)
	objects, d := userNotificationSettingsObjects(ctx, s)
	diags.Append(d...)
	values, d := userNotificationSettingsValues(ctx, attrTypes, objects)
	diags.Append(d...)

	costAnomalies, d := buildUserNotificationCostAnomalies(ctx, s.CostAnomalies, prior)
	diags.Append(d...)
	values["cost_anomalies"] = costAnomalies

	if diags.HasError() {
		return resource_user_notification.NewSettingsValueNull(), diags
	}

	settings, d := resource_user_notification.NewSettingsValue(attrTypes, values)
	diags.Append(d...)

	return settings, diags
}

func buildUserNotificationCostAnomalies(ctx context.Context, ca *models.CostAnomalySettings, prior resource_user_notification.CostAnomaliesValue) (resource_user_notification.CostAnomaliesValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	if ca == nil {
		return resource_user_notification.NewCostAnomaliesValueNull(), diags
	}

	ids := []string{}
	if ca.Allocations.Ids != nil {
		ids = ca.Allocations.Ids
	}
	idList, d := types.ListValueFrom(ctx, types.StringType, ids)
	diags.Append(d...)
	allocations, d := resource_user_notification.NewAllocationsValue(
		resource_user_notification.AllocationsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"ids":  idList,
			"mode": types.StringValue(string(ca.Allocations.Mode)),
		},
	)
	diags.Append(d...)

	minExcessCost, d := resource_user_notification.NewMinExcessCostValue(
		resource_user_notification.MinExcessCostValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"amount":   preserveCostAnomalyAmount(prior.MinExcessCost.Amount, ca.MinExcessCost.Amount),
			"currency": types.StringValue(ca.MinExcessCost.Currency),
		},
	)
	diags.Append(d...)

	repeat, d := buildUserNotificationRepeat(ctx, ca.Repeat, prior.Repeat)
	diags.Append(d...)

	costAnomalies, d := resource_user_notification.NewCostAnomaliesValue(
		resource_user_notification.CostAnomaliesValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"allocations":     allocations,
			"min_excess_cost": minExcessCost,
			"min_severity":    types.StringValue(string(ca.MinSeverity)),
			"repeat":          repeat,
		},
	)
	diags.Append(d...)

	return costAnomalies, diags
}

func buildUserNotificationRepeat(ctx context.Context, repeat *models.CostAnomalyRepeat, prior resource_user_notification.RepeatValue) (resource_user_notification.RepeatValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	// The API omits repeat when no resend control is set.
	if repeat == nil {
		return resource_user_notification.NewRepeatValueNull(), diags
	}

	whenCostGrows := resource_user_notification.NewWhenCostGrowsValueNull()
	if wcg := repeat.WhenCostGrows; wcg != nil {
		minIncrease := resource_user_notification.NewMinIncreaseValueNull()
		if wcg.MinIncrease != nil {
			var d diag.Diagnostics
			minIncrease, d = resource_user_notification.NewMinIncreaseValue(
				resource_user_notification.MinIncreaseValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"amount":   preserveCostAnomalyAmount(prior.WhenCostGrows.MinIncrease.Amount, wcg.MinIncrease.Amount),
					"currency": types.StringValue(wcg.MinIncrease.Currency),
				},
			)
			diags.Append(d...)
		}

		match := types.StringNull()
		if wcg.Match != nil {
			match = types.StringValue(string(*wcg.Match))
		}

		var d diag.Diagnostics
		whenCostGrows, d = resource_user_notification.NewWhenCostGrowsValue(
			resource_user_notification.WhenCostGrowsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"match":            match,
				"min_increase":     minIncrease,
				"percent_increase": intPointerToInt64(wcg.PercentIncrease),
				"within_hours":     intPointerToInt64(wcg.WithinHours),
			},
		)
		diags.Append(d...)
	}

	v, d := resource_user_notification.NewRepeatValue(
		resource_user_notification.RepeatValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"every_hours":     intPointerToInt64(repeat.EveryHours),
			"when_cost_grows": whenCostGrows,
		},
	)
	diags.Append(d...)

	return v, diags
}

func intPointerToInt64(v *int) types.Int64 {
	if v == nil {
		return types.Int64Null()
	}
	return types.Int64Value(int64(*v))
}

// overlayUserNotificationComputedFields applies the plan-first overlay for Create/Update.
func overlayUserNotificationComputedFields(ctx context.Context, apiResp *models.UserNotification, plan *userNotificationResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	// Phase 1: Build fully-resolved state from API response.
	resolved := *plan
	diags.Append(mapUserNotificationToModel(ctx, apiResp, &resolved)...)
	if diags.HasError() {
		return diags
	}

	// Phase 2: Overlay Computed-only fields unconditionally from resolved.
	plan.Id = resolved.Id
	plan.CreateTime = resolved.CreateTime
	plan.Destinations = resolved.Destinations

	// update_time and etag: Computed-only but change server-side on every Update.
	if plan.UpdateTime.IsUnknown() {
		plan.UpdateTime = resolved.UpdateTime //nolint:overlaycheck // guarded to avoid inconsistent result on Update
	}
	if plan.Etag.IsUnknown() {
		plan.Etag = resolved.Etag //nolint:overlaycheck // guarded to avoid inconsistent result on Update
	}

	// user_id and name: Required — never touch. settings is Required, but its
	// types and their settings are Optional+Computed.
	if plan.Settings.IsUnknown() {
		plan.Settings = resolved.Settings
	} else if !plan.Settings.IsNull() {
		overlayUserNotificationSettings(ctx, &resolved.Settings, &plan.Settings)
	}

	return diags
}

// overlayUserNotificationSettings resolves the settings the plan leaves
// unknown. A type is unknown only while its configuration is; the filter lists
// of a type that is new in this plan are unknown when omitted.
func overlayUserNotificationSettings(ctx context.Context, resolved, plan *resource_user_notification.SettingsValue) {
	if plan.CreditUtilization.IsUnknown() {
		plan.CreditUtilization = resolved.CreditUtilization
	}
	if plan.QuotaUtilization.IsUnknown() {
		plan.QuotaUtilization = resolved.QuotaUtilization
	}
	if plan.CloudIncidents.IsUnknown() {
		plan.CloudIncidents = resolved.CloudIncidents
	}
	if plan.NewSkuUsage.IsUnknown() {
		plan.NewSkuUsage = resolved.NewSkuUsage
	}
	if plan.MonthlyDigest.IsUnknown() {
		plan.MonthlyDigest = resolved.MonthlyDigest
	}
	if plan.ExpertInsights.IsUnknown() {
		plan.ExpertInsights = resolved.ExpertInsights
	}
	if plan.SavingsPlanApproval.IsUnknown() {
		plan.SavingsPlanApproval = resolved.SavingsPlanApproval
	}
	if plan.SavingsPlanPurchased.IsUnknown() {
		plan.SavingsPlanPurchased = resolved.SavingsPlanPurchased
	}
	if plan.CudApproval.IsUnknown() {
		plan.CudApproval = resolved.CudApproval
	}
	if plan.CudPurchased.IsUnknown() {
		plan.CudPurchased = resolved.CudPurchased
	}
	if plan.DataIntegrationSyncFailure.IsUnknown() {
		plan.DataIntegrationSyncFailure = resolved.DataIntegrationSyncFailure
	}
	if plan.DailyDigest.IsUnknown() {
		plan.DailyDigest = resolved.DailyDigest
	}
	if plan.WeeklyDigest.IsUnknown() {
		plan.WeeklyDigest = resolved.WeeklyDigest
	}
	if plan.NewInvoices.IsUnknown() {
		plan.NewInvoices = resolved.NewInvoices
	} else if !plan.NewInvoices.IsNull() {
		overlayUserNotificationNewInvoices(ctx, &resolved.NewInvoices, &plan.NewInvoices)
	}
	if plan.NewProformaInvoices.IsUnknown() {
		plan.NewProformaInvoices = resolved.NewProformaInvoices
	} else if !plan.NewProformaInvoices.IsNull() {
		overlayUserNotificationNewProformaInvoices(ctx, &resolved.NewProformaInvoices, &plan.NewProformaInvoices)
	}
	if plan.PaymentReminders.IsUnknown() {
		plan.PaymentReminders = resolved.PaymentReminders
	} else if !plan.PaymentReminders.IsNull() {
		overlayUserNotificationPaymentReminders(ctx, &resolved.PaymentReminders, &plan.PaymentReminders)
	}
	if plan.Insights.IsUnknown() {
		plan.Insights = resolved.Insights
	} else if !plan.Insights.IsNull() {
		overlayUserNotificationInsights(ctx, &resolved.Insights, &plan.Insights)
	}
	if plan.CostAnomalies.IsUnknown() {
		plan.CostAnomalies = resolved.CostAnomalies
	} else if !plan.CostAnomalies.IsNull() {
		overlayUserNotificationCostAnomalies(ctx, &resolved.CostAnomalies, &plan.CostAnomalies)
	}
}

func overlayUserNotificationNewInvoices(_ context.Context, resolved, plan *resource_user_notification.NewInvoicesValue) {
	if plan.BillingProfileIds.IsUnknown() {
		plan.BillingProfileIds = resolved.BillingProfileIds
	}
}

func overlayUserNotificationNewProformaInvoices(_ context.Context, resolved, plan *resource_user_notification.NewProformaInvoicesValue) {
	if plan.BillingProfileIds.IsUnknown() {
		plan.BillingProfileIds = resolved.BillingProfileIds
	}
}

func overlayUserNotificationPaymentReminders(_ context.Context, resolved, plan *resource_user_notification.PaymentRemindersValue) {
	if plan.BillingProfileIds.IsUnknown() {
		plan.BillingProfileIds = resolved.BillingProfileIds
	}
}

func overlayUserNotificationInsights(_ context.Context, resolved, plan *resource_user_notification.InsightsValue) {
	if plan.Pillars.IsUnknown() {
		plan.Pillars = resolved.Pillars
	}
	if plan.Severities.IsUnknown() {
		plan.Severities = resolved.Severities
	}
	if plan.Providers.IsUnknown() {
		plan.Providers = resolved.Providers
	}
}

func overlayUserNotificationCostAnomalies(ctx context.Context, resolved, plan *resource_user_notification.CostAnomaliesValue) {
	if plan.Repeat.IsUnknown() {
		plan.Repeat = resolved.Repeat
	} else if !plan.Repeat.IsNull() {
		overlayUserNotificationRepeat(ctx, &resolved.Repeat, &plan.Repeat)
	}
}

func overlayUserNotificationRepeat(ctx context.Context, resolved, plan *resource_user_notification.RepeatValue) {
	if plan.EveryHours.IsUnknown() {
		plan.EveryHours = resolved.EveryHours
	}
	if plan.WhenCostGrows.IsUnknown() {
		plan.WhenCostGrows = resolved.WhenCostGrows
	} else if !plan.WhenCostGrows.IsNull() {
		overlayUserNotificationWhenCostGrows(ctx, &resolved.WhenCostGrows, &plan.WhenCostGrows)
	}
}

func overlayUserNotificationWhenCostGrows(_ context.Context, resolved, plan *resource_user_notification.WhenCostGrowsValue) {
	if plan.Match.IsUnknown() {
		plan.Match = resolved.Match
	}
	if plan.PercentIncrease.IsUnknown() {
		plan.PercentIncrease = resolved.PercentIncrease
	}
	if plan.WithinHours.IsUnknown() {
		plan.WithinHours = resolved.WithinHours
	}
	if plan.MinIncrease.IsUnknown() {
		plan.MinIncrease = resolved.MinIncrease
	}
}

// toCreateRequest converts the TF model to a CreateUserNotificationRequest.
func (plan *userNotificationResourceModel) toCreateRequest(ctx context.Context) (models.CreateUserNotificationJSONRequestBody, diag.Diagnostics) {
	var diags diag.Diagnostics

	req := models.CreateUserNotificationJSONRequestBody{
		Name: plan.Name.ValueString(),
	}

	diags.Append(toUserNotificationCreateSettings(ctx, plan.Settings, &req.Settings)...)

	return req, diags
}

func toCreateCostAnomalies(ctx context.Context, ca resource_user_notification.CostAnomaliesValue) (models.CostAnomalySettings, diag.Diagnostics) {
	ids, diags := userNotificationAllocationIDs(ctx, ca.Allocations)

	out := models.CostAnomalySettings{
		MinSeverity: models.CostAnomalySettingsMinSeverity(ca.MinSeverity.ValueString()),
		MinExcessCost: models.Money{
			Amount:   ca.MinExcessCost.Amount.ValueString(),
			Currency: ca.MinExcessCost.Currency.ValueString(),
		},
		Allocations: models.CostAnomalyAllocations{
			Mode: models.CostAnomalyAllocationsMode(ca.Allocations.Mode.ValueString()),
			Ids:  ids,
		},
	}

	if !ca.Repeat.IsNull() && !ca.Repeat.IsUnknown() {
		repeat := &models.CostAnomalyRepeat{
			EveryHours: int64ToIntPointer(ca.Repeat.EveryHours),
		}
		if wcg := ca.Repeat.WhenCostGrows; !wcg.IsNull() && !wcg.IsUnknown() {
			repeat.WhenCostGrows = &models.CostAnomalyWhenCostGrows{
				PercentIncrease: int64ToIntPointer(wcg.PercentIncrease),
				WithinHours:     int64ToIntPointer(wcg.WithinHours),
			}
			if !wcg.Match.IsNull() && !wcg.Match.IsUnknown() {
				repeat.WhenCostGrows.Match = new(models.CostAnomalyWhenCostGrowsMatch(wcg.Match.ValueString()))
			}
			if !wcg.MinIncrease.IsNull() && !wcg.MinIncrease.IsUnknown() {
				repeat.WhenCostGrows.MinIncrease = &models.Money{
					Amount:   wcg.MinIncrease.Amount.ValueString(),
					Currency: wcg.MinIncrease.Currency.ValueString(),
				}
			}
		}
		out.Repeat = repeat
	}

	return out, diags
}

// toUpdateRequest converts the TF model to a JSON merge patch (RFC 7396).
//
// The patch always carries the full desired settings. Nested objects are
// deep-merged server-side, so every type the plan does not subscribe to, and
// every optional member of repeat, is sent as an explicit value or an explicit
// null — omitting it would keep the stored value.
func (plan *userNotificationResourceModel) toUpdateRequest(ctx context.Context) (models.UpdateUserNotificationApplicationMergePatchPlusJSONRequestBody, diag.Diagnostics) {
	var diags diag.Diagnostics

	settings := &models.PatchUserNotificationSettings{}
	diags.Append(toUserNotificationPatchSettings(ctx, plan.Settings, settings)...)

	req := models.UpdateUserNotificationApplicationMergePatchPlusJSONRequestBody{
		Name:     new(plan.Name.ValueString()),
		Settings: settings,
	}

	return req, diags
}

// toPatchCostAnomalies builds the cost-anomaly part of the merge patch.
func toPatchCostAnomalies(ctx context.Context, ca resource_user_notification.CostAnomaliesValue) (models.PatchCostAnomalySettings, diag.Diagnostics) {
	ids, diags := userNotificationAllocationIDs(ctx, ca.Allocations)

	patch := models.PatchCostAnomalySettings{
		MinSeverity: new(models.PatchCostAnomalySettingsMinSeverity(ca.MinSeverity.ValueString())),
		MinExcessCost: &models.Money{
			Amount:   ca.MinExcessCost.Amount.ValueString(),
			Currency: ca.MinExcessCost.Currency.ValueString(),
		},
		Allocations: &models.PatchCostAnomalyAllocations{
			Mode: new(models.PatchCostAnomalyAllocationsMode(ca.Allocations.Mode.ValueString())),
			Ids:  &ids,
		},
	}

	if ca.Repeat.IsNull() || ca.Repeat.IsUnknown() {
		patch.Repeat.SetNull()
	} else {
		repeat := models.PatchCostAnomalyRepeat{
			EveryHours: int64ToNullableInt(ca.Repeat.EveryHours),
		}
		if wcg := ca.Repeat.WhenCostGrows; wcg.IsNull() || wcg.IsUnknown() {
			repeat.WhenCostGrows.SetNull()
		} else {
			patchWCG := models.PatchCostAnomalyWhenCostGrows{
				PercentIncrease: int64ToNullableInt(wcg.PercentIncrease),
				WithinHours:     int64ToNullableInt(wcg.WithinHours),
			}
			if wcg.Match.IsNull() || wcg.Match.IsUnknown() {
				patchWCG.Match.SetNull()
			} else {
				patchWCG.Match.Set(models.PatchCostAnomalyWhenCostGrowsMatch(wcg.Match.ValueString()))
			}
			if wcg.MinIncrease.IsNull() || wcg.MinIncrease.IsUnknown() {
				patchWCG.MinIncrease.SetNull()
			} else {
				patchWCG.MinIncrease.Set(models.Money{
					Amount:   wcg.MinIncrease.Amount.ValueString(),
					Currency: wcg.MinIncrease.Currency.ValueString(),
				})
			}
			repeat.WhenCostGrows.Set(patchWCG)
		}
		patch.Repeat.Set(repeat)
	}

	return patch, diags
}

// userNotificationAllocationIDs returns the configured allocation IDs, never nil:
// the API requires the ids array even when it is empty (mode "all").
func userNotificationAllocationIDs(ctx context.Context, allocations resource_user_notification.AllocationsValue) ([]string, diag.Diagnostics) {
	ids := []string{}
	diags := allocations.Ids.ElementsAs(ctx, &ids, false)
	return ids, diags
}

// int64ToIntPointer returns nil for null and for unknown: an omitted resend
// control that is not stored plans unknown, and there is nothing to send.
func int64ToIntPointer(v types.Int64) *int {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	return new(int(v.ValueInt64()))
}

// int64ToNullableInt returns an explicit null for null and for unknown (an
// omitted control that is not stored).
func int64ToNullableInt(v types.Int64) nullable.Nullable[int] {
	if v.IsNull() || v.IsUnknown() {
		return nullable.NewNullNullable[int]()
	}
	return valueToNullable(int(v.ValueInt64()))
}
