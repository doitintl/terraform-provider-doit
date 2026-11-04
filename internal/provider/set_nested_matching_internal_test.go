package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Elements that differ only in an attribute one of them leaves unknown or
// omits can be matched in more than one way. The matching must find a
// one-to-one assignment whenever one exists, whatever the element order.

func TestPlanSetNestedAttributesFromState_OverlappingElements(t *testing.T) {
	priorX := tfItem("a", map[string]tftypes.Value{
		"note": tfString("x"), "status": tfString("ok"), "tags": tfEmptyTags, "settings": tfSettings(true),
	})
	priorY := tfItem("a", map[string]tftypes.Value{
		"note": tfString("y"), "status": tfString("ok"), "tags": tfEmptyTags, "settings": tfSettings(true),
	})
	// One element omits note (Category B: matches any prior note), the other
	// configures note = "x". The only valid assignment is omitted → y, x → x.
	omitted := tfItem("a", nil)
	configured := tfItem("a", map[string]tftypes.Value{"note": tfString("x")})
	plannedOmitted := tfPlannedItem("a", nil)
	plannedConfigured := tfPlannedItem("a", map[string]tftypes.Value{"note": tfString("x")})

	orders := []struct {
		name   string
		config []tftypes.Value
		plan   []tftypes.Value
		prior  []tftypes.Value
	}{
		{"config omitted first", []tftypes.Value{omitted, configured}, []tftypes.Value{plannedOmitted, plannedConfigured}, []tftypes.Value{priorX, priorY}},
		{"plan configured first", []tftypes.Value{omitted, configured}, []tftypes.Value{plannedConfigured, plannedOmitted}, []tftypes.Value{priorX, priorY}},
		{"prior y first", []tftypes.Value{omitted, configured}, []tftypes.Value{plannedOmitted, plannedConfigured}, []tftypes.Value{priorY, priorX}},
		{"config configured first", []tftypes.Value{configured, omitted}, []tftypes.Value{plannedConfigured, plannedOmitted}, []tftypes.Value{priorX, priorY}},
	}
	for _, o := range orders {
		t.Run(o.name, func(t *testing.T) {
			state := tfRoot(tfString("id-1"), o.prior...)
			got := runSetNestedPlan(t, tfRoot(tfNullString, o.config...), tfRoot(tfUnknownString, o.plan...), state)
			assertTF(t, "plan", got, state)
		})
	}
}

func TestKnownValuesMatch_OverlappingSetElements(t *testing.T) {
	setOf := func(items ...tftypes.Value) tftypes.Value {
		return tftypes.NewValue(tftypes.Set{ElementType: setNestedTestItemType}, items)
	}
	x := tfItem("a", map[string]tftypes.Value{"note": tfString("x")})
	y := tfItem("a", map[string]tftypes.Value{"note": tfString("y")})
	pattern := setOf(
		tfItem("a", map[string]tftypes.Value{"note": tfUnknownString}),
		tfItem("a", map[string]tftypes.Value{"note": tfString("x")}),
	)
	if !knownValuesMatch(pattern, setOf(x, y)) {
		t.Error("pattern should match: unknown → y, x → x")
	}
}

func TestOverlaySetElements_OverlappingElements(t *testing.T) {
	ctx := t.Context()
	elemType := types.ObjectType{AttrTypes: map[string]attr.Type{
		"name": types.StringType, "note": types.StringType, "status": types.StringType,
	}}
	obj := func(name, note, status attr.Value) attr.Value {
		return types.ObjectValueMust(elemType.AttrTypes, map[string]attr.Value{"name": name, "note": note, "status": status})
	}
	set := func(elems ...attr.Value) types.Set {
		return types.SetValueMust(elemType, elems)
	}
	a := types.StringValue("a")

	plan := set(
		obj(a, types.StringUnknown(), types.StringUnknown()),
		obj(a, types.StringValue("x"), types.StringUnknown()),
	)
	resolved := set(
		obj(a, types.StringValue("x"), types.StringValue("from x")),
		obj(a, types.StringValue("y"), types.StringValue("from y")),
	)

	overlay := func(_ context.Context, resolved, plan *types.Object) diag.Diagnostics {
		r, p := resolved.Attributes(), plan.Attributes()
		for _, name := range []string{"note", "status"} {
			if p[name].IsUnknown() {
				p[name] = r[name]
			}
		}
		v, diags := types.ObjectValue(elemType.AttrTypes, p)
		*plan = v
		return diags
	}
	if diags := overlaySetElements(ctx, &resolved, &plan, overlay); diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	v, err := plan.ToTerraformValue(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !v.IsFullyKnown() {
		t.Errorf("plan = %v, want every element resolved", plan)
	}
}

// An omitted Category A attribute inside a nested list or set of a changed set
// element gets its cleared value, like one directly in the element.
func TestPlanSetNestedAttributesFromState_ClearsNestedCollections(t *testing.T) {
	childAttrs := map[string]schema.Attribute{
		"key": schema.StringAttribute{Required: true},
		"tags": schema.ListAttribute{
			ElementType:   types.StringType,
			Optional:      true,
			Computed:      true,
			PlanModifiers: []planmodifier.List{useNullForUnknownListWhenConfigNull()},
		},
	}
	s := schema.Schema{Attributes: map[string]schema.Attribute{
		"items": schema.SetNestedAttribute{
			Required: true,
			NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
				"name":   schema.StringAttribute{Required: true},
				"rules":  schema.ListNestedAttribute{Optional: true, NestedObject: schema.NestedAttributeObject{Attributes: childAttrs}},
				"labels": schema.SetNestedAttribute{Optional: true, NestedObject: schema.NestedAttributeObject{Attributes: childAttrs}},
			}},
		},
	}}

	childType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"key": tftypes.String, "tags": tftypes.List{ElementType: tftypes.String}}}
	itemType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"name":   tftypes.String,
		"rules":  tftypes.List{ElementType: childType},
		"labels": tftypes.Set{ElementType: childType},
	}}
	rootType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"items": tftypes.Set{ElementType: itemType}}}
	child := func(tags tftypes.Value) tftypes.Value {
		return tftypes.NewValue(childType, map[string]tftypes.Value{"key": tfString("k"), "tags": tags})
	}
	root := func(name string, tags tftypes.Value) tftypes.Value {
		item := tftypes.NewValue(itemType, map[string]tftypes.Value{
			"name":   tfString(name),
			"rules":  tftypes.NewValue(tftypes.List{ElementType: childType}, []tftypes.Value{child(tags)}),
			"labels": tftypes.NewValue(tftypes.Set{ElementType: childType}, []tftypes.Value{child(tags)}),
		})
		return tftypes.NewValue(rootType, map[string]tftypes.Value{"items": tftypes.NewValue(tftypes.Set{ElementType: itemType}, []tftypes.Value{item})})
	}

	// The element's name changed, so it matches no prior element.
	config := root("b", tfNullTags)
	plan := root("b", tfUnknownTags)
	state := root("a", tfTags("old"))

	req := resource.ModifyPlanRequest{
		Config: tfsdk.Config{Schema: s, Raw: config},
		Plan:   tfsdk.Plan{Schema: s, Raw: plan},
		State:  tfsdk.State{Schema: s, Raw: state},
	}
	resp := &resource.ModifyPlanResponse{Plan: req.Plan}
	planSetNestedAttributesFromState(t.Context(), req, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}
	assertTF(t, "plan", resp.Plan.Raw, root("b", tfEmptyTags))
}
