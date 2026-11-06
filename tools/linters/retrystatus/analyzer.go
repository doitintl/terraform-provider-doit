// Package retrystatus flags branches on HTTP statuses that DCIRetryClient never
// hands to resource code.
//
// DCIRetryClient.Do returns a response only for 2xx, 3xx and the 4xx statuses it
// passes through on purpose (404, 412, 425). Every other status becomes an
// error: 429/502/503/504 after the retries are exhausted, the rest as
// backoff.Permanent. A branch such as
//
//	if resp.StatusCode() == 403 { ... parse resp.JSON403 ... }
//
// is therefore dead code, and a unit test that exercises the helper in
// isolation will not notice. The status and body of such a response are only
// available in the error message, which already includes both.
//
// The pass-through set is not restated here. The analyzer reads it from the
// package under analysis: the case clause of the status switch in
// DCIRetryClient.Do that ends in "return resp, nil". Editing that switch is
// therefore the only way to change what resource code may branch on, and the
// two cannot drift. Packages without a DCIRetryClient.Do are not checked.
//
// Two checks, both limited to statuses >= 400 that are not passed through:
//
//   - a StatusCode() call compared against such a constant, in a comparison or
//     a switch case. Range checks such as "StatusCode() >= 400" are not flagged;
//     they are the generic way to detect an error and are valid;
//   - a read of a generated response's JSON<status> or
//     ApplicationproblemJSON<status> field, or of its GetJSON<status> getter,
//     for such a status. The 4xx and 5xx
//     catch-all fields are flagged only when no status in that class is passed
//     through.
//
// Test files are excluded in .golangci.yml: tests that build their own
// transport can legitimately see any status.
package retrystatus

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"regexp"
	"strconv"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const (
	// retryClientType is matched by name rather than full package path so the
	// analyzer's testdata, which has its own module path, takes the same code
	// path as the real package.
	retryClientType = "DCIRetryClient"

	// minErrorStatus is the lowest status the retry client can turn into an
	// error. Everything below it is returned by the default branch.
	minErrorStatus = 400
)

// bodyFieldPattern matches the generated typed-body fields, for example JSON403
// and ApplicationproblemJSON404, and the class forms JSON4xx and JSON5xx.
//
// The generated getters GetJSON403 and friends return the same field, so the
// pattern accepts an optional Get prefix.
var bodyFieldPattern = regexp.MustCompile(`^(?:Get)?(?:Applicationproblem)?JSON([0-9]{3}|[45]xx)$`)

// Analyzer is the go/analysis Analyzer for retrystatus.
var Analyzer = &analysis.Analyzer{
	Name:     "retrystatus",
	Doc:      "Flags branches on statuses that DCIRetryClient turns into errors, so resource code never sees them.",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func run(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	passed, ok := passThroughStatuses(pass, insp)
	if !ok {
		return nil, nil
	}

	insp.Preorder([]ast.Node{
		(*ast.BinaryExpr)(nil),
		(*ast.SwitchStmt)(nil),
		(*ast.SelectorExpr)(nil),
	}, func(n ast.Node) {
		switch n := n.(type) {
		case *ast.BinaryExpr:
			checkComparison(pass, passed, n)
		case *ast.SwitchStmt:
			checkSwitch(pass, passed, n)
		case *ast.SelectorExpr:
			checkBodyField(pass, passed, n)
		}
	})

	return nil, nil
}

// passThroughStatuses reads the statuses DCIRetryClient.Do returns as a
// response rather than an error. The second result is false when the package
// does not define the client.
func passThroughStatuses(pass *analysis.Pass, insp *inspector.Inspector) (map[int]bool, bool) {
	var passed map[int]bool

	insp.Preorder([]ast.Node{(*ast.FuncDecl)(nil)}, func(n ast.Node) {
		fn := n.(*ast.FuncDecl)
		if fn.Name.Name != "Do" || fn.Body == nil || receiverTypeName(fn) != retryClientType {
			return
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			clause, ok := n.(*ast.CaseClause)
			if !ok || !returnsResponse(clause) {
				return true
			}
			if passed == nil {
				passed = map[int]bool{}
			}
			for _, expr := range clause.List {
				if status, ok := statusConstant(pass, expr); ok {
					passed[status] = true
				}
			}
			return true
		})
	})

	return passed, passed != nil
}

// receiverTypeName returns the receiver's type name without pointer or
// parentheses, or "" for a plain function.
func receiverTypeName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}
	expr := fn.Recv.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

// returnsResponse reports whether the clause ends by returning a non-nil
// response and a nil error, which is how Do hands a response to its caller.
func returnsResponse(clause *ast.CaseClause) bool {
	if len(clause.Body) == 0 {
		return false
	}
	ret, ok := clause.Body[len(clause.Body)-1].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 2 {
		return false
	}
	return !isNil(ret.Results[0]) && isNil(ret.Results[1])
}

