// apply-pr merges OpenAPI specification changes from an upstream GitHub pull
// request into a local spec file using a three-way merge against the PR's real
// base and head commits, then verifies the merged YAML semantically so hunks
// cannot silently land in the wrong schema or path.
//
// Usage:
//
//	go run ./tools/apply-pr -pr 72485
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	defaultRepo         = "doiteng/omni"
	defaultUpstreamPath = "services/external-api/openapi.yaml"
	defaultTargetPath   = "OpenAPI/openapi_spec_full.yml"
)

// config holds CLI options for applying an upstream PR's spec changes.
type config struct {
	PR           int
	Repo         string
	UpstreamPath string
	Target       string
}

// prMetadata is the subset of `gh pr view --json` output needed to resolve commits.
type prMetadata struct {
	State       string `json:"state"`
	BaseRefOID  string `json:"baseRefOid"`
	HeadRefOID  string `json:"headRefOid"`
	MergeCommit *struct {
		OID string `json:"oid"`
	} `json:"mergeCommit"`
}

// commitPair identifies the exact base and head commits representing a PR's changes.
type commitPair struct {
	BaseSHA     string
	HeadSHA     string
	Description string
}

// githubClient abstracts GitHub CLI queries for unit testing.
type githubClient interface {
	PRView(ctx context.Context, repo string, pr int) (prMetadata, error)
	CommitParents(ctx context.Context, repo, sha string) ([]string, error)
	MergeBase(ctx context.Context, repo, base, head string) (string, error)
	RawFile(ctx context.Context, repo, path, ref string) ([]byte, error)
}

// ghCLI implements githubClient using the `gh` CLI.
type ghCLI struct{}

func (ghCLI) PRView(ctx context.Context, repo string, pr int) (prMetadata, error) {
	out, err := runGH(ctx,
		"pr", "view", strconv.Itoa(pr),
		"--repo", repo,
		"--json", "state,baseRefOid,headRefOid,mergeCommit",
	)
	if err != nil {
		return prMetadata{}, fmt.Errorf("fetching PR #%d metadata from %s: %w", pr, repo, err)
	}
	var meta prMetadata
	if err := json.Unmarshal(out, &meta); err != nil {
		return prMetadata{}, fmt.Errorf("parsing PR #%d metadata: %w", pr, err)
	}
	return meta, nil
}

func (ghCLI) CommitParents(ctx context.Context, repo, sha string) ([]string, error) {
	endpoint := fmt.Sprintf("repos/%s/commits/%s", repo, sha)
	out, err := runGH(ctx, "api", endpoint)
	if err != nil {
		return nil, fmt.Errorf("fetching commit %s from %s: %w", shortSHA(sha), repo, err)
	}
	var payload struct {
		Parents []struct {
			SHA string `json:"sha"`
		} `json:"parents"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, fmt.Errorf("parsing commit %s metadata: %w", shortSHA(sha), err)
	}
	parents := make([]string, 0, len(payload.Parents))
	for _, p := range payload.Parents {
		if p.SHA != "" {
			parents = append(parents, p.SHA)
		}
	}
	return parents, nil
}

func (ghCLI) MergeBase(ctx context.Context, repo, base, head string) (string, error) {
	endpoint := fmt.Sprintf("repos/%s/compare/%s...%s", repo, base, head)
	out, err := runGH(ctx, "api", endpoint)
	if err != nil {
		return "", fmt.Errorf("resolving merge-base for %s...%s in %s: %w", shortSHA(base), shortSHA(head), repo, err)
	}
	var payload struct {
		MergeBaseCommit struct {
			SHA string `json:"sha"`
		} `json:"merge_base_commit"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return "", fmt.Errorf("parsing compare response for %s...%s: %w", shortSHA(base), shortSHA(head), err)
	}
	return payload.MergeBaseCommit.SHA, nil
}

func (ghCLI) RawFile(ctx context.Context, repo, path, ref string) ([]byte, error) {
	endpoint := fmt.Sprintf("repos/%s/contents/%s?ref=%s", repo, path, ref)
	out, err := runGH(ctx, "api", endpoint, "-H", "Accept: application/vnd.github.raw")
	if err != nil {
		return nil, fmt.Errorf("fetching %s at %s from %s: %w", path, shortSHA(ref), repo, err)
	}
	return out, nil
}

func runGH(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return nil, fmt.Errorf("%w: %s", err, msg)
		}
		return nil, err
	}
	return stdout.Bytes(), nil
}

