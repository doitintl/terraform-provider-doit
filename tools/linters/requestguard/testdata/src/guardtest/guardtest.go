package guardtest

// --- Direction 1: Redundant IsUnknown() guards ---

// toCreateRequest is a request builder method on the model.
func (plan *GuardTestModel) toCreateRequest() ApiRequest {
	req := ApiRequest{}

	// GOOD: Required field — no guard needed, direct access is correct.
	req.Name = plan.Name.ValueStringPointer()

	// BAD: Required field — IsUnknown() guard is dead code.
	if !plan.Name.IsNull() && !plan.Name.IsUnknown() { // want `IsUnknown\(\) on Required field "name" is dead code \(Required fields are always Known\)`
		req.Name = plan.Name.ValueStringPointer()
	}

	// BAD: Optional (no Computed) field — IsUnknown() guard is dead code.
	if !plan.Label.IsNull() && !plan.Label.IsUnknown() { // want `IsUnknown\(\) on Optional \(not Computed\) field "label" is dead code \(Optional \(not Computed\) fields are always Known\)`
		req.Label = plan.Label.ValueStringPointer()
	}

	// BAD: Optional+Computed WITH default — IsUnknown() is dead code.
	if !plan.Metric.IsNull() && !plan.Metric.IsUnknown() { // want `IsUnknown\(\) on field "metric" with schema Default is dead code \(defaults are resolved at plan time\)`
		req.Metric = plan.Metric.ValueStringPointer()
	}

	// GOOD: Optional+Computed WITHOUT default — IsUnknown() guard is needed.
	if !plan.FolderId.IsNull() && !plan.FolderId.IsUnknown() {
		req.FolderId = plan.FolderId.ValueStringPointer()
	}

	// GOOD: Computed-only — IsUnknown() guard is legitimate.
	if !plan.Id.IsNull() && !plan.Id.IsUnknown() {
		// id is computed-only, guard is fine
	}

	return req
}

// --- Direction 2: Missing IsUnknown() guards ---

// toUpdateRequest tests missing guard detection.
func (plan *GuardTestModel) toUpdateRequest() ApiRequest {
	req := ApiRequest{}

	// GOOD: Required — no guard needed, ValueString is safe.
	req.Name = plan.Name.ValueStringPointer()

	// GOOD: Optional+Computed WITHOUT default, properly guarded.
	if !plan.FolderId.IsNull() && !plan.FolderId.IsUnknown() {
		req.FolderId = plan.FolderId.ValueStringPointer()
	}

	// BAD: Optional+Computed WITHOUT default, ValueString without guard.
	name := plan.FolderId.ValueString() // want `ValueString\(\) on Optional\+Computed field "folder_id" without IsUnknown\(\) guard; field may be Unknown at plan time`
	_ = name

	// GOOD: Optional+Computed WITHOUT default, ValueStringPointer without guard.
	// Pointer accessors return nil for Unknown, which is often acceptable.
	req.FolderId = plan.FolderId.ValueStringPointer()

	// GOOD: Optional+Computed WITH default — no guard needed, ValueString is safe.
	metric := plan.Metric.ValueString()
	_ = metric

	return req
}

// --- Nested field tests ---

// toConfigRequest tests nested field handling.
func toConfigRequest(plan *GuardTestModel) *ConfigRequest {
	config := plan.Config
	req := &ConfigRequest{}

	// BAD: Required nested field — IsUnknown() is dead code.
	if !config.Type.IsNull() && !config.Type.IsUnknown() { // want `IsUnknown\(\) on Required field "type" is dead code \(Required fields are always Known\)`
		req.Type = config.Type.ValueStringPointer()
	}

	// BAD: Optional+Computed WITH default nested field — IsUnknown() is dead code.
	if !config.CaseInsensitive.IsNull() && !config.CaseInsensitive.IsUnknown() { // want `IsUnknown\(\) on field "case_insensitive" with schema Default is dead code \(defaults are resolved at plan time\)`
		b := config.CaseInsensitive.ValueBool()
		req.CaseInsensitive = &b
	}

	// GOOD: Optional+Computed WITHOUT default nested field — guard is needed.
	if !config.Currency.IsNull() && !config.Currency.IsUnknown() {
		req.Currency = config.Currency.ValueStringPointer()
	}

	// BAD: Optional+Computed WITHOUT default nested field, ValueString without guard.
	val := config.Currency.ValueString() // want `ValueString\(\) on Optional\+Computed field "currency" without IsUnknown\(\) guard; field may be Unknown at plan time`
	_ = val

	return req
}

// --- Guard polarity tests ---

// toInviteRequest tests that positive IsUnknown() checks are NOT treated as guards.
func (plan *GuardTestModel) toInviteRequest() ApiRequest {
	req := ApiRequest{}

	// BAD: Positive IsUnknown() check — body runs WHEN Unknown, so
	// ValueString() inside is definitely wrong. Must still be flagged.
	if plan.FolderId.IsUnknown() {
		val := plan.FolderId.ValueString() // want `ValueString\(\) on Optional\+Computed field "folder_id" without IsUnknown\(\) guard; field may be Unknown at plan time`
		req.FolderId = &val
	}

	// GOOD: Negated IsUnknown() check — body runs when Known.
	if !plan.FolderId.IsUnknown() {
		val := plan.FolderId.ValueString()
		req.FolderId = &val
	}

	return req
}

