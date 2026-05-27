package listnullread_test

import (
	"testing"

	"github.com/doitintl/terraform-provider-doit/tools/linters/listnullread"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestListNullRead(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, listnullread.Analyzer, "listnulltest")
}
