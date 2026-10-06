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

func TestMapAllocationToModel_ChildAllocationMalformedValueExtraction_ReturnsDiags(t *testing.T) {
	ctx := t.Context()

	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"child-rule-1","name":"child-rule-name","rule":{"formula":"A","components":[],"valueExtraction":{"sources":"invalid-not-an-array"}}}`))
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
		t.Fatalf("expected error diagnostic when child allocation valueExtraction is malformed, got none")
	}
}

func TestMapAllocationToModel_SelectRuleDoesNotFetchChildDetails(t *testing.T) {
	ctx := t.Context()

	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected HTTP request to fetch child allocation for action=select: %s", r.URL.Path)
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
			}),
		},
	}

	state := &allocationResourceModel{}
	existingRule, d := resource_allocation.NewRulesValue(
		resource_allocation.RulesValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"action":           types.StringValue("select"),
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
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	var mappedRules []resource_allocation.RulesValue
	diags = state.Rules.ElementsAs(ctx, &mappedRules, false)
	if diags.HasError() || len(mappedRules) != 1 {
		t.Fatalf("failed to decode mapped rules: %v", diags)
	}
	if len(mappedRules[0].ValidityPeriods.Elements()) != 0 {
		t.Errorf("expected validity_periods to be empty list for action=select, got: %v", mappedRules[0].ValidityPeriods)
	}
	if !mappedRules[0].ValueExtraction.IsNull() {
		t.Errorf("expected value_extraction to be null for action=select, got: %v", mappedRules[0].ValueExtraction)
	}
}

func TestMapAllocationToModel_ImportDefaultsToSelectAndDoesNotFetchChildDetails(t *testing.T) {
	ctx := t.Context()

	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected HTTP request to fetch child allocation on import: %s", r.URL.Path)
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
			}),
		},
	}

	// On import, state.Rules is null.
	state := &allocationResourceModel{
		Rules: types.ListNull(resource_allocation.RulesValue{}.Type(ctx)),
	}

	diags := mapAllocationToModel(ctx, client, alloc, state)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	var mappedRules []resource_allocation.RulesValue
	diags = state.Rules.ElementsAs(ctx, &mappedRules, false)
	if diags.HasError() || len(mappedRules) != 1 {
		t.Fatalf("failed to decode mapped rules: %v", diags)
	}
	if action := mappedRules[0].Action.ValueString(); action != "select" {
		t.Errorf("expected imported rule action to default to select, got: %q", action)
	}
	if len(mappedRules[0].ValidityPeriods.Elements()) != 0 {
		t.Errorf("expected validity_periods to be empty list on import, got: %v", mappedRules[0].ValidityPeriods)
	}
	if !mappedRules[0].ValueExtraction.IsNull() {
		t.Errorf("expected value_extraction to be null on import, got: %v", mappedRules[0].ValueExtraction)
	}
}

func TestConvertValidityPeriodsToModels_NullAndUnknown(t *testing.T) {
	ctx := t.Context()

	nullList := types.ListNull(resource_allocation.ValidityPeriodsValue{}.Type(ctx))
	res, diags := convertValidityPeriodsToModels(ctx, nullList)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if res.IsSpecified() {
		t.Errorf("expected unspecified result for null list, got: %v", res)
	}

	unknownList := types.ListUnknown(resource_allocation.ValidityPeriodsValue{}.Type(ctx))
	res, diags = convertValidityPeriodsToModels(ctx, unknownList)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if res.IsSpecified() {
		t.Errorf("expected unspecified result for unknown list, got: %v", res)
	}
}

func TestConvertValidityPeriodsToModels_Empty(t *testing.T) {
	ctx := t.Context()

	emptyList, d := types.ListValueFrom(ctx, resource_allocation.ValidityPeriodsValue{}.Type(ctx), []resource_allocation.ValidityPeriodsValue{})
	if d.HasError() {
		t.Fatalf("failed to create list: %v", d)
	}
	res, diags := convertValidityPeriodsToModels(ctx, emptyList)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if res.IsNull() {
		t.Fatal("expected non-null result for empty list")
	}
	val, err := res.Get()
	if err != nil || len(val) != 0 {
		t.Errorf("expected empty slice, got: %v (err: %v)", val, err)
	}
}

func TestConvertValidityPeriodsToModels_ValidAndSkippedElements(t *testing.T) {
	ctx := t.Context()

	vp1, d := resource_allocation.NewValidityPeriodsValue(
		resource_allocation.ValidityPeriodsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"start_date": types.StringValue("2026-01-01T00:00:00Z"),
			"end_date":   types.StringValue("2026-06-01T00:00:00Z"),
		},
	)
	if d.HasError() {
		t.Fatalf("NewValidityPeriodsValue: %v", d)
	}

	vp2Null := resource_allocation.NewValidityPeriodsValueNull()
	vp3Unknown := resource_allocation.NewValidityPeriodsValueUnknown()

	vp4, d := resource_allocation.NewValidityPeriodsValue(
		resource_allocation.ValidityPeriodsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"start_date": types.StringNull(),
			"end_date":   types.StringValue("2026-12-31T23:59:59Z"),
		},
	)
	if d.HasError() {
		t.Fatalf("NewValidityPeriodsValue: %v", d)
	}

	listVal, d := types.ListValue(
		resource_allocation.ValidityPeriodsValue{}.Type(ctx),
		[]attr.Value{vp1, vp2Null, vp3Unknown, vp4},
	)
	if d.HasError() {
		t.Fatalf("ListValue: %v", d)
	}

	res, diags := convertValidityPeriodsToModels(ctx, listVal)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	val, err := res.Get()
	if err != nil {
		t.Fatalf("expected non-null result, got err: %v", err)
	}
	if len(val) != 2 {
		t.Fatalf("expected 2 valid periods (null and unknown elements skipped), got: %d", len(val))
	}
	start0 := nullableToPointer(val[0].StartDate)
	end0 := nullableToPointer(val[0].EndDate)
	if start0 == nil || *start0 != "2026-01-01T00:00:00Z" || end0 == nil || *end0 != "2026-06-01T00:00:00Z" {
		t.Errorf("unexpected period 0: %+v", val[0])
	}
	start1 := nullableToPointer(val[1].StartDate)
	end1 := nullableToPointer(val[1].EndDate)
	if start1 != nil || end1 == nil || *end1 != "2026-12-31T23:59:59Z" {
		t.Errorf("unexpected period 1: %+v", val[1])
	}
}

func TestToValidityPeriodsListValue(t *testing.T) {
	ctx := t.Context()

	// Case 1: Null models returns empty list (non-null)
	res, diags := toValidityPeriodsListValue(ctx, nullable.Nullable[[]models.AllocationRulePeriod]{})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if res.IsNull() || len(res.Elements()) != 0 {
		t.Errorf("expected empty non-null list for null input, got: %v", res)
	}

	// Case 2: Populated models
	start := "2026-01-01T00:00:00Z"
	end := "2026-06-01T00:00:00Z"
	input := valueToNullable([]models.AllocationRulePeriod{
		{
			StartDate: valueToNullable(start),
			EndDate:   valueToNullable(end),
		},
		{
			StartDate: nullable.Nullable[string]{},
			EndDate:   valueToNullable(end),
		},
	})
	res, diags = toValidityPeriodsListValue(ctx, input)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	var elements []resource_allocation.ValidityPeriodsValue
	diags = res.ElementsAs(ctx, &elements, false)
	if diags.HasError() {
		t.Fatalf("failed to decode elements: %v", diags)
	}
	if len(elements) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(elements))
	}
	if elements[0].StartDate.ValueString() != start || elements[0].EndDate.ValueString() != end {
		t.Errorf("unexpected element 0: %+v", elements[0])
	}
	if !elements[1].StartDate.IsNull() || elements[1].EndDate.ValueString() != end {
		t.Errorf("unexpected element 1: %+v", elements[1])
	}
}

func TestConvertValueExtractionToModel(t *testing.T) {
	ctx := t.Context()

	// Case 1: Null ValueExtraction returns empty nullable
	res, diags := convertValueExtractionToModel(ctx, resource_allocation.NewValueExtractionValueNull())
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if res.IsSpecified() {
		t.Errorf("expected unspecified result for null ValueExtraction, got: %v", res)
	}

	// Case 2: useFallback with explicit fallback string
	src, d := resource_allocation.NewSourcesValue(
		resource_allocation.SourcesValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"key":       types.StringValue("env"),
			"type":      types.StringValue("label"),
			"providers": types.ListValueMust(types.StringType, []attr.Value{types.StringValue("amazon-web-services")}),
		},
	)
	if d.HasError() {
		t.Fatalf("NewSourcesValue: %v", d)
	}
	sourcesList, d := types.ListValue(resource_allocation.SourcesValue{}.Type(ctx), []attr.Value{src})
	if d.HasError() {
		t.Fatalf("ListValue: %v", d)
	}

	veFallback, d := resource_allocation.NewValueExtractionValue(
		resource_allocation.ValueExtractionValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"fallback":   types.StringValue("prod"),
			"on_missing": types.StringValue("useFallback"),
			"sources":    sourcesList,
		},
	)
	if d.HasError() {
		t.Fatalf("NewValueExtractionValue: %v", d)
	}

	res, diags = convertValueExtractionToModel(ctx, veFallback)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	model, err := res.Get()
	if err != nil {
		t.Fatalf("expected non-null model: %v", err)
	}
	if model.OnMissing == nil || *model.OnMissing != models.AllocationValueExtractionOnMissingUseFallback {
		t.Errorf("expected on_missing useFallback, got: %v", model.OnMissing)
	}
	fb := nullableToPointer(model.Fallback)
	if fb == nil || *fb != "prod" {
		t.Errorf("expected 'prod' fallback in model, got: %v", fb)
	}
	if len(model.Sources) != 1 || model.Sources[0].Key != "env" {
		t.Errorf("unexpected sources: %+v", model.Sources)
	}
	provs := nullableToPointer(model.Sources[0].Providers)
	if provs == nil || len(*provs) != 1 || (*provs)[0] != "amazon-web-services" {
		t.Errorf("unexpected providers: %v", provs)
	}

	// Case 3: Omitted on_missing defaults to useFallback
	veOmittedOnMissing, d := resource_allocation.NewValueExtractionValue(
		resource_allocation.ValueExtractionValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"fallback":   types.StringValue("default_env"),
			"on_missing": types.StringNull(),
			"sources":    sourcesList,
		},
	)
	if d.HasError() {
		t.Fatalf("NewValueExtractionValue: %v", d)
	}
	res, diags = convertValueExtractionToModel(ctx, veOmittedOnMissing)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	model, err = res.Get()
	if err != nil {
		t.Fatalf("expected non-null model: %v", err)
	}
	if model.OnMissing == nil || *model.OnMissing != models.AllocationValueExtractionOnMissingUseFallback {
		t.Errorf("expected defaulted on_missing useFallback, got: %v", model.OnMissing)
	}
	fb = nullableToPointer(model.Fallback)
	if fb == nil || *fb != "default_env" {
		t.Errorf("expected default_env fallback, got: %v", fb)
	}

	// Case 4: nextRule omits fallback in model even if present in TF value
	veNextRule, d := resource_allocation.NewValueExtractionValue(
		resource_allocation.ValueExtractionValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"fallback":   types.StringValue("ignored"),
			"on_missing": types.StringValue("nextRule"),
			"sources":    sourcesList,
		},
	)
	if d.HasError() {
		t.Fatalf("NewValueExtractionValue: %v", d)
	}
	res, diags = convertValueExtractionToModel(ctx, veNextRule)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	model, err = res.Get()
	if err != nil {
		t.Fatalf("expected non-null model: %v", err)
	}
	if model.OnMissing == nil || *model.OnMissing != models.AllocationValueExtractionOnMissingNextRule {
		t.Errorf("expected nextRule, got: %v", model.OnMissing)
	}
	if nullableToPointer(model.Fallback) != nil {
		t.Errorf("expected fallback to be nil when nextRule, got: %v", nullableToPointer(model.Fallback))
	}

	// Case 5: sources with null element returns diagnostic error
	sourcesWithNull, d := types.ListValue(
		resource_allocation.SourcesValue{}.Type(ctx),
		[]attr.Value{src, resource_allocation.NewSourcesValueNull()},
	)
	if d.HasError() {
		t.Fatalf("ListValue: %v", d)
	}
	veWithNullSource, d := resource_allocation.NewValueExtractionValue(
		resource_allocation.ValueExtractionValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"fallback":   types.StringValue("prod"),
			"on_missing": types.StringValue("useFallback"),
			"sources":    sourcesWithNull,
		},
	)
	if d.HasError() {
		t.Fatalf("NewValueExtractionValue: %v", d)
	}
	_, diags = convertValueExtractionToModel(ctx, veWithNullSource)
	if !diags.HasError() {
		t.Fatalf("expected error for null source element, got none")
	}
	expectedMsg := "sources[1]: null element is not permitted"
	foundMsg := false
	for _, errDiag := range diags.Errors() {
		if errDiag.Detail() == expectedMsg {
			foundMsg = true
			break
		}
	}
	if !foundMsg {
		t.Errorf("expected diagnostic error %q, got: %v", expectedMsg, diags)
	}

	// Case 6: sources with unknown element skips the unknown element
	sourcesWithUnknown, d := types.ListValue(
		resource_allocation.SourcesValue{}.Type(ctx),
		[]attr.Value{src, resource_allocation.NewSourcesValueUnknown()},
	)
	if d.HasError() {
		t.Fatalf("ListValue: %v", d)
	}
	veWithUnknownSource, d := resource_allocation.NewValueExtractionValue(
		resource_allocation.ValueExtractionValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"fallback":   types.StringValue("prod"),
			"on_missing": types.StringValue("useFallback"),
			"sources":    sourcesWithUnknown,
		},
	)
	if d.HasError() {
		t.Fatalf("NewValueExtractionValue: %v", d)
	}
	res, diags = convertValueExtractionToModel(ctx, veWithUnknownSource)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics for unknown source element: %v", diags)
	}
	model, err = res.Get()
	if err != nil {
		t.Fatalf("expected non-null model: %v", err)
	}
	if len(model.Sources) != 1 || model.Sources[0].Key != "env" {
		t.Errorf("expected 1 source with unknown element skipped, got: %+v", model.Sources)
	}
}

func TestToValueExtractionValue(t *testing.T) {
	ctx := t.Context()

	// Case 1: Null input returns null ValueExtractionValue
	res, diags := toValueExtractionValue(ctx, nullable.Nullable[models.AllocationValueExtraction]{})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if !res.IsNull() {
		t.Errorf("expected null ValueExtractionValue, got: %v", res)
	}

	// Case 2: Model with fallback and nil onMissing (defaults to useFallback)
	modelInput := valueToNullable(models.AllocationValueExtraction{
		Fallback:  valueToNullable("prod"),
		OnMissing: nil,
		Sources: []models.AllocationValueExtractionSource{
			{
				Key:       "tag_key",
				Type:      models.AllocationValueExtractionSourceTypeTag,
				Providers: valueToNullable([]string{"google-cloud-platform"}),
			},
		},
	})
	res, diags = toValueExtractionValue(ctx, modelInput)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if res.IsNull() {
		t.Fatal("expected non-null ValueExtractionValue")
	}
	if res.Fallback.ValueString() != "prod" || res.Fallback.IsNull() {
		t.Errorf("expected fallback to be \"prod\", got: %v", res.Fallback)
	}
	if res.OnMissing.ValueString() != "useFallback" {
		t.Errorf("expected on_missing to default to useFallback, got: %v", res.OnMissing)
	}
	var sources []resource_allocation.SourcesValue
	diags = res.Sources.ElementsAs(ctx, &sources, false)
	if diags.HasError() {
		t.Fatalf("ElementsAs: %v", diags)
	}
	if len(sources) != 1 || sources[0].Key.ValueString() != "tag_key" {
		t.Errorf("unexpected sources: %v", sources)
	}
}
