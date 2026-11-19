#!/usr/bin/env bash
# Merge OpenAPI specification changes from an upstream GitHub PR into the local
# spec file using a three-way merge (`git merge-file`) against the PR's real
# base and head commits.
#
# Usage:
#   ./scripts/apply_pr.sh <pr-number>
#   make apply-pr PR=<pr-number>

set -euo pipefail

if [ $# -ne 1 ] || ! [[ "$1" =~ ^[0-9]+$ ]]; then
    echo "Error: PR number required (e.g. make apply-pr PR=61514)" >&2
    exit 1
fi

PR="$1"
REPO="${REPO:-doiteng/omni}"
UPSTREAM_PATH="${UPSTREAM_PATH:-services/external-api/openapi.yaml}"
TARGET="${TARGET:-OpenAPI/openapi_spec_full.yml}"

if [ ! -f "$TARGET" ]; then
    echo "Error: target spec file not found: $TARGET" >&2
    exit 1
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

PR_META="$(gh pr view "$PR" --repo "$REPO" \
    --json state,baseRefOid,headRefOid,mergeCommit \
    --jq '[.state, (.baseRefOid // "-"), (.headRefOid // "-"), (.mergeCommit.oid // "-")] | join(" ")')"
read -r STATE BASE_REF_OID HEAD_REF_OID MERGE_COMMIT <<< "$PR_META"

if [ "$STATE" = "MERGED" ] && [ "$MERGE_COMMIT" != "-" ]; then
    # doiteng/omni enforces squash merges, so mergeCommit^1..mergeCommit is the
    # exact landed diff for the PR.
    HEAD_SHA="$MERGE_COMMIT"
    BASE_SHA="$(gh api "repos/${REPO}/commits/${HEAD_SHA}" --jq '.parents[0].sha // empty')"
    DESC="merge commit ${HEAD_SHA:0:10}, parent ${BASE_SHA:0:10}"
else
    if [ "$STATE" = "CLOSED" ]; then
        echo "Warning: ${REPO}#${PR} is CLOSED (unmerged); resolving merge-base..head anyway." >&2
    fi
    if [ "$BASE_REF_OID" = "-" ] || [ "$HEAD_REF_OID" = "-" ]; then
        echo "Error: ${REPO}#${PR} is missing baseRefOid or headRefOid" >&2
        exit 1
    fi
    # Resolve the true merge-base rather than using baseRefOid directly so stale
    # PR branches do not revert intervening commits on the target branch.
    HEAD_SHA="$HEAD_REF_OID"
    BASE_SHA="$(gh api "repos/${REPO}/compare/${BASE_REF_OID}...${HEAD_REF_OID}" --jq '.merge_base_commit.sha // empty')"
    DESC="PR head ${HEAD_SHA:0:10}, merge-base ${BASE_SHA:0:10}"
fi

if [ -z "$BASE_SHA" ] || [ -z "$HEAD_SHA" ] || [ "$BASE_SHA" = "$HEAD_SHA" ]; then
    echo "Error: could not resolve distinct base/head commits for ${REPO}#${PR}" >&2
    exit 1
fi

echo "Applying ${REPO}#${PR} (${DESC})"

gh api "repos/${REPO}/contents/${UPSTREAM_PATH}?ref=${BASE_SHA}" \
    -H "Accept: application/vnd.github.raw" > "$TMP_DIR/base.yml"
gh api "repos/${REPO}/contents/${UPSTREAM_PATH}?ref=${HEAD_SHA}" \
    -H "Accept: application/vnd.github.raw" > "$TMP_DIR/head.yml"

if cmp -s "$TMP_DIR/base.yml" "$TMP_DIR/head.yml"; then
    echo "Error: ${REPO}#${PR} does not modify ${UPSTREAM_PATH} (${BASE_SHA:0:10}..${HEAD_SHA:0:10})" >&2
    exit 1
fi

cp "$TARGET" "$TMP_DIR/orig.yml"

set +e
git merge-file --diff3 \
    -L "local (${TARGET})" \
    -L "PR #${PR} base (${BASE_SHA:0:10})" \
    -L "PR #${PR} head (${HEAD_SHA:0:10})" \
    "$TARGET" "$TMP_DIR/base.yml" "$TMP_DIR/head.yml"
STATUS=$?
set -e

if [ "$STATUS" -eq 0 ]; then
    if cmp -s "$TMP_DIR/orig.yml" "$TARGET"; then
        echo "PR #${PR} changes are already present in ${TARGET} (no changes made)."
    else
        echo "Merged ${REPO}#${PR} into ${TARGET}."
    fi
    exit 0
fi

if [ "$STATUS" -ge 1 ] && [ "$STATUS" -le 127 ]; then
    echo "Error: three-way merge produced ${STATUS} conflict(s) in ${TARGET}; resolve conflict markers or run 'git checkout -- ${TARGET}' to abort." >&2
    exit 1
fi

cp "$TMP_DIR/orig.yml" "$TARGET"
echo "Error: git merge-file failed with exit code ${STATUS}" >&2
exit "$STATUS"
