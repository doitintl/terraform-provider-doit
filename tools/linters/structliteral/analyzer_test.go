package structliteral_test

import (
	"testing"

	"github.com/doitintl/terraform-provider-doit/tools/linters/structliteral"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestStructLiteral(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, structliteral.Analyzer, "structtest")
}
