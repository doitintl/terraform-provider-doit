package validatordefer_test

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type attributeValidator struct{}

func (attributeValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsUnknown() || req.ConfigValue.IsNull() {
		resp.Diagnostics.AddAttributeError(req.Path, "invalid", "invalid") // want "validator diagnostic may be emitted while req.ConfigValue is unknown"
	}

	unknown := req.ConfigValue.IsUnknown()
	if unknown {
		resp.Diagnostics.AddAttributeWarning(req.Path, "invalid", "invalid") // want "validator diagnostic may be emitted while req.ConfigValue is unknown"
	}
}

type dataSourceValidator struct{}

func (*dataSourceValidator) ValidateConfig(_ context.Context, _ datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	value := types.StringUnknown()
	present := !value.IsNull()
	if present {
		resp.Diagnostics.AddError("invalid", "invalid") // want "validator diagnostic may be emitted while value is unknown"
	}

	directValue := types.StringUnknown()
	if !directValue.IsNull() {
		resp.Diagnostics.AddError("invalid", "invalid") // want "validator diagnostic may be emitted while directValue is unknown"
	}
}

type resourceValidator struct{}

func (*resourceValidator) ValidateConfig(_ context.Context, _ resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	validateOwners([]testItem{{Role: types.StringUnknown()}}, &resp.Diagnostics)
	validateSafely(types.StringUnknown(), &resp.Diagnostics)
}

type guardedHelperValidator struct{}

func (*guardedHelperValidator) ValidateConfig(_ context.Context, _ resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	value := types.StringUnknown()
	if value.IsUnknown() {
		return
	}
	validateGuardedPresence(&resp.Diagnostics, value)
}

func validateGuardedPresence(diagnostics *diag.Diagnostics, value types.String) {
	validateNestedGuardedPresence(value, diagnostics)
}

func validateNestedGuardedPresence(value types.String, diagnostics *diag.Diagnostics) {
	present := !value.IsNull()
	if present {
		diagnostics.AddError("known", "known")
	}
}

type nestedBranchValidator struct{}

func (*nestedBranchValidator) ValidateConfig(_ context.Context, _ resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	value := types.StringUnknown()
	enabled := true
	if value.IsUnknown() || enabled {
		if !value.IsUnknown() {
			resp.Diagnostics.AddError("known", "known")
			if !value.IsNull() {
				resp.Diagnostics.AddWarning("known", "known")
			}
		}
	}
}

type falseBranchValidator struct{}

func (*falseBranchValidator) ValidateConfig(_ context.Context, _ resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	value := types.StringUnknown()
	if value.IsUnknown() {
		return
	} else {
		if !value.IsNull() {
			resp.Diagnostics.AddError("known", "known")
		}
	}
}

type safeElseIfValidator struct{}

func (*safeElseIfValidator) ValidateConfig(_ context.Context, _ resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	value := types.StringUnknown()
	if value.IsUnknown() {
		return
	} else if !value.IsNull() {
		resp.Diagnostics.AddError("known", "known")
	}
}

type unsafeElseIfHelperValidator struct{}

func (*unsafeElseIfHelperValidator) ValidateConfig(_ context.Context, _ resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	value := types.StringUnknown()
	enabled := false
	if enabled {
		return
	} else if !value.IsNull() {
		validateElseIfPresence(value, &resp.Diagnostics)
	}
}

func validateElseIfPresence(value types.String, diagnostics *diag.Diagnostics) {
	if !value.IsNull() {
		diagnostics.AddError("invalid", "invalid") // want "validator diagnostic may be emitted while value is unknown"
	}
}

type exitingConditionValidator struct{}

func (*exitingConditionValidator) ValidateConfig(_ context.Context, _ resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	unsafeValue := types.StringUnknown()
	skip := false
	if unsafeValue.IsUnknown() && skip {
		return
	}
	if !unsafeValue.IsNull() {
		resp.Diagnostics.AddError("invalid", "invalid") // want "validator diagnostic may be emitted while unsafeValue is unknown"
	}

	left := types.StringUnknown()
	right := types.StringUnknown()
	if left.IsUnknown() || right.IsUnknown() {
		return
	}
	if !left.IsNull() && !right.IsNull() {
		resp.Diagnostics.AddWarning("known", "known")
	}
}

type guardedSharedHelperValidator struct{}

func (*guardedSharedHelperValidator) ValidateConfig(_ context.Context, _ resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	value := types.StringUnknown()
	if value.IsUnknown() {
		return
	}
	validateSharedPresence(value, &resp.Diagnostics)
}

type unguardedSharedHelperValidator struct{}

func (*unguardedSharedHelperValidator) ValidateConfig(_ context.Context, _ resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	validateSharedPresence(types.StringUnknown(), &resp.Diagnostics)
}

func validateSharedPresence(value types.String, diagnostics *diag.Diagnostics) {
	if !value.IsNull() {
		diagnostics.AddError("invalid", "invalid") // want "validator diagnostic may be emitted while value is unknown"
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

func validateSafely(value types.String, diagnostics *diag.Diagnostics) {
	if value.IsUnknown() {
		return
	}
	if value.IsNull() {
		diagnostics.AddError("invalid", "invalid")
	}

	if !value.IsNull() {
		diagnostics.AddWarning("known", "known")
	}

	ownerCount := 0
	unknownCount := 0
	items := []testItem{{Role: value}}
	for _, item := range items {
		if item.Role.IsUnknown() {
			unknownCount++
			continue
		}
		presentRole := !item.Role.IsNull()
		if presentRole {
			diagnostics.AddWarning("known role", "known role")
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
