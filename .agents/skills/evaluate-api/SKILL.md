---
name: evaluate-api
description: Evaluate a DoiT API endpoint or API spec PR for Terraform provider compatibility. Run the code generation pipeline as a test, check path parameters, reserved attribute names, schema composition, CRUD support, response behavior, field normalization, and identify spec issues. Do NOT fix issues in the OpenAPI spec and do NOT work around them — flag them upstream.
---

# Evaluate API Endpoint

Use this skill when evaluating a new DoiT API endpoint for Terraform resource or data source compatibility. This applies when an endpoint is being planned, is in draft (including a `doiteng/api-specs` pull request), or was recently added.

## Scope: new contracts vs. legacy endpoints

The checks below describe what a **new or changed API contract** must provide. They are not a description of every endpoint the provider already supports. Many endpoints predate the provider, and some shipped resources carry provider-side workarounds for them: `asset` (no DELETE), `customer` (create by PATCH), `user` (read mapped to PATCH), `sharing` and `active_theme` (PUT singletons), `datahub_dataset` (`{name}` key), unpaginated lists. **Do not cite those as precedent** for a new contract, and do not hold a new contract to their shape.

For a legacy endpoint whose contract will not change (frozen v1 content), the API owner has to confirm that it is frozen. The verdict may then be **Suitable with documented workaround**. Name the workaround and where it lives (hand-written code, never the forbidden list below). For everything else the forbidden workarounds apply without exception.

### Sources of truth

- The DoiT API Design Standards, as vendored in `doiteng/api-ruleset` under `standards/`. §13 "Designing for Terraform Providers" is the chapter that matters, and §7.5 covers identifiers. Cite the section in findings so the API team can look it up.
- The `doit-lint` ruleset in `doiteng/api-ruleset`: `doit-ids-item-path-param-id` and the `doit-terraform-*` rules check most of the mechanical items below on the 3.1 source spec. Run them first (see Step 1). Do not repeat a lint finding as your own; reference it.
- The generator itself (`github.com/doitintl/terraform-plugin-codegen-openapi`, pinned in `go.mod`). When the standards and the generator disagree, the generator decides what works. Report the disagreement, too.

## Critical Rules

This skill **evaluates**. It does not fix, adapt, or compensate. The deliverable is a verdict plus a findings list — never a patched pipeline.

> **NEVER modify the OpenAPI spec (`OpenAPI/openapi_spec_full.yml`)** — it is a carbon copy of the upstream version and all manual changes WILL be overwritten on the next sync. If there are issues with the spec, flag them as findings.

> **The schema must come from the code generator.** If the generator produces an incorrect, incomplete, or invalid schema, that is a finding — not something to fix locally.

> **An incompatible spec is a BLOCKED verdict, not a puzzle to solve.** Report the incompatibility and stop. A blocked evaluation delivered honestly is a success; a working implementation built on a local deviation from the API contract is a failure.

### Forbidden workarounds

Every item below looks clever and is wrong. If you catch yourself reaching for one, that is the signal to stop and write the finding instead.

| Do not | Why it is wrong |
|--------|-----------------|
| Rename an attribute anywhere in the pipeline | The Terraform attribute permanently diverges from the API contract. The next upstream sync either silently reverts it or double-applies it. |
| Edit `OpenAPI/openapi_spec_full.yml` | Carbon copy of upstream; overwritten on sync. |
| Edit `OpenAPI/extra_paths.yml` or `tools/extract-inline-schemas` to reshape the schema | Same divergence, hidden one layer deeper where the next reader will not find it. |
| Add an alias or override in `OpenAPI/1_tfplugingen-openapi/{datasources,resources}.yml` to dodge a name | Hides an upstream contract bug behind provider-local config. |
| Hand-edit or post-process a `*_gen.go` file | Regenerated on every `make generate`. |
| Drop or exclude the offending field | Silent data loss — users cannot see a field the API returns. |
| Wrap the object in an artificial nesting level to move a field off the schema root | Invents an API shape that does not exist, purely to defeat a validation rule. |
| Downgrade to "data source only" to sidestep a schema-level problem | Reserved and invalid attribute names apply identically to data sources. This solves nothing. |

