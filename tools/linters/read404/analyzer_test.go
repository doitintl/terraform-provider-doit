package read404

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestRead404(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "readtest")
}
