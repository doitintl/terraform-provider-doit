package provider

import (
	"encoding/json"
	"maps"
	"reflect"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_budget"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// wantBudgetDiag is one expected error diagnostic.
type wantBudgetDiag struct {
	path    path.Path
	summary string
}

// assertBudgetDiagnostics checks that diagnostics are exactly the expected
// errors, in order.
func assertBudgetDiagnostics(t *testing.T, diagnostics diag.Diagnostics, want []wantBudgetDiag) {
	t.Helper()
	if len(diagnostics) != len(want) {
		t.Fatalf("diagnostic count = %d, want %d: %v", len(diagnostics), len(want), diagnostics)
	}
	for i, w := range want {
		d := diagnostics[i]
		if d.Severity() != diag.SeverityError {
			t.Errorf("diagnostic %d severity = %s, want error", i, d.Severity())
		}
		if d.Summary() != w.summary {
			t.Errorf("diagnostic %d summary = %q, want %q", i, d.Summary(), w.summary)
		}
		withPath, ok := d.(diag.DiagnosticWithPath)
		if !ok {
			t.Fatalf("diagnostic %d (%T) has no path", i, d)
		}
		if !withPath.Path().Equal(w.path) {
			t.Errorf("diagnostic %d path = %s, want %s", i, withPath.Path(), w.path)
		}
	}
}

func testSeasonalList(values ...attr.Value) types.List {
	return types.ListValueMust(types.Float64Type, values)
}

// testSeasonalAmounts returns a known list of n amounts.
func testSeasonalAmounts(n int) types.List {
	values := make([]attr.Value, n)
	for i := range values {
		values[i] = types.Float64Value(float64(10 * (i + 1)))
	}
	return testSeasonalList(values...)
}

func testRollover(t *testing.T, enabled, limit attr.Value) resource_budget.RolloverValue {
	t.Helper()
	v, diags := resource_budget.NewRolloverValue(resource_budget.RolloverValue{}.AttributeTypes(t.Context()), map[string]attr.Value{
		"enabled": enabled,
		"limit":   limit,
	})
	if diags.HasError() {
		t.Fatalf("NewRolloverValue: %v", diags)
	}
	return v
}

var (
	errAmount        = wantBudgetDiag{path.Root("amount"), "Invalid Attribute Combination"}
	errUsePrevSpend  = wantBudgetDiag{path.Root("use_prev_spend"), "Invalid Attribute Combination"}
	errAllowGrowth   = wantBudgetDiag{path.Root("allow_growth"), "Invalid Attribute Combination"}
	errRolloverOn    = wantBudgetDiag{path.Root("rollover").AtName("enabled"), "Invalid Attribute Combination"}
	errSeasonal      = wantBudgetDiag{path.Root("seasonal_amounts"), "Invalid Seasonal Amounts"}
	errRolloverLimit = wantBudgetDiag{path.Root("rollover").AtName("limit"), "Invalid Rollover Limit"}
)

func TestBudgetSeasonalAmountsValidator(t *testing.T) {
	ctx := t.Context()
	twelve := testSeasonalAmounts(12)
	base := func(overrides map[string]attr.Value) map[string]attr.Value {
		values := map[string]attr.Value{
			"type":             types.StringValue("recurring"),
			"time_interval":    types.StringValue("month"),
			"seasonal_amounts": twelve,
		}
		maps.Copy(values, overrides)
		return values
	}

	tests := []struct {
		name   string
		values map[string]attr.Value
		want   []wantBudgetDiag
	}{
		{"seasonal null with amount", base(map[string]attr.Value{"seasonal_amounts": types.ListNull(types.Float64Type), "amount": types.Float64Value(100)}), nil},
		{"seasonal unknown with amount", base(map[string]attr.Value{"seasonal_amounts": types.ListUnknown(types.Float64Type), "amount": types.Float64Value(100)}), nil},
		// Generated import configuration writes seasonal_amounts = [] next to amount.
		{"seasonal empty with everything", base(map[string]attr.Value{
			"seasonal_amounts": testSeasonalList(), "amount": types.Float64Value(100), "use_prev_spend": types.BoolValue(true),
			"allow_growth": types.BoolValue(true), "type": types.StringValue("fixed"), "time_interval": types.StringValue("week"),
		}), nil},
		{"valid month", base(nil), nil},
		{"valid explicit off switches", base(map[string]attr.Value{
			"use_prev_spend": types.BoolValue(false), "allow_growth": types.BoolValue(false),
			"rollover": testRollover(t, types.BoolValue(false), types.Int64Value(0)),
		}), nil},

		{"amount", base(map[string]attr.Value{"amount": types.Float64Value(100)}), []wantBudgetDiag{errAmount}},
		{"amount zero", base(map[string]attr.Value{"amount": types.Float64Value(0)}), []wantBudgetDiag{errAmount}},
		{"amount unknown", base(map[string]attr.Value{"amount": types.Float64Unknown()}), nil},

		{"use_prev_spend true", base(map[string]attr.Value{"use_prev_spend": types.BoolValue(true)}), []wantBudgetDiag{errUsePrevSpend}},
		{"use_prev_spend unknown", base(map[string]attr.Value{"use_prev_spend": types.BoolUnknown()}), nil},

		{"allow_growth true", base(map[string]attr.Value{"allow_growth": types.BoolValue(true)}), []wantBudgetDiag{errAllowGrowth}},
		{"allow_growth unknown", base(map[string]attr.Value{"allow_growth": types.BoolUnknown()}), nil},

		{"rollover enabled", base(map[string]attr.Value{"rollover": testRollover(t, types.BoolValue(true), types.Int64Value(2))}), []wantBudgetDiag{errRolloverOn}},
		{"rollover enabled unknown", base(map[string]attr.Value{"rollover": testRollover(t, types.BoolUnknown(), types.Int64Value(2))}), nil},
		{"rollover unknown", base(map[string]attr.Value{"rollover": resource_budget.NewRolloverValueUnknown()}), nil},

		{"fixed budget", base(map[string]attr.Value{"type": types.StringValue("fixed")}), []wantBudgetDiag{errSeasonal}},
		{"type unknown", base(map[string]attr.Value{"type": types.StringUnknown()}), nil},

		{"week interval", base(map[string]attr.Value{"time_interval": types.StringValue("week")}), []wantBudgetDiag{errSeasonal}},
		{"wrong length for quarter", base(map[string]attr.Value{"time_interval": types.StringValue("quarter")}), []wantBudgetDiag{errSeasonal}},
		{"day needs 7", base(map[string]attr.Value{"time_interval": types.StringValue("day"), "seasonal_amounts": testSeasonalAmounts(7)}), nil},
		{"year needs 5", base(map[string]attr.Value{"time_interval": types.StringValue("year"), "seasonal_amounts": testSeasonalAmounts(4)}), []wantBudgetDiag{errSeasonal}},
		{"interval unknown", base(map[string]attr.Value{"time_interval": types.StringUnknown(), "seasonal_amounts": testSeasonalAmounts(3)}), nil},
		{"interval null", base(map[string]attr.Value{"time_interval": types.StringNull(), "seasonal_amounts": testSeasonalAmounts(3)}), nil},
		// A known list has a known length, even when its elements are unknown.
		{"unknown element, right length", base(map[string]attr.Value{"time_interval": types.StringValue("quarter"),
			"seasonal_amounts": testSeasonalList(types.Float64Unknown(), types.Float64Value(1), types.Float64Value(2), types.Float64Value(3))}), nil},
		{"unknown element first, wrong length", base(map[string]attr.Value{"time_interval": types.StringValue("quarter"),
			"seasonal_amounts": testSeasonalList(types.Float64Unknown(), types.Float64Value(1), types.Float64Value(2))}), []wantBudgetDiag{errSeasonal}},
		{"unknown element last, wrong length", base(map[string]attr.Value{"time_interval": types.StringValue("quarter"),
			"seasonal_amounts": testSeasonalList(types.Float64Value(1), types.Float64Value(2), types.Float64Unknown())}), []wantBudgetDiag{errSeasonal}},

		// Independent errors are all reported, and an unknown value only
		// silences its own check.
		{"every conflict", base(map[string]attr.Value{
			"amount": types.Float64Value(100), "use_prev_spend": types.BoolValue(true), "allow_growth": types.BoolValue(true),
			"rollover": testRollover(t, types.BoolValue(true), types.Int64Value(2)), "type": types.StringValue("fixed"),
			"time_interval": types.StringValue("week"),
		}), []wantBudgetDiag{errAmount, errUsePrevSpend, errAllowGrowth, errRolloverOn, errSeasonal, errSeasonal}},
		{"unknown amount keeps other errors", base(map[string]attr.Value{
			"amount": types.Float64Unknown(), "use_prev_spend": types.BoolValue(true),
		}), []wantBudgetDiag{errUsePrevSpend}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := buildBudgetValidatorConfig(ctx, t, tt.values)
			resp := &resource.ValidateConfigResponse{}
			budgetSeasonalAmountsValidator{}.ValidateResource(ctx, resource.ValidateConfigRequest{Config: config}, resp)
			assertBudgetDiagnostics(t, resp.Diagnostics, tt.want)
		})
	}
}

