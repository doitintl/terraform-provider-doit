package provider

import (
	"maps"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_budget"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var (
	errGrowthNegative    = wantBudgetDiag{path.Root("growth_per_period"), "Invalid Growth Per Period"}
	errFixedAllowGrowth  = errAllowGrowth
	errFixedGrowthRate   = errGrowthRate
	errFixedRollover     = errRolloverOn
	errFixedUsePrevSpend = errUsePrevSpend
	errMissingAmount     = wantBudgetDiag{path.Root("amount"), "Missing Budget Amount"}
)

// TestBudgetGrowthValidator covers growth_per_period against allow_growth. An
// omitted growth_per_period is its default 0 and an omitted allow_growth is
// decided by the request builder, so neither is an error.
func TestBudgetGrowthValidator(t *testing.T) {
	ctx := t.Context()
	base := func(overrides map[string]attr.Value) map[string]attr.Value {
		values := map[string]attr.Value{"type": types.StringValue("recurring")}
		maps.Copy(values, overrides)
		return values
	}

	tests := []struct {
		name   string
		values map[string]attr.Value
		want   []wantBudgetDiag
	}{
		{"nothing set", base(nil), nil},
		{"allow_growth true, rate", base(map[string]attr.Value{"allow_growth": types.BoolValue(true), "growth_per_period": types.Float64Value(5)}), nil},
		{"allow_growth true, rate 0", base(map[string]attr.Value{"allow_growth": types.BoolValue(true), "growth_per_period": types.Float64Value(0)}), nil},
		{"allow_growth false, rate 0", base(map[string]attr.Value{"allow_growth": types.BoolValue(false), "growth_per_period": types.Float64Value(0)}), nil},
		{"allow_growth false, rate omitted", base(map[string]attr.Value{"allow_growth": types.BoolValue(false)}), nil},
		{"allow_growth omitted, rate", base(map[string]attr.Value{"growth_per_period": types.Float64Value(5)}), nil},

		{"allow_growth false, rate", base(map[string]attr.Value{"allow_growth": types.BoolValue(false), "growth_per_period": types.Float64Value(5)}), []wantBudgetDiag{errGrowthRate}},
		{"allow_growth false, fractional rate", base(map[string]attr.Value{"allow_growth": types.BoolValue(false), "growth_per_period": types.Float64Value(0.5)}), []wantBudgetDiag{errGrowthRate}},
		{"allow_growth false, negative rate", base(map[string]attr.Value{"allow_growth": types.BoolValue(false), "growth_per_period": types.Float64Value(-1)}), []wantBudgetDiag{errGrowthNegative, errGrowthRate}},
		{"allow_growth true, negative rate", base(map[string]attr.Value{"allow_growth": types.BoolValue(true), "growth_per_period": types.Float64Value(-1)}), []wantBudgetDiag{errGrowthNegative}},
		{"allow_growth omitted, negative rate", base(map[string]attr.Value{"growth_per_period": types.Float64Value(-0.1)}), []wantBudgetDiag{errGrowthNegative}},

		{"allow_growth unknown, rate", base(map[string]attr.Value{"allow_growth": types.BoolUnknown(), "growth_per_period": types.Float64Value(5)}), nil},
		{"allow_growth false, rate unknown", base(map[string]attr.Value{"allow_growth": types.BoolValue(false), "growth_per_period": types.Float64Unknown()}), nil},
		{"both unknown", base(map[string]attr.Value{"allow_growth": types.BoolUnknown(), "growth_per_period": types.Float64Unknown()}), nil},
		// An unknown allow_growth only silences the combination check; the
		// negative rate is wrong whatever allow_growth turns out to be.
		{"allow_growth unknown, negative rate", base(map[string]attr.Value{"allow_growth": types.BoolUnknown(), "growth_per_period": types.Float64Value(-1)}), []wantBudgetDiag{errGrowthNegative}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := buildBudgetValidatorConfig(ctx, t, tt.values)
			resp := &resource.ValidateConfigResponse{}
			budgetGrowthValidator{}.ValidateResource(ctx, resource.ValidateConfigRequest{Config: config}, resp)
			assertBudgetDiagnostics(t, resp.Diagnostics, tt.want)
		})
	}
}

