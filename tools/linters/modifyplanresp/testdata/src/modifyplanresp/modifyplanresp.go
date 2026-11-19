package modifyplanresp

import "github.com/hashicorp/terraform-plugin-framework/resource"

// Not a test file: never flagged.
func notATest() {
	_ = &resource.ModifyPlanResponse{}
	_ = new(resource.ModifyPlanResponse)
	var resp resource.ModifyPlanResponse
	_ = resp
}
