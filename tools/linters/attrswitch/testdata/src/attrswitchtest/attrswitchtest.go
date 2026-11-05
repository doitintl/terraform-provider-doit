package attrswitchtest

import (
	ds "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// GOOD: all four nested kinds.
func complete(a schema.Attribute) int {
	switch a := a.(type) {
	case schema.SingleNestedAttribute:
		return len(a.Attributes)
	case schema.ListNestedAttribute:
		return len(a.NestedObject.Attributes)
	case schema.SetNestedAttribute:
		return len(a.NestedObject.Attributes)
	case schema.MapNestedAttribute:
		return len(a.NestedObject.Attributes)
	}
	return 0
}

// BAD: Map is missing.
func missingMap(a schema.Attribute) int {
	switch a := a.(type) { // want `handles some nested attribute kinds but not MapNestedAttribute`
	case schema.SingleNestedAttribute:
		return len(a.Attributes)
	case schema.ListNestedAttribute:
		return len(a.NestedObject.Attributes)
	case schema.SetNestedAttribute:
		return len(a.NestedObject.Attributes)
	}
	return 0
}

// GOOD: a commented default says the other kinds are skipped on purpose.
func commentedDefault(a schema.Attribute) int {
	switch a := a.(type) {
	case schema.SetNestedAttribute:
		return len(a.NestedObject.Attributes)
	default:
		// Only sets are relevant here.
		return 0
	}
}

// BAD: a bare default does not say why.
func bareDefault(a schema.Attribute) int {
	switch a := a.(type) { // want `handles some nested attribute kinds but not SingleNestedAttribute, ListNestedAttribute, MapNestedAttribute`
	case schema.SetNestedAttribute:
		return len(a.NestedObject.Attributes)
	default:
		return 0
	}
}

// GOOD: no nested kinds and no checked fields read.
func scalarsOnly(a schema.Attribute) bool {
	switch a.(type) {
	case schema.StringAttribute, schema.Int64Attribute:
		return true
	}
	return false
}

// BAD: reads PlanModifiers from some kinds only.
func planModifiersPartial(a schema.Attribute) int {
	switch a := a.(type) { // want `reads PlanModifiers but does not handle Int64Attribute, ListNestedAttribute, MapNestedAttribute, SetNestedAttribute, SingleNestedAttribute`
	case schema.StringAttribute:
		return len(a.PlanModifiers)
	}
	return 0
}

// GOOD: reads Validators from every kind that has them.
func validatorsAll(a schema.Attribute) int {
	switch a := a.(type) {
	case schema.StringAttribute:
		return len(a.Validators)
	case schema.Int64Attribute:
		return len(a.Validators)
	case schema.SingleNestedAttribute:
		return len(a.Validators)
	case schema.ListNestedAttribute:
		return len(a.Validators)
	case schema.SetNestedAttribute:
		return len(a.Validators)
	case schema.MapNestedAttribute:
		return len(a.Validators)
	}
	return 0
}

// BAD: Default exists only on String and Int64; Int64 is missing.
func defaultPartial(a schema.Attribute) bool {
	switch a := a.(type) { // want `reads Default but does not handle Int64Attribute`
	case schema.StringAttribute:
		return a.Default != nil
	}
	return false
}

// BAD: data source schema package.
func dataSourceMissing(a ds.Attribute) int {
	switch a := a.(type) { // want `handles some nested attribute kinds but not ListNestedAttribute, SetNestedAttribute, MapNestedAttribute`
	case ds.SingleNestedAttribute:
		return len(a.Attributes)
	}
	return 0
}

// GOOD: switches over other interfaces are ignored.
func other(v any) int {
	switch v.(type) {
	case schema.SingleNestedAttribute:
		return 1
	}
	return 0
}

type attr = schema.Attribute

type mapAttr = schema.MapNestedAttribute

// BAD: the switch input is declared through an alias.
func aliasInput(a attr) int {
	switch a := a.(type) { // want `handles some nested attribute kinds but not MapNestedAttribute`
	case schema.SingleNestedAttribute:
		return len(a.Attributes)
	case schema.ListNestedAttribute:
		return len(a.NestedObject.Attributes)
	case schema.SetNestedAttribute:
		return len(a.NestedObject.Attributes)
	}
	return 0
}

// GOOD: a case type declared through an alias still counts as handled.
func aliasCase(a schema.Attribute) int {
	switch a := a.(type) {
	case schema.SingleNestedAttribute:
		return len(a.Attributes)
	case schema.ListNestedAttribute:
		return len(a.NestedObject.Attributes)
	case schema.SetNestedAttribute:
		return len(a.NestedObject.Attributes)
	case mapAttr:
		return len(a.NestedObject.Attributes)
	}
	return 0
}

// BAD: the comment documents the case after the bare default, not the default.
func defaultFirst(a schema.Attribute) int {
	switch a := a.(type) { // want `handles some nested attribute kinds but not SingleNestedAttribute, ListNestedAttribute, MapNestedAttribute`
	default:
		return 0
	// Sets are the only nested kind that matters here.
	case schema.SetNestedAttribute:
		return len(a.NestedObject.Attributes)
	}
}

// GOOD: a comment inside a default that is not the last clause.
func commentedDefaultFirst(a schema.Attribute) int {
	switch a := a.(type) {
	default:
		// Only sets are relevant here.
		return 0
	case schema.SetNestedAttribute:
		return len(a.NestedObject.Attributes)
	}
}

// GOOD: Default is only written, never read.
func writeOnly(a schema.Attribute) schema.Attribute {
	switch a := a.(type) {
	case schema.StringAttribute:
		a.Default = nil
		return a
	}
	return a
}

// BAD: append reads Validators before assigning it.
func readModifyWrite(a schema.Attribute) schema.Attribute {
	switch a := a.(type) { // want `reads Validators but does not handle Int64Attribute, ListNestedAttribute, MapNestedAttribute, SetNestedAttribute, SingleNestedAttribute`
	case schema.StringAttribute:
		a.Validators = append(a.Validators, nil)
		return a
	}
	return a
}

// GOOD: a final default that holds only a comment.
func commentOnlyFinalDefault(a schema.Attribute) int {
	switch a := a.(type) {
	case schema.SetNestedAttribute:
		return len(a.NestedObject.Attributes)
	default:
		// Only sets are relevant here.
	}
	return 0
}

// GOOD: a trailing comment on the default's last statement.
func trailingCommentOnStatement(a schema.Attribute) int {
	switch a := a.(type) {
	default:
		return 0 // Other kinds are intentionally ignored.
	case schema.SetNestedAttribute:
		return len(a.NestedObject.Attributes)
	}
}

// GOOD: an empty default with a comment on its own line, then another case.
func trailingCommentOnEmptyDefault(a schema.Attribute) int {
	switch a := a.(type) {
	default: // Other kinds are intentionally ignored.
	case schema.SetNestedAttribute:
		return len(a.NestedObject.Attributes)
	}
	return 0
}

// BAD: an empty default whose next case has a comment on the line above it.
func emptyDefaultThenCommentedCase(a schema.Attribute) int {
	switch a := a.(type) { // want `handles some nested attribute kinds but not SingleNestedAttribute, ListNestedAttribute, MapNestedAttribute`
	default:
	// Sets are the only nested kind that matters here.
	case schema.SetNestedAttribute:
		return len(a.NestedObject.Attributes)
	}
	return 0
}
