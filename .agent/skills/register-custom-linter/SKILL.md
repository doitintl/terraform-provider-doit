---
name: register-custom-linter
description: Register a new custom golangci-lint linter in this Terraform provider repo. Use when adding a new linter under tools/linters/ and wiring it into the plugin system and .golangci.yml config.
---

# Register a Custom Linter

This skill covers the **three registration steps** needed after creating a new analyzer under `tools/linters/<name>/analyzer.go`. All three are required — missing any one causes "unknown linters" errors at runtime.

## Prerequisites

- The analyzer package exists at `tools/linters/<name>/analyzer.go`
- It exports `var Analyzer = &analysis.Analyzer{Name: "<name>", ...}`
- Its tests pass: `cd tools/linters && go test ./<name>/... -v`

## Step 1: Register in `plugin.go`

File: [`tools/linters/plugin.go`](file:///Users/hannes/Desktop/git/terraform-provider-doit/tools/linters/plugin.go)

Add two things:

### 1a. Import

```go
import (
    // ... existing imports ...
    "github.com/doitintl/terraform-provider-doit/tools/linters/<name>"
)
```

### 1b. Plugin registration in `init()`

Add inside the `init()` function, following the existing pattern:

```go
register.Plugin("<name>", func(_ any) (register.LinterPlugin, error) {
    return &analyzerPlugin{analyzers: []*analysis.Analyzer{<name>.Analyzer}}, nil
})
```

> [!IMPORTANT]
> The string passed to `register.Plugin()` **must exactly match** the `Name` field in your `analysis.Analyzer`.

## Step 2: Configure `.golangci.yml`

File: [`.golangci.yml`](file:///Users/hannes/Desktop/git/terraform-provider-doit/.golangci.yml)

Three sub-sections need updating:

### 2a. Enable the linter

Under `linters.enable:`, add the linter name:

```yaml
linters:
  enable:
    # ... existing linters ...
    - <name>
```

### 2b. Declare as custom module (CRITICAL — easy to forget!)

Under `settings.custom:`, add the linter declaration. **This is the step that causes "unknown linters" if missed:**

```yaml
  settings:
    custom:
      # ... existing custom linters ...
      <name>:
        type: "module"
        description: "<one-line description>"
```

> [!CAUTION]
> Without this `settings.custom` entry, golangci-lint v2 will report `unknown linters: '<name>'` even though the code is compiled into the binary. The `type: "module"` declaration is what tells golangci-lint to look for the plugin in the custom binary.

### 2c. Add exclusion rules

Custom linters typically need exclusions for:

1. **Generated files** (`_gen.go`): Under the existing `path: _gen\.go` exclusion block
2. **Test files** (`_test.go`): Under the existing `path: _test\.go` exclusion block
3. **Data sources** (`_data_source.go`): If the linter is resource-only
4. **Provider config** (`provider.go`): If the linter only applies to resources/data sources

Find each exclusion block and add `- <name>` to the `linters:` list.

Example — for a resource-only linter, add to ALL of these blocks:

```yaml
exclusions:
  rules:
    - path: _gen\.go
      linters:
        - <name>        # ← add here
    - path: _test\.go
      linters:
        - <name>        # ← add here
    - path: _data_source\.go
      linters:
        - <name>        # ← add here
    - path: provider\.go
      linters:
        - <name>        # ← add here
```

## Step 3: Update Go modules

```bash
cd tools/linters && go mod tidy
```

## Step 4: Rebuild and verify

> [!CAUTION]
> **golangci-lint aggressively caches analyzer results.** When developing or debugging a new linter, you **must** clean the cache first. Without this, golangci-lint reuses stale results from before your linter existed, causing it to appear as if findings are missing or the linter isn't running.

```bash
# Rebuild the custom binary
cd /path/to/repo
rm -f custom-gcl
golangci-lint custom

# CRITICAL: Clean the analysis cache before running!
./custom-gcl cache clean

# Run the linter
./custom-gcl run ./internal/provider/... 2>&1 | grep <name>
```

### Debugging tips

1. **Always run `./custom-gcl cache clean` after any linter code change.** The Go build cache (`go clean -cache`) is separate from golangci-lint's analysis cache. You need BOTH if the binary seems stale.

2. **The binary may rebuild but use cached analyzer results.** `golangci-lint custom` rebuilds the binary from source via the `path:` directive in `.custom-gcl.yml`. But the analysis framework caches fact/result data separately. The binary can contain new code while golangci-lint serves stale analyzer outputs.

3. **`pass.Reportf()` diagnostics on the same file+line get deduplicated.** If your linter reports multiple findings at the same position (e.g., all at `fn.Pos()`), only one survives. Use unique positions per finding — either from AST nodes in the function body, or synthetic offsets from `fn.Body.Lbrace`.

4. **golangci-lint captures stderr.** `fmt.Fprintf(os.Stderr, ...)` from analyzer code is NOT visible in terminal output. For debug logging, write to a temp file instead:
   ```go
   f, _ := os.OpenFile("/tmp/linter_debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
   fmt.Fprintf(f, "debug: %v\n", value)
   f.Close()
   ```

5. **`generated: lax` suppresses diagnostics, not analysis.** Gen files are still analyzed and produce facts/results. Diagnostics reported at gen file positions are silently dropped.

## Checklist

- [ ] `tools/linters/<name>/analyzer.go` — Analyzer with `Name: "<name>"`
- [ ] `tools/linters/<name>/analyzer_test.go` — Tests pass
- [ ] `tools/linters/plugin.go` — Import + `register.Plugin("<name>", ...)`
- [ ] `.golangci.yml` `linters.enable` — `- <name>`
- [ ] `.golangci.yml` `settings.custom.<name>` — `type: "module"` ← **THE STEP YOU KEEP FORGETTING**
- [ ] `.golangci.yml` exclusion rules — added to relevant `path:` blocks
- [ ] `go mod tidy` in `tools/linters/`
- [ ] `golangci-lint custom` rebuild
- [ ] `./custom-gcl cache clean` ← **ALWAYS DO THIS WHEN DEVELOPING**
- [ ] `./custom-gcl run` — linter runs without "unknown linters" error

## Reference: Existing Linters

See [plugin.go](file:///Users/hannes/Desktop/git/terraform-provider-doit/tools/linters/plugin.go) for all registered linters and [.golangci.yml](file:///Users/hannes/Desktop/git/terraform-provider-doit/.golangci.yml) for the full config.
