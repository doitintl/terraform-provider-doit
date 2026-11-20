---
name: implement-datasource
description: How to add or modify a Terraform data source in this provider. Covers scaffolding, unknown input handling, list data sources with pagination, and API response validation.
---

# Implement Data Source

Step-by-step guide for adding a new data source or modifying an existing one. Before starting, also read the [implementation-conventions](../implementation-conventions/SKILL.md) skill for shared patterns. When adding or changing validation, also read [implement-validator](../implement-validator/SKILL.md).

## Step 1: Add to datasources.yml

Add the data source definition to `OpenAPI/1_tfplugingen-openapi/datasources.yml`:

```yaml
data_sources:
  my_resource:
    read:
      path: /service/v1/my-resources/{id}
      method: GET
```

The read operation's path parameters become the data source's required inputs, so an `{id}` item path gives the data source a required `id` that merges with the response `id`. A resource-named parameter (`{myResourceId}`) would make `my_resource_id` the input instead; that is an upstream finding (see [evaluate-api](../evaluate-api/SKILL.md), Step 3).

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

### Unknown Input Handling

When data source configuration contains an unknown value, Terraform Core versions covered by this provider's acceptance suite (`> 1.10`) defer the read until apply and mark the complete data source result unknown. This is the normal practitioner-facing planning behavior; Core does not ordinarily call `ReadDataSource` with that configuration.

The plugin protocol nevertheless permits a client to pass unknown configuration to `ReadDataSource`. Keep a defensive unknown guard before request construction and API calls so direct protocol clients or future execution paths cannot turn unknown values into zero-value request parameters or perform premature I/O.

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

If the defensive fallback writes state, it must predict what a real read of the same configuration writes:

- Every value the read takes from the API is `Unknown`, not `Null`. `Null` means "this value does not exist" while `Unknown` means "not yet determined." That includes every computed-only attribute, not just the main list.
- Every value the read leaves as configured stays as configured, known, unknown or null. An omitted Optional+Computed filter that the API never echoes stays null, because a real read leaves it null.

Decide each attribute from what this data source's own read writes, not from its schema class. `Optional+Computed` covers both cases: a filter is an input the read keeps, while `page_token` in a list data source is a result, because every read replaces it (see [Unknown guard](#unknown-guard)).

> **Linter:** `unknownguard` — flags data source `Read` methods missing a defensive unknown-configuration guard. For list data sources (`max_results`, `page_token` and `row_count`) it also requires a `!req.Config.Raw.IsFullyKnown()` guard (alone or ORed with other conditions; an ANDed or inverted check lets unknown configuration through) as a statement of the `Read` body itself, not nested in another branch, that sets `PageToken` and `RowCount` unknown unconditionally and ends with `return`. Otherwise it checks guard presence only.

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

This check only sees 2xx, 3xx, 404, 412 and 425: the provider's retry client turns every other status into an error (see [Statuses resource code can see](../implementation-conventions/SKILL.md#statuses-resource-code-can-see)). To give a hint for another status, match it in the `err` branch, the way `resultsTooLargeMarker` (`async_report.go`) handles a 413:

```go
const invalidCursorMarker = "non-retryable error: 400"

apiResp, err := d.client.ListThingsWithResponse(ctx, params)
if err != nil {
    detail := fmt.Sprintf("Unable to read things: %v", err)
    if strings.Contains(err.Error(), invalidCursorMarker) && params.PageToken != nil {
        detail += "\nThe page_token may be invalid or expired."
    }
    resp.Diagnostics.AddError("Error Reading Things", detail)
    return
}
```

