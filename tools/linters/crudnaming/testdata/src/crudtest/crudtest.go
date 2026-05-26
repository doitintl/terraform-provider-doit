package crudtest

import "context"

type myModel struct{}

type stateProvider struct{}

func (s stateProvider) Get(ctx context.Context, target interface{}) {}

type request struct {
	Plan  stateProvider
	State stateProvider
}

// BAD: "state" from req.Plan.Get — wrong source name
func badPlanWithStateName(ctx context.Context) {
	var state myModel
	req := request{}
	req.Plan.Get(ctx, &state) // want `variable "state" is populated from req.Plan.Get\(\) but name suggests state; use "plan" instead`
}

// BAD: "plan" from req.State.Get — wrong source name
func badStateWithPlanName(ctx context.Context) {
	var plan myModel
	req := request{}
	req.State.Get(ctx, &plan) // want `variable "plan" is populated from req.State.Get\(\) but name suggests plan; use "state" instead`
}

// GOOD: "plan" from req.Plan.Get — correct name
func goodPlan(ctx context.Context) {
	var plan myModel
	req := request{}
	req.Plan.Get(ctx, &plan)
}

// GOOD: "state" from req.State.Get — correct name
func goodState(ctx context.Context) {
	var state myModel
	req := request{}
	req.State.Get(ctx, &state)
}

// GOOD: "oldState" from req.State.Get — acceptable variant (contains "state")
func goodOldState(ctx context.Context) {
	var oldState myModel
	req := request{}
	req.State.Get(ctx, &oldState)
}

// GOOD: "data" from req.Plan.Get — neutral name, no wrong-source signal
func goodNeutral(ctx context.Context) {
	var data myModel
	req := request{}
	req.Plan.Get(ctx, &data)
}
