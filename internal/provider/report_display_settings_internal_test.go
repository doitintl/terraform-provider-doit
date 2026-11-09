package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	resource_report "github.com/doitintl/terraform-provider-doit/internal/provider/resource_report"
)

// chartOptionBools holds the three display_settings chart options in the order
// show_data_labels, logarithmic_scale, show_totals_panel.
type chartOptionBools struct {
	showDataLabels, logarithmicScale, showTotalsPanel *bool
}

// Read mapping: both mappers must fill the chart options, or the generated
// NewDisplaySettingsValue rejects the object for a missing attribute.
func TestMapReportToModel_DisplaySettingsChartOptions(t *testing.T) {
	ctx := t.Context()

	tests := []struct {
		name string
		api  chartOptionBools
		want [3]types.Bool
	}{
		{
			name: "all true",
			api:  chartOptionBools{new(true), new(true), new(true)},
			want: [3]types.Bool{types.BoolValue(true), types.BoolValue(true), types.BoolValue(true)},
		},
		{
			name: "mixed",
			api:  chartOptionBools{new(true), new(false), new(true)},
			want: [3]types.Bool{types.BoolValue(true), types.BoolValue(false), types.BoolValue(true)},
		},
		{
			name: "omitted by the API",
			api:  chartOptionBools{},
			want: [3]types.Bool{types.BoolNull(), types.BoolNull(), types.BoolNull()},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &models.ExternalReport{
				Config: &models.ExternalConfig{
					DisplaySettings: &models.ExternalDisplaySettings{
						ShowDataLabels:   tt.api.showDataLabels,
						LogarithmicScale: tt.api.logarithmicScale,
						ShowTotalsPanel:  tt.api.showTotalsPanel,
					},
				},
			}

			state := &reportResourceModel{}
			if diags := mapReportToModel(ctx, resp, state); diags.HasError() {
				t.Fatalf("mapReportToModel returned errors: %v", diags)
			}
			ds := state.Config.DisplaySettings
			got := [3]types.Bool{ds.ShowDataLabels, ds.LogarithmicScale, ds.ShowTotalsPanel}
			if got != tt.want {
				t.Errorf("resource: got %v, want %v", got, tt.want)
			}

			dsState := &reportDataSourceModel{}
			if diags := (&reportDataSource{}).populateState(ctx, dsState, resp); diags.HasError() {
				t.Fatalf("populateState returned errors: %v", diags)
			}
			dds := dsState.Config.DisplaySettings
			dgot := [3]types.Bool{dds.ShowDataLabels, dds.LogarithmicScale, dds.ShowTotalsPanel}
			if dgot != tt.want {
				t.Errorf("data source: got %v, want %v", dgot, tt.want)
			}
		})
	}
}

// Request builder: null and unknown are left out so the API keeps the stored
// value, a configured false is sent (it is what clears a stored true).
func TestToExternalConfig_DisplaySettingsChartOptions(t *testing.T) {
	ctx := t.Context()

	tests := []struct {
		name string
		in   [3]types.Bool
		want chartOptionBools
	}{
		{"all null", [3]types.Bool{types.BoolNull(), types.BoolNull(), types.BoolNull()}, chartOptionBools{}},
		{"all unknown", [3]types.Bool{types.BoolUnknown(), types.BoolUnknown(), types.BoolUnknown()}, chartOptionBools{}},
		{
			"all true",
			[3]types.Bool{types.BoolValue(true), types.BoolValue(true), types.BoolValue(true)},
			chartOptionBools{new(true), new(true), new(true)},
		},
		{
			"all false",
			[3]types.Bool{types.BoolValue(false), types.BoolValue(false), types.BoolValue(false)},
			chartOptionBools{new(false), new(false), new(false)},
		},
		{
			"each option decided on its own",
			[3]types.Bool{types.BoolValue(true), types.BoolUnknown(), types.BoolValue(false)},
			chartOptionBools{new(true), nil, new(false)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgMap := validCumulativeComparisonConfigMap(ctx, t)
			cfgMap["display_settings"] = resource_report.NewDisplaySettingsValueMust(
				resource_report.DisplaySettingsValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"axis_label_font_size": types.StringNull(),
					"data_label_font_size": types.StringNull(),
					"decimal_precision":    types.Int64Null(),
					"number_scale":         types.StringNull(),
					"show_data_labels":     tt.in[0],
					"logarithmic_scale":    tt.in[1],
					"show_totals_panel":    tt.in[2],
					"theme_id":             types.StringNull(),
				})
			cfgVal, diags := resource_report.NewConfigValue(resource_report.ConfigValue{}.AttributeTypes(ctx), cfgMap)
			if diags.HasError() {
				t.Fatalf("NewConfigValue: %v", diags)
			}

			ext, extDiags := toExternalConfig(ctx, cfgVal)
			if extDiags.HasError() {
				t.Fatalf("toExternalConfig: %v", extDiags)
			}
			if ext.DisplaySettings == nil {
				t.Fatal("DisplaySettings is nil")
			}

			got := chartOptionBools{
				ext.DisplaySettings.ShowDataLabels,
				ext.DisplaySettings.LogarithmicScale,
				ext.DisplaySettings.ShowTotalsPanel,
			}
			for i, pair := range [][2]*bool{
				{got.showDataLabels, tt.want.showDataLabels},
				{got.logarithmicScale, tt.want.logarithmicScale},
				{got.showTotalsPanel, tt.want.showTotalsPanel},
			} {
				if (pair[0] == nil) != (pair[1] == nil) || (pair[0] != nil && *pair[0] != *pair[1]) {
					t.Errorf("option %d: got %v, want %v", i, derefBool(pair[0]), derefBool(pair[1]))
				}
			}
		})
	}
}

