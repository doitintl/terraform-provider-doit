package interfacestyle

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestInterfaceStyle(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "interfacetest")
}
