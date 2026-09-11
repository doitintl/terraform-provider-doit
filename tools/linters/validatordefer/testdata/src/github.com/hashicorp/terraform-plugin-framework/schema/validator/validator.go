package validator

import (
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type StringRequest struct {
	ConfigValue types.String
	Path        any
}

type StringResponse struct {
	Diagnostics diag.Diagnostics
}
