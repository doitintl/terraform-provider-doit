package requestguard_test

import (
	"testing"

	"github.com/doitintl/terraform-provider-doit/tools/linters/requestguard"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestRequestGuard(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, requestguard.Analyzer, "guardtest")
}
