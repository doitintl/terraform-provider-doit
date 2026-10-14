package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_user_notification"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type userNotificationResourceModel struct {
	resource_user_notification.UserNotificationModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (r *userNotificationResource) populateState(ctx context.Context, state *userNotificationResourceModel) diag.Diagnostics {
	getResp, err := r.client.GetUserNotificationWithResponse(ctx, state.UserId.ValueString(), state.Id.ValueString())
	if err != nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic(
				"Error Reading User Notification",
				"Could not read user notification ID "+state.Id.ValueString()+": "+err.Error(),
			),
		}
	}

	if getResp.StatusCode() == 404 {
		state.Id = types.StringNull()
		return nil
	}

	if getResp.StatusCode() != 200 {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic(
				"Error Reading User Notification",
				fmt.Sprintf(
					"Unexpected status code %d for user notification ID %s: %s",
					getResp.StatusCode(),
					state.Id.ValueString(),
					string(getResp.Body),
				),
			),
		}
	}

	if getResp.JSON200 == nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic(
				"Error Reading User Notification",
				"Received empty response body for user notification ID "+state.Id.ValueString(),
			),
		}
	}

	return mapUserNotificationToModel(ctx, getResp.JSON200, state)
}

func mapUserNotificationToModel(ctx context.Context, resp *models.UserNotification, state *userNotificationResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	id := nullableToPointer(resp.Id)
	state.Id = types.StringPointerValue(id)
	state.NotificationId = types.StringPointerValue(id)
	state.Name = types.StringValue(resp.Name)
	state.Etag = types.StringPointerValue(nullableToPointer(resp.Etag))

	if t := nullableToPointer(resp.CreateTime); t != nil {
		state.CreateTime = types.StringValue(t.Format(time.RFC3339))
	} else {
		state.CreateTime = types.StringNull()
	}

	if t := nullableToPointer(resp.UpdateTime); t != nil {
		state.UpdateTime = types.StringValue(t.Format(time.RFC3339))
	} else {
		state.UpdateTime = types.StringNull()
	}

	destinations, destDiags := mapDestinationsToModel(ctx, resp.Destinations)
	diags.Append(destDiags...)
	state.Destinations = destinations

	settings, settingsDiags := mapSettingsToModel(ctx, resp.Settings)
	diags.Append(settingsDiags...)
	state.Settings = settings

	return diags
}

func mapDestinationsToModel(ctx context.Context, destinations *[]models.UserNotificationEmailDestination) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics

	elemType := resource_user_notification.DestinationsType{
		ObjectType: types.ObjectType{
			AttrTypes: resource_user_notification.DestinationsValue{}.AttributeTypes(ctx),
		},
	}

	if destinations == nil || len(*destinations) == 0 {
		listVal, listDiags := types.ListValue(elemType, []attr.Value{})
		diags.Append(listDiags...)
		return listVal, diags
	}

	values := make([]attr.Value, 0, len(*destinations))
	for _, d := range *destinations {
		address := ""
		if d.Address != nil {
			address = string(*d.Address)
		}
		values = append(values, resource_user_notification.NewDestinationsValueMust(
			resource_user_notification.DestinationsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"address": types.StringValue(address),
				"type":    types.StringValue(d.Type),
			},
		))
	}

	listVal, listDiags := types.ListValue(elemType, values)
	diags.Append(listDiags...)
	return listVal, diags
}