func derefBool(b *bool) any {
	if b == nil {
		return "nil"
	}
	return *b
}

// Overlay: unknown planned options resolve from the API, planned values stay.
func TestOverlayDisplaySettings_ChartOptions(t *testing.T) {
	ctx := t.Context()

	build := func(a, b, c types.Bool) resource_report.DisplaySettingsValue {
		return resource_report.NewDisplaySettingsValueMust(
			resource_report.DisplaySettingsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"axis_label_font_size": types.StringNull(),
				"data_label_font_size": types.StringNull(),
				"decimal_precision":    types.Int64Null(),
				"number_scale":         types.StringNull(),
				"show_data_labels":     a,
				"logarithmic_scale":    b,
				"show_totals_panel":    c,
				"theme_id":             types.StringValue("default"),
			})
	}

	resolved := build(types.BoolValue(true), types.BoolValue(false), types.BoolValue(true))
	plan := build(types.BoolUnknown(), types.BoolValue(true), types.BoolUnknown())

	overlayDisplaySettings(&resolved, &plan)

	want := build(types.BoolValue(true), types.BoolValue(true), types.BoolValue(true))
	if !plan.Equal(want) {
		t.Errorf("got %v, want %v", plan, want)
	}
}

// The three chart options must carry the clearing modifier, so that omitting
// one plans false (sent to the API) instead of keeping the stored value.
func TestReportSchema_ChartOptionsAreClearable(t *testing.T) {
	ctx := t.Context()

	var resp resource.SchemaResponse
	(&reportResource{}).Schema(ctx, resource.SchemaRequest{}, &resp)

	cfg, ok := resp.Schema.Attributes["config"].(schema.SingleNestedAttribute)
	if !ok {
		t.Fatal("config is not a SingleNestedAttribute")
	}
	ds, ok := cfg.Attributes["display_settings"].(schema.SingleNestedAttribute)
	if !ok {
		t.Fatal("display_settings is not a SingleNestedAttribute")
	}

	for _, name := range []string{"show_data_labels", "logarithmic_scale", "show_totals_panel"} {
		b, ok := ds.Attributes[name].(schema.BoolAttribute)
		if !ok {
			t.Fatalf("%s is not a BoolAttribute", name)
		}
		found := false
		for _, m := range b.PlanModifiers {
			if _, ok := m.(reportConfigBoolClearModifier); ok {
				found = true
			}
		}
		if !found {
			t.Errorf("%s has no reportConfigBoolClearModifier", name)
		}
	}
}

