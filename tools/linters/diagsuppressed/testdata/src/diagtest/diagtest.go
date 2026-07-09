package diagtest

import (
	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// Stub types to simulate the Terraform framework.
type StringValue struct{}
type ListValue struct{}
type AttrValue interface{}

func listValue() (ListValue, diag.Diagnostics) {
	return ListValue{}, nil
}

func stringValue() StringValue {
	return StringValue{}
}

func twoStrings() (string, string) {
	return "", ""
}

// elementsAs returns diag.Diagnostics as a single value, mirroring
// types.Set.ElementsAs / types.List.ElementsAs.
func (ListValue) elementsAs(_ interface{}) diag.Diagnostics {
	return nil
}

// decode returns a value plus diagnostics, mirroring a multi-return helper.
func decode() (ListValue, diag.Diagnostics) {
	return ListValue{}, nil
}

// noReturn mirrors a void method like diags.Append(...).
func (ListValue) noReturn() {}

// --- BAD: suppressed diagnostics ---

func badSuppressedDiag() {
	_, _ = listValue() // want "diag.Diagnostics return value must not be suppressed"
}

func badSuppressedDiagShortAssign() {
	list, _ := listValue() // want "diag.Diagnostics return value must not be suppressed"
	_ = list
}

// --- BAD: discarded bare call (return value dropped entirely) ---

func badDiscardedSingleReturn() {
	var list ListValue
	list.elementsAs(nil) // want "diag.Diagnostics return value must not be discarded"
}

func badDiscardedMultiReturn() {
	decode() // want "diag.Diagnostics return value must not be discarded"
}

// --- GOOD: diagnostics properly handled ---

func goodHandledDiag() {
	_, diags := listValue()
	_ = diags
}

func goodHandledDiagAppend() {
	var allDiags diag.Diagnostics
	_, diags := listValue()
	allDiags.Append(diags...)
	_ = allDiags
}

func goodCapturedDiscardedCall() {
	var list ListValue
	var allDiags diag.Diagnostics
	diags := list.elementsAs(nil)
	allDiags.Append(diags...)
	_ = allDiags
}

// --- NOT flagged: non-diagnostic blank identifiers ---

func okBlankNonDiag() {
	_ = stringValue() // Single return, not diagnostics.
}

func okBlankTwoStrings() {
	_, _ = twoStrings() // Two strings, not diagnostics.
}

// --- NOT flagged: bare calls that do not return diagnostics ---

func okBareVoidCall() {
	var list ListValue
	list.noReturn() // Returns nothing.
}

func okBareNonDiagCall() {
	stringValue() // Single non-diagnostic return, discarded.
}
