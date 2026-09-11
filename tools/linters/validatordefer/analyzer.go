// Package validatordefer detects Terraform validators that emit diagnostics
// before unknown configuration values have resolved.
package validatordefer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/ctrlflow"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/cfg"
	"golang.org/x/tools/go/types/typeutil"
)

// Analyzer is the go/analysis analyzer for validatordefer.
var Analyzer = &analysis.Analyzer{
	Name: "validatordefer",
	Doc:  "Detects Terraform validator diagnostics that do not defer unknown values.",
	Run:  run,
	Requires: []*analysis.Analyzer{
		inspect.Analyzer,
		ctrlflow.Analyzer,
	},
}

var diagnosticMethods = map[string]bool{
	"AddAttributeError":   true,
	"AddAttributeWarning": true,
	"AddError":            true,
	"AddWarning":          true,
}

type presenceAlias struct {
	value                 string
	unknownMakesAliasTrue bool
}

type functionAnalysis struct {
	pass           *analysis.Pass
	reachable      map[token.Pos]bool
	reported       map[token.Pos]bool
	knownAtIf      map[token.Pos]map[string]bool
	unknownAliases map[string]presenceAlias
}

type analysisContext struct {
	function *types.Func
	knownKey string
}

type flowFacts struct {
	knownAtIf   map[token.Pos]map[string]bool
	knownAtCall map[token.Pos]map[string]bool
}

func run(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	cfgs := pass.ResultOf[ctrlflow.Analyzer].(*ctrlflow.CFGs)

	decls := map[*types.Func]*ast.FuncDecl{}
	var roots []*ast.FuncDecl
	insp.Preorder([]ast.Node{(*ast.FuncDecl)(nil)}, func(node ast.Node) {
		decl := node.(*ast.FuncDecl)
		if decl.Body == nil || decl.Name == nil {
			return
		}
		fn, ok := pass.TypesInfo.Defs[decl.Name].(*types.Func)
		if !ok {
			return
		}
		decls[fn] = decl
		if isValidatorEntryPoint(fn) {
			roots = append(roots, decl)
		}
	})

	seen := map[analysisContext]bool{}
	reported := map[token.Pos]bool{}
	var analyze func(*ast.FuncDecl, map[string]bool)
	analyze = func(decl *ast.FuncDecl, entryKnown map[string]bool) {
		fn, ok := pass.TypesInfo.Defs[decl.Name].(*types.Func)
		if !ok {
			return
		}
		// A helper can be safe from one guarded call site and unsafe from
		// another, so analyze each distinct parameter-fact context. Diagnostics
		// are deduplicated globally and remain reported if any context is unsafe.
		context := analysisContext{function: fn, knownKey: setKey(entryKnown)}
		if seen[context] {
			return
		}
		seen[context] = true

		unknownAliases := collectUnknownAliases(decl.Body)
		flow := collectFlowFacts(decl.Body, unknownAliases, entryKnown)

		state := &functionAnalysis{
			pass:           pass,
			reachable:      reachablePositions(cfgs.FuncDecl(decl)),
			reported:       reported,
			knownAtIf:      flow.knownAtIf,
			unknownAliases: unknownAliases,
		}
		state.checkExplicitUnknownDiagnostics(decl.Body)
		state.checkPresenceAliases(decl.Body)
		state.checkUncertainLoopCounts(decl.Body)

		inspectWithoutFuncLits(decl.Body, func(node ast.Node) {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return
			}
			callerKnown, supported := flow.knownAtCall[call.Pos()]
			if !supported {
				return
			}
			callee := typeutil.StaticCallee(pass.TypesInfo, call)
			if next, ok := decls[callee]; ok {
				analyze(next, mapKnownArguments(call, next, callerKnown))
			}
		})
	}

	for _, root := range roots {
		analyze(root, nil)
	}

	return nil, nil
}

func isValidatorEntryPoint(fn *types.Func) bool {
	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return false
	}
	for index := 0; index < sig.Params().Len(); index++ {
		named := namedType(sig.Params().At(index).Type())
		if named == nil || named.Obj().Pkg() == nil {
			continue
		}
		path := named.Obj().Pkg().Path()
		name := named.Obj().Name()
		if path == "github.com/hashicorp/terraform-plugin-framework/schema/validator" && strings.HasSuffix(name, "Request") {
			return true
		}
		if (path == "github.com/hashicorp/terraform-plugin-framework/resource" ||
			path == "github.com/hashicorp/terraform-plugin-framework/datasource" ||
			path == "github.com/hashicorp/terraform-plugin-framework/provider") && name == "ValidateConfigRequest" {
			return true
		}
	}
	return false
}

