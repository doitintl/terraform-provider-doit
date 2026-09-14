package validatorshape_test

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type goodAttribute struct{}

func (goodAttribute) ValidateString(_ context.Context, req validator.StringRequest, _ *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
}

type goodResource struct{}

func (*goodResource) ValidateConfig(_ context.Context, _ resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	value := types.StringUnknown()
	if resp.Diagnostics.HasError() || value.IsUnknown() {
		return
	}
	if !value.IsUnknown() && !value.IsNull() {
		resp.Diagnostics.AddError("known", "known")
	}
	validateGoodHelper(value)
}

func validateGoodHelper(value types.String) {
	if value.IsUnknown() {
		return
	}
}

type goodDataSource struct{}

func (*goodDataSource) ValidateConfig(_ context.Context, _ datasource.ValidateConfigRequest, _ *datasource.ValidateConfigResponse) {
	items := []types.String{types.StringUnknown()}
	unknownCount := 0
	for _, item := range items {
		if item.IsUnknown() {
			unknownCount++
			continue
		}
	}
	_ = unknownCount
}

type goodProvider struct{}

func (*goodProvider) ValidateConfig(_ context.Context, _ provider.ValidateConfigRequest, _ *provider.ValidateConfigResponse) {
	items := []types.String{types.StringUnknown()}
	hasUnknown := false
	for _, item := range items {
		if item.IsUnknown() {
			hasUnknown = true
			continue
		}
	}
	_ = hasUnknown
}

type customValue struct {
	types.String
}

type goodCustomValue struct{}

func (*goodCustomValue) ValidateConfig(_ context.Context, _ resource.ValidateConfigRequest, _ *resource.ValidateConfigResponse) {
	value := customValue{String: types.StringUnknown()}
	if value.IsUnknown() {
		return
	}
	if value.IsNull() {
		return
	}
}

type badKnownPredicateAlias struct{}

func (*badKnownPredicateAlias) ValidateConfig(_ context.Context, _ resource.ValidateConfigRequest, _ *resource.ValidateConfigResponse) {
	value := customValue{String: types.StringUnknown()}
	if value.IsUnknown() {
		return
	}
	missing := value.IsNull() // want "do not store IsUnknown or IsNull results"
	switch {
	case missing:
		return
	}
}

type badUnguardedAccessor struct{}

func (*badUnguardedAccessor) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.ValueString() == "" { // want "Terraform value accessor ValueString requires a dominating unknown guard"
		resp.Diagnostics.AddAttributeError(req.Path, "invalid", "invalid")
	}
}

type badReassignedAccessor struct{}

func (*badReassignedAccessor) ValidateConfig(_ context.Context, _ resource.ValidateConfigRequest, _ *resource.ValidateConfigResponse) {
	value := types.StringUnknown()
	if value.IsUnknown() {
		return
	}
	value = types.StringUnknown()
	_ = value.ValueString() // want "Terraform value accessor ValueString requires a dominating unknown guard"
}

type reassignedState struct {
	value   types.String
	sibling types.String
}

type badReassignedSelectorAccessor struct{}

func (*badReassignedSelectorAccessor) ValidateConfig(_ context.Context, _ resource.ValidateConfigRequest, _ *resource.ValidateConfigResponse) {
	state := reassignedState{value: types.StringUnknown()}
	if state.value.IsUnknown() {
		return
	}
	state.value = types.StringUnknown()
	_ = state.value.ValueString() // want "Terraform value accessor ValueString requires a dominating unknown guard"
}

type badReassignedRootAccessor struct{}

func (*badReassignedRootAccessor) ValidateConfig(_ context.Context, _ resource.ValidateConfigRequest, _ *resource.ValidateConfigResponse) {
	state := reassignedState{value: types.StringUnknown()}
	if state.value.IsUnknown() {
		return
	}
	state = reassignedState{value: types.StringUnknown()}
	_ = state.value.ValueString() // want "Terraform value accessor ValueString requires a dominating unknown guard"
}

type goodReassignedAndReguardedAccessor struct{}

func (*goodReassignedAndReguardedAccessor) ValidateConfig(_ context.Context, _ resource.ValidateConfigRequest, _ *resource.ValidateConfigResponse) {
	value := types.StringUnknown()
	if value.IsUnknown() {
		return
	}
	value = types.StringUnknown()
	if value.IsUnknown() {
		return
	}
	_ = value.ValueString()
}

type goodSiblingReassignmentAccessor struct{}

func (*goodSiblingReassignmentAccessor) ValidateConfig(_ context.Context, _ resource.ValidateConfigRequest, _ *resource.ValidateConfigResponse) {
	state := reassignedState{value: types.StringUnknown(), sibling: types.StringUnknown()}
	if state.sibling.IsUnknown() {
		return
	}
	state.value = types.StringUnknown()
	_ = state.sibling.ValueString()
}

type badDistinctConstantIndexAccessor struct{}

func (*badDistinctConstantIndexAccessor) ValidateConfig(_ context.Context, _ resource.ValidateConfigRequest, _ *resource.ValidateConfigResponse) {
	values := []types.String{types.StringUnknown(), types.StringUnknown()}
	if values[0].IsUnknown() {
		return
	}
	_ = values[1].ValueString() // want "Terraform value accessor ValueString requires a dominating unknown guard"
}

type goodSameConstantIndexAccessor struct{}

func (*goodSameConstantIndexAccessor) ValidateConfig(_ context.Context, _ resource.ValidateConfigRequest, _ *resource.ValidateConfigResponse) {
	values := []types.String{types.StringUnknown()}
	if values[0].IsUnknown() {
		return
	}
	_ = values[0].ValueString()
}

