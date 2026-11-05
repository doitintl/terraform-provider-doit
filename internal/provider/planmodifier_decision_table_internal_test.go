package provider

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_allocation"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_report"
)

// valueState is how a config, state or plan value is shaped in a decision table.
type valueState string

const (
	vsNull    valueState = "null"
	vsUnknown valueState = "unknown"
	vsKnown   valueState = "known"
)

// allNullObject is a known object whose attributes are all null, whatever the
// attribute types are.
func allNullObject(ctx context.Context, t *testing.T, attrTypes map[string]attr.Type) types.Object {
	t.Helper()
	attrs := make(map[string]attr.Value, len(attrTypes))
	for name, at := range attrTypes {
		v, err := at.ValueFromTerraform(ctx, tftypes.NewValue(at.TerraformType(ctx), nil))
		if err != nil {
			t.Fatalf("null value for %s: %v", name, err)
		}
		attrs[name] = v
	}
	return types.ObjectValueMust(attrTypes, attrs)
}

// markedObject is a known object with one string (else float) attribute set
// to a value that depends on marker and every other attribute null, so two
// objects built with different markers differ and the same marker builds equal
// objects. Tables use it to tell the planned value from the prior state's.
func markedObject(ctx context.Context, t *testing.T, attrTypes map[string]attr.Type, marker int) types.Object {
	t.Helper()
	names := make([]string, 0, len(attrTypes))
	for name := range attrTypes {
		names = append(names, name)
	}
	sort.Strings(names)
	marked := ""
	for _, kind := range []attr.Type{types.StringType, types.Float64Type} {
		for _, name := range names {
			if attrTypes[name].Equal(kind) {
				marked = name
				break
			}
		}
		if marked != "" {
			break
		}
	}
	if marked == "" {
		t.Fatalf("no string or float attribute to mark in %v", names)
	}
	attrs := allNullObject(ctx, t, attrTypes).Attributes()
	if attrTypes[marked].Equal(types.StringType) {
		attrs[marked] = types.StringValue(fmt.Sprintf("marker-%d", marker))
	} else {
		attrs[marked] = types.Float64Value(float64(marker))
	}
	return types.ObjectValueMust(attrTypes, attrs)
}

func objectInState(ctx context.Context, t *testing.T, s valueState, attrTypes map[string]attr.Type) types.Object {
	t.Helper()
	switch s {
	case vsNull:
		return types.ObjectNull(attrTypes)
	case vsUnknown:
		return types.ObjectUnknown(attrTypes)
	default:
		return allNullObject(ctx, t, attrTypes)
	}
}

