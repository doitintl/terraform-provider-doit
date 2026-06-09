# Plan-First Overlay Pattern

This is the core state management pattern used by **all** resources in this provider. It prevents "Provider produced inconsistent result" errors caused by API normalization of user-provided values.

## Why Plan-First Is Universal

DoIT APIs return complete objects in Create and Update responses, but frequently normalize values (stripping sentinels, renaming type aliases, trimming suffixes). Using the API response directly to populate state causes mismatches with the user's plan. The overlay pattern:

- Structurally prevents "inconsistent result" errors regardless of future API changes
- Has negligible overhead and consistent implementation across all resources
- Treats the Terraform plan as the source of truth for user-configured fields

## Step 1: Classify Every Field

| Category | Schema | Overlay Behavior | Example |
|----------|--------|-------------------|---------|
| **Computed-only (stable)** | `Computed: true` only | Always set from API response | `id`, `create_time` |
| **Computed-only (volatile)** | `Computed: true` only | Guard with `IsUnknown()` — set from API on Create, preserve plan on Update | `update_time`, `last_alerted` |
| **Required** | `Required: true` | Never touch — preserve plan value | `name` |
| **Optional** | `Optional: true` | Never touch — preserve plan value | `description` |
| **Optional+Computed** | `Optional: true, Computed: true` | Resolve only when `IsUnknown()` | `formula`, boolean flags |
| **Optional+Computed clearable list** | `Optional: true, Computed: true` + list modifier | Resolve `IsUnknown()` to `null` (not API response) | `labels`, `reports` |
| **Optional+Computed with Default** | `Optional: true, Computed: true, Default: ...` | Never touch — default resolves at plan time | `metric`, `case_insensitive` |

Key rules:

- **Computed-only (stable)**: always overwrite with API value (`id`, `create_time`)
- **Computed-only (volatile)**: guard with `IsUnknown()` — set from API on Create (where the field IS Unknown), preserve the plan value on Update (where the field is Known from prior state). This prevents "inconsistent result" errors when Updates are triggered by plan modifiers. The next Read fetches the updated value.
- **Required / Optional**: never overwrite — user's plan wins
- **Optional+Computed when `IsUnknown()`**: user omitted the field — resolve from API response, fall back to null/false
- **Optional+Computed clearable list when `IsUnknown()`**: user omitted the field — resolve to `types.ListNull()` (not API response) to match the `useNullForUnknownListWhenConfigNull` modifier semantics
- **Optional+Computed when known**: user explicitly set it — never overwrite
- **Optional+Computed with Default**: always known — the framework resolves the default at plan time, so the field is never Unknown. **Do not add `IsUnknown()` guards** — they are dead code

## Step 2: Implement overlayXxxComputedFields

Always include the resource name in the function name (e.g. `overlayReportComputedFields`, not `overlayComputedFields`). This must be a **standalone function** (no receiver) unless `mapXxxToModel` makes additional API calls via `r.client` (e.g. allocation), in which case the receiver is allowed:

```go
func overlayMyResourceComputedFields(ctx context.Context, apiResp *models.MyResource, plan *myResourceModel) diag.Diagnostics {
    var diags diag.Diagnostics

    // 1. Computed-only stable fields: ALWAYS set from API response.
    plan.Id = types.StringPointerValue(apiResp.Id)
    plan.CreateTime = types.Int64PointerValue(apiResp.CreateTime)

    // 2. Computed-only volatile fields: guard with IsUnknown().
    //    On Create (Unknown) → set from API. On Update (Known) → preserve plan.
    if plan.UpdateTime.IsUnknown() {
        plan.UpdateTime = types.Int64PointerValue(apiResp.UpdateTime)
    }

    // 3. Optional+Computed scalars: resolve ONLY when unknown.
    if plan.Formula.IsUnknown() {
        plan.Formula = types.StringPointerValue(apiResp.Formula)
    }
    // If plan.Formula is known, leave it untouched — user's value wins.

    // 4. Optional+Computed clearable lists: resolve to null, not API response.
    if plan.Labels.IsUnknown() {
        plan.Labels = types.ListNull(types.StringType)
    }

    // 5. Optional+Computed booleans: resolve when unknown, default to false.
    if !plan.Components.IsNull() && !plan.Components.IsUnknown() {
        resolved, compDiags := resolveComponentUnknowns(ctx, &plan.Components)
        diags.Append(compDiags...)
    }

    return diags
}
```

### Nested Attributes (ListNestedAttribute / SingleNestedAttribute)

For fields with nested attributes (e.g., `alerts`, `config`, `rules`), use **sub-overlay helper functions** rather than inline field handling. The `overlaycheck` linter validates sub-overlay functions against their nested schemas.

**Pattern: `overlayListElements` with sub-overlay callback**

