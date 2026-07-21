package provider

import (
	"context"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_report"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_report"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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
