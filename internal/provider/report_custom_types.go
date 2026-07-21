package provider

import (
	"context"
	"maps"

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

	attrs := make(map[string]attr.Value)
	maps.Copy(attrs, in.Attributes())

	if fcdr, ok := attrs["future_custom_date_range"]; ok {
		if objVal, ok := fcdr.(basetypes.ObjectValue); ok {
			if objVal.IsNull() {
				attrs["future_custom_date_range"] = resource_report.NewFutureCustomDateRangeValueNull()
			} else if objVal.IsUnknown() {
				attrs["future_custom_date_range"] = resource_report.NewFutureCustomDateRangeValueUnknown()
			} else if _, isTyped := fcdr.(resource_report.FutureCustomDateRangeValue); !isTyped {
				safeType := SafeFutureCustomDateRangeType{}
				converted, cDiags := safeType.ValueFromObject(ctx, objVal)
				diags.Append(cDiags...)
				if !cDiags.HasError() {
					attrs["future_custom_date_range"] = converted
				}
			}
		}
	}

	if hcdr, ok := attrs["historical_custom_date_range"]; ok {
		if objVal, ok := hcdr.(basetypes.ObjectValue); ok {
			if objVal.IsNull() {
				attrs["historical_custom_date_range"] = resource_report.NewHistoricalCustomDateRangeValueNull()
			} else if objVal.IsUnknown() {
				attrs["historical_custom_date_range"] = resource_report.NewHistoricalCustomDateRangeValueUnknown()
			} else if _, isTyped := hcdr.(resource_report.HistoricalCustomDateRangeValue); !isTyped {
				safeType := SafeHistoricalCustomDateRangeType{}
				converted, cDiags := safeType.ValueFromObject(ctx, objVal)
				diags.Append(cDiags...)
				if !cDiags.HasError() {
					attrs["historical_custom_date_range"] = converted
				}
			}
		}
	}

	if diags.HasError() {
		return nil, diags
	}

	safeIn, sDiags := types.ObjectValue(in.AttributeTypes(ctx), attrs)
	diags.Append(sDiags...)
	if diags.HasError() {
		return nil, diags
	}

	return t.ForecastSettingsType.ValueFromObject(ctx, safeIn)
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

type SafeFutureCustomDateRangeTypeDataSource struct {
	datasource_report.FutureCustomDateRangeType
}

func (t SafeFutureCustomDateRangeTypeDataSource) Equal(o attr.Type) bool {
	other, ok := o.(SafeFutureCustomDateRangeTypeDataSource)
	if !ok {
		if baseOther, ok := o.(datasource_report.FutureCustomDateRangeType); ok {
			return t.FutureCustomDateRangeType.Equal(baseOther)
		}
		return false
	}
	return t.FutureCustomDateRangeType.Equal(other.FutureCustomDateRangeType)
}

func (t SafeFutureCustomDateRangeTypeDataSource) ValueFromObject(ctx context.Context, in basetypes.ObjectValue) (basetypes.ObjectValuable, diag.Diagnostics) {
	var diags diag.Diagnostics

	if in.IsNull() {
		return datasource_report.NewFutureCustomDateRangeValueNull(), diags
	}

	if in.IsUnknown() {
		return datasource_report.NewFutureCustomDateRangeValueUnknown(), diags
	}

	attrs := make(map[string]attr.Value)
	maps.Copy(attrs, in.Attributes())

	if _, ok := attrs["from"]; !ok {
		attrs["from"] = types.StringNull()
	}
	if _, ok := attrs["to"]; !ok {
		attrs["to"] = types.StringNull()
	}

	safeIn, sDiags := types.ObjectValue(in.AttributeTypes(ctx), attrs)
	diags.Append(sDiags...)
	if diags.HasError() {
		return nil, diags
	}

	return t.FutureCustomDateRangeType.ValueFromObject(ctx, safeIn)
}

type SafeHistoricalCustomDateRangeTypeDataSource struct {
	datasource_report.HistoricalCustomDateRangeType
}

func (t SafeHistoricalCustomDateRangeTypeDataSource) Equal(o attr.Type) bool {
	other, ok := o.(SafeHistoricalCustomDateRangeTypeDataSource)
	if !ok {
		if baseOther, ok := o.(datasource_report.HistoricalCustomDateRangeType); ok {
			return t.HistoricalCustomDateRangeType.Equal(baseOther)
		}
		return false
	}
	return t.HistoricalCustomDateRangeType.Equal(other.HistoricalCustomDateRangeType)
}

func (t SafeHistoricalCustomDateRangeTypeDataSource) ValueFromObject(ctx context.Context, in basetypes.ObjectValue) (basetypes.ObjectValuable, diag.Diagnostics) {
	var diags diag.Diagnostics

	if in.IsNull() {
		return datasource_report.NewHistoricalCustomDateRangeValueNull(), diags
	}

	if in.IsUnknown() {
		return datasource_report.NewHistoricalCustomDateRangeValueUnknown(), diags
	}

	attrs := make(map[string]attr.Value)
	maps.Copy(attrs, in.Attributes())

	if _, ok := attrs["from"]; !ok {
		attrs["from"] = types.StringNull()
	}
	if _, ok := attrs["to"]; !ok {
		attrs["to"] = types.StringNull()
	}

	safeIn, sDiags := types.ObjectValue(in.AttributeTypes(ctx), attrs)
	diags.Append(sDiags...)
	if diags.HasError() {
		return nil, diags
	}

	return t.HistoricalCustomDateRangeType.ValueFromObject(ctx, safeIn)
}

func (t SafeForecastSettingsTypeDataSource) ValueFromObject(ctx context.Context, in basetypes.ObjectValue) (basetypes.ObjectValuable, diag.Diagnostics) {
	var diags diag.Diagnostics

	if in.IsNull() {
		return datasource_report.NewForecastSettingsValueNull(), diags
	}

	if in.IsUnknown() {
		return datasource_report.NewForecastSettingsValueUnknown(), diags
	}

	attrs := make(map[string]attr.Value)
	maps.Copy(attrs, in.Attributes())

	if fcdr, ok := attrs["future_custom_date_range"]; ok {
		if objVal, ok := fcdr.(basetypes.ObjectValue); ok {
			if objVal.IsNull() {
				attrs["future_custom_date_range"] = datasource_report.NewFutureCustomDateRangeValueNull()
			} else if objVal.IsUnknown() {
				attrs["future_custom_date_range"] = datasource_report.NewFutureCustomDateRangeValueUnknown()
			} else if _, isTyped := fcdr.(datasource_report.FutureCustomDateRangeValue); !isTyped {
				safeType := SafeFutureCustomDateRangeTypeDataSource{}
				converted, cDiags := safeType.ValueFromObject(ctx, objVal)
				diags.Append(cDiags...)
				if !cDiags.HasError() {
					attrs["future_custom_date_range"] = converted
				}
			}
		}
	}

	if hcdr, ok := attrs["historical_custom_date_range"]; ok {
		if objVal, ok := hcdr.(basetypes.ObjectValue); ok {
			if objVal.IsNull() {
				attrs["historical_custom_date_range"] = datasource_report.NewHistoricalCustomDateRangeValueNull()
			} else if objVal.IsUnknown() {
				attrs["historical_custom_date_range"] = datasource_report.NewHistoricalCustomDateRangeValueUnknown()
			} else if _, isTyped := hcdr.(datasource_report.HistoricalCustomDateRangeValue); !isTyped {
				safeType := SafeHistoricalCustomDateRangeTypeDataSource{}
				converted, cDiags := safeType.ValueFromObject(ctx, objVal)
				diags.Append(cDiags...)
				if !cDiags.HasError() {
					attrs["historical_custom_date_range"] = converted
				}
			}
		}
	}

	if diags.HasError() {
		return nil, diags
	}

	safeIn, sDiags := types.ObjectValue(in.AttributeTypes(ctx), attrs)
	diags.Append(sDiags...)
	if diags.HasError() {
		return nil, diags
	}

	return t.ForecastSettingsType.ValueFromObject(ctx, safeIn)
}

type SafeFutureCustomDateRangeType struct {
	resource_report.FutureCustomDateRangeType
}

func (t SafeFutureCustomDateRangeType) Equal(o attr.Type) bool {
	other, ok := o.(SafeFutureCustomDateRangeType)
	if !ok {
		if baseOther, ok := o.(resource_report.FutureCustomDateRangeType); ok {
			return t.FutureCustomDateRangeType.Equal(baseOther)
		}
		return false
	}
	return t.FutureCustomDateRangeType.Equal(other.FutureCustomDateRangeType)
}

func (t SafeFutureCustomDateRangeType) ValueFromObject(ctx context.Context, in basetypes.ObjectValue) (basetypes.ObjectValuable, diag.Diagnostics) {
	var diags diag.Diagnostics

	if in.IsNull() {
		return resource_report.NewFutureCustomDateRangeValueNull(), diags
	}

	if in.IsUnknown() {
		return resource_report.NewFutureCustomDateRangeValueUnknown(), diags
	}

	attrs := make(map[string]attr.Value)
	maps.Copy(attrs, in.Attributes())

	if _, ok := attrs["from"]; !ok {
		attrs["from"] = types.StringNull()
	}
	if _, ok := attrs["to"]; !ok {
		attrs["to"] = types.StringNull()
	}

	attrTypes := map[string]attr.Type{
		"from": types.StringType,
		"to":   types.StringType,
	}

	safeIn, sDiags := types.ObjectValue(attrTypes, attrs)
	diags.Append(sDiags...)
	if diags.HasError() {
		return nil, diags
	}

	return t.FutureCustomDateRangeType.ValueFromObject(ctx, safeIn)
}

type SafeHistoricalCustomDateRangeType struct {
	resource_report.HistoricalCustomDateRangeType
}

func (t SafeHistoricalCustomDateRangeType) Equal(o attr.Type) bool {
	other, ok := o.(SafeHistoricalCustomDateRangeType)
	if !ok {
		if baseOther, ok := o.(resource_report.HistoricalCustomDateRangeType); ok {
			return t.HistoricalCustomDateRangeType.Equal(baseOther)
		}
		return false
	}
	return t.HistoricalCustomDateRangeType.Equal(other.HistoricalCustomDateRangeType)
}

func (t SafeHistoricalCustomDateRangeType) ValueFromObject(ctx context.Context, in basetypes.ObjectValue) (basetypes.ObjectValuable, diag.Diagnostics) {
	var diags diag.Diagnostics

	if in.IsNull() {
		return resource_report.NewHistoricalCustomDateRangeValueNull(), diags
	}

	if in.IsUnknown() {
		return resource_report.NewHistoricalCustomDateRangeValueUnknown(), diags
	}

	attrs := make(map[string]attr.Value)
	maps.Copy(attrs, in.Attributes())

	if _, ok := attrs["from"]; !ok {
		attrs["from"] = types.StringNull()
	}
	if _, ok := attrs["to"]; !ok {
		attrs["to"] = types.StringNull()
	}

	attrTypes := map[string]attr.Type{
		"from": types.StringType,
		"to":   types.StringType,
	}

	safeIn, sDiags := types.ObjectValue(attrTypes, attrs)
	diags.Append(sDiags...)
	if diags.HasError() {
		return nil, diags
	}

	return t.HistoricalCustomDateRangeType.ValueFromObject(ctx, safeIn)
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

type SafeCustomTimeRangeType struct {
	resource_report.CustomTimeRangeType
}

func (t SafeCustomTimeRangeType) Equal(o attr.Type) bool {
	other, ok := o.(SafeCustomTimeRangeType)
	if !ok {
		if baseOther, ok := o.(resource_report.CustomTimeRangeType); ok {
			return t.CustomTimeRangeType.Equal(baseOther)
		}
		return false
	}
	return t.CustomTimeRangeType.Equal(other.CustomTimeRangeType)
}

func (t SafeCustomTimeRangeType) ValueFromObject(ctx context.Context, in basetypes.ObjectValue) (basetypes.ObjectValuable, diag.Diagnostics) {
	var diags diag.Diagnostics

	if in.IsNull() {
		return resource_report.NewCustomTimeRangeValueNull(), diags
	}

	if in.IsUnknown() {
		return resource_report.NewCustomTimeRangeValueUnknown(), diags
	}

	attrs := make(map[string]attr.Value)
	maps.Copy(attrs, in.Attributes())

	if _, ok := attrs["from"]; !ok {
		attrs["from"] = types.StringNull()
	}
	if _, ok := attrs["to"]; !ok {
		attrs["to"] = types.StringNull()
	}

	attrTypes := map[string]attr.Type{
		"from": types.StringType,
		"to":   types.StringType,
	}

	safeIn, sDiags := types.ObjectValue(attrTypes, attrs)
	diags.Append(sDiags...)
	if diags.HasError() {
		return nil, diags
	}

	return t.CustomTimeRangeType.ValueFromObject(ctx, safeIn)
}

type SafeCustomTimeRangeTypeDataSource struct {
	datasource_report.CustomTimeRangeType
}

func (t SafeCustomTimeRangeTypeDataSource) Equal(o attr.Type) bool {
	other, ok := o.(SafeCustomTimeRangeTypeDataSource)
	if !ok {
		if baseOther, ok := o.(datasource_report.CustomTimeRangeType); ok {
			return t.CustomTimeRangeType.Equal(baseOther)
		}
		return false
	}
	return t.CustomTimeRangeType.Equal(other.CustomTimeRangeType)
}

func (t SafeCustomTimeRangeTypeDataSource) ValueFromObject(ctx context.Context, in basetypes.ObjectValue) (basetypes.ObjectValuable, diag.Diagnostics) {
	var diags diag.Diagnostics

	if in.IsNull() {
		return datasource_report.NewCustomTimeRangeValueNull(), diags
	}

	if in.IsUnknown() {
		return datasource_report.NewCustomTimeRangeValueUnknown(), diags
	}

	attrs := make(map[string]attr.Value)
	maps.Copy(attrs, in.Attributes())

	if _, ok := attrs["from"]; !ok {
		attrs["from"] = types.StringNull()
	}
	if _, ok := attrs["to"]; !ok {
		attrs["to"] = types.StringNull()
	}

	attrTypes := map[string]attr.Type{
		"from": types.StringType,
		"to":   types.StringType,
	}

	safeIn, sDiags := types.ObjectValue(attrTypes, attrs)
	diags.Append(sDiags...)
	if diags.HasError() {
		return nil, diags
	}

	return t.CustomTimeRangeType.ValueFromObject(ctx, safeIn)
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

type SafeSecondaryTimeRangeType struct {
	resource_report.SecondaryTimeRangeType
}

func (t SafeSecondaryTimeRangeType) Equal(o attr.Type) bool {
	other, ok := o.(SafeSecondaryTimeRangeType)
	if !ok {
		if baseOther, ok := o.(resource_report.SecondaryTimeRangeType); ok {
			return t.SecondaryTimeRangeType.Equal(baseOther)
		}
		return false
	}
	return t.SecondaryTimeRangeType.Equal(other.SecondaryTimeRangeType)
}

func (t SafeSecondaryTimeRangeType) ValueFromObject(ctx context.Context, in basetypes.ObjectValue) (basetypes.ObjectValuable, diag.Diagnostics) {
	var diags diag.Diagnostics

	if in.IsNull() {
		return resource_report.NewSecondaryTimeRangeValueNull(), diags
	}

	if in.IsUnknown() {
		return resource_report.NewSecondaryTimeRangeValueUnknown(), diags
	}

	attrs := make(map[string]attr.Value)
	maps.Copy(attrs, in.Attributes())

	if _, ok := attrs["amount"]; !ok {
		attrs["amount"] = types.Int64Null()
	}

	if _, ok := attrs["include_current"]; !ok {
		attrs["include_current"] = types.BoolNull()
	}

	if ctr, ok := attrs["custom_time_range"]; ok {
		if objVal, ok := ctr.(basetypes.ObjectValue); ok {
			if objVal.IsNull() {
				attrs["custom_time_range"] = resource_report.NewCustomTimeRangeValueNull()
			} else if objVal.IsUnknown() {
				attrs["custom_time_range"] = resource_report.NewCustomTimeRangeValueUnknown()
			} else if _, isTyped := ctr.(resource_report.CustomTimeRangeValue); !isTyped {
				safeType := SafeCustomTimeRangeType{}
				converted, cDiags := safeType.ValueFromObject(ctx, objVal)
				diags.Append(cDiags...)
				if !cDiags.HasError() {
					attrs["custom_time_range"] = converted
				}
			}
		}
	} else {
		attrs["custom_time_range"] = resource_report.NewCustomTimeRangeValueNull()
	}

	attrTypes := map[string]attr.Type{
		"amount":          types.Int64Type,
		"include_current": types.BoolType,
		"custom_time_range": resource_report.CustomTimeRangeType{
			ObjectType: types.ObjectType{
				AttrTypes: resource_report.CustomTimeRangeValue{}.AttributeTypes(ctx),
			},
		},
	}

	safeIn, sDiags := types.ObjectValue(attrTypes, attrs)
	diags.Append(sDiags...)
	if diags.HasError() {
		return nil, diags
	}

	return t.SecondaryTimeRangeType.ValueFromObject(ctx, safeIn)
}
