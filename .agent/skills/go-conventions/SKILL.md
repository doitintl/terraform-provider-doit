---
name: go-conventions
description: Go code style rules for the Terraform provider. Covers diagnostics handling, generated constructors, pointer conversions, interface checks, and troubleshooting "inconsistent result" errors.
---

# Go Conventions

Code style rules that apply to **all** Go code in this provider. These are actively enforced by custom linters in `tools/linters/`.

## Diagnostics Must Never Be Suppressed or Dropped

**CRITICAL:** Never suppress `diag.Diagnostics` return values with `_`. All diagnostics must be properly handled:

```go
// BAD — diagnostics suppressed, errors silently ignored
myList, _ = types.ListValue(types.StringType, []attr.Value{})

// GOOD — diagnostics properly handled
var listDiags diag.Diagnostics
myList, listDiags = types.ListValue(types.StringType, []attr.Value{})
diags.Append(listDiags...)
```

This applies to all Terraform Framework functions that return diagnostics:

- `types.ListValue()`, `types.MapValue()`, `types.ObjectValue()`
- `types.ListValueFrom()`, `types.MapValueFrom()`
- `NewXxxValue()` generated constructors
- Any other function returning `diag.Diagnostics`

> **Linter:** `diagsuppressed` — flags suppressed diagnostics.

**Also:** never return `nil` when a `diag.Diagnostics` variable has been captured — this silently drops non-error diagnostics (e.g. warnings):

```go
// BAD — non-error diagnostics (warnings) silently lost
func populateState(...) diag.Diagnostics {
    user, diags := r.lookupUser(ctx, email)
    if diags.HasError() { return diags }
    return nil  // ← drops warnings in diags
}

// GOOD — all diagnostics propagated
func populateState(...) diag.Diagnostics {
    user, diags := r.lookupUser(ctx, email)
    if diags.HasError() { return diags }
    return diags
}
```

> **Linter:** `diagdrop` — flags `return nil` that drops captured diagnostics.

## Generated Constructors (NewXxxValue)

**Always** use generated constructor functions instead of struct literals for Terraform Plugin Framework types. Struct literals leave internal `state` fields zeroed, causing `Unknown`/`Null` states:

```go
// BAD — internal state field missing
scopeVal := resource_budget.ScopesValue{
    Id: types.StringValue("foo"),
}

// GOOD — internal state properly initialized
scopeVal, diags := resource_budget.NewScopesValue(
    resource_budget.ScopesValue{}.AttributeTypes(ctx),
    map[string]attr.Value{
        "id": types.StringValue("foo"),
    },
)
```

> **Linter:** `structliteral` — flags struct literal construction of generated types.

## Use `new(expr)` for Pointer Conversions (Go 1.26+)

Prefer `new(expr)` over creating a temporary variable solely to take its address:

```go
// OLD — verbose
filter := data.Filter.ValueString()
params.Filter = &filter

// NEW — concise
params.Filter = new(data.Filter.ValueString())
```

**When to use:**

- ✅ Converting value to pointer for API parameters
- ✅ Any temp-variable-then-address pattern
- ❌ Constructor functions (still use `&type{}`)
- ❌ When the named variable is needed for readability later

> **Linter:** `newexpr` — suggests `new(expr)` for temp-then-address patterns.

## Nullable Type Helpers (`nullable.Nullable[T]`)

Generated API models use `nullable.Nullable[T]` (from `github.com/oapi-codegen/nullable`) instead of `*T` for fields where explicit `null` clearing is or will be supported. Three generic helpers in `nullable_helpers.go` bridge between Terraform types and nullable fields:

| Helper | Signature | Use Case |
|--------|-----------|----------|
| `nullableToPointer` | `nullable.Nullable[T] → *T` | **Read path** — convert nullable response fields to `*T` for `types.XxxPointerValue()` |
| `valueToNullable` | `T → nullable.Nullable[T]` | **Write path** — wrap a concrete value for a nullable request field (preferred) |
| `pointerToNullable` | `*T → nullable.Nullable[T]` | **Write path** — wrap a `ValueXxxPointer()` result for a nullable request field |

### Read path (API response → TF state)

```go
// Scalar nullable field
state.Field = types.StringPointerValue(nullableToPointer(resp.Field))

// Complex nullable field that needs dereferencing
if rule := nullableToPointer(resp.Rule); rule != nil {
    formula = rule.Formula
}
```

### Write path (TF plan → API request)

Prefer `valueToNullable` when the value is known non-nil — it avoids unnecessary pointer indirection:

```go
// PREFERRED — concrete value
req.Name = valueToNullable(plan.Name.ValueString())

// OK — when you already have a *T from ValueXxxPointer()
req.Description = pointerToNullable(plan.Description.ValueStringPointer())
```

### Nullable struct fields (`.Set()`)

For nullable struct-typed fields, build the struct first, then call `.Set()`:

```go
// OLD — pointer assignment
req.Rule = &models.AllocationRule{Formula: plan.Rule.Formula.ValueString()}

// NEW — build then Set()
var rule models.AllocationRule
rule.Formula = plan.Rule.Formula.ValueString()
req.Rule.Set(rule)
```

### Nullable slices in lists

When a list contains nullable elements (`[]nullable.Nullable[T]`), use `.Set()` per element:

```go
rules := make([]nullable.Nullable[models.GroupAllocationRule], len(planRules))
for i := range planRules {
    var rule models.GroupAllocationRule
    rule.Name = planRules[i].Name.ValueStringPointer()
    rules[i].Set(rule)
}
req.Rules = &rules
```

### Why nullable types?

Not all API fields are `*T` pointers now — only fields where the API supports or will support explicit `null` clearing use `nullable.Nullable[T]`. This distinction matters for nested objects where sending the zero value (`{}`) is interpreted as "set defaults" rather than "clear." The API is gradually adding `null` acceptance per endpoint.

## .gitignore Check

Always check `.gitignore` before committing. Some files (like `OpenAPI/api_endpoint_analysis.md`) are local documentation:

```bash
git check-ignore -v path/to/file
```

## "Provider Produced Inconsistent Result"

This error is **always** a provider bug. It is never a flaky test or API issue.

It means the state returned by Create or Update doesn't match the plan. Common causes:

1. **Wrong pointer semantics** — `ValueBool()` + `&val` instead of `ValueBoolPointer()`
2. **Null vs empty list mismatch** — `types.ListNull()` when user configured `field = []`
3. **Missing generated constructors** — struct literals instead of `NewXxxValue()`
4. **Timestamp normalization** — API returns UTC, user provided different timezone
5. **Using `mapResourceToModel` in Create/Update** — bypasses overlay pattern

**Mitigation:** The plan-first overlay pattern structurally prevents this error class. If you encounter it, the overlay is missing or incomplete.
