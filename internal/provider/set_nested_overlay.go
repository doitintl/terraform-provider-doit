package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// overlaySetElements is the set counterpart of overlayListElements. Set
// elements have no index, so each planned element is paired one-to-one with a
// resolved element that agrees with all of its known values (unknown values
// match anything), and the overlay function is invoked for each pair. Planned elements
// without a counterpart keep their values.
//
// Resolved elements are built by the mapping function with the plan as prior
// state, so values the API normalizes keep their planned spelling and still
// pair up.
func overlaySetElements[T attr.Value](ctx context.Context, resolved, plan *types.Set, overlayFn func(context.Context, *T, *T) diag.Diagnostics) diag.Diagnostics {
	var diags diag.Diagnostics
	if resolved == nil || resolved.IsNull() || resolved.IsUnknown() || plan == nil || plan.IsNull() || plan.IsUnknown() {
		return diags
	}
	var planElems []T
	var resolvedElems []T

	planDiags := plan.ElementsAs(ctx, &planElems, true)
	diags.Append(planDiags...)
	if planDiags.HasError() {
		return diags
	}
	resolvedDiags := resolved.ElementsAs(ctx, &resolvedElems, true)
	diags.Append(resolvedDiags...)
	if resolvedDiags.HasError() {
		return diags
	}

	resolvedValues := make([]tftypes.Value, len(resolvedElems))
	for i, r := range resolvedElems {
		v, err := r.ToTerraformValue(ctx)
		if err != nil {
			diags.AddError("Error Overlaying Set Elements", err.Error())
			return diags
		}
		resolvedValues[i] = v
	}

	planValues := make([]tftypes.Value, len(planElems))
	for i, pe := range planElems {
		v, err := pe.ToTerraformValue(ctx)
		if err != nil {
			diags.AddError("Error Overlaying Set Elements", err.Error())
			return diags
		}
		planValues[i] = v
	}

	matches, _ := matchOneToOne(len(planElems), len(resolvedElems), func(pi, ri int) (bool, error) {
		if planElems[pi].IsNull() || planElems[pi].IsUnknown() || resolvedElems[ri].IsNull() || resolvedElems[ri].IsUnknown() {
			return false, nil
		}
		return knownValuesMatch(planValues[pi], resolvedValues[ri]), nil
	})
	for pi, ri := range matches {
		if ri >= 0 {
			diags.Append(overlayFn(ctx, &resolvedElems[ri], &planElems[pi])...)
		}
	}

	newSet, rebuildDiags := types.SetValueFrom(ctx, plan.ElementType(ctx), planElems)
	diags.Append(rebuildDiags...)
	if !rebuildDiags.HasError() {
		*plan = newSet
	}
	return diags
}

// knownValuesMatch reports whether value agrees with every known part of
// pattern. Unknown parts of pattern match anything.
func knownValuesMatch(pattern, value tftypes.Value) bool {
	if !pattern.IsKnown() {
		return true
	}
	if !value.IsKnown() || pattern.IsNull() || value.IsNull() {
		return pattern.IsNull() == value.IsNull() && value.IsKnown()
	}

	switch pattern.Type().(type) {
	case tftypes.Object, tftypes.Map:
		var p, v map[string]tftypes.Value
		if pattern.As(&p) != nil || value.As(&v) != nil || len(p) != len(v) {
			return false
		}
		for k, pv := range p {
			vv, ok := v[k]
			if !ok || !knownValuesMatch(pv, vv) {
				return false
			}
		}
		return true
	case tftypes.List, tftypes.Tuple:
		var p, v []tftypes.Value
		if pattern.As(&p) != nil || value.As(&v) != nil || len(p) != len(v) {
			return false
		}
		for i := range p {
			if !knownValuesMatch(p[i], v[i]) {
				return false
			}
		}
		return true
	case tftypes.Set:
		var p, v []tftypes.Value
		if pattern.As(&p) != nil || value.As(&v) != nil || len(p) != len(v) {
			return false
		}
		matches, _ := matchOneToOne(len(p), len(v), func(pi, vi int) (bool, error) {
			return knownValuesMatch(p[pi], v[vi]), nil
		})
		return allMatched(matches)
	}
	return pattern.Equal(value)
}
