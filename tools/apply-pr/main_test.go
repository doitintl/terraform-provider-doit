package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type stubGitHub struct {
	prs        map[int]prMetadata
	parents    map[string][]string
	mergeBases map[string]string
	files      map[string][]byte

	mergeBaseCalled bool
}

func (s *stubGitHub) PRView(_ context.Context, _ string, pr int) (prMetadata, error) {
	meta, ok := s.prs[pr]
	if !ok {
		return prMetadata{}, fmt.Errorf("PR #%d not found", pr)
	}
	return meta, nil
}

func (s *stubGitHub) CommitParents(_ context.Context, _ string, sha string) ([]string, error) {
	parents, ok := s.parents[sha]
	if !ok {
		return nil, fmt.Errorf("commit %s not found", sha)
	}
	return parents, nil
}

func (s *stubGitHub) MergeBase(_ context.Context, _ string, base, head string) (string, error) {
	s.mergeBaseCalled = true
	mb, ok := s.mergeBases[base+"..."+head]
	if !ok {
		return "", fmt.Errorf("merge-base %s...%s not found", base, head)
	}
	return mb, nil
}

func (s *stubGitHub) RawFile(_ context.Context, _, _, ref string) ([]byte, error) {
	data, ok := s.files[ref]
	if !ok {
		return nil, fmt.Errorf("file at ref %s not found", ref)
	}
	return data, nil
}

func TestResolvePRCommits_MergedUsesMergeCommitParent(t *testing.T) {
	t.Parallel()

	stub := &stubGitHub{
		prs: map[int]prMetadata{
			72485: {
				State:      "MERGED",
				BaseRefOID: "base-on-dev",
				HeadRefOID: "pr-branch-tip",
				MergeCommit: &struct {
					OID string `json:"oid"`
				}{OID: "4bb99c7d1ce2775678e01e07cdbad44d723e4e62"},
			},
		},
		parents: map[string][]string{
			"4bb99c7d1ce2775678e01e07cdbad44d723e4e62": {"08858069ed4f6b86570e4449e93f3f906e3e30cb"},
		},
	}

	var stderr bytes.Buffer
	got, err := resolvePRCommits(t.Context(), stub, defaultRepo, 72485, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.mergeBaseCalled {
		t.Fatal("expected merged PR to use merge commit parent, not compare merge-base")
	}
	if got.BaseSHA != "08858069ed4f6b86570e4449e93f3f906e3e30cb" {
		t.Fatalf("BaseSHA = %q, want merge commit parent", got.BaseSHA)
	}
	if got.HeadSHA != "4bb99c7d1ce2775678e01e07cdbad44d723e4e62" {
		t.Fatalf("HeadSHA = %q, want merge commit", got.HeadSHA)
	}
}

func TestResolvePRCommits_OpenUsesTrueMergeBase(t *testing.T) {
	t.Parallel()

	stub := &stubGitHub{
		prs: map[int]prMetadata{
			72650: {
				State:      "OPEN",
				BaseRefOID: "moving-dev-tip",
				HeadRefOID: "open-pr-head",
			},
		},
		mergeBases: map[string]string{
			"moving-dev-tip...open-pr-head": "true-merge-base",
		},
	}

	var stderr bytes.Buffer
	got, err := resolvePRCommits(t.Context(), stub, defaultRepo, 72650, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.BaseSHA != "true-merge-base" {
		t.Fatalf("BaseSHA = %q, want true-merge-base (not baseRefOid)", got.BaseSHA)
	}
	if got.HeadSHA != "open-pr-head" {
		t.Fatalf("HeadSHA = %q, want open-pr-head", got.HeadSHA)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr warning for open PR, got: %q", stderr.String())
	}
}

func TestResolvePRCommits_ClosedUnmergedWarns(t *testing.T) {
	t.Parallel()

	stub := &stubGitHub{
		prs: map[int]prMetadata{
			999: {
				State:      "CLOSED",
				BaseRefOID: "dev-tip",
				HeadRefOID: "closed-head",
			},
		},
		mergeBases: map[string]string{
			"dev-tip...closed-head": "mb-sha",
		},
	}

	var stderr bytes.Buffer
	got, err := resolvePRCommits(t.Context(), stub, defaultRepo, 999, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.BaseSHA != "mb-sha" || got.HeadSHA != "closed-head" {
		t.Fatalf("unexpected commit pair: %+v", got)
	}
	if !strings.Contains(stderr.String(), "CLOSED") {
		t.Fatalf("expected CLOSED warning on stderr, got: %q", stderr.String())
	}
}

func TestResolvePRCommits_ErrorCases(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		stub    *stubGitHub
		wantErr string
	}{
		{
			name: "merged commit with no parents",
			stub: &stubGitHub{
				prs: map[int]prMetadata{
					1: {
						State: "MERGED",
						MergeCommit: &struct {
							OID string `json:"oid"`
						}{OID: "root-sha"},
					},
				},
				parents: map[string][]string{"root-sha": nil},
			},
			wantErr: "no parents",
		},
		{
			name: "open PR missing refs",
			stub: &stubGitHub{
				prs: map[int]prMetadata{
					1: {State: "OPEN"},
				},
			},
			wantErr: "missing baseRefOid or headRefOid",
		},
		{
			name: "open PR with no commits ahead of base",
			stub: &stubGitHub{
				prs: map[int]prMetadata{
					1: {
						State:      "OPEN",
						BaseRefOID: "same-sha",
						HeadRefOID: "same-sha",
					},
				},
				mergeBases: map[string]string{"same-sha...same-sha": "same-sha"},
			},
			wantErr: "no commits ahead of base",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var stderr bytes.Buffer
			_, err := resolvePRCommits(t.Context(), tc.stub, defaultRepo, 1, &stderr)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tc.wantErr, err)
			}
		})
	}
}

