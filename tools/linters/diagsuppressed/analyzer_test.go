package diagsuppressed_test

import (
	"testing"

	"github.com/doitintl/terraform-provider-doit/tools/linters/diagsuppressed"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestDiagSuppressed(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, diagsuppressed.Analyzer, "diagtest")
}
