package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_allocation"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/oapi-codegen/nullable"
)

// TestToAllocationRuleComponentsListValue_EmptySlice verifies that passing an
// empty-but-non-nil slice does not panic. The function indexes stateComponents[0]
// to get the element type, which would panic with len=0.
func TestToAllocationRuleComponentsListValue_EmptySlice(t *testing.T) {
	ctx := t.Context()

	// This must not panic
	result, diags := toAllocationRuleComponentsListValue(ctx, []models.AllocationComponent{}, nil)

	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if result.IsNull() || result.IsUnknown() {
		t.Error("expected a known, non-null list value for empty components")
	}

	if len(result.Elements()) != 0 {
		t.Errorf("expected 0 elements, got %d", len(result.Elements()))
	}
}

// ---------------------------------------------------------------------------
// Single-rule allocation sends "rules": [] on update → API 500
// ---------------------------------------------------------------------------

// TestFillAllocationCommon_SingleRule_NoEmptyRules guards against a regression
// where fillAllocationCommon would serialize an empty Rules list as "rules": []
// in the API request for single-rule allocations.
//
// Pre-fix behavior: mapAllocationToModel set Rules to an empty list for single
// allocations, and fillAllocationCommon blindly forwarded it, producing
// "rules": [] which the API rejects with a 500 error.
//
// Post-fix behavior: fillAllocationCommon omits req.Rules when len == 0.
func TestFillAllocationCommon_SingleRule_NoEmptyRules(t *testing.T) {
	ctx := t.Context()

	// Simulate the state that mapAllocationToModel produces for a single-rule
	// allocation: Rules is set to an empty list (not null) because of the
	// "always return empty list for user-configurable attributes" pattern.
	emptyRules, d := types.ListValueFrom(
		ctx,
		resource_allocation.RulesValue{}.Type(ctx),
		[]resource_allocation.RulesValue{},
	)
	if d.HasError() {
		t.Fatalf("failed to create empty rules list: %v", d)
	}

	// Build a single-rule component
	comp, compDiags := resource_allocation.NewComponentsValue(
		resource_allocation.ComponentsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"case_insensitive": types.BoolValue(false),
			"include_null":     types.BoolNull(),
			"inverse":          types.BoolNull(),
			"key":              types.StringValue("country"),
			"mode":             types.StringValue("is"),
			"type":             types.StringValue("fixed"),
			"values":           types.ListValueMust(types.StringType, []attr.Value{types.StringValue("JP")}),
		},
	)
	if compDiags.HasError() {
		t.Fatalf("failed to create component: %v", compDiags)
	}

	compList, compListDiags := types.ListValueFrom(ctx, resource_allocation.ComponentsValue{}.Type(ctx), []resource_allocation.ComponentsValue{comp})
	if compListDiags.HasError() {
		t.Fatalf("failed to create component list: %v", compListDiags)
	}

	ruleVal, ruleDiags := resource_allocation.NewRuleValue(
		resource_allocation.RuleValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"formula":          types.StringValue("A"),
			"components":       compList,
			"validity_periods": types.ListNull(resource_allocation.ValidityPeriodsValue{}.Type(ctx)),
		},
	)
	if ruleDiags.HasError() {
		t.Fatalf("failed to create rule: %v", ruleDiags)
	}

	plan := &allocationResourceModel{}
	plan.Id = types.StringValue("test-id")
	plan.Name = types.StringValue("test-single-alloc")
	plan.Description = types.StringValue("test")
	plan.Rule = ruleVal
	plan.Rules = emptyRules // Simulate pre-fix state: empty list instead of null for single allocations
	plan.UnallocatedCosts = types.StringNull()
	plan.AllocationType = types.StringValue("single")
	plan.Type = types.StringNull()

	req := new(models.UpdateAllocationRequest)
	diags := plan.fillAllocationCommon(ctx, req)
	if diags.HasError() {
		t.Fatalf("fillAllocationCommon returned error: %v", diags)
	}

	// Serialize to JSON and check whether "rules" appears
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if _, hasRules := parsed["rules"]; hasRules {
		t.Errorf("BUG: single-rule allocation request contains \"rules\": %s\n"+
			"The API rejects this with 500. For single allocations, \"rules\" must not be present.",
			string(body))
	}
}

// TestMapAllocationToModel_SingleRule_RulesNull verifies that mapAllocationToModel
// keeps Rules null for single-rule allocations. This is the root cause of the 500
// error: the empty list gets carried into the plan and serialized as "rules": [].
func TestMapAllocationToModel_SingleRule_RulesNull(t *testing.T) {
	ctx := t.Context()

	allocType := models.AllocationAllocationType("single")
	apiResp := &models.Allocation{
		AllocationType: &allocType,
		Rule: valueToNullable(models.AllocationRule{
			Formula: "A",
			Components: []models.AllocationComponent{
				{
					Key:    "country",
					Mode:   "is",
					Type:   "fixed",
					Values: []string{"JP"},
				},
			},
		}),
	}

	state := &allocationResourceModel{}
	state.Id = types.StringValue("test-id")
	state.Name = types.StringValue("test")
	state.Rule = resource_allocation.NewRuleValueNull()
	state.Rules = types.ListNull(resource_allocation.RulesValue{}.Type(ctx))

	diags := mapAllocationToModel(ctx, nil, apiResp, state)
	if diags.HasError() {
		t.Fatalf("mapAllocationToModel returned error: %v", diags)
	}

	if !state.Rules.IsNull() {
		t.Errorf("BUG: mapAllocationToModel set Rules to non-null for single allocation.\n"+
			"state.Rules.IsNull()=%v, elements=%d\n"+
			"For single allocations, Rules should remain null so it's omitted from update requests.",
			state.Rules.IsNull(), len(state.Rules.Elements()))
	}
}

