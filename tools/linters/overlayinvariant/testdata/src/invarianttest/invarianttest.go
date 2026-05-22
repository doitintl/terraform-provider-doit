package invarianttest

import "context"

// Stubs to simulate the Terraform framework types.
type createRequest struct{}
type createResponse struct{}
type readRequest struct{}
type readResponse struct{}
type updateRequest struct{}
type updateResponse struct{}
type importRequest struct{}
type importResponse struct{}

type myModel struct{}

type myResource struct{}

func overlayComputedFields(m *myModel)         {}
func mapResourceToModel(m *myModel)            {}
func (r *myResource) populateState(m *myModel) {}
func mapBudgetToModel(m *myModel)              {}

// --- BAD: Create calls mapping functions ---

func (r *myResource) Create(ctx context.Context, req createRequest, resp *createResponse) {
	var plan myModel
	mapResourceToModel(&plan) // want "Create must not call mapResourceToModel directly"
}

// --- BAD: Update calls populateState ---

func (r *myResource) Update(ctx context.Context, req updateRequest, resp *updateResponse) {
	var plan myModel
	r.populateState(&plan) // want "Update must not call populateState directly"
}

// --- BAD: Read calls overlay ---

func (r *myResource) Read(ctx context.Context, req readRequest, resp *readResponse) {
	var state myModel
	overlayComputedFields(&state) // want "Read must not call overlayComputedFields"
}

// --- BAD: ImportState calls overlay ---

func (r *myResource) ImportState(ctx context.Context, req importRequest, resp *importResponse) {
	var state myModel
	overlayComputedFields(&state) // want "ImportState must not call overlayComputedFields"
}

// --- Additional BAD: Create calls mapXxxToModel ---

type anotherResource struct{}

func (r *anotherResource) Create(ctx context.Context, req createRequest, resp *createResponse) {
	var plan myModel
	mapBudgetToModel(&plan) // want "Create must not call mapBudgetToModel directly"
}

// --- GOOD: Create calls overlay ---

type goodResource struct{}

func (r *goodResource) populateState(m *myModel) {}

func (r *goodResource) Create(ctx context.Context, req createRequest, resp *createResponse) {
	var plan myModel
	overlayComputedFields(&plan) // OK
}

// --- GOOD: Read calls populateState ---

func (r *goodResource) Read(ctx context.Context, req readRequest, resp *readResponse) {
	var state myModel
	r.populateState(&state) // OK
}

func (r *goodResource) Update(ctx context.Context, req updateRequest, resp *updateResponse) {
	var plan myModel
	overlayComputedFields(&plan) // OK
}
