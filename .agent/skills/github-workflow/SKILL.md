---
name: github-workflow
description: GitHub workflow conventions for the Terraform provider. Covers using the gh CLI, creating PRs, extracting CI errors, and fetching review comments.
---

# GitHub Workflow

## Always Use `gh` CLI

When interacting with GitHub — PRs, issues, CI/CD runs — **always use the `gh` CLI**. Do NOT use `read_url_content` to scrape GitHub pages.

```bash
# View a PR
gh pr view 42 --repo doitintl/terraform-provider-doit

# View PR diff
gh pr diff 42 --repo doitintl/terraform-provider-doit

# View CI run summary
gh run view <RUN_ID> --repo doitintl/terraform-provider-doit

# View failed CI logs
gh run view <RUN_ID> --repo doitintl/terraform-provider-doit --log-failed

# View an issue
gh issue view 42 --repo doitintl/terraform-provider-doit
```

## Extracting CI Errors

The `--log-failed` output is verbose. Use these grep patterns:

```bash
# Step 1: List failed tests
gh run view <RUN_ID> --repo <REPO> --log-failed 2>&1 | grep "^Run Tests.*FAIL" | head -20

# Step 2: Get actual error messages
gh run view <RUN_ID> --repo <REPO> --log-failed 2>&1 | \
  grep -E "(inconsistent|was cty\.|but now|null fallback|non-retryable)" | head -20

# Step 3: Context around a specific test
gh run view <RUN_ID> --repo <REPO> --log-failed 2>&1 | \
  grep -E "TestAccMyTest" -B 5 -A 15 | head -50
```

## Fetching PR Review Comments

```bash
# List all review IDs and authors
gh api repos/<OWNER>/<REPO>/pulls/<PR>/reviews --jq '.[] | "\(.id) \(.user.login) \(.state)"'

# Get inline comments from a specific review
gh api repos/<OWNER>/<REPO>/pulls/<PR>/reviews/<REVIEW_ID>/comments \
  --jq '.[] | "--- \(.path):\(.original_line // .line) ---\n\(.body)\n"'

# One-liner: get all Copilot review comments
REVIEW_ID=$(gh api repos/<OWNER>/<REPO>/pulls/<PR>/reviews --jq '.[] | select(.user.login | test("copilot|bot")) | .id') && \
gh api repos/<OWNER>/<REPO>/pulls/<PR>/reviews/${REVIEW_ID}/comments \
  --jq '.[] | "--- \(.path):\(.original_line // .line) ---\n\(.body)\n"'
```

## .gitignore Check

Always check `.gitignore` before committing:

```bash
git check-ignore -v path/to/file
```

Some files (like `OpenAPI/api_endpoint_analysis.md`, `OpenAPI/issues/`) are local documentation and should NOT be committed.
