package constructor

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestConstructor(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "constructortest")
}
