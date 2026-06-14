# PRD: Complete Heracles as a Dogfooded Delivery Orchestrator

## Problem Statement

Heracles has a working Go foundation for coordinating agent-driven delivery, but its current CLI and documentation do not expose the complete workflow needed to use it as a daily tool. Planning is not yet a provider-owned interactive Grilling Session, preferences are incomplete, provider support and setup are narrow, and delivery behavior is not fully connected from an approved PRD through an emptied Defined Backlog.

Users need one installable command that can initialize a project, launch their preferred agent providers with the right skills and MCP tools, turn an approved PRD into reconciled implementation issues, implement and review those issues, recover from ordinary interruptions, and deliver verified Change Sets. Heracles must prove this workflow by using itself to complete this PRD.

## Solution

Complete Heracles as a portable CLI for macOS, Linux, and Windows. `heracles init` configures a project through fast or complete setup. `heracles plan` launches an interactive Planner Grilling Session and publishes a PRD Issue for approval. Approval automatically launches background issue generation. `heracles labor` continues through implementation, review, verification, CI, and delivery, while `heracles issues`, `heracles run`, `heracles status`, `heracles doctor`, configuration, skills, installation, and update commands expose the individual operational surfaces.

Heracles will support Codex, Claude Code, OpenCode, Kimi Code, OpenClaw, and Hermes as direct CLI providers. It will inject shipped planning and issue-authoring skills plus Heracles MCP configuration into launched sessions. Preferences may be global, project-local, or launch-specific. Execution state remains local under `.heracles/`; GitHub remains the Issue Tracker and delivery surface, not a Labor-state recovery backend.

## User Stories

1. As a new user, I want to install Heracles as a user or system command, so that I can invoke it from any project.
2. As a maintainer, I want release binaries for macOS, Linux, and Windows, so that Heracles is portable across common development environments.
3. As a user, I want `heracles init` to offer Fast Setup and Complete Setup first, so that I can choose the appropriate configuration depth.
4. As a user, I want Fast Setup to ask only for provider, model, effort, and variant choices, so that I can begin quickly.
5. As a user, I want Complete Setup to configure every Agent Role and Labor policy, so that advanced behavior is explicit.
6. As a user, I want to reuse the Implementer provider for every Agent Role during setup, so that repetitive configuration is minimized.
7. As a user, I want setup to detect installed providers and supported capabilities without invoking paid sessions, so that choices are reliable.
8. As a user, I want setup to detect and confirm verification commands from the project stack, so that delivery validates the right behavior.
9. As a user, I want setup to create a bootstrap PRD and issues when a project has no quality commands, so that agents can add them before feature work.
10. As a returning user, I want initialization to reconfigure or repair an existing project without silently overwriting it, so that prior choices remain safe.
11. As a user, I want global and project Preferences for every Agent Role, so that provider choices do not require editing portable project configuration.
12. As a user, I want configuration keys accepted as dotted pairs and dashed launch flags, so that commands are concise and scriptable.
13. As a user, I want complete configuration management with show, set, unset, path, append, and confirmation behavior, so that Preferences are maintainable.
14. As a user, I want launch overrides to take precedence over project and global Preferences, so that one-off runs are easy.
15. As a user, I want unsupported provider settings rejected immediately, so that Heracles never silently ignores configuration.
16. As a user, I want Codex, Claude Code, OpenCode, Kimi Code, OpenClaw, and Hermes available as direct providers, so that I can use my preferred CLI.
17. As a user, I want Heracles to use verified provider-specific permission bypasses, so that unattended Agent Roles can complete their work.
18. As a user, I want first-use acknowledgement of full-permission execution, so that the project trust boundary is explicit.
19. As a user, I want provider authentication left to each provider CLI, so that Heracles never handles provider credentials.
20. As a user, I want `heracles plan` to launch my preferred Planner in an interactive terminal, so that the provider owns the complete Grilling Session context.
21. As a user, I want the Planner to ask one question at a time and respect a Question Budget, so that planning remains focused.
22. As a user, I want the Planner to ask whether to exceed the Question Budget or proceed to a PRD, so that I control planning depth.
23. As a user, I want the Planner to publish one durable draft PRD Issue, so that review and revisions share one identity.
24. As a user, I want to approve the PRD from inside the provider session, so that I do not have to exit and re-enter planning.
25. As a user, I want PRD approval to automatically launch background issue generation, so that the Issue Stage needs no routine review.
26. As a user, I want exceptional Issue Author ambiguity to block without publishing issues, so that unsafe assumptions do not enter the backlog.
27. As a user, I want `heracles issues <prd-url>` to generate issues from an approved PRD, so that I can run the Issue Stage independently.
28. As a user, I want implementation issues linked to one Parent PRD and identified by semantic IDs, so that the Defined Backlog remains stable across revisions.
29. As a user, I want unchanged PRDs to cause no issue mutations, so that regeneration is idempotent.
30. As a user, I want changed PRDs to update untouched work, create new work, and preserve in-progress or completed work, so that revisions are safe.
31. As a user, I want obsolete untouched issues marked but not closed automatically, so that history remains visible.
32. As a user, I want semantic dependency references validated and converted into GitHub URLs, so that generated issue graphs are executable.
33. As a user, I want `heracles labor "Problem"` to run planning, issue creation, implementation, and delivery, so that the complete workflow is connected.
34. As a user, I want `heracles run <prd-url>` to process only that PRD's Defined Backlog, so that unrelated tracker work is untouched.
35. As a user, I want bare `heracles run` to require confirmation before processing all eligible issues, so that broad execution is deliberate.
36. As a user, I want Ready Issues refreshed after each batch, so that newly unblocked work runs in the same Labor.
37. As a user, I want HITL Issues skipped while independent agent work continues, so that manual work does not unnecessarily stop delivery.
38. As a user, I want a Labor blocked only when unresolved HITL work blocks remaining agent work, so that completion status is meaningful.
39. As a user, I want configurable issue limits and concurrency, so that I can control cost and throughput.
40. As a user, I want concurrent issues isolated and serialized when dependencies, repositories, or conflict keys overlap, so that parallel work is safe.
41. As a user, I want only one active Labor per project, so that local state and repository coordination remain predictable.
42. As a user, I want auto-merge enabled by default with an explicit opt-out, so that verified Change Sets deliver without routine intervention.
43. As a user, I want automatic correction cycles after requested changes or CI failures, so that a Labor can continue without manual restarts.
44. As a user, I want exhausted corrections to preserve work and block clearly, so that failures are diagnosable.
45. As a user, I want verification and providers to receive only allowlisted environment variables, so that secrets and unrelated state are protected.
46. As a user, I want secret values redacted from output and persistence, so that logs are safe to retain.
47. As a user, I want mandatory Doctor preflight before every Labor, so that predictable setup blockers stop before execution.
48. As a user, I want `heracles doctor --fix` to perform safe repairs, so that common setup problems are easy to resolve.
49. As an automation client, I want JSON output from Doctor, status, and Labor execution, so that Heracles can integrate with other tools.
50. As a user, I want `heracles status` to inspect current or past local Labors without mutation, so that I understand progress and recovery options.
51. As a user, I want graceful first-interrupt and immediate second-interrupt behavior, so that active work remains resumable when possible.
52. As a user, I want irreversible cancellation to leave GitHub work unchanged, so that published work is never destroyed accidentally.
53. As a user, I want compatible Heracles upgrades to resume Labors and incompatible upgrades to stop before mutation, so that format changes are safe.
54. As a user, I want shipped skills installable through Heracles or skills.sh, so that provider sessions can use the same workflows outside a Labor.
55. As a user, I want Heracles-launched sessions to receive the shipped skills and MCP tools automatically, so that no manual session setup is required.
56. As a user, I want provider-specific MCP setup examples and smoke tests, so that I can connect Heracles to external provider sessions.
57. As a user, I want silent cached update checks outside active Labors, so that I learn about new releases without workflow disruption.
58. As a maintainer, I want Heracles to complete this PRD through its own Labor, so that the initial release is proven end to end.

