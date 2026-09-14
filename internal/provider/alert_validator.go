package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_alert"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// validateAlertDestinationsPlan validates that alerts have at least one destination (email or Slack channel),
// taking into account create-time defaults and update-time state retention.
func validateAlertDestinationsPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	var planRecipients types.List
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("recipients"), &planRecipients)...)

	var planSlackChannels types.List
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("recipients_slack_channels"), &planSlackChannels)...)

	var configRecipients types.List
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("recipients"), &configRecipients)...)

	var configSlackChannels types.List
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("recipients_slack_channels"), &configSlackChannels)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if configRecipients.IsUnknown() || configSlackChannels.IsUnknown() {
		return
	}
	if !configRecipients.IsNull() && planRecipients.IsUnknown() {
		return
	}
	if !configSlackChannels.IsNull() && planSlackChannels.IsUnknown() {
		return
	}

	isCreate := req.State.Raw.Type() == nil || req.State.Raw.IsNull()
	var stateRecipients types.List
	if !isCreate {
		resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("recipients"), &stateRecipients)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if stateRecipients.IsUnknown() {
			return
		}
	}

	// Scan Slack channels in plan for uncertainty and non-null presence
	var channelVals []resource_alert.RecipientsSlackChannelsValue
	if !configSlackChannels.IsNull() && !planSlackChannels.IsUnknown() && !planSlackChannels.IsNull() {
		diags := planSlackChannels.ElementsAs(ctx, &channelVals, false)
		resp.Diagnostics.Append(diags...)
		if diags.HasError() {
			return
		}
	}

	knownChannels := 0
	unknownChannels := 0
	for _, ch := range channelVals {
		if ch.IsUnknown() {
			unknownChannels++
			continue
		}
		if !ch.IsNull() {
			knownChannels++
		}
	}

	if unknownChannels > 0 {
		return
	}

	// On Create:
	if isCreate {
		// If recipients is omitted from config, it defaults to the creator/owner email
		// unless Slack destinations are supplied. In either case, destinations >= 1.
		if configRecipients.IsNull() {
			return
		}

		// When recipients is explicitly configured:
		// If recipients has at least one element, it satisfies the requirement.
		if len(configRecipients.Elements()) > 0 {
			return
		}

		// recipients is explicitly empty []
		if knownChannels == 0 {
			resp.Diagnostics.AddAttributeError(
				path.Root("recipients"),
				"At Least One Recipient Required",
				"The 'recipients' attribute must contain at least one email address unless "+
					"at least one Slack channel is specified in 'recipients_slack_channels'. "+
					"If you want to use the default (creator), omit the recipients attribute entirely.",
			)
		}
		return
	}

	// On Update:
	effectiveEmailCount := 0
	if configRecipients.IsNull() {
		// Category B: omitted in config retains prior state
		if !stateRecipients.IsNull() {
			effectiveEmailCount = len(stateRecipients.Elements())
		}
	} else {
		effectiveEmailCount = len(planRecipients.Elements())
	}

	// recipients_slack_channels uses useNullForUnknownListWhenConfigNull(),
	// so omitting it clears Slack channels (knownChannels is 0).
	effectiveSlackCount := knownChannels

	if effectiveEmailCount == 0 && effectiveSlackCount == 0 {
		if !configRecipients.IsNull() && len(configRecipients.Elements()) == 0 {
			resp.Diagnostics.AddAttributeError(
				path.Root("recipients"),
				"At Least One Recipient Required",
				"The 'recipients' attribute must contain at least one email address unless "+
					"at least one Slack channel is specified in 'recipients_slack_channels'. "+
					"If you want to use the default (creator), omit the recipients attribute entirely.",
			)
		} else {
			resp.Diagnostics.AddAttributeError(
				path.Root("recipients_slack_channels"),
				"At Least One Destination Required",
				"At least one email recipient or Slack channel destination must remain.",
			)
		}
	}
}

// alertIgnoreValuesRangeValidator validates that ignore_values_range has valid bounds
// (lower_bound <= upper_bound) and is only configured when condition is 'percentage-change'.
type alertIgnoreValuesRangeValidator struct{}

