package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_allocation"
)

// TestAllocationResource_ModifyPlan_AllowUnknownElements confirms that allowUnknown=true is required
// when unmarshaling an unknown plan rules list or unknown rule elements during plan modification.
func TestAllocationResource_ModifyPlan_AllowUnknownElements(t *testing.T) {
	ctx := context.Background()

	// Construct an unknown rules list (e.g. from an unknown variable or resource dependency)
	unknownListVal := types.ListUnknown(resource_allocation.RulesValue{}.Type(ctx))

	// 1. Verify allowUnknown = false fails with diagnostic error on unknown list value
	var planRulesFalse []resource_allocation.RulesValue
	diagsFalse := unknownListVal.ElementsAs(ctx, &planRulesFalse, false)
	if !diagsFalse.HasError() {
		t.Errorf("expected error diagnostic with allowUnknown=false on unknown list, got none")
	}

	// 2. Verify allowUnknown = true succeeds without error on unknown list value
	var planRulesTrue []resource_allocation.RulesValue
	diagsTrue := unknownListVal.ElementsAs(ctx, &planRulesTrue, true)
	if diagsTrue.HasError() {
		t.Errorf("expected no error diagnostic with allowUnknown=true on unknown list, got: %v", diagsTrue)
	}
}

func modifyPlanTestSchema(t *testing.T) schema.Schema {
	t.Helper()
	ctx := context.Background()
	r := &allocationResource{}
	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	return schemaResp.Schema
}

func modifyPlanTestTimeouts(t *testing.T, sch schema.Schema) timeouts.Value {
	t.Helper()
	timeoutsAttrTypes := make(map[string]attr.Type)
	if timeoutsSingle, ok := sch.Attributes["timeouts"].(schema.SingleNestedAttribute); ok {
		for k, v := range timeoutsSingle.Attributes {
			timeoutsAttrTypes[k] = v.GetType()
		}
	}
	return timeouts.Value{
		Object: types.ObjectNull(timeoutsAttrTypes),
	}
}

func modifyPlanTestRule(t *testing.T, attrs map[string]attr.Value) resource_allocation.RulesValue {
	t.Helper()
	ctx := context.Background()
	return resource_allocation.NewRulesValueMust(
		resource_allocation.RulesValue{}.AttributeTypes(ctx),
		attrs,
	)
}

func modifyPlanTestComponent(t *testing.T, key string, values []string) resource_allocation.ComponentsValue {
	t.Helper()
	ctx := context.Background()
	valElems := make([]attr.Value, len(values))
	for i, v := range values {
		valElems[i] = types.StringValue(v)
	}
	return resource_allocation.NewComponentsValueMust(
		resource_allocation.ComponentsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"key":               types.StringValue(key),
			"mode":              types.StringValue("is"),
			"type":              types.StringValue("fixed"),
			"values":            types.ListValueMust(types.StringType, valElems),
			"case_insensitive":  types.BoolNull(),
			"include_null":      types.BoolNull(),
			"inverse":           types.BoolNull(),
			"inverse_selection": types.BoolNull(),
		},
	)
}

func modifyPlanTestComponentList(t *testing.T, components ...resource_allocation.ComponentsValue) basetypes.ListValue {
	t.Helper()
	ctx := context.Background()
	elems := make([]attr.Value, len(components))
	for i, c := range components {
		elems[i] = c
	}
	return types.ListValueMust(resource_allocation.ComponentsValue{}.Type(ctx), elems)
}

func modifyPlanTestModel(t *testing.T, rules []attr.Value, tv timeouts.Value) allocationResourceModel {
	t.Helper()
	ctx := context.Background()
	return allocationResourceModel{
		AllocationModel: resource_allocation.AllocationModel{
			Id:               types.StringValue("group-123"),
			Name:             types.StringValue("group-alloc"),
			Description:      types.StringValue("desc"),
			Rules:            types.ListValueMust(resource_allocation.RulesValue{}.Type(ctx), rules),
			AllocationType:   types.StringValue("group"),
			AnomalyDetection: types.BoolValue(false),
			CreateTime:       types.Int64Value(100),
			UpdateTime:       types.Int64Value(200),
			FolderId:         types.StringNull(),
			Rule:             resource_allocation.NewRuleValueNull(),
			Type:             types.StringNull(),
			UnallocatedCosts: types.StringNull(),
		},
		Timeouts: tv,
	}
}