// TestBudgetFixedValidator covers a fixed budget. What a fixed budget reads
// back as (allow_growth false, rate 0, disabled rollover, use_prev_spend false)
// must stay valid, because a generated import configuration holds those values.
func TestBudgetFixedValidator(t *testing.T) {
	ctx := t.Context()
	base := func(budgetType attr.Value, overrides map[string]attr.Value) map[string]attr.Value {
		values := map[string]attr.Value{"type": budgetType}
		maps.Copy(values, overrides)
		return values
	}
	fixed := types.StringValue("fixed")
	recurring := types.StringValue("recurring")
	disabled := testRollover(t, types.BoolValue(false), types.Int64Value(0))
	enabled := testRollover(t, types.BoolValue(true), types.Int64Value(2))

	tests := []struct {
		name   string
		values map[string]attr.Value
		want   []wantBudgetDiag
	}{
		{"fixed, nothing set", base(fixed, nil), nil},
		{"fixed, read-back values", base(fixed, map[string]attr.Value{
			"allow_growth": types.BoolValue(false), "growth_per_period": types.Float64Value(0),
			"rollover": disabled, "use_prev_spend": types.BoolValue(false),
		}), nil},
		{"fixed, rollover with limit but disabled", base(fixed, map[string]attr.Value{"rollover": testRollover(t, types.BoolValue(false), types.Int64Value(5))}), nil},

		{"fixed, allow_growth", base(fixed, map[string]attr.Value{"allow_growth": types.BoolValue(true)}), []wantBudgetDiag{errFixedAllowGrowth}},
		{"fixed, rate", base(fixed, map[string]attr.Value{"growth_per_period": types.Float64Value(5)}), []wantBudgetDiag{errFixedGrowthRate}},
		{"fixed, negative rate", base(fixed, map[string]attr.Value{"growth_per_period": types.Float64Value(-1)}), []wantBudgetDiag{errFixedGrowthRate}},
		{"fixed, rollover", base(fixed, map[string]attr.Value{"rollover": enabled}), []wantBudgetDiag{errFixedRollover}},
		{"fixed, use_prev_spend", base(fixed, map[string]attr.Value{"use_prev_spend": types.BoolValue(true)}), []wantBudgetDiag{errFixedUsePrevSpend}},

		// Recurring budgets may use all four.
		{"recurring, everything", base(recurring, map[string]attr.Value{
			"allow_growth": types.BoolValue(true), "growth_per_period": types.Float64Value(5), "rollover": enabled,
		}), nil},
		{"recurring, use_prev_spend", base(recurring, map[string]attr.Value{"use_prev_spend": types.BoolValue(true)}), nil},
		{"type null", base(types.StringNull(), map[string]attr.Value{"allow_growth": types.BoolValue(true)}), nil},
		{"type unknown", base(types.StringUnknown(), map[string]attr.Value{
			"allow_growth": types.BoolValue(true), "growth_per_period": types.Float64Value(5), "rollover": enabled, "use_prev_spend": types.BoolValue(true),
		}), nil},

		// Unknown values defer.
		{"fixed, allow_growth unknown", base(fixed, map[string]attr.Value{"allow_growth": types.BoolUnknown()}), nil},
		{"fixed, rate unknown", base(fixed, map[string]attr.Value{"growth_per_period": types.Float64Unknown()}), nil},
		{"fixed, rollover unknown", base(fixed, map[string]attr.Value{"rollover": resource_budget.NewRolloverValueUnknown()}), nil},
		{"fixed, rollover enabled unknown", base(fixed, map[string]attr.Value{"rollover": testRollover(t, types.BoolUnknown(), types.Int64Value(2))}), nil},
		{"fixed, use_prev_spend unknown", base(fixed, map[string]attr.Value{"use_prev_spend": types.BoolUnknown()}), nil},

		// Independent errors are all reported, and an unknown value only
		// silences its own check.
		{"fixed, every conflict", base(fixed, map[string]attr.Value{
			"allow_growth": types.BoolValue(true), "growth_per_period": types.Float64Value(5), "rollover": enabled, "use_prev_spend": types.BoolValue(true),
		}), []wantBudgetDiag{errFixedAllowGrowth, errFixedGrowthRate, errFixedRollover, errFixedUsePrevSpend}},
		{"fixed, unknown allow_growth keeps other errors", base(fixed, map[string]attr.Value{
			"allow_growth": types.BoolUnknown(), "growth_per_period": types.Float64Value(5), "use_prev_spend": types.BoolValue(true),
		}), []wantBudgetDiag{errFixedGrowthRate, errFixedUsePrevSpend}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := buildBudgetValidatorConfig(ctx, t, tt.values)
			resp := &resource.ValidateConfigResponse{}
			budgetFixedValidator{}.ValidateResource(ctx, resource.ValidateConfigRequest{Config: config}, resp)
			assertBudgetDiagnostics(t, resp.Diagnostics, tt.want)
		})
	}
}

