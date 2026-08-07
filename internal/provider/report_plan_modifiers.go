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
// nested object whenever the config value is null.
//
// This is required — not merely cosmetic — for Optional+Computed nested objects
// whose children are Required, when the API populates the object even though the
// practitioner never configured it (config.metric / config.metrics are mirrors of
// each other and the API always returns both).
//
// Terraform Core's objchange.proposedNewAttributes contains:
//
//	case attr.Computed && configV.IsNull():
//	    newV = priorV
//	    if optionalValueNotComputable(attr, priorV) {
//	        newV = configV // null
//	    }
//
// optionalValueNotComputable reports true when the attribute is Optional, has a
// NestedType, and any non-null descendant of the PRIOR value maps to a schema
// attribute with Computed:false. Core reads that as "the practitioner must have
// configured this before and has now removed it" and proposes null — which no
// longer matches the prior state, so the plan is non-empty. The framework then
// marks every config-null Computed attribute without a Default as unknown
// (MarkComputedNilsAsUnknown), cascading into a whole-resource "known after
// apply" diff that never converges.
//
// Keeping the object null in state when the practitioner did not configure it
// removes the non-null non-computed descendants Core keys off, so Core preserves
// prior (null == null) and the plan stays empty. The Read path must agree — see
// the state-aware handling in mapReportToModel.
//
// This is deliberately NOT named use{Empty,Null}ForUnknown*WhenConfigNull: it is
// not a clearing modifier and clearableattr must not classify it as Category A.
// Removing the attribute from config does not clear anything server-side — the
// API derives the mirror and always keeps it — so there is no null to send in
// the request builder. config.metric stays Category B (acknowledgeNotClearable).
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
// useNullForUnconfiguredMetricMirror. It proposes an empty list whenever
// config.metrics is null, keeping the API-echoed mirror out of state for the
// same reason (see that modifier for the Core mechanism).
//
// It deliberately does NOT reuse useNullForUnknownListWhenConfigNull: that one is
// guarded on !req.StateValue.IsNull() and therefore does not fire on Create, so
// the API echo would still reach state on the very first apply and poison the
// next plan. Empty list (rather than null) keeps the repo's "user-configurable
// lists are [] not null" convention that the Read path and listnullread expect.
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
