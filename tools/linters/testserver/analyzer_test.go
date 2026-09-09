package testserver

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestTestServer(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "testserver")
}
