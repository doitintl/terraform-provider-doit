---
name: prepare-release
description: Prepare a new release by updating all versions, dependencies, tools, CI actions, pre-commit hooks, changelog, and documentation.
---

# Prepare Release

This skill guides you through preparing a new release of the terraform-provider-doit.

## Prerequisites

// turbo-all

1. Read the relevant skills in `.agent/skills/` for context on provider conventions.
2. Confirm the target version number with the user (e.g., `v1.3.0`).
3. Create a release branch from `main`:
   ```bash
   git checkout main && git pull origin main
   git checkout -b release/v<VERSION>
   ```

---

## Step 1: Update Go Version

Check for the latest stable Go release:

```bash
# Check current version in go.mod
head -5 go.mod
```

1. Search the web for the latest Go stable release version.
2. If a new version is available, update:
   - `go.mod` — the `go` directive (line 3).
   - `README.md` — the Go requirement (line mentioning `Go >= X.XX`).
   - `flake.nix` — **only for major Go releases** (e.g., 1.26 → 1.27), NOT for patch releases (e.g., 1.26.0 → 1.26.1). Patch releases are picked up automatically by nix via the existing `go_1_XX` package. For major releases, update the `go` package reference (e.g., `go_1_26` → `go_1_27`) and find a new nixpkgs commit that includes the new Go package on [NixOS/nixpkgs](https://github.com/NixOS/nixpkgs).
3. After changing Go version, run:
   ```bash
   go mod tidy
   ```

---

## Step 2: Update Go Dependencies

Update all direct and indirect Go dependencies:

```bash
go get -u ./...
go mod tidy
```

Verify the build compiles successfully:
```bash
go build ./...
```

### Review Release Notes for Breaking Changes

For each **direct** dependency in `go.mod` (the first `require` block), check the release notes for breaking changes:

| Dependency | Check for |
|---|---|
| `hashicorp/terraform-plugin-framework` | [Changelog](https://github.com/hashicorp/terraform-plugin-framework/blob/main/CHANGELOG.md) — breaking API changes, deprecations |
| `hashicorp/terraform-plugin-testing` | [Changelog](https://github.com/hashicorp/terraform-plugin-testing/blob/main/CHANGELOG.md) — new test helper patterns |
| `hashicorp/terraform-plugin-go` | [Changelog](https://github.com/hashicorp/terraform-plugin-go/blob/main/CHANGELOG.md) — protocol changes |
| `hashicorp/terraform-plugin-framework-validators` | [Changelog](https://github.com/hashicorp/terraform-plugin-framework-validators/blob/main/CHANGELOG.md) — new validators |
| `oapi-codegen/runtime` | [Releases](https://github.com/oapi-codegen/runtime/releases) — request/response handling changes |
| `cenkalti/backoff/v5` | [Releases](https://github.com/cenkalti/backoff/releases) — retry behaviour changes |

Report any relevant findings to the user before proceeding.

---

## Step 3: Update Go Tools

The `tool` directive in `go.mod` lists the Go tools used by this project. Update them:

```bash
# Update each tool to latest
go get github.com/doitintl/terraform-plugin-codegen-framework/cmd/tfplugingen-framework@latest
go get github.com/doitintl/terraform-plugin-codegen-openapi/cmd/tfplugingen-openapi@latest
go get github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@latest
go get github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
go mod tidy
```

Verify code generation still works:
```bash
make generate
make docs
```

---

## Step 4: Update GitHub Action Versions

Check for newer versions of all GitHub Actions used in `.github/workflows/`:

| Action | Current | Check |
|---|---|---|
| `actions/checkout` | Check current pinned hash | [Releases](https://github.com/actions/checkout/releases) |
| `actions/setup-go` | Check current pinned hash | [Releases](https://github.com/actions/setup-go/releases) |
| `hashicorp/setup-terraform` | Check current pinned hash | [Releases](https://github.com/hashicorp/setup-terraform/releases) |
| `goreleaser/goreleaser-action` | Check current pinned hash | [Releases](https://github.com/goreleaser/goreleaser-action/releases) |
| `crazy-max/ghaction-import-gpg` | Check current pinned hash | [Releases](https://github.com/crazy-max/ghaction-import-gpg/releases) |
| `golangci/golangci-lint-action` | Check current pinned hash | [Releases](https://github.com/golangci/golangci-lint-action/releases) |

### Update procedure for each action:
1. Find the latest release tag (e.g., `v6.1.0`).
2. Get the full commit SHA for that tag:
   ```bash
   git ls-remote https://github.com/<owner>/<repo>.git refs/tags/<tag>
   ```
   If the result is a tag object (not a commit), dereference it:
   ```bash
   git ls-remote https://github.com/<owner>/<repo>.git refs/tags/<tag>^{}
   ```
3. Update the SHA and version comment in **all workflow files** that use the action:
   - `.github/workflows/release.yml`
   - `.github/workflows/tests.yml`
   - `.github/workflows/golangci-lint.yml`

**Format:** `uses: owner/action@<full-sha> # <version-tag>`

---

## Step 5: Update Pre-commit Hooks

Check for newer versions of pre-commit hook repos in `.pre-commit-config.yaml`:

| Hook repo | Check |
|---|---|
| `golangci/golangci-lint` | [Releases](https://github.com/golangci/golangci-lint/releases) — update `rev:` |
| `pre-commit/pre-commit-hooks` | [Releases](https://github.com/pre-commit/pre-commit-hooks/releases) — update `rev:` |

**Important:** If `golangci-lint` is updated, also update:
- `.pre-commit-config.yaml` — the `rev:` field
- `.github/workflows/golangci-lint.yml` — the `version:` field in the golangci-lint-action step
- `flake.nix` — the comment referencing the version (and nixpkgs pin if needed)

---

## Step 6: Verify `.goreleaser.yml`

Compare the local `.goreleaser.yml` against the upstream HashiCorp scaffolding framework:

**Reference:** https://github.com/hashicorp/terraform-provider-scaffolding-framework/blob/main/.goreleaser.yml

Fetch the latest reference and diff:
```bash
curl -sL https://raw.githubusercontent.com/hashicorp/terraform-provider-scaffolding-framework/main/.goreleaser.yml > /tmp/goreleaser-upstream.yml
diff .goreleaser.yml /tmp/goreleaser-upstream.yml
```

If there are meaningful differences (not just whitespace), update `.goreleaser.yml` to match upstream. Report any discrepancies to the user — some divergences may be intentional (e.g., custom `ldflags`).

---

## Step 7: Verify Documentation is Up to Date

### README.md
Check and update:
- [ ] Go version requirement matches `go.mod`
- [ ] Nix flake tool versions match `flake.nix` (Go, Terraform, golangci-lint versions)
- [ ] The `# Requirements` section matches current tool versions
- [ ] The environment variables tables match `.envrc.example`

### .envrc.example
Check and update:
- [ ] All required env vars from `README.md` are present
- [ ] All optional env vars from `README.md` are present
- [ ] Any new env vars added since the last release are included
- [ ] Compare against the actual `.envrc.local` (but do NOT commit `.envrc.local`)

### Cross-check environment variables
- [ ] The environment variables in `.envrc.example` match what `README.md` documents

### DCI API Status Document

Update `.test/Current Status of DCI API.md`:

1. **Check open Jira tickets:** For each ticket listed in the "Open API Issues" section that is NOT in the "Resolved Issues" table, fetch the current status from Jira using the Atlassian MCP:
   ```
   getJiraIssue(cloudId: "doitintl.atlassian.net", issueIdOrKey: "CMP-XXXXX", fields: ["summary", "status"])
   ```

2. **Update ticket statuses:** If a ticket status changed:
   - If now **Done** or **Canceled**: move it from the open issues table to the "Resolved Issues" table.
   - If status changed but still open: update the Status column in the open issues table.

3. **Update implementation matrix:** Check if any new data sources or resources were implemented since the last release:
   - Update the "Endpoint Suitability Matrix" Data Source column (e.g., from `📋 Ready` to `✅ Implemented`).
   - Move items from "Data Sources Ready for Implementation" to "Data Sources Implemented" table.
   - Update the "Implementation Opportunities" table with a Status column.
   - Update the "Data Source Only — Implemented" and "Ready for Implementation" roadmap tables.
   - Update the implementation counts (e.g., "Data Sources Implemented (N)").

4. **Update the document date** at the top of the file.

---

## Step 8: Update CHANGELOG.md

Gather all changes since the last release tag:

```bash
# Find the last release tag and gather PRs
LATEST_TAG=$(git describe --tags --abbrev=0)
echo "Last release: $LATEST_TAG"
git log "${LATEST_TAG}..HEAD" --oneline --no-merges
```

Also check the GitHub PR list for merged PRs since the last release. Use the GitHub MCP `search_issues` tool:

```
repo:doitintl/terraform-provider-doit is:pr is:merged merged:>YYYY-MM-DD
```

Write a new changelog entry at the top of `CHANGELOG.md` following the existing format:

```markdown
## v<VERSION> (<YYYY-MM-DD>)

### BREAKING CHANGES
(only if applicable)

### FEATURES
- **resource/doit_xxx**: Description ([#PR](url))

### ENHANCEMENTS
- Description ([#PR](url))

### BUG FIXES
- Description ([#PR](url))

### DOCUMENTATION
- Description

### INTERNAL
- Upgraded Go to X.XX
- Upgraded dependencies: list key upgrades
- Upgraded CI workflow actions (list versions)
```

**Categories to use** (omit empty ones):
- `BREAKING CHANGES` — only for user-facing breaking changes
- `FEATURES` — new resources or data sources
- `ENHANCEMENTS` — additions to existing resources
- `BUG FIXES` — bug fixes
- `DOCUMENTATION` — documentation-only changes
- `INTERNAL` — CI, deps, refactoring (not user-facing)

---

## Step 9: Final Verification

Run the full verification suite:

```bash
# Build
go build ./...

# Lint
golangci-lint run

# Generate and check for drift
make generate
make docs
git diff --exit-code -- docs/ internal/provider/datasource_* internal/provider/resource_*

# Run unit tests
go test -v -timeout 120s ./...
```

If acceptance tests are desired (this makes real API calls):
```bash
make testacc
```

---

## Step 10: Commit and Create PR

```bash
git add -A
git status
```

Review the changeset with the user before committing. Suggested commit message:

```
chore: prepare release v<VERSION>

- Upgrade Go to <version>
- Update all dependencies
- Upgrade CI actions and pre-commit hooks
- Update CHANGELOG.md
```

Create a PR. After merge, tag the release:
```bash
git tag v<VERSION>
git push origin v<VERSION>
```

The `release.yml` workflow will automatically create the GitHub release and publish to the Terraform Registry.