func TestBudgetRolloverValidator(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name     string
		rollover attr.Value
		interval types.String
		want     []wantBudgetDiag
	}{
		{"null", resource_budget.NewRolloverValueNull(), types.StringValue("month"), nil},
		{"unknown", resource_budget.NewRolloverValueUnknown(), types.StringValue("month"), nil},
		{"enabled unknown", testRollover(t, types.BoolUnknown(), types.Int64Value(0)), types.StringValue("month"), nil},
		{"limit unknown", testRollover(t, types.BoolValue(true), types.Int64Unknown()), types.StringValue("month"), nil},
		{"disabled with limit 0", testRollover(t, types.BoolValue(false), types.Int64Value(0)), types.StringValue("month"), nil},
		{"disabled above maximum", testRollover(t, types.BoolValue(false), types.Int64Value(99)), types.StringValue("month"), nil},
		{"enabled limit 0", testRollover(t, types.BoolValue(true), types.Int64Value(0)), types.StringValue("month"), []wantBudgetDiag{errRolloverLimit}},
		{"enabled negative limit", testRollover(t, types.BoolValue(true), types.Int64Value(-1)), types.StringValue("month"), []wantBudgetDiag{errRolloverLimit}},
		{"enabled month maximum", testRollover(t, types.BoolValue(true), types.Int64Value(12)), types.StringValue("month"), nil},
		{"enabled above month maximum", testRollover(t, types.BoolValue(true), types.Int64Value(13)), types.StringValue("month"), []wantBudgetDiag{errRolloverLimit}},
		{"enabled week maximum", testRollover(t, types.BoolValue(true), types.Int64Value(52)), types.StringValue("week"), nil},
		{"enabled above day maximum", testRollover(t, types.BoolValue(true), types.Int64Value(91)), types.StringValue("day"), []wantBudgetDiag{errRolloverLimit}},
		{"enabled above year maximum", testRollover(t, types.BoolValue(true), types.Int64Value(6)), types.StringValue("year"), []wantBudgetDiag{errRolloverLimit}},
		{"enabled quarter maximum", testRollover(t, types.BoolValue(true), types.Int64Value(8)), types.StringValue("quarter"), nil},
		// The maximum needs the interval; the minimum does not.
		{"above maximum, interval unknown", testRollover(t, types.BoolValue(true), types.Int64Value(99)), types.StringUnknown(), nil},
		{"above maximum, interval null", testRollover(t, types.BoolValue(true), types.Int64Value(99)), types.StringNull(), nil},
		{"limit 0, interval unknown", testRollover(t, types.BoolValue(true), types.Int64Value(0)), types.StringUnknown(), []wantBudgetDiag{errRolloverLimit}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := buildBudgetValidatorConfig(ctx, t, map[string]attr.Value{"rollover": tt.rollover, "time_interval": tt.interval})
			resp := &resource.ValidateConfigResponse{}
			budgetRolloverValidator{}.ValidateResource(ctx, resource.ValidateConfigRequest{Config: config}, resp)
			assertBudgetDiagnostics(t, resp.Diagnostics, tt.want)
		})
	}
}

