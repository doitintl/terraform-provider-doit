package modifyplanresp

import (
	"other"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
)

type alias = resource.ModifyPlanResponse

func modifyPlan(_ resource.ModifyPlanRequest, _ *resource.ModifyPlanResponse) {}

func TestBad(t *testing.T) {
	req := resource.ModifyPlanRequest{}

	modifyPlan(req, &resource.ModifyPlanResponse{}) // want `ModifyPlanResponse without Plan hands ModifyPlan an empty plan`

	resp := resource.ModifyPlanResponse{} // want `without Plan`
	modifyPlan(req, &resp)

	modifyPlan(req, &resource.ModifyPlanResponse{Diagnostics: nil}) // want `without Plan`

	modifyPlan(req, &resource.ModifyPlanResponse{ // want `without Plan`
		Diagnostics:     nil,
		RequiresReplace: nil,
	})

	modifyPlan(req, &alias{}) // want `without Plan`

	modifyPlan(req, new(resource.ModifyPlanResponse)) // want `from new\(\)`

	var zero resource.ModifyPlanResponse // want `declared as a zero value`
	modifyPlan(req, &zero)

	var (
		a, b resource.ModifyPlanResponse // want `declared as a zero value`
	)
	modifyPlan(req, &a)
	modifyPlan(req, &b)

	// Elided literals in table-driven tests.
	_ = []*resource.ModifyPlanResponse{{}}                                  // want `without Plan`
	_ = map[string]*resource.ModifyPlanResponse{"case": {Diagnostics: nil}} // want `without Plan`
	_ = []resource.ModifyPlanResponse{{}}                                   // want `without Plan`

	// An unseeded literal in new(expr) is reported once, as a literal.
	modifyPlan(req, new(resource.ModifyPlanResponse{})) // want `without Plan`
}

func TestGood(t *testing.T) {
	req := resource.ModifyPlanRequest{}

	modifyPlan(req, &resource.ModifyPlanResponse{Plan: req.Plan})

	modifyPlan(req, &resource.ModifyPlanResponse{
		Diagnostics: nil,
		Plan:        tfsdk.Plan{Schema: nil, Raw: nil},
	})

	// An explicit empty Plan is deliberate.
	modifyPlan(req, &resource.ModifyPlanResponse{Plan: tfsdk.Plan{}})

	// Unkeyed literals set every field.
	modifyPlan(req, &resource.ModifyPlanResponse{nil, req.Plan, nil})

	var initialized resource.ModifyPlanResponse = resource.ModifyPlanResponse{Plan: req.Plan}
	modifyPlan(req, &initialized)

	var ptr *resource.ModifyPlanResponse
	_ = ptr

	_ = []*resource.ModifyPlanResponse{{Plan: req.Plan}}
	_ = map[string]*resource.ModifyPlanResponse{"case": {Plan: req.Plan}}

	// new(expr) copies a seeded value.
	seeded := resource.ModifyPlanResponse{Plan: req.Plan}
	modifyPlan(req, new(seeded))
	modifyPlan(req, new(resource.ModifyPlanResponse{Plan: req.Plan}))

	// An unrelated type of the same name.
	_ = &other.ModifyPlanResponse{}
	_ = new(other.ModifyPlanResponse)
	var o other.ModifyPlanResponse
	_ = o
}
