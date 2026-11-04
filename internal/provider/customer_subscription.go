package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_customer_subscription"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

type customerSubscriptionResourceModel struct {
	resource_customer_subscription.CustomerSubscriptionModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

var customerSubscriptionAttachmentValues = []string{
	string(models.CustomerSubscriptionAttachmentPdf),
	string(models.CustomerSubscriptionAttachmentCsv),
}

// populateState fetches the subscription from the API and populates the Terraform state.
// On 404, state.Id is set to null to signal Terraform to remove the resource from state.
func (r *customerSubscriptionResource) populateState(ctx context.Context, state *customerSubscriptionResourceModel) diag.Diagnostics {
	id := state.Id.ValueString()

	getResp, err := r.client.GetCustomerSubscriptionWithResponse(ctx, id)
	if err != nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Customer Subscription", fmt.Sprintf("Could not read subscription %s: %s", id, err)),
		}
	}

	if getResp.StatusCode() == 404 {
		state.Id = types.StringNull()
		return nil
	}

	if getResp.StatusCode() != 200 {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Customer Subscription", fmt.Sprintf("Unexpected status code %d for subscription %s: %s", getResp.StatusCode(), id, string(getResp.Body))),
		}
	}

	if getResp.JSON200 == nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Customer Subscription", fmt.Sprintf("Received empty response body for subscription %s", id)),
		}
	}

	// Alerts and budgets share the subscription ID space, but they are
	// managed by their own resources and cannot be deleted through this API.
	for _, entry := range getResp.JSON200.Subscriptions {
		if entry.Type == models.CustomerSubscriptionTypeAlert || entry.Type == models.CustomerSubscriptionTypeBudget {
			return diag.Diagnostics{
				diag.NewErrorDiagnostic(
					"Unsupported Subscription Type",
					fmt.Sprintf("Subscription %s is a %s. Manage it with the doit_%s resource instead of doit_customer_subscription.", id, entry.Type, entry.Type),
				),
			}
		}
	}

	return mapCustomerSubscriptionToModel(ctx, getResp.JSON200, state)
}

// mapCustomerSubscriptionToModel maps the API response to the Terraform model.
// The prior state in state decides how emails and timestamps are spelled.
func mapCustomerSubscriptionToModel(ctx context.Context, resp *models.CustomerSubscription, state *customerSubscriptionResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	state.Id = types.StringPointerValue(nullableToPointer(resp.Id))
	state.Name = types.StringPointerValue(resp.Name)
	state.Etag = types.StringPointerValue(nullableToPointer(resp.Etag))
	state.CreatedBy = types.StringNull()
	if resp.CreatedBy != nil {
		state.CreatedBy = types.StringValue(string(*resp.CreatedBy))
	}

	recipients, d := buildCustomerSubscriptionRecipients(ctx, resp.Recipients, state.Recipients)
	diags.Append(d...)
	state.Recipients = recipients

	entries, d := buildCustomerSubscriptionEntries(ctx, resp.Subscriptions, state.Subscriptions)
	diags.Append(d...)
	state.Subscriptions = entries

	return diags
}

// buildCustomerSubscriptionRecipients maps the recipients object. prior is
// the value currently in state: the API lowercases emails, so an email keeps the
// prior spelling when it only differs in case.
func buildCustomerSubscriptionRecipients(ctx context.Context, recipients models.CustomerSubscriptionRecipients, prior resource_customer_subscription.RecipientsValue) (resource_customer_subscription.RecipientsValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	priorEmails := []string{}
	if !prior.Emails.IsNull() && !prior.Emails.IsUnknown() {
		diags.Append(prior.Emails.ElementsAs(ctx, &priorEmails, false)...)
	}
	emails := make([]string, len(recipients.Emails))
	for i, email := range recipients.Emails {
		emails[i] = preserveCustomerNotificationEmail(priorEmails, i, string(email))
	}

	emailList, d := types.ListValueFrom(ctx, types.StringType, emails)
	diags.Append(d...)
	slackList, d := types.ListValueFrom(ctx, types.StringType, nonNilStrings(recipients.SlackChannels))
	diags.Append(d...)

	v, d := resource_customer_subscription.NewRecipientsValue(
		resource_customer_subscription.RecipientsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"emails":         emailList,
			"slack_channels": slackList,
		},
	)
	diags.Append(d...)

	return v, diags
}

