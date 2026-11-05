package validatorshape_test

import (
	"testing"

	"github.com/doitintl/terraform-provider-doit/tools/linters/internal/lintertest"
	"github.com/doitintl/terraform-provider-doit/tools/linters/validatorshape"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestValidatorShape(t *testing.T) {
	testdata := analysistest.TestData()
	lintertest.RunUniqueLines(t, testdata, validatorshape.Analyzer, "validatorshape_test")
}
