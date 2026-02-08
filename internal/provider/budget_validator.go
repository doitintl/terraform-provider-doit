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

type budgetStartPeriodValidator struct{}

func (v budgetStartPeriodValidator) Description(_ context.Context) string {
	return "Ensures that the start_period is at the beginning of the period for recurring budgets."
}

func (v budgetStartPeriodValidator) MarkdownDescription(_ context.Context) string {
	return "Ensures that the start_period is at the beginning of the period for recurring budgets."
}

func (v budgetStartPeriodValidator) ValidateInt64(ctx context.Context, req validator.Int64Request, resp *validator.Int64Response) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
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

	startPeriodMs := req.ConfigValue.ValueInt64()

	if err := validateBudgetStartPeriod(budgetType.ValueString(), timeInterval.ValueString(), startPeriodMs); err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Budget Start Period",
			err.Error(),
		)
	}
}

func validateBudgetStartPeriod(budgetType, timeInterval string, startPeriodMs int64) error {
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

	// If type is "recurring" and end_period is set, that's an error
	if !budgetType.IsNull() && !budgetType.IsUnknown() && budgetType.ValueString() == "recurring" {
		if !endPeriod.IsNull() && !endPeriod.IsUnknown() {
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
	if !budgetType.IsNull() && !budgetType.IsUnknown() && budgetType.ValueString() == "fixed" {
		if endPeriod.IsNull() || endPeriod.IsUnknown() {
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

// budgetScopeMutuallyExclusiveValidator validates that exactly one of 'scope' or 'scopes' is set.
type budgetScopeMutuallyExclusiveValidator struct{}

func (v budgetScopeMutuallyExclusiveValidator) Description(_ context.Context) string {
	return "Validates that exactly one of 'scope' or 'scopes' is set"
}

func (v budgetScopeMutuallyExclusiveValidator) MarkdownDescription(_ context.Context) string {
	return "Validates that exactly one of `scope` or `scopes` is set"
}

func (v budgetScopeMutuallyExclusiveValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var scope types.List
	var scopes types.List

	// Get the attributes
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("scope"), &scope)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("scopes"), &scopes)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if scope.IsUnknown() || scopes.IsUnknown() {
		return
	}

	hasScope := !scope.IsNull()
	hasScopes := !scopes.IsNull()

	if hasScope && hasScopes {
		resp.Diagnostics.AddError(
			"Invalid Attribute Combination",
			"Attributes 'scope' and 'scopes' are mutually exclusive. Please specify only one.",
		)
	}

	if !hasScope && !hasScopes {
		resp.Diagnostics.AddError(
			"Missing Required Attribute",
			"One of 'scope' or 'scopes' must be specified.",
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

// budgetCollaboratorsOwnerValidator validates that collaborators list contains exactly one owner.
// Empty collaborators list is not allowed - the API requires exactly one owner.
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

	// Empty list is not allowed - API requires exactly one owner
	if len(collaborators.Elements()) == 0 {
		resp.Diagnostics.AddAttributeError(
			path.Root("collaborators"),
			"Exactly One Owner Required",
			"The 'collaborators' attribute cannot be empty. "+
				"When setting collaborators explicitly, exactly one collaborator with role 'owner' is required. "+
				"If you want to use the default (creator as owner), omit the collaborators attribute entirely.",
		)
		return
	}

	// Count owners in the collaborators list
	ownerCount := 0
	for _, elem := range collaborators.Elements() {
		// Use the generated CollaboratorsValue type
		collabVal, ok := elem.(resource_budget.CollaboratorsValue)
		if !ok {
			continue
		}

		// Skip if the element is null or unknown
		if collabVal.IsNull() || collabVal.IsUnknown() {
			continue
		}

		// Check role
		if collabVal.Role.IsNull() || collabVal.Role.IsUnknown() {
			continue
		}

		if collabVal.Role.ValueString() == "owner" {
			ownerCount++
		}
	}

	if ownerCount == 0 {
		resp.Diagnostics.AddAttributeError(
			path.Root("collaborators"),
			"Exactly One Owner Required",
			"The 'collaborators' list must contain exactly one collaborator with role 'owner'. "+
				"Found 0 owners. Add a collaborator with role = \"owner\".",
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
