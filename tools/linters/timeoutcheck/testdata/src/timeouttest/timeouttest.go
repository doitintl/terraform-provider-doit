package timeouttest

import (
	"context"
	"time"
)

// Stubs
type createRequest struct{}
type createResponse struct {
	Diagnostics diagList
	State       stateObj
}
type readRequest struct{ State stateObj }
type readResponse struct {
	Diagnostics diagList
	State       stateObj
}
type updateRequest struct{}
type updateResponse struct {
	Diagnostics diagList
	State       stateObj
}
type deleteRequest struct{}
type deleteResponse struct{ Diagnostics diagList }
type diagList struct{}

func (d diagList) Append(diags ...interface{})       {}
func (d diagList) HasError() bool                    { return false }
func (d diagList) AddError(summary, detail string)   {}
func (d diagList) AddWarning(summary, detail string) {}

type stateObj struct{}

func (s stateObj) Get(ctx context.Context, v interface{}) diagList { return diagList{} }
func (s stateObj) Set(ctx context.Context, v interface{}) diagList { return diagList{} }
func (s stateObj) RemoveResource(ctx context.Context)              {}

type httpResponse struct{ Body []byte }

func (r httpResponse) StatusCode() int { return 200 }

type myModel struct{}
type myClient struct{}

func (c *myClient) GetResourceWithResponse(ctx context.Context, id string) (httpResponse, error) {
	return httpResponse{}, nil
}
func (c *myClient) CreateResourceWithResponse(ctx context.Context, body interface{}) (httpResponse, error) {
	return httpResponse{}, nil
}
func (c *myClient) DeleteResourceWithResponse(ctx context.Context, id string) (httpResponse, error) {
	return httpResponse{}, nil
}

// --- BAD: Create without timeout ---

type badResource struct{ client *myClient }

func (r *badResource) Create(ctx context.Context, req createRequest, resp *createResponse) { // want "Create method must wrap context"
	apiResp, _ := r.client.CreateResourceWithResponse(ctx, nil)
	if apiResp.StatusCode() != 201 {
		resp.Diagnostics.AddError("Error", "failed")
	}
}

// --- BAD: Read without timeout ---

func (r *badResource) Read(ctx context.Context, req readRequest, resp *readResponse) { // want "Read method must wrap context"
	apiResp, _ := r.client.GetResourceWithResponse(ctx, "id")
	if apiResp.StatusCode() != 200 {
		resp.Diagnostics.AddError("Error", "failed")
	}
}

// --- GOOD: Create with timeout ---

type goodResource struct{ client *myClient }

func (r *goodResource) Create(ctx context.Context, req createRequest, resp *createResponse) {
	ctx, cancel := context.WithTimeout(ctx, 0)
	defer cancel()

	apiResp, _ := r.client.CreateResourceWithResponse(ctx, nil)
	if apiResp.StatusCode() != 201 {
		resp.Diagnostics.AddError("Error", "failed")
	}
}

func (r *goodResource) Read(ctx context.Context, req readRequest, resp *readResponse) {
	ctx, cancel := context.WithTimeout(ctx, 0)
	defer cancel()

	apiResp, _ := r.client.GetResourceWithResponse(ctx, "id")
	if apiResp.StatusCode() != 200 {
		resp.Diagnostics.AddError("Error", "failed")
	}
}

func (r *goodResource) Update(ctx context.Context, req updateRequest, resp *updateResponse) {
	ctx, cancel := context.WithTimeout(ctx, 0)
	defer cancel()

	apiResp, _ := r.client.CreateResourceWithResponse(ctx, nil)
	if apiResp.StatusCode() != 200 {
		resp.Diagnostics.AddError("Error", "failed")
	}
}

func (r *goodResource) Delete(ctx context.Context, req deleteRequest, resp *deleteResponse) {
	ctx, cancel := context.WithTimeout(ctx, 0)
	defer cancel()

	apiResp, _ := r.client.DeleteResourceWithResponse(ctx, "id")
	if apiResp.StatusCode() != 200 && apiResp.StatusCode() != 404 {
		resp.Diagnostics.AddError("Error", "failed")
	}
}

