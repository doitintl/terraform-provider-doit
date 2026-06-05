---
name: implement-resource
description: How to add or modify a Terraform resource in this provider. Covers scaffolding, the plan-first overlay pattern, state consistency, CRUD implementation, and import support.
---

# Implement Resource

Step-by-step guide for adding a new resource or modifying an existing one. Before starting, also read the [implementation-conventions](../implementation-conventions/SKILL.md) skill for shared patterns.

## Step 1: Add to resources.yml

Add the resource definition to `OpenAPI/1_tfplugingen-openapi/resources.yml`.

## Step 2: Generate Code

```bash
make generate
```

This generates the schema and model types in `internal/provider/resource_<name>/`.

## Step 3: Scaffold the Resource

**Always use the scaffold command** — do NOT write the file from scratch:

```bash
go run github.com/doitintl/terraform-plugin-codegen-framework/cmd/tfplugingen-framework scaffold resource \
  --name <name> \
  --output-file <name>_resource.go \
  --package provider \
  --output-dir internal/provider
```

> **NOTE:** `--output-file` must be the **basename only** (e.g., `my_resource.go`), not a full path.

Required interface declarations (verify in scaffolded file):

```go
var _ resource.Resource = (*myResource)(nil)
var _ resource.ResourceWithConfigure = (*myResource)(nil)
var _ resource.ResourceWithImportState = (*myResource)(nil)
```

## Step 4: Implement CRUD Methods

