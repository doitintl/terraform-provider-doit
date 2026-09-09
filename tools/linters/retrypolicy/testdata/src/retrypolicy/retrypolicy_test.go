package retrypolicy

import (
	"net/http"
	"testing"
)

func TestBad_OmitsBackoff(t *testing.T) {
	c := &http.Client{}
	_ = &DCIRetryClient{client: c} // want `omits newBackOff`
}

func TestBad_EmptyLiteral(t *testing.T) {
	_ = &DCIRetryClient{} // want `omits newBackOff`
}

func TestBad_ValueLiteral(t *testing.T) {
	_ = DCIRetryClient{client: &http.Client{}} // want `omits newBackOff`
}

func TestBad_ElidedNestedLiteral(t *testing.T) {
	// lit.Type is nil here; matching on it would miss this site.
	_ = []*DCIRetryClient{{client: &http.Client{}}} // want `omits newBackOff`
}

func TestBad_HelperWithNilPolicy(t *testing.T) {
	_ = newTestRetryClient(&http.Client{}, 1, nil) // want `nil backoff policy`
}

func TestGood_NamesPolicy(t *testing.T) {
	_ = &DCIRetryClient{
		client:     &http.Client{},
		newBackOff: constantBackOff(1),
	}
}

func TestGood_ExplicitNilIsTheEscapeHatch(t *testing.T) {
	_ = &DCIRetryClient{
		client:     &http.Client{},
		newBackOff: nil,
	}
}

func TestGood_PositionalLiteralSuppliesEveryField(t *testing.T) {
	_ = &DCIRetryClient{&http.Client{}, constantBackOff(1)}
}

func TestGood_HelperWithRealPolicy(t *testing.T) {
	_ = newTestRetryClient(&http.Client{}, 1, constantBackOff(1))
}

func TestGood_ProductionConstructorInheritsNilOnPurpose(t *testing.T) {
	_ = newClientWithHTTPClient(&http.Client{})
}

func TestBad_WrapperWithNilPolicy(t *testing.T) {
	// The policy sits at a different argument position here than in
	// newTestRetryClient; the type-driven check finds it regardless.
	_ = newTestRetryAPIClient(&http.Client{}, 1, nil) // want `nil backoff policy`
}

func TestGood_WrapperWithRealPolicy(t *testing.T) {
	_ = newTestRetryAPIClient(&http.Client{}, 1, constantBackOff(1))
}

func TestGood_NilForANonFactoryParamIsIgnored(t *testing.T) {
	takesAnyPointer(nil)
}

func TestBoundary_NilViaVariableIsNotCaught(t *testing.T) {
	// Documents a known limitation: only a literal nil is detected. Catching
	// this would need dataflow analysis. No `want` comment — it is not reported.
	var policy func() BackOff
	_ = newTestRetryClient(&http.Client{}, 1, policy)
}

func TestBoundary_LaterAssignmentIsNotCaught(t *testing.T) {
	// Documents the known false negative recorded in the package doc: the field
	// is set after the literal, so the literal itself names it and passes. No
	// `want` comment — the literal below is reported, but this one is not,
	// because it does name the field.
	c := &DCIRetryClient{
		client:     &http.Client{},
		newBackOff: constantBackOff(1),
	}
	c.newBackOff = nil
	_ = c
}
