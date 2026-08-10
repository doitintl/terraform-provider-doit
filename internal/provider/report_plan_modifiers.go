package provider

import (
	"context"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_report"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func useNullOrDefaultForForecastSettings() planmodifier.Object {
	return useNullOrDefaultForForecastSettingsModifier{}
}

type useNullOrDefaultForForecastSettingsModifier struct{}

func (m useNullOrDefaultForForecastSettingsModifier) Description(_ context.Context) string {
	return "Proposes a default object with totals mode if forecast is enabled and forecast_settings is null, otherwise proposes null."
}

func (m useNullOrDefaultForForecastSettingsModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m useNullOrDefaultForForecastSettingsModifier) PlanModifyObject(ctx context.Context, req planmodifier.ObjectRequest, resp *planmodifier.ObjectResponse) {
	if req.ConfigValue.IsNull() {
		var configForecast types.Bool
		_ = req.Config.GetAttribute(ctx, path.Root("config").AtName("advanced_analysis").AtName("forecast"), &configForecast)

		if configForecast.IsNull() {
			resp.PlanValue = req.ConfigValue
			return
		}

		var forecast types.Bool
		diags := req.Plan.GetAttribute(ctx, path.Root("config").AtName("advanced_analysis").AtName("forecast"), &forecast)
		resp.Diagnostics.Append(diags...)
		if diags.HasError() {
			return
		}

		if forecast.IsUnknown() {
			attrTypes := resource_report.ForecastSettingsValue{}.AttributeTypes(ctx)
			resp.PlanValue = types.ObjectUnknown(attrTypes)
			return
		}

		if !forecast.IsNull() && forecast.ValueBool() {
			attrTypes := resource_report.ForecastSettingsValue{}.AttributeTypes(ctx)
			attrs := map[string]attr.Value{
				"future_custom_date_range":     resource_report.NewFutureCustomDateRangeValueNull(),
				"future_time_intervals":        types.Int64Null(),
				"historical_custom_date_range": resource_report.NewHistoricalCustomDateRangeValueNull(),
				"historical_time_intervals":    types.Int64Null(),
				"mode":                         types.StringValue("totals"),
			}
			fsVal, diags := resource_report.NewForecastSettingsValue(attrTypes, attrs)
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}
			defaultObj, diags := fsVal.ToObjectValue(ctx)
			resp.Diagnostics.Append(diags...)
			if !resp.Diagnostics.HasError() {
				resp.PlanValue = defaultObj
			}
			return
		}

		resp.PlanValue = req.ConfigValue
	}
}

// useNullForUnconfiguredMetricMirror proposes null for an Optional+Computed
// nested object whenever the config value is null, keeping an API-populated
// object out of state when the practitioner did not configure it.
//
// Required for Optional+Computed objects with Required children: Terraform Core
// (objchange.optionalValueNotComputable) proposes null for such an attribute when
// its prior value holds any non-null non-computed descendant, which no longer
// matches prior state. The framework then marks every config-null Computed
// attribute without a Default as unknown, producing a whole-resource
// "known after apply" diff that never converges. Holding null in state instead
// removes the descendants Core keys off. The Read path must agree — see
// mapReportToModel.
//
// Not named use{Empty,Null}ForUnknown*WhenConfigNull on purpose: clearableattr
// classifies by that naming, and this does not clear anything server-side, so
// there is no null to send in the request builder.
func useNullForUnconfiguredMetricMirror() planmodifier.Object {
	return useNullForUnconfiguredMetricMirrorModifier{}
}

type useNullForUnconfiguredMetricMirrorModifier struct{}

func (m useNullForUnconfiguredMetricMirrorModifier) Description(_ context.Context) string {
	return "Proposes null when the config value is null, so an API-populated object is not stored for an unconfigured attribute."
}

func (m useNullForUnconfiguredMetricMirrorModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m useNullForUnconfiguredMetricMirrorModifier) PlanModifyObject(_ context.Context, req planmodifier.ObjectRequest, resp *planmodifier.ObjectResponse) {
	if req.ConfigValue.IsNull() {
		resp.PlanValue = req.ConfigValue
	}
}

// useEmptyForUnconfiguredMetricsMirror is the config.metrics counterpart of
// useNullForUnconfiguredMetricMirror; see that modifier for the mechanism.
//
// Proposes an empty list rather than null to keep the "user-configurable lists
// are [] not null" convention the Read path and listnullread expect. It cannot
// reuse useNullForUnknownListWhenConfigNull, which is guarded on
// !req.StateValue.IsNull() and so does not fire on Create.
func useEmptyForUnconfiguredMetricsMirror() planmodifier.List {
	return useEmptyForUnconfiguredMetricsMirrorModifier{}
}

type useEmptyForUnconfiguredMetricsMirrorModifier struct{}

func (m useEmptyForUnconfiguredMetricsMirrorModifier) Description(_ context.Context) string {
	return "Proposes an empty list when config.metrics is null, so the API-echoed metrics mirror is not stored for an unconfigured attribute."
}

func (m useEmptyForUnconfiguredMetricsMirrorModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m useEmptyForUnconfiguredMetricsMirrorModifier) PlanModifyList(ctx context.Context, req planmodifier.ListRequest, resp *planmodifier.ListResponse) {
	if !req.ConfigValue.IsNull() {
		return
	}
	emptyList, diags := types.ListValue(resource_report.MetricsValue{}.Type(ctx), []attr.Value{})
	resp.Diagnostics.Append(diags...)
	if !resp.Diagnostics.HasError() {
		resp.PlanValue = emptyList
	}
}
