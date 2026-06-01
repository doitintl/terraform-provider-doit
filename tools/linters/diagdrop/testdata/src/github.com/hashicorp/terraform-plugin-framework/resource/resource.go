// Package resource is a stub of github.com/hashicorp/terraform-plugin-framework/resource
// for use in analysistest test fixtures.
package resource

import (
	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// ValidateConfigRequest is a stub request type.
type ValidateConfigRequest struct{}

// ValidateConfigResponse is a stub response type with Diagnostics.
type ValidateConfigResponse struct {
	Diagnostics diag.Diagnostics
}

// CreateRequest is a stub request type.
type CreateRequest struct{}

// CreateResponse is a stub response type with Diagnostics.
type CreateResponse struct {
	Diagnostics diag.Diagnostics
}

// ReadRequest is a stub request type.
type ReadRequest struct{}

// ReadResponse is a stub response type with Diagnostics.
type ReadResponse struct {
	Diagnostics diag.Diagnostics
}

// UpdateRequest is a stub request type.
type UpdateRequest struct{}

// UpdateResponse is a stub response type with Diagnostics.
type UpdateResponse struct {
	Diagnostics diag.Diagnostics
}

// DeleteRequest is a stub request type.
type DeleteRequest struct{}

// DeleteResponse is a stub response type with Diagnostics.
type DeleteResponse struct {
	Diagnostics diag.Diagnostics
}