// TestObjectNullWhenConfigNullModifiers runs every object modifier that
// proposes null when the configuration omits the object through the same
// null/unknown/known decision table: only a null config value may change the
// plan, whatever the state (create or update) and the plan value are.
func TestObjectNullWhenConfigNullModifiers(t *testing.T) {
	ctx := t.Context()
	presentReportConfig := buildReportConfigWithMap(ctx, t, validCumulativeComparisonConfigMap(ctx, t))

	modifiers := []struct {
		name      string
		modifier  planmodifier.Object
		attrTypes map[string]attr.Type
		config    tfsdk.Config
	}{
		{"alert ignore_values_range", useNullForIgnoreValuesRange(),
			map[string]attr.Type{"lower_bound": types.Float64Type, "upper_bound": types.Float64Type}, tfsdk.Config{}},
		{"allocation value_extraction", useNullForUnknownValueExtraction(),
			resource_allocation.ValueExtractionValue{}.AttributeTypes(ctx), tfsdk.Config{}},
		{"report count", useNullForCount(), resource_report.CountValue{}.AttributeTypes(ctx), presentReportConfig},
		{"report custom_time_range", useNullForCustomTimeRange(), resource_report.CustomTimeRangeValue{}.AttributeTypes(ctx), presentReportConfig},
		{"report limit_by_change", useNullForLimitByChange(), resource_report.LimitByChangeValue{}.AttributeTypes(ctx), presentReportConfig},
		{"report metric_filter", useNullForMetricFilter(), resource_report.MetricFilterValue{}.AttributeTypes(ctx), presentReportConfig},
	}

	// The configured and the planned object are the same (a configured
	// value is planned as it is); the state holds a different one.
	const plannedMarker, storedMarker = 2, 1
	objectOf := func(t *testing.T, s valueState, attrTypes map[string]attr.Type, marker int) types.Object {
		t.Helper()
		switch s {
		case vsNull:
			return types.ObjectNull(attrTypes)
		case vsUnknown:
			return types.ObjectUnknown(attrTypes)
		default:
			return markedObject(ctx, t, attrTypes, marker)
		}
	}

	for _, m := range modifiers {
		for _, config := range []valueState{vsNull, vsUnknown, vsKnown} {
			for _, state := range []valueState{vsNull, vsKnown} {
				for _, plan := range []valueState{vsUnknown, vsKnown} {
					t.Run(m.name+"/config "+string(config)+", state "+string(state)+", plan "+string(plan), func(t *testing.T) {
						req := planmodifier.ObjectRequest{
							Config:      m.config,
							ConfigValue: objectOf(t, config, m.attrTypes, plannedMarker),
							StateValue:  objectOf(t, state, m.attrTypes, storedMarker),
							PlanValue:   objectOf(t, plan, m.attrTypes, plannedMarker),
						}
						resp := &planmodifier.ObjectResponse{PlanValue: req.PlanValue}

						m.modifier.PlanModifyObject(ctx, req, resp)

						if resp.Diagnostics.HasError() {
							t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
						}
						// Only a null config value changes the plan; any other plan stays as it was.
						want := objectOf(t, plan, m.attrTypes, plannedMarker)
						if config == vsNull {
							want = types.ObjectNull(m.attrTypes)
						}
						if !resp.PlanValue.Equal(want) {
							t.Errorf("PlanValue = %v, want %v", resp.PlanValue, want)
						}
					})
				}
			}
		}
	}
}

// forecastTestInput builds a config or plan holding only
// config.advanced_analysis.forecast, the one attribute the forecast settings
// modifier reads.
func forecastTestInput(forecast tftypes.Value) (schema.Schema, tftypes.Value) {
	forecastType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"forecast": tftypes.Bool}}
	analysisType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"advanced_analysis": forecastType}}
	rootType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"config": analysisType}}
	s := schema.Schema{Attributes: map[string]schema.Attribute{
		"config": schema.SingleNestedAttribute{Optional: true, Attributes: map[string]schema.Attribute{
			"advanced_analysis": schema.SingleNestedAttribute{Optional: true, Attributes: map[string]schema.Attribute{
				"forecast": schema.BoolAttribute{Optional: true},
			}},
		}},
	}}
	raw := tftypes.NewValue(rootType, map[string]tftypes.Value{
		"config": tftypes.NewValue(analysisType, map[string]tftypes.Value{
			"advanced_analysis": tftypes.NewValue(forecastType, map[string]tftypes.Value{"forecast": forecast}),
		}),
	})
	return s, raw
}

