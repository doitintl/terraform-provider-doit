package datasource

import "github.com/hashicorp/terraform-plugin-framework/diag"

type ValidateConfigRequest struct{}

type ValidateConfigResponse struct {
	Diagnostics diag.Diagnostics
}
