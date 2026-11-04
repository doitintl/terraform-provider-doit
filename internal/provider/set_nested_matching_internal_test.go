package provider

import (
	"context"
	"strings"
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

// Pairing a configured element with its planned element must respect unknown
// and omitted values: elements that share their Required values must not swap,
// or an unknown configured value is planned as the prior value, or a
// configured list loses its known elements.
func TestPlanSetNestedAttributesFromState_PairsByUnknownAndOmittedValues(t *testing.T) {
	prior := tfItem("a", map[string]tftypes.Value{
		"note": tfString("n"), "status": tfString("ok"), "tags": tfEmptyTags, "settings": tfSettings(true),
	})
	state := tfRoot(tfString("id-1"), prior)

	t.Run("unknown configured value", func(t *testing.T) {
		config := tfRoot(tfNullString, tfItem("a", map[string]tftypes.Value{"opt": tfUnknownString}), tfItem("a", nil))
		plan := tfRoot(tfUnknownString, tfPlannedItem("a", map[string]tftypes.Value{"opt": tfUnknownString}), tfPlannedItem("a", nil))
		got := runSetNestedPlan(t, config, plan, state)

		var root map[string]tftypes.Value
		var items []tftypes.Value
		if err := got.As(&root); err != nil {
			t.Fatal(err)
		}
		if err := root["items"].As(&items); err != nil {
			t.Fatal(err)
		}
		unknownOpt := 0
		for _, item := range items {
			var v map[string]tftypes.Value
			if err := item.As(&v); err != nil {
				t.Fatal(err)
			}
			if !v["opt"].IsKnown() {
				unknownOpt++
			}
		}
		if unknownOpt != 1 {
			t.Errorf("elements with unknown opt = %d, want 1 (the configured unknown): %s", unknownOpt, got)
		}
	})

	t.Run("partially unknown configured list", func(t *testing.T) {
		partial := tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{tfString("x"), tfUnknownString})
		config := tfRoot(tfNullString, tfItem("a", map[string]tftypes.Value{"tags": partial}), tfItem("a", nil))
		plan := tfRoot(tfUnknownString, tfPlannedItem("a", map[string]tftypes.Value{"tags": partial}), tfPlannedItem("a", nil))
		got := runSetNestedPlan(t, config, plan, state)

		var root map[string]tftypes.Value
		var items []tftypes.Value
		if err := got.As(&root); err != nil {
			t.Fatal(err)
		}
		if err := root["items"].As(&items); err != nil {
			t.Fatal(err)
		}
		kept := 0
		for _, item := range items {
			var v map[string]tftypes.Value
			if err := item.As(&v); err != nil {
				t.Fatal(err)
			}
			if v["tags"].Equal(partial) {
				kept++
			}
		}
		if kept != 1 {
			t.Errorf("elements with the configured tags = %d, want 1: %s", kept, got)
		}
	})
}

// A set nested in a map is planned from the prior state like any other set.
func TestPlanSetNestedAttributesFromState_SetInMap(t *testing.T) {
	s := schema.Schema{Attributes: map[string]schema.Attribute{
		"groups": schema.MapNestedAttribute{
			Optional: true,
			NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
				"items": schema.SetNestedAttribute{
					Optional: true,
					NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
						"name":   schema.StringAttribute{Required: true},
						"status": schema.StringAttribute{Computed: true},
					}},
				},
			}},
		},
	}}
	itemType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"name": tftypes.String, "status": tftypes.String}}
	groupType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"items": tftypes.Set{ElementType: itemType}}}
	rootType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"groups": tftypes.Map{ElementType: groupType}}}
	root := func(status tftypes.Value) tftypes.Value {
		item := tftypes.NewValue(itemType, map[string]tftypes.Value{"name": tfString("a"), "status": status})
		group := tftypes.NewValue(groupType, map[string]tftypes.Value{"items": tftypes.NewValue(tftypes.Set{ElementType: itemType}, []tftypes.Value{item})})
		return tftypes.NewValue(rootType, map[string]tftypes.Value{"groups": tftypes.NewValue(tftypes.Map{ElementType: groupType}, map[string]tftypes.Value{"g": group})})
	}

	state := root(tfString("ok"))
	got := runSchemaSetNestedPlan(t, s, root(tfNullString), root(tfUnknownString), state)
	assertTF(t, "plan", got, state)
}

