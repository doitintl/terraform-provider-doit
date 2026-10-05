package provider

import (
	"context"
	"maps"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/oapi-codegen/nullable"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_allocation"
)

func ownershipRule(action string, id types.String) resource_allocation.RulesValue {
	ctx := context.Background()
	attrTypes := resource_allocation.RulesValue{}.AttributeTypes(ctx)
	attrs := make(map[string]attr.Value, len(attrTypes))
	for name, typ := range attrTypes {
		attrs[name] = tftypesNull(ctx, typ)
	}
	attrs["action"] = types.StringValue(action)
	attrs["id"] = id
	return resource_allocation.NewRulesValueMust(attrTypes, attrs)
}

func tftypesNull(ctx context.Context, typ attr.Type) attr.Value {
	v, err := typ.ValueFromTerraform(ctx, tftypes.NewValue(typ.TerraformType(ctx), nil))
	if err != nil {
		panic(err)
	}
	return v
}

func TestCreatedRuleIDs(t *testing.T) {
	tests := []struct {
		name  string
		rules []resource_allocation.RulesValue
		want  []string
	}{
		{"no rules", nil, nil},
		{"create with id is owned", []resource_allocation.RulesValue{ownershipRule("create", types.StringValue("a"))}, []string{"a"}},
		{"select is never owned", []resource_allocation.RulesValue{ownershipRule("select", types.StringValue("a"))}, nil},
		{"update is an inline definition too", []resource_allocation.RulesValue{ownershipRule("update", types.StringValue("a"))}, []string{"a"}},
		{"create without id is skipped", []resource_allocation.RulesValue{
			ownershipRule("create", types.StringNull()),
			ownershipRule("create", types.StringUnknown()),
			ownershipRule("create", types.StringValue("")),
		}, nil},
		{"duplicates collapse, order kept", []resource_allocation.RulesValue{
			ownershipRule("create", types.StringValue("b")),
			ownershipRule("select", types.StringValue("x")),
			ownershipRule("create", types.StringValue("a")),
			ownershipRule("create", types.StringValue("b")),
		}, []string{"b", "a"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := inlineRuleIDs(tt.rules); !slices.Equal(got, tt.want) {
				t.Errorf("inlineRuleIDs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRemovedInlineRuleIDs(t *testing.T) {
	create := func(id string) resource_allocation.RulesValue { return ownershipRule("create", types.StringValue(id)) }
	update := func(id string) resource_allocation.RulesValue { return ownershipRule("update", types.StringValue(id)) }
	selectRule := func(id string) resource_allocation.RulesValue { return ownershipRule("select", types.StringValue(id)) }

	tests := []struct {
		name           string
		prior, planned []resource_allocation.RulesValue
		want           []string
	}{
		{"nothing changed", []resource_allocation.RulesValue{create("a")}, []resource_allocation.RulesValue{create("a")}, nil},
		{"dropped create rule", []resource_allocation.RulesValue{create("a"), create("b")}, []resource_allocation.RulesValue{create("a")}, []string{"b"}},
		{"all rules dropped", []resource_allocation.RulesValue{create("a")}, nil, []string{"a"}},
		{"dropped select rule is not deleted", []resource_allocation.RulesValue{selectRule("a"), create("b")}, []resource_allocation.RulesValue{create("b")}, nil},
		{"create flipped to select keeps allocation", []resource_allocation.RulesValue{create("a")}, []resource_allocation.RulesValue{selectRule("a")}, nil},
		{"new rule with unknown id does not protect others", []resource_allocation.RulesValue{create("a")}, []resource_allocation.RulesValue{ownershipRule("create", types.StringUnknown())}, []string{"a"}},
		{"create switched to update keeps allocation", []resource_allocation.RulesValue{create("a")}, []resource_allocation.RulesValue{update("a")}, nil},
		{"dropped update rule is deleted", []resource_allocation.RulesValue{update("a"), create("b")}, []resource_allocation.RulesValue{create("b")}, []string{"a"}},
		{"replaced by a different id", []resource_allocation.RulesValue{create("a")}, []resource_allocation.RulesValue{create("c")}, []string{"a"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := removedInlineRuleIDs(tt.prior, tt.planned); !slices.Equal(got, tt.want) {
				t.Errorf("removedInlineRuleIDs() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ownershipStateRule builds a prior-state/plan rule for mapAllocationToModel tests.
func ownershipStateRule(t *testing.T, action string, id types.String) attr.Value {
	t.Helper()
	return ownershipRuleWith(t, action, id, map[string]attr.Value{
		"name":    types.StringValue("n"),
		"formula": types.StringValue("A"),
	})
}

func ownershipRuleWith(t *testing.T, action string, id types.String, extra map[string]attr.Value) attr.Value {
	t.Helper()
	ctx := t.Context()
	attrTypes := resource_allocation.RulesValue{}.AttributeTypes(ctx)
	attrs := make(map[string]attr.Value, len(attrTypes))
	for name, typ := range attrTypes {
		attrs[name] = tftypesNull(ctx, typ)
	}
	attrs["action"] = types.StringValue(action)
	attrs["id"] = id
	maps.Copy(attrs, extra)
	return resource_allocation.NewRulesValueMust(attrTypes, attrs)
}

func ownershipGroupResponse(ids ...string) *models.Allocation {
	groupType := models.AllocationAllocationType("group")
	rules := make([]nullable.Nullable[models.GroupAllocationRule], 0, len(ids))
	for _, id := range ids {
		formula := "A"
		components := []models.AllocationComponent{{Key: "country", Mode: "is", Type: "fixed", Values: []string{"JP"}}}
		rules = append(rules, valueToNullable(models.GroupAllocationRule{
			Id: &id, Name: &id, Formula: &formula, Components: &components,
		}))
	}
	return &models.Allocation{AllocationType: &groupType, Rules: &rules}
}

func ownershipMappedActions(t *testing.T, prior []attr.Value, apiResp *models.Allocation) []string {
	t.Helper()
	ctx := t.Context()
	state := &allocationResourceModel{}
	state.Id = types.StringValue("group")
	state.Rule = resource_allocation.NewRuleValueNull()
	state.Rules = types.ListValueMust(resource_allocation.RulesValue{}.Type(ctx), prior)
	if diags := mapAllocationToModel(ctx, nil, apiResp, state); diags.HasError() {
		t.Fatalf("mapAllocationToModel: %v", diags)
	}
	rules, diags := rulesFromList(ctx, state.Rules)
	if diags.HasError() {
		t.Fatalf("rulesFromList: %v", diags)
	}
	actions := make([]string, len(rules))
	for i, r := range rules {
		actions[i] = r.Action.ValueString()
	}
	return actions
}

// A rule that was replaced outside Terraform shows up with an id the prior state does not know.
// It must not inherit the prior rule's "create" action by position: the provider would then treat
// someone else's allocation as its own and delete it.
func TestMapAllocationToModel_ReplacedRuleDoesNotInheritOwnershipByIndex(t *testing.T) {
	prior := []attr.Value{
		ownershipStateRule(t, "create", types.StringValue("mine")),
		ownershipStateRule(t, "select", types.StringValue("shared")),
	}
	got := ownershipMappedActions(t, prior, ownershipGroupResponse("someone-elses", "shared"))
	if want := []string{"select", "select"}; !slices.Equal(got, want) {
		t.Errorf("actions = %v, want %v", got, want)
	}
}

// Right after Create/Update the plan's new rules have unknown ids, so matching them to the API
// response by position is the only way to keep their configured action.
func TestMapAllocationToModel_NewRuleWithUnknownIDKeepsActionByIndex(t *testing.T) {
	prior := []attr.Value{
		ownershipStateRule(t, "select", types.StringValue("shared")),
		ownershipStateRule(t, "create", types.StringUnknown()),
	}
	got := ownershipMappedActions(t, prior, ownershipGroupResponse("shared", "new-id"))
	if want := []string{"select", "create"}; !slices.Equal(got, want) {
		t.Errorf("actions = %v, want %v", got, want)
	}
}
