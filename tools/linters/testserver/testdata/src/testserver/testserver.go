// Package testserver is testdata. It imports the real net/http/httptest —
// analysistest resolves stdlib from the host toolchain, so unlike third-party
// dependencies it needs no stub, and the analyzer's package-path match is
// exercised faithfully.
package testserver

import "net/http/httptest"

// wrapper embeds *httptest.Server the way this repo's async test helper does,
// so a promoted Close types as *wrapper rather than *httptest.Server.
type wrapper struct {
	*httptest.Server
	name string
}

// notAServer has a Close of its own, to prove the check is type-gated rather
// than keyed off the method name.
type notAServer struct{}

func (notAServer) Close() error { return nil }

// productionClose is in a non-test file: never flagged.
func productionClose(s *httptest.Server) {
	defer s.Close()
}

var _ = productionClose
