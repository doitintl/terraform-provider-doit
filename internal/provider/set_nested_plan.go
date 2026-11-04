package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// planSetNestedAttributesFromState plans the elements of every
// SetNestedAttribute in the schema from the prior state.
//
// Set elements have no identity Terraform can track. For set blocks, Terraform
// Core matches elements to the prior state by their configured values, but it
// does not do so for nested attributes, and the framework cannot either: it
// marks every Computed (and every omitted Optional+Computed) attribute of every
// element unknown, so the set shows a replaced element on every plan. Plan
// modifiers on attributes inside a set element cannot fix this: the framework
// passes them the configuration and prior state of the element at the same
// index, which is generally a different element. UseStateForUnknown there can
// copy another element's value, so it must not be used on set element
// attributes; the clearing modifiers are safe because the value they plan does
// not depend on the prior value.
//
// This function does the matching Core does for set blocks: each planned
// element whose configuration equals a prior element is planned as that prior
// element. Computed-only attributes and omitted Optional+Computed attributes
// (Category B) match anything; an omitted attribute with a clearing modifier
// (Category A) matches only its cleared value, and is planned as the cleared
// value in an element that changed. When the whole configuration equals the
// prior state and the plan agrees with it on every known value, the plan
// becomes the prior state, so it is empty. A known value that differs was
// planned on purpose by another plan modifier, so that plan is kept.
//
// Call it first in ModifyPlan of every resource whose schema has a
// SetNestedAttribute. It only changes Computed values, so it never contradicts
// the configuration.
func planSetNestedAttributesFromState(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || resp.Plan.Raw.IsNull() {
		return // create or destroy — nothing to match
	}
	s, ok := req.Plan.Schema.(schema.Schema)
	if !ok {
		return
	}

	p := setNestedPlanner{ctx: ctx}
	planned, err := p.object(s.Attributes, req.Config.Raw, resp.Plan.Raw, req.State.Raw, "")
	if err == nil {
		var unchanged bool
		unchanged, err = p.unchanged(s.Attributes, req.Config.Raw, planned, req.State.Raw)
		// Only unknowns may differ: a known value that differs from the prior
		// state was planned on purpose (by another plan modifier) and is kept.
		if unchanged && knownValuesMatch(planned, req.State.Raw) {
			planned = req.State.Raw
		}
	}
	if collapse, ok := errors.AsType[setElementsCollapseError](err); ok {
		resp.Diagnostics.AddError(
			"Duplicate Set Elements",
			fmt.Sprintf("Two elements of %s describe the same element: one omits an attribute that the other sets to the value "+
				"omitting it means (its cleared or current value). A set cannot hold the same element twice; remove one of them.", collapse.path),
		)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Planning Set Attributes",
			fmt.Sprintf("Could not match planned set elements to the prior state. Please report this issue to the provider developers: %s", err),
		)
		return
	}
	resp.Plan.Raw = planned
}

// configNullClearer is implemented by the clearing plan modifiers. clearedValue
// is the value the modifier plans when the configuration is null.
type configNullClearer interface {
	clearedValue(t tftypes.Type) tftypes.Value
}

type setNestedPlanner struct {
	ctx context.Context
}

// object plans the children of an object value.
func (p setNestedPlanner) object(attrs map[string]schema.Attribute, config, plan, prior tftypes.Value, at string) (tftypes.Value, error) {
	if !isKnownNonNull(config) || !isKnownNonNull(plan) || !isKnownNonNull(prior) {
		return plan, nil
	}
	var c, pl, s map[string]tftypes.Value
	if err := asAll(config, &c, plan, &pl, prior, &s); err != nil {
		return plan, err
	}

	changed := false
	for name, a := range attrs {
		v, err := p.attribute(a, c[name], pl[name], s[name], attributePath(at, name))
		if err != nil {
			return plan, err
		}
		if !v.Equal(pl[name]) {
			pl[name] = v
			changed = true
		}
	}
	if !changed {
		return plan, nil
	}
	return tftypes.NewValue(plan.Type(), pl), nil
}

