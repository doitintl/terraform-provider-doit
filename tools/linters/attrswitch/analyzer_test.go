package attrswitch_test

import (
	"testing"

	"github.com/doitintl/terraform-provider-doit/tools/linters/attrswitch"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAttrSwitch(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, attrswitch.Analyzer, "attrswitchtest")
}
