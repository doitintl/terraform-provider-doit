package readtest

import "context"

// Stubs
type readRequest struct{ State stateObj }
type readResponse struct{ Diagnostics diagList; State stateObj }
type diagList struct{}

func (d diagList) Append(diags ...interface{})     {}
func (d diagList) HasError() bool                  { return false }
func (d diagList) AddError(summary, detail string) {}

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

// --- BAD: Read without RemoveResource ---

type badResource struct{ client *myClient }

func (r *badResource) Read(ctx context.Context, req readRequest, resp *readResponse) { // want "Read method must call resp.State.RemoveResource"
	apiResp, _ := r.client.GetResourceWithResponse(ctx, "id")
	if apiResp.StatusCode() != 200 {
		resp.Diagnostics.AddError("Error", "not found")
	}
}

// --- GOOD: Read with direct RemoveResource ---

type goodResource struct{ client *myClient }

func (r *goodResource) Read(ctx context.Context, req readRequest, resp *readResponse) {
	apiResp, _ := r.client.GetResourceWithResponse(ctx, "id")

	if apiResp.StatusCode() == 404 {
		resp.State.RemoveResource(ctx)
		return
	}

	if apiResp.StatusCode() != 200 {
		resp.Diagnostics.AddError("Error", "failed")
	}
}

// --- GOOD: Read with indirect pattern (populateState + RemoveResource) ---

type indirectResource struct{ client *myClient }

func (r *indirectResource) populateState(ctx context.Context, state *myModel) diagList {
	return diagList{}
}

func (r *indirectResource) Read(ctx context.Context, req readRequest, resp *readResponse) {
	var state myModel
	r.populateState(ctx, &state)

	// Check null ID → RemoveResource
	resp.State.RemoveResource(ctx)
}

// --- Not a method — should be ignored ---

func Read(ctx context.Context, req readRequest, resp *readResponse) {
	apiResp := httpResponse{}
	if apiResp.StatusCode() != 200 {
		resp.Diagnostics.AddError("Error", "failed")
	}
}