// resetChartOptionsWhenDisplaySettingsOmitted decision table: it acts only when
// the block is omitted and a block is stored, and then plans the stored block
// with the three chart options false and every other leaf as stored.
func TestResetChartOptionsWhenDisplaySettingsOmitted(t *testing.T) {
	ctx := t.Context()
	attrTypes := resource_report.DisplaySettingsValue{}.AttributeTypes(ctx)

	build := func(scale string, a, b, c bool) types.Object {
		v := resource_report.NewDisplaySettingsValueMust(attrTypes, map[string]attr.Value{
			"axis_label_font_size": types.StringValue("large"),
			"data_label_font_size": types.StringNull(),
			"decimal_precision":    types.Int64Value(2),
			"number_scale":         types.StringValue(scale),
			"show_data_labels":     types.BoolValue(a),
			"logarithmic_scale":    types.BoolValue(b),
			"show_totals_panel":    types.BoolValue(c),
			"theme_id":             types.StringValue("default"),
		})
		o, d := v.ToObjectValue(ctx)
		if d.HasError() {
			t.Fatalf("ToObjectValue: %v", d)
		}
		return o
	}
	objType := types.ObjectNull(attrTypes)

	tests := []struct {
		name                string
		config, state, plan types.Object
		want                types.Object
	}{
		{"omitted, stored options on, plan unknown", objType, build("millions", true, true, true), types.ObjectUnknown(attrTypes), build("millions", false, false, false)},
		{"omitted, stored options on, plan holds stored", objType, build("millions", true, false, true), build("millions", true, false, true), build("millions", false, false, false)},
		{"omitted, stored options already off", objType, build("millions", false, false, false), types.ObjectUnknown(attrTypes), build("millions", false, false, false)},
		{"create: nothing stored, plan unknown stays unknown", objType, objType, types.ObjectUnknown(attrTypes), types.ObjectUnknown(attrTypes)},
		{"configured block is left alone", build("billions", true, true, true), build("millions", true, true, true), build("billions", true, true, true), build("billions", true, true, true)},
		{"unknown config is left alone", types.ObjectUnknown(attrTypes), build("millions", true, true, true), types.ObjectUnknown(attrTypes), types.ObjectUnknown(attrTypes)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &planmodifier.ObjectResponse{PlanValue: tt.plan}
			resetChartOptionsWhenDisplaySettingsOmitted().PlanModifyObject(ctx, planmodifier.ObjectRequest{
				ConfigValue: tt.config, StateValue: tt.state, PlanValue: tt.plan,
			}, resp)
			if resp.Diagnostics.HasError() {
				t.Fatalf("diagnostics: %v", resp.Diagnostics)
			}
			if !resp.PlanValue.Equal(tt.want) {
				t.Errorf("got %v, want %v", resp.PlanValue, tt.want)
			}
		})
	}
}

// While the config wrapper is omitted, config is preserved (Category B), so
// neither the object modifier nor the leaf modifier may plan anything: the plan
// holds the stored value and must stay as it is.
func TestChartOptionModifiers_ConfigWrapperOmitted(t *testing.T) {
	ctx := t.Context()

	var schemaResp resource.SchemaResponse
	(&reportResource{}).Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	rawNull := tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), nil)
	omittedConfig := tfsdk.Config{Schema: schemaResp.Schema, Raw: rawNull}

	attrTypes := resource_report.DisplaySettingsValue{}.AttributeTypes(ctx)
	storedVal := resource_report.NewDisplaySettingsValueMust(attrTypes, map[string]attr.Value{
		"axis_label_font_size": types.StringNull(),
		"data_label_font_size": types.StringNull(),
		"decimal_precision":    types.Int64Null(),
		"number_scale":         types.StringNull(),
		"show_data_labels":     types.BoolValue(true),
		"logarithmic_scale":    types.BoolValue(true),
		"show_totals_panel":    types.BoolValue(true),
		"theme_id":             types.StringValue("default"),
	})
	stored, d := storedVal.ToObjectValue(ctx)
	if d.HasError() {
		t.Fatalf("ToObjectValue: %v", d)
	}

	t.Run("object modifier", func(t *testing.T) {
		resp := &planmodifier.ObjectResponse{PlanValue: stored}
		resetChartOptionsWhenDisplaySettingsOmitted().PlanModifyObject(ctx, planmodifier.ObjectRequest{
			Config:      omittedConfig,
			ConfigValue: types.ObjectNull(attrTypes),
			StateValue:  stored,
			PlanValue:   stored,
		}, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("diagnostics: %v", resp.Diagnostics)
		}
		if !resp.PlanValue.Equal(stored) {
			t.Errorf("plan changed to %v, want the stored value %v", resp.PlanValue, stored)
		}
	})

	t.Run("leaf modifier", func(t *testing.T) {
		resp := &planmodifier.BoolResponse{PlanValue: types.BoolValue(true)}
		useEmptyForUnknownBoolInReportConfigWhenConfigNull().PlanModifyBool(ctx, planmodifier.BoolRequest{
			Config:      omittedConfig,
			ConfigValue: types.BoolNull(),
			StateValue:  types.BoolValue(true),
			PlanValue:   types.BoolValue(true),
		}, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("diagnostics: %v", resp.Diagnostics)
		}
		if !resp.PlanValue.Equal(types.BoolValue(true)) {
			t.Errorf("plan changed to %v, want true", resp.PlanValue)
		}
	})
}