// testClearInt64Modifier is a clearing modifier on an Int64 attribute.
type testClearInt64Modifier struct{}

func (testClearInt64Modifier) Description(context.Context) string         { return "clears to 0" }
func (testClearInt64Modifier) MarkdownDescription(context.Context) string { return "clears to 0" }
func (testClearInt64Modifier) PlanModifyInt64(context.Context, planmodifier.Int64Request, *planmodifier.Int64Response) {
}
func (testClearInt64Modifier) clearedValue(t tftypes.Type) tftypes.Value {
	return tftypes.NewValue(t, 0)
}

// Every attribute kind with plan modifiers can carry a clearing modifier.
func TestPlanSetNestedAttributesFromState_ClearerOnAnyAttributeKind(t *testing.T) {
	s := schema.Schema{Attributes: map[string]schema.Attribute{
		"items": schema.SetNestedAttribute{
			Required: true,
			NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
				"name": schema.StringAttribute{Required: true},
				"count": schema.Int64Attribute{
					Optional:      true,
					Computed:      true,
					PlanModifiers: []planmodifier.Int64{testClearInt64Modifier{}},
				},
			}},
		},
	}}
	itemType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"name": tftypes.String, "count": tftypes.Number}}
	rootType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"items": tftypes.Set{ElementType: itemType}}}
	root := func(name string, count tftypes.Value) tftypes.Value {
		item := tftypes.NewValue(itemType, map[string]tftypes.Value{"name": tfString(name), "count": count})
		return tftypes.NewValue(rootType, map[string]tftypes.Value{"items": tftypes.NewValue(tftypes.Set{ElementType: itemType}, []tftypes.Value{item})})
	}
	nullCount := tftypes.NewValue(tftypes.Number, nil)
	unknownCount := tftypes.NewValue(tftypes.Number, tftypes.UnknownValue)

	// The element changed (b instead of a), so its omitted count is cleared.
	got := runSchemaSetNestedPlan(t, s, root("b", nullCount), root("b", unknownCount), root("a", tftypes.NewValue(tftypes.Number, 5)))
	assertTF(t, "plan", got, root("b", tftypes.NewValue(tftypes.Number, 0)))
}

func runSchemaSetNestedPlan(t *testing.T, s schema.Schema, config, plan, state tftypes.Value) tftypes.Value {
	t.Helper()
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
	return resp.Plan.Raw
}

