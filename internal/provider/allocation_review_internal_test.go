package provider

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_allocation"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/oapi-codegen/nullable"
)

func reviewRule(t *testing.T, id, description string) resource_allocation.RulesValue {
	t.Helper()
	ctx := t.Context()
	startDate := "2026-01-01"
	if id == "B" {
		startDate = "2026-03-01"
	}
	period := resource_allocation.NewValidityPeriodsValueMust(
		resource_allocation.ValidityPeriodsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"start_date": types.StringValue(startDate),
			"end_date":   types.StringValue("2026-02-01"),
		},
	)
	source := resource_allocation.NewSourcesValueMust(
		resource_allocation.SourcesValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"key": types.StringValue("env"), "type": types.StringValue("label"),
			"providers": types.ListNull(types.StringType),
		},
	)
	extraction := resource_allocation.NewValueExtractionValueMust(
		resource_allocation.ValueExtractionValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"fallback": types.StringValue(description), "on_missing": types.StringValue("useFallback"),
			"sources": types.ListValueMust(resource_allocation.SourcesValue{}.Type(ctx), []attr.Value{source}),
		},
	)
	return modifyPlanTestRule(t, map[string]attr.Value{
		"action": types.StringValue("update"), "id": types.StringValue(id),
		"name": types.StringValue(id), "description": types.StringValue(description),
		"formula":          types.StringValue(description),
		"components":       modifyPlanTestComponentList(t, modifyPlanTestComponent(t, id, []string{description})),
		"validity_periods": types.ListValueMust(resource_allocation.ValidityPeriodsValue{}.Type(ctx), []attr.Value{period}),
		"value_extraction": extraction,
	})
}

func reviewGroup(ids ...string) *models.Allocation {
	rules := make([]nullable.Nullable[models.GroupAllocationRule], len(ids))
	for i, id := range ids {
		rules[i] = valueToNullable(models.GroupAllocationRule{Id: new(id)})
	}
	groupType := models.AllocationAllocationTypeGroup
	return &models.Allocation{Id: new("group-1"), AllocationType: &groupType, Rules: &rules}
}

func TestMapAllocationToModel_RecoveryMatchesRuleIDs(t *testing.T) {
	ctx := t.Context()
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	client := newTestAPIClient(t, server)

	for _, tc := range []struct {
		name string
		ids  []string
	}{
		{name: "reordered", ids: []string{"B", "A"}},
		{name: "replaced", ids: []string{"C", "B"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := &allocationResourceModel{Rules: types.ListValueMust(resource_allocation.RulesValue{}.Type(ctx),
				[]attr.Value{reviewRule(t, "A", "description-A"), reviewRule(t, "B", "description-B")})}
			if diags := mapAllocationToModel(ctx, client, reviewGroup(tc.ids...), state); !diags.HasError() {
				t.Fatal("expected failed child detail diagnostic")
			}
			var rules []resource_allocation.RulesValue
			if diags := state.Rules.ElementsAs(ctx, &rules, false); diags.HasError() {
				t.Fatal(diags)
			}
			for _, rule := range rules {
				id := rule.Id.ValueString()
				if id == "C" {
					if !rule.Description.IsNull() || !rule.ValueExtraction.IsNull() || len(rule.ValidityPeriods.Elements()) != 0 || len(rule.Components.Elements()) != 0 {
						t.Errorf("replacement C inherited prior metadata: %+v", rule)
					}
					continue
				}
				want := "description-" + id
				if got := rule.Description.ValueString(); got != want {
					t.Errorf("%s description = %q, want %q", id, got, want)
				}
				if got := rule.ValueExtraction.Fallback.ValueString(); got != want {
					t.Errorf("%s extraction fallback = %q, want %q", id, got, want)
				}
				if got := rule.Formula.ValueString(); got != want {
					t.Errorf("%s formula = %q, want %q", id, got, want)
				}
				if len(rule.ValidityPeriods.Elements()) != 1 || len(rule.Components.Elements()) != 1 {
					t.Errorf("%s lost periods or components", id)
					continue
				}
				var periods []resource_allocation.ValidityPeriodsValue
				var components []resource_allocation.ComponentsValue
				if d := rule.ValidityPeriods.ElementsAs(ctx, &periods, false); d.HasError() {
					t.Fatal(d)
				}
				if d := rule.Components.ElementsAs(ctx, &components, false); d.HasError() {
					t.Fatal(d)
				}
				wantDate := "2026-01-01"
				if id == "B" {
					wantDate = "2026-03-01"
				}
				if periods[0].StartDate.ValueString() != wantDate || components[0].Key.ValueString() != id {
					t.Errorf("%s inherited another rule's periods or components", id)
				}
			}
		})
	}
}