func TestRun_SequentialTwoPRReplayNoSchemaSwap(t *testing.T) {
	t.Parallel()

	// Simulates the #483/#484 failure mode:
	// - Local spec lags behind upstream (missing UpstreamOnlyA/B/C schemas, shifting line numbers).
	// - BudgetCreateRequest and BudgetUpdateRequest share identical context lines around
	//   amount, growthPerPeriod, and usePrevSpend.
	// - PR 1 modifies Budget; PR 2 modifies Budget, BudgetCreateRequest, and BudgetUpdateRequest
	//   with distinct descriptions on each request schema.
	localInitial := `openapi: 3.0.1
components:
  schemas:
    Budget:
      type: object
      properties:
        amount:
          type: number
          description: Old amount
    BudgetCreateRequest:
      type: object
      properties:
        amount:
          type: number
          description: Budget period amount
          format: double
        growthPerPeriod:
          type: number
          description: Periodical growth percentage in recurring budget
          format: double
          default: 0
        usePrevSpend:
          type: boolean
          description: Use last period spend
          default: false
    BudgetUpdateRequest:
      type: object
      properties:
        amount:
          type: number
          description: Budget period amount
          format: double
        growthPerPeriod:
          type: number
          description: Periodical growth percentage in recurring budget
          format: double
          default: 0
        usePrevSpend:
          type: boolean
          description: Use last period spend
          default: false
`

	upstreamPrefix := `openapi: 3.0.1
components:
  schemas:
    UpstreamOnlyA:
      type: object
      properties:
        a1:
          type: string
        a2:
          type: string
    UpstreamOnlyB:
      type: object
      properties:
        b1:
          type: string
        b2:
          type: string
`

	pr1Base := upstreamPrefix + `    Budget:
      type: object
      properties:
        amount:
          type: number
          description: Old amount
    BudgetCreateRequest:
      type: object
      properties:
        amount:
          type: number
          description: Budget period amount
          format: double
        growthPerPeriod:
          type: number
          description: Periodical growth percentage in recurring budget
          format: double
          default: 0
        usePrevSpend:
          type: boolean
          description: Use last period spend
          default: false
    BudgetUpdateRequest:
      type: object
      properties:
        amount:
          type: number
          description: Budget period amount
          format: double
        growthPerPeriod:
          type: number
          description: Periodical growth percentage in recurring budget
          format: double
          default: 0
        usePrevSpend:
          type: boolean
          description: Use last period spend
          default: false
`

	pr1Head := upstreamPrefix + `    Budget:
      type: object
      properties:
        amount:
          type: number
          description: Configured amount
        currentPeriodAmount:
          type: number
          readOnly: true
    BudgetCreateRequest:
      type: object
      properties:
        amount:
          type: number
          description: Budget period amount
          format: double
        growthPerPeriod:
          type: number
          description: Periodical growth percentage in recurring budget
          format: double
          default: 0
        usePrevSpend:
          type: boolean
          description: Use last period spend
          default: false
    BudgetUpdateRequest:
      type: object
      properties:
        amount:
          type: number
          description: Budget period amount
          format: double
        growthPerPeriod:
          type: number
          description: Periodical growth percentage in recurring budget
          format: double
          default: 0
        usePrevSpend:
          type: boolean
          description: Use last period spend
          default: false
`

	pr2Base := pr1Head
	pr2Head := upstreamPrefix + `    Budget:
      type: object
      properties:
        amount:
          type: number
          description: Configured amount (omitted for seasonal)
        currentPeriodAmount:
          type: number
          readOnly: true
    BudgetCreateRequest:
      type: object
      properties:
        amount:
          type: number
          description: CREATE amount description
          format: double
        allowGrowth:
          type: boolean
          description: CREATE allowGrowth description
        growthPerPeriod:
          type: number
          description: CREATE growthPerPeriod description
          format: double
          default: 0
        usePrevSpend:
          type: boolean
          description: Use last period spend
          default: false
    BudgetUpdateRequest:
      type: object
      properties:
        amount:
          type: number
          description: UPDATE amount description
          format: double
        allowGrowth:
          type: boolean
          description: UPDATE allowGrowth description
        growthPerPeriod:
          type: number
          description: UPDATE growthPerPeriod description
          format: double
          default: 0
        usePrevSpend:
          type: boolean
          description: Use last period spend
          default: false
`

	stub := &stubGitHub{
		prs: map[int]prMetadata{
			72329: {
				State: "MERGED",
				MergeCommit: &struct {
					OID string `json:"oid"`
				}{OID: "pr1-head"},
			},
			72485: {
				State: "MERGED",
				MergeCommit: &struct {
					OID string `json:"oid"`
				}{OID: "pr2-head"},
			},
		},
		parents: map[string][]string{
			"pr1-head": {"pr1-base"},
			"pr2-head": {"pr2-base"},
		},
		files: map[string][]byte{
			"pr1-base": []byte(pr1Base),
			"pr1-head": []byte(pr1Head),
			"pr2-base": []byte(pr2Base),
			"pr2-head": []byte(pr2Head),
		},
	}

	target := filepath.Join(t.TempDir(), "openapi_spec_full.yml")
	if err := os.WriteFile(target, []byte(localInitial), 0o600); err != nil {
		t.Fatalf("writing target: %v", err)
	}

	var stdout, stderr bytes.Buffer
	cfg1 := config{PR: 72329, Repo: defaultRepo, UpstreamPath: defaultUpstreamPath, Target: target}
	if err := run(t.Context(), cfg1, stub, &stdout, &stderr); err != nil {
		t.Fatalf("applying PR 1 failed: %v", err)
	}

	cfg2 := config{PR: 72485, Repo: defaultRepo, UpstreamPath: defaultUpstreamPath, Target: target}
	if err := run(t.Context(), cfg2, stub, &stdout, &stderr); err != nil {
		t.Fatalf("applying PR 2 failed: %v", err)
	}

	finalBytes, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("reading final spec: %v", err)
	}
	finalDoc, err := parseYAMLDoc(finalBytes, "final")
	if err != nil {
		t.Fatalf("parsing final spec: %v", err)
	}
	pr2HeadDoc, err := parseYAMLDoc([]byte(pr2Head), "pr2Head")
	if err != nil {
		t.Fatalf("parsing pr2Head: %v", err)
	}

	finalSchemas := mappingValues(mappingValues(mappingValues(finalDoc)["components"])["schemas"])
	headSchemas := mappingValues(mappingValues(mappingValues(pr2HeadDoc)["components"])["schemas"])

	for _, name := range []string{"Budget", "BudgetCreateRequest", "BudgetUpdateRequest"} {
		if !nodesEqual(finalSchemas[name], headSchemas[name]) {
			t.Fatalf("schema %s does not match PR 2 head", name)
		}
	}
	if _, leaked := finalSchemas["UpstreamOnlyA"]; leaked {
		t.Fatal("unrelated upstream schema UpstreamOnlyA leaked into local spec")
	}

	// Re-applying PR 2 should be idempotent.
	stdout.Reset()
	if err := run(t.Context(), cfg2, stub, &stdout, &stderr); err != nil {
		t.Fatalf("idempotent re-apply failed: %v", err)
	}
	if !strings.Contains(stdout.String(), "already present") {
		t.Fatalf("expected 'already present' message on re-apply, got: %q", stdout.String())
	}
}

