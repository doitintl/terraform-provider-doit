package setnestedplan_test

import (
	"testing"

	"github.com/doitintl/terraform-provider-doit/tools/linters/internal/lintertest"
	"github.com/doitintl/terraform-provider-doit/tools/linters/setnestedplan"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestSetNestedPlan(t *testing.T) {
	testdata := analysistest.TestData()
	lintertest.RunUniqueLines(t, testdata, setnestedplan.Analyzer, "setnestedtest")
}