The correct output for any of these situations is: **the API contract must change upstream before this endpoint can be implemented.**

## Step 1: Run the Code Generation Pipeline (Test Only)

**Get the spec.** The pipeline consumes the OpenAPI 3.0.1 spec in `doiteng/omni`:

```bash
make sync-spec PR=<omni-pr>   # or REF=<branch>; replaces OpenAPI/openapi_spec_full.yml
make apply-pr PR=<omni-pr>    # alternative: apply only the PR's diff to the synced spec
```

A contract that exists only as a `doiteng/api-specs` PR is OpenAPI 3.1, and the pipeline cannot consume it yet. `tools/extract-inline-schemas` reads `type` as a single string, and the hand-written code relies on 3.0 `nullable: true`. For such a PR:

1. Run the ruleset against the source spec (the rules need no bundling):

   ```bash
   # in a doiteng/api-ruleset checkout, from its root
   go build -o doit-lint ./cmd/doit-lint
   ./doit-lint -r ruleset.yaml <api-specs>/services/external/<svc>/v<n>/<svc>.openapi.yaml \
     | jq '.[] | select(.code | test("doit-terraform-|doit-ids-")) | {code, line: .range.start.line, message}'
   ```

2. Evaluate the rest by reading the spec against the steps below.
3. Run the pipeline once the 3.0.1 projection reaches omni.

If the spec for the endpoint is present, run the full code generation pipeline **as a test** — not an actual implementation:

```bash
# 1. Add the endpoint to datasources.yml or resources.yml
# 2. Run the generator
make generate
# 3. Inspect the generated output, and the generator log: mapping failures
#    are warnings, not a failed build (see Step 4)
ls internal/provider/datasource_<name>/  # or resource_<name>/
cat internal/provider/datasource_<name>/*_gen.go
```

**What to look for in the generated output:**

- Are all expected fields present?
- Are field types correct (string vs int64 vs bool)?
- Are pointer/non-pointer types correct?
- Are required/optional/computed classifications correct?
- Does the generated schema compile?

**After evaluation, discard the generated changes** — do not commit them:

```bash
git checkout -- OpenAPI/ internal/provider/
# Remove what the run added. git clean only touches untracked files, so the
# tracked schemas of an existing resource or data source stay. Review first:
git clean -n -- internal/provider/
git clean -fd -- internal/provider/datasource_<name>/ internal/provider/resource_<name>/
```

## Step 2: Reserved and Invalid Attribute Names

Run this immediately after generation, before spending effort on the rest of the evaluation — it is the fastest hard fail, and no amount of downstream work rescues an endpoint that trips it.

Terraform reserves a set of names at the **root** of every resource and data source schema, because a practitioner would otherwise need special configuration syntax to set them. The framework rejects them at schema-validation time.

Source of truth: `internal/fwschema/attribute_name_validation.go` in `hashicorp/terraform-plugin-framework`.

| Scope | Reserved names |
|-------|----------------|
| Resource **and** data source root | `connection`, `count`, `depends_on`, `for_each`, `lifecycle`, `provider`, `provisioner` |
| Provider schema root | `alias`, `version` |
| Any attribute, any depth | Must match `^[a-z_][a-z0-9_]*$` — lowercase alphanumerics and underscores, no leading digit, no hyphens, no uppercase |

**Root level only.** `IsReservedResourceAttributeName` returns early for any path with more than one step, so a `provider` field nested inside an object or list element is completely fine. Only names promoted to the schema root collide. This distinction decides whether an endpoint is blocked, so resolve it against the **generated schema**, not the raw OpenAPI properties — a list endpoint puts item fields at `items[*].provider` (depth 2, fine), while a singular endpoint puts the same field at the root (blocked).

**How it fails.** Not at compile time. `schema.Schema.ValidateImplementation` fires when Terraform calls `GetProviderSchema`, so the generated code builds cleanly and then every test touching that resource or data source fails with:

```
Reserved Root Attribute/Block Name
"provider" is a reserved root attribute/block name.
```

