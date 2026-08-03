---
name: implementation-conventions
description: Shared implementation conventions for Terraform provider resources and data sources. Covers timeouts, error handling, constructor style, variable naming, and API authentication patterns.
---

# Implementation Conventions

Shared patterns that apply to **both** resources and data sources in this provider. Enforced by custom linters in `tools/linters/`.

## Constructor Style

Use `&type{}` composite literal style for resource/data source constructors:

```go
// CORRECT
func NewLabelResource() resource.Resource {
    return &labelResource{}
}

// WRONG — do not use new(type) for constructors
func NewLabelResource() resource.Resource {
    return new(labelResource)
}
```

> **Linter:** `constructor` — flags `new(type)` in constructor return statements.

## File Structure & Code Organization

### Resource File Layout

Every resource splits into two files:

| File | Contents |
|------|----------|
| `<name>_resource.go` | Type/model declarations, interface checks, `NewXxxResource()`, `Configure`, `Metadata`, `Schema`, `ImportState`, `ConfigValidators`, CRUD methods |
| `<name>.go` | `populateState`, `mapXxxToModel`, `overlayXxxComputedFields`, `toXxxRequest`, and any helper functions |

Additional files as needed: `<name>_validator.go`, `<name>_state_upgrader.go`.

Data sources follow the same companion-file pattern when they have non-trivial helper functions:

| File | Contents |
|------|----------|
| `<name>_data_source.go` | Type/model declarations, interface checks, `NewXxxDataSource()`, `Configure`, `Metadata`, `Schema`, Read method |
| `<name>.go` | `mapXxxToModel` and any helper functions (shared with the resource if one exists) |

Simple data sources that only do a few scalar assignments may keep mapping logic inline in Read.

### Variable Initialization

Always use `var` for plan/state variables, never `new()`:

```go
// CORRECT
var plan myResourceModel
resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

// WRONG — do not use new() for model variables
plan := new(myResourceModel)
```

### Standard Helper Functions

Every resource must implement these four functions in `<name>.go`:

| Function | Signature | Purpose |
|----------|-----------|---------|
| `populateState` | `(r *xResource) populateState(ctx, state *xResourceModel) diag.Diagnostics` | Fetches from API using the identifier in `state` (e.g. `state.Id`, `state.Name`), calls `mapXxxToModel`. Sets the identifier attribute to `types.StringNull()` on 404. Used by Read (and ImportState via Read). |
| `mapXxxToModel` | `mapXxxToModel([ctx,] apiResp, state) [diag.Diagnostics]` | Pure mapping from API response to TF model. **Standalone function — no receiver.** Used by `populateState` and as Phase 1 of overlay. |
| `overlayXxxComputedFields` | `overlayXxxComputedFields([ctx,] apiResp, plan) [diag.Diagnostics]` | Two-phase overlay. **Standalone function — no receiver. Always prefix with the resource name** (e.g. `overlayReportComputedFields`, not `overlayComputedFields`). Used by Create/Update only. |
| `toXxxRequest` | `(plan *xResourceModel) toXxxRequest([ctx]) (req[, diag.Diagnostics])` | **Method on the plan model**, converts TF model to API request. When create and update share a request type, name it `toUpdateRequest`. |

If create and update use different API request types, implement both `toCreateRequest` and `toUpdateRequest`.

> **Receiver rule:** Only `populateState` has a receiver (it needs `r.client`). `mapXxxToModel` and `overlayXxxComputedFields` must be standalone functions. `toXxxRequest` / `toCreateRequest` / `toUpdateRequest` are methods on the plan model. **Exception:** If `mapXxxToModel` makes additional API calls (e.g. allocation fetches full rule details via `r.client`), it may retain the receiver, which propagates to `overlayXxxComputedFields`.

> **Signature flexibility:** The `ctx` and `diag.Diagnostics` parameters are required when the function maps nested objects (lists, objects) or can produce errors. Simple resources that only do scalar assignments (e.g. `types.StringValue`, `types.StringPointerValue`) may omit `ctx` and return nothing. Match the complexity of your resource.

### Mapping Functions and Schema Defaults

When a schema field has a `Default` (e.g., `stringdefault.StaticString("cost")`), `mapXxxToModel` must map `nil` API responses to the default value, not to `null`:

