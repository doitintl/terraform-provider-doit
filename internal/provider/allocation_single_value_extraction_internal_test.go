package provider

import (
	"encoding/json"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_allocation"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// singleRuleVE builds the value_extraction value a plan holds for a configured block.
func singleRuleVE(t *testing.T) resource_allocation.ValueExtractionValue {
	t.Helper()
	ve, diags := toValueExtractionValue(t.Context(), valueToNullable(models.AllocationValueExtraction{
		OnMissing: new(models.AllocationValueExtractionOnMissingUseFallback),
		Fallback:  valueToNullable("unknown"),
		Sources: []models.AllocationValueExtractionSource{
			{Type: models.AllocationValueExtractionSourceType("label"), Key: "team"},
		},
	}))
	if diags.HasError() {
		t.Fatalf("toValueExtractionValue: %v", diags)
	}
	return ve
}

// singleRulePlan builds a single-allocation plan whose rule carries the given
// value_extraction and validity_periods values.
func singleRulePlan(t *testing.T, ve resource_allocation.ValueExtractionValue, vp types.List) *allocationResourceModel {
	t.Helper()
	ctx := t.Context()
	rule, diags := resource_allocation.NewRuleValue(
		resource_allocation.RuleValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"formula":          types.StringValue("A"),
			"components":       types.ListValueMust(resource_allocation.ComponentsValue{}.Type(ctx), []attr.Value{}),
			"validity_periods": vp,
			"value_extraction": ve,
		},
	)
	if diags.HasError() {
		t.Fatalf("NewRuleValue: %v", diags)
	}
	return &allocationResourceModel{
		Id:             types.StringValue("single-1"),
		Name:           types.StringValue("single"),
		AllocationType: types.StringValue("single"),
		Rule:           rule,
		Rules:          types.ListNull(resource_allocation.RulesValue{}.Type(ctx)),
	}
}

// ruleJSON serializes the update request built from plan and returns its "rule" object.
func ruleJSON(t *testing.T, plan *allocationResourceModel) map[string]any {
	t.Helper()
	req := new(models.UpdateAllocationRequest)
	if diags := plan.fillAllocationCommon(t.Context(), req); diags.HasError() {
		t.Fatalf("fillAllocationCommon: %v", diags)
	}
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	rule, ok := parsed["rule"].(map[string]any)
	if !ok {
		t.Fatalf("request has no rule object: %s", body)
	}
	return rule
}

// The single PATCH keeps an omitted valueExtraction and clears only an explicit null,
// so the request must distinguish a configured block, a removed block and an unknown one.
func TestFillAllocationCommon_SingleRule_ValueExtraction(t *testing.T) {
	ctx := t.Context()
	emptyVP := types.ListValueMust(resource_allocation.ValidityPeriodsValue{}.Type(ctx), []attr.Value{})

	tests := []struct {
		name string
		ve   resource_allocation.ValueExtractionValue
		// wantKey reports whether valueExtraction must be present in the rule.
		wantKey bool
		// wantNull reports whether the present key must be an explicit JSON null.
		wantNull bool
	}{
		{name: "configured block is sent", ve: singleRuleVE(t), wantKey: true},
		{name: "removed block sends explicit null", ve: resource_allocation.NewValueExtractionValueNull(), wantKey: true, wantNull: true},
		{name: "unknown is left out (API keeps the stored value)", ve: resource_allocation.NewValueExtractionValueUnknown()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := ruleJSON(t, singleRulePlan(t, tt.ve, emptyVP))
			got, present := rule["valueExtraction"]
			if present != tt.wantKey {
				t.Fatalf("valueExtraction present=%v, want %v (rule=%v)", present, tt.wantKey, rule)
			}
			if !tt.wantKey {
				return
			}
			if tt.wantNull {
				if got != nil {
					t.Fatalf("valueExtraction = %v, want explicit null", got)
				}
				return
			}
			obj, ok := got.(map[string]any)
			if !ok {
				t.Fatalf("valueExtraction = %v, want an object", got)
			}
			if obj["fallback"] != "unknown" || obj["onMissing"] != "useFallback" {
				t.Errorf("unexpected valueExtraction object: %v", obj)
			}
			sources, _ := obj["sources"].([]any)
			if len(sources) != 1 {
				t.Fatalf("sources = %v, want one source", obj["sources"])
			}
		})
	}
}