All resources use the **plan-first overlay pattern** and the **standard helper functions** defined in [implementation-conventions](../implementation-conventions/SKILL.md#standard-helper-functions). See [Overlay Pattern Reference](references/overlay-pattern.md) for the overlay deep-dive.

### Helper functions (in `<name>.go`)

Every resource must implement these in `<name>.go`:
- `populateState` — wraps API GET + `mapXxxToModel`. Sets `state.Id = null` on 404.
- `mapXxxToModel` — pure mapping from API response to TF model.
- `overlayXxxComputedFields` — two-phase overlay for Create/Update.
- `toXxxRequest` (or `toCreateRequest` / `toUpdateRequest`) — converts TF model to API request.

### CRUD flow

| Method | Flow |
|--------|------|
| **Create** | `plan.toXxxRequest(ctx)` → API call → `overlayXxxComputedFields()` → `resp.State.Set()` |
| **Read** | `r.populateState(ctx, &state)` → check `state.Id.IsNull()` → `RemoveResource()` or `resp.State.Set()` |
| **Update** | Same as Create (get ID from state) |
| **Delete** | Treat 404 as success |
| **ImportState** | `ImportStatePassthroughID` |

**Invariant:** Create/Update = overlay wrapper. Read/Import = full mapping via populateState. Never mix these.

> **Linters:** `overlaycheck`, `overlayinvariant` — enforce overlay pattern correctness.

## Step 5: Register the Resource

Add to `provider.go` in the `Resources()` method.

## Step 6: Add Tests and Examples

- Test file: `internal/provider/<name>_resource_test.go`
- Example: `examples/resources/doit_<name>/resource.tf`
- Add test env vars to `.envrc.local`
- **Always `terraform apply` every example at least once** to verify the config is accepted

## Step 7: Generate Docs

```bash
make docs
```

---

## State Consistency Patterns

### Timestamp Preservation

When the user provides a timestamp like `2024-01-15T00:00:00-05:00` but the API returns UTC (`2024-01-15T05:00:00Z`), check semantic equality before overwriting:

```go
existingTime, err := time.Parse(time.RFC3339, state.Timestamp.ValueString())
if err == nil && existingTime.Equal(resp.Timestamp) {
    // Keep existing string to avoid diffs
} else {
    state.Timestamp = types.StringValue(resp.Timestamp.UTC().Format(time.RFC3339))
}
```

### List Attribute Mapping

Terraform distinguishes `[]` (empty list) from `null` (no value). For user-configurable list attributes, **always return an empty list** when the API returns nil:

```go
if resp.Rules != nil && len(*resp.Rules) > 0 {
    // Map the list
} else {
    // Return empty list — never types.ListNull() for user-configurable attrs
    state.Rules, d = types.ListValueFrom(ctx, elemType, []MyValueType{})
    diags.Append(d...)
}
```

> **Linter:** `listnullread` — flags `types.ListNull()` in Read paths for user-configurable lists.

### Type-Safe Empty Lists

Use the specific generated type instead of `[]attr.Value{}` for type safety:

```go
// GOOD — type-safe
emptyScopes, d := types.ListValueFrom(ctx, scopeType, []resource_report.ScopesValue{})

// OK but less safe — for primitive types
emptyStrings, d := types.ListValueFrom(ctx, types.StringType, []attr.Value{})
```

### API Default Echo Preservation

Some API fields return a default value instead of echoing the input. In Read paths, prefer the prior state value and fall back to API response only for ImportState:

```go
includeNullVal := types.BoolValue(false)
if i < len(existingIncludeNull) && existingIncludeNull[i] != nil {
    includeNullVal = types.BoolValue(*existingIncludeNull[i])  // state wins
} else if scope.IncludeNull != nil {
    includeNullVal = types.BoolValue(*scope.IncludeNull)       // API fallback
}
```

### UseStateForUnknown for Stable Fields

Add `UseStateForUnknown()` plan modifiers to Computed-only fields that never change after creation:

| Safe ✅ | NOT safe ❌ |
|---------|------------|
| `id`, `create_time`, `type` | `update_time`, `current_utilization` |

> **Linter:** `usestatefunknown` — flags stable Computed-only fields missing this modifier.

### Clearing Optional+Computed Attributes

For `Optional+Computed` attributes, Terraform Core copies the prior state value into the `ProposedNewState` when the config value is null. The framework then skips its `MarkComputedNilsAsUnknown` phase because `ProposedNewState` already equals `PriorState`. This makes it **impossible for users to clear** the attribute by setting it to `null` or omitting it — the plan silently preserves the old value.

Apply the `useNullForUnknownWhenConfigNull()` plan modifier to any `Optional+Computed` attribute that users should be able to clear:

```go
// In Schema():
if nested, ok := rrAttr.NestedObject.Attributes["metadata"].(schema.StringAttribute); ok {
    nested.PlanModifiers = append(nested.PlanModifiers, useNullForUnknownWhenConfigNull())
    rrAttr.NestedObject.Attributes["metadata"] = nested
}
```

**When to use:**
- ✅ User-provided fields where absent/null means "clear" (e.g. `metadata`, `external_id`, `description`)
- ✅ Resources with PUT (full-replacement) semantics
- ❌ Fields where the API computes a server-side default when the field is absent — the modifier would clear the computed value on every plan

**Important:** Terraform does not distinguish "attribute omitted" from "attribute explicitly set to null." Both result in a null config value. With this modifier, omitting an attribute from config will clear it rather than preserve the prior value.

The overlay (`IsUnknown()` check) correctly interacts with this modifier: when the modifier sets the plan to null (not unknown), the overlay leaves it as null.

See: [`planmodifier_null_on_config_null.go`](internal/provider/planmodifier_null_on_config_null.go) and [framework issue #603](https://github.com/hashicorp/terraform-plugin-framework/issues/603).

### RFC3339 Timestamp Validation

Use the `rfc3339Validator` for schema-level validation of timestamp attributes. This provides early feedback at plan time instead of apply time.

```go
if timestamp, ok := s.Attributes["timestamp"]; ok {
    if strAttr, ok := timestamp.(schema.StringAttribute); ok {
        strAttr.Validators = append(strAttr.Validators, rfc3339Validator{})
        s.Attributes["timestamp"] = strAttr
    }
}
```

## OpenAPI Spec vs. Go Types

The Go models from OpenAPI may differ between Request and Response wrappers:

- Response `Scopes`: `[]ExternalConfigFilter`
- Request `Scopes`: `*[]ExternalConfigFilter`

Check `models/models_gen.go` when compilation fails. You may need `&slice` for requests but use the slice directly from responses.
