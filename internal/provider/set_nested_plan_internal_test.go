package provider

import (
	"maps"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// setNestedTestSchema has a set whose elements cover every attribute class:
//
//	name     Required
//	opt      Optional
//	note     Optional+Computed (Category B)
//	tags     Optional+Computed with a clearing modifier (Category A)
//	status   Computed-only
//	settings Optional+Computed object with a Required leaf
func setNestedTestSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true},
			"items": schema.SetNestedAttribute{
				Required: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name":   schema.StringAttribute{Required: true},
						"opt":    schema.StringAttribute{Optional: true},
						"note":   schema.StringAttribute{Optional: true, Computed: true},
						"status": schema.StringAttribute{Computed: true},
						"tags": schema.ListAttribute{
							ElementType:   types.StringType,
							Optional:      true,
							Computed:      true,
							PlanModifiers: []planmodifier.List{useNullForUnknownListWhenConfigNull()},
						},
						"settings": schema.SingleNestedAttribute{
							Optional: true,
							Computed: true,
							Attributes: map[string]schema.Attribute{
								"flag": schema.BoolAttribute{Required: true},
							},
						},
					},
				},
			},
		},
	}
}

var (
	setNestedTestSettingsType = tftypes.Object{AttributeTypes: map[string]tftypes.Type{"flag": tftypes.Bool}}
	setNestedTestItemType     = tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"name":     tftypes.String,
		"opt":      tftypes.String,
		"note":     tftypes.String,
		"status":   tftypes.String,
		"tags":     tftypes.List{ElementType: tftypes.String},
		"settings": setNestedTestSettingsType,
	}}
	setNestedTestRootType = tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"id":    tftypes.String,
		"items": tftypes.Set{ElementType: setNestedTestItemType},
	}}
	tfUnknownString = tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	tfNullString    = tftypes.NewValue(tftypes.String, nil)
	tfUnknownTags   = tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, tftypes.UnknownValue)
	tfNullTags      = tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil)
	tfEmptyTags     = tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{})
	tfUnknownObj    = tftypes.NewValue(setNestedTestSettingsType, tftypes.UnknownValue)
	tfNullObj       = tftypes.NewValue(setNestedTestSettingsType, nil)
)

func tfString(s string) tftypes.Value { return tftypes.NewValue(tftypes.String, s) }

func tfTags(tags ...string) tftypes.Value {
	values := make([]tftypes.Value, len(tags))
	for i, tag := range tags {
		values[i] = tfString(tag)
	}
	return tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, values)
}

func tfSettings(flag bool) tftypes.Value {
	return tftypes.NewValue(setNestedTestSettingsType, map[string]tftypes.Value{"flag": tftypes.NewValue(tftypes.Bool, flag)})
}

// tfItem builds an element; overrides replace the defaults, which are those of
// a configuration that sets only name.
func tfItem(name string, overrides map[string]tftypes.Value) tftypes.Value {
	attrs := map[string]tftypes.Value{
		"name":     tfString(name),
		"opt":      tfNullString,
		"note":     tfNullString,
		"status":   tfNullString,
		"tags":     tfNullTags,
		"settings": tfNullObj,
	}
	maps.Copy(attrs, overrides)
	return tftypes.NewValue(setNestedTestItemType, attrs)
}

// tfPlannedItem is what the framework plans from a configured element: every
// Computed attribute the configuration leaves null is unknown.
func tfPlannedItem(name string, overrides map[string]tftypes.Value) tftypes.Value {
	attrs := map[string]tftypes.Value{
		"note":     tfUnknownString,
		"status":   tfUnknownString,
		"tags":     tfUnknownTags,
		"settings": tfUnknownObj,
	}
	maps.Copy(attrs, overrides)
	return tfItem(name, attrs)
}

func tfRoot(id tftypes.Value, items ...tftypes.Value) tftypes.Value {
	return tftypes.NewValue(setNestedTestRootType, map[string]tftypes.Value{
		"id":    id,
		"items": tftypes.NewValue(tftypes.Set{ElementType: setNestedTestItemType}, items),
	})
}

