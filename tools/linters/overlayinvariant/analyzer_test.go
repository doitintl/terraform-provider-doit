package overlayinvariant_test

import (
	"testing"

	"github.com/doitintl/terraform-provider-doit/tools/linters/overlayinvariant"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestOverlayInvariant(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, overlayinvariant.Analyzer, "invarianttest")
}
