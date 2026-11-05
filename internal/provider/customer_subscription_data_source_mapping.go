package provider

import (
	"context"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_customer_subscription"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// The singular and plural data sources generate separate nested types, so each
// has its own builders (customer_subscriptions_data_source_mapping.go holds
// the plural ones, with the same structure). Both map the API values as
// returned: unlike the resource there is no configuration to keep, so emails
// keep the API's lowercase spelling and timestamps its UTC spelling.
//
// The API also returns alerts and budgets, which are not scheduled
// subscriptions. They have no schedule, attachments or PDF settings.

// customerSubscriptionAPITimestamp formats a timestamp the way the API returns
// it, keeping fractional seconds.
func customerSubscriptionAPITimestamp(t time.Time) types.String {
	return types.StringValue(t.UTC().Format(time.RFC3339Nano))
}

// customerSubscriptionAttachmentNames converts the attachments of an entry.
// The API only returns them for report subscriptions; a missing list is [] so
// that consumers can iterate it.
func customerSubscriptionAttachmentNames(attachments *[]models.CustomerSubscriptionAttachment) []string {
	names := []string{}
	if attachments != nil {
		for _, a := range *attachments {
			names = append(names, string(a))
		}
	}
	return names
}

// mapCustomerSubscriptionToDataSource maps the API response to the singular
// data source model.
func mapCustomerSubscriptionToDataSource(ctx context.Context, resp *models.CustomerSubscription, data *customerSubscriptionDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	data.Id = types.StringPointerValue(nullableToPointer(resp.Id))
	data.Name = types.StringPointerValue(resp.Name)
	data.Etag = types.StringPointerValue(nullableToPointer(resp.Etag))
	data.CreatedBy = types.StringNull()
	if resp.CreatedBy != nil {
		data.CreatedBy = types.StringValue(string(*resp.CreatedBy))
	}

	recipients, d := buildCustomerSubscriptionDataSourceRecipients(ctx, resp.Recipients)
	diags.Append(d...)
	data.Recipients = recipients

	entries, d := buildCustomerSubscriptionDataSourceEntries(ctx, resp.Subscriptions)
	diags.Append(d...)
	data.Subscriptions = entries

	return diags
}

func buildCustomerSubscriptionDataSourceRecipients(ctx context.Context, recipients models.CustomerSubscriptionRecipients) (datasource_customer_subscription.RecipientsValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	emails := make([]string, len(recipients.Emails))
	for i, email := range recipients.Emails {
		emails[i] = string(email)
	}
	emailList, d := types.ListValueFrom(ctx, types.StringType, emails)
	diags.Append(d...)
	slackList, d := types.ListValueFrom(ctx, types.StringType, nonNilStrings(recipients.SlackChannels))
	diags.Append(d...)

	v, d := datasource_customer_subscription.NewRecipientsValue(
		datasource_customer_subscription.RecipientsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"emails":         emailList,
			"slack_channels": slackList,
		},
	)
	diags.Append(d...)
	return v, diags
}

func buildCustomerSubscriptionDataSourceEntries(ctx context.Context, entries []models.CustomerSubscriptionEntry) (types.Set, diag.Diagnostics) {
	var diags diag.Diagnostics

	values := make([]datasource_customer_subscription.SubscriptionsValue, 0, len(entries))
	for _, entry := range entries {
		v, d := buildCustomerSubscriptionDataSourceEntry(ctx, entry)
		diags.Append(d...)
		values = append(values, v)
	}

	set, d := types.SetValueFrom(ctx, datasource_customer_subscription.SubscriptionsValue{}.Type(ctx), values)
	diags.Append(d...)
	return set, diags
}

func buildCustomerSubscriptionDataSourceEntry(ctx context.Context, entry models.CustomerSubscriptionEntry) (datasource_customer_subscription.SubscriptionsValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	source, d := datasource_customer_subscription.NewSourceValue(
		datasource_customer_subscription.SourceValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"id":   types.StringValue(entry.Source.Id),
			"name": types.StringPointerValue(entry.Source.Name),
		},
	)
	diags.Append(d...)

	// The API omits the schedule only for alerts and budgets.
	schedule := datasource_customer_subscription.NewScheduleValueNull()
	if s := entry.Schedule; s != nil {
		nextAt := types.StringNull()
		if s.NextAt != nil {
			nextAt = customerSubscriptionAPITimestamp(*s.NextAt)
		}
		customPeriodUnit := types.StringNull()
		if s.CustomPeriodUnit != nil {
			customPeriodUnit = types.StringValue(string(*s.CustomPeriodUnit))
		}
		schedule, d = datasource_customer_subscription.NewScheduleValue(
			datasource_customer_subscription.ScheduleValue{}.AttributeTypes(ctx),
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

	pdfSettings := datasource_customer_subscription.NewPdfSettingsValueNull()
	if p := entry.PdfSettings; p != nil {
		pdfSettings, d = datasource_customer_subscription.NewPdfSettingsValue(
			datasource_customer_subscription.PdfSettingsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"include_annotations": types.BoolValue(p.IncludeAnnotations),
				"include_table":       types.BoolValue(p.IncludeTable),
				"show_filters":        types.BoolValue(p.ShowFilters),
			},
		)
		diags.Append(d...)
	}

	permissions := datasource_customer_subscription.NewPermissionsValueNull()
	if p := entry.Permissions; p != nil {
		permissions, d = datasource_customer_subscription.NewPermissionsValue(
			datasource_customer_subscription.PermissionsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"can_delete":          types.BoolValue(p.CanDelete),
				"can_edit_recipients": types.BoolValue(p.CanEditRecipients),
				"can_unsubscribe":     types.BoolValue(p.CanUnsubscribe),
			},
		)
		diags.Append(d...)
	}

	v, d := datasource_customer_subscription.NewSubscriptionsValue(
		datasource_customer_subscription.SubscriptionsValue{}.AttributeTypes(ctx),
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
