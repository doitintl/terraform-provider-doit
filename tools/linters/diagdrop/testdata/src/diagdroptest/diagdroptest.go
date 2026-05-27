package diagdroptest

import (
	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// Stub helpers to simulate Terraform framework calls.
func lookupUser() (string, diag.Diagnostics) {
	return "", nil
}

func mapToModel() diag.Diagnostics {
	return nil
}

type NullValue struct{}

func newNullValue() (NullValue, diag.Diagnostics) {
	return NullValue{}, nil
}

// --- BAD: return nil after capturing diags ---

// badReturnNilAfterCapture captures diags then returns nil on success.
func badReturnNilAfterCapture() diag.Diagnostics {
	user, diags := lookupUser()
	if diags.HasError() {
		return diags
	}
	if user == "" {
		return nil // want "return nil drops captured diag.Diagnostics variable \"diags\"; return diags instead"
	}
	return nil // want "return nil drops captured diag.Diagnostics variable \"diags\"; return diags instead"
}

// badReturnNilMultiCapture captures diags via multiple assignments.
func badReturnNilMultiCapture() diag.Diagnostics {
	_, diags := lookupUser()
	if diags.HasError() {
		return diags
	}
	diags = mapToModel()
	if diags.HasError() {
		return diags
	}
	return nil // want "return nil drops captured diag.Diagnostics variable \"diags\"; return diags instead"
}

// badNamedReturnExplicitNil uses a named return but explicitly returns nil.
func badNamedReturnExplicitNil() (diags diag.Diagnostics) {
	_, d := lookupUser()
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}
	return nil // want "return nil drops captured diag.Diagnostics variable \"diags\"; return diags instead"
}

// badSingleReturnDiags has only diag.Diagnostics as the return type.
func badSingleReturnDiags() diag.Diagnostics {
	diags := mapToModel()
	if diags.HasError() {
		return diags
	}
	return nil // want "return nil drops captured diag.Diagnostics variable \"diags\"; return diags instead"
}

// badMultiReturn has multiple return values including diags.
func badMultiReturn() (string, diag.Diagnostics) {
	user, diags := lookupUser()
	if diags.HasError() {
		return "", diags
	}
	return user, nil // want "return nil drops captured diag.Diagnostics variable \"diags\"; return diags instead"
}

// --- GOOD: diagnostics properly returned ---

// goodReturnDiagsAlways returns diags on all paths.
func goodReturnDiagsAlways() diag.Diagnostics {
	user, diags := lookupUser()
	if diags.HasError() {
		return diags
	}
	if user == "" {
		return diags
	}
	return diags
}

// goodNoCaptureInlineConstruct never captures diags — constructs inline.
func goodNoCaptureInlineConstruct() diag.Diagnostics {
	return nil
}

// goodNoCaptureInlineError returns inline-constructed diagnostics.
func goodNoCaptureInlineError() diag.Diagnostics {
	return diag.Diagnostics{}
}

// goodOverlayHelper never captures diags, returns nil intentionally.
func goodOverlayHelper() diag.Diagnostics {
	// Simulates overlay helpers that do field assignments and return nil.
	_ = "do something"
	return nil
}

// goodNamedReturnBareReturn uses named return with bare return.
func goodNamedReturnBareReturn() (diags diag.Diagnostics) {
	_, d := lookupUser()
	diags.Append(d...)
	return
}

// goodNotDiagsReturn is a function that returns error, not diags.
func goodNotDiagsReturn() error {
	return nil
}

// goodMultiReturnProper returns diags in multi-return on all paths.
func goodMultiReturnProper() (string, diag.Diagnostics) {
	user, diags := lookupUser()
	if diags.HasError() {
		return "", diags
	}
	return user, diags
}

// goodEarlyReturnBeforeCapture returns nil before any diag variable is assigned.
// This is the common nil-guard pattern in mapping helpers.
func goodEarlyReturnBeforeCapture() (NullValue, diag.Diagnostics) {
	// Early return — no diag variable exists yet.
	if true {
		return NullValue{}, nil
	}

	// diags variable appears later.
	val, d := newNullValue()
	_ = d
	return val, d
}

// goodMultiEarlyReturnBeforeCapture has multiple early returns before capture.
func goodMultiEarlyReturnBeforeCapture() (NullValue, diag.Diagnostics) {
	if true {
		return NullValue{}, nil
	}
	if false {
		return NullValue{}, nil
	}
	var diags diag.Diagnostics
	return NullValue{}, diags
}
