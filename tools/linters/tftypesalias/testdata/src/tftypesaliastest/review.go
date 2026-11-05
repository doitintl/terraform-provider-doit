package tftypesaliastest

import (
	"maps"
	"slices"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Mutating slices helpers, including results assigned back to the destination.
func mutatingHelpers(v tftypes.Value, x tftypes.Value) {
	var s []tftypes.Value
	_ = v.As(&s)
	_ = slices.Delete(s, 0, 1)                                               // want `writing to "s" changes the original value`
	_ = slices.DeleteFunc(s, func(tftypes.Value) bool { return true })       // want `writing to "s" changes the original value`
	_ = slices.Insert(s, 0, x)                                               // want `writing to "s" changes the original value`
	_ = slices.Replace(s, 0, 1, x)                                           // want `writing to "s" changes the original value`
	_ = slices.Compact(s)                                                    // want `writing to "s" changes the original value`
	_ = slices.CompactFunc(s, func(a, b tftypes.Value) bool { return true }) // want `writing to "s" changes the original value`
}

func deleteAssignedBack(v tftypes.Value) {
	var s []tftypes.Value
	_ = v.As(&s)
	s = slices.Delete(s, 0, 1) // want `writing to "s" changes the original value`
}

// A second As call re-arms the check after a clone.
func secondAs(v tftypes.Value, x tftypes.Value) {
	var m map[string]tftypes.Value
	_ = v.As(&m)
	m = maps.Clone(m)
	m["safe"] = x
	_ = v.As(&m)
	m["a"] = x // want `writing to "m" changes the original value`
}

// A short declaration that reuses m is a reassignment.
func shortDeclReassign(v tftypes.Value, x tftypes.Value) {
	var m map[string]tftypes.Value
	_ = v.As(&m)
	m, copied := maps.Clone(m), true
	_ = copied
	m["a"] = x
}

// Writes through a subslice mutate the same backing array.
func subslice(v tftypes.Value, x tftypes.Value, other []tftypes.Value) {
	var s []tftypes.Value
	_ = v.As(&s)
	copy(s[1:], other) // want `writing to "s" changes the original value`
	clear(s[:])        // want `writing to "s" changes the original value`
	s[:][0] = x        // want `writing to "s" changes the original value`
}

// A reassignment that is not guaranteed before the write does not clear it.
func conditionalClone(v tftypes.Value, x tftypes.Value, clone bool) {
	var m map[string]tftypes.Value
	_ = v.As(&m)
	if clone {
		m = maps.Clone(m)
	}
	m["a"] = x // want `writing to "m" changes the original value`
}

func cloneInUncalledClosure(v tftypes.Value, x tftypes.Value) {
	var m map[string]tftypes.Value
	_ = v.As(&m)
	_ = func() {
		m = maps.Clone(m)
	}
	m["a"] = x // want `writing to "m" changes the original value`
}

// A clone in an enclosing block still clears later writes in nested blocks.
func cloneThenNestedWrite(v tftypes.Value, x tftypes.Value, ok bool) {
	var m map[string]tftypes.Value
	_ = v.As(&m)
	m = maps.Clone(m)
	if ok {
		m["a"] = x
	}
}