// attribute plans the nested attributes of one attribute value.
func (p setNestedPlanner) attribute(a schema.Attribute, config, plan, prior tftypes.Value, at string) (tftypes.Value, error) {
	switch a := a.(type) {
	case schema.SetNestedAttribute:
		return p.set(a.NestedObject.Attributes, config, plan, prior, at)
	case schema.SingleNestedAttribute:
		return p.object(a.Attributes, config, plan, prior, at)
	case schema.ListNestedAttribute:
		return p.list(a.NestedObject.Attributes, config, plan, prior, at)
	case schema.MapNestedAttribute:
		return p.mapElements(a.NestedObject.Attributes, config, plan, prior, at)
	}
	return plan, nil
}

// mapElements plans map elements key by key: Terraform matches map elements to
// the prior state by key already, but a set inside an element needs this.
func (p setNestedPlanner) mapElements(attrs map[string]schema.Attribute, config, plan, prior tftypes.Value, at string) (tftypes.Value, error) {
	if !isKnownNonNull(config) || !isKnownNonNull(plan) || !isKnownNonNull(prior) {
		return plan, nil
	}
	var c, pl, s map[string]tftypes.Value
	if err := asAll(config, &c, plan, &pl, prior, &s); err != nil {
		return plan, err
	}

	changed := false
	for key, pv := range pl {
		cv, inConfig := c[key]
		sv, inPrior := s[key]
		if !inConfig || !inPrior {
			continue
		}
		v, err := p.object(attrs, cv, pv, sv, at+"[*]")
		if err != nil {
			return plan, err
		}
		if !v.Equal(pv) {
			pl[key] = v
			changed = true
		}
	}
	if !changed {
		return plan, nil
	}
	return tftypes.NewValue(plan.Type(), pl), nil
}

// list plans list elements index by index: Terraform matches list elements to
// the prior state by position already, but a set inside an element needs this.
func (p setNestedPlanner) list(attrs map[string]schema.Attribute, config, plan, prior tftypes.Value, at string) (tftypes.Value, error) {
	if !isKnownNonNull(config) || !isKnownNonNull(plan) || !isKnownNonNull(prior) {
		return plan, nil
	}
	var c, pl, s []tftypes.Value
	if err := asAll(config, &c, plan, &pl, prior, &s); err != nil {
		return plan, err
	}
	if len(c) != len(pl) {
		return plan, nil
	}

	changed := false
	for i := range pl {
		if i >= len(s) {
			break
		}
		v, err := p.object(attrs, c[i], pl[i], s[i], at+"[*]")
		if err != nil {
			return plan, err
		}
		if !v.Equal(pl[i]) {
			pl[i] = v
			changed = true
		}
	}
	if !changed {
		return plan, nil
	}
	return tftypes.NewValue(plan.Type(), pl), nil
}

// set plans set elements: each element whose configuration equals a prior
// element becomes that element; the others get their cleared values.
func (p setNestedPlanner) set(attrs map[string]schema.Attribute, config, plan, prior tftypes.Value, at string) (tftypes.Value, error) {
	if !isKnownNonNull(config) || !isKnownNonNull(plan) {
		return plan, nil
	}
	var c, pl, s []tftypes.Value
	if err := asAll(config, &c, plan, &pl); err != nil {
		return plan, err
	}
	if isKnownNonNull(prior) {
		if err := prior.As(&s); err != nil {
			return plan, err
		}
	}

	pairs, err := p.pair(attrs, c, pl)
	if err != nil || pairs == nil {
		return plan, err // no element correspondence: leave the plan alone
	}

	matches, err := matchOneToOne(len(c), len(s), func(ci, si int) (bool, error) {
		return p.unchanged(attrs, c[ci], pl[pairs[ci]], s[si])
	})
	if err != nil {
		return plan, err
	}

	out := make([]tftypes.Value, len(pl))
	configs := make([]tftypes.Value, len(pl))
	for ci, pi := range pairs {
		configs[pi] = c[ci]
		if si := matches[ci]; si >= 0 {
			out[pi] = s[si]
			continue
		}
		v, err := p.cleared(attrs, c[ci], pl[pi], at+"[*]")
		if err != nil {
			return plan, err
		}
		out[pi] = v
	}

	if distinct, err := distinctSetElements(out, configs, at); err != nil || !distinct {
		return plan, err
	}
	return tftypes.NewValue(plan.Type(), out), nil
}

