package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestBudgetPeriodValidatorsAttached(t *testing.T) {
	ctx := t.Context()
	var response resource.SchemaResponse
	(&budgetResource{}).Schema(ctx, resource.SchemaRequest{}, &response)
	if response.Diagnostics.HasError() {
		t.Fatal(response.Diagnostics)
	}

	start, ok := response.Schema.Attributes["start_period"].(schema.Int64Attribute)
	if !ok {
		t.Fatal("start_period must be an Int64Attribute")
	}
	interval, ok := response.Schema.Attributes["time_interval"].(schema.StringAttribute)
	if !ok {
		t.Fatal("time_interval must be a StringAttribute")
	}
	if !start.IsOptional() || !start.IsComputed() || !interval.IsOptional() || !interval.IsComputed() {
		t.Fatal("period attributes must remain Optional+Computed")
	}
	var startValidator validator.Int64
	for _, candidate := range start.Validators {
		if _, ok := candidate.(budgetStartPeriodValidator); ok {
			startValidator = candidate
		}
	}
	if startValidator == nil {
		t.Fatal("budgetStartPeriodValidator is not attached to start_period")
	}
	var intervalValidator validator.String
	for _, candidate := range interval.Validators {
		if _, ok := candidate.(budgetTimeIntervalValidator); ok {
			intervalValidator = candidate
		}
	}
	if intervalValidator == nil {
		t.Fatal("budgetTimeIntervalValidator is not attached to time_interval")
	}

	startConfig := buildBudgetValidatorConfig(ctx, t, map[string]attr.Value{
		"type": types.StringValue("recurring"), "time_interval": types.StringValue("day"),
		"start_period": types.Int64Value(budgetMinStartPeriodMs - 1),
	})
	startResponse := &validator.Int64Response{}
	startValidator.ValidateInt64(ctx, validator.Int64Request{Config: startConfig, ConfigValue: types.Int64Value(budgetMinStartPeriodMs - 1), Path: path.Root("start_period")}, startResponse)
	assertBudgetValidatorDiagnostics(t, startResponse.Diagnostics, true, "Invalid Budget Start Period", path.Root("start_period"))

	intervalResponse := &validator.StringResponse{}
	intervalValidator.ValidateString(ctx, validator.StringRequest{ConfigValue: types.StringValue("decade"), Path: path.Root("time_interval")}, intervalResponse)
	assertBudgetValidatorDiagnostics(t, intervalResponse.Diagnostics, true, "Invalid Budget Time Interval", path.Root("time_interval"))
}

func TestBudgetStartPeriodAttributeValidator(t *testing.T) {
	ctx := t.Context()
	cases := []struct {
		name       string
		start      types.Int64
		budgetType types.String
		interval   types.String
		wantError  bool
	}{
		{"null deferred", types.Int64Null(), types.StringValue("recurring"), types.StringValue("day"), false},
		{"unknown deferred", types.Int64Unknown(), types.StringValue("recurring"), types.StringValue("day"), false},
		{"minimum below recurring", types.Int64Value(budgetMinStartPeriodMs - 1), types.StringValue("recurring"), types.StringValue("day"), true},
		{"minimum boundary recurring", types.Int64Value(budgetMinStartPeriodMs), types.StringValue("recurring"), types.StringValue("day"), false},
		{"minimum below fixed", types.Int64Value(budgetMinStartPeriodMs - 1), types.StringValue("fixed"), types.StringNull(), true},
		{"minimum boundary fixed", types.Int64Value(budgetMinStartPeriodMs), types.StringValue("fixed"), types.StringNull(), false},
		{"minimum below unknown type", types.Int64Value(budgetMinStartPeriodMs - 1), types.StringUnknown(), types.StringUnknown(), true},
		{"minimum below unknown interval", types.Int64Value(budgetMinStartPeriodMs - 1), types.StringValue("recurring"), types.StringUnknown(), true},
		{"alignment unknown type deferred", types.Int64Value(budgetMinStartPeriodMs + 1), types.StringUnknown(), types.StringValue("day"), false},
		{"alignment unknown interval deferred", types.Int64Value(budgetMinStartPeriodMs + 1), types.StringValue("recurring"), types.StringUnknown(), false},
		{"fixed need not align", types.Int64Value(budgetMinStartPeriodMs + 1), types.StringValue("fixed"), types.StringValue("month"), false},
		{"day aligned", types.Int64Value(1759276800000), types.StringValue("recurring"), types.StringValue("day"), false},
		{"day unaligned", types.Int64Value(1759276800001), types.StringValue("recurring"), types.StringValue("day"), true},
		{"week aligned", types.Int64Value(1764547200000), types.StringValue("recurring"), types.StringValue("week"), false},
		{"week unaligned", types.Int64Value(1764633600000), types.StringValue("recurring"), types.StringValue("week"), true},
		{"month aligned", types.Int64Value(1759276800000), types.StringValue("recurring"), types.StringValue("month"), false},
		{"month unaligned", types.Int64Value(1759363200000), types.StringValue("recurring"), types.StringValue("month"), true},
		{"quarter aligned", types.Int64Value(1759276800000), types.StringValue("recurring"), types.StringValue("quarter"), false},
		{"quarter unaligned", types.Int64Value(1756684800000), types.StringValue("recurring"), types.StringValue("quarter"), true},
		{"year aligned", types.Int64Value(1735689600000), types.StringValue("recurring"), types.StringValue("year"), false},
		{"year unaligned", types.Int64Value(1759276800000), types.StringValue("recurring"), types.StringValue("year"), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config := buildBudgetValidatorConfig(ctx, t, map[string]attr.Value{"type": tc.budgetType, "time_interval": tc.interval, "start_period": tc.start})
			resp := &validator.Int64Response{}
			budgetStartPeriodValidator{}.ValidateInt64(ctx, validator.Int64Request{Config: config, ConfigValue: tc.start, Path: path.Root("start_period")}, resp)
			assertBudgetValidatorDiagnostics(t, resp.Diagnostics, tc.wantError, "Invalid Budget Start Period", path.Root("start_period"))
		})
	}
}

