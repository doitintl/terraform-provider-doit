package requestguard_test

import (
	"testing"

	"github.com/doitintl/terraform-provider-doit/tools/linters/internal/lintertest"
	"github.com/doitintl/terraform-provider-doit/tools/linters/requestguard"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestRequestGuard(t *testing.T) {
	testdata := analysistest.TestData()
	lintertest.RunUniqueLines(t, testdata, requestguard.Analyzer, "guardtest")
}