**Check it explicitly.** Root attributes are emitted at exactly three tabs of indentation in the generated schema. Use `[[:blank:]]`, not `\t`: GNU grep does not treat `\t` as a tab in `-E` patterns and silently matches nothing.

```bash
grep -ohE '^[[:blank:]]{3}"[a-z0-9_]+": schema\.' internal/provider/{datasource,resource}_<name>/*_gen.go \
  | sed 's/.*"\(.*\)": schema\./\1/'
```

Compare that list against the reserved names above. Then confirm every attribute name in the file — at any depth — matches the identifier regex:

```bash
grep -ohE '"[A-Za-z0-9_-]+": schema\.' internal/provider/{datasource,resource}_<name>/*_gen.go \
  | sed 's/"\(.*\)": schema\./\1/' | grep -vE '^[a-z_][a-z0-9_]*$'
```

**Verdict:**

| Finding | Verdict |
|---------|---------|
| Reserved name at the schema root | **BLOCKED** — upstream must rename the API field |
| Reserved name only at depth ≥ 2 | Not a problem; note it and move on |
| Name fails the identifier regex at any depth | **BLOCKED** — upstream must rename the API field |

**Worked example — `ServiceQuota` (`GET /core/v1/service-quotas`).** The schema's required `provider` field (`aws` | `gcp`) lands at the root of a singular data source and collides with Terraform's `provider` meta-argument. There is no provider-side fix: `provider` cannot be renamed locally, cannot be dropped (it is required and identifies the cloud), and cannot be buried under a synthetic nesting level. The finding is that the API must rename the field — `cloud_provider` or `cloud` — before the endpoint is implementable. That is the entire deliverable for this endpoint.

## Step 3: Path and Query Parameters

The generator turns every `path` and `query` parameter of the **read** operation into an attribute. It merges each one with the request or response field of the **same OpenAPI name** (`ResourceAttributes.Merge` compares the original names). Names are converted to snake_case only afterwards, when the schema is emitted (`ToSpec`). So `{user_id}` next to a body `userId` gives two attributes that both emit as `user_id`, a duplicate; check the generated schema for it. Required-ness differs between the two mappers:

- **Resources:** a read path parameter is Required only if the create path has it too (a parent ID); otherwise it is Optional+Computed. Query parameters are Optional+Computed.
- **Data sources:** every path or query parameter marked `required: true` is Required, and the others are Optional+Computed. So an item data source takes a required `id`.

A path and a query parameter with the same original name fail generation in both.

| Finding | Verdict |
|---------|---------|
| Item path parameter is not `{id}` (`/notifications/{notificationId}`) while the response has `id` | **BLOCKER** — the resource gets a stray Optional+Computed `notification_id` that the response never fills, and the data source takes `notification_id` as input instead of `id`. Upstream renames it to `{id}` (standards §7.5, `doit-ids-item-path-param-id`). |
| A parent is named `{id}` (`/users/{id}/notifications/{notificationId}`) | **BLOCKER** — the parent and the child collapse into one `id` attribute: Required from the path, overwritten with the child ID on read, and the apply fails. Parents are `{<parent>Id}`; only the item is `{id}`. |
| Query parameter on the item GET (`fields`, `view`, …) | Finding — it becomes an attribute on the resource (Optional+Computed) and on the data source (Required if the parameter is `required: true`, otherwise Optional+Computed), and a partial read cannot confirm a write. Ask for it to be dropped from the item GET (standards §13.1). |
| A singleton sub-resource of an item (`/reports/{id}/config`) | Fine — `{id}` is still the item. |

## Step 4: Schema Composition and Types

The generator maps only a narrow set of composition. One property it cannot map fails the whole request or response body that contains it (`oas/attribute.go`). What happens next depends on which body that is (`resource_mapper.go`, `datasource_mapper.go`):

- **The resource's create request body, or the data source's read response:** the resource or data source is **skipped**, with a warning.
- **The resource's update request, create response or read response:** that body is dropped, with a warning. The resource is still generated, just without every attribute that body contributed.

So an emitted resource does not prove the contract mapped. Compare the generated attributes with the spec, and read the generator log, not just the build result.

