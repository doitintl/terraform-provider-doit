package configuretype

import (
	"testing"

	"github.com/doitintl/terraform-provider-doit/tools/linters/internal/lintertest"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestConfigureType(t *testing.T) {
	testdata := analysistest.TestData()
	lintertest.RunUniqueLines(t, testdata, Analyzer,
		"configtest/good_resource",
		"configtest/bad_resource",
		"configtest/good_data_source",
	)
}
