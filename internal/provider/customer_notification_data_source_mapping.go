package provider

import (
	"context"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_customer_notification"
	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_customer_notifications"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// The singular and plural data sources generate separate nested types, so each
// has its own builders. Both map the API values as returned: unlike the
// resource there is no configuration to keep, so emails keep the API's
// lowercase spelling and amounts its two-decimal spelling ("100.00").

// mapCustomerNotificationToDataSource maps the API response to the singular
// data source model.
func mapCustomerNotificationToDataSource(ctx context.Context, resp *models.CustomerNotification, data *customerNotificationDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	data.Id = types.StringPointerValue(nullableToPointer(resp.Id))
	data.Name = types.StringValue(resp.Name)
	data.CreateTime = nullableTimeToString(resp.CreateTime)
	data.UpdateTime = nullableTimeToString(resp.UpdateTime)
	data.Etag = types.StringPointerValue(nullableToPointer(resp.Etag))

	destinations, d := buildCustomerNotificationDataSourceDestinations(ctx, resp.Destinations)
	diags.Append(d...)
	data.Destinations = destinations

	settings, d := buildCustomerNotificationDataSourceSettings(ctx, &resp.Settings.CostAnomalies)
	diags.Append(d...)
	data.Settings = settings

	return diags
}

func buildCustomerNotificationDataSourceDestinations(ctx context.Context, destinations models.CustomerNotificationDestinations) (datasource_customer_notification.DestinationsValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	emails := make([]string, len(destinations.Emails))
	for i, email := range destinations.Emails {
		emails[i] = string(email)
	}
	emailList, d := types.ListValueFrom(ctx, types.StringType, emails)
	diags.Append(d...)
	slackList, d := types.ListValueFrom(ctx, types.StringType, nonNilStrings(destinations.SlackChannels))
	diags.Append(d...)
	teamsList, d := types.ListValueFrom(ctx, types.StringType, nonNilStrings(destinations.TeamsChannels))
	diags.Append(d...)

	v, d := datasource_customer_notification.NewDestinationsValue(
		datasource_customer_notification.DestinationsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"emails":         emailList,
			"slack_channels": slackList,
			"teams_channels": teamsList,
		},
	)
	diags.Append(d...)
	return v, diags
}

func buildCustomerNotificationDataSourceSettings(ctx context.Context, ca *models.CostAnomalySettings) (datasource_customer_notification.SettingsValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	idList, d := types.ListValueFrom(ctx, types.StringType, nonNilStrings(ca.Allocations.Ids))
	diags.Append(d...)
	allocations, d := datasource_customer_notification.NewAllocationsValue(
		datasource_customer_notification.AllocationsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"ids":  idList,
			"mode": types.StringValue(string(ca.Allocations.Mode)),
		},
	)
	diags.Append(d...)

	minExcessCost, d := datasource_customer_notification.NewMinExcessCostValue(
		datasource_customer_notification.MinExcessCostValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"amount":   types.StringValue(ca.MinExcessCost.Amount),
			"currency": types.StringValue(ca.MinExcessCost.Currency),
		},
	)
	diags.Append(d...)

	repeat, d := buildCustomerNotificationDataSourceRepeat(ctx, ca.Repeat)
	diags.Append(d...)

	costAnomalies, d := datasource_customer_notification.NewCostAnomaliesValue(
		datasource_customer_notification.CostAnomaliesValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"allocations":     allocations,
			"min_excess_cost": minExcessCost,
			"min_severity":    types.StringValue(string(ca.MinSeverity)),
			"repeat":          repeat,
		},
	)
	diags.Append(d...)

	settings, d := datasource_customer_notification.NewSettingsValue(
		datasource_customer_notification.SettingsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"cost_anomalies": costAnomalies},
	)
	diags.Append(d...)

	return settings, diags
}

func buildCustomerNotificationDataSourceRepeat(ctx context.Context, repeat *models.CostAnomalyRepeat) (datasource_customer_notification.RepeatValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	// The API omits repeat when no resend control is set.
	if repeat == nil {
		return datasource_customer_notification.NewRepeatValueNull(), diags
	}

	whenCostGrows := datasource_customer_notification.NewWhenCostGrowsValueNull()
	if wcg := repeat.WhenCostGrows; wcg != nil {
		minIncrease := datasource_customer_notification.NewMinIncreaseValueNull()
		if wcg.MinIncrease != nil {
			var d diag.Diagnostics
			minIncrease, d = datasource_customer_notification.NewMinIncreaseValue(
				datasource_customer_notification.MinIncreaseValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"amount":   types.StringValue(wcg.MinIncrease.Amount),
					"currency": types.StringValue(wcg.MinIncrease.Currency),
				},
			)
			diags.Append(d...)
		}

		var d diag.Diagnostics
		whenCostGrows, d = datasource_customer_notification.NewWhenCostGrowsValue(
			datasource_customer_notification.WhenCostGrowsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"match":            userNotificationMatch(wcg.Match),
				"min_increase":     minIncrease,
				"percent_increase": intPointerToInt64(wcg.PercentIncrease),
				"within_hours":     intPointerToInt64(wcg.WithinHours),
			},
		)
		diags.Append(d...)
	}

	v, d := datasource_customer_notification.NewRepeatValue(
		datasource_customer_notification.RepeatValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"every_hours":     intPointerToInt64(repeat.EveryHours),
			"when_cost_grows": whenCostGrows,
		},
	)
	diags.Append(d...)

	return v, diags
}

