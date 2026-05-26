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

Data sources use a single file: `<name>_data_source.go` (mapping logic inline in Read).

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
| `populateState` | `(r *xResource) populateState(ctx, state *xResourceModel) diag.Diagnostics` | Fetches from API using the identifier in `state` (e.g. `state.Id`), calls `mapXxxToModel`. Sets `state.Id = types.StringNull()` on 404. Used by Read only. |
| `mapXxxToModel` | `mapXxxToModel([ctx,] apiResp, state) [diag.Diagnostics]` | Pure mapping from API response to TF model. **Standalone function — no receiver.** Used by `populateState` and as Phase 1 of overlay. |
| `overlayXxxComputedFields` | `overlayXxxComputedFields([ctx,] apiResp, plan) [diag.Diagnostics]` | Two-phase overlay. **Standalone function — no receiver. Always prefix with the resource name** (e.g. `overlayReportComputedFields`, not `overlayComputedFields`). Used by Create/Update only. |
| `toXxxRequest` | `(plan *xResourceModel) toXxxRequest([ctx]) (req[, diag.Diagnostics])` | **Method on the plan model**, converts TF model to API request. When create and update share a request type, name it `toUpdateRequest`. |

If create and update use different API request types, implement both `toCreateRequest` and `toUpdateRequest`.

> **Receiver rule:** Only `populateState` has a receiver (it needs `r.client`). `mapXxxToModel` and `overlayXxxComputedFields` must be standalone functions. `toXxxRequest` / `toCreateRequest` / `toUpdateRequest` are methods on the plan model.

> **Signature flexibility:** The `ctx` and `diag.Diagnostics` parameters are required when the function maps nested objects (lists, objects) or can produce errors. Simple resources that only do scalar assignments (e.g. `types.StringValue`, `types.StringPointerValue`) may omit `ctx` and return nothing. Match the complexity of your resource.

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
2. **Customer context** via `customerContext` query parameter

```bash
curl -X GET \
  "${DOIT_HOST}/analytics/v1/budgets?customerContext=${DOIT_CUSTOMER_CONTEXT}" \
  -H "Authorization: Bearer ${DOIT_API_TOKEN}" \
  -H "Accept: application/json"
```

| Account Type | customerContext | Use Case |
|-------------|----------------|----------|
| DoiT Employee | **REQUIRED** | Access any customer's resources |
| Regular User | **MUST NOT set** | Access only own customer |
