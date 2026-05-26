package newexpr

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestNewExpr(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "newexprtest")
}
