# Agent Skills

Repo-local [Agent Skills](https://agentskills.io) for working on this provider. Each subdirectory is one skill: a `SKILL.md` with YAML frontmatter (`name` must match the directory name), plus optional `references/`, `scripts/`, and `assets/`.

| Skill | Use when |
|-------|----------|
| `evaluate-api` | Assessing a DoIT API endpoint for provider compatibility |
| `github-workflow` | Branching, PRs, and CI conventions |
| `go-conventions` | Writing Go in this repo |
| `implement-datasource` | Adding a data source |
| `implement-resource` | Adding a resource (incl. the plan-first overlay pattern) |
| `implementation-conventions` | Cross-cutting provider conventions |
| `prepare-release` | Cutting a release |
| `register-custom-linter` | Adding a custom golangci-lint linter |
| `testing` | Writing and running unit / acceptance tests |

## Why `.agents/skills/`

The Agent Skills standard defines the skill *format* but deliberately leaves discovery paths to each client, and no single directory is native to every harness. `.agents/` (plural) is the cross-tool interoperability path — Gemini CLI and Antigravity read it directly, and it is documented as vendor-neutral.

Claude Code only scans `~/.claude/skills/`, `.claude/skills/`, and plugin skill directories, but it does follow symlinks. So the canonical copy lives here and `.claude/skills` is a symlink to it:

```bash
ln -s ../.agents/skills .claude/skills
```

That symlink is tracked in git; everything else under `.claude/` is machine-local and gitignored. To add support for another harness, add another symlink rather than a second copy.

> **Note:** `.agent/` (singular) is not a convention any tool implements — skills placed there are discovered by nothing.
