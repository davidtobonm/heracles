# Heracles

Heracles coordinates agent-driven software delivery from an understood problem to an emptied implementation backlog. It can run Planning, Issue, and Implementation Stages independently or compose them into a durable Labor with human Approval Gates.

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

Download the binary for your operating system and architecture from [GitHub Releases](https://github.com/davidtobonm/heracles/releases), verify it against `checksums.txt`, make it executable on Linux or macOS, and place it on `PATH`.

## Self-Install And Self-Update

Once a downloaded binary is executable, it can install itself:

```sh
heracles install
heracles install --system
heracles install --dir /custom/path
```

`heracles install` creates the target directory if needed, copies itself there, and reports whether the directory is already on `PATH`. Add `--json` for a stable machine-readable result.

`heracles update` checks GitHub Releases for a newer version, caching the result so repeated checks stay silent and inexpensive:

```sh
heracles update
heracles update --check
heracles update --apply
```

`heracles update --apply` downloads the release binary matching the current OS and architecture, verifies it against the release's `checksums.txt`, and only then atomically replaces the running executable.

## Develop

```sh
make check
make build
./bin/heracles version
```

CI runs formatting, static analysis, race-enabled tests, and cross-platform builds. Tests use deterministic fake executables and never invoke paid or authenticated agent CLIs.

See [CONTRIBUTING.md](CONTRIBUTING.md) for development and release expectations.

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

See [Provider Capabilities](docs/providers.md) for the exact capability matrix and official CLI references. Validated topology examples live under [`examples/`](examples/).

Launch-time Agent Role overrides preserve the original agent-loop command vocabulary and take precedence over configured preferences:

```sh
heracles run \
  --implementer opencode \
  --implementer-model opencode-go/kimi-k2.6 \
  --implementer-effort medium \
  --reviewer codex \
  --reviewer-model gpt-5.5 \
  --reviewer-effort high \
  --limit 40
```

`--limit` caps issues attempted by that invocation; omit it to drain all compatible Ready Issues. For OpenCode, the compatibility `--implementer-effort` and `--reviewer-effort` flags map to OpenCode variants.

Persist the same choices without editing `heracles.yaml`:

```sh
heracles config set --global --implementer opencode --implementer-model opencode-go/kimi-k2.6 --implementer-effort medium
heracles config set --project --reviewer codex --reviewer-model gpt-5.5 --reviewer-effort high
heracles config show --global
heracles config show --project
```

Precedence is launch flags, project preferences, global preferences, then `heracles.yaml`. Global preferences live at `~/.config/heracles/preferences.yaml`; project preferences live at `.heracles/preferences.yaml`.

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

## Labors

A Labor composes Planning, Issue, and Implementation Stages from a problem description through an empty defined backlog. It has two distinct durable human decisions:

1. Approve or reject the Planning PRD.
2. Approve or reject issue publication.

Rejection returns only the affected stage to revision. Approval and publication are idempotent, so interruption after a committed decision resumes without repeating it or duplicating issues. Active and blocked Labors can resume from their latest durable boundary; a Labor reaches `completed` only when its Implementation Stage reports the defined backlog exhausted. Cross-stage status, stage records, and both Approval Gates are mirrored into Execution History.

## CLI Control Surface

The CLI invokes the same high-level application services used by other Control Surfaces:

```sh
heracles plan --id plan-1 --problem "Clarify reliable delivery"
heracles issues --id issues-1 --prd .heracles/planning/plan-1/PRD.md
heracles run
heracles labor --id labor-1 --problem "Deliver the approved roadmap"

heracles approve planning labor-1 --reason "Scope approved"
heracles reject issues labor-1 --reason "Split the migration slice"
heracles retry labor-1-acme-backlog-7
heracles resume labor-1
heracles cancel labor-1 --reason "Superseded"
```

Use `heracles list` with `labors`, `issues`, `change-sets`, `gates`, `logs`, or `evidence`, and use `heracles inspect <kind> <id>` for a single record. Every operational command supports `--config`; stage commands discover `heracles.yaml` upward by default. Add `--json` for a stable machine-readable result and non-zero error exit behavior.

## MCP Control Surface

Start the standards-compatible newline-delimited stdio MCP server with:

```sh
heracles mcp serve
```

The server negotiates MCP protocol version `2025-11-25` and exposes the same named high-level operations as the CLI, including initialization, diagnostics, stages, Labors, approvals, retry, resume, cancel, list, and inspect. It deliberately exposes no arbitrary shell or command-execution tool. Tool failures return actionable MCP error results while the underlying application services preserve durable state.

Example MCP client configuration:

```json
{
  "mcpServers": {
    "heracles": {
      "command": "heracles",
      "args": ["mcp", "serve", "--config", "/absolute/path/to/heracles.yaml"]
    }
  }
}
```

Without `--config`, the long-lived server discovers a project upward. Before a project exists, call `heracles_init`; the server initializes the project and then switches to the fully wired local Control Surface.

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

## Implementation Stage

The independently runnable Implementation Stage composes the delivery boundaries into one durable attempt:

```text
claimed -> workspace_ready -> implemented -> reviewed -> verified -> delivered -> completed
```

The configured Implementer works in isolated Issue Workspace worktrees and must return auditable Red and Green Evidence or a reasoned TDD Exemption. A separately configured Reviewer checks the complete issue and PRD contract, may make corrective edits, and must verify those corrections. Local repository gates run before Change Set delivery.

Every transition, evidence reference, verification result, Review outcome, and Change Set is preserved in resumable attempt state and mirrored into Execution History. Failures and blocked outcomes publish actionable shared issue state and preserve the Issue Workspace. An explicit retry resumes that workspace from the last durable boundary. The backlog runner repeatedly refreshes newly eligible Ready Issues until no defined work remains or the remaining backlog is genuinely blocked.

## Change Sets

Heracles delivers a completed issue as one Change Set with exactly one pull request for each touched Target Repository. Every pull request links the issue and related pull requests, and includes the review summary, QA steps, and local evidence references.

Automatic merging is disabled by default. It must be enabled explicitly, and can declare cross-repository merge order:

```yaml
delivery:
  auto_merge: true
  merge_order: [backend, shared, frontend]
```

Before opening pull requests, every touched repository must pass its configured local verification. With automatic merging enabled, Heracles additionally waits for required GitHub checks before merging each pull request in order. If a later merge fails, already merged pull requests remain recorded, remaining pull requests stay open, and the Change Set becomes blocked for operator attention.

## Reference

- [Representative Workflows](docs/workflows.md)
- [Workflow Contracts](docs/contracts.md)
- [End-to-End Acceptance Scenario](docs/acceptance.md)
- [Product Requirements](PRD.md)
- [Architecture Decisions](docs/adr/)
- [MIT License](LICENSE)