// customerSubscriptionEntriesType is the element type of the subscriptions set.
func customerSubscriptionEntriesType(ctx context.Context) attr.Type {
	return resource_customer_subscription.SubscriptionsValue{}.Type(ctx)
}

// customerSubscriptionEntryKey identifies a set element: the API allows each
// type at most once, so it is the element's stable identity.
func customerSubscriptionEntryKey(entry resource_customer_subscription.SubscriptionsValue) string {
	return entry.SubscriptionsType.ValueString()
}

// buildCustomerSubscriptionEntries maps the subscriptions set. Each API entry is
// paired with the prior state element of the same type, which only decides how
// the schedule timestamps are spelled.
func buildCustomerSubscriptionEntries(ctx context.Context, entries []models.CustomerSubscriptionEntry, prior types.Set) (types.Set, diag.Diagnostics) {
	var diags diag.Diagnostics

	priorByType := map[string]resource_customer_subscription.SubscriptionsValue{}
	if !prior.IsNull() && !prior.IsUnknown() {
		var priorEntries []resource_customer_subscription.SubscriptionsValue
		diags.Append(prior.ElementsAs(ctx, &priorEntries, false)...)
		for _, entry := range priorEntries {
			if !entry.IsNull() && !entry.IsUnknown() {
				priorByType[customerSubscriptionEntryKey(entry)] = entry
			}
		}
	}

	values := make([]resource_customer_subscription.SubscriptionsValue, 0, len(entries))
	for _, entry := range entries {
		v, d := buildCustomerSubscriptionEntry(ctx, entry, priorByType[string(entry.Type)])
		diags.Append(d...)
		values = append(values, v)
	}

	set, d := types.SetValueFrom(ctx, customerSubscriptionEntriesType(ctx), values)
	diags.Append(d...)
	return set, diags
}

func buildCustomerSubscriptionEntry(ctx context.Context, entry models.CustomerSubscriptionEntry, prior resource_customer_subscription.SubscriptionsValue) (resource_customer_subscription.SubscriptionsValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	source, d := resource_customer_subscription.NewSourceValue(
		resource_customer_subscription.SourceValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"id":   types.StringValue(entry.Source.Id),
			"name": types.StringPointerValue(entry.Source.Name),
		},
	)
	diags.Append(d...)

	// A zero prior (no state element of this type) has null timestamps, so the
	// API spelling is used.
	schedule, d := buildCustomerSubscriptionSchedule(ctx, entry.Schedule, prior.Schedule)
	diags.Append(d...)

	// attachments is only returned for report subscriptions. It is a clearable
	// list, so a missing list is [] for every type: that is the value the
	// clearing modifier plans when the configuration omits it.
	values := []string{}
	if entry.Attachments != nil {
		for _, a := range *entry.Attachments {
			values = append(values, string(a))
		}
	}
	attachments, d := types.ListValueFrom(ctx, types.StringType, values)
	diags.Append(d...)

	// pdf_settings is only returned for report subscriptions.

	pdfSettings := resource_customer_subscription.NewPdfSettingsValueNull()
	if p := entry.PdfSettings; p != nil {
		pdfSettings, d = resource_customer_subscription.NewPdfSettingsValue(
			resource_customer_subscription.PdfSettingsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"include_annotations": types.BoolValue(p.IncludeAnnotations),
				"include_table":       types.BoolValue(p.IncludeTable),
				"show_filters":        types.BoolValue(p.ShowFilters),
			},
		)
		diags.Append(d...)
	}

	permissions := resource_customer_subscription.NewPermissionsValueNull()
	if p := entry.Permissions; p != nil {
		permissions, d = resource_customer_subscription.NewPermissionsValue(
			resource_customer_subscription.PermissionsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"can_delete":          types.BoolValue(p.CanDelete),
				"can_edit_recipients": types.BoolValue(p.CanEditRecipients),
				"can_unsubscribe":     types.BoolValue(p.CanUnsubscribe),
			},
		)
		diags.Append(d...)
	}

	v, d := resource_customer_subscription.NewSubscriptionsValue(
		resource_customer_subscription.SubscriptionsValue{}.AttributeTypes(ctx),
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

func buildCustomerSubscriptionSchedule(ctx context.Context, schedule *models.CustomerSubscriptionSchedule, prior resource_customer_subscription.ScheduleValue) (resource_customer_subscription.ScheduleValue, diag.Diagnostics) {
	// The API omits schedule only for alerts and budgets.
	if schedule == nil {
		return resource_customer_subscription.NewScheduleValueNull(), nil
	}

	nextAt := types.StringNull()
	if schedule.NextAt != nil {
		nextAt = types.StringValue(schedule.NextAt.UTC().Format(time.RFC3339))
	}

	customPeriodUnit := types.StringNull()
	if schedule.CustomPeriodUnit != nil {
		customPeriodUnit = types.StringValue(string(*schedule.CustomPeriodUnit))
	}

	return resource_customer_subscription.NewScheduleValue(
		resource_customer_subscription.ScheduleValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"custom_period_amount": intPointerToInt64(schedule.CustomPeriodAmount),
			"custom_period_unit":   customPeriodUnit,
			"frequency":            types.StringValue(string(schedule.Frequency)),
			"next_at":              nextAt,
			"start_date":           preserveCustomerSubscriptionTimestamp(prior.StartDate, schedule.StartDate),
			"time":                 preserveCustomerSubscriptionTimestamp(prior.Time, schedule.Time),
			"time_zone":            types.StringValue(schedule.TimeZone),
		},
	)
}

