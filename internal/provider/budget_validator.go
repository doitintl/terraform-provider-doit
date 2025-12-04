package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ validator.Int64 = budgetStartPeriodValidator{}

type budgetStartPeriodValidator struct{}

func (v budgetStartPeriodValidator) Description(ctx context.Context) string {
	return "Ensures that the start_period is at the beginning of the period for recurring budgets."
}

func (v budgetStartPeriodValidator) MarkdownDescription(ctx context.Context) string {
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

func (v budgetTimeIntervalValidator) Description(ctx context.Context) string {
	return "Ensures that the time_interval is one of: day, week, month, quarter, year."
}

func (v budgetTimeIntervalValidator) MarkdownDescription(ctx context.Context) string {
	return "Ensures that the time_interval is one of: day, week, month, quarter, year."
}

func (v budgetTimeIntervalValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
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
