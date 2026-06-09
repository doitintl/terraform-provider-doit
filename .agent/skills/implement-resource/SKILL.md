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

| Method          | Flow                                                                                                  |
| --------------- | ----------------------------------------------------------------------------------------------------- |
| **Create**      | `plan.toXxxRequest(ctx)` → API call → `overlayXxxComputedFields()` → `resp.State.Set()`               |
| **Read**        | `r.populateState(ctx, &state)` → check `state.Id.IsNull()` → `RemoveResource()` or `resp.State.Set()` |
| **Update**      | Same as Create (get ID from state)                                                                    |
| **Delete**      | Treat 404 as success                                                                                  |
| **ImportState** | `ImportStatePassthroughID`                                                                            |

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

> **Exception — Clearable lists (Category A):** Lists with the `useNullForUnknownListWhenConfigNull()` modifier use a **state-aware Read path** that preserves null when the prior state was null. See [Clearable List Attributes](#clearable-list-attributes) below.

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

| Safe ✅                     | NOT safe ❌                          |
| --------------------------- | ------------------------------------ |
| `id`, `create_time`, `type` | `update_time`, `current_utilization` |

> **Linter:** `usestatefunknown` — flags stable Computed-only fields missing this modifier.

### Clearing Optional+Computed Attributes

For `Optional+Computed` attributes, Terraform Core copies the prior state value into the `ProposedNewState` when the config value is null. The framework then skips its `MarkComputedNilsAsUnknown` phase because `ProposedNewState` already equals `PriorState`. This makes it **impossible for users to clear** the attribute by setting it to `null` or omitting it — the plan silently preserves the old value.

Not all Optional+Computed attributes should be clearable. **Every Optional+Computed attribute without a `Default` requires a conscious classification decision:**

> **Linter:** `clearableattr` — flags Optional+Computed attributes without Default that are missing either the `useNullForUnknownWhenConfigNull()` modifier or a `//nolint:clearableattr` suppression.

#### Category A: Clearable (user-controlled)

Apply the `useNullForUnknownWhenConfigNull()` plan modifier. Null config means "clear this value." Use typed variants for non-string types:

| Type | Modifier |
|------|----------|
| `schema.StringAttribute` | `useNullForUnknownWhenConfigNull()` |
| `schema.BoolAttribute` | `useNullForUnknownBoolWhenConfigNull()` |
| `schema.ListAttribute` / `schema.ListNestedAttribute` | `useNullForUnknownListWhenConfigNull()` |

Examples of clearable attributes:
- User-authored content: `description`, `labels`, `reports`, `metadata`
- Explicit associations: `external_id`, `external_url`, `report_url`
- Resources with PUT (full-replacement) semantics where omitting means "remove"

```go
// In Schema() — scalar attribute:
if nested, ok := rrAttr.NestedObject.Attributes["metadata"].(schema.StringAttribute); ok {
    nested.PlanModifiers = append(nested.PlanModifiers, useNullForUnknownWhenConfigNull())
    rrAttr.NestedObject.Attributes["metadata"] = nested
}

// In Schema() — list attribute:
if attr, ok := s.Attributes["labels"].(schema.ListAttribute); ok {
    attr.PlanModifiers = append(attr.PlanModifiers, useNullForUnknownListWhenConfigNull())
    s.Attributes["labels"] = attr
}
```

> **⚠️ External change consequence:** With this modifier, if the value is changed outside Terraform (e.g., via the Console UI) and the user's config doesn't include the attribute, Terraform will plan to **clear it on the next apply**. This is "config is source of truth" behavior — if you didn't configure it, it shouldn't exist. This is correct for user-controlled fields but would cause unwanted churn for API-computed defaults.

#### Clearable List Attributes

List attributes with `useNullForUnknownListWhenConfigNull()` require a **three-part fix** across overlay, Read, and Update to maintain null↔[] consistency. All three changes are required — any one missing causes drift.

**Part 1: Overlay — resolve Unknown to null (not API response)**

When the user omits a clearable list, the overlay must resolve Unknown to `null` (matching the modifier's semantics) instead of the API response (which would be `[]`):

```go
// In overlayXxxComputedFields:
// ── Labels: Optional+Computed clearable list ──
if plan.Labels.IsUnknown() {
    plan.Labels = types.ListNull(types.StringType)
}
```

**Part 2: Read — state-aware nil mapping**

The Read path must preserve null when the prior state was null (from clearing). When the prior state was non-null, return `[]` as before:

```go
// In mapXxxToModel:
if resp.Labels != nil && len(*resp.Labels) > 0 {
    // API has values — map normally
    state.Labels, d = types.ListValueFrom(ctx, types.StringType, labels)
} else if !state.Labels.IsNull() { //nolint:listnullread // clearable: preserve null
    // Prior state was non-null — return empty list (existing behavior)
    state.Labels, d = types.ListValueFrom(ctx, types.StringType, []string{})
}
// If state.Labels IS null (from clearing), don't touch it — preserves null
```

This works because `mapXxxToModel` receives the state struct with prior values already loaded from `req.State.Get(ctx, &state)` in Read. The `state.Labels.IsNull()` check tells us whether the prior state was null.

| Scenario | Prior state | API returns | Read produces | Stable? |
|----------|-------------|-------------|---------------|---------|
| Create, list omitted | null (overlay) | nil | null (preserved) | ✅ |
| Create, list set | `["a"]` | `["a"]` | `["a"]` | ✅ |
| Clear (remove from config) | null (modifier→overlay) | nil | null (preserved) | ✅ |
| Import, no values | null (default) | nil | null | ✅ |
| Import, has values | null (default) | `["a"]` | `["a"]` | ✅ |
| Explicit empty `[]` | `[]` | nil | `[]` | ✅ |

**Part 3: Update request — send empty list on null**

The API treats an omitted field as "no change", so clearing must send an explicit empty list:

```go
// In toUpdateRequest:
if plan.Labels.IsNull() {
    emptyLabels := []string{}
    req.Labels = &emptyLabels
} else if !plan.Labels.IsUnknown() {
    var labels []string
    diags.Append(plan.Labels.ElementsAs(ctx, &labels, false)...)
    req.Labels = &labels
}
```

> **Note on Computed-only fields:** When the list clearing modifier triggers an Update, Computed-only timestamp fields like `update_time` must be guarded with `IsUnknown()` in the overlay to avoid "inconsistent result" errors. On Create the field is Unknown (overlay sets it); on modifier-triggered Updates the field is Known (overlay preserves it; the next Read fetches the new value).

```go
// In overlayXxxComputedFields:
if plan.UpdateTime.IsUnknown() {
    plan.UpdateTime = resolved.UpdateTime
}
```

#### Category B: Not clearable (API-computed default)

**Do not add any plan modifier.** The default framework behavior (prior state sticks) is correct. Add an explicit `//nolint:clearableattr` block in the Schema() method to signal that this attribute was consciously classified:

```go
// In Schema():
if attr, ok := s.Attributes["currency"].(schema.StringAttribute); ok { //nolint:clearableattr // API-computed default (org currency)
    s.Attributes["currency"] = attr
}
```

- Fields where the API assigns a meaningful default: `currency`, `time_interval`
- Fields where the API always populates the field on Create: `recipients` (defaults to creator's email)
- Fields tied to server-side identity or assignment: `role_id`, `status`
- Fields where `null` is not a valid API state (server always provides a value)

> **Note:** `UseStateForUnknown()` is for **Computed-only** stable fields (`id`, `create_time`), not for Optional+Computed. It is a no-op on Optional+Computed because Terraform Core copies prior state before the modifier runs, so the value is never Unknown.

#### Classification rules

| Question                                                                                             | If yes →                       |
| ---------------------------------------------------------------------------------------------------- | ------------------------------ |
| Is `null`/absent a valid, stable state in the API? (API stores null, doesn't replace with a default) | **Category A** — clearable     |
| Is the field purely user-authored content with no server-side semantics?                             | **Category A** — clearable     |
| Does the API assign a non-null default when the field is omitted from the request?                   | **Category B** — not clearable |
| Does the API always return a value regardless of what was sent?                                      | **Category B** — not clearable |

#### Testing requirements

Every clearable attribute (Category A) must have a clearing lifecycle test:

```go
// Step 1: Create with attribute SET → verify value in state
// Step 2: Drift check (ExpectEmptyPlan)
// Step 3: Clear attribute (omit from config) → ExpectResourceAction(..., ResourceActionUpdate)
// Step 4: Drift check (ExpectEmptyPlan) → confirms cleared value is stable
```

For omitted clearable lists, verify the state is null (not empty):

```go
// In tests where a clearable list is omitted from config:
statecheck.ExpectKnownValue(
    "doit_annotation.omitted_lists",
    tfjsonpath.New("labels"),
    knownvalue.Null()),  // NOT ListSizeExact(0)
```

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
