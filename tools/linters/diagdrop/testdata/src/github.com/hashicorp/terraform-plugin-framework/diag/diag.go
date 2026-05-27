// Package diag is a stub of github.com/hashicorp/terraform-plugin-framework/diag
// for use in analysistest test fixtures.
package diag

// Diagnostics is a collection of diagnostic messages.
type Diagnostics []Diagnostic

// Diagnostic is a single diagnostic message.
type Diagnostic interface {
	Severity() string
	Summary() string
	Detail() string
}

// Append adds diagnostics.
func (d *Diagnostics) Append(in ...Diagnostic) {
}

// HasError returns true if any diagnostic has error severity.
func (d Diagnostics) HasError() bool {
	return false
}