func namedType(value types.Type) *types.Named {
	if pointer, ok := value.(*types.Pointer); ok {
		value = pointer.Elem()
	}
	named, _ := value.(*types.Named)
	return named
}

func reachablePositions(graph *cfg.CFG) map[token.Pos]bool {
	if graph == nil {
		return map[token.Pos]bool{}
	}
	positions := map[token.Pos]bool{}
	for _, block := range graph.Blocks {
		if !block.Live {
			continue
		}
		for _, node := range block.Nodes {
			ast.Inspect(node, func(candidate ast.Node) bool {
				if candidate != nil {
					positions[candidate.Pos()] = true
				}
				return true
			})
		}
	}
	return positions
}

func (state *functionAnalysis) checkExplicitUnknownDiagnostics(body *ast.BlockStmt) {
	inspectWithoutFuncLits(body, func(node ast.Node) {
		ifStmt, ok := node.(*ast.IfStmt)
		if !ok {
			return
		}
		unknowns := positiveUnknownValuesWithAliases(ifStmt.Cond, state.unknownAliases)
		unknowns = withoutKnownValues(unknowns, state.knownAtIf[ifStmt.Pos()])
		if len(unknowns) == 0 {
			return
		}
		for _, diagnostic := range diagnosticsIn(ifStmt.Body) {
			state.report(diagnostic, unknowns)
		}
	})
}

func (state *functionAnalysis) checkPresenceAliases(body *ast.BlockStmt) {
	aliases := collectNullPresenceAliases(body)

	inspectWithoutFuncLits(body, func(node ast.Node) {
		ifStmt, ok := node.(*ast.IfStmt)
		if !ok {
			return
		}
		unsafeValues := unsafePresenceValues(ifStmt.Cond, aliases, true)
		unsafeValues = withoutKnownValues(unsafeValues, state.knownAtIf[ifStmt.Pos()])
		if len(unsafeValues) == 0 {
			return
		}
		for _, diagnostic := range diagnosticsIn(ifStmt.Body) {
			state.report(diagnostic, unsafeValues)
		}
	})
}

func collectFlowFacts(body *ast.BlockStmt, aliases map[string]presenceAlias, entryKnown map[string]bool) flowFacts {
	facts := flowFacts{
		knownAtIf:   map[token.Pos]map[string]bool{},
		knownAtCall: map[token.Pos]map[string]bool{},
	}
	recordCalls := func(node ast.Node, known map[string]bool) {
		if node == nil {
			return
		}
		inspectWithoutFuncLits(node, func(candidate ast.Node) {
			if call, ok := candidate.(*ast.CallExpr); ok {
				facts.knownAtCall[call.Pos()] = cloneSet(known)
			}
		})
	}
	var visitBlock func(*ast.BlockStmt, map[string]bool)
	visitBlock = func(block *ast.BlockStmt, incoming map[string]bool) {
		known := cloneSet(incoming)
		for _, statement := range block.List {
			switch statement := statement.(type) {
			case *ast.IfStmt:
				recordCalls(statement.Init, known)
				recordCalls(statement.Cond, known)
				branchKnown := cloneSet(known)
				addValues(branchKnown, definitelyKnownValues(statement.Cond, aliases))
				facts.knownAtIf[statement.Pos()] = branchKnown
				visitBlock(statement.Body, branchKnown)
				if elseBlock, ok := statement.Else.(*ast.BlockStmt); ok {
					visitBlock(elseBlock, known)
				}
				if blockEndsWithExit(statement.Body) {
					addValues(known, positiveUnknownValuesWithAliases(statement.Cond, aliases))
				}
			case *ast.ForStmt:
				recordCalls(statement.Init, known)
				recordCalls(statement.Cond, known)
				recordCalls(statement.Post, known)
				visitBlock(statement.Body, known)
			case *ast.RangeStmt:
				recordCalls(statement.X, known)
				visitBlock(statement.Body, known)
			case *ast.BlockStmt:
				visitBlock(statement, known)
			default:
				recordCalls(statement, known)
			}
		}
	}
	visitBlock(body, entryKnown)
	return facts
}

func collectUnknownAliases(body *ast.BlockStmt) map[string]presenceAlias {
	return collectPredicateAliases(body, unknownPredicateAlias)
}

func collectNullPresenceAliases(body *ast.BlockStmt) map[string]presenceAlias {
	return collectPredicateAliases(body, nullPresenceAlias)
}