func TestUseNullOrDefaultForForecastSettings(t *testing.T) {
	ctx := t.Context()
	attrTypes := resource_report.ForecastSettingsValue{}.AttributeTypes(ctx)
	boolValue := func(b bool) tftypes.Value { return tftypes.NewValue(tftypes.Bool, b) }
	nullBool := tftypes.NewValue(tftypes.Bool, nil)
	unknownBool := tftypes.NewValue(tftypes.Bool, tftypes.UnknownValue)

	// configuredForecastSettings is what a configuration that sets
	// forecast_settings plans: a known object with its own mode.
	configuredForecastSettings := func() types.Object {
		attrs := map[string]attr.Value{}
		for name, at := range attrTypes {
			v, err := at.ValueFromTerraform(ctx, tftypes.NewValue(at.TerraformType(ctx), nil))
			if err != nil {
				t.Fatalf("null value for %s: %v", name, err)
			}
			attrs[name] = v
		}
		attrs["mode"] = types.StringValue("grouping")
		return types.ObjectValueMust(attrTypes, attrs)
	}
	configValueFor := func(s valueState) types.Object {
		if s == vsKnown {
			return configuredForecastSettings()
		}
		return objectInState(ctx, t, s, attrTypes)
	}

	const (
		wantNull    = "null"
		wantUnknown = "unknown"
		wantTotals  = "totals default"
		wantAsPlan  = "plan untouched"
	)

	tests := []struct {
		name           string
		configValue    valueState // config.forecast_settings
		configForecast tftypes.Value
		planForecast   tftypes.Value
		missingPath    bool // schema has no config.advanced_analysis.forecast
		want           string
	}{
		{name: "forecast_settings configured", configValue: vsKnown, configForecast: boolValue(true), planForecast: boolValue(true), want: wantAsPlan},
		{name: "forecast_settings unknown", configValue: vsUnknown, configForecast: boolValue(true), planForecast: boolValue(true), want: wantUnknown},
		{name: "forecast omitted", configValue: vsNull, configForecast: nullBool, planForecast: nullBool, want: wantNull},
		{name: "forecast omitted, plan unknown", configValue: vsNull, configForecast: nullBool, planForecast: unknownBool, want: wantNull},
		{name: "forecast true", configValue: vsNull, configForecast: boolValue(true), planForecast: boolValue(true), want: wantTotals},
		{name: "forecast false", configValue: vsNull, configForecast: boolValue(false), planForecast: boolValue(false), want: wantNull},
		{name: "forecast unknown in config and plan", configValue: vsNull, configForecast: unknownBool, planForecast: unknownBool, want: wantUnknown},
		{name: "forecast known in config, plan null", configValue: vsNull, configForecast: boolValue(true), planForecast: nullBool, want: wantNull},
		{name: "forecast attribute missing from the config schema", configValue: vsNull, missingPath: true, want: wantNull},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg tfsdk.Config
			var plan tfsdk.Plan
			if tt.missingPath {
				s := schema.Schema{Attributes: map[string]schema.Attribute{"other": schema.StringAttribute{Optional: true}}}
				raw := tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{"other": tftypes.String}},
					map[string]tftypes.Value{"other": tftypes.NewValue(tftypes.String, nil)})
				cfg = tfsdk.Config{Schema: s, Raw: raw}
				plan = tfsdk.Plan{Schema: s, Raw: raw}
			} else {
				s, cfgRaw := forecastTestInput(tt.configForecast)
				_, planRaw := forecastTestInput(tt.planForecast)
				cfg = tfsdk.Config{Schema: s, Raw: cfgRaw}
				plan = tfsdk.Plan{Schema: s, Raw: planRaw}
			}

			// With forecast_settings configured the framework plans the configured
			// object, so the plan is known there; elsewhere it is unknown.
			planValue := types.ObjectUnknown(attrTypes)
			if tt.configValue == vsKnown {
				planValue = configuredForecastSettings()
			}
			req := planmodifier.ObjectRequest{
				Config:      cfg,
				Plan:        plan,
				ConfigValue: configValueFor(tt.configValue),
				PlanValue:   planValue,
			}
			resp := &planmodifier.ObjectResponse{PlanValue: planValue}

			useNullOrDefaultForForecastSettings().PlanModifyObject(ctx, req, resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
			}
			switch tt.want {
			case wantNull:
				if !resp.PlanValue.IsNull() {
					t.Errorf("PlanValue = %v, want null", resp.PlanValue)
				}
			case wantUnknown:
				if !resp.PlanValue.IsUnknown() {
					t.Errorf("PlanValue = %v, want unknown", resp.PlanValue)
				}
			case wantAsPlan:
				// A fresh object, not the one handed to the modifier.
				if want := configuredForecastSettings(); !resp.PlanValue.Equal(want) {
					t.Errorf("PlanValue = %v, want the configured object %v", resp.PlanValue, want)
				}
			case wantTotals:
				if resp.PlanValue.IsNull() || resp.PlanValue.IsUnknown() {
					t.Fatalf("PlanValue = %v, want the default object", resp.PlanValue)
				}
				got := resp.PlanValue.Attributes()
				if !got["mode"].Equal(types.StringValue("totals")) {
					t.Errorf("mode = %v, want totals", got["mode"])
				}
				for _, name := range []string{"future_time_intervals", "historical_time_intervals", "future_custom_date_range", "historical_custom_date_range"} {
					if !got[name].IsNull() {
						t.Errorf("%s = %v, want null", name, got[name])
					}
				}
			}
		})
	}
}