```go
// In the top-level overlay:
if plan.Alerts.IsUnknown() {
    plan.Alerts = resolved.Alerts
} else if !plan.Alerts.IsNull() {
    diags.Append(overlayListElements(ctx, &resolved.Alerts, &plan.Alerts, overlayMyResourceAlert)...)
}

// Sub-overlay helper — validated by overlaycheck against the nested schema:
func overlayMyResourceAlert(_ context.Context, resolved, plan *resource_myresource.AlertsValue) diag.Diagnostics {
    // Computed-only nested fields: unconditional
    plan.Triggered = resolved.Triggered

    // Optional+Computed nested fields: guarded by IsUnknown
    if plan.Percentage.IsUnknown() {
        plan.Percentage = resolved.Percentage
    }

    // Required nested fields: not mentioned
    return nil
}
```

**Pattern: direct sub-overlay for SingleNestedAttribute**

```go
if !plan.Config.IsNull() && !plan.Config.IsUnknown() {
    diags.Append(overlayConfigFields(ctx, &resolved.Config, &plan.Config)...)
} else if plan.Config.IsUnknown() {
    plan.Config = resolved.Config
}
```

**Multi-level nesting**: Sub-overlays can call deeper sub-overlays. The linter recursively validates the entire chain.

> **Linter:** `overlaycheck` **enforces** that nested attributes with computed fields use sub-overlay helper functions — inline handling is not allowed. The linter validates both top-level overlay functions and all sub-overlay helpers reachable via `overlayListElements` callbacks or direct `overlay*` calls with `&plan.X` arguments.
>
> **Exception:** If all children of a nested attribute are Required or have schema Defaults, no sub-overlay is needed — the field can be assigned unconditionally when Unknown. The linter skips enforcement for these cases.

## Step 3: Wire Into Create and Update

```go
func (r *myResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
    var plan myResourceModel
    resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
    // ... build request from plan, call API ...

    // Plan-first: keep user values, overlay only computed fields
    resp.Diagnostics.Append(overlayMyResourceComputedFields(ctx, apiResp, &plan)...)
    resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}
```

Apply the same to Update. Read and ImportState continue using `populateState` (which calls `mapXxxToModel`) since they have no plan to preserve.

## Step 4: Handle Read-Path Reconciliation

The Read path detects **real** external changes while ignoring API normalization:

**Sentinel restoration** (`mergeSentinelValues`): When the API strips user-provided sentinels (e.g. `[Service N/A]`), restore them if the API value is a substring of the state value.

**Type alias normalization** (`normalizeDimensionsType`): When the API returns a canonical type name for a user-provided alias (e.g. `attribution` for `allocation_rule`), normalize back to the user's alias.

## Common Pitfalls

1. **Forgetting to resolve nested unknowns.** Walk into each level of nested objects to resolve unknown booleans — an unknown at any depth causes "inconsistent result".

2. **Rebuilding nested values after modification.** When modifying fields inside a list element, rebuild the parent list with `types.ListValueFrom()`. List elements are immutable in the framework.

3. **null↔[] consistency between overlay and Read.** When the user omits an Optional+Computed list, the overlay and Read path must agree on the representation. For **non-clearable lists** (Category B): both must use `[]`. For **clearable lists** (Category A with `useNullForUnknownListWhenConfigNull`): overlay must resolve Unknown to `null`, and Read must preserve null when prior state was null. See [Clearable List Attributes](../implement-resource/SKILL.md#clearable-list-attributes).

4. **Not testing the Update path separately.** Create and Update can have different API behaviors. Always test both.

5. **Mixing overlay and full mapping.** Never call `mapXxxToModel`/`populateState` directly from Create/Update — always go through the overlay wrapper. Conversely, never use `overlayXxxComputedFields` in Read/ImportState.

6. **API-defaulted lists need Category B classification.** Lists that are auto-populated by the API when omitted (e.g. `recipients` defaults to creator's email, `collaborators` adds creator as owner) must be classified as Category B (not clearable). Applying the clearing modifier to these causes perpetual drift because the API always repopulates the default.

7. **Test for null vs [] based on clearability.** For non-clearable lists (Category B), add `ListSizeExact(0)` checks on omitted list state. For clearable lists (Category A), add `knownvalue.Null()` checks instead — omitted clearable lists resolve to null.

8. **Don't add IsUnknown() guards for fields with Defaults.** When a field has `Default: stringdefault.StaticString(...)` (or similar), the framework resolves it at plan time — the field is never Unknown. An `IsUnknown()` guard is dead code and can mask legitimate changes from cross-resource references. The `overlaycheck` linter flags these automatically.

9. **Guard volatile Computed-only fields with IsUnknown().** Fields like `update_time` that change on every API write must be guarded in the overlay. Without the guard, modifier-triggered Updates write a new value that mismatches the plan's prior-state value, causing "inconsistent result" errors.
