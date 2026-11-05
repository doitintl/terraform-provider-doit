package retrypolicy

import (
	"testing"

	"github.com/doitintl/terraform-provider-doit/tools/linters/internal/lintertest"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestRetryPolicy(t *testing.T) {
	testdata := analysistest.TestData()
	lintertest.RunUniqueLines(t, testdata, Analyzer, "retrypolicy")
}
