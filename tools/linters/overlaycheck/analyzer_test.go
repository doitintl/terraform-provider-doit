package overlaycheck_test

import (
	"testing"

	"github.com/doitintl/terraform-provider-doit/tools/linters/internal/lintertest"
	"github.com/doitintl/terraform-provider-doit/tools/linters/overlaycheck"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestOverlayCheck(t *testing.T) {
	testdata := analysistest.TestData()
	lintertest.RunUniqueLines(t, testdata, overlaycheck.Analyzer, "overlaytest")
}