Some rows below are generator failures. Others are standards policy (DoiT API Design Standards §13.10) that the generator would tolerate. Keep the two apart in findings, and name which one applies.

| Pattern | Generator | Verdict |
|---------|-----------|---------|
| `oneOf`/`anyOf` with more than two branches, or two non-null branches other than string-plus-primitive. This includes validation-only `anyOf` whose branches only add `required` or `minItems`. | Fails. With N ≠ 2: `found N oneOf subschema(s), schema composition is currently not supported`. With two branches: the multi-type error, or `no 'type' array …` for branches without a type. | **BLOCKER.** Ask for a `kind` enum with flat fields, with cross-field rules stated in the description and enforced with 422. |
| `oneOf`/`anyOf` of `string` plus `integer`/`number`/`boolean`, or `type: [string, integer]` and similar | Maps to a **string** attribute (`getMultiTypeSchema`, `retrieveType`). | Finding. The standards call it a type union ("No"). The provider must also convert the API's number or boolean to and from a string, and the read has to return a form that compares equal (`1` vs `"1"` vs `1.0`). |
| Other `type` lists with two non-null types | Fails: the multi-type error. | **BLOCKER.** |
| `discriminator` | Ignored; the keyword is never read. The `oneOf` it accompanies is what fails (first row). | Report the `oneOf`. The standards rule out discriminated `oneOf`. A `discriminator` on an otherwise flat schema is only a note. |
| A property that is still untyped after `allOf` flattening, e.g. `allOf: [Money, {properties: {currency: {const: USD}}}]` | Fails with `no 'type' array or supported allOf, oneOf, anyOf constraint`. On a conflict, preprocessing (`flattenAllOf`) keeps the later member's definition of the property whole, so `{const: USD}` replaces `{type: string}`. | **BLOCKER.** Ask for a dedicated typed schema, e.g. `WholeDollarUsdMoney`. |
| An `allOf` member without its own `type` (`{required: [amount]}`, `{description: …}`) | Fine. Flattening gives the merged schema the members' common type, or `object`. | Not a finding on its own. |
| Two `allOf` members defining the same property | Keeps the later definition whole (no deep merge) and logs a warning. | Note. This is fine when the later definition is complete. Check that it repeats the type and every constraint that should survive. |
| `properties` next to an `allOf` of two or more members | Dropped. `applyMergedAllOf` replaces the schema with the merged members and keeps only `description` and `example`. | **BLOCKER** for those attributes. Ask for them to move into a member. |
| `items` or `additionalProperties` schema without a `type`, e.g. `{additionalProperties: true}` | Fails with `no 'type' array …`; only `properties` lets the generator infer `object`. | **BLOCKER.** Ask for `type: array` / `type: object`. |
| `if`/`then`/`else`, `not`, `dependentRequired` | Not carried into the 3.0.1 projection. | Finding. Ask for the rule in prose, enforced with 422. |
| `type: object` without `properties`, or `additionalProperties: true` without `properties` | Becomes a `jsontypes.Normalized` attribute: practitioners write raw JSON, which is syntax-checked and compared semantically, but nothing about its contents is typed or validated. | Finding. Ask whether the shape is really open. Typed maps (`additionalProperties: <schema>`) become map attributes and are fine. |
| `additionalProperties: true` next to named `properties` | The named properties become nested attributes. Extra keys are not represented, so the provider can neither set nor keep them. | Note. Ask whether the extra keys matter to users. |
| A field that only admits `null` (a placeholder for a future shape) | Fails: `invalid schema type 'null'` (no null case in `BuildResourceAttribute`/`BuildDataSourceAttribute`), with the consequences above. Through the 3.0.1 projection it instead becomes a nullable empty object, an attribute the API always rejects. | **BLOCKER.** Ask for it to be left out until it has a shape; adding an optional field later is not breaking for the provider, while retyping a shipped one breaks configs and state. |
| Constraints on array items (`items: {minimum: …}`) | Dropped. | Note. Only the server enforces them. |
| `number` without `format: double`/`float` | Maps to a big-float Number with no validators. | Note. |
| Secret string without `format: password` | `format: password` is what marks the attribute sensitive. `writeOnly` and `x-terraform-*` are not read. | Finding. |