func runSetNestedPlan(t *testing.T, config, plan, state tftypes.Value) tftypes.Value {
	t.Helper()
	s := setNestedTestSchema()
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

// setNestedItems returns the planned elements keyed by name.
func setNestedItems(t *testing.T, root tftypes.Value) map[string]map[string]tftypes.Value {
	t.Helper()
	var attrs map[string]tftypes.Value
	var items []tftypes.Value
	if err := root.As(&attrs); err != nil {
		t.Fatal(err)
	}
	if err := attrs["items"].As(&items); err != nil {
		t.Fatal(err)
	}
	out := map[string]map[string]tftypes.Value{}
	for _, item := range items {
		var v map[string]tftypes.Value
		if err := item.As(&v); err != nil {
			t.Fatal(err)
		}
		var name string
		if err := v["name"].As(&name); err != nil {
			t.Fatal(err)
		}
		out[name] = v
	}
	return out
}

func assertTF(t *testing.T, what string, got, want tftypes.Value) {
	t.Helper()
	if !got.Equal(want) {
		t.Errorf("%s = %s, want %s", what, got, want)
	}
}

func TestPlanSetNestedAttributesFromState(t *testing.T) {
	priorA := tfItem("a", map[string]tftypes.Value{
		"note": tfString("api note"), "status": tfString("ok"), "tags": tfEmptyTags, "settings": tfSettings(true),
	})
	priorB := tfItem("b", map[string]tftypes.Value{
		"note": tfString("b note"), "status": tfString("ok"), "tags": tfEmptyTags, "settings": tfSettings(true),
	})
	state := tfRoot(tfString("id-1"), priorA, priorB)

	t.Run("unchanged plans the prior state", func(t *testing.T) {
		config := tfRoot(tfNullString, tfItem("a", nil), tfItem("b", nil))
		plan := tfRoot(tfUnknownString, tfPlannedItem("a", nil), tfPlannedItem("b", nil))
		assertTF(t, "plan", runSetNestedPlan(t, config, plan, state), state)
	})

	t.Run("configured values equal to the prior state", func(t *testing.T) {
		config := tfRoot(tfNullString,
			tfItem("a", map[string]tftypes.Value{"note": tfString("api note"), "tags": tfEmptyTags, "settings": tfSettings(true)}),
			tfItem("b", nil))
		plan := tfRoot(tfUnknownString,
			tfPlannedItem("a", map[string]tftypes.Value{"note": tfString("api note"), "tags": tfEmptyTags, "settings": tfSettings(true)}),
			tfPlannedItem("b", nil))
		assertTF(t, "plan", runSetNestedPlan(t, config, plan, state), state)
	})

	t.Run("one changed element", func(t *testing.T) {
		config := tfRoot(tfNullString, tfItem("a", nil), tfItem("b", map[string]tftypes.Value{"opt": tfString("x")}))
		plan := tfRoot(tfUnknownString, tfPlannedItem("a", nil), tfPlannedItem("b", map[string]tftypes.Value{"opt": tfString("x")}))
		got := runSetNestedPlan(t, config, plan, state)

		items := setNestedItems(t, got)
		assertTF(t, "unchanged element a", tftypes.NewValue(setNestedTestItemType, items["a"]), priorA)
		b := items["b"]
		assertTF(t, "b.opt", b["opt"], tfString("x"))
		assertTF(t, "b.status (Computed-only)", b["status"], tfUnknownString)
		assertTF(t, "b.note (Category B)", b["note"], tfUnknownString)
		assertTF(t, "b.tags (Category A, cleared)", b["tags"], tfEmptyTags)
		var root map[string]tftypes.Value
		if err := got.As(&root); err != nil {
			t.Fatal(err)
		}
		assertTF(t, "id", root["id"], tfUnknownString)
	})

	// Omitting a Category A attribute clears it; omitting a Category B one
	// keeps the prior value.
	t.Run("omitted Category A attribute with a prior value", func(t *testing.T) {
		tagged := tfRoot(tfString("id-1"), tfItem("a", map[string]tftypes.Value{
			"note": tfString("api note"), "status": tfString("ok"), "tags": tfTags("x"), "settings": tfSettings(true),
		}))
		config := tfRoot(tfNullString, tfItem("a", nil))
		plan := tfRoot(tfUnknownString, tfPlannedItem("a", nil))
		a := setNestedItems(t, runSetNestedPlan(t, config, plan, tagged))["a"]
		assertTF(t, "a.tags", a["tags"], tfEmptyTags)
	})

	t.Run("omitted plain Optional attribute with a prior value", func(t *testing.T) {
		withOpt := tfRoot(tfString("id-1"), tfItem("a", map[string]tftypes.Value{
			"opt": tfString("old"), "note": tfString("api note"), "status": tfString("ok"), "tags": tfEmptyTags, "settings": tfSettings(true),
		}))
		config := tfRoot(tfNullString, tfItem("a", nil))
		plan := tfRoot(tfUnknownString, tfPlannedItem("a", nil))
		a := setNestedItems(t, runSetNestedPlan(t, config, plan, withOpt))["a"]
		assertTF(t, "a.opt", a["opt"], tfNullString)
		assertTF(t, "a.status", a["status"], tfUnknownString)
	})

	t.Run("new element", func(t *testing.T) {
		config := tfRoot(tfNullString, tfItem("a", nil), tfItem("b", nil), tfItem("c", nil))
		plan := tfRoot(tfUnknownString, tfPlannedItem("a", nil), tfPlannedItem("b", nil), tfPlannedItem("c", nil))
		items := setNestedItems(t, runSetNestedPlan(t, config, plan, state))
		assertTF(t, "a", tftypes.NewValue(setNestedTestItemType, items["a"]), priorA)
		assertTF(t, "b", tftypes.NewValue(setNestedTestItemType, items["b"]), priorB)
		assertTF(t, "c.status", items["c"]["status"], tfUnknownString)
		assertTF(t, "c.tags", items["c"]["tags"], tfEmptyTags)
	})

	t.Run("removed element", func(t *testing.T) {
		config := tfRoot(tfNullString, tfItem("a", nil))
		plan := tfRoot(tfUnknownString, tfPlannedItem("a", nil))
		got := runSetNestedPlan(t, config, plan, state)
		items := setNestedItems(t, got)
		if len(items) != 1 {
			t.Fatalf("items = %d, want 1", len(items))
		}
		assertTF(t, "a", tftypes.NewValue(setNestedTestItemType, items["a"]), priorA)
	})

	// An unknown configured value (a reference to a resource that is not
	// created yet) never matches a prior element.
	t.Run("unknown configured value", func(t *testing.T) {
		config := tfRoot(tfNullString, tfItem("a", map[string]tftypes.Value{"opt": tfUnknownString}), tfItem("b", nil))
		plan := tfRoot(tfUnknownString, tfPlannedItem("a", map[string]tftypes.Value{"opt": tfUnknownString}), tfPlannedItem("b", nil))
		items := setNestedItems(t, runSetNestedPlan(t, config, plan, state))
		assertTF(t, "a.opt", items["a"]["opt"], tfUnknownString)
		assertTF(t, "a.status", items["a"]["status"], tfUnknownString)
		assertTF(t, "b", tftypes.NewValue(setNestedTestItemType, items["b"]), priorB)
	})

	t.Run("create is left alone", func(t *testing.T) {
		config := tfRoot(tfNullString, tfItem("a", nil))
		got := runSetNestedPlan(t, config, tfRoot(tfUnknownString, tfPlannedItem("a", nil)), tftypes.NewValue(setNestedTestRootType, nil))
		assertTF(t, "plan", got, tfRoot(tfUnknownString, tfPlannedItem("a", nil)))
	})
}

func TestKnownValuesMatch(t *testing.T) {
	full := tfItem("a", map[string]tftypes.Value{"note": tfString("n"), "status": tfString("ok"), "tags": tfTags("x", "y"), "settings": tfSettings(true)})
	tests := []struct {
		name    string
		pattern tftypes.Value
		want    bool
	}{
		{"identical", full, true},
		{"unknowns match anything", tfPlannedItem("a", nil), true},
		{"different known value", tfPlannedItem("b", nil), false},
		{"null does not match a value", tfItem("a", map[string]tftypes.Value{"status": tfUnknownString, "tags": tfUnknownTags, "settings": tfUnknownObj}), false},
		{"list order matters", tfPlannedItem("a", map[string]tftypes.Value{"tags": tfTags("y", "x")}), false},
		{"nested unknown", tfPlannedItem("a", map[string]tftypes.Value{"settings": tftypes.NewValue(setNestedTestSettingsType, map[string]tftypes.Value{"flag": tftypes.NewValue(tftypes.Bool, tftypes.UnknownValue)})}), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := knownValuesMatch(tt.pattern, full); got != tt.want {
				t.Errorf("knownValuesMatch = %v, want %v", got, tt.want)
			}
		})
	}

	setOf := func(items ...tftypes.Value) tftypes.Value {
		return tftypes.NewValue(tftypes.Set{ElementType: setNestedTestItemType}, items)
	}
	if !knownValuesMatch(setOf(tfPlannedItem("b", nil), tfPlannedItem("a", nil)), setOf(full, tfItem("b", nil))) {
		t.Error("set elements should match regardless of order")
	}
	if knownValuesMatch(setOf(tfPlannedItem("a", nil), tfPlannedItem("a", nil)), setOf(full, tfItem("b", nil))) {
		t.Error("each set element may match only once")
	}
}
