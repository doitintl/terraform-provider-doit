package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_customer_notification"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var customerNotificationCostAnomaliesPath = path.Root("settings").AtName("cost_anomalies")

// customerNotificationSettingsValidator checks the cross-attribute rules of
// settings.cost_anomalies that the API otherwise rejects with 422 at apply.
type customerNotificationSettingsValidator struct{}

var _ resource.ConfigValidator = customerNotificationSettingsValidator{}

func (v customerNotificationSettingsValidator) Description(_ context.Context) string {
	return "allocations.ids must be empty for mode \"all\" and non-empty for \"include\"/\"exclude\" without duplicates; " +
		"repeat must set every_hours or when_cost_grows; when_cost_grows must set percent_increase or min_increase, and match when both are set"
}

func (v customerNotificationSettingsValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v customerNotificationSettingsValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var allocations resource_customer_notification.AllocationsValue
	d := req.Config.GetAttribute(ctx, customerNotificationCostAnomaliesPath.AtName("allocations"), &allocations)
	resp.Diagnostics.Append(d...)
	if !d.HasError() {
		validateCustomerNotificationAllocations(allocations, &resp.Diagnostics)
	}

	var repeat resource_customer_notification.RepeatValue
	d = req.Config.GetAttribute(ctx, customerNotificationCostAnomaliesPath.AtName("repeat"), &repeat)
	resp.Diagnostics.Append(d...)
	if !d.HasError() {
		validateCustomerNotificationRepeat(repeat, &resp.Diagnostics)
		validateCustomerNotificationWhenCostGrows(repeat.WhenCostGrows, &resp.Diagnostics)
	}
}

// validateCustomerNotificationAllocations checks that ids match mode and has no duplicates.
func validateCustomerNotificationAllocations(allocations resource_customer_notification.AllocationsValue, diags *diag.Diagnostics) {
	if allocations.IsNull() || allocations.IsUnknown() {
		return
	}
	if allocations.Ids.IsNull() || allocations.Ids.IsUnknown() {
		return
	}

	idsPath := customerNotificationCostAnomaliesPath.AtName("allocations").AtName("ids")

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

// validateCustomerNotificationRepeat rejects an empty repeat object.
func validateCustomerNotificationRepeat(repeat resource_customer_notification.RepeatValue, diags *diag.Diagnostics) {
	if repeat.IsNull() || repeat.IsUnknown() {
		return
	}
	if repeat.EveryHours.IsUnknown() || repeat.WhenCostGrows.IsUnknown() {
		return
	}

	if repeat.EveryHours.IsNull() && repeat.WhenCostGrows.IsNull() {
		diags.AddAttributeError(
			customerNotificationCostAnomaliesPath.AtName("repeat"),
			"Empty Repeat Configuration",
			"`repeat` must set `every_hours`, `when_cost_grows`, or both. Remove the `repeat` block to turn resending off.",
		)
	}
}

// validateCustomerNotificationWhenCostGrows requires at least one growth amount,
// and match when both are set.
func validateCustomerNotificationWhenCostGrows(whenCostGrows resource_customer_notification.WhenCostGrowsValue, diags *diag.Diagnostics) {
	if whenCostGrows.IsNull() || whenCostGrows.IsUnknown() {
		return
	}
	if whenCostGrows.PercentIncrease.IsUnknown() || whenCostGrows.MinIncrease.IsUnknown() {
		return
	}

	wcgPath := customerNotificationCostAnomaliesPath.AtName("repeat").AtName("when_cost_grows")

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

var customerNotificationDestinationsPath = path.Root("destinations")

// customerNotificationDestinationsValidator checks the destination rules the
// API otherwise rejects with 422 at apply: at least one destination, and no
// duplicate within a list (emails compare case-insensitively).
type customerNotificationDestinationsValidator struct{}

var _ resource.ConfigValidator = customerNotificationDestinationsValidator{}

func (v customerNotificationDestinationsValidator) Description(_ context.Context) string {
	return "destinations must have at least one email, Slack channel or Teams channel, without duplicates inside a list"
}

func (v customerNotificationDestinationsValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v customerNotificationDestinationsValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var destinations resource_customer_notification.DestinationsValue
	d := req.Config.GetAttribute(ctx, customerNotificationDestinationsPath, &destinations)
	resp.Diagnostics.Append(d...)
	if d.HasError() {
		return
	}
	validateCustomerNotificationDestinations(destinations, &resp.Diagnostics)
}

// validateCustomerNotificationDestinations requires one non-empty list and
// rejects duplicates. Unknown lists or elements defer the checks that depend on them.
func validateCustomerNotificationDestinations(destinations resource_customer_notification.DestinationsValue, diags *diag.Diagnostics) {
	if destinations.IsNull() || destinations.IsUnknown() {
		return
	}

	lists := []struct {
		name          string
		list          types.List
		caseSensitive bool
	}{
		{"emails", destinations.Emails, false},
		{"slack_channels", destinations.SlackChannels, true},
		{"teams_channels", destinations.TeamsChannels, true},
	}

	anyEntry := false // a known or unknown entry exists in some list
	for _, l := range lists {
		if l.list.IsUnknown() {
			anyEntry = true // might be non-empty
			continue
		}
		if l.list.IsNull() {
			continue
		}
		seen := map[string]bool{}
		for _, element := range l.list.Elements() {
			value, ok := element.(types.String)
			if !ok {
				continue
			}
			anyEntry = true
			if value.IsUnknown() {
				continue
			}
			listPath := customerNotificationDestinationsPath.AtName(l.name)
			if value.IsNull() || strings.TrimSpace(value.ValueString()) == "" {
				diags.AddAttributeError(listPath, "Invalid Destination", "Destination entries must not be null or empty.")
				continue
			}
			key := value.ValueString()
			if !l.caseSensitive {
				key = strings.ToLower(key)
			}
			if seen[key] {
				diags.AddAttributeError(listPath, "Duplicate Destination", fmt.Sprintf("%q is listed more than once.", value.ValueString()))
			}
			seen[key] = true
		}
	}

	if !anyEntry {
		diags.AddAttributeError(
			customerNotificationDestinationsPath,
			"No Destinations",
			"At least one of `emails`, `slack_channels` or `teams_channels` must contain an entry.",
		)
	}
}

// customerNotificationTrimmedValidator rejects values with leading or trailing
// whitespace: the API trims them, which would otherwise differ from the config.
type customerNotificationTrimmedValidator struct{}

var _ validator.String = customerNotificationTrimmedValidator{}

func (v customerNotificationTrimmedValidator) Description(_ context.Context) string {
	return "value must not have leading or trailing whitespace"
}

func (v customerNotificationTrimmedValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v customerNotificationTrimmedValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if value := req.ConfigValue.ValueString(); value != strings.TrimSpace(value) {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Value",
			fmt.Sprintf("Attribute %s must not have leading or trailing whitespace, got: %q", req.Path, value),
		)
	}
}
