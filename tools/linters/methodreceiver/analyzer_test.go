package methodreceiver_test

import (
	"testing"

	"github.com/doitintl/terraform-provider-doit/tools/linters/methodreceiver"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestMethodReceiver(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, methodreceiver.Analyzer, "receivertest")
}