// TestBudgetUsePrevSpendValidator covers use_prev_spend with rollover. A
// configured amount is accepted: the API reads back the previous period's
// spend, so a generated import configuration carries an amount next to
// use_prev_spend = true.
func TestBudgetUsePrevSpendValidator(t *testing.T) {
	ctx := t.Context()
	enabled := testRollover(t, types.BoolValue(true), types.Int64Value(2))
	disabled := testRollover(t, types.BoolValue(false), types.Int64Value(0))

	tests := []struct {
		name   string
		values map[string]attr.Value
		want   []wantBudgetDiag
	}{
		{"nothing set", nil, nil},
		{"use_prev_spend true", map[string]attr.Value{"use_prev_spend": types.BoolValue(true)}, nil},
		{"use_prev_spend true, rollover disabled", map[string]attr.Value{"use_prev_spend": types.BoolValue(true), "rollover": disabled}, nil},
		{"use_prev_spend false, rollover enabled", map[string]attr.Value{"use_prev_spend": types.BoolValue(false), "rollover": enabled}, nil},
		{"use_prev_spend omitted, rollover enabled", map[string]attr.Value{"rollover": enabled}, nil},
		{"use_prev_spend true, generated import values", map[string]attr.Value{
			"use_prev_spend": types.BoolValue(true), "amount": types.Float64Value(0), "rollover": disabled,
		}, nil},

		{"use_prev_spend true, rollover enabled", map[string]attr.Value{"use_prev_spend": types.BoolValue(true), "rollover": enabled}, []wantBudgetDiag{errRolloverOn}},

		{"use_prev_spend unknown, rollover enabled", map[string]attr.Value{"use_prev_spend": types.BoolUnknown(), "rollover": enabled}, nil},
		{"use_prev_spend true, rollover unknown", map[string]attr.Value{"use_prev_spend": types.BoolValue(true), "rollover": resource_budget.NewRolloverValueUnknown()}, nil},
		{"use_prev_spend true, rollover enabled unknown", map[string]attr.Value{"use_prev_spend": types.BoolValue(true), "rollover": testRollover(t, types.BoolUnknown(), types.Int64Value(2))}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := buildBudgetValidatorConfig(ctx, t, tt.values)
			resp := &resource.ValidateConfigResponse{}
			budgetUsePrevSpendValidator{}.ValidateResource(ctx, resource.ValidateConfigRequest{Config: config}, resp)
			assertBudgetDiagnostics(t, resp.Diagnostics, tt.want)
		})
	}
}

