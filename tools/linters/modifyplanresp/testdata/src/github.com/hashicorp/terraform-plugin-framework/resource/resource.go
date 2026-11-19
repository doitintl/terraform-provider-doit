package resource

import "github.com/hashicorp/terraform-plugin-framework/tfsdk"

type Diagnostics []string

type ModifyPlanRequest struct {
	Plan tfsdk.Plan
}

type ModifyPlanResponse struct {
	Diagnostics     Diagnostics
	Plan            tfsdk.Plan
	RequiresReplace []string
}