```go
// CORRECT — preserves the schema default on nil response (pointer field)
if apiResp.Metric != nil {
    state.Metric = types.StringValue(*apiResp.Metric)
} else {
    state.Metric = types.StringValue("cost") // schema default
}

// CORRECT — nullable field variant
if metric := nullableToPointer(apiResp.Metric); metric != nil {
    state.Metric = types.StringValue(*metric)
} else {
    state.Metric = types.StringValue("cost") // schema default
}

// WRONG — drifts against the schema default
state.Metric = types.StringPointerValue(apiResp.Metric) // nil → null ≠ "cost"
```

> **Linter:** `defaultdrift` — flags `PointerValue` usage on fields with schema defaults in Read-path mapping functions.

### Request Builders and IsUnknown()

In `toCreateRequest` / `toUpdateRequest` and similar request builder functions, `IsUnknown()` guards are only needed for `Optional+Computed` fields **without** a `Default`. For all other fields:

| Field class | `IsUnknown()` needed? | Why |
|---|---|---|
| Required | No | User must set it — always Known |
| Optional (no Computed) | No | Known or Null, never Unknown |
| Optional+Computed **with Default** | No | Default resolves at plan time |
| Optional+Computed **without Default** | **Yes** | May be Unknown at plan time |

For non-pointer value accessors (`ValueString()`, `ValueBool()`, `ValueFloat64()`, `ValueInt64()`), always guard with `IsUnknown()` when the field is Optional+Computed without Default — these accessors return zero values for Unknown, not nil.

