package crudnaming

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestCrudNaming(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "crudtest")
}
