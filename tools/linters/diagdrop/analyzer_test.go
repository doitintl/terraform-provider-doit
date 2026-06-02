package diagdrop_test

import (
	"testing"

	"github.com/doitintl/terraform-provider-doit/tools/linters/diagdrop"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestDiagDrop(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, diagdrop.Analyzer, "diagdroptest")
}
