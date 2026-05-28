package overlaytest

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Stub mapping functions for test.
func mapGoodToModel(apiResp *ApiResponse, m *GoodModel)             {}
func mapBadMissingToModel(apiResp *ApiResponse, m *BadMissingModel) {}
func mapBadUnconditionalToModel(apiResp *ApiResponse, m *BadUnconditionalModel) {}
func mapBadRequiredToModel(apiResp *ApiResponse, m *BadRequiredModel) {}
func mapIfElseToModel(apiResp *Int64Pointer, m *IfElseModel)        {}
func mapRequiredNestedToModel(apiResp *ApiResponse, m *RequiredNestedModel) {}

// --- GOOD: correct overlay with 2-phase pattern ---

func overlayGoodComputedFields(apiResp *ApiResponse, plan *GoodModel) {
	resolved := *plan
	mapGoodToModel(apiResp, &resolved)

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
	resolved := *plan
	mapBadMissingToModel(apiResp, &resolved)

	// Only sets "id", but "create_time" is missing.
	plan.Id = resolved.Id

	if plan.Amount.IsUnknown() {
		plan.Amount = resolved.Amount
	}
}

// --- BAD: unconditionally assigns Optional+Computed ---

func overlayBadUnconditionalComputedFields(apiResp *ApiResponse, plan *BadUnconditionalModel) {
	resolved := *plan
	mapBadUnconditionalToModel(apiResp, &resolved)

	plan.Id = resolved.Id

	// BAD: amount is Optional+Computed but assigned unconditionally.
	plan.Amount = resolved.Amount // want `overlayBadUnconditionalComputedFields: Optional\+Computed field "amount" is assigned unconditionally`
}

// --- BAD: assigns to Required field ---

func overlayBadRequiredComputedFields(apiResp *ApiResponse, plan *BadRequiredModel) {
	resolved := *plan
	mapBadRequiredToModel(apiResp, &resolved)

	plan.Id = resolved.Id

	// BAD: name is Required but is being assigned.
	plan.Name = resolved.Name // want `overlayBadRequiredComputedFields: Required field "name" must not be assigned in overlay`
}

// --- GOOD: if/else covering both branches is unconditional ---

