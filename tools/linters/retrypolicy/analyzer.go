// Package retrypolicy requires a DCIRetryClient built in a test to name its
// backoff policy.
//
// DCIRetryClient.Do falls back to the production policy — newRetryBackOff(), 2s
// doubling to 60s — when newBackOff is nil, and it runs with
// backoff.WithMaxElapsedTime(0), so the request context is the only thing that
// can end the retry loop. A test that leaves newBackOff nil and passes an
// unbounded context therefore retries forever the first time it meets a 429,
// 502, 503, or 504. Because the unit suite runs under a single -timeout, that
// surfaces as a package-wide "panic: test timed out" — taking every other test
// in the package down with it — rather than as a failure of the test at fault.
//
// The nil fallback is a production affordance, not a defect:
// newClientWithHTTPClient, the constructor NewClient itself delegates to, also
// leaves the field nil, which is how production picks up the real policy
// without restating it. So this linter checks only test files.
//
// Two checks:
//
//   - a DCIRetryClient composite literal that omits newBackOff;
//   - a newTestRetryClient call whose backoff argument is the nil literal,
//     which reaches the same fallback despite going through the helper.
//
// It deliberately does not flag:
//
//   - literals outside _test.go files — the production constructor is one;
//   - tests that obtain a client from newClientWithHTTPClient or NewClient.
//     Those inherit the production nil policy on purpose: exercising the
//     production constructor is the point, and injecting a policy would make
//     the assertion about something else. Bounding such a test's context is the
//     complementary defence, which this linter does not check;
//   - "newBackOff: nil" written out. Naming the field is the rule, and an
//     explicit nil is a greppable, reviewable statement that the test wants the
//     production policy — the intended escape hatch, in place of a //nolint.
//
// Known false positive: a literal followed by a separate "c.newBackOff = ..."
// assignment. The fix is to move the field into the literal, which is what this
// repo's structliteral and newexpr linters push toward anyway.
//
// Note this linter is meaningless if golangci-lint's run.tests is ever
// disabled, and would then fail silently rather than loudly.
package retrypolicy

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const (
	// retryClientType is matched by name rather than by full package path so the
	// analyzer's own testdata, which lives in its own module path, exercises the
	// same code path as the real package. The name is distinctive enough that a
	// collision is not a practical concern.
	retryClientType = "DCIRetryClient"

	// backoffField is the field whose absence selects the production policy.
	backoffField = "newBackOff"

	// retryClientHelper is the test helper that takes a policy as its third
	// argument. Passing nil there reaches the same fallback as omitting the
	// field, so going through the helper is not on its own sufficient.
	retryClientHelper = "newTestRetryClient"

	// helperBackoffArg is the position of the helper's backoff argument.
	helperBackoffArg = 2
)

// Analyzer is the go/analysis Analyzer for retrypolicy.
var Analyzer = &analysis.Analyzer{
	Name:     "retrypolicy",
	Doc:      "Requires a DCIRetryClient built in a test to name its backoff policy.",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func run(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{
		(*ast.CompositeLit)(nil),
		(*ast.CallExpr)(nil),
	}
	insp.Preorder(nodeFilter, func(n ast.Node) {
		// Only test files. The production constructor builds a nil-policy client
		// deliberately, and config cannot express "test files only" — the
		// .golangci.yml path rules are an exclusion list.
		if !strings.HasSuffix(pass.Fset.Position(n.Pos()).Filename, "_test.go") {
			return
		}

		switch node := n.(type) {
		case *ast.CompositeLit:
			checkLiteral(pass, node)
		case *ast.CallExpr:
			checkHelperCall(pass, node)
		}
	})

	return nil, nil
}

// checkLiteral reports a DCIRetryClient literal that does not set newBackOff.
func checkLiteral(pass *analysis.Pass, lit *ast.CompositeLit) {
	// TypeOf on the literal, not on lit.Type: an elided nested literal such as
	// []*DCIRetryClient{{client: c}} has a nil Type, and matching on it would
	// silently miss the site. &T{} needs no special handling either — it is a
	// UnaryExpr wrapping this CompositeLit, which Preorder visits.
	if !isRetryClient(pass.TypesInfo.TypeOf(lit)) {
		return
	}

	// A non-empty positional literal must supply every field, so newBackOff is
	// necessarily set. An empty literal omits it.
	if len(lit.Elts) > 0 {
		if _, keyed := lit.Elts[0].(*ast.KeyValueExpr); !keyed {
			return
		}
		for _, elt := range lit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			if key, ok := kv.Key.(*ast.Ident); ok && key.Name == backoffField {
				return
			}
		}
	}

	pass.Reportf(lit.Pos(),
		"%s literal in a test omits %s, so Do falls back to the production "+
			"2s-to-60s policy with no elapsed-time limit; use "+
			"%s(server, timeout, constantBackOff(d)), or set %s explicitly "+
			"(nil for the production policy)",
		retryClientType, backoffField, retryClientHelper, backoffField)
}

// checkHelperCall reports newTestRetryClient(_, _, nil), which reaches the
// production fallback despite naming the field inside the helper.
func checkHelperCall(pass *analysis.Pass, call *ast.CallExpr) {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok || ident.Name != retryClientHelper {
		return
	}
	if len(call.Args) <= helperBackoffArg {
		return
	}

	arg, ok := call.Args[helperBackoffArg].(*ast.Ident)
	if !ok || arg.Name != "nil" {
		return
	}
	// Confirm it is the predeclared nil and not a shadowing identifier.
	if _, isNil := pass.TypesInfo.ObjectOf(arg).(*types.Nil); !isNil {
		return
	}

	pass.Reportf(arg.Pos(),
		"%s is passed a nil backoff policy, which reaches the same production "+
			"2s-to-60s fallback as omitting %s; pass constantBackOff(d)",
		retryClientHelper, backoffField)
}

// isRetryClient reports whether t is DCIRetryClient or a pointer to it.
func isRetryClient(t types.Type) bool {
	if t == nil {
		return false
	}
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	return named.Obj().Name() == retryClientType
}
