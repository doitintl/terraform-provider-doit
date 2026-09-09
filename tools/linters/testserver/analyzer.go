// Package testserver enforces how tests construct and clean up an
// httptest.Server.
//
// Two checks:
//
//   - A redundant cleanup call. httptest.NewTestServer registers its own
//     t.Cleanup(s.Close), so an explicit "defer s.Close()" or
//     "t.Cleanup(s.Close)" is dead code. Harmless in an ordinary test, but
//     inside a testing/synctest bubble it is not: deferred calls run before
//     registered cleanups, so the explicit Close jumps ahead of the
//     response-body closes, blocks on connections still in flight, and can
//     deadlock the bubble — which surfaces as a package-wide test timeout
//     rather than a failure of the test at fault.
//
//   - httptest.NewServer, NewTLSServer, or NewUnstartedServer. These bind a
//     real TCP port, which the in-memory NewTestServer avoids, and a real
//     listener is not durably blocking, so a synctest bubble built on one never
//     advances its clock. They also do not register cleanup, which is what
//     makes the first check correct: it can only assume a Close is redundant
//     because every server here comes from NewTestServer. Banning the others
//     keeps that invariant total rather than merely true today.
//
// Both checks apply to test files only — NewTestServer takes a testing.TB, so
// neither concern exists outside one.
//
// The cleanup check is keyed off the receiver's type, so it does not know which
// constructor produced the server. Its diagnostic therefore states the rule —
// a server from NewTestServer needs no explicit Close — rather than asserting
// that this particular server's cleanup is already registered, which would be
// false for one built by a banned constructor. Where both checks fire on the
// same variable they are meant to be read together: switch the constructor, and
// the close becomes unnecessary. Tracking provenance to suppress one of them
// would be precise but pointless, since the state where it matters cannot pass
// lint in the first place — the constructor is rejected too.
//
// The redundant-cleanup check deliberately does not flag an immediate
// "s.Close()". A test may legitimately close a server early to assert what a
// client does against a refused connection; only a deferred or registered close
// is redundant.
//
// Note this linter is meaningless if golangci-lint's run.tests is ever
// disabled, and would then fail silently rather than loudly.
package testserver

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const (
	// httptestPkg is matched by full package path, not by type name. This
	// deviates from the sibling retrypolicy linter, which matches
	// DCIRetryClient by name because that name is distinctive; httptest's type
	// is named plainly "Server", so a name-only match would fire on any Server
	// type in any test file.
	httptestPkg = "net/http/httptest"

	// serverType is the type NewTestServer returns, and whose cleanup it
	// registers.
	serverType = "Server"

	// closeMethod is the call NewTestServer makes unnecessary.
	closeMethod = "Close"

	// cleanupMethod registers a function to run at test end. NewTestServer
	// calls it with Close already, so passing Close to it again adds nothing.
	cleanupMethod = "Cleanup"

	// preferredCtor is the constructor the banned ones should become.
	preferredCtor = "NewTestServer"
)

// bannedCtors are the httptest constructors that bind a real TCP port and
// register no cleanup.
var bannedCtors = map[string]bool{
	"NewServer":          true,
	"NewTLSServer":       true,
	"NewUnstartedServer": true,
}

// Analyzer is the go/analysis Analyzer for testserver.
var Analyzer = &analysis.Analyzer{
	Name:     "testserver",
	Doc:      "Enforces httptest.NewTestServer and flags its redundant cleanup calls in tests.",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func run(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{
		(*ast.DeferStmt)(nil),
		(*ast.CallExpr)(nil),
	}
	insp.Preorder(nodeFilter, func(n ast.Node) {
		if !strings.HasSuffix(pass.Fset.Position(n.Pos()).Filename, "_test.go") {
			return
		}

		switch node := n.(type) {
		case *ast.DeferStmt:
			checkDeferredClose(pass, node)
		case *ast.CallExpr:
			checkRegisteredClose(pass, node)
			checkBannedCtor(pass, node)
		}
	})

	return nil, nil
}

// checkDeferredClose reports "defer server.Close()".
//
// Matched as a DeferStmt rather than as the CallExpr inside it: Preorder visits
// that CallExpr independently, but from there the deferredness is invisible,
// and an immediate Close must not be flagged.
func checkDeferredClose(pass *analysis.Pass, stmt *ast.DeferStmt) {
	sel, ok := stmt.Call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	if !isServerClose(pass, sel) {
		return
	}

	pass.Reportf(stmt.Pos(),
		"an httptest.Server from %s needs no explicit %s — it registers its own "+
			"cleanup — and a deferred close runs before the registered cleanups, "+
			"which inside a synctest bubble can block on connections still in flight",
		preferredCtor, closeMethod)
}

// checkRegisteredClose reports "t.Cleanup(server.Close)".
//
// The argument is a method value — a bare SelectorExpr, not a call.
func checkRegisteredClose(pass *analysis.Pass, call *ast.CallExpr) {
	fun, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || fun.Sel.Name != cleanupMethod {
		return
	}

	for _, arg := range call.Args {
		sel, ok := arg.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		if !isServerClose(pass, sel) {
			continue
		}
		pass.Reportf(arg.Pos(),
			"an httptest.Server from %s needs no explicit %s — it registers its "+
				"own cleanup",
			preferredCtor, closeMethod)
	}
}

// checkBannedCtor reports httptest.NewServer and friends.
func checkBannedCtor(pass *analysis.Pass, call *ast.CallExpr) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || !bannedCtors[sel.Sel.Name] {
		return
	}
	// A constructor is a qualified identifier, so resolve the package directly
	// rather than through Selections, which holds only field and method
	// selections.
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return
	}
	pkgName, ok := pass.TypesInfo.ObjectOf(ident).(*types.PkgName)
	if !ok || pkgName.Imported().Path() != httptestPkg {
		return
	}

	pass.Reportf(call.Pos(),
		"use httptest.%s instead of httptest.%s: it routes in memory with no TCP "+
			"listener, registers its own cleanup, and is durably blocking, which a "+
			"real listener is not — a synctest bubble built on one never advances "+
			"its clock",
		preferredCtor, sel.Sel.Name)
}

// isServerClose reports whether sel selects Close on an *httptest.Server.
//
// The receiver's declaring type is used, not the expression's type: this repo
// embeds *httptest.Server in a test helper struct, so an expression can select
// a promoted Close while typing as the outer struct. Selections gives the
// method actually resolved, and its receiver is the declaring type.
func isServerClose(pass *analysis.Pass, sel *ast.SelectorExpr) bool {
	if sel.Sel.Name != closeMethod {
		return false
	}

	selection, ok := pass.TypesInfo.Selections[sel]
	if !ok || selection.Kind() != types.MethodVal {
		return false
	}
	fn, ok := selection.Obj().(*types.Func)
	if !ok {
		return false
	}
	recv := fn.Signature().Recv()
	if recv == nil {
		return false
	}

	t := recv.Type()
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	named, ok := types.Unalias(t).(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Pkg() != nil && obj.Pkg().Path() == httptestPkg && obj.Name() == serverType
}
