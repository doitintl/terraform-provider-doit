package unknownguard

import (
	"testing"

	"github.com/doitintl/terraform-provider-doit/tools/linters/schemaparser"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestUnknownGuard(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "unknownguardtest")
	_ = schemaparser.Analyzer // ensure dependency is used
}