func TestRun_MergeConflictWritesMarkersAndFails(t *testing.T) {
	t.Parallel()

	local := `openapi: 3.0.1
components:
  schemas:
    Budget:
      type: object
      description: Local edit
`
	base := `openapi: 3.0.1
components:
  schemas:
    Budget:
      type: object
      description: Base text
`
	head := `openapi: 3.0.1
components:
  schemas:
    Budget:
      type: object
      description: PR edit
`

	stub := &stubGitHub{
		prs: map[int]prMetadata{
			100: {
				State: "MERGED",
				MergeCommit: &struct {
					OID string `json:"oid"`
				}{OID: "head-sha"},
			},
		},
		parents: map[string][]string{"head-sha": {"base-sha"}},
		files: map[string][]byte{
			"base-sha": []byte(base),
			"head-sha": []byte(head),
		},
	}

	target := filepath.Join(t.TempDir(), "spec.yml")
	if err := os.WriteFile(target, []byte(local), 0o600); err != nil {
		t.Fatalf("writing target: %v", err)
	}

	var stdout, stderr bytes.Buffer
	cfg := config{PR: 100, Repo: defaultRepo, UpstreamPath: defaultUpstreamPath, Target: target}
	err := run(t.Context(), cfg, stub, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "1 conflict(s)") {
		t.Fatalf("expected 1 conflict error, got: %v", err)
	}

	gotBytes, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatalf("reading conflicted target: %v", readErr)
	}
	got := string(gotBytes)
	for _, marker := range []string{"<<<<<<<", "|||||||", "=======", ">>>>>>>"} {
		if !strings.Contains(got, marker) {
			t.Fatalf("expected diff3 conflict marker %q in target file, got:\n%s", marker, got)
		}
	}
}

