package diagdroptest

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
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

// --- BAD: var declaration patterns ---

// badVarDeclaration uses var declaration instead of :=.
func badVarDeclaration() diag.Diagnostics {
	var diags diag.Diagnostics
	_, d := lookupUser()
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}
	return nil // want "return nil drops captured diag.Diagnostics variable \"diags\"; return diags instead"
}

// badVarDeclarationOnly uses only var declaration, no assignments.
func badVarDeclarationOnly() diag.Diagnostics {
	var diags diag.Diagnostics
	_ = diags.HasError()
	return nil // want "return nil drops captured diag.Diagnostics variable \"diags\"; return diags instead"
}

// --- BAD: accumulator preference ---

// badAccumulatorPreferred has both a temporary d and an accumulator diags.
// The suggestion should name "diags" (the accumulator), not "d" (the temporary).
func badAccumulatorPreferred() diag.Diagnostics {
	_, d := lookupUser()
	var diags diag.Diagnostics
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}
	return nil // want "return nil drops captured diag.Diagnostics variable \"diags\"; return diags instead"
}

// badClosureFalsePositive returns diag.Diagnostics but contains a closure that returns error/nil.
func badClosureFalsePositive() diag.Diagnostics {
	_, diags := lookupUser()
	if diags.HasError() {
		return diags
	}
	fn := func() error {
		return nil // Should NOT be flagged
	}
	_ = fn()
	return nil // want "return nil drops captured diag.Diagnostics variable \"diags\"; return diags instead"
}

// goodInnerBlockScope returns nil at the end, but the diags variable was only in an inner scope.
func goodInnerBlockScope() diag.Diagnostics {
	if true {
		diags := mapToModel()
		if diags.HasError() {
			return diags
		}
	}
	return nil // Should NOT be flagged
}

// ============================================================================
// Direction 2: unappended diagnostics in resp-parameter functions
// ============================================================================

// Stub to simulate GetAttribute returning diags.
func getAttribute() diag.Diagnostics {
	return nil
}

// Stub to simulate a helper that propagates diags.
func propagateHelper(d diag.Diagnostics) {}

type myValidator struct{}

// --- BAD: captured diags never appended ---

// badUnappendedDiags captures diags but only uses HasError(), never appends.
func (v *myValidator) ValidateResource(_ context.Context, _ resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	diags := getAttribute() // want `captured diag.Diagnostics variable "diags" is never appended to resp.Diagnostics`
	if diags.HasError() {
		return
	}
	_ = "do something"
}

// badUnappendedDiagsInLoop captures diags in a loop without appending.
func badUnappendedDiagsInLoop(_ resource.CreateRequest, resp *resource.CreateResponse) {
	paths := []string{"a", "b"}
	for range paths {
		diags := getAttribute() // want `captured diag.Diagnostics variable "diags" is never appended to resp.Diagnostics`
		if diags.HasError() {
			continue
		}
	}
}

// --- GOOD: diags properly appended ---

// goodAppendedDiags appends diags before checking.
func goodAppendedDiags(_ resource.ReadRequest, resp *resource.ReadResponse) {
	diags := getAttribute()
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
}

// goodDiagsPassedToHelper passes diags to another function.
func goodDiagsPassedToHelper(_ resource.UpdateRequest, resp *resource.UpdateResponse) {
	diags := getAttribute()
	propagateHelper(diags)
	if diags.HasError() {
		return
	}
	_ = resp
}

// goodDiagsReturned is a function that also returns diags (direction 1 territory).
func goodDiagsReturned(_ resource.DeleteRequest, _ *resource.DeleteResponse) diag.Diagnostics {
	diags := getAttribute()
	if diags.HasError() {
		return diags
	}
	return diags
}

// goodNoCapture has a resp parameter but never captures diags.
func goodNoCapture(_ resource.CreateRequest, resp *resource.CreateResponse) {
	resp.Diagnostics.Append(diag.Diagnostics{}...)
}

// badMixedLoopsSameName has two loops using the same variable name "diags".
// Loop 1 does NOT append, loop 2 DOES append. Object-based tracking ensures
// only loop 1's capture is flagged despite the shared name.
func badMixedLoopsSameName(_ resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	paths1 := []string{"a", "b"}
	for range paths1 {
		diags := getAttribute() // want `captured diag.Diagnostics variable "diags" is never appended to resp.Diagnostics`
		if diags.HasError() {
			continue
		}
	}

	paths2 := []string{"c", "d"}
	for range paths2 {
		diags := getAttribute()
		resp.Diagnostics.Append(diags...)
		if diags.HasError() {
			continue
		}
	}
}

// --- Scope guard: Direction 2 should NOT fire for non-resp functions ---

// goodNotRespFunction captures diags but has no resp parameter — only Direction 1 applies.
func goodNotRespFunction() diag.Diagnostics {
	diags := getAttribute()
	if diags.HasError() {
		return diags
	}
	return nil // want "return nil drops captured diag.Diagnostics variable \"diags\"; return diags instead"
}
