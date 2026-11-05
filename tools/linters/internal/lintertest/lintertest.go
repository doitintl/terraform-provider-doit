// Package lintertest holds shared helpers for analyzer tests.
package lintertest

import (
	"fmt"
	"sort"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
)

// RunUniqueLines runs analysistest.Run and additionally fails the test when two
// diagnostics share a file and line. golangci-lint keeps one issue per line, so
// a second finding on the same line is silently hidden in real runs even though
// analysistest, which matches `// want` comments per line, would accept it.
// Use it for every analyzer that reports more than one kind of finding.
func RunUniqueLines(
	t *testing.T,
	dir string,
	analyzer *analysis.Analyzer,
	patterns ...string,
) []*analysistest.Result {
	t.Helper()

	results := analysistest.Run(t, dir, analyzer, patterns...)
	RequireUniqueLines(t, results)

	return results
}

// RequireUniqueLines fails the test for each file and line that has more than
// one diagnostic across results.
func RequireUniqueLines(t *testing.T, results []*analysistest.Result) {
	t.Helper()

	messagesByLine := map[string][]string{}
	for _, result := range results {
		for _, diagnostic := range result.Diagnostics {
			position := result.Pass.Fset.Position(diagnostic.Pos)
			key := fmt.Sprintf("%s:%d", position.Filename, position.Line)
			messagesByLine[key] = append(messagesByLine[key], diagnostic.Message)
		}
	}

	lines := make([]string, 0, len(messagesByLine))
	for line := range messagesByLine {
		lines = append(lines, line)
	}
	sort.Strings(lines)

	for _, line := range lines {
		messages := messagesByLine[line]
		if len(messages) > 1 {
			t.Errorf("%d diagnostics on one line; golangci-lint would keep only one: %s\n  %v",
				len(messages), line, messages)
		}
	}
}
