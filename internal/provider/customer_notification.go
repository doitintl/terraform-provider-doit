package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_customer_notification"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

type customerNotificationResourceModel struct {
	resource_customer_notification.CustomerNotificationModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

// populateState fetches the notification from the API and populates the Terraform state.
// On 404, state.Id is set to null to signal Terraform to remove the resource from state.
func (r *customerNotificationResource) populateState(ctx context.Context, state *customerNotificationResourceModel) diag.Diagnostics {
	id := state.Id.ValueString()

	getResp, err := r.client.GetCustomerNotificationWithResponse(ctx, id)
	if err != nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Customer Notification", fmt.Sprintf("Could not read notification %s: %s", id, err)),
		}
	}

	if getResp.StatusCode() == 404 {
		state.Id = types.StringNull()
		return nil
	}

	if getResp.StatusCode() != 200 {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Customer Notification", fmt.Sprintf("Unexpected status code %d for notification %s: %s", getResp.StatusCode(), id, string(getResp.Body))),
		}
	}

	if getResp.JSON200 == nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Customer Notification", fmt.Sprintf("Received empty response body for notification %s", id)),
		}
	}

	return mapCustomerNotificationToModel(ctx, getResp.JSON200, state)
}

// mapCustomerNotificationToModel maps the API response to the Terraform model.
func mapCustomerNotificationToModel(ctx context.Context, resp *models.CustomerNotification, state *customerNotificationResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	state.Id = types.StringPointerValue(nullableToPointer(resp.Id))
	state.Name = types.StringValue(resp.Name)
	state.CreateTime = nullableTimeToString(resp.CreateTime)
	state.UpdateTime = nullableTimeToString(resp.UpdateTime)
	state.Etag = types.StringPointerValue(nullableToPointer(resp.Etag))

	destinations, d := buildCustomerNotificationDestinations(ctx, resp.Destinations, state.Destinations)
	diags.Append(d...)
	state.Destinations = destinations

	settings, d := buildCustomerNotificationSettings(ctx, &resp.Settings.CostAnomalies, state.Settings.CostAnomalies)
	diags.Append(d...)
	state.Settings = settings

	return diags
}

// buildCustomerNotificationDestinations maps the destinations object. prior is
// the value currently in state: the API lowercases emails, so an email keeps the
// prior spelling when it only differs in case.
func buildCustomerNotificationDestinations(ctx context.Context, destinations models.CustomerNotificationDestinations, prior resource_customer_notification.DestinationsValue) (resource_customer_notification.DestinationsValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	priorEmails := []string{}
	if !prior.Emails.IsNull() && !prior.Emails.IsUnknown() {
		diags.Append(prior.Emails.ElementsAs(ctx, &priorEmails, false)...)
	}
	emails := make([]string, len(destinations.Emails))
	for i, email := range destinations.Emails {
		emails[i] = preserveCustomerNotificationEmail(priorEmails, i, string(email))
	}

	emailList, d := types.ListValueFrom(ctx, types.StringType, emails)
	diags.Append(d...)
	slackList, d := types.ListValueFrom(ctx, types.StringType, nonNilStrings(destinations.SlackChannels))
	diags.Append(d...)
	teamsList, d := types.ListValueFrom(ctx, types.StringType, nonNilStrings(destinations.TeamsChannels))
	diags.Append(d...)

	v, d := resource_customer_notification.NewDestinationsValue(
		resource_customer_notification.DestinationsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"emails":         emailList,
			"slack_channels": slackList,
			"teams_channels": teamsList,
		},
	)
	diags.Append(d...)

	return v, diags
}

// preserveCustomerNotificationEmail returns the prior email at index when it
// equals the API's email ignoring case and surrounding whitespace.
func preserveCustomerNotificationEmail(prior []string, index int, apiEmail string) string {
	if index < len(prior) && strings.EqualFold(strings.TrimSpace(prior[index]), apiEmail) {
		return prior[index]
	}
	return apiEmail
}

