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
| **Computed-only** | `Computed: true` only | Always set from API response | `id`, `create_time`, `update_time` |
| **Required** | `Required: true` | Never touch — preserve plan value | `name` |
| **Optional** | `Optional: true` | Never touch — preserve plan value | `description` |
| **Optional+Computed** | `Optional: true, Computed: true` | Resolve only when `IsUnknown()` | `formula`, boolean flags |

Key rules:

- **Computed-only**: always overwrite with API value
- **Required / Optional**: never overwrite — user's plan wins
- **Optional+Computed when `IsUnknown()`**: user omitted the field — resolve from API response, fall back to null/false
- **Optional+Computed when known**: user explicitly set it — never overwrite

## Step 2: Implement overlayComputedFields

```go
func overlayComputedFields(ctx context.Context, apiResp *models.MyResource, plan *myResourceModel) diag.Diagnostics {
    var diags diag.Diagnostics

    // 1. Computed-only fields: ALWAYS set from API response.
    plan.Id = types.StringPointerValue(apiResp.Id)
    plan.CreateTime = types.Int64PointerValue(apiResp.CreateTime)
    plan.UpdateTime = types.Int64PointerValue(apiResp.UpdateTime)

    // 2. Optional+Computed scalars: resolve ONLY when unknown.
    if plan.Formula.IsUnknown() {
        plan.Formula = types.StringPointerValue(apiResp.Formula)
    }
    // If plan.Formula is known, leave it untouched — user's value wins.

    // 3. Optional+Computed booleans: resolve when unknown, default to false.
    if !plan.Components.IsNull() && !plan.Components.IsUnknown() {
        resolved, compDiags := resolveComponentUnknowns(ctx, &plan.Components)
        diags.Append(compDiags...)
    }

    return diags
}
```

## Step 3: Wire Into Create and Update

```go
func (r *myResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
    var plan myResourceModel
    resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
    // ... build request from plan, call API ...

    // Plan-first: keep user values, overlay only computed fields
    resp.Diagnostics.Append(overlayComputedFields(ctx, apiResp, &plan)...)
    resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}
```

Apply the same to Update. Read and ImportState continue using `mapResourceToModel` / `populateState` since they have no plan to preserve.

## Step 4: Handle Read-Path Reconciliation

The Read path detects **real** external changes while ignoring API normalization:

**Sentinel restoration** (`mergeSentinelValues`): When the API strips user-provided sentinels (e.g. `[Service N/A]`), restore them if the API value is a substring of the state value.

**Type alias normalization** (`normalizeDimensionsType`): When the API returns a canonical type name for a user-provided alias (e.g. `attribution` for `allocation_rule`), normalize back to the user's alias.

## Common Pitfalls

1. **Forgetting to resolve nested unknowns.** Walk into each level of nested objects to resolve unknown booleans — an unknown at any depth causes "inconsistent result".

2. **Rebuilding nested values after modification.** When modifying fields inside a list element, rebuild the parent list with `types.ListValueFrom()`. List elements are immutable in the framework.

3. **null↔[] consistency between overlay and Read.** When the user omits an Optional+Computed list, the overlay must resolve `IsUnknown()` to the **same representation** the Read path uses. If Read returns `[]`, overlay must also resolve to `[]` — using `ListNull()` causes state churn. Only use `null` if Read also returns `null` for that field.

4. **Not testing the Update path separately.** Create and Update can have different API behaviors. Always test both.

5. **Mixing overlay and full mapping.** Never call `mapResourceToModel`/`populateState` directly from Create/Update — always go through the overlay wrapper. Conversely, never use `overlayComputedFields` in Read/ImportState.

6. **API-defaulted lists need API response, not null.** Some lists are auto-populated by the API when omitted (e.g. `collaborators` adds creator as owner). Resolving `IsUnknown()` to `null` causes drift. Resolve from the API response instead.

7. **Test for null↔[] flips explicitly.** When omitting Optional+Computed list fields, add `ListSizeExact(0)` checks on both Create and drift-check steps.