func overlayIfElseComputedFields(apiResp *Int64Pointer, plan *IfElseModel) {
	resolved := *plan
	mapIfElseToModel(apiResp, &resolved)

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

// --- GOOD: Computed-only nested object assigned unconditionally (no helper needed) ---

func mapComputedOnlyNestedToModel(apiResp *ApiResponse, m *ComputedOnlyNestedModel) {}

func overlayComputedOnlyNestedComputedFields(apiResp *ApiResponse, plan *ComputedOnlyNestedModel) {
	resolved := *plan
	mapComputedOnlyNestedToModel(apiResp, &resolved)

	// Computed-only fields: unconditional.
	plan.Id = resolved.Id
	plan.Summary = resolved.Summary // Computed-only nested — no sub-overlay needed. ✓
}

// --- GOOD: Required nested object handled via sub-overlay helper ---

func overlayRequiredNestedConfig(resolved *ConfigValue, plan *ConfigValue) {
	if plan.Mode.IsUnknown() {
		plan.Mode = resolved.Mode
	}
	if plan.Amount.IsUnknown() {
		plan.Amount = resolved.Amount
	}
}

func overlayRequiredNestedComputedFields(apiResp *ApiResponse, plan *RequiredNestedModel) {
	resolved := *plan
	mapRequiredNestedToModel(apiResp, &resolved)

	plan.Id = resolved.Id

	// Required nested object with Optional+Computed children: handled via helper. ✓
	if plan.Config.IsUnknown() {
		plan.Config = resolved.Config
	} else if !plan.Config.IsNull() {
		overlayRequiredNestedConfig(&resolved.Config, &plan.Config)
	}
}

// --- BAD: Required nested object handled inline (should use helper) ---

func overlayRequiredNestedInlineComputedFields(apiResp *ApiResponse, plan *RequiredNestedModel) { // want `overlayRequiredNestedInlineComputedFields: nested attribute "config" has computed fields that need overlay`
	resolved := *plan
	mapRequiredNestedToModel(apiResp, &resolved)

	plan.Id = resolved.Id

	// INLINE handling — should be flagged. ✗
	if plan.Config.IsUnknown() {
		plan.Config = resolved.Config
	}
}

// --- BAD: missing 2-phase pattern (no resolved, no mapping) ---

func overlayNo2PhaseComputedFields(apiResp *ApiResponse, plan *GoodModel) { // want `overlayNo2PhaseComputedFields: missing 2-phase pattern`
	// Directly accesses apiResp without going through resolved/mapping.
	plan.Id = plan.Id
	plan.CreateTime = plan.CreateTime

	if plan.Amount.IsUnknown() {
		plan.Amount = plan.Amount
	}
	if plan.Currency.IsUnknown() {
		plan.Currency = plan.Currency
	}
}

// --- BAD: has resolved but no mapping call ---

func overlayNoMappingComputedFields(apiResp *ApiResponse, plan *GoodModel) { // want `overlayNoMappingComputedFields: missing mapping function call`
	var resolved GoodModel
	_ = resolved

	plan.Id = plan.Id
	plan.CreateTime = plan.CreateTime

	if plan.Amount.IsUnknown() {
		plan.Amount = plan.Amount
	}
	if plan.Currency.IsUnknown() {
		plan.Currency = plan.Currency
	}
}

// --- GOOD: Optional+Computed nested field handled via helper function ---

func mapHelperOverlayToModel(apiResp *ApiResponse, m *HelperOverlayModel) {}

func overlayHelperConfigFields(resolved *ConfigValue, plan *ConfigValue) {
	// mode: Optional+Computed — guarded by IsUnknown. ✓
	if plan.Mode.IsUnknown() {
		plan.Mode = resolved.Mode
	}
	// amount: Optional+Computed — guarded by IsUnknown. ✓
	if plan.Amount.IsUnknown() {
		plan.Amount = resolved.Amount
	}
}

func overlayHelperOverlayComputedFields(apiResp *ApiResponse, plan *HelperOverlayModel) {
	resolved := *plan
	mapHelperOverlayToModel(apiResp, &resolved)

	// Computed-only: unconditional. ✓
	plan.Id = resolved.Id

	// Optional+Computed nested: handled via helper function. ✓
	if !plan.Config.IsNull() && !plan.Config.IsUnknown() {
		overlayHelperConfigFields(&resolved.Config, &plan.Config)
	} else if plan.Config.IsUnknown() {
		plan.Config = resolved.Config
	}
}

// --- GOOD: Optional+Computed field with Default is never Unknown ---

func mapDefaultToModel(apiResp *ApiResponse, m *DefaultModel) {}

func overlayDefaultComputedFields(apiResp *ApiResponse, plan *DefaultModel) {
	resolved := *plan
	mapDefaultToModel(apiResp, &resolved)

	// Computed-only: unconditional. ✓
	plan.Id = resolved.Id

	// folder_id: has schema Default — never Unknown at plan time. ✓
	// No IsUnknown() guard needed.
}

// --- GOOD: Prefixed Go field name (DimensionsType → tfsdk:"type") ---
// The code generator prefixes "type" with the parent struct name because
// "type" is a Go keyword. The overlay correctly handles it via DimensionsType.

func mapPrefixedTypeToModel(apiResp *ApiResponse, m *PrefixedTypeModel) {}

func overlayPrefixedTypeComputedFields(apiResp *ApiResponse, plan *PrefixedTypeModel) {
	resolved := *plan
	mapPrefixedTypeToModel(apiResp, &resolved)

	// Computed-only: unconditional. ✓
	plan.Id = resolved.Id

	// Optional+Computed via prefixed Go name (DimensionsType → tfsdk:"type"). ✓
	if plan.DimensionsType.IsUnknown() {
		plan.DimensionsType = resolved.DimensionsType
	}
	if plan.Amount.IsUnknown() {
		plan.Amount = resolved.Amount
	}
}

// --- BAD: Prefixed Go field name missing in overlay ---

func mapPrefixedTypeBadToModel(apiResp *ApiResponse, m *PrefixedTypeModel) {}

func overlayPrefixedTypeBadComputedFields(apiResp *ApiResponse, plan *PrefixedTypeModel) { // want `overlayPrefixedTypeBadComputedFields: Optional\+Computed field\(s\) not handled: type`
	resolved := *plan
	mapPrefixedTypeBadToModel(apiResp, &resolved)

	plan.Id = resolved.Id

	// amount: handled. ✓
	if plan.Amount.IsUnknown() {
		plan.Amount = resolved.Amount
	}
	// DimensionsType (tfsdk:"type"): MISSING! ✗
}

// --- GOOD: Sub-overlay handles all nested fields correctly ---


func mapNestedListToModel(apiResp *ApiResponse, m *NestedListModel) {}

func overlayGoodNestedAlert(_ *ApiResponse, resolved, plan *AlertsValue) {
	// percentage: Optional+Computed — guarded by IsUnknown. ✓
	if plan.Percentage.IsUnknown() {
		plan.Percentage = resolved.Percentage
	}
	// triggered: Computed-only — unconditional. ✓
	plan.Triggered = resolved.Triggered
	// threshold: Required — not mentioned. ✓
}

func overlayNestedListComputedFields(apiResp *ApiResponse, plan *NestedListModel) {
	resolved := *plan
	mapNestedListToModel(apiResp, &resolved)

	// Computed-only: unconditional. ✓
	plan.Id = resolved.Id

	// Alerts: Optional+Computed list — overlay elements. ✓
	if plan.Alerts.IsUnknown() {
		plan.Alerts = resolved.Alerts
	} else if !plan.Alerts.IsNull() {
		overlayListElements(ctx, &resolved.Alerts, &plan.Alerts, overlayGoodNestedAlert)
	}
}

// --- BAD: Sub-overlay missing Computed-only nested field ---

func overlayBadNestedMissing(_ *ApiResponse, resolved, plan *AlertsValue) { // want `overlayBadNestedMissing: Computed-only field\(s\) not set from API response: triggered`
	// percentage: Optional+Computed — guarded by IsUnknown. ✓
	if plan.Percentage.IsUnknown() {
		plan.Percentage = resolved.Percentage
	}
	// triggered: Computed-only — MISSING! ✗
}

func mapBadNestedMissingToModel(apiResp *ApiResponse, m *NestedListModel) {}

func overlayBadNestedMissingComputedFields(apiResp *ApiResponse, plan *NestedListModel) {
	resolved := *plan
	mapBadNestedMissingToModel(apiResp, &resolved)

	plan.Id = resolved.Id

	if plan.Alerts.IsUnknown() {
		plan.Alerts = resolved.Alerts
	} else if !plan.Alerts.IsNull() {
		overlayListElements(ctx, &resolved.Alerts, &plan.Alerts, overlayBadNestedMissing)
	}
}

// --- BAD: Sub-overlay unconditionally assigns Optional+Computed ---

func overlayBadNestedUnconditional(_ *ApiResponse, resolved, plan *AlertsValue) {
	// percentage: Optional+Computed — assigned unconditionally. ✗
	plan.Percentage = resolved.Percentage // want `overlayBadNestedUnconditional: Optional\+Computed field "percentage" is assigned unconditionally`
	// triggered: Computed-only — unconditional. ✓
	plan.Triggered = resolved.Triggered
}

func mapBadNestedUncondToModel(apiResp *ApiResponse, m *NestedListModel) {}

func overlayBadNestedUncondComputedFields(apiResp *ApiResponse, plan *NestedListModel) {
	resolved := *plan
	mapBadNestedUncondToModel(apiResp, &resolved)

	plan.Id = resolved.Id

	if plan.Alerts.IsUnknown() {
		plan.Alerts = resolved.Alerts
	} else if !plan.Alerts.IsNull() {
		overlayListElements(ctx, &resolved.Alerts, &plan.Alerts, overlayBadNestedUnconditional)
	}
}

// --- GOOD: Multi-level nesting with sub-overlay calling deeper sub-overlay ---

func mapMultiLevelToModel(apiResp *ApiResponse, m *MultiLevelModel) {}

func overlayMultiLevelComponent(_ *ApiResponse, resolved, plan *ComponentsValue) {
	// case_insensitive: Optional+Computed — guarded. ✓
	if plan.CaseInsensitive.IsUnknown() {
		plan.CaseInsensitive = resolved.CaseInsensitive
	}
	// mode: Optional+Computed — guarded. ✓
	if plan.Mode.IsUnknown() {
		plan.Mode = resolved.Mode
	}
	// key: Required — not mentioned. ✓
}

func overlayMultiLevelRule(_ *ApiResponse, resolved, plan *RulesValue) {
	// action: Optional+Computed — guarded. ✓
	if plan.Action.IsUnknown() {
		plan.Action = resolved.Action
	}
	// components: Optional+Computed list — overlay via deeper sub-overlay. ✓
	if plan.Components.IsUnknown() {
		plan.Components = resolved.Components
	} else if !plan.Components.IsNull() {
		overlayListElements(ctx, &resolved.Components, &plan.Components, overlayMultiLevelComponent)
	}
}

func overlayMultiLevelComputedFields(apiResp *ApiResponse, plan *MultiLevelModel) {
	resolved := *plan
	mapMultiLevelToModel(apiResp, &resolved)

	plan.Id = resolved.Id

	if plan.Rules.IsUnknown() {
		plan.Rules = resolved.Rules
	} else if !plan.Rules.IsNull() {
		overlayListElements(ctx, &resolved.Rules, &plan.Rules, overlayMultiLevelRule)
	}
}

// Stub overlayListElements for test compilation.
func overlayListElements(ctx context.Context, resolved, plan interface{}, fn interface{}) {}

// Package-level ctx for test function calls.
var ctx = context.Background()
