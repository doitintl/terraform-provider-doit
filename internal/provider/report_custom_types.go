package provider

import (
	"context"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_report"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_report"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

type SafeForecastSettingsType struct {
	resource_report.ForecastSettingsType
}

func (t SafeForecastSettingsType) Equal(o attr.Type) bool {
	other, ok := o.(SafeForecastSettingsType)
	if !ok {
		if baseOther, ok := o.(resource_report.ForecastSettingsType); ok {
			return t.ForecastSettingsType.Equal(baseOther)
		}
		return false
	}
	return t.ForecastSettingsType.Equal(other.ForecastSettingsType)
}

func (t SafeForecastSettingsType) ValueFromObject(ctx context.Context, in basetypes.ObjectValue) (basetypes.ObjectValuable, diag.Diagnostics) {
	var diags diag.Diagnostics

	if in.IsNull() {
		return resource_report.NewForecastSettingsValueNull(), diags
	}

	if in.IsUnknown() {
		return resource_report.NewForecastSettingsValueUnknown(), diags
	}

	return t.ForecastSettingsType.ValueFromObject(ctx, in)
}

type SafeForecastSettingsTypeDataSource struct {
	datasource_report.ForecastSettingsType
}

func (t SafeForecastSettingsTypeDataSource) Equal(o attr.Type) bool {
	other, ok := o.(SafeForecastSettingsTypeDataSource)
	if !ok {
		if baseOther, ok := o.(datasource_report.ForecastSettingsType); ok {
			return t.ForecastSettingsType.Equal(baseOther)
		}
		return false
	}
	return t.ForecastSettingsType.Equal(other.ForecastSettingsType)
}

func (t SafeForecastSettingsTypeDataSource) ValueFromObject(ctx context.Context, in basetypes.ObjectValue) (basetypes.ObjectValuable, diag.Diagnostics) {
	var diags diag.Diagnostics

	if in.IsNull() {
		return datasource_report.NewForecastSettingsValueNull(), diags
	}

	if in.IsUnknown() {
		return datasource_report.NewForecastSettingsValueUnknown(), diags
	}

	return t.ForecastSettingsType.ValueFromObject(ctx, in)
}

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
		var forecast types.Bool
		diags := req.Plan.GetAttribute(ctx, path.Root("config").AtName("advanced_analysis").AtName("forecast"), &forecast)
		resp.Diagnostics.Append(diags...)
		if diags.HasError() {
			return
		}

		if !forecast.IsNull() && !forecast.IsUnknown() && forecast.ValueBool() {
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