func mapSettingsToModel(ctx context.Context, settings models.UserNotificationResourceSettings) (resource_user_notification.SettingsValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	ca := settings.CostAnomalies

	ids := ca.Allocations.Ids
	if ids == nil {
		ids = []string{}
	}
	idsList, idsDiags := types.ListValueFrom(ctx, types.StringType, ids)
	diags.Append(idsDiags...)

	allocations := resource_user_notification.NewAllocationsValueMust(
		resource_user_notification.AllocationsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"ids":  idsList,
			"mode": types.StringValue(string(ca.Allocations.Mode)),
		},
	)

	minExcess := resource_user_notification.NewMinExcessCostValueMust(
		resource_user_notification.MinExcessCostValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"amount":   types.StringValue(normalizeUSDWholeDollarAmount(ca.MinExcessCost.Amount)),
			"currency": types.StringValue(ca.MinExcessCost.Currency),
		},
	)

	repeat := resource_user_notification.NewRepeatValueNull()
	if ca.Repeat != nil {
		repeatVal, repeatDiags := mapRepeatToModel(ctx, ca.Repeat)
		diags.Append(repeatDiags...)
		repeat = repeatVal
	}

	costAnomalies := resource_user_notification.NewCostAnomaliesValueMust(
		resource_user_notification.CostAnomaliesValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"allocations":     allocations,
			"min_excess_cost": minExcess,
			"min_severity":    types.StringValue(string(ca.MinSeverity)),
			"repeat":          repeat,
		},
	)

	return resource_user_notification.NewSettingsValueMust(
		resource_user_notification.SettingsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"cost_anomalies": costAnomalies,
		},
	), diags
}

func mapRepeatToModel(ctx context.Context, repeat *models.CostAnomalyRepeat) (resource_user_notification.RepeatValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	everyHours := types.Int64Null()
	if repeat.EveryHours != nil {
		everyHours = types.Int64Value(int64(*repeat.EveryHours))
	}

	whenCostGrows := resource_user_notification.NewWhenCostGrowsValueNull()
	if repeat.WhenCostGrows != nil {
		wcg, wcgDiags := mapWhenCostGrowsToModel(ctx, repeat.WhenCostGrows)
		diags.Append(wcgDiags...)
		whenCostGrows = wcg
	}

	return resource_user_notification.NewRepeatValueMust(
		resource_user_notification.RepeatValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"every_hours":     everyHours,
			"when_cost_grows": whenCostGrows,
		},
	), diags
}

func mapWhenCostGrowsToModel(ctx context.Context, wcg *models.CostAnomalyWhenCostGrows) (resource_user_notification.WhenCostGrowsValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	match := types.StringNull()
	if wcg.Match != nil {
		match = types.StringValue(string(*wcg.Match))
	}

	percentIncrease := types.Int64Null()
	if wcg.PercentIncrease != nil {
		percentIncrease = types.Int64Value(int64(*wcg.PercentIncrease))
	}

	withinHours := types.Int64Null()
	if wcg.WithinHours != nil {
		withinHours = types.Int64Value(int64(*wcg.WithinHours))
	}

	minIncrease := resource_user_notification.NewMinIncreaseValueNull()
	if wcg.MinIncrease != nil {
		minIncrease = resource_user_notification.NewMinIncreaseValueMust(
			resource_user_notification.MinIncreaseValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"amount":   types.StringValue(normalizeUSDWholeDollarAmount(wcg.MinIncrease.Amount)),
				"currency": types.StringValue(wcg.MinIncrease.Currency),
			},
		)
	}

	return resource_user_notification.NewWhenCostGrowsValueMust(
		resource_user_notification.WhenCostGrowsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"match":            match,
			"min_increase":     minIncrease,
			"percent_increase": percentIncrease,
			"within_hours":     withinHours,
		},
	), diags
}

func overlayUserNotificationComputedFields(ctx context.Context, apiResp *models.UserNotification, plan *userNotificationResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	resolved := *plan
	diags.Append(mapUserNotificationToModel(ctx, apiResp, &resolved)...)
	if diags.HasError() {
		return diags
	}

	plan.Id = resolved.Id
	plan.NotificationId = resolved.NotificationId
	plan.CreateTime = resolved.CreateTime
	plan.Destinations = resolved.Destinations

	if plan.UpdateTime.IsUnknown() {
		plan.UpdateTime = resolved.UpdateTime //nolint:overlaycheck // guarded to avoid inconsistent result on Update
	}
	if plan.Etag.IsUnknown() {
		plan.Etag = resolved.Etag //nolint:overlaycheck // guarded to avoid inconsistent result on Update
	}

	// Keep configured settings, but adopt API-normalized money amounts.
	if !plan.Settings.IsNull() && !plan.Settings.IsUnknown() {
		plan.Settings = overlayNormalizedMoney(ctx, plan.Settings, resolved.Settings)
	} else if plan.Settings.IsUnknown() {
		plan.Settings = resolved.Settings
	}

	return diags
}

