package invarianttest

import "context"

// Stubs to simulate the Terraform framework types.
type createRequest struct{}
type createResponse struct{ State stateObj }
type readRequest struct{}
type readResponse struct{}
type updateRequest struct{}
type updateResponse struct{ State stateObj }
type importRequest struct{}
type importResponse struct{}
type stateObj struct{}
type diagnostics struct{}

func (s stateObj) Set(ctx context.Context, v interface{}) diagnostics { return diagnostics{} }

type myModel struct{}

type myResource struct{}

func overlayComputedFields(m *myModel)         {}
func mapResourceToModel(m *myModel)            {}
func (r *myResource) populateState(m *myModel) {}
func mapBudgetToModel(m *myModel)              {}

// --- BAD: Create calls mapping functions and sets state without overlay ---

func (r *myResource) Create(ctx context.Context, req createRequest, resp *createResponse) { // want "Create must call an overlay function"
	var plan myModel
	mapResourceToModel(&plan)  // want "Create must not call mapResourceToModel directly"
	resp.State.Set(ctx, &plan) // sets state — overlay is required
}

// --- BAD: Update calls populateState and sets state without overlay ---

func (r *myResource) Update(ctx context.Context, req updateRequest, resp *updateResponse) { // want "Update must call an overlay function"
	var plan myModel
	r.populateState(&plan)     // want "Update must not call populateState directly"
	resp.State.Set(ctx, &plan) // sets state — overlay is required
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

// --- Additional BAD: Create calls mapXxxToModel and sets state ---

type anotherResource struct{}

func (r *anotherResource) Create(ctx context.Context, req createRequest, resp *createResponse) { // want "Create must call an overlay function"
	var plan myModel
	mapBudgetToModel(&plan)    // want "Create must not call mapBudgetToModel directly"
	resp.State.Set(ctx, &plan) // sets state — overlay is required
}

// --- GOOD: Create calls overlay ---

type goodResource struct{}

func (r *goodResource) populateState(m *myModel) {}

func (r *goodResource) Create(ctx context.Context, req createRequest, resp *createResponse) {
	var plan myModel
	overlayComputedFields(&plan) // OK
	resp.State.Set(ctx, &plan)
}

// --- GOOD: Read calls populateState ---

func (r *goodResource) Read(ctx context.Context, req readRequest, resp *readResponse) {
	var state myModel
	r.populateState(&state) // OK
}

func (r *goodResource) Update(ctx context.Context, req updateRequest, resp *updateResponse) {
	var plan myModel
	overlayComputedFields(&plan) // OK
	resp.State.Set(ctx, &plan)
}

// --- GOOD: Create without overlay is OK when no state is set (no-op) ---

type noopResource struct{}

func (r *noopResource) Create(_ context.Context, _ createRequest, resp *createResponse) {
	// Import-only resource: no state.Set call, so no overlay needed. ✓
}