// nonNilStrings returns v, or an empty slice for nil: the API always sends
// all three destination arrays, and Terraform needs [] rather than null.
func nonNilStrings(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

// buildCustomerNotificationSettings maps the cost-anomaly settings. prior is the
// value currently in state; it only decides how amounts are spelled.
func buildCustomerNotificationSettings(ctx context.Context, ca *models.CostAnomalySettings, prior resource_customer_notification.CostAnomaliesValue) (resource_customer_notification.SettingsValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	ids := []string{}
	if ca.Allocations.Ids != nil {
		ids = ca.Allocations.Ids
	}
	idList, d := types.ListValueFrom(ctx, types.StringType, ids)
	diags.Append(d...)
	allocations, d := resource_customer_notification.NewAllocationsValue(
		resource_customer_notification.AllocationsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"ids":  idList,
			"mode": types.StringValue(string(ca.Allocations.Mode)),
		},
	)
	diags.Append(d...)

	minExcessCost, d := resource_customer_notification.NewMinExcessCostValue(
		resource_customer_notification.MinExcessCostValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"amount":   preserveCostAnomalyAmount(prior.MinExcessCost.Amount, ca.MinExcessCost.Amount),
			"currency": types.StringValue(ca.MinExcessCost.Currency),
		},
	)
	diags.Append(d...)

	repeat, d := buildCustomerNotificationRepeat(ctx, ca.Repeat, prior.Repeat)
	diags.Append(d...)

	costAnomalies, d := resource_customer_notification.NewCostAnomaliesValue(
		resource_customer_notification.CostAnomaliesValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"allocations":     allocations,
			"min_excess_cost": minExcessCost,
			"min_severity":    types.StringValue(string(ca.MinSeverity)),
			"repeat":          repeat,
		},
	)
	diags.Append(d...)

	settings, d := resource_customer_notification.NewSettingsValue(
		resource_customer_notification.SettingsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"cost_anomalies": costAnomalies,
		},
	)
	diags.Append(d...)

	return settings, diags
}

func buildCustomerNotificationRepeat(ctx context.Context, repeat *models.CostAnomalyRepeat, prior resource_customer_notification.RepeatValue) (resource_customer_notification.RepeatValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	// The API omits repeat when no resend control is set.
	if repeat == nil {
		return resource_customer_notification.NewRepeatValueNull(), diags
	}

	whenCostGrows := resource_customer_notification.NewWhenCostGrowsValueNull()
	if wcg := repeat.WhenCostGrows; wcg != nil {
		minIncrease := resource_customer_notification.NewMinIncreaseValueNull()
		if wcg.MinIncrease != nil {
			var d diag.Diagnostics
			minIncrease, d = resource_customer_notification.NewMinIncreaseValue(
				resource_customer_notification.MinIncreaseValue{}.AttributeTypes(ctx),
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
		whenCostGrows, d = resource_customer_notification.NewWhenCostGrowsValue(
			resource_customer_notification.WhenCostGrowsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"match":            match,
				"min_increase":     minIncrease,
				"percent_increase": intPointerToInt64(wcg.PercentIncrease),
				"within_hours":     intPointerToInt64(wcg.WithinHours),
			},
		)
		diags.Append(d...)
	}

	v, d := resource_customer_notification.NewRepeatValue(
		resource_customer_notification.RepeatValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"every_hours":     intPointerToInt64(repeat.EveryHours),
			"when_cost_grows": whenCostGrows,
		},
	)
	diags.Append(d...)

	return v, diags
}

// overlayCustomerNotificationComputedFields applies the plan-first overlay for Create/Update.
func overlayCustomerNotificationComputedFields(ctx context.Context, apiResp *models.CustomerNotification, plan *customerNotificationResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	// Phase 1: Build fully-resolved state from API response.
	resolved := *plan
	diags.Append(mapCustomerNotificationToModel(ctx, apiResp, &resolved)...)
	if diags.HasError() {
		return diags
	}

	// Phase 2: Overlay Computed-only fields unconditionally from resolved.
	plan.Id = resolved.Id
	plan.CreateTime = resolved.CreateTime

	// update_time and etag: Computed-only but change server-side on every Update.
	if plan.UpdateTime.IsUnknown() {
		plan.UpdateTime = resolved.UpdateTime //nolint:overlaycheck // guarded to avoid inconsistent result on Update
	}
	if plan.Etag.IsUnknown() {
		plan.Etag = resolved.Etag //nolint:overlaycheck // guarded to avoid inconsistent result on Update
	}

	// name, destinations and settings: Required (settings has no Computed children) — never touch.

	return diags
}

