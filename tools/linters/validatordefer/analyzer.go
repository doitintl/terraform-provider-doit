// Package validatordefer detects canonical Terraform validators that emit
// diagnostics before unknown configuration values have resolved.
package validatordefer

import (
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strings"

	"github.com/doitintl/terraform-provider-doit/tools/linters/internal/validatoranalysis"
	"github.com/doitintl/terraform-provider-doit/tools/linters/validatorshape"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/ctrlflow"
	"golang.org/x/tools/go/cfg"
)

// Analyzer is the go/analysis analyzer for validatordefer.
var Analyzer = &analysis.Analyzer{
	Name: "validatordefer",
	Doc:  "Detects Terraform validator diagnostics that do not defer unknown values.",
	Run:  run,
	Requires: []*analysis.Analyzer{
		ctrlflow.Analyzer,
		validatorshape.Analyzer,
	},
}

type valueSet = validatoranalysis.ValueSet

type functionAnalysis struct {
	pass        *analysis.Pass
	reachable   map[token.Pos]bool
	reported    map[token.Pos]bool
	knownAtIf   map[token.Pos]valueSet
	knownAtCall map[token.Pos]valueSet
	guards      map[token.Pos][]validatoranalysis.ValueRef
	trackers    map[token.Pos]validatorshape.TrackerFact
}

func run(pass *analysis.Pass) (any, error) {
	cfgs := pass.ResultOf[ctrlflow.Analyzer].(*ctrlflow.CFGs)
	shape := pass.ResultOf[validatorshape.Analyzer].(*validatorshape.Result)
	reported := map[token.Pos]bool{}
	for _, facts := range shape.Functions {
		if !facts.Conforms {
			continue
		}
		declaration := facts.Declaration
		state := &functionAnalysis{
			pass:        pass,
			reachable:   reachablePositions(cfgs.FuncDecl(declaration)),
			reported:    reported,
			knownAtIf:   facts.KnownAtIf,
			knownAtCall: facts.KnownAtCall,
			guards:      facts.Guards,
			trackers:    facts.Trackers,
		}
		state.checkPresenceDiagnostics(declaration.Body)
		state.checkUncertainLoopCounts(declaration.Body)
	}
	return nil, nil
}

func reachablePositions(graph *cfg.CFG) map[token.Pos]bool {
	positions := map[token.Pos]bool{}
	if graph == nil {
		return positions
	}
	for _, block := range graph.Blocks {
		if !block.Live {
			continue
		}
		for _, node := range block.Nodes {
			validatoranalysis.Inspect(node, func(candidate ast.Node) {
				if candidate != nil {
					positions[candidate.Pos()] = true
				}
			})
		}
	}
	return positions
}

func (state *functionAnalysis) checkPresenceDiagnostics(body *ast.BlockStmt) {
	validatoranalysis.Inspect(body, func(node ast.Node) {
		statement, ok := node.(*ast.IfStmt)
		if !ok {
			return
		}
		unsafe := unsafePresenceValues(state.pass, statement.Cond, state.knownAtIf[statement.Pos()])
		if len(unsafe) == 0 {
			return
		}
		for _, diagnostic := range diagnosticsIn(state.pass, statement.Body) {
			state.report(diagnostic, unsafe)
		}
	})
}

func unsafePresenceValues(pass *analysis.Pass, expression ast.Expr, known valueSet) []validatoranalysis.ValueRef {
	if parenthesized, ok := expression.(*ast.ParenExpr); ok {
		return unsafePresenceValues(pass, parenthesized.X, known)
	}
	if binary, ok := expression.(*ast.BinaryExpr); ok && binary.Op == token.LOR {
		return uniqueValues(append(
			unsafePresenceValues(pass, binary.X, known),
			unsafePresenceValues(pass, binary.Y, known)...,
		))
	}
	branchKnown := validatoranalysis.CloneValues(known)
	validatoranalysis.AddValues(branchKnown, validatoranalysis.KnownValuesWhen(pass, expression, true))
	return withoutKnown(presenceValues(pass, expression, true), branchKnown)
}