// preserveCustomerSubscriptionTimestamp returns the prior value when it is the
// same instant as the API's: the API drops the offset of a configured
// timestamp and returns it in UTC.
func preserveCustomerSubscriptionTimestamp(prior types.String, apiValue time.Time) types.String {
	if !prior.IsNull() && !prior.IsUnknown() {
		priorTime, err := time.Parse(time.RFC3339, prior.ValueString())
		if err == nil && priorTime.Equal(apiValue) {
			return prior
		}
	}
	return types.StringValue(apiValue.UTC().Format(time.RFC3339))
}

// overlayCustomerSubscriptionComputedFields applies the plan-first overlay for Create/Update.
func overlayCustomerSubscriptionComputedFields(ctx context.Context, apiResp *models.CustomerSubscription, plan *customerSubscriptionResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	// Phase 1: Build fully-resolved state from API response.
	resolved := *plan
	diags.Append(mapCustomerSubscriptionToModel(ctx, apiResp, &resolved)...)
	if diags.HasError() {
		return diags
	}

	// Phase 2: Overlay Computed-only fields unconditionally from resolved.
	plan.Id = resolved.Id
	plan.CreatedBy = resolved.CreatedBy

	// name and etag: Computed-only but change server-side on updates.
	if plan.Name.IsUnknown() {
		plan.Name = resolved.Name //nolint:overlaycheck // guarded to avoid inconsistent result on Update
	}
	if plan.Etag.IsUnknown() {
		plan.Etag = resolved.Etag //nolint:overlaycheck // guarded to avoid inconsistent result on Update
	}

	// recipients: Required with no Computed children — never touch.

	if plan.Subscriptions.IsUnknown() {
		plan.Subscriptions = resolved.Subscriptions
	} else if !plan.Subscriptions.IsNull() {
		diags.Append(overlaySetElements(ctx, &resolved.Subscriptions, &plan.Subscriptions, overlayCustomerSubscriptionEntry)...)
	}

	return diags
}

func overlayCustomerSubscriptionEntry(_ context.Context, resolved, plan *resource_customer_subscription.SubscriptionsValue) diag.Diagnostics {
	// attachments and pdf_settings: Optional+Computed. Unknown on create when
	// omitted; the API fills them in for report subscriptions.
	if plan.Attachments.IsUnknown() {
		plan.Attachments = resolved.Attachments
	}
	if plan.PdfSettings.IsUnknown() {
		plan.PdfSettings = resolved.PdfSettings
	}

	// permissions: Computed-only, evaluated per caller.
	if plan.Permissions.IsUnknown() {
		plan.Permissions = resolved.Permissions //nolint:overlaycheck // guarded to avoid inconsistent result on Update
	}

	if !plan.Source.IsNull() && !plan.Source.IsUnknown() {
		overlayCustomerSubscriptionSource(&resolved.Source, &plan.Source)
	}
	if !plan.Schedule.IsNull() && !plan.Schedule.IsUnknown() {
		overlayCustomerSubscriptionSchedule(&resolved.Schedule, &plan.Schedule)
	}

	// type: Required — never touch.
	return nil
}