type modifyPlanResult struct {
	rules       []resource_allocation.RulesValue
	diagnostics diag.Diagnostics
}

func runModifyPlan(t *testing.T, sch schema.Schema, stateModel, planModel allocationResourceModel) modifyPlanResult {
	t.Helper()
	ctx := context.Background()
	r := &allocationResource{}

	attrTypes := sch.Type().(basetypes.ObjectType).AttrTypes
	stateObj, diags := types.ObjectValueFrom(ctx, attrTypes, stateModel)
	if diags.HasError() {
		t.Fatalf("unexpected state obj diags: %v", diags)
	}
	stateRaw, err := stateObj.ToTerraformValue(ctx)
	if err != nil {
		t.Fatalf("unexpected state tf value err: %v", err)
	}

	planObj, diags := types.ObjectValueFrom(ctx, attrTypes, planModel)
	if diags.HasError() {
		t.Fatalf("unexpected plan obj diags: %v", diags)
	}
	planRaw, err := planObj.ToTerraformValue(ctx)
	if err != nil {
		t.Fatalf("unexpected plan tf value err: %v", err)
	}

	req := resource.ModifyPlanRequest{
		State: tfsdk.State{Raw: stateRaw, Schema: sch},
		Plan:  tfsdk.Plan{Raw: planRaw, Schema: sch},
	}
	resp := resource.ModifyPlanResponse{
		Plan: tfsdk.Plan{Raw: planRaw, Schema: sch},
	}

	r.ModifyPlan(ctx, req, &resp)

	var resultModel allocationResourceModel
	diags = resp.Plan.Get(ctx, &resultModel)
	if diags.HasError() {
		t.Fatalf("unexpected plan get diags: %v", diags)
	}

	var resultRules []resource_allocation.RulesValue
	diags = resultModel.Rules.ElementsAs(ctx, &resultRules, false)
	if diags.HasError() {
		t.Fatalf("unexpected rules ElementsAs diags: %v", diags)
	}
	return modifyPlanResult{rules: resultRules, diagnostics: resp.Diagnostics}
}

// TestAllocationResource_ModifyPlan_DeleteAndRenameRuleWithoutId verifies that when Rule A (index 0)
// is deleted and Rule B (index 1) is renamed, tiered matching assigns Rule B's ID via Tier 1
// (formula+components match).
func TestAllocationResource_ModifyPlan_DeleteAndRenameRuleWithoutId(t *testing.T) {
	sch := modifyPlanTestSchema(t)
	tv := modifyPlanTestTimeouts(t, sch)

	compJP := modifyPlanTestComponentList(t, modifyPlanTestComponent(t, "country", []string{"JP"}))
	compUS := modifyPlanTestComponentList(t, modifyPlanTestComponent(t, "country", []string{"US"}))

	ruleA := modifyPlanTestRule(t, map[string]attr.Value{
		"id":          types.StringValue("id-a"),
		"name":        types.StringValue("rule-a"),
		"action":      types.StringValue("create"),
		"formula":     types.StringValue("A AND B"),
		"description": types.StringNull(),
		"components":  compJP,
	})
	ruleB := modifyPlanTestRule(t, map[string]attr.Value{
		"id":          types.StringValue("id-b"),
		"name":        types.StringValue("rule-b"),
		"action":      types.StringValue("create"),
		"formula":     types.StringValue("A AND B"),
		"description": types.StringNull(),
		"components":  compUS,
	})
	stateModel := modifyPlanTestModel(t, []attr.Value{ruleA, ruleB}, tv)

	ruleBRenamed := modifyPlanTestRule(t, map[string]attr.Value{
		"id":          types.StringNull(),
		"name":        types.StringValue("rule-b-renamed"),
		"action":      types.StringValue("update"),
		"formula":     types.StringValue("A AND B"),
		"description": types.StringNull(),
		"components":  compUS,
	})
	planModel := modifyPlanTestModel(t, []attr.Value{ruleBRenamed}, tv)

	result := runModifyPlan(t, sch, stateModel, planModel)

	if got := result.rules[0].Id.ValueString(); got != "id-b" {
		t.Errorf("expected rule-b-renamed to get id-b (Tier 1: formula+components match), got %q", got)
	}
}

