package retrypolicy

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestRetryPolicy(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "retrypolicy")
}
