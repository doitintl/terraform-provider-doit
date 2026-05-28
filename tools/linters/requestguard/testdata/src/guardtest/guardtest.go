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
