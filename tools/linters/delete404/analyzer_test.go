package delete404

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestDelete404(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "deletetest")
}
