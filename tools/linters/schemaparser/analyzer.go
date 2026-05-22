// Package schemaparser extracts Terraform schema field classifications from
// generated schema files (*_gen.go). It produces a SchemaFacts result that
// other analyzers can depend on to know whether each attribute is Required,
// Optional, Computed-only, or Optional+Computed.
//
// It also parses the resource/datasource Schema() method to detect runtime
// overrides (e.g., validators, plan modifiers). While these don't change the
// field classification, they're useful for other analyzers (e.g., checking
// UseStateForUnknown).
package schemaparser

import (
	"go/ast"
	"go/token"
	"reflect"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// FieldClass represents the classification of a Terraform schema attribute.
type FieldClass int

const (
	// Unknown means the field classification could not be determined.
	Unknown FieldClass = iota
	// ComputedOnly means Computed: true, no Optional or Required.
	ComputedOnly
	// Required means Required: true.
	Required
	// Optional means Optional: true, no Computed.
	Optional
	// OptionalComputed means Optional: true, Computed: true.
	OptionalComputed
)

// String returns the human-readable name of the field classification.
func (fc FieldClass) String() string {
	switch fc {
	case ComputedOnly:
		return "ComputedOnly"
	case Required:
		return "Required"
	case Optional:
		return "Optional"
	case OptionalComputed:
		return "Optional+Computed"
	default:
		return "Unknown"
	}
}

// AttrInfo holds classification metadata for a single attribute.
type AttrInfo struct {
	Class FieldClass
	// IsList is true if the attribute is a ListAttribute or ListNestedAttribute.
	IsList bool
	// NestedAttrs holds classifications for nested attributes (if any).
	NestedAttrs map[string]*AttrInfo
}

// SchemaInfo holds the parsed schema for a single resource or data source.
type SchemaInfo struct {
	// FuncName is the schema function name (e.g., "BudgetResourceSchema").
	FuncName string
	// Attrs maps attribute name → classification info.
	Attrs map[string]*AttrInfo
}

// SchemaFacts is the result type exported by this analyzer.
// It maps schema function names to their parsed schema info.
// It implements analysis.Fact so it can be shared across packages.
type SchemaFacts struct {
	Schemas map[string]*SchemaInfo
}

// AFact implements the analysis.Fact interface.
func (*SchemaFacts) AFact() {}

// String returns a description of the schemas found, used by analysistest.
func (sf *SchemaFacts) String() string {
	names := make([]string, 0, len(sf.Schemas))
	for name := range sf.Schemas {
		names = append(names, name)
	}
	return strings.Join(names, ", ")
}

// Analyzer is the go/analysis Analyzer for schema parsing.
var Analyzer = &analysis.Analyzer{
	Name:       "schemaparser",
	Doc:        "Extracts Terraform schema field classifications from generated schema files.",
	Run:        run,
	Requires:   []*analysis.Analyzer{inspect.Analyzer},
	ResultType: reflect.TypeOf((*SchemaFacts)(nil)),
	FactTypes:  []analysis.Fact{(*SchemaFacts)(nil)},
}

func run(pass *analysis.Pass) (interface{}, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	facts := &SchemaFacts{
		Schemas: make(map[string]*SchemaInfo),
	}

	// Find all functions that return schema.Schema and whose name ends with
	// "ResourceSchema" or "DataSourceSchema".
	nodeFilter := []ast.Node{(*ast.FuncDecl)(nil)}
	insp.Preorder(nodeFilter, func(n ast.Node) {
		fn := n.(*ast.FuncDecl)
		if fn.Name == nil {
			return
		}
		name := fn.Name.Name
		if !strings.HasSuffix(name, "ResourceSchema") && !strings.HasSuffix(name, "DataSourceSchema") {
			return
		}
		// Only process generated files.
		pos := pass.Fset.Position(fn.Pos())
		if !strings.HasSuffix(pos.Filename, "_gen.go") {
			return
		}

		info := &SchemaInfo{
			FuncName: name,
			Attrs:    make(map[string]*AttrInfo),
		}

		// Find the returned schema.Schema composite literal.
		schemaLit := findReturnedCompositeLit(fn)
		if schemaLit == nil {
			return
		}

		// Find the "Attributes" key in the schema literal.
		attrsMap := findMapField(schemaLit, "Attributes")
		if attrsMap == nil {
			return
		}

		// Parse each attribute.
		parseAttributes(attrsMap, info.Attrs)

		facts.Schemas[name] = info
	})

	// Export as a package fact so downstream packages (in the same analyzer's
	// vertical dependency chain) can inherit this package's schema classifications.
	if len(facts.Schemas) > 0 {
		pass.ExportPackageFact(facts)
	}

	// Also aggregate schemas from imported packages (inherited via vertical edges).
	// This is critical: when schemaparser runs on internal/provider, it inherits
	// facts from schemaparser running on resource_label, resource_budget, etc.
	// We merge them into the Result so that overlaycheck (which reads ResultOf)
	// gets the complete picture.
	for _, pf := range pass.AllPackageFacts() {
		if sf, ok := pf.Fact.(*SchemaFacts); ok {
			for name, info := range sf.Schemas {
				if _, exists := facts.Schemas[name]; !exists {
					facts.Schemas[name] = info
				}
			}
		}
	}

	return facts, nil
}

// findReturnedCompositeLit finds the first composite literal in a return statement.
func findReturnedCompositeLit(fn *ast.FuncDecl) *ast.CompositeLit {
	if fn.Body == nil {
		return nil
	}
	for _, stmt := range fn.Body.List {
		ret, ok := stmt.(*ast.ReturnStmt)
		if !ok {
			continue
		}
		for _, result := range ret.Results {
			if lit, ok := result.(*ast.CompositeLit); ok {
				return lit
			}
		}
	}
	return nil
}

// findMapField finds a map literal field by name in a composite literal.
func findMapField(lit *ast.CompositeLit, fieldName string) *ast.CompositeLit {
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		ident, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}
		if ident.Name == fieldName {
			if mapLit, ok := kv.Value.(*ast.CompositeLit); ok {
				return mapLit
			}
		}
	}
	return nil
}

