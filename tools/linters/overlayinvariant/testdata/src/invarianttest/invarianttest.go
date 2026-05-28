package invarianttest

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// Stubs to simulate the Terraform framework types.
type schemaRequest struct{}
type schemaResponse struct{ Schema schema.Schema }
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

// Schema method for myResource — references the generated schema.
func (r *myResource) Schema(ctx context.Context, req schemaRequest, resp *schemaResponse) {
	s := MyResourceSchema(ctx)
	resp.Schema = s
}

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

// Schema method for anotherResource — references the generated schema.
func (r *anotherResource) Schema(ctx context.Context, req schemaRequest, resp *schemaResponse) {
	s := AnotherResourceSchema(ctx)
	resp.Schema = s
}

func (r *anotherResource) Create(ctx context.Context, req createRequest, resp *createResponse) { // want "Create must call an overlay function"
	var plan myModel
	mapBudgetToModel(&plan)    // want "Create must not call mapBudgetToModel directly"
	resp.State.Set(ctx, &plan) // sets state — overlay is required
}

// --- GOOD: Create calls overlay ---

type goodResource struct{}

func (r *goodResource) populateState(m *myModel) {}

// Schema method for goodResource — references the generated schema.
func (r *goodResource) Schema(ctx context.Context, req schemaRequest, resp *schemaResponse) {
	s := GoodResourceSchema(ctx)
	resp.Schema = s
}

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

// --- GOOD: Create without overlay is OK when schema has no computed fields ---
// This is the label_assignments pattern: all fields are Required/Optional,
// so no overlay is needed. The resource writes the plan directly to state.

type allRequiredResource struct{}

// Schema method references a schema with only Required/Optional fields.
func (r *allRequiredResource) Schema(ctx context.Context, req schemaRequest, resp *schemaResponse) {
	s := AllRequiredResourceSchema(ctx)
	resp.Schema = s
}

func (r *allRequiredResource) Create(ctx context.Context, req createRequest, resp *createResponse) {
	// No overlay needed: schema has no Computed-only or Optional+Computed fields. ✓
	var plan myModel
	resp.State.Set(ctx, &plan)
}

func (r *allRequiredResource) Update(ctx context.Context, req updateRequest, resp *updateResponse) {
	// No overlay needed: schema has no Computed-only or Optional+Computed fields. ✓
	var plan myModel
	resp.State.Set(ctx, &plan)
}

// --- GOOD: Create without overlay is OK when schema is defined inline ---
// This also covers the label_assignments case where the resource doesn't use
// a generated schema function at all.

type inlineSchemaResource struct{}

func (r *inlineSchemaResource) Schema(ctx context.Context, req schemaRequest, resp *schemaResponse) {
	// Inline schema — no call to XxxResourceSchema.
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{Required: true},
		},
	}
}

func (r *inlineSchemaResource) Create(ctx context.Context, req createRequest, resp *createResponse) {
	// No overlay needed: no generated schema function found. ✓
	var plan myModel
	resp.State.Set(ctx, &plan)
}

// --- BAD: Create without overlay when nested attrs have Optional+Computed ---
// Even though no top-level field is Optional+Computed, the nested "config.mode"
// is, so an overlay is required.

type nestedOnlyOCResource struct{}

func overlayNestedOnlyOCComputedFields(m *myModel) {}

func (r *nestedOnlyOCResource) Schema(ctx context.Context, req schemaRequest, resp *schemaResponse) {
	s := NestedOnlyOCResourceSchema(ctx)
	resp.Schema = s
}

func (r *nestedOnlyOCResource) Create(ctx context.Context, req createRequest, resp *createResponse) { // want "Create must call an overlay function"
	var plan myModel
	resp.State.Set(ctx, &plan)
}

// --- GOOD: Create with overlay when nested attrs have Optional+Computed ---

type nestedOnlyOCGoodResource struct{}

func (r *nestedOnlyOCGoodResource) Schema(ctx context.Context, req schemaRequest, resp *schemaResponse) {
	s := NestedOnlyOCResourceSchema(ctx)
	resp.Schema = s
}

func (r *nestedOnlyOCGoodResource) Create(ctx context.Context, req createRequest, resp *createResponse) {
	var plan myModel
	overlayNestedOnlyOCComputedFields(&plan)
	resp.State.Set(ctx, &plan)
}