// TestAllocationResource_ModifyPlan_RenameAndFormulaChange verifies Tier 2: when a rule is renamed
// AND has its formula changed, but components stay the same, the components-only match assigns the ID.
func TestAllocationResource_ModifyPlan_RenameAndFormulaChange(t *testing.T) {
	sch := modifyPlanTestSchema(t)
	tv := modifyPlanTestTimeouts(t, sch)

	comp := modifyPlanTestComponentList(t, modifyPlanTestComponent(t, "country", []string{"JP"}))

	ruleA := modifyPlanTestRule(t, map[string]attr.Value{
		"id":          types.StringValue("id-a"),
		"name":        types.StringValue("rule-a"),
		"action":      types.StringValue("create"),
		"formula":     types.StringValue("A"),
		"description": types.StringNull(),
		"components":  comp,
	})
	stateModel := modifyPlanTestModel(t, []attr.Value{ruleA}, tv)

	ruleRenamed := modifyPlanTestRule(t, map[string]attr.Value{
		"id":          types.StringNull(),
		"name":        types.StringValue("rule-a-renamed"),
		"action":      types.StringValue("update"),
		"formula":     types.StringValue("A-new"),
		"description": types.StringNull(),
		"components":  comp,
	})
	planModel := modifyPlanTestModel(t, []attr.Value{ruleRenamed}, tv)

	result := runModifyPlan(t, sch, stateModel, planModel)

	if got := result.rules[0].Id.ValueString(); got != "id-a" {
		t.Errorf("expected Tier 2 (components match) to assign id-a, got %q", got)
	}
}

// TestAllocationResource_ModifyPlan_RenameAndComponentChange verifies Tier 3: when a rule is renamed
// AND has its components changed, but formula stays the same (and is unique), the formula-only match
// assigns the ID.
func TestAllocationResource_ModifyPlan_RenameAndComponentChange(t *testing.T) {
	sch := modifyPlanTestSchema(t)
	tv := modifyPlanTestTimeouts(t, sch)

	compJP := modifyPlanTestComponentList(t, modifyPlanTestComponent(t, "country", []string{"JP"}))
	compUS := modifyPlanTestComponentList(t, modifyPlanTestComponent(t, "country", []string{"US"}))

	ruleA := modifyPlanTestRule(t, map[string]attr.Value{
		"id":          types.StringValue("id-a"),
		"name":        types.StringValue("rule-a"),
		"action":      types.StringValue("create"),
		"formula":     types.StringValue("A"),
		"description": types.StringNull(),
		"components":  compJP,
	})
	stateModel := modifyPlanTestModel(t, []attr.Value{ruleA}, tv)

	ruleRenamed := modifyPlanTestRule(t, map[string]attr.Value{
		"id":          types.StringNull(),
		"name":        types.StringValue("rule-a-renamed"),
		"action":      types.StringValue("update"),
		"formula":     types.StringValue("A"),
		"description": types.StringNull(),
		"components":  compUS,
	})
	planModel := modifyPlanTestModel(t, []attr.Value{ruleRenamed}, tv)

	result := runModifyPlan(t, sch, stateModel, planModel)

	if got := result.rules[0].Id.ValueString(); got != "id-a" {
		t.Errorf("expected Tier 3 (formula match) to assign id-a, got %q", got)
	}
}

// TestAllocationResource_ModifyPlan_RenameFormulaAndComponentChange verifies that when all three
// fields change (name, formula, components), no tier can match. A warning is emitted for
// action="update" rules.
func TestAllocationResource_ModifyPlan_RenameFormulaAndComponentChange(t *testing.T) {
	sch := modifyPlanTestSchema(t)
	tv := modifyPlanTestTimeouts(t, sch)

	compJP := modifyPlanTestComponentList(t, modifyPlanTestComponent(t, "country", []string{"JP"}))
	compUS := modifyPlanTestComponentList(t, modifyPlanTestComponent(t, "country", []string{"US"}))

	ruleA := modifyPlanTestRule(t, map[string]attr.Value{
		"id":          types.StringValue("id-a"),
		"name":        types.StringValue("rule-a"),
		"action":      types.StringValue("create"),
		"formula":     types.StringValue("A"),
		"description": types.StringNull(),
		"components":  compJP,
	})
	stateModel := modifyPlanTestModel(t, []attr.Value{ruleA}, tv)

	ruleChanged := modifyPlanTestRule(t, map[string]attr.Value{
		"id":          types.StringNull(),
		"name":        types.StringValue("rule-a-renamed"),
		"action":      types.StringValue("update"),
		"formula":     types.StringValue("A-new"),
		"description": types.StringNull(),
		"components":  compUS,
	})
	planModel := modifyPlanTestModel(t, []attr.Value{ruleChanged}, tv)

	result := runModifyPlan(t, sch, stateModel, planModel)

	if !result.rules[0].Id.IsNull() {
		t.Errorf("expected no ID match when name, formula, and components all change, got %q", result.rules[0].Id.ValueString())
	}

	if !result.diagnostics.HasError() {
		t.Error("expected error diagnostic for unmatched action=update rule")
	}
}

