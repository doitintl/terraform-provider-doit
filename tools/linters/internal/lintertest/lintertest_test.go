package lintertest_test

import (
	"go/token"
	"testing"

	"github.com/doitintl/terraform-provider-doit/tools/linters/internal/lintertest"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestRequireUniqueLines(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	file := fset.AddFile("a.go", -1, 100)
	file.SetLines([]int{0, 10, 20})

	result := func(diagnostics ...analysis.Diagnostic) *analysistest.Result {
		return &analysistest.Result{
			Pass:        &analysis.Pass{Fset: fset},
			Diagnostics: diagnostics,
		}
	}
	at := func(offset int, message string) analysis.Diagnostic {
		return analysis.Diagnostic{Pos: file.Pos(offset), Message: message}
	}

	tests := []struct {
		name    string
		results []*analysistest.Result
		failed  bool
	}{
		{"distinct lines", []*analysistest.Result{result(at(1, "a"), at(12, "b"))}, false},
		{"same line", []*analysistest.Result{result(at(1, "a"), at(5, "b"))}, true},
		{"same line across results", []*analysistest.Result{result(at(1, "a")), result(at(5, "b"))}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// The helper only calls Errorf, so a bare T records the failure
			// without failing this test.
			recorder := &testing.T{}
			lintertest.RequireUniqueLines(recorder, tt.results)

			if recorder.Failed() != tt.failed {
				t.Errorf("failed = %v, want %v", recorder.Failed(), tt.failed)
			}
		})
	}
}
