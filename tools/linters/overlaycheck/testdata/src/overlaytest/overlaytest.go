package overlaytest

import "github.com/hashicorp/terraform-plugin-framework/types"

// --- GOOD: correct overlay ---

func overlayGoodComputedFields(apiResp *ApiResponse, plan *GoodModel) {
	var resolved GoodModel

	// Computed-only: unconditional assignment. ✓
	plan.Id = resolved.Id
	plan.CreateTime = resolved.CreateTime

	// Required ("name"): not mentioned. ✓
	// Optional ("description"): not mentioned. ✓

	// Optional+Computed: guarded by IsUnknown. ✓
	if plan.Amount.IsUnknown() {
		plan.Amount = resolved.Amount
	}
	if plan.Currency.IsUnknown() {
		plan.Currency = resolved.Currency
	}
}

// --- BAD: missing Computed-only field "create_time" ---

func overlayBadMissingComputedFields(apiResp *ApiResponse, plan *BadMissingModel) { // want `overlayBadMissingComputedFields: Computed-only field\(s\) not set from API response: create_time`
	var resolved BadMissingModel

	// Only sets "id", but "create_time" is missing.
	plan.Id = resolved.Id

	if plan.Amount.IsUnknown() {
		plan.Amount = resolved.Amount
	}
}

// --- BAD: unconditionally assigns Optional+Computed ---

func overlayBadUnconditionalComputedFields(apiResp *ApiResponse, plan *BadUnconditionalModel) {
	var resolved BadUnconditionalModel

	plan.Id = resolved.Id

	// BAD: amount is Optional+Computed but assigned unconditionally.
	plan.Amount = resolved.Amount // want `overlayBadUnconditionalComputedFields: Optional\+Computed field "amount" is assigned unconditionally`
}

// --- BAD: assigns to Required field ---

func overlayBadRequiredComputedFields(apiResp *ApiResponse, plan *BadRequiredModel) {
	var resolved BadRequiredModel

	plan.Id = resolved.Id

	// BAD: name is Required but is being assigned.
	plan.Name = resolved.Name // want `overlayBadRequiredComputedFields: Required field "name" must not be assigned in overlay`
}

// --- GOOD: if/else covering both branches is unconditional ---

func overlayIfElseComputedFields(apiResp *Int64Pointer, plan *IfElseModel) {
	var resolved IfElseModel

	plan.Id = resolved.Id

	// Computed-only: if/else covering both branches = unconditional. ✓
	if apiResp.Value != nil {
		plan.CreateTime = types.Int64Value(*apiResp.Value)
	} else {
		plan.CreateTime = types.Int64Null()
	}
	if apiResp.Value != nil {
		plan.UpdateTime = types.Int64Value(*apiResp.Value)
	} else {
		plan.UpdateTime = types.Int64Null()
	}
}

// --- GOOD: Required nested object with IsUnknown guard ---

func overlayRequiredNestedComputedFields(apiResp *ApiResponse, plan *RequiredNestedModel) {
	var resolved RequiredNestedModel

	plan.Id = resolved.Id

	// Required nested object with Optional+Computed children: IsUnknown guard is OK. ✓
	if plan.Config.IsUnknown() {
		plan.Config = resolved.Config
	}
}
