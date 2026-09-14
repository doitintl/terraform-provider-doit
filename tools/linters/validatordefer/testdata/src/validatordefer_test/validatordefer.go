package validatordefer_test

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type attributeValidator struct{}

func (attributeValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if !req.ConfigValue.IsNull() {
		resp.Diagnostics.AddAttributeError(req.Path, "invalid", "invalid") // want "validator diagnostic may be emitted while req.ConfigValue is unknown"
	}
}

type dataSourceValidator struct{}

func (*dataSourceValidator) ValidateConfig(_ context.Context, _ datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	value := types.StringUnknown()
	if !value.IsNull() {
		resp.Diagnostics.AddError("invalid", "invalid") // want "validator diagnostic may be emitted while value is unknown"
	}
}

type resourceValidator struct{}

func (*resourceValidator) ValidateConfig(_ context.Context, _ resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	validateOwners([]testItem{{Role: types.StringUnknown()}}, &resp.Diagnostics)
	validateOwnersSafely([]testItem{{Role: types.StringUnknown()}}, &resp.Diagnostics)
	validateKnownIndependent(types.StringUnknown(), types.StringUnknown(), &resp.Diagnostics)
	validateShadowedValue(types.StringUnknown(), &resp.Diagnostics)
	validateKnownPresenceAlternatives(types.StringUnknown(), types.StringUnknown(), &resp.Diagnostics)
	validateUnrelatedDiagnostics(types.StringUnknown())
	validateOwnersWithInterveningStatement([]testItem{{Role: types.StringUnknown()}}, &resp.Diagnostics)
	validateNestedCollection([][]testItem{{{Role: types.StringUnknown()}}}, &resp.Diagnostics)
	validateSelectorTracker([]testItem{{Role: types.StringUnknown()}}, &resp.Diagnostics)
	validateSelectorTrackerSafely([]testItem{{Role: types.StringUnknown()}}, &resp.Diagnostics)
	validateNullFallthrough(types.StringUnknown(), &resp.Diagnostics)
	validateNullElse(types.StringUnknown(), &resp.Diagnostics)
	validateNullFallthroughSafely(types.StringUnknown(), &resp.Diagnostics)
	validateNullElseSafely(types.StringUnknown(), &resp.Diagnostics)
	validateBusinessGatedNullExit(false, types.StringUnknown(), &resp.Diagnostics)
	validateWrongBooleanTracker([]testItem{{Role: types.StringUnknown()}}, &resp.Diagnostics)
	validateWrongCounterTracker([]testItem{{Role: types.StringUnknown()}}, &resp.Diagnostics)
	validateWrongTrackerDisjunction([]testItem{{Role: types.StringUnknown()}}, &resp.Diagnostics)
	validateFalseEqualityTrackerSafely([]testItem{{Role: types.StringUnknown()}}, &resp.Diagnostics)
	validateReverseCounterEqualitySafely([]testItem{{Role: types.StringUnknown()}}, &resp.Diagnostics)
}

func validateWrongBooleanTracker(items []testItem, diagnostics *diag.Diagnostics) {
	ownerCount := 0
	hasUnknown := false
	for _, item := range items {
		if item.Role.IsUnknown() {
			hasUnknown = true
			continue
		}
		if item.Role.ValueString() == "owner" {
			ownerCount++
		}
	}
	if ownerCount == 0 && hasUnknown {
		diagnostics.AddError("missing", "missing") // want "validator diagnostic may be emitted while item.Role is unknown"
	}
}

func validateWrongCounterTracker(items []testItem, diagnostics *diag.Diagnostics) {
	ownerCount := 0
	unknownCount := 0
	for _, item := range items {
		if item.Role.IsUnknown() {
			unknownCount++
			continue
		}
		if item.Role.ValueString() == "owner" {
			ownerCount++
		}
	}
	if ownerCount == 0 && unknownCount > 0 {
		diagnostics.AddError("missing", "missing") // want "validator diagnostic may be emitted while item.Role is unknown"
	}
}

