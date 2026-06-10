package clearableattr_test

import (
	"testing"

	"github.com/doitintl/terraform-provider-doit/tools/linters/clearableattr"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestClearableAttr(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, clearableattr.Analyzer, "cattest")
}