func overlayNormalizedMoney(ctx context.Context, planSettings, resolvedSettings resource_user_notification.SettingsValue) resource_user_notification.SettingsValue {
	if planSettings.CostAnomalies.IsNull() || planSettings.CostAnomalies.IsUnknown() {
		return planSettings
	}
	if resolvedSettings.CostAnomalies.IsNull() || resolvedSettings.CostAnomalies.IsUnknown() {
		return planSettings
	}

	ca := planSettings.CostAnomalies
	resolvedCA := resolvedSettings.CostAnomalies

	minExcess := ca.MinExcessCost
	if !resolvedCA.MinExcessCost.IsNull() && !resolvedCA.MinExcessCost.IsUnknown() {
		minExcess = resolvedCA.MinExcessCost
	}

	repeat := ca.Repeat
	if !ca.Repeat.IsNull() && !ca.Repeat.IsUnknown() &&
		!ca.Repeat.WhenCostGrows.IsNull() && !ca.Repeat.WhenCostGrows.IsUnknown() &&
		!resolvedCA.Repeat.IsNull() && !resolvedCA.Repeat.IsUnknown() &&
		!resolvedCA.Repeat.WhenCostGrows.IsNull() && !resolvedCA.Repeat.WhenCostGrows.IsUnknown() &&
		!resolvedCA.Repeat.WhenCostGrows.MinIncrease.IsNull() && !resolvedCA.Repeat.WhenCostGrows.MinIncrease.IsUnknown() {
		wcg := resource_user_notification.NewWhenCostGrowsValueMust(
			resource_user_notification.WhenCostGrowsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"match":            ca.Repeat.WhenCostGrows.Match,
				"min_increase":     resolvedCA.Repeat.WhenCostGrows.MinIncrease,
				"percent_increase": ca.Repeat.WhenCostGrows.PercentIncrease,
				"within_hours":     ca.Repeat.WhenCostGrows.WithinHours,
			},
		)
		repeat = resource_user_notification.NewRepeatValueMust(
			resource_user_notification.RepeatValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"every_hours":     ca.Repeat.EveryHours,
				"when_cost_grows": wcg,
			},
		)
	}

	costAnomalies := resource_user_notification.NewCostAnomaliesValueMust(
		resource_user_notification.CostAnomaliesValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"allocations":     ca.Allocations,
			"min_excess_cost": minExcess,
			"min_severity":    ca.MinSeverity,
			"repeat":          repeat,
		},
	)

	return resource_user_notification.NewSettingsValueMust(
		resource_user_notification.SettingsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"cost_anomalies": costAnomalies,
		},
	)
}

func (plan *userNotificationResourceModel) toCreateRequest(ctx context.Context) (models.CreateUserNotificationJSONRequestBody, diag.Diagnostics) {
	var diags diag.Diagnostics

	settings, settingsDiags := plan.settingsToAPI(ctx)
	diags.Append(settingsDiags...)
	if diags.HasError() {
		return models.CreateUserNotificationJSONRequestBody{}, diags
	}

	return models.CreateUserNotificationJSONRequestBody{
		Name:     plan.Name.ValueString(),
		Settings: settings,
	}, diags
}