// TestAllocationResource_ModifyPlan_DeleteAndRenameSameFormula verifies Tier 1 with duplicate
// formulas: two rules share formula "A AND B" but have different components. When one is deleted
// and the other renamed, Tier 1 (formula+components) matches the correct surviving rule.
func TestAllocationResource_ModifyPlan_DeleteAndRenameSameFormula(t *testing.T) {
	sch := modifyPlanTestSchema(t)
	tv := modifyPlanTestTimeouts(t, sch)

	compJP := modifyPlanTestComponentList(t, modifyPlanTestComponent(t, "country", []string{"JP"}))
	compUS := modifyPlanTestComponentList(t, modifyPlanTestComponent(t, "country", []string{"US"}))

	ruleJP := modifyPlanTestRule(t, map[string]attr.Value{
		"id":          types.StringValue("id-jp"),
		"name":        types.StringValue("jp-rule"),
		"action":      types.StringValue("create"),
		"formula":     types.StringValue("A AND B"),
		"description": types.StringNull(),
		"components":  compJP,
	})
	ruleUS := modifyPlanTestRule(t, map[string]attr.Value{
		"id":          types.StringValue("id-us"),
		"name":        types.StringValue("us-rule"),
		"action":      types.StringValue("create"),
		"formula":     types.StringValue("A AND B"),
		"description": types.StringNull(),
		"components":  compUS,
	})
	stateModel := modifyPlanTestModel(t, []attr.Value{ruleJP, ruleUS}, tv)

	// Delete JP, rename US.
	ruleUSRenamed := modifyPlanTestRule(t, map[string]attr.Value{
		"id":          types.StringNull(),
		"name":        types.StringValue("us-rule-renamed"),
		"action":      types.StringValue("update"),
		"formula":     types.StringValue("A AND B"),
		"description": types.StringNull(),
		"components":  compUS,
	})
	planModel := modifyPlanTestModel(t, []attr.Value{ruleUSRenamed}, tv)

	result := runModifyPlan(t, sch, stateModel, planModel)

	if got := result.rules[0].Id.ValueString(); got != "id-us" {
		t.Errorf("expected Tier 1 to match us-rule by formula+components, got %q", got)
	}

	// Verify that the deleted JP rule's ID was NOT assigned.
	if got := result.rules[0].Id.ValueString(); got == "id-jp" {
		t.Errorf("incorrectly assigned deleted JP rule's ID to renamed US rule")
	}

	if result.diagnostics.HasError() {
		t.Errorf("unexpected errors: %v", result.diagnostics)
	}
}