// setElementsCollapseError reports configured set elements that become the
// same element once their omitted attributes take their prior or cleared
// values: one element omits an attribute that another sets to that value.
// Both describe the same remote element, and a set cannot hold it twice.
type setElementsCollapseError struct {
	path string
}

func (e setElementsCollapseError) Error() string {
	return fmt.Sprintf("elements of %s are the same element once their omitted attributes are resolved", e.path)
}

// distinctSetElements reports whether the planned elements of the set at path
// are distinct. configs holds the configured element of each planned element.
//
// Equal elements whose configurations are fully known describe one remote
// element twice: that is a setElementsCollapseError. When an equal element's
// configuration has unknown values (a reference to a resource that is not
// created yet), the elements may still differ once those values are known.
// That is no error, but the planned set cannot keep them apart, so distinct is
// false and the caller keeps the framework's plan for the set.
func distinctSetElements(elements, configs []tftypes.Value, path string) (bool, error) {
	for i := range elements {
		for j := i + 1; j < len(elements); j++ {
			if !elements[i].Equal(elements[j]) {
				continue
			}
			if configs[i].IsFullyKnown() && configs[j].IsFullyKnown() {
				return false, setElementsCollapseError{path: path}
			}
			return false, nil
		}
	}
	return true, nil
}

// attributePath joins an attribute name to its parent's path, using the [*]
// element notation of acknowledgeNotClearable.
func attributePath(parent, name string) string {
	if parent == "" {
		return name
	}
	return parent + "." + name
}

// pair returns, for every configured element, the index of the planned
// element derived from it, or nil when the elements do not correspond.
func (p setNestedPlanner) pair(attrs map[string]schema.Attribute, config, plan []tftypes.Value) ([]int, error) {
	if len(config) != len(plan) {
		return nil, nil
	}
	pairs, err := matchOneToOne(len(config), len(plan), func(ci, pi int) (bool, error) {
		return p.derived(attrs, config[ci], plan[pi])
	})
	if err != nil || !allMatched(pairs) {
		return nil, err
	}
	return pairs, nil
}

// matchOneToOne assigns each left index a distinct right index for which ok
// holds, with as many assigned pairs as possible. Elements that differ only in
// values one of them leaves unknown or omits can match in more than one way,
// so a first-fit choice could take the only counterpart of a later element;
// augmenting paths reassign earlier choices instead. matches[l] is the right
// index, or -1 when l has no counterpart.
func matchOneToOne(left, right int, ok func(l, r int) (bool, error)) ([]int, error) {
	candidates := make([][]int, left)
	for l := range left {
		for r := range right {
			match, err := ok(l, r)
			if err != nil {
				return nil, err
			}
			if match {
				candidates[l] = append(candidates[l], r)
			}
		}
	}

	owner := make([]int, right) // left index assigned to each right index
	for r := range owner {
		owner[r] = -1
	}
	var assign func(l int, visited []bool) bool
	assign = func(l int, visited []bool) bool {
		for _, r := range candidates[l] {
			if visited[r] {
				continue
			}
			visited[r] = true
			if owner[r] < 0 || assign(owner[r], visited) {
				owner[r] = l
				return true
			}
		}
		return false
	}
	for l := range left {
		assign(l, make([]bool, right))
	}

	matches := make([]int, left)
	for l := range matches {
		matches[l] = -1
	}
	for r, l := range owner {
		if l >= 0 {
			matches[l] = r
		}
	}
	return matches, nil
}

