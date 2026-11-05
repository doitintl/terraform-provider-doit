package diagsuppressed_test

import (
	"testing"

	"github.com/doitintl/terraform-provider-doit/tools/linters/diagsuppressed"
	"github.com/doitintl/terraform-provider-doit/tools/linters/internal/lintertest"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestDiagSuppressed(t *testing.T) {
	testdata := analysistest.TestData()
	lintertest.RunUniqueLines(t, testdata, diagsuppressed.Analyzer, "diagtest")
}
