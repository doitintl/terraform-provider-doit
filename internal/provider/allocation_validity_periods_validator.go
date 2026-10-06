package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_allocation"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// allocationValidityPeriodsValidator validates constraints on allocation validity_periods:
//   - Only the first period in the list may omit start_date.
//   - Only the last period in the list may omit end_date.
//   - Within each period, start_date must be on or before end_date.
//   - Between consecutive periods, each period's start_date must be after the previous period's end_date.
var _ validator.List = allocationValidityPeriodsValidator{}

type allocationValidityPeriodsValidator struct{}

func (v allocationValidityPeriodsValidator) Description(_ context.Context) string {
	return "validates that validity_periods date ranges are ordered, non-overlapping, and only omit start_date on the first period and end_date on the last period"
}

func (v allocationValidityPeriodsValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v allocationValidityPeriodsValidator) ValidateList(ctx context.Context, req validator.ListRequest, resp *validator.ListResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	elements := req.ConfigValue.Elements()
	n := len(elements)
	if n == 0 {
		return
	}

	for i, elem := range elements {
		periodVal, ok := elem.(resource_allocation.ValidityPeriodsValue)
		if !ok {
			continue
		}

		if periodVal.IsNull() || periodVal.IsUnknown() {
			continue
		}

		if i > 0 {
			validatePeriodStartDate(req.Path, i, periodVal.StartDate, resp)
		}

		if i < n-1 {
			validatePeriodEndDate(req.Path, i, periodVal.EndDate, resp)
		}

		validatePeriodDateOrder(req.Path, i, periodVal.StartDate, periodVal.EndDate, resp)
	}

	for i := 0; i < n-1; i++ {
		currentVal, okCurr := elements[i].(resource_allocation.ValidityPeriodsValue)
		nextVal, okNext := elements[i+1].(resource_allocation.ValidityPeriodsValue)
		if !okCurr || !okNext {
			continue
		}

		if currentVal.IsNull() || currentVal.IsUnknown() || nextVal.IsNull() || nextVal.IsUnknown() {
			continue
		}

		validateConsecutivePeriodOrder(req.Path, i+1, currentVal.EndDate, nextVal.StartDate, resp)
	}
}

func validatePeriodStartDate(attrPath path.Path, index int, startDate basetypes.StringValue, resp *validator.ListResponse) {
	if startDate.IsUnknown() {
		return
	}
	if startDate.IsNull() || startDate.ValueString() == "" {
		resp.Diagnostics.AddAttributeError(
			attrPath.AtListIndex(index).AtName("start_date"),
			"Missing Required Attribute",
			fmt.Sprintf("validity_periods[%d]: only the first period in the list may omit start_date", index),
		)
	}
}

func validatePeriodEndDate(attrPath path.Path, index int, endDate basetypes.StringValue, resp *validator.ListResponse) {
	if endDate.IsUnknown() {
		return
	}
	if endDate.IsNull() || endDate.ValueString() == "" {
		resp.Diagnostics.AddAttributeError(
			attrPath.AtListIndex(index).AtName("end_date"),
			"Missing Required Attribute",
			fmt.Sprintf("validity_periods[%d]: only the last period in the list may omit end_date", index),
		)
	}
}

func validatePeriodDateOrder(attrPath path.Path, index int, startDate, endDate basetypes.StringValue, resp *validator.ListResponse) {
	if startDate.IsUnknown() || endDate.IsUnknown() {
		return
	}
	if !startDate.IsNull() && !endDate.IsNull() && startDate.ValueString() != "" && endDate.ValueString() != "" {
		startDateStr := startDate.ValueString()
		endDateStr := endDate.ValueString()
		startTime, startErr := time.Parse(time.DateOnly, startDateStr)
		endTime, endErr := time.Parse(time.DateOnly, endDateStr)
		if startErr == nil && endErr == nil {
			if startTime.After(endTime) {
				resp.Diagnostics.AddAttributeError(
					attrPath.AtListIndex(index),
					"Invalid Validity Period",
					fmt.Sprintf("validity_periods[%d]: start_date (%s) must be on or before end_date (%s)", index, startDateStr, endDateStr),
				)
			}
		}
	}
}

func validateConsecutivePeriodOrder(attrPath path.Path, nextIndex int, currentEnd, nextStart basetypes.StringValue, resp *validator.ListResponse) {
	if currentEnd.IsUnknown() || nextStart.IsUnknown() {
		return
	}
	if !currentEnd.IsNull() && !nextStart.IsNull() && currentEnd.ValueString() != "" && nextStart.ValueString() != "" {
		currentEndStr := currentEnd.ValueString()
		nextStartStr := nextStart.ValueString()
		currEndTime, currErr := time.Parse(time.DateOnly, currentEndStr)
		nextStartTime, nextErr := time.Parse(time.DateOnly, nextStartStr)
		if currErr == nil && nextErr == nil {
			if !currEndTime.Before(nextStartTime) {
				resp.Diagnostics.AddAttributeError(
					attrPath.AtListIndex(nextIndex).AtName("start_date"),
					"Invalid Validity Period Range",
					fmt.Sprintf("validity_periods[%d]: start_date (%s) must be after previous period's end_date (%s)", nextIndex, nextStartStr, currentEndStr),
				)
			}
		}
	}
}