func isNil(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == "nil"
}

// statusConstant returns the integer value of expr when it is a constant, so
// http.StatusNotFound and 404 are the same status.
func statusConstant(pass *analysis.Pass, expr ast.Expr) (int, bool) {
	tv, ok := pass.TypesInfo.Types[expr]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.Int {
		return 0, false
	}
	value, exact := constant.Int64Val(tv.Value)
	if !exact {
		return 0, false
	}
	return int(value), true
}

// isStatusCodeCall reports whether expr is a call of a method named StatusCode,
// which is how the generated clients expose a response's status.
func isStatusCodeCall(pass *analysis.Pass, expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "StatusCode" && isGeneratedResponse(pass.TypesInfo.TypeOf(sel.X))
}

// isGeneratedResponse reports whether t is a generated client response: a
// struct holding both the raw Body bytes and the HTTPResponse. Matching the
// shape rather than a package or type name covers every generated XxxResp and
// leaves unrelated types with a StatusCode method or a JSON403 field alone.
func isGeneratedResponse(t types.Type) bool {
	if t == nil {
		return false
	}
	if ptr, ok := t.Underlying().(*types.Pointer); ok {
		t = ptr.Elem()
	}
	st, ok := t.Underlying().(*types.Struct)
	if !ok {
		return false
	}
	var hasBody, hasHTTP bool
	for field := range st.Fields() {
		switch field.Name() {
		case "Body":
			if slice, ok := field.Type().(*types.Slice); ok {
				basic, ok := slice.Elem().(*types.Basic)
				hasBody = ok && basic.Kind() == types.Byte
			}
		case "HTTPResponse":
			if ptr, ok := field.Type().(*types.Pointer); ok {
				if named, ok := ptr.Elem().(*types.Named); ok {
					hasHTTP = named.Obj().Name() == "Response"
				}
			}
		}
	}
	return hasBody && hasHTTP
}

func unreachable(passed map[int]bool, status int) bool {
	return status >= minErrorStatus && !passed[status]
}

func report(pass *analysis.Pass, pos token.Pos, what string) {
	pass.Reportf(pos,
		"%s is never seen by resource code: DCIRetryClient returns that status as an error "+
			"(the message already includes the status and body). Remove the branch, or pass the "+
			"status through in DCIRetryClient.Do if the caller needs it", what)
}

func checkComparison(pass *analysis.Pass, passed map[int]bool, expr *ast.BinaryExpr) {
	if expr.Op != token.EQL && expr.Op != token.NEQ {
		return
	}
	for _, pair := range [][2]ast.Expr{{expr.X, expr.Y}, {expr.Y, expr.X}} {
		if !isStatusCodeCall(pass, pair[0]) {
			continue
		}
		if status, ok := statusConstant(pass, pair[1]); ok && unreachable(passed, status) {
			report(pass, expr.Pos(), "status "+strconv.Itoa(status))
		}
	}
}

func checkSwitch(pass *analysis.Pass, passed map[int]bool, stmt *ast.SwitchStmt) {
	if stmt.Tag == nil || !isStatusCodeCall(pass, stmt.Tag) {
		return
	}
	for _, stmt := range stmt.Body.List {
		clause, ok := stmt.(*ast.CaseClause)
		if !ok {
			continue
		}
		for _, expr := range clause.List {
			if status, ok := statusConstant(pass, expr); ok && unreachable(passed, status) {
				report(pass, expr.Pos(), "status "+strconv.Itoa(status))
			}
		}
	}
}

func checkBodyField(pass *analysis.Pass, passed map[int]bool, sel *ast.SelectorExpr) {
	match := bodyFieldPattern.FindStringSubmatch(sel.Sel.Name)
	if match == nil {
		return
	}
	// Only members of a generated response: a package-qualified identifier or
	// an unrelated struct's JSON403 field is not a response body.
	if !isGeneratedResponse(pass.TypesInfo.TypeOf(sel.X)) {
		return
	}
	switch match[1] {
	case "4xx", "5xx":
		if classPassed(passed, int(match[1][0]-'0')*100) {
			return
		}
	default:
		status, _ := strconv.Atoi(match[1])
		if !unreachable(passed, status) {
			return
		}
	}
	report(pass, sel.Sel.Pos(), "field "+sel.Sel.Name)
}

// classPassed reports whether any status in [base, base+100) is passed through.
func classPassed(passed map[int]bool, base int) bool {
	for status := range passed {
		if status >= base && status < base+100 {
			return true
		}
	}
	return false
}
