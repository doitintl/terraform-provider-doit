package validatordefer_test

import (
	"testing"

	"github.com/doitintl/terraform-provider-doit/tools/linters/validatordefer"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestValidatorDefer(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, validatordefer.Analyzer, "validatordefer_test")
}
