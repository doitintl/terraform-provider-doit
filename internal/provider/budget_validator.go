package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_budget"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ validator.Int64 = budgetStartPeriodValidator{}

const budgetMinStartPeriodMs int64 = 1514764800000 // 2018-01-01 00:00 UTC

type budgetStartPeriodValidator struct{}

func (v budgetStartPeriodValidator) Description(_ context.Context) string {
	return "Ensures that start_period is no earlier than 2018-01-01 UTC and is aligned for recurring budgets."
}

func (v budgetStartPeriodValidator) MarkdownDescription(_ context.Context) string {
	return "Ensures that start_period is no earlier than 2018-01-01 UTC and is aligned for recurring budgets."
}

func (v budgetStartPeriodValidator) ValidateInt64(ctx context.Context, req validator.Int64Request, resp *validator.Int64Response) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	startPeriodMs := req.ConfigValue.ValueInt64()
	if err := validateBudgetStartPeriodMinimum(startPeriodMs); err != nil {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid Budget Start Period", err.Error())
		return
	}

	var budgetType types.String
	diags := req.Config.GetAttribute(ctx, req.Path.ParentPath().AtName("type"), &budgetType)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if budgetType.IsNull() || budgetType.IsUnknown() {
		return
	}

	if budgetType.ValueString() != "recurring" {
		return
	}

	var timeInterval types.String
	diags = req.Config.GetAttribute(ctx, req.Path.ParentPath().AtName("time_interval"), &timeInterval)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if timeInterval.IsNull() || timeInterval.IsUnknown() {
		return
	}

	if err := validateBudgetStartPeriod(budgetType.ValueString(), timeInterval.ValueString(), startPeriodMs); err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Budget Start Period",
			err.Error(),
		)
	}
}

func validateBudgetStartPeriod(budgetType, timeInterval string, startPeriodMs int64) error {
	if err := validateBudgetStartPeriodMinimum(startPeriodMs); err != nil {
		return err
	}
	if budgetType != "recurring" {
		return nil
	}

	startPeriodTime := time.UnixMilli(startPeriodMs).UTC()

	var expectedStart time.Time
	switch timeInterval {
	case "year":
		expectedStart = time.Date(startPeriodTime.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
	case "quarter":
		month := startPeriodTime.Month()
		quarterStartMonth := time.Month(((int(month)-1)/3)*3 + 1)
		expectedStart = time.Date(startPeriodTime.Year(), quarterStartMonth, 1, 0, 0, 0, 0, time.UTC)
	case "month":
		expectedStart = time.Date(startPeriodTime.Year(), startPeriodTime.Month(), 1, 0, 0, 0, 0, time.UTC)
	case "week":
		// ISO week starts on Monday
		// Calculate days to subtract to get to the previous Monday
		weekday := startPeriodTime.Weekday()
		if weekday == time.Sunday {
			weekday = 7
		}
		daysToSubtract := int(weekday) - 1
		expectedStart = time.Date(startPeriodTime.Year(), startPeriodTime.Month(), startPeriodTime.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -daysToSubtract)
	case "day":
		expectedStart = time.Date(startPeriodTime.Year(), startPeriodTime.Month(), startPeriodTime.Day(), 0, 0, 0, 0, time.UTC)
	default:
		// ValidateBudgetTimeInterval should have handled this but we will return an error anyway if we arrive here
		return fmt.Errorf("time_interval must be one of: day, week, month, quarter, year. Provided: %s", timeInterval)
	}

	expectedStartMs := expectedStart.UnixMilli()

	if startPeriodMs != expectedStartMs {
		return fmt.Errorf("start_period %d (%s) must be at the beginning of the period %d (%s) for recurring budget with interval %q",
			startPeriodMs, startPeriodTime.Format(time.RFC3339),
			expectedStartMs, expectedStart.Format(time.RFC3339),
			timeInterval)
	}
	return nil
}

func validateBudgetStartPeriodMinimum(startPeriodMs int64) error {
	if startPeriodMs < budgetMinStartPeriodMs {
		return fmt.Errorf("start_period must be on or after 2018-01-01 00:00 UTC (%d). Provided: %d", budgetMinStartPeriodMs, startPeriodMs)
	}
	return nil
}

var _ validator.String = budgetTimeIntervalValidator{}

type budgetTimeIntervalValidator struct{}

func (v budgetTimeIntervalValidator) Description(_ context.Context) string {
	return "Ensures that the time_interval is one of: day, week, month, quarter, year."
}

func (v budgetTimeIntervalValidator) MarkdownDescription(_ context.Context) string {
	return "Ensures that the time_interval is one of: day, week, month, quarter, year."
}

func (v budgetTimeIntervalValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	timeInterval := req.ConfigValue.ValueString()
	if err := validateBudgetTimeInterval(timeInterval); err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Budget Time Interval",
			err.Error(),
		)
	}
}