// mapCustomerNotificationsToDataSource maps the API notifications to data.Items.
// The result is an empty list, never null, when the tenant has no notifications.
func mapCustomerNotificationsToDataSource(ctx context.Context, items []models.CustomerNotification, data *customerNotificationsDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	values := make([]datasource_customer_notifications.ItemsValue, 0, len(items))
	for i := range items {
		item := &items[i]

		destinations, d := buildCustomerNotificationsDataSourceDestinations(ctx, item.Destinations)
		diags.Append(d...)
		settings, d := buildCustomerNotificationsDataSourceSettings(ctx, &item.Settings.CostAnomalies)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}

		value, d := datasource_customer_notifications.NewItemsValue(
			datasource_customer_notifications.ItemsValue{}.AttributeTypes(ctx),
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

	list, d := types.ListValueFrom(ctx, datasource_customer_notifications.ItemsValue{}.Type(ctx), values)
	diags.Append(d...)
	data.Items = list
	return diags
}

func buildCustomerNotificationsDataSourceDestinations(ctx context.Context, destinations models.CustomerNotificationDestinations) (datasource_customer_notifications.DestinationsValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	emails := make([]string, len(destinations.Emails))
	for i, email := range destinations.Emails {
		emails[i] = string(email)
	}
	emailList, d := types.ListValueFrom(ctx, types.StringType, emails)
	diags.Append(d...)
	slackList, d := types.ListValueFrom(ctx, types.StringType, nonNilStrings(destinations.SlackChannels))
	diags.Append(d...)
	teamsList, d := types.ListValueFrom(ctx, types.StringType, nonNilStrings(destinations.TeamsChannels))
	diags.Append(d...)

	v, d := datasource_customer_notifications.NewDestinationsValue(
		datasource_customer_notifications.DestinationsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"emails":         emailList,
			"slack_channels": slackList,
			"teams_channels": teamsList,
		},
	)
	diags.Append(d...)
	return v, diags
}

func buildCustomerNotificationsDataSourceSettings(ctx context.Context, ca *models.CostAnomalySettings) (datasource_customer_notifications.SettingsValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	idList, d := types.ListValueFrom(ctx, types.StringType, nonNilStrings(ca.Allocations.Ids))
	diags.Append(d...)
	allocations, d := datasource_customer_notifications.NewAllocationsValue(
		datasource_customer_notifications.AllocationsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"ids":  idList,
			"mode": types.StringValue(string(ca.Allocations.Mode)),
		},
	)
	diags.Append(d...)

	minExcessCost, d := datasource_customer_notifications.NewMinExcessCostValue(
		datasource_customer_notifications.MinExcessCostValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"amount":   types.StringValue(ca.MinExcessCost.Amount),
			"currency": types.StringValue(ca.MinExcessCost.Currency),
		},
	)
	diags.Append(d...)

	repeat, d := buildCustomerNotificationsDataSourceRepeat(ctx, ca.Repeat)
	diags.Append(d...)

	costAnomalies, d := datasource_customer_notifications.NewCostAnomaliesValue(
		datasource_customer_notifications.CostAnomaliesValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"allocations":     allocations,
			"min_excess_cost": minExcessCost,
			"min_severity":    types.StringValue(string(ca.MinSeverity)),
			"repeat":          repeat,
		},
	)
	diags.Append(d...)

	settings, d := datasource_customer_notifications.NewSettingsValue(
		datasource_customer_notifications.SettingsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"cost_anomalies": costAnomalies},
	)
	diags.Append(d...)

	return settings, diags
}

func buildCustomerNotificationsDataSourceRepeat(ctx context.Context, repeat *models.CostAnomalyRepeat) (datasource_customer_notifications.RepeatValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	// The API omits repeat when no resend control is set.
	if repeat == nil {
		return datasource_customer_notifications.NewRepeatValueNull(), diags
	}

	whenCostGrows := datasource_customer_notifications.NewWhenCostGrowsValueNull()
	if wcg := repeat.WhenCostGrows; wcg != nil {
		minIncrease := datasource_customer_notifications.NewMinIncreaseValueNull()
		if wcg.MinIncrease != nil {
			var d diag.Diagnostics
			minIncrease, d = datasource_customer_notifications.NewMinIncreaseValue(
				datasource_customer_notifications.MinIncreaseValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"amount":   types.StringValue(wcg.MinIncrease.Amount),
					"currency": types.StringValue(wcg.MinIncrease.Currency),
				},
			)
			diags.Append(d...)
		}

		var d diag.Diagnostics
		whenCostGrows, d = datasource_customer_notifications.NewWhenCostGrowsValue(
			datasource_customer_notifications.WhenCostGrowsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"match":            userNotificationMatch(wcg.Match),
				"min_increase":     minIncrease,
				"percent_increase": intPointerToInt64(wcg.PercentIncrease),
				"within_hours":     intPointerToInt64(wcg.WithinHours),
			},
		)
		diags.Append(d...)
	}

	v, d := datasource_customer_notifications.NewRepeatValue(
		datasource_customer_notifications.RepeatValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"every_hours":     intPointerToInt64(repeat.EveryHours),
			"when_cost_grows": whenCostGrows,
		},
	)
	diags.Append(d...)

	return v, diags
}