// TestBudgetModifyPlanSeasonalChecks covers the checks that need the stored
// budget. Each row gives the configuration and the stored values; the plan is
// built the way the framework plans it: configured values as configured,
// growth_per_period at its default 0 when omitted, and an omitted
// allow_growth or time_interval at the stored value (an update that changes
// nothing else) or unknown (create, or an update that changes something else).
func TestBudgetModifyPlanSeasonalChecks(t *testing.T) {
	ctx := t.Context()
	const createRow, updateRow = false, true

	tests := []struct {
		name         string
		update       bool
		plannedKnown bool // omitted Optional+Computed values plan as stored (true) or unknown (false)
		config       map[string]attr.Value
		stored       map[string]attr.Value
		want         []wantBudgetDiag
	}{
		// Create: growth_per_period above 0 turns growth on unless allow_growth is set.
		{"create, rate above 0", createRow, false, map[string]attr.Value{"seasonal_amounts": testSeasonalAmounts(12), "growth_per_period": types.Float64Value(5)}, nil, []wantBudgetDiag{errAllowGrowth}},
		{"create, rate 0", createRow, false, map[string]attr.Value{"seasonal_amounts": testSeasonalAmounts(12), "growth_per_period": types.Float64Value(0)}, nil, nil},
		{"create, rate omitted (default 0)", createRow, false, map[string]attr.Value{"seasonal_amounts": testSeasonalAmounts(12)}, nil, nil},
		{"create, rate unknown", createRow, false, map[string]attr.Value{"seasonal_amounts": testSeasonalAmounts(12), "growth_per_period": types.Float64Unknown()}, nil, nil},
		{"create, rate above 0, allow_growth false", createRow, false, map[string]attr.Value{"seasonal_amounts": testSeasonalAmounts(12), "growth_per_period": types.Float64Value(5), "allow_growth": types.BoolValue(false)}, nil, nil},
		{"create, rate above 0, allow_growth unknown", createRow, false, map[string]attr.Value{"seasonal_amounts": testSeasonalAmounts(12), "growth_per_period": types.Float64Value(5), "allow_growth": types.BoolUnknown()}, nil, nil},
		{"create, rate above 0, no seasonal", createRow, false, map[string]attr.Value{"growth_per_period": types.Float64Value(5)}, nil, nil},
		{"create, rate above 0, seasonal empty", createRow, false, map[string]attr.Value{"seasonal_amounts": testSeasonalList(), "growth_per_period": types.Float64Value(5)}, nil, nil},
		{"create, rate above 0, seasonal unknown", createRow, false, map[string]attr.Value{"seasonal_amounts": types.ListUnknown(types.Float64Type), "growth_per_period": types.Float64Value(5)}, nil, nil},

		// Update: the stored allow_growth decides, whatever the rate.
		{"update, growth stored", updateRow, false, map[string]attr.Value{"seasonal_amounts": testSeasonalAmounts(12)}, map[string]attr.Value{"allow_growth": types.BoolValue(true)}, []wantBudgetDiag{errAllowGrowth}},
		{"update, growth stored, plan holds it", updateRow, true, map[string]attr.Value{"seasonal_amounts": testSeasonalAmounts(12)}, map[string]attr.Value{"allow_growth": types.BoolValue(true)}, []wantBudgetDiag{errAllowGrowth}},
		{"update, growth stored, allow_growth false", updateRow, false, map[string]attr.Value{"seasonal_amounts": testSeasonalAmounts(12), "allow_growth": types.BoolValue(false)}, map[string]attr.Value{"allow_growth": types.BoolValue(true)}, nil},
		{"update, no growth stored, rate above 0", updateRow, false, map[string]attr.Value{"seasonal_amounts": testSeasonalAmounts(12), "growth_per_period": types.Float64Value(5)}, map[string]attr.Value{"allow_growth": types.BoolValue(false)}, nil},
		{"update, growth stored, seasonal empty", updateRow, false, map[string]attr.Value{"seasonal_amounts": testSeasonalList()}, map[string]attr.Value{"allow_growth": types.BoolValue(true)}, nil},
		{"update, stored allow_growth null", updateRow, false, map[string]attr.Value{"seasonal_amounts": testSeasonalAmounts(12)}, map[string]attr.Value{"allow_growth": types.BoolNull()}, nil},

		// Update with time_interval omitted: the stored interval applies.
		{"update, stored quarter, 12 amounts", updateRow, false, map[string]attr.Value{"time_interval": types.StringNull(), "seasonal_amounts": testSeasonalAmounts(12)}, map[string]attr.Value{"time_interval": types.StringValue("quarter")}, []wantBudgetDiag{errSeasonal}},
		{"update, stored quarter, 12 amounts, plan holds it", updateRow, true, map[string]attr.Value{"time_interval": types.StringNull(), "seasonal_amounts": testSeasonalAmounts(12)}, map[string]attr.Value{"time_interval": types.StringValue("quarter")}, []wantBudgetDiag{errSeasonal}},
		{"update, stored month, 12 amounts", updateRow, false, map[string]attr.Value{"time_interval": types.StringNull(), "seasonal_amounts": testSeasonalAmounts(12)}, map[string]attr.Value{"time_interval": types.StringValue("month")}, nil},
		{"update, stored week, 7 amounts", updateRow, false, map[string]attr.Value{"time_interval": types.StringNull(), "seasonal_amounts": testSeasonalAmounts(7)}, map[string]attr.Value{"time_interval": types.StringValue("week")}, []wantBudgetDiag{errSeasonal}},
		// A configured interval is the config validator's job.
		{"update, configured quarter, 12 amounts", updateRow, false, map[string]attr.Value{"time_interval": types.StringValue("quarter"), "seasonal_amounts": testSeasonalAmounts(12)}, map[string]attr.Value{"time_interval": types.StringValue("month")}, nil},
		{"update, stored month, rollover above maximum", updateRow, false, map[string]attr.Value{"time_interval": types.StringNull(), "rollover": testRollover(t, types.BoolValue(true), types.Int64Value(13))}, map[string]attr.Value{"time_interval": types.StringValue("month")}, []wantBudgetDiag{errRolloverLimit}},
		{"update, stored year, rollover at month maximum", updateRow, false, map[string]attr.Value{"time_interval": types.StringNull(), "rollover": testRollover(t, types.BoolValue(true), types.Int64Value(12))}, map[string]attr.Value{"time_interval": types.StringValue("year")}, []wantBudgetDiag{errRolloverLimit}},
		{"update, stored month, rollover at maximum", updateRow, false, map[string]attr.Value{"time_interval": types.StringNull(), "rollover": testRollover(t, types.BoolValue(true), types.Int64Value(12))}, map[string]attr.Value{"time_interval": types.StringValue("month")}, nil},
		{"update, stored month, disabled rollover above maximum", updateRow, false, map[string]attr.Value{"time_interval": types.StringNull(), "rollover": testRollover(t, types.BoolValue(false), types.Int64Value(13))}, map[string]attr.Value{"time_interval": types.StringValue("month")}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configValues := map[string]attr.Value{
				"name":          types.StringValue("budget"),
				"type":          types.StringValue("recurring"),
				"start_period":  types.Int64Value(1759276800000),
				"time_interval": types.StringValue("month"),
			}
			maps.Copy(configValues, tt.config)
			config := buildBudgetValidatorConfig(ctx, t, configValues)

			storedValues := map[string]attr.Value{
				"name":              types.StringValue("budget"),
				"type":              types.StringValue("recurring"),
				"start_period":      types.Int64Value(1759276800000),
				"time_interval":     types.StringValue("month"),
				"allow_growth":      types.BoolValue(false),
				"growth_per_period": types.Float64Value(0),
				"seasonal_amounts":  testSeasonalList(),
			}
			maps.Copy(storedValues, tt.stored)

			planValues := maps.Clone(configValues)
			if _, ok := tt.config["growth_per_period"]; !ok {
				planValues["growth_per_period"] = types.Float64Value(0)
			}
			for _, name := range []string{"allow_growth", "time_interval"} {
				configured := configValues[name]
				if configured != nil && !configured.IsNull() {
					continue
				}
				switch {
				case tt.update && tt.plannedKnown:
					planValues[name] = storedValues[name]
				case name == "allow_growth":
					planValues[name] = types.BoolUnknown()
				default:
					planValues[name] = types.StringUnknown()
				}
			}
			plan := buildBudgetValidatorConfig(ctx, t, planValues)

			stateRaw := tftypes.NewValue(config.Raw.Type(), nil)
			if tt.update {
				stateRaw = buildBudgetValidatorConfig(ctx, t, storedValues).Raw
			}

			// The create-only checks in ModifyPlan need the schema the
			// resource serves; the generated one is enough for these rows.
			resp := &resource.ModifyPlanResponse{Plan: tfsdk.Plan{Schema: config.Schema, Raw: plan.Raw.Copy()}}
			(&budgetResource{}).ModifyPlan(ctx, resource.ModifyPlanRequest{
				Config: config,
				State:  tfsdk.State{Schema: config.Schema, Raw: stateRaw},
				Plan:   tfsdk.Plan{Schema: config.Schema, Raw: plan.Raw},
			}, resp)
			assertBudgetDiagnostics(t, resp.Diagnostics, tt.want)
		})
	}
}