**Nullable request fields:** Generated request structs may use `nullable.Nullable[T]` instead of `*T` for fields that support explicit null clearing. Use `valueToNullable(val)` for concrete values and `pointerToNullable(ptr)` for pointer values. See the [go-conventions](../go-conventions/SKILL.md#nullable-type-helpers-nullablenullablet) skill for the full helper reference.

> **Warning (Unknown State pointers):** Be extremely careful with `.ValueStringPointer()` (or equivalent accessor methods) on framework attribute values. If the framework attribute has an `Unknown` state, `.ValueStringPointer()` returns a pointer to a **zero value** (e.g. `*""` or `*0`), NOT `nil`. If you pass this zero value pointer to `pointerToNullable()`, it will mark the `Nullable` as explicitly specified rather than `null`/omitted. Always guard access with `!plan.Attribute.IsUnknown()` for `Optional+Computed` fields before using their pointer methods.

> **Linter:** `requestguard` — flags both redundant guards (dead code) and missing guards (bug risk) in request builder functions.

## Variable Naming

Variable names must match their data source:

- `req.Plan.Get(ctx, &x)` → `x` must be named `plan` (or contain "plan")
- `req.State.Get(ctx, &x)` → `x` must be named `state` (or contain "state")
- `req.State.GetAttribute(ctx, ..., &x)` → no constraint (scalar extraction)

```go
func (r *myResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
    var plan myResourceModel
    resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
}

func (r *myResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
    var state myResourceModel
    resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
}
```

> **Linter:** `crudnaming` — flags mismatches between variable names and data sources.

## Error Messages

Always include **both** status code and response body in HTTP error messages:

```go
if resp.StatusCode() != 201 {
    resp.Diagnostics.AddError(
        "Error Creating Resource",
        fmt.Sprintf("Could not create resource, status: %d, body: %s",
            resp.StatusCode(), string(resp.Body)),
    )
}
```

> **Linter:** `errformat` — flags `AddError` calls missing status code or body.

## Delete 404 Handling

Treat HTTP 404 as success in Delete — the resource is already gone:

```go
if deleteResp.StatusCode() != 200 && deleteResp.StatusCode() != 204 && deleteResp.StatusCode() != 404 {
    // error
}
```

> **Linter:** `delete404` — flags Delete methods that don't accept 404.

## Read 404 Handling (Externally Deleted Resources)

When Read returns 404, the resource was deleted outside Terraform. Call `RemoveResource()`:

**Two-part pattern:**

1. In `populateState`: Set `state.Id = types.StringNull()` on 404
2. In `Read` method: Check for null ID and call `RemoveResource()`

```go
// In populateState:
if httpResp.StatusCode() == 404 {
    state.Id = types.StringNull()
    return  // No error, just mark for removal
}

// In Read method (REQUIRED):
resp.Diagnostics.Append(r.populateState(ctx, &state)...)
if resp.Diagnostics.HasError() {
    return
}
if state.Id.IsNull() {
    resp.State.RemoveResource(ctx)
    return
}
resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
```

> **Linter:** `read404` — flags Read methods that don't call `RemoveResource`.



## Configure Error Type

The error string in `Configure()` must match the type — "Resource" for resources, "Data Source" for data sources:

```go
// Resource
func (r *myResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
    resp.Diagnostics.AddError(
        "Unexpected Resource Configure Type",  // ← must say "Resource"
        // ...
    )
}
```

> **Linter:** `configuretype` — catches mismatches (common scaffold bug).

## Interface Satisfaction Style

Use `(*type)(nil)` nil-cast style for compile-time interface checks:

```go
// CORRECT
var _ resource.Resource = (*myResource)(nil)
var _ resource.ResourceWithConfigure = (*myResource)(nil)

// WRONG
var _ resource.Resource = &myResource{}
```

> **Linter:** `interfacestyle` — flags `&type{}` in interface checks.

## Timeout Support

All resources and data sources must include timeout support. This is a two-layer architecture:

| Layer | Controls | Default |
|-------|----------|---------|
| **Request timeout** (provider `request_timeout`) | Individual HTTP request | 120s |
| **Operation timeout** (resource `timeouts = {}`) | Entire Terraform operation incl. retries | CRUD: 5m, Read: 2m |

### For Resources (3 steps)

**Step 1:** Add `Timeouts` field to model:

```go
import "github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"

type myResourceModel struct {
    resource_my.MyResourceModel
    Timeouts timeouts.Value `tfsdk:"timeouts"`
}
```

**Step 2:** Add `timeouts` to schema:

```go
resp.Schema.Attributes["timeouts"] = timeouts.Attributes(ctx, timeouts.Opts{
    Create: true, Read: true, Update: true, Delete: true,
})
```

> **CRITICAL:** Forgetting this step causes runtime errors (not compile errors).

**Step 3:** Wrap CRUD methods with `context.WithTimeout`:

```go
createTimeout, diags := plan.Timeouts.Create(ctx, 5*time.Minute)
resp.Diagnostics.Append(diags...)
if resp.Diagnostics.HasError() { return }
ctx, cancel := context.WithTimeout(ctx, createTimeout)
defer cancel()
```

### For Data Sources

Same pattern, simpler — only Read timeout. Use `datasource/timeouts` import (not `resource/timeouts`).

### Common Pitfalls

1. **Schema/struct mismatch** — model has `Timeouts` but schema doesn't → runtime error
2. **Wrong import** — resources use `resource/timeouts`, data sources use `datasource/timeouts`
3. **Nested attribute syntax** — users write `timeouts = { create = "10m" }` with `=`, not `timeouts { ... }`

> **Linter:** `timeoutcheck` — flags CRUD methods missing `context.WithTimeout`.

## API Authentication

All DoIT API requests require:

1. **Bearer token** in the `Authorization` header
2. **Customer context** via the `X-Tenant-Id` header — **only** when using a DoiT
   employee token. Set it to the target customer ID. Regular-user tokens must not
   send it (the customer is derived from the token).

```bash
# Regular user — no customer context header:
curl -X GET \
  "${DOIT_HOST}/analytics/v1/budgets" \
  -H "Authorization: Bearer ${DOIT_API_TOKEN}" \
  -H "Accept: application/json"

# DoiT employee — scope to a customer with the X-Tenant-Id header:
curl -X GET \
  "${DOIT_HOST}/analytics/v1/budgets" \
  -H "Authorization: Bearer ${DOIT_API_TOKEN}" \
  -H "X-Tenant-Id: ${DOIT_CUSTOMER_CONTEXT}" \
  -H "Accept: application/json"
```

| Account Type | `X-Tenant-Id` header | Use Case |
|-------------|----------------------|----------|
| DoiT Employee | **REQUIRED** | Access any customer's resources |
| Regular User | **MUST NOT set** | Access only own customer |

> Historical note: this customer context was formerly passed as a
> `customerContext` query parameter. The API migrated to the `X-Tenant-Id`
> header; the query parameter is no longer used.
