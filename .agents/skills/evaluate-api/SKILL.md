---
name: evaluate-api
description: Evaluate a DoIT API endpoint for Terraform provider compatibility. Run the code generation pipeline as a test, check reserved attribute names, CRUD support, response behavior, field normalization, and identify spec issues. Do NOT fix issues in the OpenAPI spec and do NOT work around them — flag them upstream.
---

# Evaluate API Endpoint

Use this skill when evaluating a new DoIT API endpoint for Terraform resource or data source compatibility. This applies when an endpoint is being planned, is in draft, or was recently added.

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

If the schema / OpenAPI spec for the new endpoint is present, run the full code generation pipeline **as a test** — not an actual implementation:

```bash
# 1. Add the endpoint to datasources.yml or resources.yml
# 2. Run the generator
make generate
# 3. Inspect the generated output
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

**Check it explicitly.** Root attributes are emitted at exactly three tabs of indentation in the generated schema:

```bash
grep -ohE '^\t\t\t"[a-z0-9_]+": schema\.' internal/provider/datasource_<name>/*_gen.go \
  | sed 's/.*"\(.*\)": schema\./\1/'
```

Compare that list against the reserved names above. Then confirm every attribute name in the file — at any depth — matches the identifier regex:

```bash
grep -ohE '"[A-Za-z0-9_-]+": schema\.' internal/provider/datasource_<name>/*_gen.go \
  | sed 's/"\(.*\)": schema\./\1/' | grep -vE '^[a-z_][a-z0-9_]*$'
```

**Verdict:**

| Finding | Verdict |
|---------|---------|
| Reserved name at the schema root | **BLOCKED** — upstream must rename the API field |
| Reserved name only at depth ≥ 2 | Not a problem; note it and move on |
| Name fails the identifier regex at any depth | **BLOCKED** — upstream must rename the API field |

**Worked example — `ServiceQuota` (`GET /core/v1/service-quotas`).** The schema's required `provider` field (`aws` | `gcp`) lands at the root of a singular data source and collides with Terraform's `provider` meta-argument. There is no provider-side fix: `provider` cannot be renamed locally, cannot be dropped (it is required and identifies the cloud), and cannot be buried under a synthetic nesting level. The finding is that the API must rename the field — `cloud_provider` or `cloud` — before the endpoint is implementable. That is the entire deliverable for this endpoint.

## Step 3: CRUD Operation Support

For a **resource**, the API must support all CRUD operations:

| Operation | Required HTTP Method | Check |
|-----------|---------------------|-------|
| Create | `POST` | Must return the created object with `id` |
| Read | `GET /{id}` | Must return the full object |
| Update | `PUT /{id}` or `PATCH /{id}` | Must return the updated object |
| Delete | `DELETE /{id}` | 200/204 on success, 404 should be idempotent |

For a **data source**, only Read (GET) is needed. List data sources need a paginated list endpoint.

## Step 4: Response Completeness

**Critical:** The API response must include ALL fields that were sent in the request. Check for:

- [ ] Does the response include the resource `id`?
- [ ] Does the response echo back all user-provided fields?
- [ ] Are there any **write-only fields** (accepted in request but not returned)? These cause perpetual drift.
- [ ] Are there any **computed fields** (returned but not sent)? These need `Computed: true` in the schema.

## Step 5: Field Normalization

Check if the API normalizes user-provided values. Common patterns:

| Normalization | Example | Impact |
|---------------|---------|--------|
| Sentinel stripping | `[Service N/A]` → `Service` | Need `mergeSentinelValues` in Read |
| Type aliasing | `allocation_rule` → `attribution` | Need `normalizeDimensionsType` |
| Timestamp to UTC | `2024-01-15T00:00:00-05:00` → `2024-01-15T05:00:00Z` | Need timestamp preservation |
| Boolean defaults | `include_null: true` → API returns `false` | Need state-first read pattern |
| Empty list to null | `scopes: []` → API returns `null` | Need empty-list fallback |

**Test method:** Send a POST/PUT with known values, then GET the resource and compare.

## Step 6: Pagination Support (List Endpoints)

- [ ] Does the endpoint support pagination (`pageToken`, `maxResults`)?
- [ ] Is `rowCount` returned in the response?
- [ ] Does an empty `pageToken` in the response indicate the last page?
- [ ] What is the default page size?

## Step 7: Error Response Format

- [ ] Does 404 return a proper status code (not 200 with error body)?
- [ ] Are error messages useful for debugging?
- [ ] Is delete idempotent (404 for already-deleted resources)?

## Step 8: Schema Field Classification

Map each API field to a Terraform schema category:

| Category | Criteria | Example |
|----------|----------|---------|
| **Required** | Must be provided by user, no default | `name` |
| **Optional** | User can provide, API has default | `description` |
| **Optional+Computed** | User can provide or API will generate | `formula` |
| **Computed-only** | API generates, user cannot set | `id`, `create_time` |

## Step 9: OpenAPI Spec Accuracy

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
   - **BLOCKED** — a reserved or invalid attribute name at the schema root, or another incompatibility that cannot be resolved provider-side. Name the required upstream change. Do not propose a provider-side workaround.
   - **Suitable as resource** — full CRUD support confirmed.
   - **Suitable as data source only** — no usable write path (a genuine API limitation, never a way around a schema-level problem).
   - **Not suitable** — the endpoint does not model a Terraform-manageable object.
2. **Blocking issues** — reserved root attribute names, invalid identifiers, and anything else requiring an upstream API change before implementation can start.
3. **Spec issues** — discrepancies between the OpenAPI spec and actual API behavior (these must be fixed upstream).
4. **Code generator issues** — problems with the generated schema or types.
5. **Normalization concerns** — field normalization that will need handling.
6. **Field classification table** — map each field to Required/Optional/Computed.
7. **Recommendations** — API changes needed before implementation, stated as the change the API team must make.