func TestMapBudgetRollover(t *testing.T) {
	ctx := t.Context()
	enabled3 := func() resource_budget.RolloverValue {
		return testRollover(t, types.BoolValue(true), types.Int64Value(3))
	}
	disabled3 := func() resource_budget.RolloverValue {
		return testRollover(t, types.BoolValue(false), types.Int64Value(3))
	}

	priors := map[string]func() resource_budget.RolloverValue{
		"prior null":     resource_budget.NewRolloverValueNull,
		"prior unknown":  resource_budget.NewRolloverValueUnknown,
		"prior enabled":  enabled3,
		"prior disabled": disabled3,
	}
	apis := map[string]*models.BudgetRollover{
		"api nil":              nil,
		"api disabled":         {Enabled: false, Limit: 0},
		"api disabled limit 5": {Enabled: false, Limit: 5},
		"api enabled":          {Enabled: true, Limit: 2},
	}

	for priorName, prior := range priors {
		for apiName, api := range apis {
			t.Run(priorName+", "+apiName, func(t *testing.T) {
				priorKnown := priorName == "prior enabled" || priorName == "prior disabled"
				var want resource_budget.RolloverValue
				switch {
				case api == nil:
					want = resource_budget.NewRolloverValueNull()
				case !api.Enabled && !priorKnown:
					want = resource_budget.NewRolloverValueNull()
				default:
					want = testRollover(t, types.BoolValue(api.Enabled), types.Int64Value(api.Limit))
				}

				got, diags := mapBudgetRollover(ctx, api, prior())
				if diags.HasError() {
					t.Fatalf("mapBudgetRollover: %v", diags)
				}
				if !got.Equal(want) {
					t.Errorf("rollover = %v, want %v", got, want)
				}
			})
		}
	}
}