type badDistinctVariableIndexAccessor struct{}

func (*badDistinctVariableIndexAccessor) ValidateConfig(_ context.Context, _ resource.ValidateConfigRequest, _ *resource.ValidateConfigResponse) {
	values := []types.String{types.StringUnknown(), types.StringUnknown()}
	guardedIndex := 0
	accessedIndex := 1
	if values[guardedIndex].IsUnknown() {
		return
	}
	_ = values[accessedIndex].ValueString() // want "Terraform value accessor ValueString requires a dominating unknown guard"
}

type goodSameVariableIndexAccessor struct{}

func (*goodSameVariableIndexAccessor) ValidateConfig(_ context.Context, _ resource.ValidateConfigRequest, _ *resource.ValidateConfigResponse) {
	values := []types.String{types.StringUnknown()}
	index := 0
	if values[index].IsUnknown() {
		return
	}
	_ = values[index].ValueString()
}

type badReassignedVariableIndexAccessor struct{}

func (*badReassignedVariableIndexAccessor) ValidateConfig(_ context.Context, _ resource.ValidateConfigRequest, _ *resource.ValidateConfigResponse) {
	values := []types.String{types.StringUnknown(), types.StringUnknown()}
	index := 0
	if values[index].IsUnknown() {
		return
	}
	index = 1
	_ = values[index].ValueString() // want "Terraform value accessor ValueString requires a dominating unknown guard"
}

type goodGuardedBusinessCondition struct{}

func (*goodGuardedBusinessCondition) ValidateConfig(_ context.Context, _ resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	value := types.StringUnknown()
	enabled := true
	if enabled && value.IsNull() {
		resp.Diagnostics.AddError("known null", "known null")
	}
	if value.IsNull() || (!value.IsUnknown() && value.ValueString() == "") {
		resp.Diagnostics.AddError("known null or empty", "known null or empty")
	}
}

type badPositiveBranch struct{}

func (*badPositiveBranch) ValidateConfig(_ context.Context, _ resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	value := types.StringUnknown()
	if value.IsUnknown() { // want "positive IsUnknown checks must return, continue, or record uncertainty and continue"
		resp.Diagnostics.AddError("invalid", "invalid")
	}
}

type badMixedGuard struct{}

func (*badMixedGuard) ValidateConfig(_ context.Context, _ resource.ValidateConfigRequest, _ *resource.ValidateConfigResponse) {
	value := types.StringUnknown()
	enabled := true
	if value.IsUnknown() || enabled { // want "IsUnknown must be used in a direct \\|\\|-joined deferral guard without business conditions"
		return
	}
}

type badMixedNullBusiness struct{}

func (*badMixedNullBusiness) ValidateConfig(_ context.Context, _ resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	value := types.StringUnknown()
	if value.IsNull() || value.ValueString() == "" { // want "IsNull cannot share a diagnostic condition with business logic until the value is known"
		resp.Diagnostics.AddError("invalid", "invalid")
	}
}

type fakeDiagnostics struct{}

func (fakeDiagnostics) HasError() bool { return false }

type badSameNamedDiagnostics struct{}

func (*badSameNamedDiagnostics) ValidateConfig(_ context.Context, _ resource.ValidateConfigRequest, _ *resource.ValidateConfigResponse) {
	value := types.StringUnknown()
	fake := fakeDiagnostics{}
	if fake.HasError() || value.IsUnknown() { // want "IsUnknown must be used in a direct \\|\\|-joined deferral guard"
		return
	}
}

type badSwitch struct{}

func (*badSwitch) ValidateConfig(_ context.Context, _ resource.ValidateConfigRequest, _ *resource.ValidateConfigResponse) {
	value := types.StringUnknown()
	switch {
	case value.IsUnknown(): // want "Terraform value predicate uses unsupported validator control flow"
		return
	}
}

type badAlias struct{}

func (*badAlias) ValidateConfig(_ context.Context, _ resource.ValidateConfigRequest, _ *resource.ValidateConfigResponse) {
	value := types.StringUnknown()
	unknown := value.IsUnknown() // want "do not store IsUnknown or IsNull results"
	if unknown {
		return
	}
}

type badNullAlias struct{}

func (*badNullAlias) ValidateConfig(_ context.Context, _ resource.ValidateConfigRequest, _ *resource.ValidateConfigResponse) {
	value := types.StringUnknown()
	present := !value.IsNull() // want "do not store IsUnknown or IsNull results"
	_ = present
}

type badClosure struct{}

func (*badClosure) ValidateConfig(_ context.Context, _ resource.ValidateConfigRequest, _ *resource.ValidateConfigResponse) {
	value := types.StringUnknown()
	_ = func() bool {
		return value.IsUnknown() // want "Terraform value predicates in validator closures are unsupported"
	}
}

type badConditionalTracker struct{}

func (*badConditionalTracker) ValidateConfig(_ context.Context, _ resource.ValidateConfigRequest, _ *resource.ValidateConfigResponse) {
	items := []types.String{types.StringUnknown()}
	hasUnknown := false
	for _, item := range items {
		if item.IsUnknown() { // want "positive IsUnknown checks must return, continue, or record uncertainty and continue"
			if len(items) > 1 {
				hasUnknown = true
			}
			continue
		}
	}
	_ = hasUnknown
}

type unrelated struct{}

func (unrelated) IsUnknown() bool { return false }

func validateUnrelated(_ types.String, _ *diag.Diagnostics) {
	value := unrelated{}
	if value.IsUnknown() {
		return
	}
}
