package volatileplan_test

import (
	"testing"

	"github.com/doitintl/terraform-provider-doit/tools/linters/internal/lintertest"
	"github.com/doitintl/terraform-provider-doit/tools/linters/volatileplan"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestVolatilePlan(t *testing.T) {
	testdata := analysistest.TestData()
	lintertest.RunUniqueLines(t, testdata, volatileplan.Analyzer, "volatiletest")
}
