package provider

import (
	"context"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_user_notification"
	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_user_notifications"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// The singular and plural data sources generate separate nested types, so each
// has its own builders. Both map the API values as returned: unlike the
// resource there is no configuration to keep, so amounts keep the API's
// two-decimal spelling ("100.00").

// mapUserNotificationToDataSource maps the API response to the singular data
// source model. user_id is not part of the response; the configured value is kept.
func mapUserNotificationToDataSource(ctx context.Context, resp *models.UserNotification, data *userNotificationDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	data.Id = types.StringPointerValue(nullableToPointer(resp.Id))
	data.Name = types.StringValue(resp.Name)
	data.CreateTime = nullableTimeToString(resp.CreateTime)
	data.UpdateTime = nullableTimeToString(resp.UpdateTime)
	data.Etag = types.StringPointerValue(nullableToPointer(resp.Etag))

	destinations, d := buildUserNotificationDataSourceDestinations(ctx, resp.Destinations)
	diags.Append(d...)
	data.Destinations = destinations

	settings, d := buildUserNotificationDataSourceSettings(ctx, &resp.Settings.CostAnomalies)
	diags.Append(d...)
	data.Settings = settings

	return diags
}

func buildUserNotificationDataSourceDestinations(ctx context.Context, destinations *[]models.UserNotificationEmailDestination) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics

	attrTypes := datasource_user_notification.DestinationsValue{}.AttributeTypes(ctx)
	items := []datasource_user_notification.DestinationsValue{}
	if destinations != nil {
		for _, dest := range *destinations {
			v, d := datasource_user_notification.NewDestinationsValue(attrTypes, map[string]attr.Value{
				"address": userNotificationDestinationAddress(dest),
				"type":    types.StringValue(string(dest.Type)),
			})
			diags.Append(d...)
			items = append(items, v)
		}
	}

	list, d := types.ListValueFrom(ctx, datasource_user_notification.DestinationsValue{}.Type(ctx), items)
	diags.Append(d...)
	return list, diags
}

func buildUserNotificationDataSourceSettings(ctx context.Context, ca *models.CostAnomalySettings) (datasource_user_notification.SettingsValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	ids := ca.Allocations.Ids
	if ids == nil {
		ids = []string{}
	}
	idList, d := types.ListValueFrom(ctx, types.StringType, ids)
	diags.Append(d...)
	allocations, d := datasource_user_notification.NewAllocationsValue(
		datasource_user_notification.AllocationsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"ids":  idList,
			"mode": types.StringValue(string(ca.Allocations.Mode)),
		},
	)
	diags.Append(d...)

	minExcessCost, d := datasource_user_notification.NewMinExcessCostValue(
		datasource_user_notification.MinExcessCostValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"amount":   types.StringValue(ca.MinExcessCost.Amount),
			"currency": types.StringValue(ca.MinExcessCost.Currency),
		},
	)
	diags.Append(d...)

	repeat, d := buildUserNotificationDataSourceRepeat(ctx, ca.Repeat)
	diags.Append(d...)

	costAnomalies, d := datasource_user_notification.NewCostAnomaliesValue(
		datasource_user_notification.CostAnomaliesValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"allocations":     allocations,
			"min_excess_cost": minExcessCost,
			"min_severity":    types.StringValue(string(ca.MinSeverity)),
			"repeat":          repeat,
		},
	)
	diags.Append(d...)

	settings, d := datasource_user_notification.NewSettingsValue(
		datasource_user_notification.SettingsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"cost_anomalies": costAnomalies},
	)
	diags.Append(d...)

	return settings, diags
}

func buildUserNotificationDataSourceRepeat(ctx context.Context, repeat *models.CostAnomalyRepeat) (datasource_user_notification.RepeatValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	// The API omits repeat when no resend control is set.
	if repeat == nil {
		return datasource_user_notification.NewRepeatValueNull(), diags
	}

	whenCostGrows := datasource_user_notification.NewWhenCostGrowsValueNull()
	if wcg := repeat.WhenCostGrows; wcg != nil {
		minIncrease := datasource_user_notification.NewMinIncreaseValueNull()
		if wcg.MinIncrease != nil {
			var d diag.Diagnostics
			minIncrease, d = datasource_user_notification.NewMinIncreaseValue(
				datasource_user_notification.MinIncreaseValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"amount":   types.StringValue(wcg.MinIncrease.Amount),
					"currency": types.StringValue(wcg.MinIncrease.Currency),
				},
			)
			diags.Append(d...)
		}

		var d diag.Diagnostics
		whenCostGrows, d = datasource_user_notification.NewWhenCostGrowsValue(
			datasource_user_notification.WhenCostGrowsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"match":            userNotificationMatch(wcg.Match),
				"min_increase":     minIncrease,
				"percent_increase": intPointerToInt64(wcg.PercentIncrease),
				"within_hours":     intPointerToInt64(wcg.WithinHours),
			},
		)
		diags.Append(d...)
	}

	v, d := datasource_user_notification.NewRepeatValue(
		datasource_user_notification.RepeatValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"every_hours":     intPointerToInt64(repeat.EveryHours),
			"when_cost_grows": whenCostGrows,
		},
	)
	diags.Append(d...)

	return v, diags
}

