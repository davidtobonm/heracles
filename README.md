# 🏹 Heracles

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

Running `heracles init` with none of `--tracker`, `--repo`, or other configuration flags launches an interactive setup wizard instead. For a new project it offers **Fast Setup** (an Implementer profile with sensible defaults) or **Complete Setup** (configure every Agent Role, Labor and delivery policy, and per-repository verification commands). Run against an existing `heracles.yaml`, it offers **Fast Reconfigure**, **Complete Reconfigure**, **Repair Missing Values** (fills in only what is absent without touching existing settings), or **Cancel**.

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

Heracles supports Codex, Claude Code, OpenCode, Kimi Code, OpenClaw, and Hermes. Provider-specific model, effort, and variant settings are validated instead of silently ignored.

### Environment And Secrets

A launched provider or verification command only receives a fixed set of essential process variables (`HOME`, `PATH`, `LANG`, `SHELL`, `TERM`, and similar) plus whatever an Agent Profile's `env_allowlist` adds, such as `CI_TOKEN` above. Repository-level `verify_env` works the same way for verification commands. If a required verification environment variable is missing, `heracles doctor` reports it as a blocking finding before any work starts. Values that look like secrets (API keys, tokens, credentials referenced by an allowlisted variable) are scrubbed to `***REDACTED***` before they appear in CLI output, logs under `.heracles/logs/`, or Execution History — Heracles never persists or displays the underlying value.

Each provider's adapter launches with its verified full-permission bypass flags so unattended Agent Roles never stop for interactive tool approval. Before the first Labor that launches a given provider, Heracles shows a one-time disclosure ("Heracles will launch `<provider>` with its verified full-permission bypass, granting unattended access to your shell and Target Repositories for this Labor. Continue?") and records the acknowledgement per project; later Labors using an already-acknowledged provider proceed without re-prompting.

See [Provider Capabilities](docs/providers.md) for the exact capability matrix and official CLI references. Validated topology examples live under [`examples/`](examples/).

### Doctor

Run `heracles doctor` before a Labor to validate the project. Diagnostics never invoke a paid agent session or authenticate a provider. It checks:

- the `git` and `gh` executables and GitHub authentication
- access to the configured Issue Tracker and the presence of every required shared-state label (`heracles:ready`, `heracles:blocked`, `heracles:in-progress`, `heracles:review`, `heracles:done`, `heracles:hitl`, `heracles:tdd-exempt`, `heracles:implementation`, `heracles:obsolete`)
- each Target Repository's path, base branch, configured verification commands and verification environment variables, auto-merge permission, and GitHub Actions CI configuration
- the Issue Workspace root
- every configured Agent Profile: that it resolves, that its provider adapter validates the profile, that its executable is on `PATH`, that the provider is authenticated, and (where applicable) that its configured model is available
- the stdio MCP control surface and shipped skills availability

Findings are either blocking (`OK: false`, stop before starting a Labor) or warnings (e.g. auto-merge not permitted on a repository, CI not configured, the Issue Workspace root missing) which are reported but do not by themselves stop execution. `heracles doctor --fix` performs safe, idempotent repairs limited to creating missing Tracker labels and creating the Issue Workspace root; it never authenticates a provider, changes secrets, or makes destructive repository changes. Add `--json` for a stable machine-readable report. `heracles labor` and `heracles run` run Doctor's checks as a mandatory, non-mutating preflight and stop on any blocking finding.

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

`heracles config` also supports `unset`, `append`, and `path`:

```sh
heracles config unset --global agents.implementer.model
heracles config append --project agents.implementer.env_allowlist=CI_TOKEN
heracles config path --global
```

