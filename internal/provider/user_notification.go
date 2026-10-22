package provider

import (
	"context"
	"fmt"
	"math/big"
	"regexp"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_user_notification"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/oapi-codegen/nullable"
)

// userNotificationAmountPattern is a whole number of dollars, optionally with
// zero cents. The API rejects fractional amounts and more than two decimals,
// and echoes accepted amounts with two decimals ("100" → "100.00").
var userNotificationAmountPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.0{1,2})?$`)

const userNotificationAmountMessage = "must be a whole number of US dollars, such as `100` or `100.00`"

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

	settings, d := buildUserNotificationSettings(ctx, &resp.Settings.CostAnomalies, state.Settings.CostAnomalies)
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

// buildUserNotificationSettings maps the cost-anomaly settings. prior is the
// value currently in state; it only decides how amounts are spelled.
func buildUserNotificationSettings(ctx context.Context, ca *models.CostAnomalySettings, prior resource_user_notification.CostAnomaliesValue) (resource_user_notification.SettingsValue, diag.Diagnostics) {
	var diags diag.Diagnostics

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
			"amount":   preserveUserNotificationAmount(prior.MinExcessCost.Amount, ca.MinExcessCost.Amount),
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

	settings, d := resource_user_notification.NewSettingsValue(
		resource_user_notification.SettingsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"cost_anomalies": costAnomalies,
		},
	)
	diags.Append(d...)

	return settings, diags
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
					"amount":   preserveUserNotificationAmount(prior.WhenCostGrows.MinIncrease.Amount, wcg.MinIncrease.Amount),
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

// preserveUserNotificationAmount keeps the amount in state when it has the same
// value as the API's. The API always answers with two decimals ("100" →
// "100.00"), so taking its spelling would drift from a config that omits them.
func preserveUserNotificationAmount(prior types.String, apiAmount string) types.String {
	if !prior.IsUnknown() && !prior.IsNull() && userNotificationAmountsEqual(prior.ValueString(), apiAmount) {
		return prior
	}
	return types.StringValue(apiAmount)
}

func userNotificationAmountsEqual(a, b string) bool {
	ra, ok := new(big.Rat).SetString(a)
	if !ok {
		return false
	}
	rb, ok := new(big.Rat).SetString(b)
	if !ok {
		return false
	}
	return ra.Cmp(rb) == 0
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

	// user_id, name and settings: Required (settings has no Computed children) — never touch.

	return diags
}

// toCreateRequest converts the TF model to a CreateUserNotificationRequest.
func (plan *userNotificationResourceModel) toCreateRequest(ctx context.Context) (models.CreateUserNotificationJSONRequestBody, diag.Diagnostics) {
	var diags diag.Diagnostics

	ca := plan.Settings.CostAnomalies

	ids, d := userNotificationAllocationIDs(ctx, ca.Allocations)
	diags.Append(d...)

	req := models.CreateUserNotificationJSONRequestBody{
		Name: plan.Name.ValueString(),
		Settings: models.UserNotificationSettings{
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
// The patch always carries the full desired settings. Nested objects are
// deep-merged server-side, so every optional member of repeat is sent as an
// explicit value or an explicit null — omitting it would keep the stored value.
func (plan *userNotificationResourceModel) toUpdateRequest(ctx context.Context) (models.UpdateUserNotificationApplicationMergePatchPlusJSONRequestBody, diag.Diagnostics) {
	var diags diag.Diagnostics

	ca := plan.Settings.CostAnomalies

	ids, d := userNotificationAllocationIDs(ctx, ca.Allocations)
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

	req := models.UpdateUserNotificationApplicationMergePatchPlusJSONRequestBody{
		Name: new(plan.Name.ValueString()),
		Settings: &models.PatchUserNotificationSettings{
			CostAnomalies: &patch,
		},
	}

	return req, diags
}

// userNotificationAllocationIDs returns the configured allocation IDs, never nil:
// the API requires the ids array even when it is empty (mode "all").
func userNotificationAllocationIDs(ctx context.Context, allocations resource_user_notification.AllocationsValue) ([]string, diag.Diagnostics) {
	ids := []string{}
	diags := allocations.Ids.ElementsAs(ctx, &ids, false)
	return ids, diags
}

func int64ToIntPointer(v types.Int64) *int {
	if v.IsNull() {
		return nil
	}
	return new(int(v.ValueInt64()))
}

func int64ToNullableInt(v types.Int64) nullable.Nullable[int] {
	if v.IsNull() {
		return nullable.NewNullNullable[int]()
	}
	return valueToNullable(int(v.ValueInt64()))
}
