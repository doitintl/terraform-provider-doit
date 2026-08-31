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

The targets run tests through [`gotestsum`](https://github.com/gotestyourself/gotestsum),
which **automatically reruns failed tests** (up to 2 extra attempts) to absorb
upstream API flakiness. See [Capturing Full Output](#capturing-full-output) for how
to tell a flaky rerun apart from a real failure.

### Capturing Full Output

`gotestsum` prints a clean, de-interleaved failure summary at the end of the run
(plain `go test -v` output from parallel tests is interleaved and hard to read).
Still capture the full output so you don't have to re-run — use `set -o pipefail`
so the pipe to `tee` doesn't swallow the test exit code:

```bash
set -o pipefail
make testacc-run TEST=TestAccReport 2>&1 | tee /tmp/test-output.txt
echo "exit: $?"   # WITHOUT pipefail this prints tee's status (0), not the test result
```

**The exit code is the source of truth** for overall pass/fail:

- **exit 0** = every test ultimately passed. Some may have needed a rerun (flaky),
  but none is a real failure.
- **non-zero** = at least one test failed on its _final_ attempt.

To tell a genuine failure from a test that merely flaked, read the rerun report —
the targets write one line per reran test to `/tmp/testacc-reruns.txt`:

```bash
cat /tmp/testacc-reruns.txt   # absent or empty if nothing was rerun
# doit.TestAccX: 3 runs, 3 failures   <- failures == runs → never passed → REAL failure
# doit.TestAccY: 3 runs, 1 failures   <- failures  < runs → recovered   → flaky (report, not blocking)
```

Flaky (recovered) tests are not blocking, but are worth reporting (see
[Known API Issues](#known-api-issues)).

Do **not** infer real-vs-flaky from `(re-run N)` markers in the log: a test can fail
`(re-run 1)` and still pass on `(re-run 2)`. The `=== Failed` block lists every
failed _attempt_, including attempts of tests that later recovered — use it to read
failure _output_, not to decide pass/fail:

```bash
sed -n '/^=== Failed/,$p' /tmp/test-output.txt   # failure output (may include recovered flakies)
```

**Do NOT trust `test-results.xml` for pass/fail.** gotestsum writes one `<testcase>`
per _attempt_, so a recovered flaky still carries a `<failure>` node even on a green
run — a naive parse reports false failures. That file exists only to feed the CI
JUnit check (which sets `check_retries: true` to compensate). Locally, rely on the
exit code and `/tmp/testacc-reruns.txt`.

### Required Environment Variables

See `.envrc.example` for the full list. Key variables:

| Variable           | Description                             |
| ------------------ | --------------------------------------- |
| `TF_ACC`           | Set to `1` to enable acceptance tests   |
| `DOIT_API_TOKEN`   | Your DoiT API token                     |
| `DOIT_HOST`        | API host (e.g., `https://api.doit.com`) |
| `TEST_USER`        | Email for test budget collaborators     |
| `TEST_ATTRIBUTION` | Attribution ID for test budget scope    |

---

## Unit Tests (run these too — CI does)

`make testacc` and `make testacc-run TEST=...` strictly run acceptance tests
matching `-run '^TestAcc'`; they **skip the package's unit tests**
(`*_internal_test.go` and other non-`TestAcc` tests). The unit suite (`make test`)
runs in a hermetic cleanroom environment where all credentials and `TF_ACC` are
explicitly cleared. CI runs both `make test` (credential-free) and `make testacc`
in separate jobs. **Before pushing, always run the unit suite:**

```bash
make test          # full credential-free unit suite (clears TF_ACC and DOIT_* envs)
```

Targeted unit tests (which need no API env) may use `go test` directly — the
"use Makefile targets" rule exists for acceptance tests that load `.envrc.local`:

```bash
go test ./internal/provider/ -run 'TestReportTimestampValidator|TestToExternalConfig'
```

### Gotcha: adding a field to a generated nested object breaks `NewXxxValue` callers

When you add an attribute to a generated nested object (e.g. a new field under
`config`) and run `make generate`, the generated `NewXxxValue` / `NewConfigValue`
constructors start **requiring an entry for the new attribute** — they return a
`"a missing attribute value was detected"` diagnostic otherwise. Every hand-written
call site must add the new key, including **internal test helpers** that build these
values (e.g. `report_validator_internal_test.go`'s `buildConfigWithForecastSettings`
/ `buildForecastConfigValue`). These only fail under `make test`, not under a
filtered acceptance run — which is exactly why the unit suite must be run.

After `make generate`, grep for every constructor call and add the new key:

```bash
grep -rn "NewConfigValue(" internal/provider/   # update each (incl. *_test.go)
```

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

| Test Type             | Drift Step    | Reason                                |
| --------------------- | ------------- | ------------------------------------- |
| Main tests            | ✅ Required   | Update step catches drift from create |
| Feature tests         | ✅ Required   | Feature-specific attributes           |
| Import tests          | ❌ Not needed | Tests import, not drift               |
| Validation/Disappears | ❌ Not needed | Tests error handling                  |

### Known API Issues

When an API bug causes legitimate drift, skip the drift step with a TODO:

```go
// TODO(TICKET-ID): Enable drift verification once API returns field X.
```

---

## Overlay Pattern Tests

Every plan-first resource must have these test categories:

| Test                                    | What It Verifies                                                                                                                                                                    |
| --------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Create + drift check**                | Create, then re-apply with `ExpectEmptyPlan()`                                                                                                                                      |
| **Update + drift check**                | Create, modify, then re-apply with `ExpectEmptyPlan()`                                                                                                                              |
| **Import + drift check**                | Create, import, then re-apply with `ExpectEmptyPlan()`                                                                                                                              |
| **Omitted Optional+Computed**           | Omit each field, verify no drift after API assigns defaults                                                                                                                         |
| **null↔[] consistency (non-clearable)** | Omit list fields, assert `ListSizeExact(0)` on Create AND drift-check                                                                                                               |
| **null↔[] consistency (clearable)**     | Omit clearable list fields, assert `knownvalue.ListExact([]knownvalue.Check{})` on Create AND drift-check                                                                           |
| **Clearing lifecycle**                  | Set a clearable attribute → drift check → clear it → drift check (see [Clearing Optional+Computed Attributes](../implement-resource/SKILL.md#clearing-optionalcomputed-attributes)) |
| **API normalization**                   | Use values the API will normalize, verify user value preserved                                                                                                                      |
| **Value with boolean flags**            | Omit booleans, verify they resolve to `false` not `Unknown`                                                                                                                         |

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

### Constructing API Response Objects with Nullable Fields

When constructing `models.*` structs in unit tests, use `valueToNullable` for fields that are `nullable.Nullable[T]` in `models_gen.go`:

```go
// OLD — pointer field
apiResp := &models.Allocation{
    Rule: &models.AllocationRule{Formula: "A"},
}

// NEW — nullable field
apiResp := &models.Allocation{
    Rule: valueToNullable(models.AllocationRule{Formula: "A"}),
}
```

Check `models_gen.go` for the actual field types — both `*T` and `nullable.Nullable[T]` coexist.

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
