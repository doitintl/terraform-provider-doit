package tftypesaliastest

import (
	"maps"
	"slices"
	"sort"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func mapIndexWrite(v tftypes.Value, x tftypes.Value) {
	var m map[string]tftypes.Value
	if err := v.As(&m); err != nil {
		return
	}
	m["a"] = x // want `writing to "m" changes the original value`
}

func sliceIndexWrite(v tftypes.Value, x tftypes.Value) {
	var s []tftypes.Value
	_ = v.As(&s)
	s[0] = x // want `writing to "s" changes the original value`
}

func deleteAndClear(v tftypes.Value) {
	var m map[string]tftypes.Value
	_ = v.As(&m)
	delete(m, "a") // want `writing to "m" changes the original value`
	clear(m)       // want `writing to "m" changes the original value`
}

func inPlaceSlices(v tftypes.Value, other []tftypes.Value) {
	var s []tftypes.Value
	_ = v.As(&s)
	slices.Reverse(s)                                             // want `writing to "s" changes the original value`
	copy(s, other)                                                // want `writing to "s" changes the original value`
	sort.Slice(s, func(i, j int) bool { return i < j })           // want `writing to "s" changes the original value`
	slices.SortFunc(s, func(a, b tftypes.Value) int { return 0 }) // want `writing to "s" changes the original value`
}

func writeInLoopAndClosure(v tftypes.Value, x tftypes.Value) {
	var m map[string]tftypes.Value
	_ = v.As(&m)
	for k := range m {
		m[k] = x // want `writing to "m" changes the original value`
	}
	func() {
		m["b"] = x // want `writing to "m" changes the original value`
	}()
}

func shortVarDecl(v tftypes.Value, x tftypes.Value) {
	m := map[string]tftypes.Value{}
	_ = v.As(&m)
	m["a"] = x // want `writing to "m" changes the original value`
}

func readOnly(v tftypes.Value) int {
	var m map[string]tftypes.Value
	_ = v.As(&m)
	n := 0
	for range m {
		n++
	}
	_ = m["a"]
	return n + len(m)
}

func clonedBeforeWrite(v tftypes.Value, x tftypes.Value) {
	var m map[string]tftypes.Value
	_ = v.As(&m)
	m = maps.Clone(m)
	m["a"] = x

	var s []tftypes.Value
	_ = v.As(&s)
	s = slices.Clone(s)
	s[0] = x
}

// asCopy-style helper: the destination is an any, not a typed variable.
func asCopy(v tftypes.Value, dst any) error {
	return v.As(dst)
}

func otherTypes(v tftypes.Value) {
	var str string
	var strs []string
	_ = v.As(&str)
	_ = v.As(&strs)
	strs[0] = "x"
}

func unrelatedMap(v tftypes.Value, x tftypes.Value) {
	var m map[string]tftypes.Value
	_ = v.As(&m)
	other := map[string]tftypes.Value{}
	other["a"] = x
}