func (plan *userNotificationResourceModel) settingsToAPI(ctx context.Context) (models.UserNotificationSettings, diag.Diagnostics) {
	var diags diag.Diagnostics
	ca := plan.Settings.CostAnomalies

	var ids []string
	diags.Append(ca.Allocations.Ids.ElementsAs(ctx, &ids, false)...)
	if diags.HasError() {
		return models.UserNotificationSettings{}, diags
	}
	if ids == nil {
		ids = []string{}
	}

	out := models.CostAnomalySettings{
		Allocations: models.CostAnomalyAllocations{
			Ids:  ids,
			Mode: models.CostAnomalyAllocationsMode(ca.Allocations.Mode.ValueString()),
		},
		MinExcessCost: models.Money{
			Amount:   normalizeUSDWholeDollarAmount(ca.MinExcessCost.Amount.ValueString()),
			Currency: ca.MinExcessCost.Currency.ValueString(),
		},
		MinSeverity: models.CostAnomalySettingsMinSeverity(ca.MinSeverity.ValueString()),
	}

	if !ca.Repeat.IsNull() && !ca.Repeat.IsUnknown() {
		repeat, repeatDiags := repeatToAPI(ctx, ca.Repeat)
		diags.Append(repeatDiags...)
		if diags.HasError() {
			return models.UserNotificationSettings{}, diags
		}
		out.Repeat = repeat
	}

	return models.UserNotificationSettings{CostAnomalies: out}, diags
}

func repeatToAPI(ctx context.Context, repeat resource_user_notification.RepeatValue) (*models.CostAnomalyRepeat, diag.Diagnostics) {
	var diags diag.Diagnostics
	out := &models.CostAnomalyRepeat{}

	if !repeat.EveryHours.IsNull() && !repeat.EveryHours.IsUnknown() {
		v := int(repeat.EveryHours.ValueInt64())
		out.EveryHours = &v
	}

	if !repeat.WhenCostGrows.IsNull() && !repeat.WhenCostGrows.IsUnknown() {
		wcg, wcgDiags := whenCostGrowsToAPI(ctx, repeat.WhenCostGrows)
		diags.Append(wcgDiags...)
		if diags.HasError() {
			return nil, diags
		}
		out.WhenCostGrows = wcg
	}

	return out, diags
}

func whenCostGrowsToAPI(_ context.Context, wcg resource_user_notification.WhenCostGrowsValue) (*models.CostAnomalyWhenCostGrows, diag.Diagnostics) {
	var diags diag.Diagnostics
	out := &models.CostAnomalyWhenCostGrows{}

	if !wcg.Match.IsNull() && !wcg.Match.IsUnknown() {
		m := models.CostAnomalyWhenCostGrowsMatch(wcg.Match.ValueString())
		out.Match = &m
	}
	if !wcg.PercentIncrease.IsNull() && !wcg.PercentIncrease.IsUnknown() {
		v := int(wcg.PercentIncrease.ValueInt64())
		out.PercentIncrease = &v
	}
	if !wcg.WithinHours.IsNull() && !wcg.WithinHours.IsUnknown() {
		v := int(wcg.WithinHours.ValueInt64())
		out.WithinHours = &v
	}
	if !wcg.MinIncrease.IsNull() && !wcg.MinIncrease.IsUnknown() {
		out.MinIncrease = &models.Money{
			Amount:   normalizeUSDWholeDollarAmount(wcg.MinIncrease.Amount.ValueString()),
			Currency: wcg.MinIncrease.Currency.ValueString(),
		}
	}

	return out, diags
}