// parseAttributes parses a map[string]schema.Attribute composite literal,
// classifying each attribute and handling nested attributes recursively.
func parseAttributes(mapLit *ast.CompositeLit, result map[string]*AttrInfo) {
	for _, elt := range mapLit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}

		// Key is a string literal like "id", "name".
		attrName := unquote(kv.Key)
		if attrName == "" {
			continue
		}

		attrLit, ok := kv.Value.(*ast.CompositeLit)
		if !ok {
			continue
		}

		info := classifyAttributeLit(attrLit)
		result[attrName] = info
	}
}

// classifyAttributeLit extracts the classification from an attribute composite literal.
func classifyAttributeLit(lit *ast.CompositeLit) *AttrInfo {
	info := &AttrInfo{}

	hasComputed := false
	hasOptional := false
	hasRequired := false

	// Determine the attribute type name (e.g., "StringAttribute", "ListNestedAttribute").
	typeName := selectorTypeName(lit.Type)

	// Check if it's a list type.
	info.IsList = strings.HasPrefix(typeName, "List")

	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}

		switch key.Name {
		case "Computed":
			hasComputed = isTrueLiteral(kv.Value)
		case "Optional":
			hasOptional = isTrueLiteral(kv.Value)
		case "Required":
			hasRequired = isTrueLiteral(kv.Value)
		case "NestedObject":
			// Recurse into nested attributes.
			nestedLit, ok := kv.Value.(*ast.CompositeLit)
			if !ok {
				continue
			}
			nestedAttrsMap := findMapField(nestedLit, "Attributes")
			if nestedAttrsMap != nil {
				info.NestedAttrs = make(map[string]*AttrInfo)
				parseAttributes(nestedAttrsMap, info.NestedAttrs)
			}
		}
	}

	switch {
	case hasRequired:
		info.Class = Required
	case hasComputed && hasOptional:
		info.Class = OptionalComputed
	case hasComputed:
		info.Class = ComputedOnly
	case hasOptional:
		info.Class = Optional
	default:
		info.Class = Unknown
	}

	return info
}

// selectorTypeName extracts the type name from a selector expression or ident.
// For schema.StringAttribute{}, it returns "StringAttribute".
func selectorTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.SelectorExpr:
		return t.Sel.Name
	case *ast.Ident:
		return t.Name
	}
	return ""
}

// isTrueLiteral checks if an expression is the boolean literal `true`.
func isTrueLiteral(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return false
	}
	return ident.Name == "true" && ident.Obj == nil
}

// unquote extracts a string from a basic literal, removing quotes.
func unquote(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			// Remove surrounding quotes.
			s := e.Value
			if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
				return s[1 : len(s)-1]
			}
		}
	}
	return ""
}