// allMatched reports whether every left index of matchOneToOne has a counterpart.
func allMatched(matches []int) bool {
	for _, m := range matches {
		if m < 0 {
			return false
		}
	}
	return true
}

// derived reports whether a planned object can have been planned from the
// configured object: every known configured value is in the plan.
func (p setNestedPlanner) derived(attrs map[string]schema.Attribute, config, plan tftypes.Value) (bool, error) {
	if !config.IsKnown() {
		return !plan.IsKnown(), nil
	}
	if config.IsNull() || plan.IsNull() || !plan.IsKnown() {
		return config.IsNull() == plan.IsNull() && plan.IsKnown(), nil
	}
	var c, pl map[string]tftypes.Value
	if err := asAll(config, &c, plan, &pl); err != nil {
		return false, err
	}
	for name, a := range attrs {
		cv, pv := c[name], pl[name]
		if !cv.IsKnown() {
			// A configured unknown is planned unknown.
			if pv.IsKnown() {
				return false, nil
			}
			continue
		}
		if cv.IsNull() {
			// Omitted: a Computed attribute may be filled in by a default or a
			// plan modifier, or be marked unknown; any other attribute is null.
			if !a.IsComputed() && !pv.IsNull() {
				return false, nil
			}
			continue
		}
		var ok bool
		var err error
		switch a := a.(type) {
		case schema.SingleNestedAttribute:
			ok, err = p.derived(a.Attributes, cv, pv)
		case schema.SetNestedAttribute:
			ok, err = p.derivedElements(a.NestedObject.Attributes, cv, pv, true)
		case schema.ListNestedAttribute:
			ok, err = p.derivedElements(a.NestedObject.Attributes, cv, pv, false)
		case schema.MapNestedAttribute:
			ok, err = p.derivedMap(a.NestedObject.Attributes, cv, pv)
		default:
			// Configured values are planned as configured, unknown parts included.
			ok = pv.Equal(cv)
		}
		if err != nil || !ok {
			return false, err
		}
	}
	return true, nil
}

// derivedElements is derived for the elements of a nested list or set.
func (p setNestedPlanner) derivedElements(attrs map[string]schema.Attribute, config, plan tftypes.Value, isSet bool) (bool, error) {
	if !plan.IsKnown() || plan.IsNull() {
		return false, nil
	}
	var c, pl []tftypes.Value
	if err := asAll(config, &c, plan, &pl); err != nil {
		return false, err
	}
	if len(c) != len(pl) {
		return false, nil
	}
	if isSet {
		pairs, err := p.pair(attrs, c, pl)
		return pairs != nil, err
	}
	for i := range c {
		ok, err := p.derived(attrs, c[i], pl[i])
		if err != nil || !ok {
			return false, err
		}
	}
	return true, nil
}

// derivedMap is derived for the elements of a nested map.
func (p setNestedPlanner) derivedMap(attrs map[string]schema.Attribute, config, plan tftypes.Value) (bool, error) {
	if !plan.IsKnown() || plan.IsNull() {
		return false, nil
	}
	var c, pl map[string]tftypes.Value
	if err := asAll(config, &c, plan, &pl); err != nil {
		return false, err
	}
	if len(c) != len(pl) {
		return false, nil
	}
	for key, cv := range c {
		pv, ok := pl[key]
		if !ok {
			return false, nil
		}
		if ok, err := p.derived(attrs, cv, pv); err != nil || !ok {
			return false, err
		}
	}
	return true, nil
}

