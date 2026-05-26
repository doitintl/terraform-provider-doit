package deletetest

import "context"

// Stubs
type deleteRequest struct{}
type deleteResponse struct{ Diagnostics diagList }
type diagList struct{}

func (d diagList) Append(diags ...interface{})             {}
func (d diagList) HasError() bool                          { return false }
func (d diagList) AddError(summary, detail string)         {}
func (d diagList) AddWarning(summary, detail string)       {}

type stateObj struct{}
func (s stateObj) Get(ctx context.Context, v interface{}) diagList { return diagList{} }

type httpResponse struct{ Body []byte }
func (r httpResponse) StatusCode() int { return 200 }

// --- BAD: Delete doesn't handle 404 ---

type badResource struct{}

func (r *badResource) Delete(ctx context.Context, req deleteRequest, resp *deleteResponse) {
	deleteResp := httpResponse{}

	if deleteResp.StatusCode() != 200 && deleteResp.StatusCode() != 204 { // want "Delete method must treat 404 as success"
		resp.Diagnostics.AddError("Error", "failed")
	}
}

// --- BAD: Delete only checks != 200 ---

type badResource2 struct{}

func (r *badResource2) Delete(ctx context.Context, req deleteRequest, resp *deleteResponse) {
	deleteResp := httpResponse{}

	if deleteResp.StatusCode() != 200 { // want "Delete method must treat 404 as success"
		resp.Diagnostics.AddError("Error", "failed")
	}
}

// --- GOOD: Delete handles 404 with != ---

type goodResource struct{}

func (r *goodResource) Delete(ctx context.Context, req deleteRequest, resp *deleteResponse) {
	deleteResp := httpResponse{}

	if deleteResp.StatusCode() != 200 && deleteResp.StatusCode() != 204 && deleteResp.StatusCode() != 404 {
		resp.Diagnostics.AddError("Error", "failed")
	}
}

// --- GOOD: Delete handles 404 with == (early return) ---

type goodResource2 struct{}

func (r *goodResource2) Delete(ctx context.Context, req deleteRequest, resp *deleteResponse) {
	deleteResp := httpResponse{}

	if deleteResp.StatusCode() == 404 {
		return // already gone
	}

	if deleteResp.StatusCode() != 200 {
		resp.Diagnostics.AddError("Error", "failed")
	}
}

// --- GOOD: No-op Delete (no API call, no StatusCode check) ---

type noopResource struct{}

func (r *noopResource) Delete(ctx context.Context, req deleteRequest, resp *deleteResponse) {
	resp.Diagnostics.AddWarning("Not Deleted", "resource does not support deletion via API")
}

// --- Not a method (no receiver) — should be ignored ---

func Delete(ctx context.Context, req deleteRequest, resp *deleteResponse) {
	deleteResp := httpResponse{}
	if deleteResp.StatusCode() != 200 {
		resp.Diagnostics.AddError("Error", "failed")
	}
}