// toCreateRequest converts the TF model to a CreateCustomerNotificationRequest.
func (plan *customerNotificationResourceModel) toCreateRequest(ctx context.Context) (models.CreateCustomerNotificationJSONRequestBody, diag.Diagnostics) {
	var diags diag.Diagnostics

	ca := plan.Settings.CostAnomalies

	ids, d := customerNotificationAllocationIDs(ctx, ca.Allocations)
	diags.Append(d...)

	destinations, d := plan.customerNotificationDestinations(ctx)
	diags.Append(d...)

	req := models.CreateCustomerNotificationJSONRequestBody{
		Name:         plan.Name.ValueString(),
		Destinations: destinations,
		Settings: models.CustomerNotificationSettings{
			CostAnomalies: models.CostAnomalySettings{
				MinSeverity: models.CostAnomalySettingsMinSeverity(ca.MinSeverity.ValueString()),
				MinExcessCost: models.Money{
					Amount:   ca.MinExcessCost.Amount.ValueString(),
					Currency: ca.MinExcessCost.Currency.ValueString(),
				},
				Allocations: models.CostAnomalyAllocations{
					Mode: models.CostAnomalyAllocationsMode(ca.Allocations.Mode.ValueString()),
					Ids:  ids,
				},
			},
		},
	}

	if !ca.Repeat.IsNull() {
		repeat := &models.CostAnomalyRepeat{
			EveryHours: int64ToIntPointer(ca.Repeat.EveryHours),
		}
		if wcg := ca.Repeat.WhenCostGrows; !wcg.IsNull() {
			repeat.WhenCostGrows = &models.CostAnomalyWhenCostGrows{
				PercentIncrease: int64ToIntPointer(wcg.PercentIncrease),
				WithinHours:     int64ToIntPointer(wcg.WithinHours),
			}
			if !wcg.Match.IsNull() {
				repeat.WhenCostGrows.Match = new(models.CostAnomalyWhenCostGrowsMatch(wcg.Match.ValueString()))
			}
			if !wcg.MinIncrease.IsNull() {
				repeat.WhenCostGrows.MinIncrease = &models.Money{
					Amount:   wcg.MinIncrease.Amount.ValueString(),
					Currency: wcg.MinIncrease.Currency.ValueString(),
				}
			}
		}
		req.Settings.CostAnomalies.Repeat = repeat
	}

	return req, diags
}

// toUpdateRequest converts the TF model to a JSON merge patch (RFC 7396).
//
// The patch always carries the full desired name, destinations (which replace
// the stored object) and settings. Nested objects are
// deep-merged server-side, so every optional member of repeat is sent as an
// explicit value or an explicit null — omitting it would keep the stored value.
func (plan *customerNotificationResourceModel) toUpdateRequest(ctx context.Context) (models.UpdateCustomerNotificationApplicationMergePatchPlusJSONRequestBody, diag.Diagnostics) {
	var diags diag.Diagnostics

	ca := plan.Settings.CostAnomalies

	ids, d := customerNotificationAllocationIDs(ctx, ca.Allocations)
	diags.Append(d...)

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

	if ca.Repeat.IsNull() {
		patch.Repeat.SetNull()
	} else {
		repeat := models.PatchCostAnomalyRepeat{
			EveryHours: int64ToNullableInt(ca.Repeat.EveryHours),
		}
		if wcg := ca.Repeat.WhenCostGrows; wcg.IsNull() {
			repeat.WhenCostGrows.SetNull()
		} else {
			patchWCG := models.PatchCostAnomalyWhenCostGrows{
				PercentIncrease: int64ToNullableInt(wcg.PercentIncrease),
				WithinHours:     int64ToNullableInt(wcg.WithinHours),
			}
			if wcg.Match.IsNull() {
				patchWCG.Match.SetNull()
			} else {
				patchWCG.Match.Set(models.PatchCostAnomalyWhenCostGrowsMatch(wcg.Match.ValueString()))
			}
			if wcg.MinIncrease.IsNull() {
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

	destinations, d := plan.customerNotificationDestinations(ctx)
	diags.Append(d...)

	req := models.UpdateCustomerNotificationApplicationMergePatchPlusJSONRequestBody{
		Name:         new(plan.Name.ValueString()),
		Destinations: &destinations,
		Settings: &models.PatchCustomerNotificationSettings{
			CostAnomalies: &patch,
		},
	}

	return req, diags
}

// customerNotificationDestinations converts the destinations object. The API
// requires all three arrays, so they are never nil.
func (plan *customerNotificationResourceModel) customerNotificationDestinations(ctx context.Context) (models.CustomerNotificationDestinations, diag.Diagnostics) {
	var diags diag.Diagnostics

	var emails, slack, teams []string
	diags.Append(plan.Destinations.Emails.ElementsAs(ctx, &emails, false)...)
	diags.Append(plan.Destinations.SlackChannels.ElementsAs(ctx, &slack, false)...)
	diags.Append(plan.Destinations.TeamsChannels.ElementsAs(ctx, &teams, false)...)

	addresses := make([]openapi_types.Email, len(emails))
	for i, email := range emails {
		addresses[i] = openapi_types.Email(email)
	}

	return models.CustomerNotificationDestinations{
		Emails:        addresses,
		SlackChannels: nonNilStrings(slack),
		TeamsChannels: nonNilStrings(teams),
	}, diags
}

// customerNotificationAllocationIDs returns the configured allocation IDs, never nil:
// the API requires the ids array even when it is empty (mode "all").
func customerNotificationAllocationIDs(ctx context.Context, allocations resource_customer_notification.AllocationsValue) ([]string, diag.Diagnostics) {
	ids := []string{}
	diags := allocations.Ids.ElementsAs(ctx, &ids, false)
	return ids, diags
}