func validateBudgetTimeInterval(timeInterval string) error {
	switch timeInterval {
	case "day", "week", "month", "quarter", "year":
		return nil
	default:
		return fmt.Errorf("time_interval must be one of: day, week, month, quarter, year. Provided: %s", timeInterval)
	}
}

// budgetTypeEndPeriodValidator validates that:
// - end_period is not set when type is "recurring"
// - end_period is required when type is "fixed".
type budgetTypeEndPeriodValidator struct{}

func (v budgetTypeEndPeriodValidator) Description(_ context.Context) string {
	return "Validates that end_period is not set for recurring budgets and is required for fixed budgets"
}

func (v budgetTypeEndPeriodValidator) MarkdownDescription(_ context.Context) string {
	return "Validates that `end_period` is not set for recurring budgets and is required for fixed budgets"
}

func (v budgetTypeEndPeriodValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var budgetType types.String
	var endPeriod types.Int64

	// Get the type attribute
	diags := req.Config.GetAttribute(ctx, path.Root("type"), &budgetType)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get the end_period attribute
	diags = req.Config.GetAttribute(ctx, path.Root("end_period"), &endPeriod)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validity depends on both values. Defer until Terraform has resolved them.
	if budgetType.IsNull() || budgetType.IsUnknown() || endPeriod.IsUnknown() {
		return
	}

	// If type is "recurring" and end_period is set, that's an error
	if budgetType.ValueString() == "recurring" {
		if !endPeriod.IsNull() {
			resp.Diagnostics.AddAttributeError(
				path.Root("end_period"),
				"Invalid Attribute Combination",
				"Attribute end_period cannot be set when type is \"recurring\". "+
					"For recurring budgets, the budget continues indefinitely without an end date. "+
					"Only fixed budgets require an end_period.",
			)
		}
	}

	// If type is "fixed" and end_period is not set, that's an error
	if budgetType.ValueString() == "fixed" {
		if endPeriod.IsNull() {
			resp.Diagnostics.AddAttributeError(
				path.Root("end_period"),
				"Missing Required Attribute",
				"Attribute end_period is required when type is \"fixed\". "+
					"Fixed budgets must have a defined end date.",
			)
		}
	}
}

// budgetAlertsLengthValidator validates that alerts list has 1-3 items when specified.
// Empty alerts are not allowed because the API ignores empty lists and applies default alerts;
// requiring 1-3 alerts or omitting the attribute avoids unexpected API-side defaults.
type budgetAlertsLengthValidator struct{}

func (v budgetAlertsLengthValidator) Description(_ context.Context) string {
	return "Validates that alerts list has 1-3 items when specified"
}

func (v budgetAlertsLengthValidator) MarkdownDescription(_ context.Context) string {
	return "Validates that `alerts` list has 1-3 items when specified"
}

func (v budgetAlertsLengthValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var alerts types.List

	// Get the alerts attribute
	diags := req.Config.GetAttribute(ctx, path.Root("alerts"), &alerts)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if alerts.IsNull() || alerts.IsUnknown() {
		return
	}

	if len(alerts.Elements()) == 0 {
		resp.Diagnostics.AddAttributeError(
			path.Root("alerts"),
			"Invalid Alerts Configuration",
			"Budget alerts cannot be empty. Specify 1-3 alerts or omit the attribute.",
		)
		return
	}

	if len(alerts.Elements()) > 3 {
		resp.Diagnostics.AddAttributeError(
			path.Root("alerts"),
			"Invalid Alerts Configuration",
			fmt.Sprintf("Budget can have up to 3 alerts. Found %d alerts.", len(alerts.Elements())),
		)
	}
}