var _ resource.ConfigValidator = alertIgnoreValuesRangeValidator{}

func (v alertIgnoreValuesRangeValidator) Description(_ context.Context) string {
	return "Validates that ignore_values_range has valid bounds and is only configured for percentage-change alerts"
}

func (v alertIgnoreValuesRangeValidator) MarkdownDescription(_ context.Context) string {
	return "Validates that `config.ignore_values_range` has `lower_bound <= upper_bound` and is only used when `config.condition` is `percentage-change`."
}

func (v alertIgnoreValuesRangeValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	rangePath := path.Root("config").AtName("ignore_values_range")
	var rangeVal resource_alert.IgnoreValuesRangeValue
	diags := req.Config.GetAttribute(ctx, rangePath, &rangeVal)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() || rangeVal.IsNull() || rangeVal.IsUnknown() {
		return
	}

	condPath := path.Root("config").AtName("condition")
	var condition types.String
	diags = req.Config.GetAttribute(ctx, condPath, &condition)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	validateIgnoreValuesRangeCondition(condition, rangePath, resp)
	validateIgnoreValuesRangeBounds(rangeVal, rangePath, resp)
}

func validateIgnoreValuesRangeCondition(condition types.String, rangePath path.Path, resp *resource.ValidateConfigResponse) {
	if condition.IsUnknown() {
		return
	}

	if condition.IsNull() {
		resp.Diagnostics.AddAttributeError(
			rangePath,
			"Invalid Ignore Values Range Condition",
			"ignore_values_range is only valid when condition is 'percentage-change', but condition is 'threshold'.",
		)
		return
	}

	if condition.ValueString() != "percentage-change" {
		resp.Diagnostics.AddAttributeError(
			rangePath,
			"Invalid Ignore Values Range Condition",
			"ignore_values_range is only valid when condition is 'percentage-change', but condition is '"+condition.ValueString()+"'.",
		)
	}
}

func validateIgnoreValuesRangeBounds(rangeVal resource_alert.IgnoreValuesRangeValue, rangePath path.Path, resp *resource.ValidateConfigResponse) {
	if rangeVal.LowerBound.IsUnknown() || rangeVal.UpperBound.IsUnknown() {
		return
	}

	if rangeVal.LowerBound.IsNull() || rangeVal.UpperBound.IsNull() {
		return
	}

	if rangeVal.LowerBound.ValueFloat64() > rangeVal.UpperBound.ValueFloat64() {
		resp.Diagnostics.AddAttributeError(
			rangePath.AtName("lower_bound"),
			"Invalid Ignore Values Range Bounds",
			fmt.Sprintf("lower_bound (%v) must be less than or equal to upper_bound (%v).",
				rangeVal.LowerBound.ValueFloat64(), rangeVal.UpperBound.ValueFloat64()),
		)
	}
}

// alertSlackChannelsValidator validates shape constraints (shared vs workspace)
// and ensures no duplicate destinations exist in recipients_slack_channels.
type alertSlackChannelsValidator struct{}

var _ resource.ConfigValidator = alertSlackChannelsValidator{}

func (v alertSlackChannelsValidator) Description(_ context.Context) string {
	return "Validates shape constraints and uniqueness of Slack channel destinations"
}

func (v alertSlackChannelsValidator) MarkdownDescription(_ context.Context) string {
	return "Validates that `recipients_slack_channels` destinations have valid shape (shared vs workspace) and are not duplicated."
}

func (v alertSlackChannelsValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	channelsPath := path.Root("recipients_slack_channels")
	var channels types.List
	diags := req.Config.GetAttribute(ctx, channelsPath, &channels)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() || channels.IsNull() || channels.IsUnknown() {
		return
	}

	var channelVals []resource_alert.RecipientsSlackChannelsValue
	diags = channels.ElementsAs(ctx, &channelVals, false)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	seen := make(map[string]int)

	for i, ch := range channelVals {
		if ch.IsUnknown() {
			continue
		}
		itemPath := channelsPath.AtListIndex(i)
		if ch.IsNull() {
			resp.Diagnostics.AddAttributeError(
				itemPath,
				"Null Slack Channel",
				"Slack channel element must not be null.",
			)
			continue
		}

		validateSlackChannelShape(ch, itemPath, resp)
		validateSlackChannelDuplicate(ch, i, itemPath, seen, resp)
	}
}