## Step 5: CRUD Operation Support

For a **resource** built on a new contract, the API must support all CRUD operations:

| Operation | Required HTTP Method | Check |
|-----------|---------------------|-------|
| Create | `POST` on the collection | Has a request body schema (without one the resource is skipped). Returns **201** with the full resource, including `id`. |
| Read | `GET /{id}` | Returns the full resource. Unknown or deleted IDs return **404**. |
| Update | `PATCH /{id}` (merge patch) or `PUT /{id}` | Returns **200** with the full resource. The provider takes computed values from the write response; 204-then-read needs hand-written code. |
| Delete | `DELETE /{id}` | 204 (or 200) on success. 204 or 404 for an already-deleted resource; the provider treats 404 as success. |

Every status the API returns must be declared in the spec with its body. The generated client fills `JSON201` / `JSON200` only for declared responses, and the provider checks for the exact status (`!= 201`).

Every configurable field must be settable on create and changeable on update. A field that changes only through an `/actions/<verb>` endpoint (e.g. `visibility` via `share`/`unshare`) cannot be a normal attribute, because the generator produces no action calls: **BLOCKER** for that field. Fields accepted on create but not on update must be documented as immutable; the provider marks them `RequiresReplace`.

A new contract missing update or delete is a finding. Ask whether that is intended before settling on **Suitable as data source only**.

For a **data source**, only Read (GET) is needed. A list data source wants `{items, pageToken, rowCount}` with `pageToken`/`maxResults`. An unpaginated `{items}` is acceptable when the collection is small and bounded; ask.

## Step 6: Response Completeness and Round-Trip

**Critical:** Terraform compares the value read back after an apply with the configuration. The API response must include ALL fields that were sent in the request, unchanged. Check for:

