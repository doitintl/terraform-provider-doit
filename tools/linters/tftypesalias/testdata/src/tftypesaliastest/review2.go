package tftypesaliastest

import (
	"maps"
	"slices"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// The As call cannot reach the write: m is an independent map there.
func asOnOtherBranch(v tftypes.Value, x tftypes.Value, useValue bool) {
	m := make(map[string]tftypes.Value)
	if useValue {
		_ = v.As(&m)
		return
	}
	m["a"] = x
}

// A write in the same block as the As call is still reported.
func asThenWriteInNestedBlock(v tftypes.Value, x tftypes.Value, ok bool) {
	var m map[string]tftypes.Value
	_ = v.As(&m)
	if ok {
		m["a"] = x // want `writing to "m" changes the original value`
	}
}

// A loop post statement runs after the body, and not at all for zero iterations.
func loopPostClone(v tftypes.Value, x tftypes.Value, run bool) {
	var m map[string]tftypes.Value
	_ = v.As(&m)
	for ; run; m = maps.Clone(m) {
		m["a"] = x // want `writing to "m" changes the original value`
		run = false
	}
}

func loopPostCloneThenWrite(v tftypes.Value, x tftypes.Value, run bool) {
	var m map[string]tftypes.Value
	_ = v.As(&m)
	for ; run; m = maps.Clone(m) {
		run = false
	}
	m["a"] = x // want `writing to "m" changes the original value`
}

// A loop init statement runs once, before the body.
func loopInitClone(v tftypes.Value, x tftypes.Value, run bool) {
	var m map[string]tftypes.Value
	_ = v.As(&m)
	for m = maps.Clone(m); run; run = false {
		m["a"] = x
	}
}

// Range assignment targets are writes.
func rangeAssignTarget(v tftypes.Value, values []tftypes.Value) {
	var m map[string]tftypes.Value
	_ = v.As(&m)
	for _, m["a"] = range values { // want `writing to "m" changes the original value`
	}
}

// As through a pointer receiver returns the same shared collections.
func pointerReceiver(v *tftypes.Value) {
	var m map[string]tftypes.Value
	_ = v.As(&m)
	delete(m, "a") // want `writing to "m" changes the original value`
}

// Explicit instantiation of a generic mutator.
func instantiated(v tftypes.Value) {
	var s []tftypes.Value
	_ = v.As(&s)
	slices.Reverse[[]tftypes.Value](s)                // want `writing to "s" changes the original value`
	slices.Reverse[[]tftypes.Value, tftypes.Value](s) // want `writing to "s" changes the original value`
}