// --- GOOD: No-op Delete (no API call, no timeout needed) ---

type noopResource struct{}

func (r *noopResource) Delete(ctx context.Context, req deleteRequest, resp *deleteResponse) {
	resp.Diagnostics.AddWarning("Not Deleted", "no-op")
}

// --- Not a method — should be ignored ---

func Create(ctx context.Context, req createRequest, resp *createResponse) {
	apiResp := httpResponse{}
	if apiResp.StatusCode() != 200 {
		resp.Diagnostics.AddError("Error", "failed")
	}
}

// --- Timeouts default must be a named constant ---

type timeoutsValue struct{}

func (t timeoutsValue) Create(ctx context.Context, d time.Duration) (time.Duration, diagList) {
	return d, diagList{}
}
func (t timeoutsValue) Read(ctx context.Context, d time.Duration) (time.Duration, diagList) {
	return d, diagList{}
}
func (t timeoutsValue) Update(ctx context.Context, d time.Duration) (time.Duration, diagList) {
	return d, diagList{}
}
func (t timeoutsValue) Delete(ctx context.Context, d time.Duration) (time.Duration, diagList) {
	return d, diagList{}
}

type modelWithTimeouts struct{ Timeouts timeoutsValue }

const (
	DefaultCreateTimeout = 5 * time.Minute
	DefaultReadTimeout   = 5 * time.Minute
	DefaultUpdateTimeout = 5 * time.Minute
	DefaultDeleteTimeout = 5 * time.Minute
)

// BAD: literal durations as the default.
func literalTimeoutDefaults(ctx context.Context) {
	var data modelWithTimeouts

	_, _ = data.Timeouts.Create(ctx, 5*time.Minute) // want "Timeouts.Create must use the DefaultCreateTimeout constant"
	_, _ = data.Timeouts.Read(ctx, 2*time.Minute)   // want "Timeouts.Read must use the DefaultReadTimeout constant"
	_, _ = data.Timeouts.Update(ctx, 5*time.Minute) // want "Timeouts.Update must use the DefaultUpdateTimeout constant"
	_, _ = data.Timeouts.Delete(ctx, 2*time.Minute) // want "Timeouts.Delete must use the DefaultDeleteTimeout constant"
	_, _ = data.Timeouts.Read(ctx, 0)               // want "Timeouts.Read must use the DefaultReadTimeout constant"
}

// GOOD: named constants as the default.
func namedTimeoutDefaults(ctx context.Context) {
	var data modelWithTimeouts

	_, _ = data.Timeouts.Create(ctx, DefaultCreateTimeout)
	_, _ = data.Timeouts.Read(ctx, DefaultReadTimeout)
	_, _ = data.Timeouts.Update(ctx, DefaultUpdateTimeout)
	_, _ = data.Timeouts.Delete(ctx, DefaultDeleteTimeout)
}

// BAD: a variable holding a literal duration is not a named constant.
func variableTimeoutDefaults(ctx context.Context) {
	var data modelWithTimeouts

	timeout := 2 * time.Minute
	_, _ = data.Timeouts.Read(ctx, timeout) // want "Timeouts.Read must use the DefaultReadTimeout constant"

	var deadline time.Duration = 5 * time.Minute
	_, _ = data.Timeouts.Create(ctx, deadline) // want "Timeouts.Create must use the DefaultCreateTimeout constant"
}

// BAD: a constant that is not this operation's default.
func mismatchedTimeoutDefaults(ctx context.Context) {
	var data modelWithTimeouts

	// A locally declared literal is still a literal.
	const short = 2 * time.Minute
	_, _ = data.Timeouts.Read(ctx, short) // want "Timeouts.Read must use the DefaultReadTimeout constant"

	// Another operation's default — an easy copy-paste error between CRUD methods.
	_, _ = data.Timeouts.Read(ctx, DefaultCreateTimeout) // want "Timeouts.Read must use the DefaultReadTimeout constant"
	_, _ = data.Timeouts.Delete(ctx, DefaultReadTimeout) // want "Timeouts.Delete must use the DefaultDeleteTimeout constant"
}