// unchanged reports whether the prior object satisfies the configuration, so
// that planning it unchanged is correct. plan is the object planned from config.
func (p setNestedPlanner) unchanged(attrs map[string]schema.Attribute, config, plan, prior tftypes.Value) (bool, error) {
	if !config.IsKnown() || !prior.IsKnown() {
		return false, nil
	}
	if config.IsNull() || prior.IsNull() {
		return config.IsNull() && prior.IsNull(), nil
	}
	var c, pl, s map[string]tftypes.Value
	if err := asAll(config, &c, prior, &s); err != nil {
		return false, err
	}
	if isKnownNonNull(plan) {
		if err := plan.As(&pl); err != nil {
			return false, err
		}
	}

	for name, a := range attrs {
		cv, pv, sv := c[name], pl[name], s[name]
		if a.IsComputed() && !a.IsOptional() {
			continue // Computed-only: the configuration has no say
		}
		if !cv.IsKnown() {
			return false, nil
		}
		if cv.IsNull() {
			if !unchangedOmitted(a, pv, sv) {
				return false, nil
			}
			continue
		}

		var ok bool
		var err error
		switch a := a.(type) {
		case schema.SingleNestedAttribute:
			ok, err = p.unchanged(a.Attributes, cv, pv, sv)
		case schema.SetNestedAttribute:
			ok, err = p.unchangedSet(a.NestedObject.Attributes, cv, pv, sv)
		case schema.ListNestedAttribute:
			ok, err = p.unchangedList(a.NestedObject.Attributes, cv, pv, sv)
		case schema.MapNestedAttribute:
			ok, err = p.unchangedMap(a.NestedObject.Attributes, cv, pv, sv)
		default:
			ok = sv.Equal(cv)
		}
		if err != nil || !ok {
			return false, err
		}
	}
	return true, nil
}

// unchangedOmitted is unchanged for an attribute the configuration omits.
func unchangedOmitted(a schema.Attribute, plan, prior tftypes.Value) bool {
	if clearer, ok := attributeClearer(a); ok {
		// Category A: omitted means cleared.
		return prior.IsNull() || prior.Equal(clearer.clearedValue(prior.Type()))
	}
	if !a.IsComputed() {
		return prior.IsNull() // plain Optional: omitted means null
	}
	if attributeHasDefault(a) {
		return prior.Equal(plan) // the plan holds the default
	}
	return true // Category B: the prior value stays
}

func (p setNestedPlanner) unchangedSet(attrs map[string]schema.Attribute, config, plan, prior tftypes.Value) (bool, error) {
	if !isKnownNonNull(prior) {
		return false, nil
	}
	var c, pl, s []tftypes.Value
	if err := asAll(config, &c, prior, &s); err != nil {
		return false, err
	}
	if len(c) != len(s) {
		return false, nil
	}
	// Each configured element pairs with its planned element, when the plan
	// is known, and must match a distinct prior element.
	var pairs []int
	if isKnownNonNull(plan) {
		if err := plan.As(&pl); err != nil {
			return false, err
		}
		var err error
		if pairs, err = p.pair(attrs, c, pl); err != nil || pairs == nil {
			return false, err
		}
	}
	matches, err := matchOneToOne(len(c), len(s), func(ci, si int) (bool, error) {
		planned := tftypes.NewValue(c[ci].Type(), nil)
		if pairs != nil {
			planned = pl[pairs[ci]]
		}
		return p.unchanged(attrs, c[ci], planned, s[si])
	})
	if err != nil {
		return false, err
	}
	return allMatched(matches), nil
}

func (p setNestedPlanner) unchangedList(attrs map[string]schema.Attribute, config, plan, prior tftypes.Value) (bool, error) {
	if !isKnownNonNull(prior) {
		return false, nil
	}
	var c, pl, s []tftypes.Value
	if err := asAll(config, &c, prior, &s); err != nil {
		return false, err
	}
	if len(c) != len(s) {
		return false, nil
	}
	if isKnownNonNull(plan) {
		if err := plan.As(&pl); err != nil {
			return false, err
		}
	}
	for i := range c {
		planned := tftypes.NewValue(c[i].Type(), nil)
		if i < len(pl) {
			planned = pl[i]
		}
		ok, err := p.unchanged(attrs, c[i], planned, s[i])
		if err != nil || !ok {
			return false, err
		}
	}
	return true, nil
}

