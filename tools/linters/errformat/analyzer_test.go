package errformat

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestErrFormat(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "errfmttest")
}