func TestUseEmptyForUnconfiguredMetricsMirror(t *testing.T) {
	ctx := t.Context()
	elemType := resource_report.MetricsValue{}.Type(ctx)
	empty := func() types.List { return types.ListValueMust(elemType, []attr.Value{}) }
	known := func() types.List {
		return types.ListValueMust(elemType, []attr.Value{resource_report.NewMetricsValueNull()})
	}
	listInState := func(s valueState) types.List {
		switch s {
		case vsNull:
			return types.ListNull(elemType)
		case vsUnknown:
			return types.ListUnknown(elemType)
		default:
			return known()
		}
	}

	for _, config := range []valueState{vsNull, vsUnknown, vsKnown} {
		for _, state := range []valueState{vsNull, vsKnown} {
			for _, plan := range []valueState{vsNull, vsUnknown, vsKnown} {
				t.Run("config "+string(config)+", state "+string(state)+", plan "+string(plan), func(t *testing.T) {
					req := planmodifier.ListRequest{
						ConfigValue: listInState(config),
						StateValue:  listInState(state),
						PlanValue:   listInState(plan),
					}
					resp := &planmodifier.ListResponse{PlanValue: req.PlanValue}

					useEmptyForUnconfiguredMetricsMirror().PlanModifyList(ctx, req, resp)

					if resp.Diagnostics.HasError() {
						t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
					}
					// Only a null config value is replaced, on create and update alike.
					want := listInState(plan)
					if config == vsNull {
						want = empty()
					}
					if !resp.PlanValue.Equal(want) {
						t.Errorf("PlanValue = %v, want %v", resp.PlanValue, want)
					}
				})
			}
		}
	}
}

func TestRequiresReplaceWhenCleared(t *testing.T) {
	ctx := t.Context()
	s := schema.Schema{Attributes: map[string]schema.Attribute{
		"id":    schema.StringAttribute{Computed: true},
		"thing": schema.StringAttribute{Optional: true},
	}}
	rootType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"id": tftypes.String, "thing": tftypes.String}}
	raw := func(thing valueState) tftypes.Value {
		var v tftypes.Value
		switch thing {
		case vsNull:
			v = tftypes.NewValue(tftypes.String, nil)
		case vsUnknown:
			v = tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
		default:
			v = tftypes.NewValue(tftypes.String, "x")
		}
		return tftypes.NewValue(rootType, map[string]tftypes.Value{"id": tftypes.NewValue(tftypes.String, "id-1"), "thing": v})
	}
	thing := path.Root("thing")

	// Only a value stored in state that the config no longer sets forces a replacement.
	tests := []struct {
		config, state valueState
		want          bool
	}{
		{config: vsNull, state: vsKnown, want: true},
		{config: vsNull, state: vsNull, want: false},
		{config: vsKnown, state: vsKnown, want: false},
		{config: vsKnown, state: vsNull, want: false},
		{config: vsUnknown, state: vsKnown, want: false},
		{config: vsUnknown, state: vsNull, want: false},
	}
	for _, tt := range tests {
		t.Run("config "+string(tt.config)+", state "+string(tt.state), func(t *testing.T) {
			req := resource.ModifyPlanRequest{
				Config: tfsdk.Config{Schema: s, Raw: raw(tt.config)},
				State:  tfsdk.State{Schema: s, Raw: raw(tt.state)},
			}
			resp := &resource.ModifyPlanResponse{}

			requiresReplaceWhenCleared[types.String](ctx, req, resp, thing)

			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
			}
			if !tt.want {
				if len(resp.RequiresReplace) != 0 {
					t.Errorf("RequiresReplace = %v, want none", resp.RequiresReplace)
				}
				return
			}
			if len(resp.RequiresReplace) != 1 || !resp.RequiresReplace[0].Equal(thing) {
				t.Errorf("RequiresReplace = %v, want exactly [%v]", resp.RequiresReplace, thing)
			}
		})
	}
}