func TestBudgetRequestNewFields(t *testing.T) {
	ctx := t.Context()

	type fields struct {
		AllowGrowth     *bool                  `json:"allowGrowth,omitempty"`
		Rollover        *models.BudgetRollover `json:"rollover,omitempty"`
		Amount          *float64               `json:"amount,omitempty"`
		SeasonalAmounts *[]float64             `json:"seasonalAmounts,omitempty"`
	}
	empty := []float64{}
	seasonal := []float64{10, 20}

	tests := []struct {
		name       string
		modify     func(plan *budgetResourceModel)
		wantCreate fields
		wantUpdate fields
	}{
		{
			name:       "omitted on create, cleared on update",
			modify:     func(plan *budgetResourceModel) {},
			wantCreate: fields{},
			// A null rollover disables it; the API keeps the stored one when it is left out.
			wantUpdate: fields{Rollover: &models.BudgetRollover{Enabled: false, Limit: 0}},
		},
		{
			name: "unknown values are left out",
			modify: func(plan *budgetResourceModel) {
				plan.AllowGrowth = types.BoolUnknown()
				plan.Rollover = resource_budget.NewRolloverValueUnknown()
				plan.Amount = types.Float64Unknown()
				plan.SeasonalAmounts = types.ListUnknown(types.Float64Type)
			},
		},
		{
			name: "configured values are sent",
			modify: func(plan *budgetResourceModel) {
				plan.AllowGrowth = types.BoolValue(false)
				plan.Rollover = testRollover(t, types.BoolValue(true), types.Int64Value(3))
				plan.SeasonalAmounts = testSeasonalList(types.Float64Value(10), types.Float64Value(20))
			},
			wantCreate: fields{AllowGrowth: new(false), Rollover: &models.BudgetRollover{Enabled: true, Limit: 3}, SeasonalAmounts: &seasonal},
			wantUpdate: fields{AllowGrowth: new(false), Rollover: &models.BudgetRollover{Enabled: true, Limit: 3}, SeasonalAmounts: &seasonal},
		},
		{
			// Removing seasonal amounts sends [] together with the amount.
			name: "empty seasonal list is sent as []",
			modify: func(plan *budgetResourceModel) {
				plan.Amount = types.Float64Value(100)
				plan.SeasonalAmounts = testSeasonalList()
				plan.Rollover = testRollover(t, types.BoolValue(false), types.Int64Value(0))
			},
			wantCreate: fields{Amount: new(100.0), SeasonalAmounts: &empty, Rollover: &models.BudgetRollover{}},
			wantUpdate: fields{Amount: new(100.0), SeasonalAmounts: &empty, Rollover: &models.BudgetRollover{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := newTestBudgetPlan(t)
			plan.Rollover = resource_budget.NewRolloverValueNull()
			tt.modify(plan)

			createReq, diags := plan.toCreateRequest(ctx)
			if diags.HasError() {
				t.Fatalf("toCreateRequest: %v", diags)
			}
			gotCreate := fields{AllowGrowth: createReq.AllowGrowth, Rollover: createReq.Rollover, Amount: createReq.Amount, SeasonalAmounts: createReq.SeasonalAmounts}
			if !reflect.DeepEqual(gotCreate, tt.wantCreate) {
				t.Errorf("create = %s, want %s", jsonOf(t, gotCreate), jsonOf(t, tt.wantCreate))
			}

			updateReq, diags := plan.toUpdateRequest(ctx)
			if diags.HasError() {
				t.Fatalf("toUpdateRequest: %v", diags)
			}
			gotUpdate := fields{AllowGrowth: updateReq.AllowGrowth, Rollover: updateReq.Rollover, Amount: updateReq.Amount, SeasonalAmounts: updateReq.SeasonalAmounts}
			if !reflect.DeepEqual(gotUpdate, tt.wantUpdate) {
				t.Errorf("update = %s, want %s", jsonOf(t, gotUpdate), jsonOf(t, tt.wantUpdate))
			}
		})
	}

	// The empty list must reach the API as [], not be dropped.
	plan := newTestBudgetPlan(t)
	plan.Amount = types.Float64Value(100)
	plan.SeasonalAmounts = testSeasonalList()
	req, diags := plan.toUpdateRequest(ctx)
	if diags.HasError() {
		t.Fatalf("toUpdateRequest: %v", diags)
	}
	body := jsonOf(t, req)
	var decoded map[string]any
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		t.Fatalf("decode request: %v", err)
	}
	if got, ok := decoded["seasonalAmounts"].([]any); !ok || len(got) != 0 {
		t.Errorf("seasonalAmounts in body = %v, want []", decoded["seasonalAmounts"])
	}
}

func jsonOf(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}
