package provider

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_allocation"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestToAllocationRuleComponentsListValue_EmptySlice verifies that passing an
// empty-but-non-nil slice does not panic. The function indexes stateComponents[0]
// to get the element type, which would panic with len=0.
func TestToAllocationRuleComponentsListValue_EmptySlice(t *testing.T) {
	ctx := context.Background()

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
	ctx := context.Background()

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
			"case_insensitive":  types.BoolValue(false),
			"include_null":      types.BoolNull(),
			"inverse":           types.BoolNull(),
			"inverse_selection": types.BoolNull(),
			"key":               types.StringValue("country"),
			"mode":              types.StringValue("is"),
			"type":              types.StringValue("fixed"),
			"values":            types.ListValueMust(types.StringType, []attr.Value{types.StringValue("JP")}),
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
			"formula":    types.StringValue("A"),
			"components": compList,
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

	var parsed map[string]interface{}
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
	ctx := context.Background()

	allocType := models.AllocationAllocationType("single")
	apiResp := &models.Allocation{
		AllocationType: &allocType,
		Rule: &models.AllocationRule{
			Formula: "A",
			Components: []models.AllocationComponent{
				{
					Key:    "country",
					Mode:   "is",
					Type:   "fixed",
					Values: []string{"JP"},
				},
			},
		},
		Rules: nil, // single-rule allocation: no group rules
	}

	state := &allocationResourceModel{}
	state.Id = types.StringValue("test-id")
	state.Name = types.StringValue("test")
	state.Rule = resource_allocation.NewRuleValueNull()
	state.Rules = types.ListNull(resource_allocation.RulesValue{}.Type(ctx))

	r := &allocationResource{}
	diags := r.mapAllocationToModel(ctx, apiResp, state)
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