`unset` removes a field (prompting for confirmation unless `--yes` is given) and drops the role entirely once it has no remaining fields. `append` adds a value to a list-typed field, such as `env_allowlist`, without replacing the existing list. `path` prints the resolved preferences file path without reading or writing it.

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
heracles status labor-1
```

Use `heracles list` with `labors`, `issues`, `change-sets`, `gates`, `logs`, or `evidence`, and use `heracles inspect <kind> <id>` for a single record. Every operational command supports `--config`; stage commands discover `heracles.yaml` upward by default. Add `--json` for a stable machine-readable result and non-zero error exit behavior. Structured (`--json`) output never includes interactive prompts or update notices.

## Status

`heracles status` reports, for every local Labor (or the one named by an optional Labor ID), its current stage, Defined Backlog progress, Planning and Issue Approval Gate state, pending Change Sets, any Human-In-The-Loop issues still blocking it, and whether `heracles resume` can continue it. A blocked Labor includes `blockers` describing why and `guidance` suggesting what to do next. If a Labor's local state could not be read (for example, after an incompatible upgrade per [Interruption, Cancellation, And Recovery](#interruption-cancellation-and-recovery)), `recovery_error` explains why instead of the rest of the report. Add `--json` for the same information as a stable machine-readable document.

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

### Provider-Specific MCP Setup

Every `heracles plan`, `heracles labor`, and `heracles run` session automatically receives a temporary `heracles` MCP server entry and Heracles's bundled skills (see [Skills](#skills)) for the duration of that session, without overwriting any existing provider configuration. Manual setup below is only needed to use the Heracles MCP server outside a Heracles-launched session.

Claude Code, OpenCode, Kimi Code, and OpenClaw read `.mcp.json` (project) or their global MCP config in the same `mcpServers` shape:

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

Codex reads `.codex/config.toml` (project) or `~/.codex/config.toml` (global):

```toml
[mcp_servers.heracles]
command = "heracles"
args = ["mcp", "serve", "--config", "/absolute/path/to/heracles.yaml"]
```

Hermes follows the same `mcpServers` JSON shape as Claude Code; consult its documentation for the config file location.

### Troubleshooting

- Run `heracles doctor` to validate the project, Agent Profiles, and tracker access before starting a Labor.
- Smoke-test the MCP server directly without a paid provider turn:

  ```sh
  echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}' | heracles mcp serve --config /absolute/path/to/heracles.yaml
  ```

  A successful response confirms the server starts, finds the Project Configuration, and negotiates the protocol version.

## Skills

Heracles ships three skills used by its own Planning and Issue Stages, kept skills.sh-compatible so they also work outside Heracles-launched sessions:

- `grill-with-docs` - runs the interactive Grilling Session (ADR 0013) that explores Target Repositories and documentation before drafting a PRD.
- `to-prd-for-heracles` - drafts and publishes the durable PRD Issue (ADR 0014).
- `to-issues-for-heracles` - converts an approved PRD into Heracles-compatible issue proposals (ADR 0015).

List the bundled skills and detected providers:

```sh
heracles skills list
```

Install them into a provider's skill directory, following the `.{provider}/skills` convention skills.sh uses:

```sh
heracles skills install --project --provider claude
heracles skills install --global --provider codex
```

`--provider` may be repeated; if omitted, Heracles installs to every detected provider. Locally modified skills are never overwritten unless `--force` is given. Heracles-launched sessions receive these skills automatically and do not require this command.

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

Across all of this, only one Labor may actively mutate a project at a time. `heracles labor`, `run`, and `resume` acquire a project-level lock under `.heracles/` for their duration and release it on exit; a concurrent invocation fails immediately rather than racing the first. If a previous run was killed without releasing the lock, Heracles checks whether the recorded process is still running and, if not, treats the lock as stale and proceeds — there is no manual force-unlock.

## Implementation Stage

The independently runnable Implementation Stage composes the delivery boundaries into one durable attempt:

```text
claimed -> workspace_ready -> implemented -> reviewed -> verified -> delivered -> completed
```

The configured Implementer works in isolated Issue Workspace worktrees and must return auditable Red and Green Evidence or a reasoned TDD Exemption. A separately configured Reviewer checks the complete issue and PRD contract, may make corrective edits, and must verify those corrections. Local repository gates run before Change Set delivery.

Every transition, evidence reference, verification result, Review outcome, and Change Set is preserved in resumable attempt state and mirrored into Execution History. Failures and blocked outcomes publish actionable shared issue state and preserve the Issue Workspace. An explicit retry resumes that workspace from the last durable boundary. The backlog runner repeatedly refreshes newly eligible Ready Issues until no defined work remains or the remaining backlog is genuinely blocked.

## Change Sets

Heracles delivers a completed issue as one Change Set with exactly one pull request for each touched Target Repository. Every pull request links the issue and related pull requests, and includes the review summary, QA steps, and local evidence references.

Automatic merging is enabled by default and can declare cross-repository merge order; set `auto_merge: false` to require manual merges instead:

```yaml
delivery:
  auto_merge: true
  merge_order: [backend, shared, frontend]
```

Before opening pull requests, every touched repository must pass its configured local verification. With automatic merging enabled, Heracles additionally waits for required GitHub checks before merging each pull request in order. If a repository does not permit auto-merge, `heracles doctor` reports it as a warning and that repository's pull requests fall back to review mode: the issue enters a review state and `heracles resume` reconciles the merge once it lands. If a later merge fails, already merged pull requests remain recorded, remaining pull requests stay open, and the Change Set becomes blocked for operator attention.

### Correction Cycles

If a pull request is blocked by requested reviewer changes or a CI failure, Heracles first classifies the failure as **infrastructure** (e.g. a runner outage or transient network error — retried as-is) or **code** (a real failure in the delivered change). A code failure or requested change starts a correction cycle: the Issue Workspace is preserved, the Implementer and Reviewer make corrective changes, and verification reruns. Up to 3 correction cycles are attempted automatically; if the issue is still blocked after that, the Change Set is left blocked for operator attention with a diagnostic status explaining the last failure.

## Interruption, Cancellation, And Recovery

`heracles run`, `labor`, and `resume` durably checkpoint progress so they can be safely interrupted:

- The **first** `Ctrl-C` (SIGINT) or SIGTERM asks the running Labor to stop at its next durable boundary, leaving resumable state. The current issue's Issue Workspace is preserved, not torn down mid-edit.
- A **second** `Ctrl-C`/SIGTERM cancels a hard context, terminating in-flight provider subprocesses immediately. The most recently checkpointed state is preserved even so.

`heracles resume <labor-id>` continues a stopped or blocked Labor from its latest durable boundary without repeating committed work or duplicating issues or pull requests.

`heracles cancel <labor-id>` marks a Labor as cancelled in local state. It is irreversible locally and prompts for confirmation ("Cancel Labor `<id>`? This cannot be undone locally. Issues, Pull Requests, and Change Sets on GitHub are left unchanged.") unless `--yes` is given; GitHub issues, pull requests, and Change Sets it already created are left exactly as they are.

### Upgrades And State Migration

Local state under `.heracles/` (Labor state, preferences, history) carries a schema version. When a newer Heracles version reads state written by an older one, it applies versioned migrations automatically after writing a backup of the original file. A migration that would change meaning in a way that cannot be safely automated requires explicit confirmation before it mutates anything. An **older** binary that encounters state written by a newer schema version refuses to read or rewrite it, so downgrading never silently corrupts state — `heracles status` reports the unreadable Labor's `recovery_error` instead.

## Reference

- [Representative Workflows](docs/workflows.md)
- [Workflow Contracts](docs/contracts.md)
- [End-to-End Acceptance Scenario](docs/acceptance.md)
- [Product Requirements](PRD.md)
- [Architecture Decisions](docs/adr/)
- [MIT License](LICENSE)