func validateWrongTrackerDisjunction(items []testItem, diagnostics *diag.Diagnostics) {
	ownerCount := 0
	hasUnknown := false
	for _, item := range items {
		if item.Role.IsUnknown() {
			hasUnknown = true
			continue
		}
		if item.Role.ValueString() == "owner" {
			ownerCount++
		}
	}
	if ownerCount == 0 || !hasUnknown {
		diagnostics.AddError("missing", "missing") // want "validator diagnostic may be emitted while item.Role is unknown"
	}
}

func validateFalseEqualityTrackerSafely(items []testItem, diagnostics *diag.Diagnostics) {
	ownerCount := 0
	hasUnknown := false
	for _, item := range items {
		if item.Role.IsUnknown() {
			hasUnknown = true
			continue
		}
		if item.Role.ValueString() == "owner" {
			ownerCount++
		}
	}
	if ownerCount == 0 && false == hasUnknown {
		diagnostics.AddError("missing", "missing")
	}
}

func validateReverseCounterEqualitySafely(items []testItem, diagnostics *diag.Diagnostics) {
	ownerCount := 0
	unknownCount := 0
	for _, item := range items {
		if item.Role.IsUnknown() {
			unknownCount++
			continue
		}
		if item.Role.ValueString() == "owner" {
			ownerCount++
		}
	}
	if ownerCount == 0 && 0 == unknownCount {
		diagnostics.AddError("missing", "missing")
	}
}

func validateNullFallthrough(value types.String, diagnostics *diag.Diagnostics) {
	if value.IsNull() {
		return
	}
	diagnostics.AddError("present", "present") // want "validator diagnostic may be emitted while value is unknown"
}

func validateNullElse(value types.String, diagnostics *diag.Diagnostics) {
	if value.IsNull() {
		return
	} else {
		diagnostics.AddError("present", "present") // want "validator diagnostic may be emitted while value is unknown"
	}
}

func validateNullFallthroughSafely(value types.String, diagnostics *diag.Diagnostics) {
	if value.IsNull() || value.IsUnknown() {
		return
	}
	diagnostics.AddError("present", "present")
}

func validateNullElseSafely(value types.String, diagnostics *diag.Diagnostics) {
	if value.IsNull() {
		return
	} else {
		if value.IsUnknown() {
			return
		}
		diagnostics.AddError("present", "present")
	}
}

func validateBusinessGatedNullExit(enabled bool, value types.String, diagnostics *diag.Diagnostics) {
	if enabled && value.IsNull() {
		return
	}
	if !enabled {
		diagnostics.AddError("disabled", "disabled")
	}
}

type selectorTrackerState struct {
	hasUnknown bool
	ownerCount int
}

func validateSelectorTracker(items []testItem, diagnostics *diag.Diagnostics) {
	state := selectorTrackerState{}
	ownerCount := 0
	for _, item := range items {
		if item.Role.IsUnknown() {
			state.hasUnknown = true
			continue
		}
		if item.Role.ValueString() == "owner" {
			ownerCount++
		}
	}
	if ownerCount == 0 && state.ownerCount == 0 {
		diagnostics.AddError("missing", "missing") // want "validator diagnostic may be emitted while item.Role is unknown"
	}
}

func validateSelectorTrackerSafely(items []testItem, diagnostics *diag.Diagnostics) {
	state := selectorTrackerState{}
	for _, item := range items {
		if item.Role.IsUnknown() {
			state.hasUnknown = true
			continue
		}
		if item.Role.ValueString() == "owner" {
			state.ownerCount++
		}
	}
	if state.ownerCount == 0 && !state.hasUnknown {
		diagnostics.AddError("missing", "missing")
	}
}

func validateOwnersWithInterveningStatement(items []testItem, diagnostics *diag.Diagnostics) {
	ownerCount := 0
	for _, item := range items {
		if item.Role.IsUnknown() {
			continue
		}
		if item.Role.ValueString() == "owner" {
			ownerCount++
		}
	}
	_ = len(items)
	if ownerCount == 0 {
		diagnostics.AddError("missing", "missing") // want "validator diagnostic may be emitted while item.Role is unknown"
	}
}