func presenceValues(pass *analysis.Pass, expression ast.Expr, positive bool) []validatoranalysis.ValueRef {
	switch expression := expression.(type) {
	case *ast.ParenExpr:
		return presenceValues(pass, expression.X, positive)
	case *ast.UnaryExpr:
		if expression.Op == token.NOT {
			return presenceValues(pass, expression.X, !positive)
		}
	case *ast.BinaryExpr:
		if expression.Op == token.LAND || expression.Op == token.LOR {
			return uniqueValues(append(
				presenceValues(pass, expression.X, positive),
				presenceValues(pass, expression.Y, positive)...,
			))
		}
	case *ast.CallExpr:
		kind, value, ok := validatoranalysis.Predicate(pass, expression)
		if ok && kind == validatoranalysis.PredicateNull && !positive {
			return []validatoranalysis.ValueRef{value}
		}
	}
	return nil
}

func (state *functionAnalysis) checkUncertainLoopCounts(body *ast.BlockStmt) {
	var visitBlock func(*ast.BlockStmt)
	visitBlock = func(block *ast.BlockStmt) {
		for index, statement := range block.List {
			if loop, ok := statement.(*ast.RangeStmt); ok {
				branches := unknownContinueBranches(loop.Body, state.guards, state.trackers)
				var intervening []ast.Stmt
				for _, following := range block.List[index+1:] {
					conditional, ok := following.(*ast.IfStmt)
					if !ok {
						if _, nested := following.(*ast.RangeStmt); nested {
							break
						}
						intervening = append(intervening, following)
						continue
					}
					counts := minimumCountObjects(state.pass, conditional.Cond)
					if len(counts) == 0 {
						continue
					}
					references := referencedObjects(state.pass, conditional.Cond)
					var unsafe []validatoranalysis.ValueRef
					for _, branch := range branches {
						relevant := cloneObjectSet(counts)
						addObjects(relevant, branch.trackers)
						if statementsMentionObjects(state.pass, intervening, relevant) {
							continue
						}
						if !intersectsObjects(branch.trackers, references) {
							unsafe = append(unsafe, branch.values...)
						}
					}
					for _, diagnostic := range diagnosticsIn(state.pass, conditional.Body) {
						state.report(diagnostic, unsafe)
					}
				}
			}

			switch statement := statement.(type) {
			case *ast.BlockStmt:
				visitBlock(statement)
			case *ast.IfStmt:
				visitBlock(statement.Body)
				if alternative, ok := statement.Else.(*ast.BlockStmt); ok {
					visitBlock(alternative)
				}
			case *ast.ForStmt:
				visitBlock(statement.Body)
			case *ast.RangeStmt:
				visitBlock(statement.Body)
			}
		}
	}
	visitBlock(body)
}

func cloneObjectSet(values map[types.Object]bool) map[types.Object]bool {
	result := map[types.Object]bool{}
	addObjects(result, values)
	return result
}

func addObjects(destination, values map[types.Object]bool) {
	for object := range values {
		destination[object] = true
	}
}

func statementsMentionObjects(pass *analysis.Pass, statements []ast.Stmt, objects map[types.Object]bool) bool {
	for _, statement := range statements {
		mentioned := false
		validatoranalysis.Inspect(statement, func(node ast.Node) {
			identifier, ok := node.(*ast.Ident)
			if ok && objects[pass.TypesInfo.ObjectOf(identifier)] {
				mentioned = true
			}
		})
		if mentioned {
			return true
		}
	}
	return false
}

type unknownBranch struct {
	values   []validatoranalysis.ValueRef
	trackers map[types.Object]bool
}

func unknownContinueBranches(
	body *ast.BlockStmt,
	guards map[token.Pos][]validatoranalysis.ValueRef,
	trackers map[token.Pos]validatorshape.TrackerFact,
) []unknownBranch {
	var branches []unknownBranch
	for _, node := range body.List {
		statement, ok := node.(*ast.IfStmt)
		if !ok || !endsWithContinue(statement.Body) {
			continue
		}
		if tracker, ok := trackers[statement.Pos()]; ok {
			branches = append(branches, unknownBranch{
				values:   tracker.Values,
				trackers: map[types.Object]bool{tracker.Tracker.Key.Root: true},
			})
			continue
		}
		if values := guards[statement.Pos()]; len(values) > 0 {
			branches = append(branches, unknownBranch{values: values, trackers: map[types.Object]bool{}})
		}
	}
	return branches
}

