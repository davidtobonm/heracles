# Heracles

Heracles coordinates agent-driven software delivery from an understood problem to an emptied implementation backlog.

The project is under active development. The current binary foundation provides stable help and version contracts while the Planning, Issue, and Implementation Stages are built from the [product requirements](PRD.md).

## Requirements

- Go 1.24 or newer when building from source
- Git and GitHub CLI for the workflows that will coordinate repositories and issues

## Install From Source

```sh
git clone https://github.com/davidtobonm/heracles.git
cd heracles
make install
heracles --help
```

Versioned release binaries for Linux, macOS, and Windows are published from `v*` tags.

## Develop

```sh
make check
make build
./bin/heracles version
```

CI runs formatting, static analysis, race-enabled tests, and cross-platform builds. Tests use deterministic fake executables and never invoke paid or authenticated agent CLIs.

## Initialize A Project

From anywhere inside a Git repository with a GitHub `origin`:

```sh
heracles init
```

Heracles writes `heracles.yaml` at the repository root and uses that repository as both the Issue Tracker and first Target Repository.

For sibling or unrelated repositories, provide a separate tracker and repeat `--repo`:

```sh
heracles init \
  --tracker acme/delivery-backlog \
  --repo ../backend \
  --repo /absolute/path/to/frontend
```

Repository paths passed as relative paths are stored relative to `heracles.yaml`; absolute paths remain absolute. Project Configuration discovery searches upward from the current directory, and later commands can select a configuration explicitly with `--config`.

## Local Execution History

Heracles stores authoritative local workflow state in `.heracles/history.db`. Human-readable JSONL logs live under `.heracles/logs/`, and evidence artifacts live under `.heracles/artifacts/`. Reopening a project rebuilds JSONL mirrors from committed SQLite events, so interrupted workflows remain inspectable and resumable.

## Agent Profiles And Diagnostics

Agent Roles select reusable, inheritable profiles:

```yaml
agents:
  default_profile: default
  profiles:
    default:
      provider: codex
      model: gpt-5.4
      effort: high
      timeout: 45m
      env_allowlist: [PATH, HOME]
      concurrency: 1
    reviewer:
      extends: default
      provider: claude
      model: sonnet
  roles:
    reviewer: reviewer
```

Heracles supports Codex, Claude Code, OpenCode, and Kimi Code. Provider-specific model, effort, and variant settings are validated instead of silently ignored. Run `heracles doctor` before a Labor to validate the Project Configuration, Target Repositories, GitHub authentication, Agent Profiles, capabilities, and required executables. Diagnostics never invoke a paid agent session.

## Planning Stage

The independently runnable Planning Stage gives the configured Planner every Target Repository workspace and relevant existing documentation. It clarifies a problem within a soft Question Budget, asks for only necessary documentation updates, persists its PRD under `.heracles/planning/<stage-id>/PRD.md`, and pauses at a durable Approval Gate.

```yaml
planning:
  question_budget: 20
```

Planning may finish before the budget is exhausted. Questions beyond the budget remain pending until a user explicitly permits them. Approval completes the stage; rejection returns the same durable stage to revision without replaying already committed agent work.

## Issue Stage

The independently runnable Issue Stage gives a separately configured Issue Author an approved PRD and asks for tracer-bullet proposals. Every proposal must classify as AFK or HITL, cover user stories, include acceptance criteria, use full GitHub issue URLs for dependencies, and declare Exclusive Scopes where concurrent work would be unsafe.

The complete proposal set pauses at an Approval Gate before publication. Approved publication is restart-safe: each created issue URL is persisted immediately, so resuming after an interruption skips already-created issues. AFK issues receive `heracles:ready` and remain ineligible until their dependencies resolve; human-dependent issues receive `heracles:hitl`.

The bundled skills.sh-compatible skill lives at `skills/to-issues-for-heracles/SKILL.md` and can be installed globally or copied into a project skill directory.

## Heracles-Compatible Issues

The GitHub Issue Tracker uses explicit shared state labels:

- `heracles:ready`
- `heracles:blocked`
- `heracles:in-progress`
- `heracles:done`
- `heracles:hitl`
- `heracles:tdd-exempt`

Only open `heracles:ready` issues without HITL or unresolved dependency state are eligible for unattended execution. Dependencies belong under `## Blocked by` as full `https://github.com/<owner>/<repo>/issues/<number>` URLs, allowing work to depend on issues in other repositories. Claim, block, and completion transitions preserve unrelated labels and publish shared status comments.

## Issue Workspaces

Each issue runs in a coordinated Issue Workspace with one temporary Git worktree per Target Repository. The default lifecycle policy is:

```yaml
workspaces:
  root: .heracles/workspaces
  cleanup_success: true
  preserve_failed: true
  preserve_blocked: true
```

Original working copies may remain dirty and on their current branches. Heracles records baseline commits, detects both committed and uncommitted issue changes, preserves failed or blocked work for inspection and resume, and removes successful worktrees according to policy.

Each Target Repository may declare its own verification commands:

```yaml
repositories:
  - name: backend
    path: ../backend
    github: acme/backend
    base_branch: main
    verify:
      - go test ./...
      - go vet ./...
```

Delivery is blocked unless Red Evidence records an observed failure before implementation and Green Evidence records a later passing result with linked artifacts. A `heracles:tdd-exempt` issue may proceed only with a reasoned exemption. The Reviewer receives the issue, PRD, changes, evidence, and verification results, checks correctness, YAGNI, and DRY, and may make corrective changes before rerunning verification.

Issue execution is sequential by default:

```yaml
labor:
  issue_concurrency: 1
```

Higher concurrency still respects dependencies, issue `## Exclusive Scopes`, isolated Issue Workspaces, and each selected Agent Profile's concurrency limit. The scheduler skips temporarily conflicting work so independent Ready Issues are not starved, then repeatedly selects newly unblocked work until the defined backlog is empty.

## Change Sets

Heracles delivers a completed issue as one Change Set with exactly one pull request for each touched Target Repository. Every pull request links the issue and related pull requests, and includes the review summary, QA steps, and local evidence references.

Automatic merging is disabled by default. It must be enabled explicitly, and can declare cross-repository merge order:

```yaml
delivery:
  auto_merge: true
  merge_order: [backend, shared, frontend]
```

Before opening pull requests, every touched repository must pass its configured local verification. With automatic merging enabled, Heracles additionally waits for required GitHub checks before merging each pull request in order. If a later merge fails, already merged pull requests remain recorded, remaining pull requests stay open, and the Change Set becomes blocked for operator attention.