A `StatusCode() == 400` branch compiles and passes a unit test built on a plain `models.NewClientWithResponses`, but never runs in the provider. Test it through the retry client (see the testing skill, [Unit Test HTTP Mocking](../testing/SKILL.md#unit-test-http-mocking-with-httptestnewtestserver)).

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
- **Always `terraform apply` every example at least once** to verify (see [Applying an example](#applying-an-example))

### Applying an example

Examples have no `terraform` block, so on their own Terraform resolves `doit` to `hashicorp/doit`. Apply one against a locally built provider in a scratch directory:

```bash
go build -o "$PWD/terraform-provider-doit" .
scratch=$(mktemp -d)
cat > "$scratch/dev.tfrc" <<TFRC
provider_installation {
  dev_overrides {
    "doitintl/doit" = "$PWD"
  }
  direct {}
}
TFRC
cat > "$scratch/providers.tf" <<'TF'
terraform {
  required_providers {
    doit = { source = "doitintl/doit" }
  }
}
TF
cp examples/data-sources/doit_<name>/data-source.tf "$scratch/"
(cd "$scratch" && TF_CLI_CONFIG_FILE="$scratch/dev.tfrc" terraform apply)
```

With `dev_overrides` there is no `terraform init` step; Terraform warns that the override is active, which confirms it uses the local binary. `make validate-examples` (`scripts/validate_examples.sh`) sets up the same override, but only runs `terraform validate`. Data source examples only read, so applying them against the test tenant is safe; for resources, apply and then destroy.

## Step 7: Generate Docs

```bash
make docs
```

---

## List Data Sources (Plural Endpoints)

List data sources (e.g., `doit_budgets`, `doit_allocations`) return arrays of items. Their user-facing contract is the Pagination Guide (`templates/guides/pagination.md`, rendered to `docs/guides/pagination.md`). Read it first, and add a new paged data source to its table.

### Smart Pagination

`max_results` set reads a single page; omitted, every page is fetched.

| | `max_results` set (single page) | `max_results` omitted (auto) |
|---|---|---|
| Requests | one, from `page_token` if set | every page, from `page_token` if set |
| `page_token` in state | the API's next token; null after the last page | null |
| `max_results` in state | as configured | null |
| `row_count` | see [Row Count](#row-count) | see [Row Count](#row-count) |

Each computed attribute means the same in every mode. `page_token` in state is always the token of the next page, and after reading every page there is none. A configured `page_token` is where the read starts; it is not kept in state.

```go
if !data.PageToken.IsNull() {
    params.PageToken = new(data.PageToken.ValueString())
}

var items []models.Item
if !data.MaxResults.IsNull() {
    params.MaxResults = new(data.MaxResults.ValueInt64())
    page, ok := d.listPage(ctx, params, &resp.Diagnostics)
    if !ok {
        return
    }
    items = page.Items
    data.PageToken = types.StringPointerValue(page.PageToken)
    data.RowCount = listRowCount(nullableToPointer(page.RowCount), len(items))
} else {
    // rowCount is the total across all pages, the same on every page, so it
    // is taken from the first page even when a page_token starts the read
    // after the first item.
    var apiRowCount *int64
    for first := true; ; first = false {
        page, ok := d.listPage(ctx, params, &resp.Diagnostics)
        if !ok {
            return
        }
        if first {
            apiRowCount = nullableToPointer(page.RowCount)
        }
        items = append(items, page.Items...)
        if page.PageToken == nil || *page.PageToken == "" {
            break
        }
        params.PageToken = page.PageToken
    }
    // Without a count from the API, the items fetched over all pages stand in for it.
    data.RowCount = listRowCount(apiRowCount, len(items))
    data.PageToken = types.StringNull()
    data.MaxResults = types.Int64Null()
}
```

This is the total-kind form. A page-kind data source sets `data.RowCount = types.Int64Value(int64(len(items)))` after the loop instead.

- **End of the list.** Only an absent or empty token ends the loop. A page can be empty while the token is set, for example when the API stops after scanning a fixed number of slices and a filter removed every item it scanned. Never stop on an empty page.
- **Envelope types.** Read the generated response type in `models/models_gen.go` before writing the mapping. `rowCount` is `*int`, `*int64`, `nullable.Nullable[int64]` or a required `int64` depending on the endpoint, and `pageToken` can be `nullable.Nullable[string]`. Pass `nullableToPointer(...)` for nullable fields and `&page.RowCount` for a required one.

### Row Count

`row_count` has one meaning per data source, chosen from what the endpoint's `rowCount` counts. The spec wording is not reliable for this: `allocations`, `labels` and `annotations` describe "the total number … in the result set" and return the page size. Measure the endpoint on the test tenant before choosing: send one request with a small `maxResults` (2) and compare its `rowCount` with a full crawl (see the evaluate-api skill, [Pagination Support](../evaluate-api/SKILL.md#step-8-pagination-support-list-endpoints)).

| Kind (`listRowCountKind`) | API `rowCount` | Single page | Auto |
|---|---|---|---|
| `listRowCountTotal` | the total across pages, the same on every page | `listRowCount(api, len(page))` | `listRowCount(first page's api, len(all))`, after the loop |
| `listRowCountPage` | the items in this response | `listRowCount(api, len(page))` | `len(all)` |
| `listRowCountTotalUnlessPaged` | the total, sent only when one request saw every item | the API value, or **null** | `len(all)` |

- `listRowCount` (`list_data_source_pagination.go`) returns the API value, or the fallback when the API sent none. In auto mode the fallback is the number of items over **all** pages, so compute it after the loop. A fallback computed on the first page reports the size of that page. Taking a page-kind endpoint's first-page `rowCount` as a total does the same.
- Use the null form only when the spec says the count is unknown when paged. A page size reported as a total is worse than no total.
- Record the kind in the shared test table (see the testing skill, [List Data Sources](../testing/SKILL.md#list-data-sources)) and in the Pagination Guide's table.

### Unknown guard

```go
// The results are not yet known. page_token is one of them in every mode:
// a read replaces it with the next page's token, or null. The inputs stay
// as configured.
if !req.Config.Raw.IsFullyKnown() {
    data.Items = types.ListUnknown(datasource_things.ItemsValue{}.Type(ctx))
    data.RowCount = types.Int64Unknown()
    data.PageToken = types.StringUnknown()
    resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
    return
}
```

Set every other computed-only attribute unknown too (`risk_aggregations`, `total_count`, …). Leave `max_results`, filters and every other input as configured, and don't turn a null one into unknown. Use `IsFullyKnown()`, not a list of `IsUnknown()` checks, which misses any input it doesn't name, `timeouts` included.

### Descriptions

The generated `max_results`, `page_token` and `row_count` descriptions come from the API envelope and are written for API clients ("Use the page tokens to iterate through the entire collection", "MAY return `null`"). Replace them in `Schema()`:

```go
s := datasource_things.ThingsDataSourceSchema(ctx)
resp.Diagnostics.Append(setListPaginationDescriptions(ctx, &s, listRowCountTotal, "")...)
```

The helper type-asserts the three attributes with checked assertions (`forcetypeassert` is enabled) and sets the text for the row-count kind. The `max_results` text ends with what its validators enforce ("Value must be between 1 and 500."), since the generated text it replaces was often the only place the range was documented. A limit the spec only states in prose, without `minimum`/`maximum`, has no validator; state it in the data source's description (`extended_metrics`). Its last argument is appended to the `page_token` description, for what only this endpoint's tokens do: a token that only works with the filters it was returned for, or one that expires (see `composer_recipes_data_source.go`). Don't drop such details from the generated text; move them into the note. Other endpoint details, such as a maximum page size, belong in the data source's description. A data source that never auto-paginates uses `setSinglePageListPaginationDescriptions`. Back each statement with a test.

### Examples that chain `page_token`

A null `page_token` together with `max_results` reads the **first** page. So once the first page is also the last one, a second data source fed from its token starts over at the beginning. Create a chained read only while there is a token, and read its outputs with `one()`:

```hcl
data "doit_things" "page2" {
  count       = data.doit_things.page1.page_token != null ? 1 : 0
  max_results = 5
  page_token  = data.doit_things.page1.page_token
}

output "page2_token" {
  value = one(data.doit_things.page2[*].page_token)
}
```

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
2. **Nullable fields**: `nullable.Nullable[T]` → wrap with `nullableToPointer()` before passing to `types.XxxPointerValue()`. Example: `types.StringPointerValue(nullableToPointer(resp.Field))`
3. **Enum Types**: Convert to string first — `types.StringValue(string(s.Mode))`
4. **Nested Lists**: Create helper functions to map nested structures

### Common Mistake

Generated schema field names match the API's JSON tags in snake_case, NOT arbitrary Terraform conventions. Verify names against `datasource_xxx/xxx_data_source_gen.go` and `models/models_gen.go`.