func collectPredicateAliases(body *ast.BlockStmt, match func(ast.Expr) (presenceAlias, bool)) map[string]presenceAlias {
	aliases := map[string]presenceAlias{}
	inspectWithoutFuncLits(body, func(node ast.Node) {
		switch statement := node.(type) {
		case *ast.AssignStmt:
			if len(statement.Lhs) != len(statement.Rhs) {
				return
			}
			for index, lhs := range statement.Lhs {
				ident, ok := lhs.(*ast.Ident)
				if !ok {
					continue
				}
				if alias, ok := match(statement.Rhs[index]); ok {
					aliases[ident.Name] = alias
				}
			}
		case *ast.ValueSpec:
			if len(statement.Names) != len(statement.Values) {
				return
			}
			for index, ident := range statement.Names {
				if alias, ok := match(statement.Values[index]); ok {
					aliases[ident.Name] = alias
				}
			}
		}
	})
	return aliases
}

func mapKnownArguments(call *ast.CallExpr, callee *ast.FuncDecl, callerKnown map[string]bool) map[string]bool {
	parameters := parameterNames(callee)
	known := map[string]bool{}
	for index, argument := range call.Args {
		if index >= len(parameters) || parameters[index] == "" {
			continue
		}
		if callerKnown[expressionName(argument)] {
			known[parameters[index]] = true
		}
	}
	return known
}

func parameterNames(function *ast.FuncDecl) []string {
	if function.Type == nil || function.Type.Params == nil {
		return nil
	}
	var names []string
	for _, field := range function.Type.Params.List {
		if len(field.Names) == 0 {
			names = append(names, "")
			continue
		}
		for _, name := range field.Names {
			names = append(names, name.Name)
		}
	}
	return names
}

func setKey(values map[string]bool) string {
	keys := make([]string, 0, len(values))
	for value := range values {
		keys = append(keys, value)
	}
	sort.Strings(keys)
	return strings.Join(keys, "\x00")
}

