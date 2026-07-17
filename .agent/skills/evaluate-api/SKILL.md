---
name: evaluate-api
description: Evaluate a DoIT API endpoint for Terraform provider compatibility. Run the code generation pipeline as a test, check CRUD support, response behavior, field normalization, and identify spec issues. Do NOT fix issues in the OpenAPI spec — flag them upstream.
---

# Evaluate API Endpoint

Use this skill when evaluating a new DoIT API endpoint for Terraform resource or data source compatibility. This applies when an endpoint is being planned, is in draft, or was recently added.

## Critical Rules

> **NEVER modify the OpenAPI spec (`openapi_spec_full.yml`)** — it is a carbon copy of the upstream version and all manual changes WILL be overwritten. If there are issues with the spec, flag them as findings. Do not attempt to silently work around spec issues.

> **The schema should come from the code generator.** If the code generator produces incorrect or incomplete schemas, that is a finding — not something to fix locally.

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

## Step 2: CRUD Operation Support

For a **resource**, the API must support all CRUD operations:

| Operation | Required HTTP Method | Check |
|-----------|---------------------|-------|
| Create | `POST` | Must return the created object with `id` |
| Read | `GET /{id}` | Must return the full object |
| Update | `PUT /{id}` or `PATCH /{id}` | Must return the updated object |
| Delete | `DELETE /{id}` | 200/204 on success, 404 should be idempotent |

For a **data source**, only Read (GET) is needed. List data sources need a paginated list endpoint.

## Step 3: Response Completeness

**Critical:** The API response must include ALL fields that were sent in the request. Check for:

- [ ] Does the response include the resource `id`?
- [ ] Does the response echo back all user-provided fields?
- [ ] Are there any **write-only fields** (accepted in request but not returned)? These cause perpetual drift.
- [ ] Are there any **computed fields** (returned but not sent)? These need `Computed: true` in the schema.

## Step 4: Field Normalization

Check if the API normalizes user-provided values. Common patterns:

| Normalization | Example | Impact |
|---------------|---------|--------|
| Sentinel stripping | `[Service N/A]` → `Service` | Need `mergeSentinelValues` in Read |
| Type aliasing | `allocation_rule` → `attribution` | Need `normalizeDimensionsType` |
| Timestamp to UTC | `2024-01-15T00:00:00-05:00` → `2024-01-15T05:00:00Z` | Need timestamp preservation |
| Boolean defaults | `include_null: true` → API returns `false` | Need state-first read pattern |
| Empty list to null | `scopes: []` → API returns `null` | Need empty-list fallback |

**Test method:** Send a POST/PUT with known values, then GET the resource and compare.

## Step 5: Pagination Support (List Endpoints)

- [ ] Does the endpoint support pagination (`pageToken`, `maxResults`)?
- [ ] Is `rowCount` returned in the response?
- [ ] Does an empty `pageToken` in the response indicate the last page?
- [ ] What is the default page size?

## Step 6: Error Response Format

- [ ] Does 404 return a proper status code (not 200 with error body)?
- [ ] Are error messages useful for debugging?
- [ ] Is delete idempotent (404 for already-deleted resources)?

## Step 7: Schema Field Classification

Map each API field to a Terraform schema category:

| Category | Criteria | Example |
|----------|----------|---------|
| **Required** | Must be provided by user, no default | `name` |
| **Optional** | User can provide, API has default | `description` |
| **Optional+Computed** | User can provide or API will generate | `formula` |
| **Computed-only** | API generates, user cannot set | `id`, `create_time` |

## Step 8: OpenAPI Spec Accuracy

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

1. **Suitability verdict** — suitable for resource, data source only, or not suitable
2. **Spec issues** — any discrepancies between the OpenAPI spec and actual API behavior (these must be fixed upstream)
3. **Code generator issues** — any problems with the generated schema or types
4. **Normalization concerns** — any field normalization that will need handling
5. **Field classification table** — map each field to Required/Optional/Computed
6. **Recommendations** — any API changes needed before implementation
