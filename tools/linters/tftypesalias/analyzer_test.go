package tftypesalias_test

import (
	"testing"

	"github.com/doitintl/terraform-provider-doit/tools/linters/tftypesalias"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestTftypesAlias(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, tftypesalias.Analyzer, "tftypesaliastest")
}
