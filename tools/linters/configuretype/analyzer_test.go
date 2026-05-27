package configuretype

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestConfigureType(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer,
		"configtest/good_resource",
		"configtest/bad_resource",
		"configtest/good_data_source",
	)
}
