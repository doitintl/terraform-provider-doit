package usestatefunknown_test

import (
	"testing"

	"github.com/doitintl/terraform-provider-doit/tools/linters/usestatefunknown"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestUseStateForUnknown(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, usestatefunknown.Analyzer, "usftest")
}