func overlayCustomerSubscriptionSource(resolved, plan *resource_customer_subscription.SourceValue) {
	// name: Computed-only, read live from the source, so it follows renames.
	if plan.Name.IsUnknown() {
		plan.Name = resolved.Name //nolint:overlaycheck // guarded to avoid inconsistent result on Update
	}
	// id: Required — never touch.
}

func overlayCustomerSubscriptionSchedule(resolved, plan *resource_customer_subscription.ScheduleValue) {
	// next_at: Computed-only, moves after every delivery.
	if plan.NextAt.IsUnknown() {
		plan.NextAt = resolved.NextAt //nolint:overlaycheck // guarded to avoid inconsistent result on Update
	}
	if plan.CustomPeriodAmount.IsUnknown() {
		plan.CustomPeriodAmount = resolved.CustomPeriodAmount
	}
	if plan.CustomPeriodUnit.IsUnknown() {
		plan.CustomPeriodUnit = resolved.CustomPeriodUnit
	}
	// frequency, time, time_zone, start_date: Required — never touch.
}

// toCreateRequest converts the TF model to a CreateCustomerSubscriptionRequest.
func (plan *customerSubscriptionResourceModel) toCreateRequest(ctx context.Context) (models.CreateCustomerSubscriptionJSONRequestBody, diag.Diagnostics) {
	var diags diag.Diagnostics

	recipients, d := plan.customerSubscriptionRecipients(ctx)
	diags.Append(d...)

	var entries []resource_customer_subscription.SubscriptionsValue
	diags.Append(plan.Subscriptions.ElementsAs(ctx, &entries, false)...)

	req := models.CreateCustomerSubscriptionJSONRequestBody{
		Recipients:    recipients,
		Subscriptions: make([]models.CustomerSubscriptionEntryInput, 0, len(entries)),
	}
	for _, entry := range entries {
		schedule, d := toCustomerSubscriptionScheduleInput(entry.Schedule)
		diags.Append(d...)
		attachments, pdfSettings, d := toCustomerSubscriptionReportOptions(ctx, entry)
		diags.Append(d...)
		req.Subscriptions = append(req.Subscriptions, models.CustomerSubscriptionEntryInput{
			Type:        models.CustomerSubscriptionScheduledType(entry.SubscriptionsType.ValueString()),
			Source:      models.CustomerSubscriptionSourceInput{Id: entry.Source.Id.ValueString()},
			Schedule:    schedule,
			Attachments: attachments,
			PdfSettings: pdfSettings,
		})
	}

	return req, diags
}

// toUpdateRequest converts the TF model to a JSON merge patch (RFC 7396).
//
// The patch always carries both recipient lists and the full subscriptions set:
// the API removes entry types that are missing from the set. An omitted
// attachments list is planned as [], which clears it; an omitted pdf_settings
// is unknown and not sent, so the API keeps the stored value.
func (plan *customerSubscriptionResourceModel) toUpdateRequest(ctx context.Context) (models.UpdateCustomerSubscriptionApplicationMergePatchPlusJSONRequestBody, diag.Diagnostics) {
	var diags diag.Diagnostics

	recipients, d := plan.customerSubscriptionRecipients(ctx)
	diags.Append(d...)

	var entries []resource_customer_subscription.SubscriptionsValue
	diags.Append(plan.Subscriptions.ElementsAs(ctx, &entries, false)...)

	patches := make([]models.CustomerSubscriptionEntryPatch, 0, len(entries))
	for _, entry := range entries {
		schedule, d := toCustomerSubscriptionScheduleInput(entry.Schedule)
		diags.Append(d...)
		attachments, pdfSettings, d := toCustomerSubscriptionReportOptions(ctx, entry)
		diags.Append(d...)
		patches = append(patches, models.CustomerSubscriptionEntryPatch{
			Type:        models.CustomerSubscriptionType(entry.SubscriptionsType.ValueString()),
			Source:      models.CustomerSubscriptionSourceInput{Id: entry.Source.Id.ValueString()},
			Schedule:    &schedule,
			Attachments: attachments,
			PdfSettings: pdfSettings,
		})
	}

	req := models.UpdateCustomerSubscriptionApplicationMergePatchPlusJSONRequestBody{
		Recipients: &models.CustomerSubscriptionRecipientPatch{
			Emails:        &recipients.Emails,
			SlackChannels: &recipients.SlackChannels,
		},
		Subscriptions: &patches,
	}

	return req, diags
}