func TestRun_NoOpPRFailsLoudly(t *testing.T) {
	t.Parallel()

	spec := `openapi: 3.0.1
components:
  schemas:
    Budget:
      type: object
`
	stub := &stubGitHub{
		prs: map[int]prMetadata{
			200: {
				State: "MERGED",
				MergeCommit: &struct {
					OID string `json:"oid"`
				}{OID: "head-sha"},
			},
		},
		parents: map[string][]string{"head-sha": {"base-sha"}},
		files: map[string][]byte{
			"base-sha": []byte(spec),
			"head-sha": []byte(spec),
		},
	}

	target := filepath.Join(t.TempDir(), "spec.yml")
	if err := os.WriteFile(target, []byte(spec), 0o600); err != nil {
		t.Fatalf("writing target: %v", err)
	}

	var stdout, stderr bytes.Buffer
	cfg := config{PR: 200, Repo: defaultRepo, UpstreamPath: defaultUpstreamPath, Target: target}
	err := run(t.Context(), cfg, stub, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "does not modify") {
		t.Fatalf("expected 'does not modify' error, got: %v", err)
	}
}

func TestRun_VerificationFailureLeavesTargetUntouched(t *testing.T) {
	t.Parallel()

	// PR removes an anchor definition that is still referenced by a local-only schema.
	// git merge-file succeeds cleanly (exit 0), but VerifyMerge fails when parsing
	// the merged YAML because the alias is now broken. The target must remain untouched.
	local := `openapi: 3.0.1
components:
  x-doit-doc-anchors:
    oldAnchor: &oldAnchor Shared text
  schemas:
    LocalOnly:
      type: object
      description: *oldAnchor
    Keep:
      type: object
`
	base := `openapi: 3.0.1
components:
  x-doit-doc-anchors:
    oldAnchor: &oldAnchor Shared text
  schemas:
    Keep:
      type: object
`
	head := `openapi: 3.0.1
components:
  schemas:
    Keep:
      type: object
`

	stub := &stubGitHub{
		prs: map[int]prMetadata{
			300: {
				State: "MERGED",
				MergeCommit: &struct {
					OID string `json:"oid"`
				}{OID: "head-sha"},
			},
		},
		parents: map[string][]string{"head-sha": {"base-sha"}},
		files: map[string][]byte{
			"base-sha": []byte(base),
			"head-sha": []byte(head),
		},
	}

	target := filepath.Join(t.TempDir(), "spec.yml")
	if err := os.WriteFile(target, []byte(local), 0o600); err != nil {
		t.Fatalf("writing target: %v", err)
	}

	var stdout, stderr bytes.Buffer
	cfg := config{PR: 300, Repo: defaultRepo, UpstreamPath: defaultUpstreamPath, Target: target}
	err := run(t.Context(), cfg, stub, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "NOT modified") {
		t.Fatalf("expected verification failure leaving target untouched, got: %v", err)
	}

	afterBytes, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatalf("reading target after failure: %v", readErr)
	}
	if string(afterBytes) != local {
		t.Fatalf("target was modified despite verification failure:\ngot:\n%s\nwant:\n%s", string(afterBytes), local)
	}
}

