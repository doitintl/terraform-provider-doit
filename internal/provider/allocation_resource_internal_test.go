package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
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

func runModifyPlan(t *testing.T, sch schema.Schema, stateModel, planModel allocationResourceModel) []resource_allocation.RulesValue {
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

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected modify plan diags: %v", resp.Diagnostics)
	}

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
	return resultRules
}

// TestAllocationResource_ModifyPlan_DeleteAndRenameRuleWithoutId verifies that when Rule A (index 0)
// is deleted and Rule B (index 1) is renamed, Pass 2 matches by formula and assigns Rule B's ID.
func TestAllocationResource_ModifyPlan_DeleteAndRenameRuleWithoutId(t *testing.T) {
	ctx := context.Background()
	sch := modifyPlanTestSchema(t)
	tv := modifyPlanTestTimeouts(t, sch)

	ruleA := modifyPlanTestRule(t, map[string]attr.Value{
		"id":          types.StringValue("id-a"),
		"name":        types.StringValue("rule-a"),
		"action":      types.StringValue("create"),
		"formula":     types.StringValue("A"),
		"description": types.StringNull(),
		"components":  types.ListNull(resource_allocation.ComponentsValue{}.Type(ctx)),
	})
	ruleB := modifyPlanTestRule(t, map[string]attr.Value{
		"id":          types.StringValue("id-b"),
		"name":        types.StringValue("rule-b"),
		"action":      types.StringValue("create"),
		"formula":     types.StringValue("B"),
		"description": types.StringNull(),
		"components":  types.ListNull(resource_allocation.ComponentsValue{}.Type(ctx)),
	})
	stateModel := modifyPlanTestModel(t, []attr.Value{ruleA, ruleB}, tv)

	ruleBRenamed := modifyPlanTestRule(t, map[string]attr.Value{
		"id":          types.StringNull(),
		"name":        types.StringValue("rule-b-renamed"),
		"action":      types.StringValue("update"),
		"formula":     types.StringValue("B"),
		"description": types.StringNull(),
		"components":  types.ListNull(resource_allocation.ComponentsValue{}.Type(ctx)),
	})
	planModel := modifyPlanTestModel(t, []attr.Value{ruleBRenamed}, tv)

	resultRules := runModifyPlan(t, sch, stateModel, planModel)

	if got := resultRules[0].Id.ValueString(); got != "id-b" {
		t.Errorf("expected rule-b-renamed to get id-b (formula-based match), got %q", got)
	}
}

// TestAllocationResource_ModifyPlan_RenameAndFormulaChange verifies the documented gap:
// when a rule is simultaneously renamed AND has its formula changed, neither pass can match it.
// The rule should be left without an ID (treated as new). The user must provide an explicit id
// in HCL to force an in-place update in this case.
func TestAllocationResource_ModifyPlan_RenameAndFormulaChange(t *testing.T) {
	ctx := context.Background()
	sch := modifyPlanTestSchema(t)
	tv := modifyPlanTestTimeouts(t, sch)

	ruleA := modifyPlanTestRule(t, map[string]attr.Value{
		"id":          types.StringValue("id-a"),
		"name":        types.StringValue("rule-a"),
		"action":      types.StringValue("create"),
		"formula":     types.StringValue("A"),
		"description": types.StringNull(),
		"components":  types.ListNull(resource_allocation.ComponentsValue{}.Type(ctx)),
	})
	stateModel := modifyPlanTestModel(t, []attr.Value{ruleA}, tv)

	ruleRenamed := modifyPlanTestRule(t, map[string]attr.Value{
		"id":          types.StringNull(),
		"name":        types.StringValue("rule-a-renamed"),
		"action":      types.StringValue("update"),
		"formula":     types.StringValue("A-new"),
		"description": types.StringNull(),
		"components":  types.ListNull(resource_allocation.ComponentsValue{}.Type(ctx)),
	})
	planModel := modifyPlanTestModel(t, []attr.Value{ruleRenamed}, tv)

	resultRules := runModifyPlan(t, sch, stateModel, planModel)

	if !resultRules[0].Id.IsNull() {
		t.Errorf("expected no ID match when both name and formula change, got %q", resultRules[0].Id.ValueString())
	}
}