- [ ] Does the response include the resource `id`?
- [ ] Does the response echo back all user-provided fields, under the same name and with the same type? (`doit-terraform-round-trip-fields`)
- [ ] Are there any **write-only fields** (accepted in request but not returned)? These cause perpetual drift, unless the field is a secret, which is the documented exception.
- [ ] Are there any **computed fields** (returned but not sent)? These need `Computed: true` in the schema.
- [ ] Can the read return an enum value or a representation that create and update reject (a console-only widget type, an `extended` metric)? The request enum becomes a validator, so such a resource can be neither imported and managed nor left alone (`doit-terraform-read-enum-writable`).
- [ ] When a field is omitted, does the read return a server-chosen value? Then it cannot be cleared by omission (Category B in Step 10).
- [ ] Does every Terraform default in the generated schema equal what the read returns when the field is omitted? A spec `default` becomes a Terraform default only on the source that first defines the attribute, in practice the create request. Update-request defaults are ignored (`IgnoreDefaults`), and a later source never adds one, because `Merge` copies only description and computability. Check the defaults the generated schema actually has, not every `default` in the spec.
- [ ] Can a field silently change or disappear later (a referenced Slack channel deleted, a budget's `endTime` once it elapses)? The behaviour must be documented.
- [ ] Can users find the IDs this resource references (channel IDs, allocation IDs) through a public get or list endpoint, so a data source can look them up?

## Step 7: Field Normalization

Check if the API normalizes user-provided values. Decide per case whether it is an upstream finding or something the provider absorbs:

- **Lossy, or changes the meaning** (rounding `100.99` to `100.00`, truncating, dropping entries, sorting or de-duplicating an undocumented list): upstream finding. Ask the API to reject with 422 instead (standards §13.4). Otherwise the apply fails with an inconsistent-result error after the change has already been stored.
- **Formatting only, value kept** (`"100"` read back as `"100.00"`, a timestamp converted to UTC, a documented lowercased email): the provider absorbs it with semantic equality. It must still be documented in the spec.

Common patterns:

| Normalization | Example | Impact |
|---------------|---------|--------|
| Sentinel stripping | `[Service N/A]` → `Service` | Need `mergeSentinelValues` in Read |
| Type aliasing | `allocation_rule` → `attribution` | Need `normalizeDimensionsType` |
| Timestamp to UTC | `2024-01-15T00:00:00-05:00` → `2024-01-15T05:00:00Z` | Need timestamp preservation |
| Boolean defaults | `include_null: true` → API returns `false` | Need state-first read pattern |
| Empty list to null | `scopes: []` → API returns `null` | Need empty-list fallback |
| Array order or duplicates | IDs sorted, grouped by type, or de-duplicated | The generator emits a list unless the array declares `format: set` (`uniqueItems` only adds a validator), and no spec does today. Preferred: ask for order as sent, with duplicates rejected (422). If the API sorts or de-duplicates, a documented sort key is not enough on its own: either the array becomes a set (see **Sets** below), or users must write the server's order, and every config in another order diffs. Record which. |

**Sets.** The generator turns an array into a Terraform set only when the spec declares `format: set` on it. This is a custom format the fork documents (`DESIGN.md`) and that OpenAPI, the DoiT standards and other generators do not know. `uniqueItems: true` only adds a validator to a list. No spec uses `format: set` today, and no generated resource has a set attribute. Verified on the alert `recipients` and `recipientsSlackChannels` arrays:

- `make generate` turns `ListAttribute` / `ListNestedAttribute` into `SetAttribute` / `SetNestedAttribute`. It carries `maxItems` over as `setvalidator.SizeAtMost`, and the model fields become `types.Set`. The generated API client (`models_gen.go`) does not change; it still uses slices.
- The provider then fails to build. Every hand-written helper for that field assumes a list: the overlay, the conversions and the list-only plan modifiers such as `useNullForUnknownListWhenConfigNull`. All of them need set equivalents. Data sources that share the schema become sets too.
- On a shipped resource, list → set changes the attribute's type in user state. Plan a schema version bump and state upgrade, and treat it as a user-visible change.
- A set of objects identifies each element by its whole value. Elements with server-filled attributes (the Slack channel's `customer_id`, or a name "resolved server-side") are a known source of plan churn and "inconsistent result" errors in Terraform providers. Prefer sets of scalars, or elements the user configures fully. This was not exercised with real plans.

So `format: set` is an implementation decision for the provider team, not a free answer to "the API sorts". Prefer order kept as sent, and agree a set in the spec PR.

**Test method:** Send a POST/PUT with known values, then GET the resource and compare.

## Step 8: Pagination Support (List Endpoints)

- [ ] Does the endpoint support pagination (`pageToken`, `maxResults`)?
- [ ] Is the list under `items`? New contracts use `{items, pageToken, rowCount}`; legacy endpoints use resource-named arrays.
- [ ] Is `rowCount` returned in the response? (The provider falls back to `len(items)`.)
- [ ] Does an empty `pageToken` in the response indicate the last page?
- [ ] What is the default page size?
- [ ] Is the order stable (for example by `createTime`, then `id`), so `items[0]` means the same thing between runs?

## Step 9: Errors, Concurrency and Idempotency

- [ ] Does a GET of an unknown or deleted ID return a real **404**? Not 200 with an error body, and not 403 for "not yours". The provider removes the resource from state on a read 404; anything else leaves a deleted resource in state.
- [ ] Is delete idempotent (204 or 404 for an already-deleted resource)?
- [ ] Are error messages useful for debugging? The client surfaces the raw body of every non-retried error.
- [ ] `ETag` / `If-Match`: the provider sends the `etag` from state on update and delete. On a 412, a delete refetches the current ETag and retries once; other callers report the 412. Required and optional `If-Match` both work for the provider. Making it required later is a breaking change for other clients, so new contracts should decide now.
- [ ] `Idempotency-Key`: the provider sends one per apply operation and reuses it on retries. `dryRun` is not used by the provider.
- [ ] Retries (`DCIRetryClient.Do`):
  - 429 is retried, honouring `Retry-After` when it parses, otherwise with exponential backoff.
  - 502/503/504 are retried with exponential backoff only; their `Retry-After` is not read, so the API cannot count on it.
  - 404, 412 and 425 are passed to the caller for resource-specific handling: drop state, refetch the ETag, or poll an async result.
  - Any other status ≥ 400 is a permanent error carrying the raw body.
- [ ] Long-running create, update or delete: is there an operation resource or a state field to wait on? Is the resource readable right after create?

## Step 10: Schema Field Classification

Map each API field to the attribute it becomes, and for settable fields, to its clearing behaviour. The generator makes every non-required create or update field **Optional+Computed**. The evaluation does not turn any of them into plain Optional. It decides which clearing category from [implement-resource](../implement-resource/SKILL.md) applies, because that is what the API's behaviour determines:

| Category | Criteria | Provider mapping | Example |
|----------|----------|------------------|---------|
| **Required** | Must be provided by user, no default | Required | `name` |
| **Settable, clearable** | User can set it; omitting it or sending `null`/`[]` clears it, and the API then stores and returns nothing | Optional+Computed, Category A (clearing plan modifier) | `description`, `labels` |
| **Settable, API default** | User can set it; when omitted, the API picks or keeps a value and returns it | Optional+Computed, Category B (`acknowledgeNotClearable`) | `formula` |
| **Settable, not clearable in place** | User can set it; the update has no way to clear it (merge patch, no null representation), and removal must take effect | Optional+Computed, Category C (replace on clear) | |
| **Computed-only** | API generates, user cannot set | Computed (`UseStateForUnknown` when stable) | `id`, `create_time` |

## Step 11: OpenAPI Spec Accuracy

Compare the OpenAPI spec with actual API behavior. Discrepancies are findings — do not fix them locally:

```bash
# Test actual API behavior.
# Add -H "X-Tenant-Id: ${DOIT_CUSTOMER_CONTEXT}" ONLY when using a DoiT
# employee token (scopes the request to a customer). Omit it for regular-user
# tokens — the customer is derived from the token.
curl -s -X POST "${DOIT_HOST}/<endpoint>" \
  -H "Authorization: Bearer ${DOIT_API_TOKEN}" \
  -H "X-Tenant-Id: ${DOIT_CUSTOMER_CONTEXT}" \
  -H "Content-Type: application/json" \
  -d '{"name": "test"}' | jq .
```

Common discrepancies to flag:
- Fields marked as required in spec but optional in practice (or vice versa)
- Response types differing from spec (e.g., `*[]T` vs `[]T`)
- Missing fields in the spec that the API actually returns
- Enums with undocumented values

## Reporting Findings

Create a summary with:

1. **Suitability verdict** — one of:
   - **BLOCKED** — any **BLOCKER** above, for example:
     - a reserved or invalid attribute name at the schema root;
     - an item path parameter other than `{id}`, or a parent named `{id}`;
     - composition the generator cannot map, which skips the resource or drops a body (Step 4);
     - a state-bearing field settable only through an action;
     - another incompatibility that cannot be resolved provider-side.

     Name the required upstream change. Do not propose a provider-side workaround.
   - **Suitable as resource** — full CRUD support confirmed.
   - **Suitable with documented workaround** — legacy endpoints only, with the API owner's confirmation that the contract is frozen (see Scope).
   - **Suitable as data source only** — no usable write path (a genuine API limitation, never a way around a schema-level problem).
   - **Not suitable** — the endpoint does not model a Terraform-manageable object.
2. **Blocking issues** — everything marked **BLOCKER**, each with what breaks in the provider and the upstream change that fixes it. On an api-specs PR, post each one as its own review comment starting with **Blocker.**, citing the standards section.
3. **Spec issues** — discrepancies between the OpenAPI spec and actual API behavior (these must be fixed upstream).
4. **Code generator issues** — problems with the generated schema or types.
5. **Normalization concerns** — field normalization that will need handling.
6. **Field classification table** — map each field to Required/Optional/Computed.
7. **Questions for the API team** — behaviour the spec leaves open that decides the attribute model: defaults echoed on read, normalization, array order and duplicates, what happens to references that disappear. Ask instead of assuming.
8. **Recommendations** — API changes needed before implementation, stated as the change the API team must make.
