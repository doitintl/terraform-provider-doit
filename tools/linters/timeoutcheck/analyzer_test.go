package timeoutcheck

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestTimeoutCheck(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "timeouttest")
}