func TestMapAllocationToModel_SuccessfulDetailReadShowsClearedFields(t *testing.T) {
	ctx := t.Context()
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"A","name":"renamed-outside-terraform","description":"changed-outside-terraform","rule":{"formula":"A","components":[]}}`))
	}))
	client := newTestAPIClient(t, server)
	state := &allocationResourceModel{Rules: types.ListValueMust(resource_allocation.RulesValue{}.Type(ctx),
		[]attr.Value{reviewRule(t, "A", "prior")})}
	if diags := mapAllocationToModel(ctx, client, reviewGroup("A"), state); diags.HasError() {
		t.Fatal(diags)
	}
	var rules []resource_allocation.RulesValue
	if diags := state.Rules.ElementsAs(ctx, &rules, false); diags.HasError() {
		t.Fatal(diags)
	}
	if len(rules[0].ValidityPeriods.Elements()) != 0 || !rules[0].ValueExtraction.IsNull() || len(rules[0].Components.Elements()) != 0 {
		t.Errorf("successful detail read retained cleared fields: %+v", rules[0])
	}
	if rules[0].Name.ValueString() != "renamed-outside-terraform" || rules[0].Description.ValueString() != "changed-outside-terraform" {
		t.Errorf("successful detail read hid external name or description change: %+v", rules[0])
	}
}

func TestAllocationComponents_ReorderKeepsSentinelsWithKeys(t *testing.T) {
	ctx := t.Context()
	prior := make([]resource_allocation.ComponentsValue, 2)
	for i, spec := range []struct{ key, sentinel string }{
		{"service_description", "[Service N/A]"},
		{"country", "[Country N/A]"},
	} {
		prior[i] = resource_allocation.NewComponentsValueMust(
			resource_allocation.ComponentsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"key": types.StringValue(spec.key), "mode": types.StringValue("is"),
				"type":         types.StringValue("fixed"),
				"values":       types.ListValueMust(types.StringType, []attr.Value{types.StringValue(spec.sentinel)}),
				"include_null": types.BoolValue(true), "inverse": types.BoolValue(false),
				"case_insensitive": types.BoolValue(false),
			},
		)
	}
	components := []models.AllocationComponent{
		{Key: "country", Mode: "is", Type: "fixed", IncludeNull: new(true), Values: []string{}},
		{Key: "service_description", Mode: "is", Type: "fixed", IncludeNull: new(true), Values: []string{}},
	}
	got, diags := toAllocationRuleComponentsListValue(ctx, components, prior)
	if diags.HasError() {
		t.Fatal(diags)
	}
	var values []resource_allocation.ComponentsValue
	if d := got.ElementsAs(ctx, &values, false); d.HasError() {
		t.Fatal(d)
	}
	for i, want := range []string{"[Country N/A]", "[Service N/A]"} {
		var strings []string
		if d := values[i].Values.ElementsAs(ctx, &strings, false); d.HasError() {
			t.Fatal(d)
		}
		if len(strings) != 1 || strings[0] != want {
			t.Errorf("component %q values = %v, want [%q]", values[i].Key.ValueString(), strings, want)
		}
	}
}

func TestAllocationComponents_SuccessfulReadShowsClearedValues(t *testing.T) {
	ctx := t.Context()
	prior := []resource_allocation.ComponentsValue{modifyPlanTestComponent(t, "country", []string{"JP"})}
	components := []models.AllocationComponent{{Key: "country", Mode: "is", Type: "fixed", Values: []string{}}}
	got, diags := toAllocationRuleComponentsListValue(ctx, components, prior)
	if diags.HasError() {
		t.Fatal(diags)
	}
	var values []resource_allocation.ComponentsValue
	if d := got.ElementsAs(ctx, &values, false); d.HasError() {
		t.Fatal(d)
	}
	if len(values[0].Values.Elements()) != 0 {
		t.Errorf("successful read retained externally cleared component values: %v", values[0].Values)
	}
}

func TestAllocationUpdate_RemovedMemberCleanedUpAfterChildReadFailure(t *testing.T) {
	ctx := t.Context()
	var deleted []string
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodPatch:
			_, _ = w.Write([]byte(`{"id":"group-123","name":"group-alloc","description":"desc","allocationType":"group","type":"custom","unallocatedCosts":"other","createTime":100,"updateTime":300,"folderId":"root","rules":[{"id":"keep-1","name":"k1"}]}`))
		case http.MethodGet:
			w.WriteHeader(http.StatusInternalServerError)
		case http.MethodDelete:
			deleted = append(deleted, r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:])
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	r := &allocationResource{client: newTestAPIClient(t, server)}
	sch := modifyPlanTestSchema(t)
	tv := modifyPlanTestTimeouts(t, sch)
	state := allocationRawForTest(t, sch, modifyPlanTestModel(t, []attr.Value{
		inlineRuleForTest(t, "create", "keep-1", "k1"), inlineRuleForTest(t, "create", "removed-1", "removed"),
	}, tv))
	planModel := modifyPlanTestModel(t, []attr.Value{inlineRuleForTest(t, "create", "keep-1", "k1")}, tv)
	planModel.UnallocatedCosts = types.StringValue("other")
	planState := allocationRawForTest(t, sch, planModel)
	resp := &resource.UpdateResponse{State: state}
	r.Update(ctx, resource.UpdateRequest{State: state, Plan: tfsdk.Plan{Schema: sch, Raw: planState.Raw}}, resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected child detail error")
	}
	if len(deleted) != 1 || deleted[0] != "removed-1" {
		t.Errorf("deleted = %v, want [removed-1]", deleted)
	}
	var got allocationResourceModel
	if diags := resp.State.Get(ctx, &got); diags.HasError() {
		t.Fatal(diags)
	}
	if got.Id.ValueString() != "group-123" {
		t.Errorf("group ID = %q", got.Id.ValueString())
	}
	var rules []resource_allocation.RulesValue
	if diags := got.Rules.ElementsAs(ctx, &rules, false); diags.HasError() || len(rules) != 1 || rules[0].Id.ValueString() != "keep-1" {
		t.Errorf("recoverable rules = %v (%v)", rules, diags)
	}
}

func TestRemovedInlineRuleIDsFromResponse_RequiresMemberIDs(t *testing.T) {
	prior := []resource_allocation.RulesValue{reviewRule(t, "owned-1", "prior")}
	rules := []nullable.Nullable[models.GroupAllocationRule]{
		valueToNullable(models.GroupAllocationRule{Name: new("member-without-id")}),
	}
	removed, confirmed := removedInlineRuleIDsFromResponse(prior, &models.Allocation{Rules: &rules})
	if confirmed || len(removed) != 0 {
		t.Errorf("incomplete membership must not authorize deletion: confirmed=%t, removed=%v", confirmed, removed)
	}
}