// budgetRecipientsMinLengthValidator validates that recipients list is not empty when specified.
// The API requires at least one recipient and will add a default if none provided,
// causing a plan/state mismatch. This validator gives early feedback.
type budgetRecipientsMinLengthValidator struct{}

func (v budgetRecipientsMinLengthValidator) Description(_ context.Context) string {
	return "Validates that recipients list has at least 1 item when specified"
}

func (v budgetRecipientsMinLengthValidator) MarkdownDescription(_ context.Context) string {
	return "Validates that `recipients` list has at least 1 item when specified"
}

func (v budgetRecipientsMinLengthValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var recipients types.List

	diags := req.Config.GetAttribute(ctx, path.Root("recipients"), &recipients)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Only validate if explicitly set (not null/unknown)
	if recipients.IsNull() || recipients.IsUnknown() {
		return
	}

	if len(recipients.Elements()) == 0 {
		resp.Diagnostics.AddAttributeError(
			path.Root("recipients"),
			"Invalid Recipients Configuration",
			"Budget recipients cannot be empty. Specify at least one recipient or omit the attribute to use the default.",
		)
	}
}

// budgetEndPeriodValidator validates that end_period is not set to the magic number 2678400000.
type budgetEndPeriodValidator struct{}

func (v budgetEndPeriodValidator) Description(_ context.Context) string {
	return "Ensures that end_period is not set to the internal magic value 2678400000."
}

func (v budgetEndPeriodValidator) MarkdownDescription(_ context.Context) string {
	return "Ensures that `end_period` is not set to the internal magic value 2678400000."
}

func (v budgetEndPeriodValidator) ValidateInt64(_ context.Context, req validator.Int64Request, resp *validator.Int64Response) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	if req.ConfigValue.ValueInt64() == 2678400000 {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Budget End Period",
			"The value 2678400000 is reserved and cannot be used as an end_period.",
		)
	}
}

// budgetCollaboratorsRoleValidator rejects a configured collaborator that has no
// role. The schema marks role Optional+Computed, but the API answers a request
// without one with "invalid role" and assigns no default, so an omitted role
// can only fail at apply.
type budgetCollaboratorsRoleValidator struct{}

func (v budgetCollaboratorsRoleValidator) Description(_ context.Context) string {
	return "Validates that every collaborator has a role"
}

func (v budgetCollaboratorsRoleValidator) MarkdownDescription(_ context.Context) string {
	return "Validates that every `collaborators` entry has a `role`"
}

func (v budgetCollaboratorsRoleValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var collaborators types.List
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("collaborators"), &collaborators)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if collaborators.IsNull() || collaborators.IsUnknown() {
		return
	}

	for i, elem := range collaborators.Elements() {
		if elem.IsNull() || elem.IsUnknown() {
			continue
		}
		collabVal, ok := elem.(resource_budget.CollaboratorsValue)
		if !ok {
			continue
		}
		if collabVal.Role.IsUnknown() {
			continue
		}
		if collabVal.Role.IsNull() {
			resp.Diagnostics.AddAttributeError(
				path.Root("collaborators").AtListIndex(i).AtName("role"),
				"Collaborator Role Required",
				"Every collaborator needs a role (owner, editor or viewer). The DoiT API rejects a collaborator without one.",
			)
		}
	}
}

// budgetCollaboratorsOwnerValidator validates that a configured collaborators
// list contains exactly one owner.
//
// This is a provider rule, not an API one: the API accepts several owners. What
// it does not do is take the list as given. When the request has no owner (an
// empty list, or only editors/viewers) it appends the creator as an owner, so
// the stored list differs from the configured one and Terraform would report an
// inconsistent result or a permanent diff. A list with no owner is therefore
// rejected here, and omitting the attribute is the way to get the default owner.
// Exactly one is required (rather than at least one) to keep the configured
// list and the stored list unambiguous.
type budgetCollaboratorsOwnerValidator struct{}

func (v budgetCollaboratorsOwnerValidator) Description(_ context.Context) string {
	return "Validates that collaborators list contains exactly one owner"
}

func (v budgetCollaboratorsOwnerValidator) MarkdownDescription(_ context.Context) string {
	return "Validates that `collaborators` list contains exactly one owner"
}

func (v budgetCollaboratorsOwnerValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var collaborators types.List

	// Get the collaborators attribute
	diags := req.Config.GetAttribute(ctx, path.Root("collaborators"), &collaborators)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If null/unknown, skip validation - let API compute default (creator as owner)
	if collaborators.IsNull() || collaborators.IsUnknown() {
		return
	}

	// An empty list would make the API append the creator as owner, which then
	// no longer matches the configured []. Omitting the attribute is the supported
	// way to get that default.
	if len(collaborators.Elements()) == 0 {
		resp.Diagnostics.AddAttributeError(
			path.Root("collaborators"),
			"Exactly One Owner Required",
			"The 'collaborators' attribute cannot be empty. "+
				"When setting collaborators explicitly, exactly one collaborator with role 'owner' is required, "+
				"because the API adds the creator as owner to any list that has none, which would not match your configuration. "+
				"If you want the default (creator as owner), omit the collaborators attribute entirely.",
		)
		return
	}

	// Count owners in the collaborators list
	ownerCount := 0
	unknownCount := 0
	for _, elem := range collaborators.Elements() {
		// Use the generated CollaboratorsValue type
		collabVal, ok := elem.(resource_budget.CollaboratorsValue)
		if !ok {
			continue
		}

		if collabVal.IsUnknown() {
			unknownCount++
			continue
		}
		if collabVal.IsNull() {
			continue
		}

		if collabVal.Role.IsUnknown() {
			unknownCount++
			continue
		}
		if collabVal.Role.IsNull() {
			continue
		}

		if collabVal.Role.ValueString() == "owner" {
			ownerCount++
		}
	}

	if ownerCount == 0 && unknownCount == 0 {
		resp.Diagnostics.AddAttributeError(
			path.Root("collaborators"),
			"Exactly One Owner Required",
			"The 'collaborators' list must contain exactly one collaborator with role 'owner'. "+
				"Found 0 owners. Add a collaborator with role = \"owner\", or omit the collaborators attribute "+
				"to let the API make the creator the owner.",
		)
	} else if ownerCount > 1 {
		resp.Diagnostics.AddAttributeError(
			path.Root("collaborators"),
			"Exactly One Owner Required",
			fmt.Sprintf("The 'collaborators' list must contain exactly one collaborator with role 'owner'. "+
				"Found %d owners. Only one owner is allowed.", ownerCount),
		)
	}
}

// budgetScopeNAValidator warns when legacy NullFallback sentinel values such as
// "[Service N/A]" are found in scopes[*].values. Users should use
// include_null = true on the scope block instead.
type budgetScopeNAValidator struct{}

var _ resource.ConfigValidator = budgetScopeNAValidator{}

func (v budgetScopeNAValidator) Description(_ context.Context) string {
	return "Warns when legacy NullFallback sentinel values (e.g. [Service N/A]) are used in scope values"
}

func (v budgetScopeNAValidator) MarkdownDescription(_ context.Context) string {
	return "Warns when legacy NullFallback sentinel values (e.g. `[Service N/A]`) are used in " +
		"`scopes[*].values`. Use `include_null = true` on the scope block instead."
}

