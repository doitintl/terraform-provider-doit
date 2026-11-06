package provider

import (
	"context"
	"fmt"
	"uuid"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_customer_subscription"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

type customerSubscriptionResource struct {
	client *models.ClientWithResponses
}

// Ensure the implementation satisfies expected interfaces.
var (
	_ resource.Resource                     = (*customerSubscriptionResource)(nil)
	_ resource.ResourceWithConfigure        = (*customerSubscriptionResource)(nil)
	_ resource.ResourceWithImportState      = (*customerSubscriptionResource)(nil)
	_ resource.ResourceWithConfigValidators = (*customerSubscriptionResource)(nil)
	_ resource.ResourceWithModifyPlan       = (*customerSubscriptionResource)(nil)
)

// NewCustomerSubscriptionResource creates a new customer subscription resource instance.
func NewCustomerSubscriptionResource() resource.Resource {
	return &customerSubscriptionResource{}
}

// Configure adds the provider configured client to the resource.
func (r *customerSubscriptionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*models.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *models.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *customerSubscriptionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_customer_subscription"
}

// ImportState imports a subscription by its ID.
func (r *customerSubscriptionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// ModifyPlan plans the subscriptions set from the prior state: Terraform
// cannot match set elements with computed attributes to it on its own. It then
// marks etag unknown on every update, which the API changes.
func (r *customerSubscriptionResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	planSetNestedAttributesFromState(ctx, req, resp)
	planVolatileAttributesUnknownOnUpdate(ctx, req, resp, path.Root("etag"))
}

func (r *customerSubscriptionResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		customerSubscriptionRecipientsValidator{},
		customerSubscriptionEntriesValidator{},
	}
}

func (r *customerSubscriptionResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := resource_customer_subscription.CustomerSubscriptionResourceSchema(ctx)

	// Add UseStateForUnknown to stable Computed fields so they don't show as
	// "(known after apply)" on every plan that modifies the resource. name and
	// etag are intentionally excluded: the API changes them on updates.
	if attr, ok := s.Attributes["id"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["id"] = attr
	}
	if attr, ok := s.Attributes["created_by"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["created_by"] = attr
	}

	// The API fills in pdf_settings for report subscriptions when it is
	// omitted, and keeps the stored value when a later update omits it; it
	// cannot be cleared, only replaced. custom_period_* are only returned for
	// a Custom frequency.
	acknowledgeNotClearable(s,
		"subscriptions[*].pdf_settings",
		"subscriptions[*].schedule.custom_period_amount",
		"subscriptions[*].schedule.custom_period_unit",
	)

	if recipients, ok := s.Attributes["recipients"].(schema.SingleNestedAttribute); ok {
		s.Attributes["recipients"] = customizeCustomerSubscriptionRecipientsSchema(recipients)
	}
	if subscriptions, ok := s.Attributes["subscriptions"].(schema.SetNestedAttribute); ok {
		// attachments is user-authored: removing it from the configuration
		// removes every attachment (the API clears the list with []).
		if attachments, ok := subscriptions.NestedObject.Attributes["attachments"].(schema.ListAttribute); ok {
			attachments.PlanModifiers = append(attachments.PlanModifiers, useNullForUnknownListWhenConfigNull())
			subscriptions.NestedObject.Attributes["attachments"] = attachments
		}
		s.Attributes["subscriptions"] = customizeCustomerSubscriptionEntriesSchema(subscriptions)
	}

	// The generated descriptions are written for API clients (dry runs,
	// If-Match, camelCase names); describe the Terraform behavior instead.
	setCostAnomalyDescriptions(s.Attributes, map[string]string{
		"id":         "The ID of the subscription.",
		"name":       "Display name, set by the API: the name of the first source (by `type`) followed by `(Dashboard Subscription)`, `(Report Subscription)` or `(Commitment Subscription)`. It follows renames of that source.",
		"created_by": "Email address of the user or service account that created the subscription.",
		"etag":       "Current ETag of the subscription. The provider sends it with updates and deletes to detect changes made outside Terraform.",
		"recipients": "Who receives the deliveries. Both lists are required; use `[]` for an unused list. At least one list must be non-empty. Every entry in `subscriptions` shares these recipients.",
	})

	s.Description = "Manages an organization-wide subscription: scheduled email or Slack delivery of a dashboard, a report, a commitment, or any combination of the three, sent to one set of recipients.\n\n" +
		"Alerts and budgets also appear in the subscriptions API, but they are managed by `doit_alert` and `doit_budget`; importing one of them into this resource fails.\n\n" +
		"Slack channel IDs are supplied by you; the API has no lookup for them. A channel that is disconnected later stays in `recipients` without an error.\n\n" +
		"~> **Note:** Creating a subscription requires the Manage Settings permission or the permission for each subscribed type. The first delivery can take up to an hour."
	s.MarkdownDescription = s.Description

	s.Attributes["timeouts"] = timeouts.Attributes(ctx, timeouts.Opts{
		Create: true,
		Read:   true,
		Update: true,
		Delete: true,
	})

	rejectNullCollectionElements(s.Attributes)
	resp.Schema = s
}

func (r *customerSubscriptionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan customerSubscriptionResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, diags := plan.Timeouts.Create(ctx, DefaultCreateTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	apiReq, diags := plan.toCreateRequest(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := &models.CreateCustomerSubscriptionParams{
		IdempotencyKey: uuid.NewV4().String(),
	}

	createResp, err := r.client.CreateCustomerSubscriptionWithResponse(ctx, params, apiReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Customer Subscription",
			"Could not create customer subscription, unexpected error: "+err.Error(),
		)
		return
	}

	if createResp.StatusCode() != 201 {
		resp.Diagnostics.AddError(
			"Error Creating Customer Subscription",
			fmt.Sprintf("Could not create customer subscription, status: %d, body: %s", createResp.StatusCode(), string(createResp.Body)),
		)
		return
	}

	if createResp.JSON201 == nil {
		resp.Diagnostics.AddError(
			"Error Creating Customer Subscription",
			"Received empty response body for created customer subscription",
		)
		return
	}

	// Plan-first state pattern: overlay Computed-only and resolved fields from API response.
	resp.Diagnostics.Append(overlayCustomerSubscriptionComputedFields(ctx, createResp.JSON201, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *customerSubscriptionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state customerSubscriptionResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	readTimeout, diags := state.Timeouts.Read(ctx, DefaultReadTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	resp.Diagnostics.Append(r.populateState(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Resource was deleted outside Terraform
	if state.Id.IsNull() {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *customerSubscriptionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan customerSubscriptionResourceModel

	// Read Terraform plan data
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateTimeout, diags := plan.Timeouts.Update(ctx, DefaultUpdateTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	// Get the ID and ETag from the prior state
	var state customerSubscriptionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq, diags := plan.toUpdateRequest(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := &models.UpdateCustomerSubscriptionParams{
		IfMatch:        state.Etag.ValueString(),
		IdempotencyKey: uuid.NewV4().String(),
	}

	updateResp, err := r.client.UpdateCustomerSubscriptionWithApplicationMergePatchPlusJSONBodyWithResponse(ctx, state.Id.ValueString(), params, apiReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Customer Subscription",
			"Could not update customer subscription, unexpected error: "+err.Error(),
		)
		return
	}

	if updateResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Updating Customer Subscription",
			fmt.Sprintf("Could not update customer subscription, status: %d, body: %s", updateResp.StatusCode(), string(updateResp.Body)),
		)
		return
	}

	if updateResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Updating Customer Subscription",
			"Received empty response body for updated customer subscription",
		)
		return
	}

	// Plan-first state pattern: overlay Computed-only and resolved fields from API response.
	resp.Diagnostics.Append(overlayCustomerSubscriptionComputedFields(ctx, updateResp.JSON200, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *customerSubscriptionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state customerSubscriptionResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, diags := state.Timeouts.Delete(ctx, DefaultDeleteTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	id := state.Id.ValueString()

	etag := state.Etag.ValueString()
	if etag == "" {
		// If state lacks ETag, fetch it from API. The API requires If-Match
		// even when the subscription is already gone, so a missing one is
		// treated as deleted rather than sent without If-Match.
		current, found, diags := r.currentEtag(ctx, id)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() || !found {
			return
		}
		etag = current
	}

	params := &models.DeleteCustomerSubscriptionParams{
		IfMatch:        etag,
		IdempotencyKey: uuid.NewV4().String(),
	}

	deleteResp, err := r.client.DeleteCustomerSubscriptionWithResponse(ctx, id, params)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Customer Subscription",
			"Could not delete customer subscription, unexpected error: "+err.Error(),
		)
		return
	}

	// If 412 (stale ETag), retry once with a fresh ETag
	if deleteResp.StatusCode() == 412 {
		current, found, diags := r.currentEtag(ctx, id)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() || !found {
			return
		}
		params.IfMatch = current
		params.IdempotencyKey = uuid.NewV4().String()
		deleteResp, err = r.client.DeleteCustomerSubscriptionWithResponse(ctx, id, params)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Deleting Customer Subscription",
				"Could not delete customer subscription on retry: "+err.Error(),
			)
			return
		}
	}

	// The API returns 204 for an already-deleted subscription; 404 is
	// accepted as well since the resource is gone either way.
	if deleteResp.StatusCode() != 200 && deleteResp.StatusCode() != 204 && deleteResp.StatusCode() != 404 {
		resp.Diagnostics.AddError(
			"Error Deleting Customer Subscription",
			fmt.Sprintf("Could not delete customer subscription, status: %d, body: %s", deleteResp.StatusCode(), string(deleteResp.Body)),
		)
	}
}

// currentEtag fetches the subscription's current ETag. found is false when the
// subscription no longer exists.
func (r *customerSubscriptionResource) currentEtag(ctx context.Context, id string) (etag string, found bool, diags diag.Diagnostics) {
	getResp, err := r.client.GetCustomerSubscriptionWithResponse(ctx, id)
	if err != nil {
		diags.AddError(
			"Error Deleting Customer Subscription",
			"Could not fetch customer subscription to get ETag: "+err.Error(),
		)
		return "", false, diags
	}
	if getResp.StatusCode() == 404 {
		return "", false, diags
	}
	if getResp.StatusCode() != 200 || getResp.JSON200 == nil {
		diags.AddError(
			"Error Deleting Customer Subscription",
			fmt.Sprintf("Could not fetch customer subscription to get ETag, status: %d, body: %s", getResp.StatusCode(), string(getResp.Body)),
		)
		return "", false, diags
	}
	if e := nullableToPointer(getResp.JSON200.Etag); e != nil {
		etag = *e
	}
	return etag, true, diags
}

// customizeCustomerSubscriptionRecipientsSchema describes recipients for
// Terraform users and rejects values the API would trim.
func customizeCustomerSubscriptionRecipientsSchema(recipients schema.SingleNestedAttribute) schema.SingleNestedAttribute {
	if emails, ok := recipients.Attributes["emails"].(schema.ListAttribute); ok {
		emails.Validators = append(emails.Validators, listvalidator.ValueStringsAre(customerNotificationTrimmedValidator{}))
		recipients.Attributes["emails"] = emails
	}
	if slack, ok := recipients.Attributes["slack_channels"].(schema.ListAttribute); ok {
		slack.Validators = append(slack.Validators, listvalidator.ValueStringsAre(customerNotificationTrimmedValidator{}))
		recipients.Attributes["slack_channels"] = slack
	}

	setCostAnomalyDescriptions(recipients.Attributes, map[string]string{
		"emails":         "Email addresses that receive the deliveries, in the configured order. Each must be a user or a pending invite in an approved domain of the organization. The API stores them in lowercase; state keeps the configured spelling while the address is unchanged. Use `[]` when email is unused.",
		"slack_channels": "Slack channel IDs (such as `C0123ABCD`), in the configured order. Each must be connected to the organization's Slack workspace. Use `[]` when Slack is unused.",
	})
	return recipients
}

// customizeCustomerSubscriptionEntriesSchema adds validators and Terraform
// descriptions to the subscriptions set and its nested objects.
func customizeCustomerSubscriptionEntriesSchema(subscriptions schema.SetNestedAttribute) schema.SetNestedAttribute {
	entry := subscriptions.NestedObject.Attributes

	if attachments, ok := entry["attachments"].(schema.ListAttribute); ok {
		attachments.Validators = append(attachments.Validators, listvalidator.ValueStringsAre(
			stringvalidator.OneOf(customerSubscriptionAttachmentValues...),
		))
		entry["attachments"] = attachments
	}

	if schedule, ok := entry["schedule"].(schema.SingleNestedAttribute); ok {
		for _, name := range []string{"time", "start_date"} {
			if attr, ok := schedule.Attributes[name].(schema.StringAttribute); ok {
				attr.Validators = append(attr.Validators, rfc3339Validator{})
				schedule.Attributes[name] = attr
			}
		}
		setCostAnomalyDescriptions(schedule.Attributes, map[string]string{
			"time_zone":            "IANA time zone used to calculate deliveries, such as `Europe/Berlin`.",
			"start_date":           "When the schedule starts (RFC 3339). The start day is the calendar date of this instant in `time_zone`, not in UTC: to start on 2026-11-03 in `America/Los_Angeles`, use `2026-11-03T00:00:00-08:00`. The API stores the instant in UTC; state keeps the configured spelling while the instant is unchanged.",
			"time":                 "Delivery time (RFC 3339). Only the hour and minute of this instant, read in `time_zone`, are used for deliveries; the date part is stored but has no effect. State keeps the configured spelling while the instant is unchanged.",
			"frequency":            "How often to deliver: `Daily`, `Weekly`, `Monthly`, `Annually`, or `Custom`.",
			"custom_period_amount": "Number of `custom_period_unit` between deliveries (at least 1). Required when `frequency` is `Custom`, and not allowed otherwise.",
			"custom_period_unit":   "Unit for `custom_period_amount`: `Day`, `Week`, `Month`, or `Year`. Required when `frequency` is `Custom`, and not allowed otherwise.",
			"next_at":              "Next calculated delivery (RFC 3339, UTC).",
		})
		entry["schedule"] = schedule
	}

	if source, ok := entry["source"].(schema.SingleNestedAttribute); ok {
		setCostAnomalyDescriptions(source.Attributes, map[string]string{
			"id":   "ID of the dashboard, report (`doit_report`), or commitment (`doit_commitments`) to deliver.",
			"name": "Current name of the source, read from the source itself.",
		})
		entry["source"] = source
	}

	if pdfSettings, ok := entry["pdf_settings"].(schema.SingleNestedAttribute); ok {
		setCostAnomalyDescriptions(pdfSettings.Attributes, map[string]string{
			"show_filters":        "Include the summary of the report filters in the PDF.",
			"include_table":       "Include the data table in the PDF.",
			"include_annotations": "Include annotations in the PDF time range.",
		})
		entry["pdf_settings"] = pdfSettings
	}

	if permissions, ok := entry["permissions"].(schema.SingleNestedAttribute); ok {
		setCostAnomalyDescriptions(permissions.Attributes, map[string]string{
			"can_edit_recipients": "Whether the provider's credentials may change the recipients.",
			"can_delete":          "Whether the provider's credentials may delete this entry.",
			"can_unsubscribe":     "Whether the provider's credentials may remove their own email from the recipients.",
		})
		entry["permissions"] = permissions
	}

	setCostAnomalyDescriptions(entry, map[string]string{
		"type":         "What to deliver: `dashboard-subscription`, `report-subscription`, or `commitment-manager`. Each type may appear once.",
		"source":       "The dashboard, report, or commitment to deliver.",
		"schedule":     "Delivery schedule.",
		"attachments":  "Files to attach to report emails: `pdf`, `csv`, or both, in the configured order. Only `report-subscription` sends files; for other types it must be empty or omitted. When omitted, no files are attached: removing `attachments` from the configuration removes every attachment.",
		"pdf_settings": "PDF rendering options for report emails. Only allowed for `report-subscription`. When omitted, the API keeps its stored settings (all `true` for a new subscription).",
		"permissions":  "What the provider's credentials may do with this entry. Computed per request.",
	})

	subscriptions.NestedObject.Attributes = entry
	subscriptions.Description = "What to deliver: one to three entries, each with a different `type`. Entries that the provider's credentials cannot see are not returned and are left unchanged by updates."
	subscriptions.MarkdownDescription = subscriptions.Description
	return subscriptions
}
