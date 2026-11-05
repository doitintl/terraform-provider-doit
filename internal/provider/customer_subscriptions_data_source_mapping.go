package provider

import (
	"context"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_customer_subscriptions"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// mapCustomerSubscriptionsToDataSource maps one page of the list response to
// the items attribute. items is [] rather than null when there are none.
func mapCustomerSubscriptionsToDataSource(ctx context.Context, items []models.CustomerSubscription, data *customerSubscriptionsDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	values := make([]datasource_customer_subscriptions.ItemsValue, 0, len(items))
	for i := range items {
		item := &items[i]

		recipients, d := buildCustomerSubscriptionsDataSourceRecipients(ctx, item.Recipients)
		diags.Append(d...)
		entries, d := buildCustomerSubscriptionsDataSourceEntries(ctx, item.Subscriptions)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}

		createdBy := types.StringNull()
		if item.CreatedBy != nil {
			createdBy = types.StringValue(string(*item.CreatedBy))
		}

		value, d := datasource_customer_subscriptions.NewItemsValue(
			datasource_customer_subscriptions.ItemsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"id":            types.StringPointerValue(nullableToPointer(item.Id)),
				"name":          types.StringPointerValue(item.Name),
				"etag":          types.StringPointerValue(nullableToPointer(item.Etag)),
				"created_by":    createdBy,
				"recipients":    recipients,
				"subscriptions": entries,
			},
		)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}
		values = append(values, value)
	}

	list, d := types.ListValueFrom(ctx, datasource_customer_subscriptions.ItemsValue{}.Type(ctx), values)
	diags.Append(d...)
	data.Items = list
	return diags
}

func buildCustomerSubscriptionsDataSourceRecipients(ctx context.Context, recipients models.CustomerSubscriptionRecipients) (datasource_customer_subscriptions.RecipientsValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	emails := make([]string, len(recipients.Emails))
	for i, email := range recipients.Emails {
		emails[i] = string(email)
	}
	emailList, d := types.ListValueFrom(ctx, types.StringType, emails)
	diags.Append(d...)
	slackList, d := types.ListValueFrom(ctx, types.StringType, nonNilStrings(recipients.SlackChannels))
	diags.Append(d...)

	v, d := datasource_customer_subscriptions.NewRecipientsValue(
		datasource_customer_subscriptions.RecipientsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"emails":         emailList,
			"slack_channels": slackList,
		},
	)
	diags.Append(d...)
	return v, diags
}

func buildCustomerSubscriptionsDataSourceEntries(ctx context.Context, entries []models.CustomerSubscriptionEntry) (types.Set, diag.Diagnostics) {
	var diags diag.Diagnostics

	values := make([]datasource_customer_subscriptions.SubscriptionsValue, 0, len(entries))
	for _, entry := range entries {
		v, d := buildCustomerSubscriptionsDataSourceEntry(ctx, entry)
		diags.Append(d...)
		values = append(values, v)
	}

	set, d := types.SetValueFrom(ctx, datasource_customer_subscriptions.SubscriptionsValue{}.Type(ctx), values)
	diags.Append(d...)
	return set, diags
}

func buildCustomerSubscriptionsDataSourceEntry(ctx context.Context, entry models.CustomerSubscriptionEntry) (datasource_customer_subscriptions.SubscriptionsValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	source, d := datasource_customer_subscriptions.NewSourceValue(
		datasource_customer_subscriptions.SourceValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"id":   types.StringValue(entry.Source.Id),
			"name": types.StringPointerValue(entry.Source.Name),
		},
	)
	diags.Append(d...)

	// The API omits the schedule only for alerts and budgets.
	schedule := datasource_customer_subscriptions.NewScheduleValueNull()
	if s := entry.Schedule; s != nil {
		nextAt := types.StringNull()
		if s.NextAt != nil {
			nextAt = customerSubscriptionAPITimestamp(*s.NextAt)
		}
		customPeriodUnit := types.StringNull()
		if s.CustomPeriodUnit != nil {
			customPeriodUnit = types.StringValue(string(*s.CustomPeriodUnit))
		}
		schedule, d = datasource_customer_subscriptions.NewScheduleValue(
			datasource_customer_subscriptions.ScheduleValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"custom_period_amount": intPointerToInt64(s.CustomPeriodAmount),
				"custom_period_unit":   customPeriodUnit,
				"frequency":            types.StringValue(string(s.Frequency)),
				"next_at":              nextAt,
				"start_date":           customerSubscriptionAPITimestamp(s.StartDate),
				"time":                 customerSubscriptionAPITimestamp(s.Time),
				"time_zone":            types.StringValue(s.TimeZone),
			},
		)
		diags.Append(d...)
	}

	attachments, d := types.ListValueFrom(ctx, types.StringType, customerSubscriptionAttachmentNames(entry.Attachments))
	diags.Append(d...)

	pdfSettings := datasource_customer_subscriptions.NewPdfSettingsValueNull()
	if p := entry.PdfSettings; p != nil {
		pdfSettings, d = datasource_customer_subscriptions.NewPdfSettingsValue(
			datasource_customer_subscriptions.PdfSettingsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"include_annotations": types.BoolValue(p.IncludeAnnotations),
				"include_table":       types.BoolValue(p.IncludeTable),
				"show_filters":        types.BoolValue(p.ShowFilters),
			},
		)
		diags.Append(d...)
	}

	permissions := datasource_customer_subscriptions.NewPermissionsValueNull()
	if p := entry.Permissions; p != nil {
		permissions, d = datasource_customer_subscriptions.NewPermissionsValue(
			datasource_customer_subscriptions.PermissionsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"can_delete":          types.BoolValue(p.CanDelete),
				"can_edit_recipients": types.BoolValue(p.CanEditRecipients),
				"can_unsubscribe":     types.BoolValue(p.CanUnsubscribe),
			},
		)
		diags.Append(d...)
	}

	v, d := datasource_customer_subscriptions.NewSubscriptionsValue(
		datasource_customer_subscriptions.SubscriptionsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"attachments":  attachments,
			"pdf_settings": pdfSettings,
			"permissions":  permissions,
			"schedule":     schedule,
			"source":       source,
			"type":         types.StringValue(string(entry.Type)),
		},
	)
	diags.Append(d...)
	return v, diags
}
