package setnestedplan_test

import (
	"testing"

	"github.com/doitintl/terraform-provider-doit/tools/linters/setnestedplan"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestSetNestedPlan(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, setnestedplan.Analyzer, "setnestedtest")
}