// TestBudgetModifyPlanUsePrevSpendRemoval covers turning use_prev_spend off.
// The plan of each row is the one the framework builds: use_prev_spend takes
// its default false when omitted; an omitted amount keeps the stored value
// when nothing else changes and is unknown when the configuration changes too.
func TestBudgetModifyPlanUsePrevSpendRemoval(t *testing.T) {
	ctx := t.Context()
	seasonal := testSeasonalAmounts(12)

	tests := []struct {
		name   string
		create bool
		config map[string]attr.Value
		plan   map[string]attr.Value // planned values that differ from the configuration
		stored map[string]attr.Value
		want   []wantBudgetDiag
	}{
		{"omitted use_prev_spend, removal is the only change", false, nil,
			map[string]attr.Value{"amount": types.Float64Value(500)}, nil, []wantBudgetDiag{errMissingAmount}},
		{"omitted use_prev_spend, with another change", false, map[string]attr.Value{"name": types.StringValue("renamed")},
			map[string]attr.Value{"amount": types.Float64Unknown()}, nil, []wantBudgetDiag{errMissingAmount}},
		{"use_prev_spend false", false, map[string]attr.Value{"use_prev_spend": types.BoolValue(false)},
			map[string]attr.Value{"amount": types.Float64Unknown()}, nil, []wantBudgetDiag{errMissingAmount}},
		{"seasonal empty", false, map[string]attr.Value{"use_prev_spend": types.BoolValue(false), "seasonal_amounts": testSeasonalList()},
			map[string]attr.Value{"amount": types.Float64Unknown()}, nil, []wantBudgetDiag{errMissingAmount}},

		{"amount set", false, map[string]attr.Value{"use_prev_spend": types.BoolValue(false), "amount": types.Float64Value(200)}, nil, nil, nil},
		{"amount zero", false, map[string]attr.Value{"use_prev_spend": types.BoolValue(false), "amount": types.Float64Value(0)}, nil, nil, nil},
		{"amount unknown", false, map[string]attr.Value{"use_prev_spend": types.BoolValue(false), "amount": types.Float64Unknown()}, nil, nil, nil},
		{"seasonal amounts replace it", false, map[string]attr.Value{"use_prev_spend": types.BoolValue(false), "seasonal_amounts": seasonal},
			map[string]attr.Value{"amount": types.Float64Unknown()}, nil, nil},
		{"seasonal amounts unknown", false, map[string]attr.Value{"use_prev_spend": types.BoolValue(false), "seasonal_amounts": types.ListUnknown(types.Float64Type)},
			map[string]attr.Value{"amount": types.Float64Unknown()}, nil, nil},

		{"use_prev_spend stays true", false, map[string]attr.Value{"use_prev_spend": types.BoolValue(true)}, nil, nil, nil},
		{"use_prev_spend unknown", false, map[string]attr.Value{"use_prev_spend": types.BoolUnknown()},
			map[string]attr.Value{"amount": types.Float64Unknown()}, nil, nil},
		{"turned on", false, map[string]attr.Value{"use_prev_spend": types.BoolValue(true)}, nil,
			map[string]attr.Value{"use_prev_spend": types.BoolValue(false), "amount": types.Float64Value(100)}, nil},
		{"not stored", false, map[string]attr.Value{"name": types.StringValue("renamed")},
			map[string]attr.Value{"amount": types.Float64Unknown()},
			map[string]attr.Value{"use_prev_spend": types.BoolValue(false), "amount": types.Float64Value(100)}, nil},
		{"stored null (upgraded state)", false, map[string]attr.Value{"name": types.StringValue("renamed")},
			map[string]attr.Value{"amount": types.Float64Unknown()},
			map[string]attr.Value{"use_prev_spend": types.BoolNull()}, nil},
		{"create", true, map[string]attr.Value{"use_prev_spend": types.BoolValue(false)},
			map[string]attr.Value{"amount": types.Float64Unknown()}, nil, nil},
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

			planValues := maps.Clone(configValues)
			planValues["growth_per_period"] = types.Float64Value(0)
			planValues["allow_growth"] = types.BoolValue(false)
			if _, ok := tt.config["use_prev_spend"]; !ok {
				planValues["use_prev_spend"] = types.BoolValue(false)
			}
			maps.Copy(planValues, tt.plan)
			plan := buildBudgetValidatorConfig(ctx, t, planValues)

			storedValues := map[string]attr.Value{
				"name":              types.StringValue("budget"),
				"type":              types.StringValue("recurring"),
				"start_period":      types.Int64Value(1759276800000),
				"time_interval":     types.StringValue("month"),
				"allow_growth":      types.BoolValue(false),
				"growth_per_period": types.Float64Value(0),
				"use_prev_spend":    types.BoolValue(true),
				"amount":            types.Float64Value(500),
				"seasonal_amounts":  testSeasonalList(),
			}
			maps.Copy(storedValues, tt.stored)
			stateRaw := buildBudgetValidatorConfig(ctx, t, storedValues).Raw
			if tt.create {
				stateRaw = tftypes.NewValue(config.Raw.Type(), nil)
			}

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

// TestBudgetRequestAllowGrowth covers the allowGrowth sent for each plan. The
// API does not infer allowGrowth from a rate on update (it keeps the stored
// value and rejects a rate above 0 next to a stored false), and a fixed budget
// must have it off, so the request builder decides when the plan leaves it
// open.
func TestBudgetRequestAllowGrowth(t *testing.T) {
	boolPtr := func(b bool) *bool { return &b }

	tests := []struct {
		name       string
		budgetType string
		allow      types.Bool
		rate       types.Float64
		wantCreate *bool
		wantUpdate *bool
	}{
		{"known true", "recurring", types.BoolValue(true), types.Float64Value(5), boolPtr(true), boolPtr(true)},
		{"known false", "recurring", types.BoolValue(false), types.Float64Value(0), boolPtr(false), boolPtr(false)},
		{"known false on fixed", "fixed", types.BoolValue(false), types.Float64Value(0), boolPtr(false), boolPtr(false)},
		{"unknown, rate above 0", "recurring", types.BoolUnknown(), types.Float64Value(5), boolPtr(true), boolPtr(true)},
		{"null, rate above 0", "recurring", types.BoolNull(), types.Float64Value(0.5), boolPtr(true), boolPtr(true)},
		{"unknown, rate 0", "recurring", types.BoolUnknown(), types.Float64Value(0), nil, nil},
		{"unknown, rate null", "recurring", types.BoolUnknown(), types.Float64Null(), nil, nil},
		{"unknown, rate unknown", "recurring", types.BoolUnknown(), types.Float64Unknown(), nil, nil},
		{"unknown, negative rate", "recurring", types.BoolUnknown(), types.Float64Value(-1), nil, nil},
		// A switch to fixed has to turn growth off in the request.
		{"unknown, rate 0, fixed", "fixed", types.BoolUnknown(), types.Float64Value(0), nil, boolPtr(false)},
		{"null, rate 0, fixed", "fixed", types.BoolNull(), types.Float64Value(0), nil, boolPtr(false)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := newTestBudgetPlan(t)
			plan.Type = types.StringValue(tt.budgetType)
			plan.AllowGrowth = tt.allow
			plan.GrowthPerPeriod = tt.rate

			createReq, diags := plan.toCreateRequest(t.Context())
			if diags.HasError() {
				t.Fatalf("toCreateRequest: %v", diags)
			}
			updateReq, diags := plan.toUpdateRequest(t.Context())
			if diags.HasError() {
				t.Fatalf("toUpdateRequest: %v", diags)
			}

			assertBoolPtr(t, "create allowGrowth", createReq.AllowGrowth, tt.wantCreate)
			assertBoolPtr(t, "update allowGrowth", updateReq.AllowGrowth, tt.wantUpdate)
		})
	}
}

func assertBoolPtr(t *testing.T, what string, got, want *bool) {
	t.Helper()
	switch {
	case got == nil && want == nil:
	case got == nil || want == nil:
		t.Errorf("%s = %v, want %v", what, derefBool(got), derefBool(want))
	case *got != *want:
		t.Errorf("%s = %v, want %v", what, *got, *want)
	}
}