// --- Deep nesting tests (2+ levels) ---

// toFilterRequest tests resolution through 2 levels of nesting.
func toFilterRequest(plan *GuardTestModel) *FilterRequest {
	config := plan.Config
	req := &FilterRequest{}

	// BAD: Required at 2nd nesting level via variable chain — IsUnknown() is dead code.
	if !config.Filter.Operator.IsNull() && !config.Filter.Operator.IsUnknown() { // want `IsUnknown\(\) on Required field "operator" is dead code \(Required fields are always Known\)`
		req.Operator = config.Filter.Operator.ValueStringPointer()
	}

	// BAD: Optional+Computed without default at 2nd nesting level — missing guard.
	val := config.Filter.Mode.ValueString() // want `ValueString\(\) on Optional\+Computed field "mode" without IsUnknown\(\) guard; field may be Unknown at plan time`
	_ = val

	// GOOD: Guarded Optional+Computed without default at 2nd nesting level.
	if !config.Filter.Mode.IsUnknown() {
		req.Mode = config.Filter.Mode.ValueStringPointer()
	}

	// BAD: Required at 2nd nesting level via inline 3-level chain — IsUnknown() is dead code.
	if !plan.Config.Filter.Operator.IsUnknown() { // want `IsUnknown\(\) on Required field "operator" is dead code \(Required fields are always Known\)`
		req.Operator = plan.Config.Filter.Operator.ValueStringPointer()
	}

	return req
}

// --- Cross-function schema propagation tests ---

// toHelperTestRequest is a builder that calls helper functions with plan fields.
// The linter should propagate schema context into the helpers.
func (plan *GuardTestModel) toHelperTestRequest() ApiRequest {
	req := ApiRequest{}

	// Call a helper function with plan.Config as argument.
	fillFromConfig(&req, plan.Config)

	// Call a helper with a tracked variable.
	config := plan.Config
	fillDisplaySettings(&req, config.DisplaySettings)

	return req
}

// fillFromConfig is a helper function taking a ConfigValue directly.
// Schema context is propagated from toCreateRequest's call with plan.Config.
func fillFromConfig(req *ApiRequest, config ConfigValue) {
	// BAD: Required nested field — IsUnknown() is dead code.
	if !config.Type.IsNull() && !config.Type.IsUnknown() { // want `IsUnknown\(\) on Required field "type" is dead code \(Required fields are always Known\)`
		req.Name = config.Type.ValueStringPointer()
	}

	// BAD: Optional+Computed WITH default — IsUnknown() is dead code.
	if !config.CaseInsensitive.IsUnknown() { // want `IsUnknown\(\) on field "case_insensitive" with schema Default is dead code \(defaults are resolved at plan time\)`
		_ = config.CaseInsensitive.ValueBool()
	}

	// BAD: Optional+Computed WITHOUT default — missing guard.
	val := config.Currency.ValueString() // want `ValueString\(\) on Optional\+Computed field "currency" without IsUnknown\(\) guard; field may be Unknown at plan time`
	_ = val

	// Transitive call: pass a nested field to another helper.
	fillFilterFromConfig(config.Filter)
}

// fillFilterFromConfig is called transitively from fillFromConfig.
// Schema context propagates: builder → fillFromConfig → fillFilterFromConfig.
func fillFilterFromConfig(filter FilterValue) {
	// BAD: Required at 2nd nesting level — IsUnknown() is dead code.
	if !filter.Operator.IsUnknown() { // want `IsUnknown\(\) on Required field "operator" is dead code \(Required fields are always Known\)`
		_ = filter.Operator.ValueStringPointer()
	}

	// BAD: Optional+Computed without default — missing guard.
	val := filter.Mode.ValueString() // want `ValueString\(\) on Optional\+Computed field "mode" without IsUnknown\(\) guard; field may be Unknown at plan time`
	_ = val
}

// fillDisplaySettings is a helper taking a DisplaySettingsValue.
// Schema context propagated from toCreateRequest via config.DisplaySettings.
func fillDisplaySettings(req *ApiRequest, ds DisplaySettingsValue) {
	// BAD: Optional+Computed WITH default — IsUnknown() is dead code.
	if !ds.ThemeId.IsNull() && !ds.ThemeId.IsUnknown() { // want `IsUnknown\(\) on field "theme_id" with schema Default is dead code \(defaults are resolved at plan time\)`
		req.Metric = ds.ThemeId.ValueStringPointer()
	}
}

// --- Non-builder standalone function (should NOT be analyzed) ---

// standaloneHelper is not called from any builder. The linter should not
// analyze it, so these patterns should NOT produce diagnostics.
func standaloneHelper(config ConfigValue) {
	_ = config.Currency.ValueString() // no diagnostic — not reachable from a builder
	if !config.Type.IsUnknown() {     // no diagnostic — not reachable from a builder
		_ = config.Type.ValueStringPointer()
	}
}