func main() {
	var cfg config
	flag.IntVar(&cfg.PR, "pr", 0, "Upstream PR number (required)")
	flag.StringVar(&cfg.Repo, "repo", defaultRepo, "Upstream GitHub repository (owner/name)")
	flag.StringVar(&cfg.UpstreamPath, "upstream-path", defaultUpstreamPath, "Path to OpenAPI spec in the upstream repository")
	flag.StringVar(&cfg.Target, "target", defaultTargetPath, "Local OpenAPI spec file to update")
	flag.Parse()

	if cfg.PR <= 0 {
		fmt.Fprintln(os.Stderr, "Error: -pr must be a positive PR number (e.g. make apply-pr PR=61514)")
		os.Exit(1)
	}

	if err := run(context.Background(), cfg, ghCLI{}, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg config, client githubClient, stdout, stderr io.Writer) error {
	info, err := os.Stat(cfg.Target)
	if err != nil {
		return fmt.Errorf("reading target spec metadata %s: %w", cfg.Target, err)
	}
	targetMode := info.Mode().Perm()

	localBytes, err := os.ReadFile(cfg.Target)
	if err != nil {
		return fmt.Errorf("reading target spec %s: %w", cfg.Target, err)
	}

	commits, err := resolvePRCommits(ctx, client, cfg.Repo, cfg.PR, stderr)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Applying %s#%d (%s)\n", cfg.Repo, cfg.PR, commits.Description)

	baseBytes, err := client.RawFile(ctx, cfg.Repo, cfg.UpstreamPath, commits.BaseSHA)
	if err != nil {
		return err
	}
	headBytes, err := client.RawFile(ctx, cfg.Repo, cfg.UpstreamPath, commits.HeadSHA)
	if err != nil {
		return err
	}

	if bytes.Equal(baseBytes, headBytes) {
		return fmt.Errorf("%s#%d does not modify %s (%s..%s)", cfg.Repo, cfg.PR, cfg.UpstreamPath, shortSHA(commits.BaseSHA), shortSHA(commits.HeadSHA))
	}

	mergedBytes, conflicts, err := mergeFiles(ctx, localBytes, baseBytes, headBytes, cfg, commits)
	if err != nil {
		return err
	}
	if conflicts > 0 {
		if writeErr := os.WriteFile(cfg.Target, mergedBytes, targetMode); writeErr != nil {
			return fmt.Errorf("merge had %d conflict(s) and failed to write conflict markers to %s: %w", conflicts, cfg.Target, writeErr)
		}
		return fmt.Errorf(
			"three-way merge produced %d conflict(s) in %s; resolve the conflict markers or run `git checkout -- %s` to abort",
			conflicts, cfg.Target, cfg.Target,
		)
	}

	verifyRes, err := VerifyMerge(localBytes, baseBytes, headBytes, mergedBytes)
	if err != nil {
		return fmt.Errorf("post-merge verification failed (%s was NOT modified): %w", cfg.Target, err)
	}
	if len(verifyRes.Failures) > 0 {
		var b strings.Builder
		fmt.Fprintf(&b, "post-merge verification failed for %d item(s) (%s was NOT modified):", len(verifyRes.Failures), cfg.Target)
		for _, f := range verifyRes.Failures {
			fmt.Fprintf(&b, "\n  - %s", f.String())
		}
		return errors.New(b.String())
	}

	if bytes.Equal(localBytes, mergedBytes) {
		fmt.Fprintf(stdout, "PR #%d changes are already present in %s (no changes made).\n", cfg.PR, cfg.Target)
		return nil
	}

	if err := os.WriteFile(cfg.Target, mergedBytes, targetMode); err != nil {
		return fmt.Errorf("writing merged spec to %s: %w", cfg.Target, err)
	}

	fmt.Fprintf(stdout, "Updated %s and verified %d touched spec entry/entries:\n", cfg.Target, len(verifyRes.Touched))
	for _, entry := range verifyRes.Touched {
		fmt.Fprintf(stdout, "  - %s\n", entry)
	}
	if len(verifyRes.Diverged) > 0 {
		fmt.Fprintln(stdout, "Note: the following touched entries differed between local and PR base independently (merged and verified field-by-field):")
		for _, entry := range verifyRes.Diverged {
			fmt.Fprintf(stdout, "  - %s\n", entry)
		}
	}
	return nil
}

// resolvePRCommits determines the exact (base, head) commit pair representing a
// PR's changes. Merged PRs use mergeCommit^1..mergeCommit so the exact landed
// squash/merge commit is applied; open or unmerged PRs use the true merge-base
// of baseRefOid and headRefOid rather than baseRefOid directly so stale PR
// branches never revert intervening target-branch commits.
func resolvePRCommits(ctx context.Context, client githubClient, repo string, pr int, stderr io.Writer) (commitPair, error) {
	meta, err := client.PRView(ctx, repo, pr)
	if err != nil {
		return commitPair{}, err
	}

	if meta.State == "MERGED" && meta.MergeCommit != nil && meta.MergeCommit.OID != "" {
		headSHA := meta.MergeCommit.OID
		parents, parentErr := client.CommitParents(ctx, repo, headSHA)
		if parentErr != nil {
			return commitPair{}, parentErr
		}
		if len(parents) == 0 {
			return commitPair{}, fmt.Errorf("merge commit %s for PR #%d has no parents", shortSHA(headSHA), pr)
		}
		baseSHA := parents[0]
		if baseSHA == headSHA {
			return commitPair{}, fmt.Errorf("PR #%d base and head resolve to the same commit %s", pr, shortSHA(headSHA))
		}
		return commitPair{
			BaseSHA:     baseSHA,
			HeadSHA:     headSHA,
			Description: fmt.Sprintf("merge commit %s, parent %s", shortSHA(headSHA), shortSHA(baseSHA)),
		}, nil
	}

	if meta.State == "CLOSED" {
		fmt.Fprintf(stderr, "Warning: %s#%d is CLOSED (unmerged); resolving merge-base..head anyway.\n", repo, pr)
	}
	if meta.BaseRefOID == "" || meta.HeadRefOID == "" {
		return commitPair{}, fmt.Errorf("PR #%d is missing baseRefOid or headRefOid", pr)
	}
	baseSHA, err := client.MergeBase(ctx, repo, meta.BaseRefOID, meta.HeadRefOID)
	if err != nil {
		return commitPair{}, err
	}
	if baseSHA == "" {
		return commitPair{}, fmt.Errorf("empty merge-base returned for PR #%d", pr)
	}
	if baseSHA == meta.HeadRefOID {
		return commitPair{}, fmt.Errorf("PR #%d has no commits ahead of base (%s)", pr, shortSHA(baseSHA))
	}
	return commitPair{
		BaseSHA:     baseSHA,
		HeadSHA:     meta.HeadRefOID,
		Description: fmt.Sprintf("PR head %s, merge-base %s", shortSHA(meta.HeadRefOID), shortSHA(baseSHA)),
	}, nil
}

// mergeFiles executes `git merge-file --diff3` on temporary copies of local,
// base, and head specs. It returns the resulting file bytes, the conflict count
// (0 for a clean merge), and any fatal execution error.
func mergeFiles(ctx context.Context, localBytes, baseBytes, headBytes []byte, cfg config, commits commitPair) ([]byte, int, error) {
	tmpDir, err := os.MkdirTemp("", "apply-pr-*")
	if err != nil {
		return nil, 0, fmt.Errorf("creating temp directory: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	root, err := os.OpenRoot(tmpDir)
	if err != nil {
		return nil, 0, fmt.Errorf("opening temp directory root: %w", err)
	}
	defer func() {
		_ = root.Close()
	}()

	if writeErr := root.WriteFile("local.yml", localBytes, 0o600); writeErr != nil {
		return nil, 0, fmt.Errorf("writing temp local spec: %w", writeErr)
	}
	if writeErr := root.WriteFile("base.yml", baseBytes, 0o600); writeErr != nil {
		return nil, 0, fmt.Errorf("writing temp base spec: %w", writeErr)
	}
	if writeErr := root.WriteFile("head.yml", headBytes, 0o600); writeErr != nil {
		return nil, 0, fmt.Errorf("writing temp head spec: %w", writeErr)
	}

	localPath := filepath.Join(tmpDir, "local.yml")
	basePath := filepath.Join(tmpDir, "base.yml")
	headPath := filepath.Join(tmpDir, "head.yml")

	localLabel := fmt.Sprintf("local (%s)", cfg.Target)
	baseLabel := fmt.Sprintf("PR #%d base (%s)", cfg.PR, shortSHA(commits.BaseSHA))
	headLabel := fmt.Sprintf("PR #%d head (%s)", cfg.PR, shortSHA(commits.HeadSHA))

	cmd := exec.CommandContext(ctx, "git", "merge-file",
		"--diff3",
		"-L", localLabel,
		"-L", baseLabel,
		"-L", headLabel,
		localPath, basePath, headPath,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	runErr := cmd.Run()

	conflicts := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) && exitErr.ExitCode() >= 1 && exitErr.ExitCode() <= 127 {
			conflicts = exitErr.ExitCode()
		} else {
			msg := strings.TrimSpace(stderr.String())
			if msg != "" {
				return nil, 0, fmt.Errorf("git merge-file failed: %w: %s", runErr, msg)
			}
			return nil, 0, fmt.Errorf("git merge-file failed: %w", runErr)
		}
	}

	mergedBytes, err := root.ReadFile("local.yml")
	if err != nil {
		return nil, 0, fmt.Errorf("reading merged spec: %w", err)
	}
	return mergedBytes, conflicts, nil
}

func shortSHA(sha string) string {
	if len(sha) > 10 {
		return sha[:10]
	}
	return sha
}