// mapUserNotificationsToDataSource maps the API notifications to data.Items.
// The result is an empty list, never null, when the user has no notifications.
func mapUserNotificationsToDataSource(ctx context.Context, items []models.UserNotification, data *userNotificationsDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	values := make([]datasource_user_notifications.ItemsValue, 0, len(items))
	for i := range items {
		item := &items[i]

		destinations, d := buildUserNotificationsDataSourceDestinations(ctx, item.Destinations)
		diags.Append(d...)
		settings, d := buildUserNotificationsDataSourceSettings(ctx, &item.Settings.CostAnomalies)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}

		value, d := datasource_user_notifications.NewItemsValue(
			datasource_user_notifications.ItemsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"id":           types.StringPointerValue(nullableToPointer(item.Id)),
				"name":         types.StringValue(item.Name),
				"create_time":  nullableTimeToString(item.CreateTime),
				"update_time":  nullableTimeToString(item.UpdateTime),
				"etag":         types.StringPointerValue(nullableToPointer(item.Etag)),
				"destinations": destinations,
				"settings":     settings,
			},
		)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}
		values = append(values, value)
	}

	list, d := types.ListValueFrom(ctx, datasource_user_notifications.ItemsValue{}.Type(ctx), values)
	diags.Append(d...)
	data.Items = list
	return diags
}

func buildUserNotificationsDataSourceDestinations(ctx context.Context, destinations *[]models.UserNotificationEmailDestination) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics

	attrTypes := datasource_user_notifications.DestinationsValue{}.AttributeTypes(ctx)
	items := []datasource_user_notifications.DestinationsValue{}
	if destinations != nil {
		for _, dest := range *destinations {
			v, d := datasource_user_notifications.NewDestinationsValue(attrTypes, map[string]attr.Value{
				"address": userNotificationDestinationAddress(dest),
				"type":    types.StringValue(string(dest.Type)),
			})
			diags.Append(d...)
			items = append(items, v)
		}
	}

	list, d := types.ListValueFrom(ctx, datasource_user_notifications.DestinationsValue{}.Type(ctx), items)
	diags.Append(d...)
	return list, diags
}

func buildUserNotificationsDataSourceSettings(ctx context.Context, ca *models.CostAnomalySettings) (datasource_user_notifications.SettingsValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	ids := ca.Allocations.Ids
	if ids == nil {
		ids = []string{}
	}
	idList, d := types.ListValueFrom(ctx, types.StringType, ids)
	diags.Append(d...)
	allocations, d := datasource_user_notifications.NewAllocationsValue(
		datasource_user_notifications.AllocationsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"ids":  idList,
			"mode": types.StringValue(string(ca.Allocations.Mode)),
		},
	)
	diags.Append(d...)

	minExcessCost, d := datasource_user_notifications.NewMinExcessCostValue(
		datasource_user_notifications.MinExcessCostValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"amount":   types.StringValue(ca.MinExcessCost.Amount),
			"currency": types.StringValue(ca.MinExcessCost.Currency),
		},
	)
	diags.Append(d...)

	repeat, d := buildUserNotificationsDataSourceRepeat(ctx, ca.Repeat)
	diags.Append(d...)

	costAnomalies, d := datasource_user_notifications.NewCostAnomaliesValue(
		datasource_user_notifications.CostAnomaliesValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"allocations":     allocations,
			"min_excess_cost": minExcessCost,
			"min_severity":    types.StringValue(string(ca.MinSeverity)),
			"repeat":          repeat,
		},
	)
	diags.Append(d...)

	settings, d := datasource_user_notifications.NewSettingsValue(
		datasource_user_notifications.SettingsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"cost_anomalies": costAnomalies},
	)
	diags.Append(d...)

	return settings, diags
}

func buildUserNotificationsDataSourceRepeat(ctx context.Context, repeat *models.CostAnomalyRepeat) (datasource_user_notifications.RepeatValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	// The API omits repeat when no resend control is set.
	if repeat == nil {
		return datasource_user_notifications.NewRepeatValueNull(), diags
	}

	whenCostGrows := datasource_user_notifications.NewWhenCostGrowsValueNull()
	if wcg := repeat.WhenCostGrows; wcg != nil {
		minIncrease := datasource_user_notifications.NewMinIncreaseValueNull()
		if wcg.MinIncrease != nil {
			var d diag.Diagnostics
			minIncrease, d = datasource_user_notifications.NewMinIncreaseValue(
				datasource_user_notifications.MinIncreaseValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"amount":   types.StringValue(wcg.MinIncrease.Amount),
					"currency": types.StringValue(wcg.MinIncrease.Currency),
				},
			)
			diags.Append(d...)
		}

		var d diag.Diagnostics
		whenCostGrows, d = datasource_user_notifications.NewWhenCostGrowsValue(
			datasource_user_notifications.WhenCostGrowsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"match":            userNotificationMatch(wcg.Match),
				"min_increase":     minIncrease,
				"percent_increase": intPointerToInt64(wcg.PercentIncrease),
				"within_hours":     intPointerToInt64(wcg.WithinHours),
			},
		)
		diags.Append(d...)
	}

	v, d := datasource_user_notifications.NewRepeatValue(
		datasource_user_notifications.RepeatValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"every_hours":     intPointerToInt64(repeat.EveryHours),
			"when_cost_grows": whenCostGrows,
		},
	)
	diags.Append(d...)

	return v, diags
}

func userNotificationDestinationAddress(dest models.UserNotificationEmailDestination) types.String {
	if dest.Address == nil {
		return types.StringNull()
	}
	return types.StringValue(string(*dest.Address))
}

func userNotificationMatch(match *models.CostAnomalyWhenCostGrowsMatch) types.String {
	if match == nil {
		return types.StringNull()
	}
	return types.StringValue(string(*match))
}
