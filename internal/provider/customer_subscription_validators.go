package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_customer_subscription"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	customerSubscriptionRecipientsPath = path.Root("recipients")
	customerSubscriptionEntriesPath    = path.Root("subscriptions")
)

// customerSubscriptionRecipientsValidator checks the recipient rules the API
// otherwise rejects at apply: at least one recipient, and no duplicate within
// a list (emails compare case-insensitively).
type customerSubscriptionRecipientsValidator struct{}

var _ resource.ConfigValidator = customerSubscriptionRecipientsValidator{}

func (v customerSubscriptionRecipientsValidator) Description(_ context.Context) string {
	return "recipients must have at least one email or Slack channel, without duplicates inside a list"
}

func (v customerSubscriptionRecipientsValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v customerSubscriptionRecipientsValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var recipients resource_customer_subscription.RecipientsValue
	d := req.Config.GetAttribute(ctx, customerSubscriptionRecipientsPath, &recipients)
	resp.Diagnostics.Append(d...)
	if d.HasError() {
		return
	}
	validateCustomerSubscriptionRecipients(recipients, &resp.Diagnostics)
}

func validateCustomerSubscriptionRecipients(recipients resource_customer_subscription.RecipientsValue, diags *diag.Diagnostics) {
	if recipients.IsNull() || recipients.IsUnknown() {
		return
	}

	validateDestinationLists(customerSubscriptionRecipientsPath, []destinationList{
		{"emails", recipients.Emails, false},
		{"slack_channels", recipients.SlackChannels, true},
	}, "At least one of `emails` or `slack_channels` must contain an entry.", diags)
}

// customerSubscriptionEntriesValidator checks the subscriptions rules the API
// otherwise rejects with 422 at apply: each type at most once, report options
// only on report subscriptions, and custom periods only (and always) with a
// Custom frequency.
type customerSubscriptionEntriesValidator struct{}

var _ resource.ConfigValidator = customerSubscriptionEntriesValidator{}

func (v customerSubscriptionEntriesValidator) Description(_ context.Context) string {
	return "each subscriptions type may appear once; attachments and pdf_settings are only allowed for report-subscription; " +
		"custom_period_amount and custom_period_unit are required when frequency is Custom and not allowed otherwise"
}

func (v customerSubscriptionEntriesValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v customerSubscriptionEntriesValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var entries types.Set
	d := req.Config.GetAttribute(ctx, customerSubscriptionEntriesPath, &entries)
	resp.Diagnostics.Append(d...)
	if d.HasError() {
		return
	}
	validateCustomerSubscriptionEntries(entries, &resp.Diagnostics)
}

func validateCustomerSubscriptionEntries(entries types.Set, diags *diag.Diagnostics) {
	if entries.IsNull() || entries.IsUnknown() {
		return
	}

	seenTypes := map[string]bool{}
	for _, element := range entries.Elements() {
		entry, ok := element.(resource_customer_subscription.SubscriptionsValue)
		if !ok {
			continue
		}
		if entry.IsNull() || entry.IsUnknown() {
			continue
		}
		entryPath := customerSubscriptionEntriesPath.AtSetValue(entry)
		validateCustomerSubscriptionEntryType(entry, seenTypes, diags)
		validateCustomerSubscriptionReportOptions(entry, entryPath, diags)
		validateCustomerSubscriptionCustomPeriod(entry.Schedule, entryPath.AtName("schedule"), diags)
	}
}

// validateCustomerSubscriptionEntryType rejects a type that an earlier entry
// already uses. The set only removes identical entries, so two entries of one
// type with different schedules would otherwise reach the API.
func validateCustomerSubscriptionEntryType(entry resource_customer_subscription.SubscriptionsValue, seenTypes map[string]bool, diags *diag.Diagnostics) {
	if entry.SubscriptionsType.IsNull() || entry.SubscriptionsType.IsUnknown() {
		return
	}

	entryType := entry.SubscriptionsType.ValueString()
	if seenTypes[entryType] {
		diags.AddAttributeError(
			customerSubscriptionEntriesPath,
			"Duplicate Subscription Type",
			fmt.Sprintf("Type %q is used by more than one entry in `subscriptions`. Each type may appear once.", entryType),
		)
	}
	seenTypes[entryType] = true
}

// validateCustomerSubscriptionReportOptions rejects attachments and
// pdf_settings on any type other than report-subscription.
func validateCustomerSubscriptionReportOptions(entry resource_customer_subscription.SubscriptionsValue, entryPath path.Path, diags *diag.Diagnostics) {
	if entry.SubscriptionsType.IsNull() || entry.SubscriptionsType.IsUnknown() {
		return
	}
	if entry.SubscriptionsType.ValueString() == string(models.CustomerSubscriptionTypeReportSubscription) {
		return
	}

	if !entry.Attachments.IsUnknown() && !entry.Attachments.IsNull() {
		diags.AddAttributeError(
			entryPath.AtName("attachments"),
			"Attachments Not Allowed",
			fmt.Sprintf("`attachments` is only allowed for `report-subscription`, not for %q.", entry.SubscriptionsType.ValueString()),
		)
	}
	if !entry.PdfSettings.IsUnknown() && !entry.PdfSettings.IsNull() {
		diags.AddAttributeError(
			entryPath.AtName("pdf_settings"),
			"PDF Settings Not Allowed",
			fmt.Sprintf("`pdf_settings` is only allowed for `report-subscription`, not for %q.", entry.SubscriptionsType.ValueString()),
		)
	}
}

// validateCustomerSubscriptionCustomPeriod requires custom_period_amount and
// custom_period_unit for a Custom frequency and rejects them otherwise.
func validateCustomerSubscriptionCustomPeriod(schedule resource_customer_subscription.ScheduleValue, schedulePath path.Path, diags *diag.Diagnostics) {
	if schedule.IsNull() || schedule.IsUnknown() {
		return
	}
	if schedule.Frequency.IsNull() || schedule.Frequency.IsUnknown() {
		return
	}

	if schedule.Frequency.ValueString() == string(models.CustomerSubscriptionFrequencyCustom) {
		if schedule.CustomPeriodAmount.IsNull() {
			diags.AddAttributeError(
				schedulePath.AtName("custom_period_amount"),
				"Missing Custom Period",
				"`custom_period_amount` is required when `frequency` is `Custom`.",
			)
		}
		if schedule.CustomPeriodUnit.IsNull() {
			diags.AddAttributeError(
				schedulePath.AtName("custom_period_unit"),
				"Missing Custom Period",
				"`custom_period_unit` is required when `frequency` is `Custom`.",
			)
		}
		return
	}

	if !schedule.CustomPeriodAmount.IsUnknown() && !schedule.CustomPeriodAmount.IsNull() {
		diags.AddAttributeError(
			schedulePath.AtName("custom_period_amount"),
			"Custom Period Not Allowed",
			fmt.Sprintf("`custom_period_amount` is only allowed when `frequency` is `Custom`, not %q.", schedule.Frequency.ValueString()),
		)
	}
	if !schedule.CustomPeriodUnit.IsUnknown() && !schedule.CustomPeriodUnit.IsNull() {
		diags.AddAttributeError(
			schedulePath.AtName("custom_period_unit"),
			"Custom Period Not Allowed",
			fmt.Sprintf("`custom_period_unit` is only allowed when `frequency` is `Custom`, not %q.", schedule.Frequency.ValueString()),
		)
	}
}
