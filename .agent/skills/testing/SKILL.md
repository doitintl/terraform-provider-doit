---
name: testing
description: Acceptance test conventions for the Terraform provider. Covers running tests, drift verification, list attribute coverage, test performance, and custom type testing.
---

# Testing Conventions

## Running Acceptance Tests

**Always use the Makefile targets** — they handle environment variable loading from `.envrc.local`:

```bash
# Run all acceptance tests
make testacc

# Run a specific acceptance test
make testacc-run TEST=TestAccLabel
```

Do NOT use `go test` directly.

### Capturing Full Output

Always capture the full output so you don't have to re-run:

```bash
make testacc-run TEST=TestAccReport 2>&1 | tee /tmp/test-output.txt
```

Then search for failures:

```bash
grep -A 20 "FAIL\|Error\|inconsistent" /tmp/test-output.txt
```

### Required Environment Variables

See `.envrc.example` for the full list. Key variables:

| Variable | Description |
|----------|-------------|
| `TF_ACC` | Set to `1` to enable acceptance tests |
| `DOIT_API_TOKEN` | Your DoiT API token |
| `DOIT_HOST` | API host (e.g., `https://api.doit.com`) |
| `TEST_USER` | Email for test budget collaborators |
| `TEST_ATTRIBUTION` | Attribution ID for test budget scope |

---

## Drift Verification

All acceptance tests for resources should verify that re-applying the same configuration produces no changes:

```go
// Step 1: Create the resource
{
    Config: testAccResourceConfig(n),
    ConfigPlanChecks: resource.ConfigPlanChecks{
        PreApply: []plancheck.PlanCheck{
            plancheck.ExpectNonEmptyPlan(),
        },
    },
},
// Step 2: Verify no drift
{
    Config: testAccResourceConfig(n),  // Same config!
    ConfigPlanChecks: resource.ConfigPlanChecks{
        PreApply: []plancheck.PlanCheck{
            plancheck.ExpectEmptyPlan(),
        },
    },
},
```

### Required Test Coverage

| Test Type | Drift Step | Reason |
|-----------|-----------|--------|
| Main tests | ✅ Required | Update step catches drift from create |
| Feature tests | ✅ Required | Feature-specific attributes |
| Import tests | ❌ Not needed | Tests import, not drift |
| Validation/Disappears | ❌ Not needed | Tests error handling |

### Known API Issues

When an API bug causes legitimate drift, skip the drift step with a TODO:

```go
// TODO(TICKET-ID): Enable drift verification once API returns field X.
```

---

## Overlay Pattern Tests

Every plan-first resource must have these test categories:

| Test | What It Verifies |
|------|------------------|
| **Create + drift check** | Create, then re-apply with `ExpectEmptyPlan()` |
| **Update + drift check** | Create, modify, then re-apply with `ExpectEmptyPlan()` |
| **Import + drift check** | Create, import, then re-apply with `ExpectEmptyPlan()` |
| **Omitted Optional+Computed** | Omit each field, verify no drift after API assigns defaults |
| **null↔[] consistency (non-clearable)** | Omit list fields, assert `ListSizeExact(0)` on Create AND drift-check |
| **null↔[] consistency (clearable)** | Omit clearable list fields, assert `knownvalue.Null()` on Create AND drift-check |
| **Clearing lifecycle** | Set a clearable attribute → drift check → clear it → drift check (see [Clearing Optional+Computed Attributes](../implement-resource/SKILL.md#clearing-optionalcomputed-attributes)) |
| **API normalization** | Use values the API will normalize, verify user value preserved |
| **Value with boolean flags** | Omit booleans, verify they resolve to `false` not `Unknown` |

---

## List Attribute Coverage

All list attributes must have explicit test coverage for both:

1. **Empty list** (`attribute = []`) — user explicitly sets empty
2. **Omitted** — user doesn't specify the attribute

```go
func TestAccResource_WithEmptyLists(t *testing.T) {
    resource.ParallelTest(t, resource.TestCase{
        Steps: []resource.TestStep{
            {
                Config: testAccResourceWithEmptyLists(),
                ConfigStateChecks: []statecheck.StateCheck{
                    statecheck.ExpectKnownValue(
                        "doit_resource.test",
                        tfjsonpath.New("my_list"),
                        knownvalue.ListExact([]knownvalue.Check{})),
                },
            },
        },
    })
}
```

---

## Testing Custom Generated Types

Generated custom types (e.g., `RulesValue`) have an internal `state` field. **Never construct them with struct literals** — use `NewXxxValueMust()`:

```go
// WRONG — state field zeroed, IsNull() returns true even with populated fields
ruleVal := resource_allocation.RulesValue{
    Action: basetypes.NewStringValue("create"),
}

// CORRECT — state properly initialized
attrTypes := resource_allocation.RulesValue{}.AttributeTypes(ctx)
ruleVal := resource_allocation.NewRulesValueMust(attrTypes, map[string]attr.Value{
    "action": basetypes.NewStringValue("create"),
    "name":   basetypes.NewStringNull(),
    // ...
})
```

### Test Helper Pattern

Create helpers for constructing properly initialized values:

```go
type ruleSpec struct {
    action     string
    name       string
    nameIsNull bool
}

func createRulesValue(ctx context.Context, spec ruleSpec) resource_allocation.RulesValue {
    attrTypes := resource_allocation.RulesValue{}.AttributeTypes(ctx)
    return resource_allocation.NewRulesValueMust(attrTypes, map[string]attr.Value{
        "action": basetypes.NewStringValue(spec.action),
        "name":   nameVal,
        // ...
    })
}
```

### File Naming

Test files for internal validators use the `_internal_test.go` suffix.

---

## Test Performance

### Parallel Execution

**All acceptance tests MUST use `resource.ParallelTest()`** instead of `resource.Test()`:

```go
// REQUIRED
func TestAccResource_Basic(t *testing.T) {
    resource.ParallelTest(t, resource.TestCase{...})
}
```

Why it's safe: all tests use unique resource names.

> **Linter:** `paralleltest` — flags `resource.Test()` usage.

### Caching Expensive Helpers

Use `sync.Once` for helpers that paginate through all resources:

```go
var (
    alertCount     int
    alertCountOnce sync.Once
)

func getAlertCount(t *testing.T) int {
    t.Helper()
    alertCountOnce.Do(func() {
        alertCount = computeAlertCount(t)
    })
    return alertCount
}
```

### Checklist for New Tests

- [ ] Use `resource.ParallelTest()`
- [ ] Add drift verification step (`ExpectEmptyPlan()`)
- [ ] Cover empty lists and omitted attributes
- [ ] Use unique resource names
- [ ] Cache expensive helpers with `sync.Once`
