package overlayinvariant_test

import (
	"testing"

	"github.com/doitintl/terraform-provider-doit/tools/linters/internal/lintertest"
	"github.com/doitintl/terraform-provider-doit/tools/linters/overlayinvariant"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestOverlayInvariant(t *testing.T) {
	testdata := analysistest.TestData()
	lintertest.RunUniqueLines(t, testdata, overlayinvariant.Analyzer, "invarianttest")
}