// Two configured elements that differ only in an omitted Category A attribute
// versus its cleared value (tags omitted, tags = []) are the same element once
// the omitted attribute is cleared. The plan must not collapse them.
func TestPlanSetNestedAttributesFromState_ClearingCollapsesElements(t *testing.T) {
	childAttrs := map[string]schema.Attribute{
		"key": schema.StringAttribute{Required: true},
		"tags": schema.ListAttribute{
			ElementType:   types.StringType,
			Optional:      true,
			Computed:      true,
			PlanModifiers: []planmodifier.List{useNullForUnknownListWhenConfigNull()},
		},
	}
	childType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"key": tftypes.String, "tags": tftypes.List{ElementType: tftypes.String}}}
	child := func(tags tftypes.Value) tftypes.Value {
		return tftypes.NewValue(childType, map[string]tftypes.Value{"key": tfString("k"), "tags": tags})
	}
	childSet := func(children ...tftypes.Value) tftypes.Value {
		return tftypes.NewValue(tftypes.Set{ElementType: childType}, children)
	}

	t.Run("top-level set", func(t *testing.T) {
		s := schema.Schema{Attributes: map[string]schema.Attribute{
			"items": schema.SetNestedAttribute{Required: true, NestedObject: schema.NestedAttributeObject{Attributes: childAttrs}},
		}}
		rootType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"items": tftypes.Set{ElementType: childType}}}
		root := func(children ...tftypes.Value) tftypes.Value {
			return tftypes.NewValue(rootType, map[string]tftypes.Value{"items": childSet(children...)})
		}
		config := root(child(tfNullTags), child(tfEmptyTags))
		plan := root(child(tfUnknownTags), child(tfEmptyTags))
		state := root(child(tfTags("old")))

		req := resource.ModifyPlanRequest{
			Config: tfsdk.Config{Schema: s, Raw: config},
			Plan:   tfsdk.Plan{Schema: s, Raw: plan},
			State:  tfsdk.State{Schema: s, Raw: state},
		}
		resp := &resource.ModifyPlanResponse{Plan: req.Plan}
		planSetNestedAttributesFromState(t.Context(), req, resp)
		assertDuplicateSetElements(t, resp.Diagnostics, "items")
	})

	t.Run("set nested in a changed element", func(t *testing.T) {
		s := schema.Schema{Attributes: map[string]schema.Attribute{
			"items": schema.SetNestedAttribute{Required: true, NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
				"name":   schema.StringAttribute{Required: true},
				"labels": schema.SetNestedAttribute{Optional: true, NestedObject: schema.NestedAttributeObject{Attributes: childAttrs}},
			}}},
		}}
		itemType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"name": tftypes.String, "labels": tftypes.Set{ElementType: childType}}}
		rootType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"items": tftypes.Set{ElementType: itemType}}}
		root := func(name string, children ...tftypes.Value) tftypes.Value {
			item := tftypes.NewValue(itemType, map[string]tftypes.Value{"name": tfString(name), "labels": childSet(children...)})
			return tftypes.NewValue(rootType, map[string]tftypes.Value{"items": tftypes.NewValue(tftypes.Set{ElementType: itemType}, []tftypes.Value{item})})
		}
		// The outer element changed (b instead of a), so its labels are cleared.
		config := root("b", child(tfNullTags), child(tfEmptyTags))
		plan := root("b", child(tfUnknownTags), child(tfEmptyTags))
		state := root("a", child(tfTags("old")))

		req := resource.ModifyPlanRequest{
			Config: tfsdk.Config{Schema: s, Raw: config},
			Plan:   tfsdk.Plan{Schema: s, Raw: plan},
			State:  tfsdk.State{Schema: s, Raw: state},
		}
		resp := &resource.ModifyPlanResponse{Plan: req.Plan}
		planSetNestedAttributesFromState(t.Context(), req, resp)
		assertDuplicateSetElements(t, resp.Diagnostics, "items[*].labels")
	})
}

func assertDuplicateSetElements(t *testing.T, diags diag.Diagnostics, path string) {
	t.Helper()
	if diags.ErrorsCount() != 1 {
		t.Fatalf("errors = %d, want 1: %v", diags.ErrorsCount(), diags)
	}
	d := diags.Errors()[0]
	if d.Summary() != "Duplicate Set Elements" {
		t.Errorf("summary = %q", d.Summary())
	}
	if want := "Two elements of " + path + " describe the same element"; !strings.Contains(d.Detail(), want) {
		t.Errorf("detail = %q, want it to contain %q", d.Detail(), want)
	}
}