func validateNestedCollection(groups [][]testItem, diagnostics *diag.Diagnostics) {
	groupCount := 0
	for _, group := range groups {
		for _, item := range group {
			if item.Role.IsUnknown() {
				continue
			}
		}
		groupCount++
	}
	if groupCount == 0 {
		diagnostics.AddError("missing", "missing")
	}
}

func validateShadowedValue(value types.String, diagnostics *diag.Diagnostics) {
	if value.IsUnknown() {
		return
	}
	{
		value := types.StringUnknown()
		if !value.IsNull() {
			diagnostics.AddError("invalid", "invalid") // want "validator diagnostic may be emitted while value is unknown"
		}
	}
}

func validateKnownPresenceAlternatives(left, right types.String, diagnostics *diag.Diagnostics) {
	if (!left.IsNull() && !left.IsUnknown()) || (!right.IsNull() && !right.IsUnknown()) {
		diagnostics.AddError("known", "known")
	}
}

type unrelatedDiagnostics struct{}

func (unrelatedDiagnostics) AddError(string, string) {}

func validateUnrelatedDiagnostics(value types.String) {
	diagnostics := unrelatedDiagnostics{}
	if !value.IsNull() {
		diagnostics.AddError("not Terraform", "not Terraform")
	}
}

type providerValidator struct{}

func (*providerValidator) ValidateConfig(_ context.Context, _ provider.ValidateConfigRequest, resp *provider.ValidateConfigResponse) {
	value := types.StringUnknown()
	if value.IsUnknown() {
		return
	}
	if !value.IsNull() {
		resp.Diagnostics.AddWarning("known", "known")
	}
}

type callerGuardedHelperValidator struct{}

func (*callerGuardedHelperValidator) ValidateConfig(_ context.Context, _ resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	value := types.StringUnknown()
	if value.IsUnknown() {
		return
	}
	validateCallerGuardedPresence(value, &resp.Diagnostics)
}

func validateCallerGuardedPresence(value types.String, diagnostics *diag.Diagnostics) {
	if !value.IsNull() {
		diagnostics.AddError("invalid", "invalid") // want "validator diagnostic may be emitted while value is unknown"
	}
}

type selfGuardedHelperValidator struct{}

func (*selfGuardedHelperValidator) ValidateConfig(_ context.Context, _ resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	validateSelfGuardedPresence(types.StringUnknown(), &resp.Diagnostics)
}

func validateSelfGuardedPresence(value types.String, diagnostics *diag.Diagnostics) {
	if value.IsUnknown() {
		return
	}
	if !value.IsNull() {
		diagnostics.AddError("known", "known")
	}
}

type testItem struct {
	Role types.String
}

func validateOwners(items []testItem, diagnostics *diag.Diagnostics) {
	ownerCount := 0
	for _, item := range items {
		if item.Role.IsUnknown() {
			continue
		}
		if item.Role.ValueString() == "owner" {
			ownerCount++
		}
	}
	if ownerCount != 1 {
		diagnostics.AddError("missing", "missing") // want "validator diagnostic may be emitted while item.Role is unknown"
	}
}

func validateKnownIndependent(unrelated, invalid types.String, diagnostics *diag.Diagnostics) {
	if invalid.IsNull() {
		diagnostics.AddError("invalid", "invalid")
	}
	if unrelated.IsUnknown() {
		return
	}
}

func validateOwnersSafely(items []testItem, diagnostics *diag.Diagnostics) {
	ownerCount := 0
	unknownCount := 0
	for _, item := range items {
		if item.Role.IsUnknown() {
			unknownCount++
			continue
		}
		if item.Role.ValueString() == "owner" {
			ownerCount++
		}
	}
	if ownerCount == 0 && unknownCount == 0 {
		diagnostics.AddError("missing", "missing")
	}
	if ownerCount > 1 {
		diagnostics.AddError("multiple", "multiple")
	}
}
