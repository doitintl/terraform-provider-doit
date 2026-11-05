package tftypesaliastest

import (
	"maps"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Package-level function literals are analyzed too.
var pkgLevelMutate = func(v tftypes.Value, x tftypes.Value) {
	var m map[string]tftypes.Value
	_ = v.As(&m)
	m["a"] = x // want `writing to "m" changes the original value`
}

// A nested literal is reported once, not once per enclosing body.
var pkgLevelNested = func(v tftypes.Value, x tftypes.Value) {
	var m map[string]tftypes.Value
	_ = v.As(&m)
	func() {
		m["a"] = x // want `writing to "m" changes the original value`
	}()
}

// A range assignment rebinds m to an independent map inside the loop.
func rangeRebind(v tftypes.Value, x tftypes.Value) {
	var m map[string]tftypes.Value
	_ = v.As(&m)
	for _, m = range []map[string]tftypes.Value{make(map[string]tftypes.Value)} {
		m["a"] = x
	}
}

// ...but a zero-iteration loop does not rebind it afterwards.
func rangeRebindThenWrite(v tftypes.Value, x tftypes.Value, others []map[string]tftypes.Value) {
	var m map[string]tftypes.Value
	_ = v.As(&m)
	for _, m = range others {
	}
	m["a"] = x // want `writing to "m" changes the original value`
}

// Loop-carried alias: the write runs on the second iteration, after As.
func loopCarried(v tftypes.Value, x tftypes.Value) {
	var m map[string]tftypes.Value
	for i := 0; i < 2; i++ {
		if i > 0 {
			m["a"] = x // want `writing to "m" changes the original value`
		}
		_ = v.As(&m)
	}
}

// A clone later in each iteration makes the next iteration's write safe.
func loopCarriedCloned(v tftypes.Value, x tftypes.Value) {
	var m map[string]tftypes.Value
	for i := 0; i < 2; i++ {
		if i > 0 {
			m["a"] = x
		}
		_ = v.As(&m)
		m = maps.Clone(m)
	}
}

// As in a loop post statement runs after the body, so the body write on a
// later iteration is aliased; the loop may not run at all, so nothing is
// reported after it.
func asInLoopPost(v tftypes.Value, x tftypes.Value, n int) {
	var m map[string]tftypes.Value
	for i := 0; i < n; i, _ = i+1, v.As(&m) {
		m["a"] = x // want `writing to "m" changes the original value`
	}
	m["b"] = x
}

// An else-if initializer only runs when the earlier conditions are false.
func elseIfInit(v tftypes.Value, x tftypes.Value, keepAlias bool) {
	var m map[string]tftypes.Value
	_ = v.As(&m)
	if keepAlias {
	} else if m = maps.Clone(m); true {
	}
	m["a"] = x // want `writing to "m" changes the original value`
}

// An ordinary if initializer always runs.
func ifInit(v tftypes.Value, x tftypes.Value) {
	var m map[string]tftypes.Value
	_ = v.As(&m)
	if m = maps.Clone(m); true {
	}
	m["a"] = x
}
