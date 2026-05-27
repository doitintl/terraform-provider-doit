---
name: implement-datasource
description: How to add or modify a Terraform data source in this provider. Covers scaffolding, unknown input handling, list data sources with pagination, and API response validation.
---

# Implement Data Source

Step-by-step guide for adding a new data source or modifying an existing one. Before starting, also read the [implementation-conventions](../implementation-conventions/SKILL.md) skill for shared patterns.

## Step 1: Add to datasources.yml

Add the data source definition to `OpenAPI/1_tfplugingen-openapi/datasources.yml`:

```yaml
data_sources:
  - name: my_resource
    schema:
      attributes:
        overrides:
          - name: id
            data_source:
              attribute_type:
                name: string
                optional: false # Make ID required input
```

## Step 2: Generate Code

```bash
make generate
```

This generates the schema and model types in `internal/provider/datasource_<name>/`.

## Step 3: Scaffold the Data Source

**Always use the scaffold command** — do NOT write from scratch:

```bash
go run github.com/doitintl/terraform-plugin-codegen-framework/cmd/tfplugingen-framework scaffold data-source \
  --name <name> \
  --output-file <name>_data_source.go \
  --package provider \
  --output-dir internal/provider
```

> **NOTE:** `--output-file` must be the **basename only** (e.g., `my_data_source.go`), not a full path.

Required interface declarations (verify in scaffolded file):

```go
var _ datasource.DataSource = (*myDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*myDataSource)(nil)
```

## Step 4: Implement the Data Source

### Mapping Convention

Data sources that share an entity with a resource (e.g. `alert`, `report`) should reuse the
`mapXxxToModel` helper from the companion `<name>.go` file. Data sources with their own unique
mapping (e.g. list data sources) should place helpers in the `_data_source.go` file or in a
companion file when the mapping is non-trivial.

Simple data sources that only do a few scalar assignments may keep mapping logic inline in Read.

### Implementation

- Use `*models.ClientWithResponses` for client type
- Use generated constructors (`NewXxxValue()`) for nested objects
- Add timeout support (see [implementation-conventions](../implementation-conventions/SKILL.md#timeout-support))

### Unknown Input Handling (Critical)

Data sources are read during `terraform plan`. If an input depends on an unresolved resource, its value will be `Unknown`. **Check for unknown inputs before making API calls:**

**Scalar inputs** — check with `IsUnknown()`:

```go
if data.Id.IsUnknown() {
    data.Result = types.StringUnknown()
    resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
    return
}
```

**Composite inputs** (lists, maps, objects) — use `IsFullyKnown()`:

```go
if !req.Config.Raw.IsFullyKnown() {
    data.Id = types.StringUnknown()
    data.Results = types.ListUnknown(elemType)
    resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
    return
}
```

A list can be known while containing unknown elements (e.g., `resources = [some_resource.id]`), which `IsUnknown()` on the list itself would miss.

> **IMPORTANT:** Return `Unknown`, not `Null`. `Null` means "this value does not exist" while `Unknown` means "not yet determined." Downstream consumers treat `Null` as a real value during planning.

> **Linter:** `unknownguard` — flags data source Read methods missing unknown input checks.

### API Response Validation

Validate both status code and parsed response body:

```go
if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
    resp.Diagnostics.AddError(
        "Error Reading Resource",
        fmt.Sprintf("status: %d, body: %s", apiResp.StatusCode(), string(apiResp.Body)),
    )
    return
}
```

The generated client only populates `JSON200` when the response `Content-Type` is JSON.

### Computed-Only List Attributes

Always return an empty list `[]` instead of `null` for computed-only list attributes. Returning `null` causes errors when consumers iterate (e.g., `for_each`, `for` expressions):

```go
// CORRECT — empty list is safely iterable
emptyList, diags := types.ListValueFrom(ctx, elemType, []MyValueType{})
data.Items = emptyList

// WRONG — causes "Iteration over null value" errors
data.Items = types.ListNull(elemType)
```

## Step 5: Register the Data Source

Add to `provider.go` in the `DataSources()` method.

## Step 6: Add Tests and Examples

- Test file: `internal/provider/<name>_data_source_test.go`
- Example: `examples/data-sources/doit_<name>/data-source.tf`
- Add test env vars to `.envrc.local`
- **Always `terraform apply` every example at least once** to verify

## Step 7: Generate Docs

```bash
make docs
```

---

## List Data Sources (Plural Endpoints)

List data sources (e.g., `doit_budgets`, `doit_allocations`) return arrays of items.

### Smart Pagination

List data sources support smart pagination — they honor user-provided values when set, otherwise auto-paginate:

| User Sets | Behavior |
|-----------|----------|
| Neither `max_results` nor `page_token` | Auto-paginate: fetch all pages |
| `max_results` only | Single page: return up to N items |
| `page_token` only | Auto-paginate from token |
| Both | Manual pagination |

```go
userControlsPagination := !data.MaxResults.IsNull() && !data.MaxResults.IsUnknown()

if userControlsPagination {
    params.MaxResults = new(data.MaxResults.ValueString())
    // Single API call, preserve page_token from response
} else {
    // Auto mode: fetch all pages
    var allItems []models.ItemType
    for {
        apiResp, _ := d.client.ListItemsWithResponse(ctx, params)
        allItems = append(allItems, *apiResp.JSON200.Items...)
        if apiResp.JSON200.PageToken == nil || *apiResp.JSON200.PageToken == "" {
            break
        }
        params.PageToken = apiResp.JSON200.PageToken
    }
}
```

**Key principle:** User-provided values MUST be preserved in state to prevent perpetual diffs.

### Mapping API Types to Generated Values

Use `NewXxxValue()` constructors with `map[string]attr.Value{}`:

```go
budgetVal, diags := datasource_budgets.NewBudgetsValue(
    datasource_budgets.BudgetsValue{}.AttributeTypes(ctx),
    map[string]attr.Value{
        "id":          types.StringPointerValue(budget.Id),
        "budget_name": types.StringPointerValue(budget.BudgetName),
        "amount":      types.Float64PointerValue(budget.Amount),
    },
)
```

### Type Handling Tips

1. **Pointer vs Non-Pointer**: Check `models_gen.go` — pointer (`*string`) → `types.StringPointerValue()`, non-pointer → `types.StringValue()`
2. **Enum Types**: Convert to string first — `types.StringValue(string(s.Mode))`
3. **Nested Lists**: Create helper functions to map nested structures

### RowCount Determinism

Always guarantee a deterministic fallback by calculating slice length when the API omits `rowCount`:

```go
if apiResp.JSON200.RowCount != nil {
    data.RowCount = types.Int64Value(*apiResp.JSON200.RowCount)
} else {
    data.RowCount = types.Int64Value(int64(len(items)))
}
```

### Common Mistake

Generated schema field names match the API's JSON tags in snake_case, NOT arbitrary Terraform conventions. Verify names against `datasource_xxx/xxx_data_source_gen.go` and `models/models_gen.go`.
