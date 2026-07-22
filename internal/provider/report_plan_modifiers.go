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
			defaultObj, diags := types.ObjectValue(attrTypes, attrs)
			resp.Diagnostics.Append(diags...)
			if !resp.Diagnostics.HasError() {
				resp.PlanValue = defaultObj
			}
			return
		}

		resp.PlanValue = req.ConfigValue
	}
}

func UseNullWhenOmitted() planmodifier.Object {
	return useNullWhenOmittedModifier{}
}

type useNullWhenOmittedModifier struct{}

func (m useNullWhenOmittedModifier) Description(_ context.Context) string {
	return "Proposes a Null object when the configuration value is null (omitted)."
}

func (m useNullWhenOmittedModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m useNullWhenOmittedModifier) PlanModifyObject(ctx context.Context, req planmodifier.ObjectRequest, resp *planmodifier.ObjectResponse) {
	if req.ConfigValue.IsNull() {
		resp.PlanValue = req.ConfigValue
	}
}
