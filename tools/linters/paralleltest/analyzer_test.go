package paralleltest

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestParallelTest(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "paralleltest")
}
