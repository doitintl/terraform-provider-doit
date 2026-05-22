package crudtest

import "context"

type myModel struct{}
type myResource struct{}

type createReq struct{}
type createResp struct{}
type readReq struct{}
type readResp struct{}

// BAD: Create uses "state"
func (r *myResource) Create(ctx context.Context, req createReq, resp *createResp) {
	var state myModel // want `Create method should use "plan" instead of "state"`
	_ = state
}

// GOOD: Create uses "plan"
func (r *myResource) Update(ctx context.Context, req createReq, resp *createResp) {
	var plan myModel
	_ = plan
}

// BAD: Read uses "plan"
func (r *myResource) Read(ctx context.Context, req readReq, resp *readResp) {
	var plan myModel // want `Read method should use "state" instead of "plan"`
	_ = plan
}

// GOOD: Delete uses "state"
func (r *myResource) Delete(ctx context.Context, req readReq, resp *readResp) {
	var state myModel
	_ = state
}