## Implementation Decisions

- The canonical full workflow command is `heracles labor "Problem"`, with optional `--problem` and `--id`; conflicting problem inputs fail.
- `heracles plan` ends after PRD approval and background issue publication. `heracles issues <prd-url>` runs issue publication independently. `heracles run <prd-url>` executes a Defined Backlog.
- The Planner owns an interactive provider terminal and resumable provider conversation when supported. It uses shipped `grill-with-docs` and `to-prd-for-heracles` skills.
- The Issue Author runs non-interactively in the background with shipped `to-issues-for-heracles`; routine issue publication has no separate Approval Gate.
- PRD Issues use `heracles:prd`, `heracles:review`, and `heracles:approved`. A canonical title/body SHA-256 revision marker determines whether reconciliation is required.
- Generated issues contain a full Parent PRD URL, `heracles:implementation`, a unique semantic issue ID, target repositories, optional conflict keys, and validated dependencies.
- Reconciliation updates only untouched open issues, creates newly required issues, preserves in-progress and completed issues, and marks removed untouched issues `heracles:obsolete`.
- All Agent Roles use the same capability-aware provider/model/effort/variant/profile preference model. Supported direct adapters are Codex, Claude Code, OpenCode, Kimi Code, OpenClaw, and Hermes.
- Preference precedence is launch override, project Preference, global Preference, then portable Project Configuration.
- Canonical configuration uses `heracles config` for project Preferences and `heracles config -g` for global Preferences. It supports direct dotted key/value pairs plus show, unset, append, path, confirmation, and atomic initialization.
- Launch commands accept equivalent dashed flags and dotted key/value pairs. Conflicts and unsupported keys fail.
- `heracles init` uses an interactive arrow-key menu with numbered fallback and offers Fast Setup or Complete Setup first.
- Heracles does not authenticate providers. Setup and Doctor only detect and explain authentication state.
- Provider sessions receive temporary compatible skill and MCP configuration. Shipped skills are `grill-with-docs`, `to-prd-for-heracles`, and `to-issues-for-heracles`.
- `heracles skills` mirrors skills.sh-style provider detection, scoped installation, selection, and overwrite protection.
- Heracles uses verified provider-specific full-permission flags for every Agent Role. The first full-permission interactive Labor requires project acknowledgement.
- Doctor validates tracker access, repositories, labels, branches, workspaces, provider CLIs, authentication state, capabilities, bypasses, profiles, MCP, skills, verification commands, environment variables, auto-merge permissions, and CI. Every Labor runs mandatory non-mutating Doctor preflight.
- Doctor warnings do not block; blocking findings stop execution. `doctor --fix` performs only safe repairs and never authenticates providers, changes secrets, or performs destructive repository actions.
- One active Labor is allowed per project. Clearly stale locks are removed automatically from local process evidence; force unlock is unavailable.
- A Labor defaults to sequential issue execution. `labor.concurrency` permits independent issues in isolated workspaces while dependency-linked, repository-conflicting, uncertain, or conflict-key-sharing work is serialized.
- Ready Issues are refreshed after every completed batch. Reaching an issue limit is a successful resumable pause.
- Auto-merge is enabled by default. With auto-merge disabled, verified issues enter review state and resume reconciles merges.
- CI failures and requested changes trigger preserved-workspace correction cycles. The default is three cycles per issue; trusted users may explicitly request retry-until-pass.
- Cancellation, timeout, action-required, runner unavailability, and known infrastructure patterns are retried as infrastructure failures; test, build, lint, and unknown failures are treated as code failures.
- Providers receive Agent Profile allowlisted environment variables plus essential process variables. Verification receives per-repository allowlisted variables. Values come from the launch environment, are never persisted in configuration, and are redacted from output and history.
- Labor state, locks, provider sessions, logs, and execution history remain under `.heracles/`. GitHub is the Issue Tracker and delivery surface, not a Labor-state recovery backend.
- Lost or corrupt local Labor state is unrecoverable. A new Labor may reconcile the same approved PRD but cannot resume prior provider sessions or execution state.
- Heracles versions Project Configuration and local-state formats, backs up automatic compatible migrations, confirms breaking migrations, and never writes incompatible newer formats from an older binary.
- Heracles remains attached to its controlling terminal and does not daemonize, detect, recommend, create, or manage terminal multiplexers.
- The first interrupt stops at a durable boundary; a second terminates children while preserving latest state. Cancellation is confirmed, irreversible locally, and leaves GitHub work unchanged.
- Self-install supports user and system locations. Self-update uses GitHub releases, verifies checksums, and atomically replaces the executable. Cached update checks never interrupt active Labors or structured/quiet output.
- Supported release targets include native macOS, Linux, and Windows, with platform-specific capability validation and clear failures for unsupported provider combinations.

