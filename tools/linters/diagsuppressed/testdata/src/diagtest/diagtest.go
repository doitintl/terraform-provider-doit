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

// --- BAD: suppressed diagnostics ---

func badSuppressedDiag() {
	_, _ = listValue() // want "diag.Diagnostics return value must not be suppressed"
}

func badSuppressedDiagShortAssign() {
	list, _ := listValue() // want "diag.Diagnostics return value must not be suppressed"
	_ = list
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

// --- NOT flagged: non-diagnostic blank identifiers ---

func okBlankNonDiag() {
	_ = stringValue() // Single return, not diagnostics.
}

func okBlankTwoStrings() {
	_, _ = twoStrings() // Two strings, not diagnostics.
}