func (p setNestedPlanner) unchangedMap(attrs map[string]schema.Attribute, config, plan, prior tftypes.Value) (bool, error) {
	if !isKnownNonNull(prior) {
		return false, nil
	}
	var c, pl, s map[string]tftypes.Value
	if err := asAll(config, &c, prior, &s); err != nil {
		return false, err
	}
	if len(c) != len(s) {
		return false, nil
	}
	if isKnownNonNull(plan) {
		if err := plan.As(&pl); err != nil {
			return false, err
		}
	}
	for key, cv := range c {
		sv, ok := s[key]
		if !ok {
			return false, nil
		}
		planned, ok := pl[key]
		if !ok {
			planned = tftypes.NewValue(cv.Type(), nil)
		}
		if ok, err := p.unchanged(attrs, cv, planned, sv); err != nil || !ok {
			return false, err
		}
	}
	return true, nil
}

// cleared plans the cleared value for every omitted Category A attribute of a
// changed element, which the clearing modifiers would plan outside a set. It
// descends into nested objects, lists, sets and maps of the element.
func (p setNestedPlanner) cleared(attrs map[string]schema.Attribute, config, plan tftypes.Value, at string) (tftypes.Value, error) {
	if !isKnownNonNull(config) || !isKnownNonNull(plan) {
		return plan, nil
	}
	var c, pl map[string]tftypes.Value
	if err := asAll(config, &c, plan, &pl); err != nil {
		return plan, err
	}

	changed := false
	for name, a := range attrs {
		cv, pv := c[name], pl[name]
		v := pv
		var err error
		if clearer, ok := attributeClearer(a); ok && cv.IsNull() {
			v = clearer.clearedValue(pv.Type())
		} else {
			switch a := a.(type) {
			case schema.SingleNestedAttribute:
				v, err = p.cleared(a.Attributes, cv, pv, attributePath(at, name))
			case schema.ListNestedAttribute:
				v, err = p.clearedElements(a.NestedObject.Attributes, cv, pv, false, attributePath(at, name))
			case schema.SetNestedAttribute:
				v, err = p.clearedElements(a.NestedObject.Attributes, cv, pv, true, attributePath(at, name))
			case schema.MapNestedAttribute:
				v, err = p.clearedMap(a.NestedObject.Attributes, cv, pv, attributePath(at, name))
			}
		}
		if err != nil {
			return plan, err
		}
		if !v.Equal(pv) {
			pl[name] = v
			changed = true
		}
	}
	if !changed {
		return plan, nil
	}
	return tftypes.NewValue(plan.Type(), pl), nil
}

// clearedElements is cleared for the elements of a nested list or set. List
// elements correspond by index; set elements are paired first.
func (p setNestedPlanner) clearedElements(attrs map[string]schema.Attribute, config, plan tftypes.Value, isSet bool, at string) (tftypes.Value, error) {
	if !isKnownNonNull(config) || !isKnownNonNull(plan) {
		return plan, nil
	}
	var c, pl []tftypes.Value
	if err := asAll(config, &c, plan, &pl); err != nil {
		return plan, err
	}
	if len(c) != len(pl) {
		return plan, nil
	}
	pairs := make([]int, len(c))
	for i := range pairs {
		pairs[i] = i
	}
	if isSet {
		var err error
		if pairs, err = p.pair(attrs, c, pl); err != nil || pairs == nil {
			return plan, err
		}
	}

	out := make([]tftypes.Value, len(pl))
	configs := make([]tftypes.Value, len(pl))
	for ci, pi := range pairs {
		configs[pi] = c[ci]
		v, err := p.cleared(attrs, c[ci], pl[pi], at+"[*]")
		if err != nil {
			return plan, err
		}
		out[pi] = v
	}
	if isSet {
		if distinct, err := distinctSetElements(out, configs, at); err != nil || !distinct {
			return plan, err
		}
	}
	return tftypes.NewValue(plan.Type(), out), nil
}