## Testing Decisions

- Tests assert externally visible behavior through the highest practical seam rather than internal implementation details.
- CLI integration tests cover initialization, configuration precedence and validation, launch syntax, mandatory Doctor preflight, status, interruption, cancellation, installation, update checks, and structured output.
- Provider adapter contract tests cover command construction, supported settings, permission bypasses, authentication reporting, session resume capability, injected skills/MCP, process control, and platform-specific behavior without paid provider turns.
- Issue Tracker integration tests cover PRD revision identity, semantic IDs, dependency substitution, idempotent generation, changed-PRD reconciliation, HITL behavior, and newly unblocked Ready Issue refresh.
- Labor integration tests cover one-project locking, local resume, lost-state behavior, limits, concurrency/conflict serialization, auto-merge defaults, review mode, correction retries, CI classification, and Change Set completion.
- Security tests cover environment allowlists, missing required variables, redaction, and absence of persisted secret values.
- Migration tests cover compatible automatic migration, backups, breaking confirmation, newer-format rejection, and compatible Labor resume across Heracles versions.
- Cross-platform CI builds and tests native macOS, Linux, and Windows release targets and verifies release checksums.
- The release acceptance test is a complete dogfood Labor that initializes/configures Heracles, conducts and approves planning, generates/reconciles issues, implements/reviews/verifies/delivers them, resumes interruption, handles HITL blocking, and produces complete documentation and release binaries.

## Out of Scope

- Provider authentication or credential storage.
- OpenClaw or Hermes runtime/orchestration integration beyond invoking their provider CLIs.
- Antigravity support until a reliable non-interactive interface is verified.
- A Heracles-managed daemon, detached Labor service, cron-based updater, or terminal-multiplexer management.
- Manual force unlocking of active or uncertain project locks.
- Reconstructing or overriding lost local Labor state from GitHub.
- Automatic destructive cleanup of GitHub issues, branches, or pull requests on cancellation.
- Persisting secret values in Preferences, Project Configuration, logs, or Execution History.

## Further Notes

- The initial release is incomplete until every item in the end-to-end dogfood release gate passes.
- Existing implementation that refreshes Ready Issues within the same run must be preserved.
- Existing legacy behavior that auto-merges verified Change Sets by default must be preserved.
- README documentation must include the complete CLI, provider setup, installation/update behavior, shipped skill installation through Heracles and skills.sh, MCP connection examples, and troubleshooting.
- Implementation of this PRD must be performed through Heracles itself after the PRD Issue is approved.
