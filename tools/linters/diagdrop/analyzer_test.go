package diagdrop_test

import (
	"testing"

	"github.com/doitintl/terraform-provider-doit/tools/linters/diagdrop"
	"github.com/doitintl/terraform-provider-doit/tools/linters/internal/lintertest"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestDiagDrop(t *testing.T) {
	testdata := analysistest.TestData()
	lintertest.RunUniqueLines(t, testdata, diagdrop.Analyzer, "diagdroptest")
}