// customerSubscriptionRecipients converts the recipients object. The API
// requires both arrays, so they are never nil.
func (plan *customerSubscriptionResourceModel) customerSubscriptionRecipients(ctx context.Context) (models.CustomerSubscriptionRecipients, diag.Diagnostics) {
	var diags diag.Diagnostics

	var emails, slack []string
	diags.Append(plan.Recipients.Emails.ElementsAs(ctx, &emails, false)...)
	diags.Append(plan.Recipients.SlackChannels.ElementsAs(ctx, &slack, false)...)

	addresses := make([]openapi_types.Email, len(emails))
	for i, email := range emails {
		addresses[i] = openapi_types.Email(email)
	}

	return models.CustomerSubscriptionRecipients{
		Emails:        addresses,
		SlackChannels: nonNilStrings(slack),
	}, diags
}

// toCustomerSubscriptionScheduleInput converts a schedule. custom_period_*
// are Optional+Computed and only sent when known.
func toCustomerSubscriptionScheduleInput(schedule resource_customer_subscription.ScheduleValue) (models.CustomerSubscriptionScheduleInput, diag.Diagnostics) {
	var diags diag.Diagnostics

	startDate, err := time.Parse(time.RFC3339, schedule.StartDate.ValueString())
	if err != nil {
		diags.AddError("Invalid Schedule Start Date", fmt.Sprintf("Could not parse start_date %q as RFC 3339: %s", schedule.StartDate.ValueString(), err))
	}
	deliveryTime, err := time.Parse(time.RFC3339, schedule.Time.ValueString())
	if err != nil {
		diags.AddError("Invalid Schedule Time", fmt.Sprintf("Could not parse time %q as RFC 3339: %s", schedule.Time.ValueString(), err))
	}

	input := models.CustomerSubscriptionScheduleInput{
		Frequency: models.CustomerSubscriptionFrequency(schedule.Frequency.ValueString()),
		StartDate: startDate,
		Time:      deliveryTime,
		TimeZone:  schedule.TimeZone.ValueString(),
	}
	if !schedule.CustomPeriodAmount.IsUnknown() {
		input.CustomPeriodAmount = int64ToIntPointer(schedule.CustomPeriodAmount)
	}
	if !schedule.CustomPeriodUnit.IsUnknown() && !schedule.CustomPeriodUnit.IsNull() {
		input.CustomPeriodUnit = new(models.CustomerSubscriptionCustomPeriodUnit(schedule.CustomPeriodUnit.ValueString()))
	}
	return input, diags
}

// toCustomerSubscriptionReportOptions converts attachments and pdf_settings.
// The API rejects both on every type other than report-subscription, so they
// are only sent for reports, and only when known (Optional+Computed).
func toCustomerSubscriptionReportOptions(ctx context.Context, entry resource_customer_subscription.SubscriptionsValue) (*[]models.CustomerSubscriptionAttachment, *models.CustomerSubscriptionPdfSettings, diag.Diagnostics) {
	var diags diag.Diagnostics

	if entry.SubscriptionsType.ValueString() != string(models.CustomerSubscriptionTypeReportSubscription) {
		return nil, nil, diags
	}

	var attachments *[]models.CustomerSubscriptionAttachment
	if !entry.Attachments.IsUnknown() && !entry.Attachments.IsNull() {
		var values []string
		diags.Append(entry.Attachments.ElementsAs(ctx, &values, false)...)
		list := make([]models.CustomerSubscriptionAttachment, len(values))
		for i, v := range values {
			list[i] = models.CustomerSubscriptionAttachment(v)
		}
		attachments = &list
	}

	var pdfSettings *models.CustomerSubscriptionPdfSettings
	if !entry.PdfSettings.IsUnknown() && !entry.PdfSettings.IsNull() {
		pdfSettings = &models.CustomerSubscriptionPdfSettings{
			IncludeAnnotations: entry.PdfSettings.IncludeAnnotations.ValueBool(),
			IncludeTable:       entry.PdfSettings.IncludeTable.ValueBool(),
			ShowFilters:        entry.PdfSettings.ShowFilters.ValueBool(),
		}
	}

	return attachments, pdfSettings, diags
}
