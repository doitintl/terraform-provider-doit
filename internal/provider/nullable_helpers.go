package provider

import "github.com/oapi-codegen/nullable"

// nullableToPointer converts a nullable.Nullable[T] to a *T pointer.
// If the nullable is null or unspecified, it returns nil.
func nullableToPointer[T any](n nullable.Nullable[T]) *T {
	if !n.IsSpecified() || n.IsNull() {
		return nil
	}
	return new(n.MustGet())
}

// pointerToNullable converts a *T pointer to a nullable.Nullable[T].
// If the pointer is nil, it returns an unspecified (empty) Nullable[T].
func pointerToNullable[T any](p *T) nullable.Nullable[T] {
	var n nullable.Nullable[T]
	if p != nil {
		n.Set(*p)
	}
	return n
}

// valueToNullable converts a value to a specified nullable.Nullable[T].
func valueToNullable[T any](v T) nullable.Nullable[T] {
	var n nullable.Nullable[T]
	n.Set(v)
	return n
}
