package clearableattr_test

import (
	"testing"

	"github.com/doitintl/terraform-provider-doit/tools/linters/clearableattr"
	"github.com/doitintl/terraform-provider-doit/tools/linters/internal/lintertest"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestClearableAttr(t *testing.T) {
	testdata := analysistest.TestData()
	lintertest.RunUniqueLines(t, testdata, clearableattr.Analyzer, "cattest")
}