// Elements whose configured values are unknown may still differ once those
// values are known, so equal planned elements are not a duplicate then: the
// plan keeps them apart. Unknown Computed values do not hide a duplicate.
func TestPlanSetNestedAttributesFromState_CollapseWithUnknownValues(t *testing.T) {
	childAttrs := map[string]schema.Attribute{
		"name": schema.StringAttribute{Required: true},
		"tags": schema.ListAttribute{
			ElementType:   types.StringType,
			Optional:      true,
			Computed:      true,
			PlanModifiers: []planmodifier.List{useNullForUnknownListWhenConfigNull()},
		},
		"status": schema.StringAttribute{Computed: true},
	}
	childType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"name": tftypes.String, "tags": tftypes.List{ElementType: tftypes.String}, "status": tftypes.String,
	}}
	child := func(name, tags, status tftypes.Value) tftypes.Value {
		return tftypes.NewValue(childType, map[string]tftypes.Value{"name": name, "tags": tags, "status": status})
	}
	childSet := func(children ...tftypes.Value) tftypes.Value {
		return tftypes.NewValue(tftypes.Set{ElementType: childType}, children)
	}

	topLevel := schema.Schema{Attributes: map[string]schema.Attribute{
		"items": schema.SetNestedAttribute{Required: true, NestedObject: schema.NestedAttributeObject{Attributes: childAttrs}},
	}}
	topType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"items": tftypes.Set{ElementType: childType}}}
	top := func(children ...tftypes.Value) tftypes.Value {
		return tftypes.NewValue(topType, map[string]tftypes.Value{"items": childSet(children...)})
	}

	nested := schema.Schema{Attributes: map[string]schema.Attribute{
		"items": schema.SetNestedAttribute{Required: true, NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"group":  schema.StringAttribute{Required: true},
			"labels": schema.SetNestedAttribute{Optional: true, NestedObject: schema.NestedAttributeObject{Attributes: childAttrs}},
		}}},
	}}
	itemType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"group": tftypes.String, "labels": tftypes.Set{ElementType: childType}}}
	nestedType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"items": tftypes.Set{ElementType: itemType}}}
	nestedRoot := func(group string, children ...tftypes.Value) tftypes.Value {
		item := tftypes.NewValue(itemType, map[string]tftypes.Value{"group": tfString(group), "labels": childSet(children...)})
		return tftypes.NewValue(nestedType, map[string]tftypes.Value{"items": tftypes.NewValue(tftypes.Set{ElementType: itemType}, []tftypes.Value{item})})
	}

	run := func(t *testing.T, s schema.Schema, config, plan, state tftypes.Value) *resource.ModifyPlanResponse {
		t.Helper()
		req := resource.ModifyPlanRequest{
			Config: tfsdk.Config{Schema: s, Raw: config},
			Plan:   tfsdk.Plan{Schema: s, Raw: plan},
			State:  tfsdk.State{Schema: s, Raw: state},
		}
		resp := &resource.ModifyPlanResponse{Plan: req.Plan}
		planSetNestedAttributesFromState(t.Context(), req, resp)
		return resp
	}

	t.Run("unknown names in a top-level set", func(t *testing.T) {
		config := top(child(tfUnknownString, tfNullTags, tfNullString), child(tfUnknownString, tfEmptyTags, tfNullString))
		plan := top(child(tfUnknownString, tfUnknownTags, tfUnknownString), child(tfUnknownString, tfEmptyTags, tfUnknownString))
		state := top(child(tfString("old"), tfTags("x"), tfString("ok")))
		resp := run(t, topLevel, config, plan, state)
		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
		}
		assertTF(t, "plan", resp.Plan.Raw, plan)
	})

	t.Run("unknown names in a nested set", func(t *testing.T) {
		config := nestedRoot("b", child(tfUnknownString, tfNullTags, tfNullString), child(tfUnknownString, tfEmptyTags, tfNullString))
		plan := nestedRoot("b", child(tfUnknownString, tfUnknownTags, tfUnknownString), child(tfUnknownString, tfEmptyTags, tfUnknownString))
		state := nestedRoot("a", child(tfString("old"), tfTags("x"), tfString("ok")))
		resp := run(t, nested, config, plan, state)
		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
		}
		assertTF(t, "plan", resp.Plan.Raw, plan)
	})

	t.Run("known names with unknown computed values", func(t *testing.T) {
		config := top(child(tfString("k"), tfNullTags, tfNullString), child(tfString("k"), tfEmptyTags, tfNullString))
		plan := top(child(tfString("k"), tfUnknownTags, tfUnknownString), child(tfString("k"), tfEmptyTags, tfUnknownString))
		state := top(child(tfString("old"), tfTags("x"), tfString("ok")))
		assertDuplicateSetElements(t, run(t, topLevel, config, plan, state).Diagnostics, "items")
	})
}