// The API treats a null validityPeriods like an omitted one (keeps the periods); only
// an empty array clears them, so a null plan must be sent as [].
func TestFillAllocationCommon_SingleRule_ValidityPeriodsNullSendsEmptyArray(t *testing.T) {
	ctx := t.Context()
	vpType := resource_allocation.ValidityPeriodsValue{}.Type(ctx)

	tests := []struct {
		name    string
		vp      types.List
		wantKey bool
		wantLen int
	}{
		{name: "null plan clears with []", vp: types.ListNull(vpType), wantKey: true},
		{name: "empty list clears with []", vp: types.ListValueMust(vpType, []attr.Value{}), wantKey: true},
		{name: "unknown is left out", vp: types.ListUnknown(vpType)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := ruleJSON(t, singleRulePlan(t, resource_allocation.NewValueExtractionValueNull(), tt.vp))
			got, present := rule["validityPeriods"]
			if present != tt.wantKey {
				t.Fatalf("validityPeriods present=%v, want %v (rule=%v)", present, tt.wantKey, rule)
			}
			if !tt.wantKey {
				return
			}
			periods, ok := got.([]any)
			if !ok || len(periods) != tt.wantLen {
				t.Fatalf("validityPeriods = %#v, want an empty array", got)
			}
		})
	}
}

// A single GET returns rule.valueExtraction itself; an absent value maps to a typed null.
func TestMapAllocationToModel_SingleRule_ValueExtraction(t *testing.T) {
	ctx := t.Context()
	allocType := models.AllocationAllocationType("single")

	newState := func() *allocationResourceModel {
		return &allocationResourceModel{
			Id:    types.StringValue("single-1"),
			Name:  types.StringValue("single"),
			Rule:  resource_allocation.NewRuleValueNull(),
			Rules: types.ListNull(resource_allocation.RulesValue{}.Type(ctx)),
		}
	}
	apiRule := func(ve *models.AllocationValueExtraction) *models.Allocation {
		rule := models.AllocationRule{Formula: "A", Components: []models.AllocationComponent{}}
		if ve != nil {
			rule.ValueExtraction = valueToNullable(*ve)
		}
		return &models.Allocation{AllocationType: &allocType, Rule: valueToNullable(rule)}
	}

	t.Run("object is mapped", func(t *testing.T) {
		state := newState()
		resp := apiRule(&models.AllocationValueExtraction{
			OnMissing: new(models.AllocationValueExtractionOnMissingUseFallback),
			Fallback:  valueToNullable("unknown"),
			Sources: []models.AllocationValueExtractionSource{
				{Type: models.AllocationValueExtractionSourceType("label"), Key: "team"},
			},
		})
		if diags := mapAllocationToModel(ctx, nil, resp, state); diags.HasError() {
			t.Fatalf("mapAllocationToModel: %v", diags)
		}
		ve := state.Rule.ValueExtraction
		if ve.IsNull() || ve.IsUnknown() {
			t.Fatalf("value_extraction = %v, want a known object", ve)
		}
		if ve.Fallback.ValueString() != "unknown" || ve.OnMissing.ValueString() != "useFallback" {
			t.Errorf("unexpected mapping: fallback=%v on_missing=%v", ve.Fallback, ve.OnMissing)
		}
		if len(ve.Sources.Elements()) != 1 {
			t.Errorf("sources = %v, want one element", ve.Sources)
		}
	})

	t.Run("absent value maps to null", func(t *testing.T) {
		state := newState()
		if diags := mapAllocationToModel(ctx, nil, apiRule(nil), state); diags.HasError() {
			t.Fatalf("mapAllocationToModel: %v", diags)
		}
		if !state.Rule.ValueExtraction.IsNull() {
			t.Errorf("value_extraction = %v, want null", state.Rule.ValueExtraction)
		}
	})
}

// The overlay resolves an unknown value_extraction from the API response and leaves a
// configured one alone.
func TestOverlayAllocationSingleRule_ValueExtraction(t *testing.T) {
	ctx := t.Context()
	vpType := resource_allocation.ValidityPeriodsValue{}.Type(ctx)
	emptyVP := types.ListValueMust(vpType, []attr.Value{})

	t.Run("unknown takes the resolved value", func(t *testing.T) {
		resolved := singleRulePlan(t, singleRuleVE(t), emptyVP).Rule
		plan := singleRulePlan(t, resource_allocation.NewValueExtractionValueUnknown(), emptyVP).Rule
		if diags := overlayAllocationSingleRule(ctx, &resolved, &plan); diags.HasError() {
			t.Fatalf("overlay: %v", diags)
		}
		if plan.ValueExtraction.IsUnknown() || plan.ValueExtraction.IsNull() {
			t.Fatalf("value_extraction = %v, want the resolved object", plan.ValueExtraction)
		}
	})

	t.Run("null plan stays null", func(t *testing.T) {
		resolved := singleRulePlan(t, singleRuleVE(t), emptyVP).Rule
		plan := singleRulePlan(t, resource_allocation.NewValueExtractionValueNull(), emptyVP).Rule
		if diags := overlayAllocationSingleRule(ctx, &resolved, &plan); diags.HasError() {
			t.Fatalf("overlay: %v", diags)
		}
		if !plan.ValueExtraction.IsNull() {
			t.Fatalf("value_extraction = %v, want null (plan wins)", plan.ValueExtraction)
		}
	})
}