func validateSlackChannelShape(ch resource_alert.RecipientsSlackChannelsValue, itemPath path.Path, resp *resource.ValidateConfigResponse) {
	if ch.Shared.IsUnknown() || ch.Workspace.IsUnknown() {
		return
	}

	if !ch.Shared.IsNull() && ch.Shared.ValueBool() {
		if !ch.Workspace.IsNull() && ch.Workspace.ValueString() != "" {
			resp.Diagnostics.AddAttributeError(
				itemPath.AtName("workspace"),
				"Invalid Slack Channel Shape",
				"workspace must not be specified when shared is true.",
			)
		}
		return
	}

	if ch.Workspace.IsNull() || ch.Workspace.ValueString() == "" {
		resp.Diagnostics.AddAttributeError(
			itemPath.AtName("workspace"),
			"Invalid Slack Channel Shape",
			"workspace is required for a non-shared channel (when shared is false or omitted).",
		)
	}
}

func validateSlackChannelDuplicate(ch resource_alert.RecipientsSlackChannelsValue, index int, itemPath path.Path, seen map[string]int, resp *resource.ValidateConfigResponse) {
	if ch.Id.IsUnknown() || ch.Shared.IsUnknown() {
		return
	}
	if ch.Id.IsNull() {
		return
	}

	if !ch.Shared.IsNull() && ch.Shared.ValueBool() {
		destKey := "shared:" + ch.Id.ValueString()
		if _, exists := seen[destKey]; exists {
			resp.Diagnostics.AddAttributeError(
				itemPath,
				"Duplicate Slack Channel",
				fmt.Sprintf("Slack channel %q is specified more than once in 'recipients_slack_channels'. Each destination must be unique.", ch.Id.ValueString()),
			)
		} else {
			seen[destKey] = index
		}
		return
	}

	if ch.Workspace.IsUnknown() {
		return
	}

	destKey := "id:" + ch.Id.ValueString()
	if !ch.Workspace.IsNull() && ch.Workspace.ValueString() != "" {
		destKey = "workspace:" + ch.Workspace.ValueString() + ":" + ch.Id.ValueString()
	}

	if _, exists := seen[destKey]; exists {
		resp.Diagnostics.AddAttributeError(
			itemPath,
			"Duplicate Slack Channel",
			fmt.Sprintf("Slack channel %q is specified more than once in 'recipients_slack_channels'. Each destination must be unique.", ch.Id.ValueString()),
		)
	} else {
		seen[destKey] = index
	}
}

// alertScopeNAValidator warns when legacy NullFallback sentinel values such as
// "[Service N/A]" are found in config.scopes[*].values. Users should use
// include_null = true on the scope block instead.
type alertScopeNAValidator struct{}

var _ resource.ConfigValidator = alertScopeNAValidator{}

func (v alertScopeNAValidator) Description(_ context.Context) string {
	return "Warns when legacy NullFallback sentinel values (e.g. [Service N/A]) are used in scope values"
}

func (v alertScopeNAValidator) MarkdownDescription(_ context.Context) string {
	return "Warns when legacy NullFallback sentinel values (e.g. `[Service N/A]`) are used in " +
		"`config.scopes[*].values`. Use `include_null = true` on the scope block instead."
}

func (v alertScopeNAValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	basePath := path.Root("config").AtName("scopes")

	var scopes types.List
	diags := req.Config.GetAttribute(ctx, basePath, &scopes)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() || scopes.IsNull() || scopes.IsUnknown() {
		return
	}

	var scopeVals []resource_alert.ScopesValue
	diags = scopes.ElementsAs(ctx, &scopeVals, false)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	valueLists := make([]types.List, len(scopeVals))
	for i, s := range scopeVals {
		valueLists[i] = s.Values
	}
	warnNASentinels(ctx, basePath, valueLists, &resp.Diagnostics)
}