func TestFillAllocationCommon_GroupRule_ClearsValueExtraction(t *testing.T) {
	ctx := t.Context()

	plan := &allocationResourceModel{
		Id:             types.StringValue("group-123"),
		Name:           types.StringValue("group-alloc"),
		Description:    types.StringValue("desc"),
		AllocationType: types.StringValue("group"),
		Rule:           resource_allocation.NewRuleValueNull(),
	}

	rule := modifyPlanTestRule(t, map[string]attr.Value{
		"action":           types.StringValue("update"),
		"name":             types.StringValue("rule-1"),
		"description":      types.StringNull(),
		"formula":          types.StringValue("A"),
		"components":       modifyPlanTestComponentList(t, modifyPlanTestComponent(t, "country", []string{"JP"})),
		"id":               types.StringValue("rule-id-1"),
		"value_extraction": resource_allocation.NewValueExtractionValueNull(),
	})
	plan.Rules = types.ListValueMust(resource_allocation.RulesValue{}.Type(ctx), []attr.Value{rule})

	req := new(models.UpdateAllocationRequest)
	diags := plan.fillAllocationCommon(ctx, req)
	if diags.HasError() {
		t.Fatalf("fillAllocationCommon returned error: %v", diags)
	}

	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	rules, ok := parsed["rules"].([]any)
	if !ok || len(rules) == 0 {
		t.Fatalf("expected non-empty rules in serialized request: %s", string(body))
	}
	r0 := rules[0].(map[string]any)
	ve, hasVE := r0["valueExtraction"]
	if !hasVE {
		t.Fatalf("expected valueExtraction to be present in rule payload to clear it, but was omitted: %s", string(body))
	}
	if ve != nil {
		t.Fatalf("expected valueExtraction to be null in rule payload, got: %v", ve)
	}
}

func TestUseNullForUnknownValueExtractionModifier(t *testing.T) {
	ctx := t.Context()
	m := useNullForUnknownValueExtraction()

	attrTypes := resource_allocation.ValueExtractionValue{}.AttributeTypes(ctx)

	// Case 1: Config is null -> plan value should be set to null
	req := planmodifier.ObjectRequest{
		ConfigValue: types.ObjectNull(attrTypes),
		PlanValue:   types.ObjectUnknown(attrTypes),
	}
	resp := &planmodifier.ObjectResponse{
		PlanValue: req.PlanValue,
	}
	m.PlanModifyObject(ctx, req, resp)
	if !resp.PlanValue.IsNull() {
		t.Errorf("expected plan value to be null when config is null, got %v", resp.PlanValue)
	}

	// Case 2: Config is known/not null -> plan value should not be modified to null
	objVal, d := types.ObjectValue(
		attrTypes,
		map[string]attr.Value{
			"fallback":   types.StringValue("fb"),
			"on_missing": types.StringValue("nextRule"),
			"sources":    types.ListNull(resource_allocation.SourcesValue{}.Type(ctx)),
		},
	)
	if d.HasError() {
		t.Fatalf("ObjectValue: %v", d)
	}
	req2 := planmodifier.ObjectRequest{
		ConfigValue: objVal,
		PlanValue:   objVal,
	}
	resp2 := &planmodifier.ObjectResponse{
		PlanValue: req2.PlanValue,
	}
	m.PlanModifyObject(ctx, req2, resp2)
	if resp2.PlanValue.IsNull() {
		t.Error("expected plan value to remain unchanged when config is not null")
	}
}

func TestMapAllocationToModel_ChildAllocationErrorReturnsDiags(t *testing.T) {
	ctx := t.Context()

	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	client := newTestAPIClient(t, server)

	ruleID := "child-rule-1"
	ruleName := "child-rule-name"
	allocType := models.AllocationAllocationTypeGroup
	alloc := &models.Allocation{
		Id:             new("group-1"),
		Name:           new("group-name"),
		AllocationType: &allocType,
		Rules: &[]nullable.Nullable[models.GroupAllocationRule]{
			valueToNullable(models.GroupAllocationRule{
				Id:   &ruleID,
				Name: &ruleName,
				// Formula is nil, so it attempts to fetch child allocation
			}),
		},
	}

	state := &allocationResourceModel{}
	existingRule, d := resource_allocation.NewRulesValue(
		resource_allocation.RulesValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"action":           types.StringValue("update"),
			"components":       types.ListNull(resource_allocation.ComponentsValue{}.Type(ctx)),
			"description":      types.StringNull(),
			"formula":          types.StringNull(),
			"id":               types.StringValue(ruleID),
			"name":             types.StringValue(ruleName),
			"validity_periods": types.ListNull(resource_allocation.ValidityPeriodsValue{}.Type(ctx)),
			"value_extraction": resource_allocation.NewValueExtractionValueNull(),
		},
	)
	if d.HasError() {
		t.Fatalf("NewRulesValue: %v", d)
	}
	state.Rules = types.ListValueMust(resource_allocation.RulesValue{}.Type(ctx), []attr.Value{existingRule})

	diags := mapAllocationToModel(ctx, client, alloc, state)
	if !diags.HasError() {
		t.Fatalf("expected error diagnostic when child allocation fetch fails with 500, got none")
	}
}