func (v budgetScopeNAValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	basePath := path.Root("scopes")

	var scopes types.List
	diags := req.Config.GetAttribute(ctx, basePath, &scopes)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() || scopes.IsNull() || scopes.IsUnknown() {
		return
	}

	var scopeVals []resource_budget.ScopesValue
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

// budgetSlackChannelsValidator validates that customer_id and workspace
// are required for each Slack channel recipient unless shared is true.
type budgetSlackChannelsValidator struct{}

var _ resource.ConfigValidator = budgetSlackChannelsValidator{}

func (v budgetSlackChannelsValidator) Description(_ context.Context) string {
	return "Validates that customer_id and workspace are required for Slack channels unless shared is true"
}

func (v budgetSlackChannelsValidator) MarkdownDescription(_ context.Context) string {
	return "Validates that `customer_id` and `workspace` are required for Slack channels unless `shared` is true"
}

func (v budgetSlackChannelsValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var slackChannels types.List

	diags := req.Config.GetAttribute(ctx, path.Root("recipients_slack_channels"), &slackChannels)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || slackChannels.IsNull() || slackChannels.IsUnknown() {
		return
	}

	var channelVals []resource_budget.RecipientsSlackChannelsValue
	diags = slackChannels.ElementsAs(ctx, &channelVals, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	for i, channel := range channelVals {
		if channel.IsNull() || channel.IsUnknown() {
			continue
		}

		if !channel.Shared.IsNull() && !channel.Shared.IsUnknown() && channel.Shared.ValueBool() {
			continue
		}
		if channel.Shared.IsUnknown() {
			continue
		}

		channelPath := path.Root("recipients_slack_channels").AtListIndex(i)

		if channel.CustomerId.IsNull() || (!channel.CustomerId.IsUnknown() && channel.CustomerId.ValueString() == "") {
			resp.Diagnostics.AddAttributeError(
				channelPath.AtName("customer_id"),
				"Missing Required Attribute",
				"Attribute customer_id is required for Slack channels unless shared is true.",
			)
		}

		if channel.Workspace.IsNull() || (!channel.Workspace.IsUnknown() && channel.Workspace.ValueString() == "") {
			resp.Diagnostics.AddAttributeError(
				channelPath.AtName("workspace"),
				"Missing Required Attribute",
				"Attribute workspace is required for Slack channels unless shared is true.",
			)
		}
	}
}

// budgetSeasonalAmountsPerInterval is the number of seasonal amounts the API
// requires per time interval. Seasonal amounts are not allowed for week.
var budgetSeasonalAmountsPerInterval = map[string]int{
	"day":     7, // Monday to Sunday
	"month":   12,
	"quarter": 4,
	"year":    5, // counted from the start year
}

// budgetRolloverMaxLimit is the largest rollover limit the API accepts per
// time interval.
var budgetRolloverMaxLimit = map[string]int64{
	"day":     90,
	"week":    52,
	"month":   12,
	"quarter": 8,
	"year":    5,
}

// budgetSeasonalAmountsIntervalError returns why count seasonal amounts do not
// fit timeInterval, or "" when they do.
func budgetSeasonalAmountsIntervalError(timeInterval string, count int) string {
	want, ok := budgetSeasonalAmountsPerInterval[timeInterval]
	if !ok {
		return fmt.Sprintf("seasonal_amounts cannot be used with time_interval %q. "+
			"Use day (7 amounts), month (12), quarter (4) or year (5).", timeInterval)
	}
	if count != want {
		return fmt.Sprintf("time_interval %q requires exactly %d seasonal_amounts, one per period in calendar order. Found %d.",
			timeInterval, want, count)
	}
	return ""
}

// budgetRolloverLimitError returns why limit is not a valid limit for an
// enabled rollover with timeInterval, or "" when it is. An unrecognized
// interval only checks the minimum; budgetTimeIntervalValidator reports it.
func budgetRolloverLimitError(timeInterval string, limit int64) string {
	if limit < 1 {
		return fmt.Sprintf("rollover.limit must be at least 1 when rollover is enabled. Found %d.", limit)
	}
	if maxLimit, ok := budgetRolloverMaxLimit[timeInterval]; ok && limit > maxLimit {
		return fmt.Sprintf("rollover.limit can be at most %d for time_interval %q. Found %d.", maxLimit, timeInterval, limit)
	}
	return ""
}

// budgetSeasonalAmountsValidator rejects what the API rejects together with
// seasonal amounts: an amount (the API takes the amount of each period from
// seasonal_amounts), growth, an enabled rollover, use_prev_spend, a fixed
// budget, the week interval, and a list whose length does not fit the
// interval. An empty list means no seasonal amounts and is always valid.
type budgetSeasonalAmountsValidator struct{}

var _ resource.ConfigValidator = budgetSeasonalAmountsValidator{}

func (v budgetSeasonalAmountsValidator) Description(_ context.Context) string {
	return "Validates that seasonal_amounts is not combined with amount, growth, rollover or use_prev_spend, and fits the time interval"
}

func (v budgetSeasonalAmountsValidator) MarkdownDescription(_ context.Context) string {
	return "Validates that `seasonal_amounts` is not combined with `amount`, `allow_growth`, an enabled `rollover` " +
		"or `use_prev_spend`, and has one amount per period of `time_interval`"
}

func (v budgetSeasonalAmountsValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var seasonal types.List
	diags := req.Config.GetAttribute(ctx, path.Root("seasonal_amounts"), &seasonal)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	if seasonal.IsNull() || seasonal.IsUnknown() {
		return
	}
	count := len(seasonal.Elements())
	if count == 0 {
		return
	}

	validateBudgetSeasonalAmount(ctx, req, resp)
	validateBudgetSeasonalUsePrevSpend(ctx, req, resp)
	validateBudgetSeasonalAllowGrowth(ctx, req, resp)
	validateBudgetSeasonalRollover(ctx, req, resp)
	validateBudgetSeasonalType(ctx, req, resp)
	validateBudgetSeasonalInterval(ctx, req, resp, count)
}

func validateBudgetSeasonalAmount(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var amount types.Float64
	diags := req.Config.GetAttribute(ctx, path.Root("amount"), &amount)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	if amount.IsNull() || amount.IsUnknown() {
		return
	}
	resp.Diagnostics.AddAttributeError(
		path.Root("amount"),
		"Invalid Attribute Combination",
		"Attribute amount cannot be set together with seasonal_amounts. The amount of each period comes from "+
			"seasonal_amounts, and the DoiT API rejects a request that has both. Remove amount; "+
			"current_period_amount shows the amount of the current period.",
	)
}

func validateBudgetSeasonalUsePrevSpend(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var usePrevSpend types.Bool
	diags := req.Config.GetAttribute(ctx, path.Root("use_prev_spend"), &usePrevSpend)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	if usePrevSpend.IsNull() || usePrevSpend.IsUnknown() {
		return
	}
	if usePrevSpend.ValueBool() {
		resp.Diagnostics.AddAttributeError(
			path.Root("use_prev_spend"),
			"Invalid Attribute Combination",
			"Attribute use_prev_spend cannot be true together with seasonal_amounts.",
		)
	}
}

func validateBudgetSeasonalAllowGrowth(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var allowGrowth types.Bool
	diags := req.Config.GetAttribute(ctx, path.Root("allow_growth"), &allowGrowth)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	if allowGrowth.IsNull() || allowGrowth.IsUnknown() {
		return
	}
	if allowGrowth.ValueBool() {
		resp.Diagnostics.AddAttributeError(
			path.Root("allow_growth"),
			"Invalid Attribute Combination",
			"Attribute allow_growth cannot be true together with seasonal_amounts.",
		)
	}
}

func validateBudgetSeasonalRollover(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var rollover resource_budget.RolloverValue
	diags := req.Config.GetAttribute(ctx, path.Root("rollover"), &rollover)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	if rollover.IsNull() || rollover.IsUnknown() {
		return
	}
	if rollover.Enabled.IsNull() || rollover.Enabled.IsUnknown() {
		return
	}
	if rollover.Enabled.ValueBool() {
		resp.Diagnostics.AddAttributeError(
			path.Root("rollover").AtName("enabled"),
			"Invalid Attribute Combination",
			"Rollover cannot be enabled together with seasonal_amounts.",
		)
	}
}

func validateBudgetSeasonalType(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var budgetType types.String
	diags := req.Config.GetAttribute(ctx, path.Root("type"), &budgetType)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	if budgetType.IsNull() || budgetType.IsUnknown() {
		return
	}
	if budgetType.ValueString() == "fixed" {
		resp.Diagnostics.AddAttributeError(
			path.Root("seasonal_amounts"),
			"Invalid Seasonal Amounts",
			"Attribute seasonal_amounts can only be set for recurring budgets.",
		)
	}
}

func validateBudgetSeasonalInterval(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse, count int) {
	var timeInterval types.String
	diags := req.Config.GetAttribute(ctx, path.Root("time_interval"), &timeInterval)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	// An omitted time_interval keeps the stored one on update; ModifyPlan
	// checks the list against it.
	if timeInterval.IsNull() || timeInterval.IsUnknown() {
		return
	}
	if msg := budgetSeasonalAmountsIntervalError(timeInterval.ValueString(), count); msg != "" {
		resp.Diagnostics.AddAttributeError(path.Root("seasonal_amounts"), "Invalid Seasonal Amounts", msg)
	}
}

// budgetRolloverValidator checks the limit of an enabled rollover: at least 1,
// and at most the maximum of the time interval.
type budgetRolloverValidator struct{}

var _ resource.ConfigValidator = budgetRolloverValidator{}

func (v budgetRolloverValidator) Description(_ context.Context) string {
	return "Validates that an enabled rollover has a limit between 1 and the maximum of the time interval"
}

func (v budgetRolloverValidator) MarkdownDescription(_ context.Context) string {
	return "Validates that an enabled `rollover` has a `limit` between 1 and the maximum of `time_interval` " +
		"(90 for day, 52 for week, 12 for month, 8 for quarter, 5 for year)"
}

func (v budgetRolloverValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var rollover resource_budget.RolloverValue
	diags := req.Config.GetAttribute(ctx, path.Root("rollover"), &rollover)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	if rollover.IsNull() || rollover.IsUnknown() {
		return
	}
	if rollover.Enabled.IsUnknown() || rollover.Limit.IsUnknown() {
		return
	}
	if rollover.Enabled.IsNull() || rollover.Limit.IsNull() {
		return
	}
	if !rollover.Enabled.ValueBool() {
		return
	}

	// The minimum does not depend on the interval.
	limit := rollover.Limit.ValueInt64()
	if msg := budgetRolloverLimitError("", limit); msg != "" {
		resp.Diagnostics.AddAttributeError(path.Root("rollover").AtName("limit"), "Invalid Rollover Limit", msg)
		return
	}

	var timeInterval types.String
	diags = req.Config.GetAttribute(ctx, path.Root("time_interval"), &timeInterval)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	// A null time_interval keeps the stored one on update; ModifyPlan checks
	// the maximum against it.
	if timeInterval.IsNull() || timeInterval.IsUnknown() {
		return
	}
	if msg := budgetRolloverLimitError(timeInterval.ValueString(), limit); msg != "" {
		resp.Diagnostics.AddAttributeError(path.Root("rollover").AtName("limit"), "Invalid Rollover Limit", msg)
	}
}

// validateBudgetSeasonalGrowth rejects seasonal amounts on a budget that would
// grow, when the configuration omits allow_growth (a configured value is
// checked by budgetSeasonalAmountsValidator). The API decides growth
// differently on create and update: create turns it on for a
// growth_per_period above 0, while an update keeps the stored allow_growth
// whatever the rate is.
func validateBudgetSeasonalGrowth(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse, isCreate bool) {
	var seasonal types.List
	diags := req.Plan.GetAttribute(ctx, path.Root("seasonal_amounts"), &seasonal)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	if seasonal.IsNull() || seasonal.IsUnknown() {
		return
	}
	if len(seasonal.Elements()) == 0 {
		return
	}

	var configAllowGrowth types.Bool
	diags = req.Config.GetAttribute(ctx, path.Root("allow_growth"), &configAllowGrowth)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	if configAllowGrowth.IsUnknown() {
		return
	}
	if !configAllowGrowth.IsNull() {
		return
	}

	if isCreate {
		validateBudgetSeasonalGrowthRate(ctx, req, resp)
		return
	}
	validateBudgetSeasonalStoredGrowth(ctx, req, resp)
}

func validateBudgetSeasonalGrowthRate(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	var growth types.Float64
	diags := req.Plan.GetAttribute(ctx, path.Root("growth_per_period"), &growth)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	if growth.IsNull() || growth.IsUnknown() {
		return
	}
	if growth.ValueFloat64() > 0 {
		resp.Diagnostics.AddAttributeError(
			path.Root("allow_growth"),
			"Invalid Attribute Combination",
			"A growth_per_period above 0 turns growth on when allow_growth is omitted, and seasonal_amounts cannot "+
				"be combined with growth. Set allow_growth = false (the rate is then stored but not applied) "+
				"or set growth_per_period to 0.",
		)
	}
}

func validateBudgetSeasonalStoredGrowth(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	var stored types.Bool
	diags := req.State.GetAttribute(ctx, path.Root("allow_growth"), &stored)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	if stored.IsNull() || stored.IsUnknown() {
		return
	}
	if stored.ValueBool() {
		resp.Diagnostics.AddAttributeError(
			path.Root("allow_growth"),
			"Invalid Attribute Combination",
			"The budget has growth turned on, and seasonal_amounts cannot be combined with growth. "+
				"Set allow_growth = false to switch it to seasonal amounts.",
		)
	}
}

// validateBudgetPlanAgainstStoredInterval checks seasonal_amounts and the
// rollover limit against the stored time_interval when an update omits it,
// since the API keeps the stored interval then. A configured interval is
// checked by the config validators.
func validateBudgetPlanAgainstStoredInterval(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	var configInterval types.String
	diags := req.Config.GetAttribute(ctx, path.Root("time_interval"), &configInterval)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	if configInterval.IsUnknown() {
		return
	}
	if !configInterval.IsNull() {
		return
	}

	var stored types.String
	diags = req.State.GetAttribute(ctx, path.Root("time_interval"), &stored)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	if stored.IsNull() || stored.IsUnknown() {
		return
	}

	validateBudgetSeasonalAmountsForInterval(ctx, req, resp, stored.ValueString())
	validateBudgetRolloverLimitForInterval(ctx, req, resp, stored.ValueString())
}

func validateBudgetSeasonalAmountsForInterval(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse, timeInterval string) {
	var seasonal types.List
	diags := req.Plan.GetAttribute(ctx, path.Root("seasonal_amounts"), &seasonal)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	if seasonal.IsNull() || seasonal.IsUnknown() {
		return
	}
	count := len(seasonal.Elements())
	if count == 0 {
		return
	}
	if msg := budgetSeasonalAmountsIntervalError(timeInterval, count); msg != "" {
		resp.Diagnostics.AddAttributeError(path.Root("seasonal_amounts"), "Invalid Seasonal Amounts", msg)
	}
}

func validateBudgetRolloverLimitForInterval(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse, timeInterval string) {
	var rollover resource_budget.RolloverValue
	diags := req.Plan.GetAttribute(ctx, path.Root("rollover"), &rollover)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	if rollover.IsNull() || rollover.IsUnknown() {
		return
	}
	if rollover.Enabled.IsUnknown() || rollover.Limit.IsUnknown() {
		return
	}
	if rollover.Enabled.IsNull() || rollover.Limit.IsNull() {
		return
	}
	if !rollover.Enabled.ValueBool() {
		return
	}
	if msg := budgetRolloverLimitError(timeInterval, rollover.Limit.ValueInt64()); msg != "" {
		resp.Diagnostics.AddAttributeError(path.Root("rollover").AtName("limit"), "Invalid Rollover Limit", msg)
	}
}

// validateBudgetSeasonalRemoval rejects an update that removes the stored
// seasonal amounts without a replacement target: the API needs amount or
// use_prev_spend = true in the same request. The planned amount cannot tell an
// omitted amount from a configured one that is not known yet (it is null when
// the removal is the only change and unknown otherwise), so the configuration
// decides, and an unknown amount or use_prev_spend defers to apply.
func validateBudgetSeasonalRemoval(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	var planned types.List
	diags := req.Plan.GetAttribute(ctx, path.Root("seasonal_amounts"), &planned)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	if planned.IsUnknown() {
		return
	}
	if !planned.IsNull() && len(planned.Elements()) > 0 {
		return
	}

	var stored types.List
	diags = req.State.GetAttribute(ctx, path.Root("seasonal_amounts"), &stored)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	if stored.IsNull() || stored.IsUnknown() {
		return
	}
	if len(stored.Elements()) == 0 {
		return
	}

	var amount types.Float64
	diags = req.Config.GetAttribute(ctx, path.Root("amount"), &amount)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	if amount.IsUnknown() {
		return
	}
	if !amount.IsNull() {
		return
	}

	var usePrevSpend types.Bool
	diags = req.Config.GetAttribute(ctx, path.Root("use_prev_spend"), &usePrevSpend)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	if usePrevSpend.IsUnknown() {
		return
	}
	// An omitted use_prev_spend is its default, false.
	if !usePrevSpend.IsNull() && usePrevSpend.ValueBool() {
		return
	}

	resp.Diagnostics.AddAttributeError(
		path.Root("seasonal_amounts"),
		"Missing Budget Amount",
		"Removing seasonal_amounts needs a replacement for the amount of each period: set amount, "+
			"or use_prev_spend = true. The DoiT API rejects removing seasonal amounts without one.",
	)
}