func TestBudgetTimeIntervalAttributeValidator(t *testing.T) {
	for _, tc := range []struct {
		name      string
		value     types.String
		wantError bool
	}{
		{"null", types.StringNull(), false}, {"unknown", types.StringUnknown(), false},
		{"day", types.StringValue("day"), false}, {"week", types.StringValue("week"), false},
		{"month", types.StringValue("month"), false}, {"quarter", types.StringValue("quarter"), false},
		{"year", types.StringValue("year"), false}, {"invalid", types.StringValue("decade"), true},
		{"empty", types.StringValue(""), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := &validator.StringResponse{}
			budgetTimeIntervalValidator{}.ValidateString(t.Context(), validator.StringRequest{ConfigValue: tc.value, Path: path.Root("time_interval")}, resp)
			assertBudgetValidatorDiagnostics(t, resp.Diagnostics, tc.wantError, "Invalid Budget Time Interval", path.Root("time_interval"))
		})
	}
}

func TestBudgetPeriodModifyPlan(t *testing.T) {
	ctx := t.Context()
	cases := []struct {
		name        string
		start       types.Int64
		budgetType  types.String
		interval    types.String
		update      bool
		destroy     bool
		wantPath    string
		wantSummary string
	}{
		{"missing start", types.Int64Null(), types.StringValue("fixed"), types.StringNull(), false, false, "start_period", "Missing Required Attribute"},
		{"unknown start", types.Int64Unknown(), types.StringValue("fixed"), types.StringNull(), false, false, "", ""},
		{"known start", types.Int64Value(budgetMinStartPeriodMs), types.StringValue("fixed"), types.StringNull(), false, false, "", ""},
		{"missing recurring interval", types.Int64Value(budgetMinStartPeriodMs), types.StringValue("recurring"), types.StringNull(), false, false, "time_interval", "Missing Required Attribute"},
		{"unknown recurring interval", types.Int64Value(budgetMinStartPeriodMs), types.StringValue("recurring"), types.StringUnknown(), false, false, "", ""},
		{"known recurring interval", types.Int64Value(budgetMinStartPeriodMs), types.StringValue("recurring"), types.StringValue("day"), false, false, "", ""},
		{"unknown budget type", types.Int64Value(budgetMinStartPeriodMs), types.StringUnknown(), types.StringNull(), false, false, "", ""},
		{"unknown budget type does not hide missing start", types.Int64Null(), types.StringUnknown(), types.StringNull(), false, false, "start_period", "Missing Required Attribute"},
		{"fixed explicit month", types.Int64Value(budgetMinStartPeriodMs), types.StringValue("fixed"), types.StringValue("month"), false, false, "", ""},
		{"fixed other interval", types.Int64Value(budgetMinStartPeriodMs), types.StringValue("fixed"), types.StringValue("week"), false, false, "time_interval", "Invalid Budget Time Interval"},
		{"update omitted fields", types.Int64Null(), types.StringValue("recurring"), types.StringNull(), true, false, "", ""},
		{"destroy", types.Int64Null(), types.StringValue("recurring"), types.StringNull(), false, true, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config := buildBudgetValidatorConfig(ctx, t, map[string]attr.Value{"start_period": tc.start, "type": tc.budgetType, "time_interval": tc.interval})
			nullRaw := tftypes.NewValue(config.Raw.Type(), nil)
			stateRaw := nullRaw
			if tc.update {
				prior := buildBudgetValidatorConfig(ctx, t, map[string]attr.Value{
					"start_period":  types.Int64Value(budgetMinStartPeriodMs),
					"type":          types.StringValue("recurring"),
					"time_interval": types.StringValue("day"),
				})
				stateRaw = prior.Raw
			}
			planRaw := config.Raw
			if tc.destroy {
				planRaw = nullRaw
			}
			resp := &resource.ModifyPlanResponse{}
			(&budgetResource{}).ModifyPlan(ctx, resource.ModifyPlanRequest{
				Config: config,
				State:  tfsdk.State{Schema: config.Schema, Raw: stateRaw},
				Plan:   tfsdk.Plan{Schema: config.Schema, Raw: planRaw},
			}, resp)
			assertBudgetValidatorDiagnostics(t, resp.Diagnostics, tc.wantPath != "", tc.wantSummary, path.Root(tc.wantPath))
		})
	}
}

func TestBudgetPeriodModifyPlanBothRequiredFieldsMissing(t *testing.T) {
	ctx := t.Context()
	config := buildBudgetValidatorConfig(ctx, t, map[string]attr.Value{
		"type":          types.StringValue("recurring"),
		"start_period":  types.Int64Null(),
		"time_interval": types.StringNull(),
	})
	resp := &resource.ModifyPlanResponse{}
	(&budgetResource{}).ModifyPlan(ctx, resource.ModifyPlanRequest{
		Config: config,
		State:  tfsdk.State{Schema: config.Schema, Raw: tftypes.NewValue(config.Raw.Type(), nil)},
		Plan:   tfsdk.Plan(config),
	}, resp)
	if len(resp.Diagnostics) != 2 {
		t.Fatalf("diagnostic count = %d, want 2: %v", len(resp.Diagnostics), resp.Diagnostics)
	}
	for i, attribute := range []string{"start_period", "time_interval"} {
		assertBudgetValidatorDiagnostics(t, resp.Diagnostics[i:i+1], true, "Missing Required Attribute", path.Root(attribute))
	}
}