// TestAllocationResource_ModifyPlan_AmbiguousFormulaRenameWithComponentChange verifies that when
// multiple rules share a formula AND the renamed rule's components also change, no tier can
// disambiguate and the rule is left unmatched.
func TestAllocationResource_ModifyPlan_AmbiguousFormulaRenameWithComponentChange(t *testing.T) {
	sch := modifyPlanTestSchema(t)
	tv := modifyPlanTestTimeouts(t, sch)

	compJP := modifyPlanTestComponentList(t, modifyPlanTestComponent(t, "country", []string{"JP"}))
	compUS := modifyPlanTestComponentList(t, modifyPlanTestComponent(t, "country", []string{"US"}))
	compFR := modifyPlanTestComponentList(t, modifyPlanTestComponent(t, "country", []string{"FR"}))

	ruleJP := modifyPlanTestRule(t, map[string]attr.Value{
		"id":          types.StringValue("id-jp"),
		"name":        types.StringValue("jp-rule"),
		"action":      types.StringValue("create"),
		"formula":     types.StringValue("A AND B"),
		"description": types.StringNull(),
		"components":  compJP,
	})
	ruleUS := modifyPlanTestRule(t, map[string]attr.Value{
		"id":          types.StringValue("id-us"),
		"name":        types.StringValue("us-rule"),
		"action":      types.StringValue("create"),
		"formula":     types.StringValue("A AND B"),
		"description": types.StringNull(),
		"components":  compUS,
	})
	stateModel := modifyPlanTestModel(t, []attr.Value{ruleJP, ruleUS}, tv)

	// Rename JP and change its components to FR. Formula still shared → ambiguous.
	ruleRenamed := modifyPlanTestRule(t, map[string]attr.Value{
		"id":          types.StringNull(),
		"name":        types.StringValue("fr-rule"),
		"action":      types.StringValue("update"),
		"formula":     types.StringValue("A AND B"),
		"description": types.StringNull(),
		"components":  compFR,
	})
	planModel := modifyPlanTestModel(t, []attr.Value{ruleRenamed}, tv)

	result := runModifyPlan(t, sch, stateModel, planModel)

	if !result.rules[0].Id.IsNull() {
		t.Errorf("expected no match when formula is ambiguous and components changed, got %q", result.rules[0].Id.ValueString())
	}

	if !result.diagnostics.HasError() {
		t.Error("expected error diagnostic for unmatched action=update rule")
	}
}

// TestAllocationResource_ModifyPlan_PostImportSelectRules verifies that after ImportState (where
// fillAllocationCommon defaults all rules to action="select"), ModifyPlan can still recover IDs
// when the user's HCL uses action="create".
func TestAllocationResource_ModifyPlan_PostImportSelectRules(t *testing.T) {
	ctx := context.Background()
	sch := modifyPlanTestSchema(t)
	tv := modifyPlanTestTimeouts(t, sch)

	importedRule := modifyPlanTestRule(t, map[string]attr.Value{
		"id":          types.StringValue("id-imported"),
		"name":        types.StringValue("my-rule"),
		"action":      types.StringValue("select"),
		"formula":     types.StringValue("A"),
		"description": types.StringNull(),
		"components":  types.ListNull(resource_allocation.ComponentsValue{}.Type(ctx)),
	})
	stateModel := modifyPlanTestModel(t, []attr.Value{importedRule}, tv)

	planRule := modifyPlanTestRule(t, map[string]attr.Value{
		"id":          types.StringNull(),
		"name":        types.StringValue("my-rule"),
		"action":      types.StringValue("create"),
		"formula":     types.StringValue("A"),
		"description": types.StringNull(),
		"components":  types.ListNull(resource_allocation.ComponentsValue{}.Type(ctx)),
	})
	planModel := modifyPlanTestModel(t, []attr.Value{planRule}, tv)

	result := runModifyPlan(t, sch, stateModel, planModel)

	if got := result.rules[0].Id.ValueString(); got != "id-imported" {
		t.Errorf("expected post-import rule to recover id-imported, got %q", got)
	}
}

// TestAllocationResource_ModifyPlan_UnknownComponentsInUpdate verifies that ModifyPlan correctly
// handles an update where a plan rule has unknown nested elements (e.g. a component value derived
// from a computed resource attribute). The rule should still be matched by name (Pass 1).
func TestAllocationResource_ModifyPlan_UnknownComponentsInUpdate(t *testing.T) {
	ctx := context.Background()
	sch := modifyPlanTestSchema(t)
	tv := modifyPlanTestTimeouts(t, sch)

	stateRule := modifyPlanTestRule(t, map[string]attr.Value{
		"id":          types.StringValue("id-known"),
		"name":        types.StringValue("my-rule"),
		"action":      types.StringValue("create"),
		"formula":     types.StringValue("A"),
		"description": types.StringNull(),
		"components":  types.ListNull(resource_allocation.ComponentsValue{}.Type(ctx)),
	})
	stateModel := modifyPlanTestModel(t, []attr.Value{stateRule}, tv)

	planRule := modifyPlanTestRule(t, map[string]attr.Value{
		"id":          types.StringNull(),
		"name":        types.StringValue("my-rule"),
		"action":      types.StringValue("create"),
		"formula":     types.StringValue("A"),
		"description": types.StringNull(),
		"components":  types.ListUnknown(resource_allocation.ComponentsValue{}.Type(ctx)),
	})
	planModel := modifyPlanTestModel(t, []attr.Value{planRule}, tv)

	result := runModifyPlan(t, sch, stateModel, planModel)

	if got := result.rules[0].Id.ValueString(); got != "id-known" {
		t.Errorf("expected rule with unknown components to be matched by name, got id %q", got)
	}
}

