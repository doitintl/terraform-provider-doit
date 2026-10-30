package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_user_notification"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var userNotificationCostAnomaliesPath = path.Root("settings").AtName("cost_anomalies")

// userNotificationSettingsValidator checks the cross-attribute rules of
// settings.cost_anomalies that the API otherwise rejects with 422 at apply.
type userNotificationSettingsValidator struct{}

var _ resource.ConfigValidator = userNotificationSettingsValidator{}

func (v userNotificationSettingsValidator) Description(_ context.Context) string {
	return "allocations.ids must be empty for mode \"all\" and non-empty for \"include\"/\"exclude\" without duplicates; " +
		"repeat must set every_hours or when_cost_grows; when_cost_grows must set percent_increase or min_increase, and match when both are set"
}

func (v userNotificationSettingsValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v userNotificationSettingsValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var allocations resource_user_notification.AllocationsValue
	d := req.Config.GetAttribute(ctx, userNotificationCostAnomaliesPath.AtName("allocations"), &allocations)
	resp.Diagnostics.Append(d...)
	if !d.HasError() {
		validateUserNotificationAllocations(allocations, &resp.Diagnostics)
	}

	var repeat resource_user_notification.RepeatValue
	d = req.Config.GetAttribute(ctx, userNotificationCostAnomaliesPath.AtName("repeat"), &repeat)
	resp.Diagnostics.Append(d...)
	if !d.HasError() {
		validateUserNotificationRepeat(repeat, &resp.Diagnostics)
		validateUserNotificationWhenCostGrows(repeat.WhenCostGrows, &resp.Diagnostics)
	}
}

// validateUserNotificationAllocations checks that ids match mode and has no duplicates.
func validateUserNotificationAllocations(allocations resource_user_notification.AllocationsValue, diags *diag.Diagnostics) {
	if allocations.IsNull() || allocations.IsUnknown() {
		return
	}
	if allocations.Ids.IsNull() || allocations.Ids.IsUnknown() {
		return
	}

	idsPath := userNotificationCostAnomaliesPath.AtName("allocations").AtName("ids")

	knownCount := 0
	unknownCount := 0
	seen := map[string]bool{}
	for _, element := range allocations.Ids.Elements() {
		id, ok := element.(types.String)
		if !ok {
			continue
		}
		if id.IsUnknown() {
			unknownCount++
			continue
		}
		knownCount++
		if id.IsNull() || id.ValueString() == "" {
			diags.AddAttributeError(
				idsPath,
				"Invalid Allocation ID",
				"Allocation IDs must not be null or empty.",
			)
			continue
		}
		if seen[id.ValueString()] {
			diags.AddAttributeError(
				idsPath,
				"Duplicate Allocation ID",
				fmt.Sprintf("Allocation ID %q is listed more than once.", id.ValueString()),
			)
		}
		seen[id.ValueString()] = true
	}

	if allocations.Mode.IsNull() || allocations.Mode.IsUnknown() {
		return
	}

	switch allocations.Mode.ValueString() {
	case "all":
		if knownCount > 0 || unknownCount > 0 {
			diags.AddAttributeError(
				idsPath,
				"Invalid Allocation IDs",
				"`mode = \"all\"` covers every allocation with anomaly detection enabled, so `ids` must be `[]`.",
			)
		}
	case "include", "exclude":
		if knownCount == 0 && unknownCount == 0 {
			diags.AddAttributeError(
				idsPath,
				"Missing Allocation IDs",
				fmt.Sprintf("`mode = %q` requires at least one allocation ID in `ids`.", allocations.Mode.ValueString()),
			)
		}
	}
}

// validateUserNotificationRepeat rejects an empty repeat object.
func validateUserNotificationRepeat(repeat resource_user_notification.RepeatValue, diags *diag.Diagnostics) {
	if repeat.IsNull() || repeat.IsUnknown() {
		return
	}
	if repeat.EveryHours.IsUnknown() || repeat.WhenCostGrows.IsUnknown() {
		return
	}

	if repeat.EveryHours.IsNull() && repeat.WhenCostGrows.IsNull() {
		diags.AddAttributeError(
			userNotificationCostAnomaliesPath.AtName("repeat"),
			"Empty Repeat Configuration",
			"`repeat` must set `every_hours`, `when_cost_grows`, or both. Remove the `repeat` block to turn resending off.",
		)
	}
}

// validateUserNotificationWhenCostGrows requires at least one growth amount,
// and match when both are set.
func validateUserNotificationWhenCostGrows(whenCostGrows resource_user_notification.WhenCostGrowsValue, diags *diag.Diagnostics) {
	if whenCostGrows.IsNull() || whenCostGrows.IsUnknown() {
		return
	}
	if whenCostGrows.PercentIncrease.IsUnknown() || whenCostGrows.MinIncrease.IsUnknown() {
		return
	}

	wcgPath := userNotificationCostAnomaliesPath.AtName("repeat").AtName("when_cost_grows")

	// Checked before deferring on match: no value of match makes a rule
	// without thresholds valid.
	if whenCostGrows.PercentIncrease.IsNull() && whenCostGrows.MinIncrease.IsNull() {
		diags.AddAttributeError(
			wcgPath,
			"Missing Cost Growth Threshold",
			"`when_cost_grows` must set `percent_increase`, `min_increase`, or both.",
		)
		return
	}

	if whenCostGrows.Match.IsUnknown() {
		return
	}

	if !whenCostGrows.PercentIncrease.IsNull() && !whenCostGrows.MinIncrease.IsNull() && whenCostGrows.Match.IsNull() {
		diags.AddAttributeError(
			wcgPath.AtName("match"),
			"Missing Match",
			"`match` (`\"or\"` or `\"and\"`) is required when both `percent_increase` and `min_increase` are set.",
		)
	}
}
