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

`make testacc` automatically runs acceptance tests using `-run '^TestAcc'`,
skipping the package's unit tests (`*_internal_test.go` and other non-`TestAcc` tests).
`make testacc-run TEST=...` forwards your specific test regex (`-run '$(TEST)'`).
The unit suite (`make test`) runs in a hermetic cleanroom environment where all
credentials and `TF_ACC` are explicitly cleared. CI runs both `make test` (credential-free)
and `make testacc` in separate jobs. **Before pushing, always run the unit suite:**

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

### Unit Test HTTP Mocking with `httptest.NewTestServer`

When writing unit tests that mock DoiT API responses (e.g. `*_internal_test.go`, `delete_notfound_test.go`):

1. **Always use `httptest.NewTestServer(t, handler)`** instead of `httptest.NewServer(handler)` to use the in-memory fake network and avoid OS TCP port allocations. Server cleanup is handled automatically, so **never add `defer server.Close()`** — see the rules below for why it is actively harmful inside a bubble.
2. **Always pass `models.WithHTTPClient(server.Client())`** when instantiating the generated client (`models.NewClientWithResponses`). Because `NewTestServer` does not bind a local TCP port, failing to pass `server.Client()` will cause connection errors.
3. **`NewTestServer` starts lazily.** Startup — and the assignment of `server.URL` — happens on the first `server.Client()` call. Read `server.URL` before anything has called `Client()` and you get `""`, silently: the client reaches nothing and any "no request was made" assertion becomes vacuous. If a test needs the URL without building a client, touch `server.Client()` first.
4. **Timeout, retry, and polling tests must use `testing/synctest`** — see below. A test that waits in real wall-clock time does not belong in the unit suite.

### Virtual Time with `testing/synctest`

Tests that would otherwise sleep in real wall-clock time belong in a synctest
bubble, where the `time` package uses a fake clock that advances instantly once
every goroutine in the bubble is durably blocked. The whole unit suite runs under
a 120s budget, so a test asserting on minutes of elapsed time is only practical
this way.

```go
func TestSomethingSlow(t *testing.T) {
    // No t.Parallel(): a bubble owns its own clock and requires every goroutine
    // in it to be durably blocked before time advances.
    synctest.Test(t, func(t *testing.T) {
        srv := httptest.NewTestServer(t, handler)   // in-memory net = durably blocking
        client := newTestClient(t, srv)             // ...and no defer srv.Close()

        // t.Context(), never context.Background(): its Done channel belongs to
        // the bubble. Bound it even when the expected path never reaches the
        // deadline — it is what turns a runaway retry into a fast failure.
        ctx, cancel := context.WithTimeout(t.Context(), DefaultReadTimeout)
        defer cancel()

        start := time.Now()
        // ... code that waits on timers or a context deadline ...
        if elapsed := time.Since(start); elapsed != 15*time.Second {
            t.Errorf("elapsed = %v, want 15s", elapsed)  // virtual, costs ~0 real time
        }
    })
}
```

Rules that matter:

**The API**

- **`synctest.Test(t, func(t *testing.T){...})`** is the API. `synctest.Run` was
  the pre-1.25 experimental form and **no longer exists** — it will not compile.
- **`synctest.Wait()`** blocks until every *other* goroutine in the bubble is
  durably blocked. Call it before asserting on state a handler goroutine writes
  (request counters, recorded delays) when the assertion is not already ordered
  by a response you observed — `atomic` makes such a read safe, not ordered.
- **`synctest.Sleep(d)`** is exactly `time.Sleep(d)` + `synctest.Wait()`. Prefer
  it over a bare `time.Sleep` when the test and the system under test would
  otherwise wake at the same virtual instant and the test wants the SUT to
  settle first.
- **Never call `t.Parallel()` inside a bubble.** The ban is on the bubble's `t`;
  the outer `t` is an ordinary `*testing.T`, so wrapping is technically legal.
  Don't — a bubbled test finishes in ~0ms, so parallelism buys nothing, and the
  two-`t` shadowing is a trap. The repo's `paralleltest` linter only flags
  `resource.Test()`, so omitting `t.Parallel()` here is lint-clean.
- **Table tests:** `t.Run` on the outside, `synctest.Test` on the inside.
  `synctest.Test` must not be called from within a bubble or from a `t.Cleanup`.

**Keeping the clock running**

- **`httptest.NewTestServer` is what makes this work.** Its in-memory network is
  durably blocking; a real TCP listener is not, and time would never advance.
- **Build the server inside the bubble, with the bubble's `t`.** A server
  constructed outside it has its accept loops and connection channels created
  outside the bubble, so a blocked read is *not* durably blocking and the clock
  never advances. Instant handlers still pass, so this fails as a hang to the
  120s package timeout rather than as a test failure.
- **Nothing created outside the bubble may cross into it** — no contexts,
  channels, timers, tickers, or `WaitGroup`s. A `select` on a non-bubbled Done
  channel is not durably blocking. Use **`t.Context()`**, never
  `context.Background()`.
- **Never hold a `sync.Mutex` across a durable block.** Mutex contention is not
  durable, so the bubble never idles, the clock never advances, and a holder
  parked on a timer never wakes. synctest cannot detect this deadlock — it
  surfaces as a package timeout. Take the wait outside the lock.
- **Close every response body.** An open body keeps its connection out of the
  idle pool, so the server's cleanup cannot reap it and the transport's read
  loop never exits — which `synctest.Test` reports as a leaked goroutine.
- **Never add `defer server.Close()`.** `defer`s run *before* `t.Cleanup`s, so an
  explicit `Close` jumps ahead of the body-close cleanups, blocks waiting on
  connections still in flight, and arms a 5s hang diagnostic that virtual time
  makes free — a spurious "blocked in Close after 5 seconds" on a test that took
  0ms. The auto-registered cleanup already orders correctly: body closes →
  connection idles → `Close` reaps it.

**Writing assertions that hold**

- **Assert elapsed time with exact equality**, including `elapsed == 0` to prove
  no wait happened. There is no timer granularity to absorb, so loose bounds
  only weaken the test. `elapsed == 0` is the most direct available proof that a
  status was not retried.
- **Use the production constants.** Virtual time is free, so a test can drive the
  real `DefaultRequestTimeout` / `DefaultReadTimeout` / backoff intervals instead
  of millisecond stand-ins. Where a realistic value would invert the very
  relationship under test, keep the synthetic one and say so in a comment.
- **Bound every context, even when the expected path never reaches the
  deadline.** `DCIRetryClient` runs with `MaxElapsedTime(0)`, so the context is
  the only thing that stops a retry loop; without a deadline a regression spins
  virtual time at full CPU until the package timeout. With one, it fails fast and
  legibly. The deadline is a tripwire, not part of the assertion.
- **Do not let two timers come due at the same virtual instant.** Both cases of a
  `select` are then ready and the choice varies between runs — elapsed stays
  exact but attempt counts flip. In practice: don't let a backoff interval divide
  the deadline. Verify with `-count=20`.
- **The bubble clock starts at exactly midnight UTC 2000-01-01**, with zero
  nanoseconds. Every virtual instant is a whole second, which is why HTTP-date
  arithmetic (`Retry-After` as a date) stays exact where it would be flaky on a
  real clock.
- Real network I/O is not durably blocking. Keep the server, client, and system
  under test entirely inside the bubble.

Reference examples: `internal/provider/async_report_test.go` drives a full
submit/poll/cancel cycle across minutes of virtual time in milliseconds, and
`internal/provider/timeout_test.go` asserts the retry client's behavior at the
shipped timeout and backoff constants.

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
