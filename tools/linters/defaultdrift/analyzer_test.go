package defaultdrift_test

import (
	"testing"

	"github.com/doitintl/terraform-provider-doit/tools/linters/defaultdrift"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestDefaultDrift(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, defaultdrift.Analyzer, "drifttest")
}