// clearedMap is cleared for the elements of a nested map, key by key.
func (p setNestedPlanner) clearedMap(attrs map[string]schema.Attribute, config, plan tftypes.Value, at string) (tftypes.Value, error) {
	if !isKnownNonNull(config) || !isKnownNonNull(plan) {
		return plan, nil
	}
	var c, pl map[string]tftypes.Value
	if err := asAll(config, &c, plan, &pl); err != nil {
		return plan, err
	}
	for key, pv := range pl {
		cv, ok := c[key]
		if !ok {
			continue
		}
		v, err := p.cleared(attrs, cv, pv, at+"[*]")
		if err != nil {
			return plan, err
		}
		pl[key] = v
	}
	return tftypes.NewValue(plan.Type(), pl), nil
}

// attributeClearer returns the clearing plan modifier of an attribute, for
// every attribute kind that has plan modifiers.
func attributeClearer(a schema.Attribute) (configNullClearer, bool) {
	switch a := a.(type) {
	case schema.StringAttribute:
		return clearerIn(a.PlanModifiers)
	case schema.BoolAttribute:
		return clearerIn(a.PlanModifiers)
	case schema.Int64Attribute:
		return clearerIn(a.PlanModifiers)
	case schema.Int32Attribute:
		return clearerIn(a.PlanModifiers)
	case schema.Float64Attribute:
		return clearerIn(a.PlanModifiers)
	case schema.Float32Attribute:
		return clearerIn(a.PlanModifiers)
	case schema.NumberAttribute:
		return clearerIn(a.PlanModifiers)
	case schema.ListAttribute:
		return clearerIn(a.PlanModifiers)
	case schema.SetAttribute:
		return clearerIn(a.PlanModifiers)
	case schema.MapAttribute:
		return clearerIn(a.PlanModifiers)
	case schema.ObjectAttribute:
		return clearerIn(a.PlanModifiers)
	case schema.DynamicAttribute:
		return clearerIn(a.PlanModifiers)
	case schema.SingleNestedAttribute:
		return clearerIn(a.PlanModifiers)
	case schema.ListNestedAttribute:
		return clearerIn(a.PlanModifiers)
	case schema.SetNestedAttribute:
		return clearerIn(a.PlanModifiers)
	case schema.MapNestedAttribute:
		return clearerIn(a.PlanModifiers)
	}
	return nil, false
}

// clearerIn returns the first plan modifier that is a configNullClearer.
func clearerIn[M any](modifiers []M) (configNullClearer, bool) {
	for _, m := range modifiers {
		if clearer, ok := any(m).(configNullClearer); ok {
			return clearer, true
		}
	}
	return nil, false
}

// attributeHasDefault reports whether an attribute has a schema default.
func attributeHasDefault(a schema.Attribute) bool {
	switch a := a.(type) {
	case schema.StringAttribute:
		return a.Default != nil
	case schema.BoolAttribute:
		return a.Default != nil
	case schema.Int64Attribute:
		return a.Default != nil
	case schema.Int32Attribute:
		return a.Default != nil
	case schema.Float64Attribute:
		return a.Default != nil
	case schema.Float32Attribute:
		return a.Default != nil
	case schema.NumberAttribute:
		return a.Default != nil
	case schema.ListAttribute:
		return a.Default != nil
	case schema.SetAttribute:
		return a.Default != nil
	case schema.MapAttribute:
		return a.Default != nil
	case schema.ObjectAttribute:
		return a.Default != nil
	case schema.SingleNestedAttribute:
		return a.Default != nil
	case schema.ListNestedAttribute:
		return a.Default != nil
	case schema.SetNestedAttribute:
		return a.Default != nil
	case schema.MapNestedAttribute:
		return a.Default != nil
	}
	return false
}

func isKnownNonNull(v tftypes.Value) bool {
	return v.IsKnown() && !v.IsNull()
}

// asAll converts pairs of (value, destination) with tftypes.Value.As.
func asAll(pairs ...any) error {
	for i := 0; i+1 < len(pairs); i += 2 {
		v, ok := pairs[i].(tftypes.Value)
		if !ok {
			return fmt.Errorf("asAll: argument %d is %T, not tftypes.Value", i, pairs[i])
		}
		if err := v.As(pairs[i+1]); err != nil {
			return err
		}
	}
	return nil
}