// TestAllocationResource_ModifyPlan_UnknownComponentsRename verifies that when a rule is renamed
// and components are unknown, tiers 1 and 2 skip (can't compare unknowns). Tier 3 (formula) still
// matches if the formula is unique.
func TestAllocationResource_ModifyPlan_UnknownComponentsRename(t *testing.T) {
	ctx := context.Background()
	sch := modifyPlanTestSchema(t)
	tv := modifyPlanTestTimeouts(t, sch)

	comp := modifyPlanTestComponentList(t, modifyPlanTestComponent(t, "country", []string{"JP"}))

	stateRule := modifyPlanTestRule(t, map[string]attr.Value{
		"id":          types.StringValue("id-a"),
		"name":        types.StringValue("rule-a"),
		"action":      types.StringValue("create"),
		"formula":     types.StringValue("unique-formula"),
		"description": types.StringNull(),
		"components":  comp,
	})
	stateModel := modifyPlanTestModel(t, []attr.Value{stateRule}, tv)

	planRule := modifyPlanTestRule(t, map[string]attr.Value{
		"id":          types.StringNull(),
		"name":        types.StringValue("rule-a-renamed"),
		"action":      types.StringValue("update"),
		"formula":     types.StringValue("unique-formula"),
		"description": types.StringNull(),
		"components":  types.ListUnknown(resource_allocation.ComponentsValue{}.Type(ctx)),
	})
	planModel := modifyPlanTestModel(t, []attr.Value{planRule}, tv)

	result := runModifyPlan(t, sch, stateModel, planModel)

	if got := result.rules[0].Id.ValueString(); got != "id-a" {
		t.Errorf("expected Tier 3 (formula match) to assign id-a despite unknown components, got %q", got)
	}
}

// TestAllocationResource_ModifyPlan_TwoPlanRulesClaimSameCandidate verifies that when two plan
// rules both uniquely match the same state candidate, neither gets the ID. Both claims must be
// dropped to avoid nondeterministic assignment.
func TestAllocationResource_ModifyPlan_TwoPlanRulesClaimSameCandidate(t *testing.T) {
	sch := modifyPlanTestSchema(t)
	tv := modifyPlanTestTimeouts(t, sch)

	comp := modifyPlanTestComponentList(t, modifyPlanTestComponent(t, "country", []string{"JP"}))

	// State: one rule.
	stateRule := modifyPlanTestRule(t, map[string]attr.Value{
		"id":          types.StringValue("id-a"),
		"name":        types.StringValue("rule-a"),
		"action":      types.StringValue("create"),
		"formula":     types.StringValue("A"),
		"description": types.StringNull(),
		"components":  comp,
	})
	stateModel := modifyPlanTestModel(t, []attr.Value{stateRule}, tv)

	// Plan: two NEW rules with different names but identical formula+components.
	// Both match the same state candidate at Tier 1 — neither should get the ID.
	planRule1 := modifyPlanTestRule(t, map[string]attr.Value{
		"id":          types.StringNull(),
		"name":        types.StringValue("rule-x"),
		"action":      types.StringValue("create"),
		"formula":     types.StringValue("A"),
		"description": types.StringNull(),
		"components":  comp,
	})
	planRule2 := modifyPlanTestRule(t, map[string]attr.Value{
		"id":          types.StringNull(),
		"name":        types.StringValue("rule-y"),
		"action":      types.StringValue("create"),
		"formula":     types.StringValue("A"),
		"description": types.StringNull(),
		"components":  comp,
	})
	planModel := modifyPlanTestModel(t, []attr.Value{planRule1, planRule2}, tv)

	result := runModifyPlan(t, sch, stateModel, planModel)

	for i, r := range result.rules {
		if !r.Id.IsNull() {
			t.Errorf("rule[%d] (%s): expected no ID when two plan rules claim the same candidate, got %q",
				i, r.Name.ValueString(), r.Id.ValueString())
		}
	}
}