// updateBodyJSON builds a merge-patch body. When repeat is omitted from config it
// sends `"repeat": null` so a prior resend config is cleared.
func (plan *userNotificationResourceModel) updateBodyJSON(ctx context.Context) ([]byte, diag.Diagnostics) {
	var diags diag.Diagnostics

	settings, settingsDiags := plan.settingsToAPI(ctx)
	diags.Append(settingsDiags...)
	if diags.HasError() {
		return nil, diags
	}

	ca := map[string]any{
		"minSeverity":   string(settings.CostAnomalies.MinSeverity),
		"minExcessCost": settings.CostAnomalies.MinExcessCost,
		"allocations": map[string]any{
			"mode": string(settings.CostAnomalies.Allocations.Mode),
			"ids":  settings.CostAnomalies.Allocations.Ids,
		},
	}

	if settings.CostAnomalies.Repeat != nil {
		ca["repeat"] = settings.CostAnomalies.Repeat
	} else {
		ca["repeat"] = nil
	}

	body := map[string]any{
		"name": plan.Name.ValueString(),
		"settings": map[string]any{
			"costAnomalies": ca,
		},
	}

	buf, err := json.Marshal(body)
	if err != nil {
		diags.AddError("Error Encoding User Notification Update", err.Error())
		return nil, diags
	}
	return buf, diags
}

func normalizeUSDWholeDollarAmount(amount string) string {
	amount = strings.TrimSpace(amount)
	f, err := strconv.ParseFloat(amount, 64)
	if err != nil {
		return amount
	}
	if f < 0 {
		return amount
	}
	whole := math.Floor(f)
	return fmt.Sprintf("%.2f", whole)
}

func normalizeSettingsMoneyInPlace(ctx context.Context, settings resource_user_notification.SettingsValue) (resource_user_notification.SettingsValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	if settings.IsNull() || settings.IsUnknown() || settings.CostAnomalies.IsNull() || settings.CostAnomalies.IsUnknown() {
		return settings, diags
	}

	ca := settings.CostAnomalies
	minExcess := ca.MinExcessCost
	if !minExcess.IsNull() && !minExcess.IsUnknown() && !minExcess.Amount.IsNull() && !minExcess.Amount.IsUnknown() {
		minExcess = resource_user_notification.NewMinExcessCostValueMust(
			resource_user_notification.MinExcessCostValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"amount":   types.StringValue(normalizeUSDWholeDollarAmount(minExcess.Amount.ValueString())),
				"currency": minExcess.Currency,
			},
		)
	}

	repeat := ca.Repeat
	if !repeat.IsNull() && !repeat.IsUnknown() &&
		!repeat.WhenCostGrows.IsNull() && !repeat.WhenCostGrows.IsUnknown() &&
		!repeat.WhenCostGrows.MinIncrease.IsNull() && !repeat.WhenCostGrows.MinIncrease.IsUnknown() &&
		!repeat.WhenCostGrows.MinIncrease.Amount.IsNull() && !repeat.WhenCostGrows.MinIncrease.Amount.IsUnknown() {
		mi := resource_user_notification.NewMinIncreaseValueMust(
			resource_user_notification.MinIncreaseValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"amount":   types.StringValue(normalizeUSDWholeDollarAmount(repeat.WhenCostGrows.MinIncrease.Amount.ValueString())),
				"currency": repeat.WhenCostGrows.MinIncrease.Currency,
			},
		)
		wcg := resource_user_notification.NewWhenCostGrowsValueMust(
			resource_user_notification.WhenCostGrowsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"match":            repeat.WhenCostGrows.Match,
				"min_increase":     mi,
				"percent_increase": repeat.WhenCostGrows.PercentIncrease,
				"within_hours":     repeat.WhenCostGrows.WithinHours,
			},
		)
		repeat = resource_user_notification.NewRepeatValueMust(
			resource_user_notification.RepeatValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"every_hours":     repeat.EveryHours,
				"when_cost_grows": wcg,
			},
		)
	}

	costAnomalies := resource_user_notification.NewCostAnomaliesValueMust(
		resource_user_notification.CostAnomaliesValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"allocations":     ca.Allocations,
			"min_excess_cost": minExcess,
			"min_severity":    ca.MinSeverity,
			"repeat":          repeat,
		},
	)

	return resource_user_notification.NewSettingsValueMust(
		resource_user_notification.SettingsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"cost_anomalies": costAnomalies,
		},
	), diags
}