func TestRun_DivergedRequiredListReapply(t *testing.T) {
	t.Parallel()

	local := `openapi: 3.0.1
components:
  schemas:
    Budget:
      type: object
      required:
        - localReq
        - name
        - scope
`
	base := `openapi: 3.0.1
components:
  schemas:
    Budget:
      type: object
      required:
        - name
        - scope
`
	head := `openapi: 3.0.1
components:
  schemas:
    Budget:
      type: object
      required:
        - name
        - scope
        - prReq1
        - prReq2
`

	stub := &stubGitHub{
		prs: map[int]prMetadata{
			400: {
				State: "MERGED",
				MergeCommit: &struct {
					OID string `json:"oid"`
				}{OID: "head-sha"},
			},
		},
		parents: map[string][]string{"head-sha": {"base-sha"}},
		files: map[string][]byte{
			"base-sha": []byte(base),
			"head-sha": []byte(head),
		},
	}

	target := filepath.Join(t.TempDir(), "spec.yml")
	if err := os.WriteFile(target, []byte(local), 0o600); err != nil {
		t.Fatalf("writing target: %v", err)
	}

	cfg := config{PR: 400, Repo: defaultRepo, UpstreamPath: defaultUpstreamPath, Target: target}
	var stdout, stderr bytes.Buffer
	if err := run(t.Context(), cfg, stub, &stdout, &stderr); err != nil {
		t.Fatalf("initial diverged merge failed: %v", err)
	}

	// Reapplying the exact same PR over the already-merged diverged list must succeed idempotently.
	stdout.Reset()
	if err := run(t.Context(), cfg, stub, &stdout, &stderr); err != nil {
		t.Fatalf("reapply over diverged required list failed: %v", err)
	}
	if !strings.Contains(stdout.String(), "already present") {
		t.Fatalf("expected 'already present' message on reapply, got: %q", stdout.String())
	}
}

func TestRun_MissingTargetFails(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	cfg := config{PR: 1, Repo: defaultRepo, UpstreamPath: defaultUpstreamPath, Target: filepath.Join(t.TempDir(), "missing.yml")}
	err := run(t.Context(), cfg, &stubGitHub{}, &stdout, &stderr)
	if err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected ErrNotExist, got: %v", err)
	}
}