func endsWithContinue(body *ast.BlockStmt) bool {
	if body == nil || len(body.List) == 0 {
		return false
	}
	branch, ok := body.List[len(body.List)-1].(*ast.BranchStmt)
	return ok && branch.Tok == token.CONTINUE
}

func minimumCountObjects(pass *analysis.Pass, expression ast.Expr) map[types.Object]bool {
	objects := map[types.Object]bool{}
	ast.Inspect(expression, func(node ast.Node) bool {
		binary, ok := node.(*ast.BinaryExpr)
		if !ok {
			return true
		}
		if identifier := minimumCountIdentifier(binary); identifier != nil {
			objects[pass.TypesInfo.ObjectOf(identifier)] = true
		}
		return true
	})
	delete(objects, nil)
	return objects
}

func minimumCountIdentifier(binary *ast.BinaryExpr) *ast.Ident {
	if left, ok := binary.X.(*ast.Ident); ok {
		if right, ok := binary.Y.(*ast.BasicLit); ok && right.Kind == token.INT {
			if (right.Value == "0" && (binary.Op == token.EQL || binary.Op == token.LEQ)) ||
				(right.Value == "1" && (binary.Op == token.NEQ || binary.Op == token.LSS)) {
				return left
			}
		}
	}
	if right, ok := binary.Y.(*ast.Ident); ok {
		if left, ok := binary.X.(*ast.BasicLit); ok && left.Kind == token.INT {
			if (left.Value == "0" && (binary.Op == token.EQL || binary.Op == token.GEQ)) ||
				(left.Value == "1" && (binary.Op == token.NEQ || binary.Op == token.GTR)) {
				return right
			}
		}
	}
	return nil
}

func referencedObjects(pass *analysis.Pass, expression ast.Expr) map[types.Object]bool {
	objects := map[types.Object]bool{}
	ast.Inspect(expression, func(node ast.Node) bool {
		if identifier, ok := node.(*ast.Ident); ok {
			objects[pass.TypesInfo.ObjectOf(identifier)] = true
		}
		return true
	})
	delete(objects, nil)
	return objects
}

func intersectsObjects(left, right map[types.Object]bool) bool {
	for object := range left {
		if right[object] {
			return true
		}
	}
	return false
}

func diagnosticsIn(pass *analysis.Pass, node ast.Node) []*ast.CallExpr {
	var diagnostics []*ast.CallExpr
	validatoranalysis.Inspect(node, func(candidate ast.Node) {
		call, ok := candidate.(*ast.CallExpr)
		if ok && validatoranalysis.IsDiagnosticCall(pass, call) {
			diagnostics = append(diagnostics, call)
		}
	})
	return diagnostics
}

func (state *functionAnalysis) report(call *ast.CallExpr, values []validatoranalysis.ValueRef) {
	if len(values) == 0 || state.reported[call.Pos()] || !state.reachable[call.Pos()] {
		return
	}
	values = uniqueValues(withoutKnown(values, state.knownAtCall[call.Pos()]))
	if len(values) == 0 {
		return
	}
	state.reported[call.Pos()] = true
	displays := make([]string, 0, len(values))
	for _, value := range values {
		displays = append(displays, value.Display)
	}
	sort.Strings(displays)
	state.pass.Reportf(call.Pos(), "validator diagnostic may be emitted while %s is unknown; defer validation until the value is known", strings.Join(displays, ", "))
}

func withoutKnown(values []validatoranalysis.ValueRef, known valueSet) []validatoranalysis.ValueRef {
	result := make([]validatoranalysis.ValueRef, 0, len(values))
	for _, value := range values {
		if _, ok := known[value.Key]; !ok {
			result = append(result, value)
		}
	}
	return result
}

func uniqueValues(values []validatoranalysis.ValueRef) []validatoranalysis.ValueRef {
	return validatoranalysis.UniqueValues(values)
}
