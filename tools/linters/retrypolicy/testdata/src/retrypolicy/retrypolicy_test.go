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