func definitelyKnownValues(expression ast.Expr, aliases map[string]presenceAlias) []string {
	switch expression := expression.(type) {
	case *ast.ParenExpr:
		return definitelyKnownValues(expression.X, aliases)
	case *ast.BinaryExpr:
		if expression.Op != token.LAND {
			return nil
		}
		return uniqueSorted(append(
			definitelyKnownValues(expression.X, aliases),
			definitelyKnownValues(expression.Y, aliases)...,
		))
	case *ast.UnaryExpr:
		if expression.Op != token.NOT {
			return nil
		}
		call, ok := expression.X.(*ast.CallExpr)
		if !ok || len(call.Args) != 0 {
			return nil
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if ok && selector.Sel.Name == "IsUnknown" {
			return []string{expressionName(selector.X)}
		}
	case *ast.Ident:
		if alias, ok := aliases[expression.Name]; ok && !alias.unknownMakesAliasTrue {
			return []string{alias.value}
		}
	}
	return nil
}

func blockEndsWithExit(block *ast.BlockStmt) bool {
	if block == nil || len(block.List) == 0 {
		return false
	}
	switch statement := block.List[len(block.List)-1].(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		return statement.Tok == token.CONTINUE
	default:
		return false
	}
}

func cloneSet(values map[string]bool) map[string]bool {
	result := map[string]bool{}
	for value := range values {
		result[value] = true
	}
	return result
}

func addValues(destination map[string]bool, values []string) {
	for _, value := range values {
		destination[value] = true
	}
}

func withoutKnownValues(values []string, known map[string]bool) []string {
	result := values[:0]
	for _, value := range values {
		if !known[value] {
			result = append(result, value)
		}
	}
	return result
}

func (state *functionAnalysis) checkUncertainLoopCounts(body *ast.BlockStmt) {
	var visitBlock func(*ast.BlockStmt)
	visitBlock = func(block *ast.BlockStmt) {
		for index, statement := range block.List {
			rangeStmt, ok := statement.(*ast.RangeStmt)
			if ok {
				branches := unknownContinueBranches(rangeStmt.Body)
				if len(branches) > 0 {
					for _, following := range block.List[index+1:] {
						ifStmt, ok := following.(*ast.IfStmt)
						if !ok {
							break
						}
						minimumCounts := minimumCountNames(ifStmt.Cond)
						if len(minimumCounts) == 0 {
							continue
						}
						var unsafeValues []string
						for _, branch := range branches {
							if !intersects(branch.incremented, minimumCounts) {
								unsafeValues = append(unsafeValues, branch.values...)
							}
						}
						if len(unsafeValues) == 0 {
							continue
						}
						for _, diagnostic := range diagnosticsIn(ifStmt.Body) {
							state.report(diagnostic, unsafeValues)
						}
					}
				}
			}

			switch statement := statement.(type) {
			case *ast.BlockStmt:
				visitBlock(statement)
			case *ast.IfStmt:
				visitBlock(statement.Body)
				if elseBlock, ok := statement.Else.(*ast.BlockStmt); ok {
					visitBlock(elseBlock)
				}
			case *ast.ForStmt:
				visitBlock(statement.Body)
			case *ast.RangeStmt:
				visitBlock(statement.Body)
			case *ast.SwitchStmt:
				for _, clause := range statement.Body.List {
					caseClause, ok := clause.(*ast.CaseClause)
					if ok {
						visitBlock(&ast.BlockStmt{List: caseClause.Body})
					}
				}
			}
		}
	}
	visitBlock(body)
}

func (state *functionAnalysis) report(call *ast.CallExpr, values []string) {
	if state.reported[call.Pos()] || (state.reachable != nil && !state.reachable[call.Pos()]) {
		return
	}
	values = uniqueSorted(values)
	if len(values) == 0 {
		return
	}
	state.reported[call.Pos()] = true
	state.pass.Reportf(call.Pos(), "validator diagnostic may be emitted while %s is unknown; defer validation until the value is known", strings.Join(values, ", "))
}

func positiveUnknownValues(expression ast.Expr) []string {
	return positiveUnknownValuesWithAliases(expression, nil)
}

func positiveUnknownValuesWithAliases(expression ast.Expr, aliases map[string]presenceAlias) []string {
	var values []string
	var inspectExpression func(ast.Expr, bool)
	inspectExpression = func(expr ast.Expr, positive bool) {
		switch expr := expr.(type) {
		case *ast.ParenExpr:
			inspectExpression(expr.X, positive)
		case *ast.UnaryExpr:
			if expr.Op == token.NOT {
				inspectExpression(expr.X, !positive)
			}
		case *ast.BinaryExpr:
			if expr.Op == token.LAND || expr.Op == token.LOR {
				inspectExpression(expr.X, positive)
				inspectExpression(expr.Y, positive)
			}
		case *ast.CallExpr:
			selector, ok := expr.Fun.(*ast.SelectorExpr)
			if ok && selector.Sel.Name == "IsUnknown" && positive && len(expr.Args) == 0 {
				values = append(values, expressionName(selector.X))
			}
		case *ast.Ident:
			if alias, ok := aliases[expr.Name]; ok && alias.unknownMakesAliasTrue == positive {
				values = append(values, alias.value)
			}
		}
	}
	inspectExpression(expression, true)
	return uniqueSorted(values)
}

func unknownPredicateAlias(expression ast.Expr) (presenceAlias, bool) {
	unknownMakesTrue := true
	if unary, ok := expression.(*ast.UnaryExpr); ok && unary.Op == token.NOT {
		expression = unary.X
		unknownMakesTrue = false
	}
	call, ok := expression.(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return presenceAlias{}, false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "IsUnknown" {
		return presenceAlias{}, false
	}
	return presenceAlias{value: expressionName(selector.X), unknownMakesAliasTrue: unknownMakesTrue}, true
}

func nullPresenceAlias(expression ast.Expr) (presenceAlias, bool) {
	unknownMakesTrue := false
	if unary, ok := expression.(*ast.UnaryExpr); ok && unary.Op == token.NOT {
		expression = unary.X
		unknownMakesTrue = true
	}
	call, ok := expression.(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return presenceAlias{}, false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "IsNull" {
		return presenceAlias{}, false
	}
	return presenceAlias{value: expressionName(selector.X), unknownMakesAliasTrue: unknownMakesTrue}, true
}

func unsafePresenceValues(expression ast.Expr, aliases map[string]presenceAlias, positive bool) []string {
	var values []string
	switch expr := expression.(type) {
	case *ast.ParenExpr:
		return unsafePresenceValues(expr.X, aliases, positive)
	case *ast.UnaryExpr:
		if expr.Op == token.NOT {
			return unsafePresenceValues(expr.X, aliases, !positive)
		}
	case *ast.BinaryExpr:
		if expr.Op == token.LAND || expr.Op == token.LOR {
			values = append(values, unsafePresenceValues(expr.X, aliases, positive)...)
			values = append(values, unsafePresenceValues(expr.Y, aliases, positive)...)
		}
	case *ast.CallExpr:
		selector, ok := expr.Fun.(*ast.SelectorExpr)
		if ok && selector.Sel.Name == "IsNull" && !positive && len(expr.Args) == 0 {
			values = append(values, expressionName(selector.X))
		}
	case *ast.Ident:
		if alias, ok := aliases[expr.Name]; ok && alias.unknownMakesAliasTrue == positive {
			values = append(values, alias.value)
		}
	}
	return uniqueSorted(values)
}

type unknownBranch struct {
	values      []string
	incremented map[string]bool
}

func unknownContinueBranches(body *ast.BlockStmt) []unknownBranch {
	var branches []unknownBranch
	inspectWithoutFuncLits(body, func(node ast.Node) {
		ifStmt, ok := node.(*ast.IfStmt)
		if !ok || !containsContinue(ifStmt.Body) {
			return
		}
		values := positiveUnknownValues(ifStmt.Cond)
		if len(values) == 0 {
			return
		}
		branches = append(branches, unknownBranch{
			values:      values,
			incremented: incrementedNames(ifStmt.Body),
		})
	})
	return branches
}

func containsContinue(body *ast.BlockStmt) bool {
	found := false
	inspectWithoutFuncLits(body, func(node ast.Node) {
		branch, ok := node.(*ast.BranchStmt)
		if ok && branch.Tok == token.CONTINUE {
			found = true
		}
	})
	return found
}

func incrementedNames(body *ast.BlockStmt) map[string]bool {
	names := map[string]bool{}
	inspectWithoutFuncLits(body, func(node ast.Node) {
		statement, ok := node.(*ast.IncDecStmt)
		if !ok || statement.Tok != token.INC {
			return
		}
		if ident, ok := statement.X.(*ast.Ident); ok {
			names[ident.Name] = true
		}
	})
	return names
}

func minimumCountNames(expression ast.Expr) map[string]bool {
	names := map[string]bool{}
	ast.Inspect(expression, func(node ast.Node) bool {
		binary, ok := node.(*ast.BinaryExpr)
		if !ok {
			return true
		}
		if name, ok := uncertainCountComparison(binary); ok {
			names[name] = true
		}
		return true
	})
	return names
}

func uncertainCountComparison(binary *ast.BinaryExpr) (string, bool) {
	left, leftOK := binary.X.(*ast.Ident)
	right, rightOK := binary.Y.(*ast.BasicLit)
	if leftOK && rightOK && right.Kind == token.INT {
		if (right.Value == "0" && (binary.Op == token.EQL || binary.Op == token.LEQ)) ||
			(right.Value == "1" && (binary.Op == token.NEQ || binary.Op == token.LSS)) {
			return left.Name, true
		}
	}
	rightIdent, rightIdentOK := binary.Y.(*ast.Ident)
	leftLiteral, leftLiteralOK := binary.X.(*ast.BasicLit)
	if rightIdentOK && leftLiteralOK && leftLiteral.Kind == token.INT {
		if (leftLiteral.Value == "0" && (binary.Op == token.EQL || binary.Op == token.GEQ)) ||
			(leftLiteral.Value == "1" && (binary.Op == token.NEQ || binary.Op == token.GTR)) {
			return rightIdent.Name, true
		}
	}
	return "", false
}

func intersects(left, right map[string]bool) bool {
	for name := range left {
		if right[name] {
			return true
		}
	}
	return false
}

func diagnosticsIn(node ast.Node) []*ast.CallExpr {
	var diagnostics []*ast.CallExpr
	inspectWithoutFuncLits(node, func(candidate ast.Node) {
		call, ok := candidate.(*ast.CallExpr)
		if !ok {
			return
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if ok && diagnosticMethods[selector.Sel.Name] {
			diagnostics = append(diagnostics, call)
		}
	})
	return diagnostics
}

func inspectWithoutFuncLits(node ast.Node, visit func(ast.Node)) {
	ast.Inspect(node, func(candidate ast.Node) bool {
		if candidate == nil {
			return true
		}
		if candidate != node {
			if _, ok := candidate.(*ast.FuncLit); ok {
				return false
			}
		}
		visit(candidate)
		return true
	})
}

func expressionName(expression ast.Expr) string {
	switch expression := expression.(type) {
	case *ast.Ident:
		return expression.Name
	case *ast.SelectorExpr:
		prefix := expressionName(expression.X)
		if prefix == "" {
			return expression.Sel.Name
		}
		return prefix + "." + expression.Sel.Name
	case *ast.IndexExpr:
		return expressionName(expression.X) + "[*]"
	case *ast.ParenExpr:
		return expressionName(expression.X)
	default:
		return fmt.Sprintf("value at %d", expression.Pos())
	}
}

func uniqueSorted(values []string) []string {
	set := map[string]bool{}
	for _, value := range values {
		if value != "" {
			set[value] = true
		}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
