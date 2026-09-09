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
//   - a call passing a literal nil where a backoff-policy factory is expected,
//     which reaches the same fallback despite going through a helper. This keys
//     off the parameter's type, so it covers every such constructor and wrapper
//     rather than a list of names that would go stale.
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

	// backOffType is the result type of a backoff-policy factory —
	// func() backoff.BackOff. Any call passing nil for such a parameter reaches
	// the same production fallback as omitting the field, so the second check
	// keys off the parameter's type rather than a list of helper names: that way
	// it covers newTestRetryClient, newTestRetryAPIClient, and any wrapper added
	// later, without the list going stale.
	backOffType = "BackOff"

	// retryClientHelper is named in diagnostics as the constructor to reach for.
	retryClientHelper = "newTestRetryClient"
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
			checkNilPolicyArg(pass, node)
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

// checkNilPolicyArg reports a call that passes a literal nil where a
// backoff-policy factory is expected.
//
// Keyed off the parameter's type, not the callee's name: newTestRetryClient and
// newTestRetryAPIClient take the policy at different positions, and a future
// wrapper would take it at a third — a name-and-index list would silently go
// stale the moment one was added, which is exactly how the hazard this linter
// exists for reached the tests in the first place.
//
// Only a literal nil is detected. A nil-valued variable passed through would
// need dataflow analysis and is not covered; the testdata records that boundary.
func checkNilPolicyArg(pass *analysis.Pass, call *ast.CallExpr) {
	sig, ok := signatureOf(pass, call)
	if !ok {
		return
	}

	for i, arg := range call.Args {
		if !isNilLiteral(pass, arg) {
			continue
		}
		if !isBackOffFactory(paramTypeAt(sig, i)) {
			continue
		}
		pass.Reportf(arg.Pos(),
			"nil backoff policy reaches the same production 2s-to-60s fallback "+
				"as omitting %s; pass constantBackOff(d), as %s expects",
			backoffField, retryClientHelper)
	}
}

// signatureOf returns the callee's signature, skipping type conversions and
// builtins, for which there is no signature to inspect.
func signatureOf(pass *analysis.Pass, call *ast.CallExpr) (*types.Signature, bool) {
	t := pass.TypesInfo.TypeOf(call.Fun)
	if t == nil {
		return nil, false
	}
	sig, ok := types.Unalias(t).Underlying().(*types.Signature)
	return sig, ok
}

// paramTypeAt returns the declared type of the parameter receiving argument i,
// accounting for a variadic tail. It returns nil when there is no such
// parameter, which isBackOffFactory treats as no match.
func paramTypeAt(sig *types.Signature, i int) types.Type {
	params := sig.Params()
	if params == nil || params.Len() == 0 {
		return nil
	}

	last := params.Len() - 1
	if i < last {
		return params.At(i).Type()
	}
	if !sig.Variadic() {
		if i > last {
			return nil
		}
		return params.At(last).Type()
	}
	// The variadic parameter is a slice; an argument fills its element type.
	slice, ok := params.At(last).Type().(*types.Slice)
	if !ok {
		return nil
	}
	return slice.Elem()
}

// isNilLiteral reports whether expr is the predeclared nil, rather than an
// identifier that happens to be spelled that way.
func isNilLiteral(pass *analysis.Pass, expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	if !ok || ident.Name != "nil" {
		return false
	}
	_, isNil := pass.TypesInfo.ObjectOf(ident).(*types.Nil)
	return isNil
}

// isBackOffFactory reports whether t is func() BackOff — the shape of the
// policy factory DCIRetryClient stores. A named function type is unwrapped, so
// a future alias for the factory is still recognized.
func isBackOffFactory(t types.Type) bool {
	if t == nil {
		return false
	}
	sig, ok := types.Unalias(t).Underlying().(*types.Signature)
	if !ok {
		return false
	}
	if sig.Params().Len() != 0 || sig.Results().Len() != 1 {
		return false
	}

	result := types.Unalias(sig.Results().At(0).Type())
	if ptr, ok := result.(*types.Pointer); ok {
		result = ptr.Elem()
	}
	named, ok := result.(*types.Named)
	if !ok {
		return false
	}
	return named.Obj().Name() == backOffType
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
