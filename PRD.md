# PRD: Build Heracles, a Portable Agent-Driven Delivery Orchestrator

## Problem Statement

The existing agent queue proves that coding agents can systematically implement and review a backlog of GitHub issues across two repositories. However, it is a workspace-specific JavaScript script with hard-coded repository names, direct branch switching in live working copies, embedded provider behavior, GitHub operations coupled to orchestration, and automatic merge behavior that cannot be safely reused across projects.

Users need a portable tool that can enter a development workflow after issues are ready or earlier, when a problem still needs to be clarified, documented, converted into a PRD, and decomposed into implementation-ready issues. The tool must work with a single repository, a monorepo, sibling repositories, or unrelated repositories anywhere on disk. It must allow different agent CLIs and supported model settings for each responsibility, preserve human approval over planning decisions, enforce an auditable red/green TDD workflow, and expose the same controls through a CLI and MCP server without consuming paid agent credits in CI.

## Solution

Build Heracles as a portable Go application that coordinates agent-driven software delivery from an understood problem to an emptied implementation backlog.

Heracles exposes independently runnable Planning, Issue, and Implementation Stages. A Labor composes those stages into an end-to-end workflow, pausing at Approval Gates before publishing a PRD-derived issue plan and before publishing issues. Users may also enter directly at the Implementation Stage when Heracles-compatible GitHub issues already exist.

A portable Project Configuration declares the Issue Tracker, any number of Target Repositories, verification commands, merge policies, Labor policies, Agent Profiles, and Agent Role assignments. Each issue runs in an isolated Issue Workspace containing temporary Git worktrees, so Heracles never switches branches or requires clean working trees in the user's original repositories.

Heracles uses GitHub Issues as its v1 backlog and coordination surface. Ready Issues are claimed before execution, dependencies may cross repository boundaries, and each completed issue produces a Change Set containing one linked pull request per touched Target Repository. Automatic merging is opt-in.

The Implementer must produce Red Evidence and Green Evidence unless the issue has a reasoned TDD Exemption. The Reviewer verifies the result against the issue and PRD, checks the evidence, and reviews for correctness, YAGNI, and DRY before delivery.

The CLI, stdio MCP server, and future desktop application use the same application core. Durable Execution History is stored locally in SQLite under `.heracles/`, alongside human-readable JSONL logs and evidence artifacts.

## User Stories

1. As a developer, I want to install Heracles as a single Go binary, so that I can use it across projects without copying a workspace-specific script.
2. As a developer, I want clear installation instructions for supported platforms, so that I can start using Heracles quickly.
3. As a developer, I want `heracles init` to detect the Git repository I am currently inside, so that single-repository setup requires minimal input.
4. As a developer, I want the detected repository's GitHub origin to become the default Issue Tracker and Target Repository, so that common setup is automatic.
5. As a developer outside a Git repository, I want `heracles init` to explain that repositories must be provided, so that setup failures are actionable.
6. As a developer, I want to pass a tracker and repeated repository parameters to `heracles init`, so that I can configure sibling or unrelated repositories in one command.
7. As a developer, I want repository paths to be relative to the Project Configuration or absolute, so that the configuration remains portable while supporting unusual layouts.
8. As a developer, I want Heracles to support one Target Repository, so that it works for ordinary projects.
9. As a developer, I want Heracles to support a monorepo, so that one repository can contain all affected applications and packages.
10. As a developer, I want Heracles to support multiple Target Repositories, so that one issue can change a backend, frontend, shared library, or other related repositories.
11. As a developer, I want the Issue Tracker to be separate from all Target Repositories, so that planning can live in a dedicated GitHub repository.
12. As a developer, I want Heracles to discover `heracles.yaml` by searching upward, so that commands work naturally from nested project directories.
13. As a developer, I want to select a Project Configuration explicitly, so that I can operate a project from outside its directory tree.
14. As a maintainer, I want `heracles doctor` to validate configuration, repository access, GitHub authentication, agent CLIs, and supported profile settings, so that failures happen before a Labor starts.
15. As a maintainer, I want unsupported agent model or effort settings to fail clearly, so that Heracles never silently runs with unintended defaults.
16. As a maintainer, I want each Agent Role to select an Agent Profile, so that Planner, Issue Author, Implementer, and Reviewer can use different CLIs and models.
17. As a maintainer, I want Agent Profiles to inherit from a default profile, so that configuration does not repeat common settings.
18. As a maintainer, I want Agent Profiles to configure supported model and effort or variant settings, so that I can control quality and cost.
19. As a maintainer, I want Agent Profiles to configure timeouts, extra arguments, and environment-variable allowlists, so that CLI execution is controlled and project-specific.
20. As a maintainer, I want Codex, Claude Code, OpenCode, and Kimi Code adapters, so that I can use the agent subscriptions and runtimes available to me.
21. As a maintainer, I want Antigravity support only after its non-interactive interface is verified, so that Heracles does not advertise an unreliable adapter.
22. As a product owner, I want to start a Planning Stage from a problem description, so that Heracles can help clarify work before implementation issues exist.
23. As a product owner, I want the Planner to explore all Target Repositories and existing documentation, so that plans reflect the actual system.
24. As a product owner, I want the Planner to use a soft Question Budget of 20, so that clarification remains focused.
25. As a product owner, I want the Planner to finish before using its full Question Budget when understanding is sufficient, so that planning does not become ceremonial.
26. As a product owner, I want the Planner to request permission before exceeding its Question Budget, so that extended interviews are intentional.
27. As a maintainer, I want the Planner to create or update glossary, architecture, data-model, entity, business-term, stack, and decision documentation only when needed, so that documentation stays useful without becoming busywork.
28. As a product owner, I want the Planner to produce a PRD and pause at an Approval Gate, so that I retain control over product scope.
29. As a product owner, I want a separate Issue Author to convert an approved PRD into tracer-bullet issues, so that issue decomposition receives focused attention.
30. As a product owner, I want to review the proposed issue breakdown before publication, so that granularity and dependencies can be corrected.
31. As a product owner, I want the Issue Author to publish Heracles-compatible labeled GitHub issues after approval, so that the Implementation Stage can consume them directly.
32. As a developer, I want to run the Planning, Issue, and Implementation Stages independently, so that Heracles can enter at the appropriate point in my workflow.
33. As a developer, I want a Labor to compose all stages into one workflow, so that planning and implementation are connected rather than separate manual processes.
34. As a developer, I want a Labor to pause at Approval Gates, so that end-to-end orchestration does not remove meaningful human decisions.
35. As a maintainer, I want Ready Issues identified by explicit Heracles labels, so that queue eligibility is unambiguous.
36. As a maintainer, I want HITL Issues excluded from unattended execution, so that human-dependent work is not attempted blindly.
37. As a maintainer, I want Heracles to claim a Ready Issue before execution, so that concurrent Labors do not work on the same issue.
38. As a maintainer, I want issue dependencies expressed with full GitHub issue URLs, so that dependencies can cross Issue Trackers and repositories.
39. As a maintainer, I want blocked dependencies to prevent issue execution, so that work starts only when prerequisites are resolved.
40. As a maintainer, I want Heracles to process one issue at a time by default, so that delivery is predictable and subscription usage is controlled.
41. As a maintainer, I want configurable issue concurrency, so that independent work can be completed faster when appropriate.
42. As a maintainer, I want concurrent execution to respect dependencies, Exclusive Scopes, isolated workspaces, and profile limits, so that parallel work does not create avoidable conflicts.
43. As a developer, I want every issue to run in an isolated Issue Workspace, so that Heracles does not disturb my live working copies.
44. As a developer, I want Heracles to create one temporary worktree per Target Repository for an issue, so that cross-repository changes share one isolated working context.
45. As a developer, I want Heracles to preserve an Issue Workspace after a blocked or failed attempt, so that work and evidence can be inspected or resumed.
46. As a developer, I want successful Issue Workspaces cleaned according to policy, so that temporary state does not accumulate indefinitely.
47. As a maintainer, I want the Implementer to record Red Evidence before implementation, so that TDD begins with an observed failing behavior.
48. As a maintainer, I want the Implementer to make the smallest correct change and record Green Evidence, so that implementation remains focused.
49. As a maintainer, I want issues without Red and Green Evidence to be blocked by default, so that TDD expectations are enforceable.
50. As a maintainer, I want a reasoned TDD Exemption for work where red/green evidence is genuinely unsuitable, so that the policy remains practical.
51. As a maintainer, I want the Reviewer to inspect the issue, PRD, changes, verification, and TDD evidence, so that review covers the complete delivery contract.
52. As a maintainer, I want the Reviewer to correct problems directly when appropriate, so that minor review findings do not require a separate manual cycle.
53. As a maintainer, I want the Reviewer to check for unnecessary scope, premature abstractions, and meaningful duplication, so that YAGNI and DRY shape delivered code.
54. As a maintainer, I want configured verification commands to run for each touched Target Repository, so that repository-specific quality gates are respected.
55. As a maintainer, I want Heracles to produce one pull request per touched Target Repository, so that cross-repository work fits GitHub's repository boundaries.
56. As a maintainer, I want related pull requests linked as a Change Set, so that one issue's complete delivery remains understandable.
57. As a maintainer, I want Change Sets to include review summaries, QA steps, and evidence references, so that pull requests are useful to human reviewers.
58. As a maintainer, I want automatic merging disabled by default, so that adopting Heracles does not unexpectedly merge code.
59. As a maintainer, I want to opt into automatic merging with a configured repository order, so that trusted projects can burn through a backlog unattended.
60. As a maintainer, I want all configured local verification and CI checks to pass before automatic merging, so that delivery respects project gates.
61. As a maintainer, I want a partial cross-repository merge failure to leave remaining pull requests open and mark the issue blocked, so that Heracles does not pretend delivery was atomic.
62. As a maintainer, I want GitHub labels and comments to show shared issue status, so that collaborators can understand Heracles activity without local access.
63. As a maintainer, I want local Execution History to persist in SQLite, so that Labors can be inspected and resumed reliably.
64. As a maintainer, I want JSONL logs and evidence artifacts alongside SQLite, so that execution remains human-readable and easy to archive.
65. As a maintainer, I want interrupted Labors and issue attempts to resume from durable state, so that long-running work survives process restarts.
66. As a maintainer, I want the CLI to list and inspect Labors, issues, Change Sets, Approval Gates, logs, and evidence, so that I can operate Heracles from a terminal.
67. As an agent client, I want an MCP server with the same capabilities as the CLI, so that agents can operate Heracles without shell-specific orchestration.
68. As an agent client, I want MCP operations to be high-level and exclude arbitrary shell execution, so that the integration has a clear safety boundary.
69. As a maintainer, I want CLI and MCP operations to use the same application core, so that behavior does not drift between Control Surfaces.
70. As a future desktop-app user, I want Heracles state and operations exposed through reusable application services, so that a desktop interface can be added without rewriting orchestration.
71. As a skill user, I want a `to-issues-for-heracles` skill, so that an approved PRD can become Heracles-compatible issues.
72. As a skill user, I want to install `to-issues-for-heracles` globally or into a specific project using the conventions supported by skills.sh-compatible agents, so that the workflow is easy to adopt.
73. As a maintainer, I want the skill to create tracer-bullet issues with Heracles labels, full-URL dependencies, AFK or HITL classification, acceptance criteria, and Exclusive Scopes when needed, so that published issues are executable.
74. As a maintainer, I want clear README documentation for installation, initialization, configuration, commands, provider capabilities, labels, TDD evidence, MCP setup, and workflows, so that users can operate Heracles without reading its source.
75. As a contributor, I want a project-appropriate `.gitignore`, so that binaries, local state, worktrees, logs, databases, and secrets are not committed accidentally.
76. As a contributor, I want CI to run formatting, static analysis, unit tests, integration tests, and builds without invoking paid agent CLIs, so that repository validation does not consume subscription credits.
77. As a contributor, I want provider adapters tested with deterministic fake executables, so that CI validates command construction and parsing without external agent calls.
78. As a maintainer, I want release automation to build portable binaries, so that Heracles can be installed easily on supported systems.
79. As a maintainer, I want failures to produce actionable issue comments and preserved local evidence, so that blocked work can be diagnosed.
80. As a maintainer, I want Heracles to continue selecting newly unblocked Ready Issues until the defined backlog is empty, so that a Labor fulfills its purpose.

## Implementation Decisions

- Heracles will be implemented in Go and distributed as a portable command-line binary.
- The application will have one shared core used by every Control Surface. CLI commands, MCP tools, and a future desktop application will invoke the same application services.
- The primary commands will include `heracles init`, `heracles doctor`, `heracles plan`, `heracles issues`, `heracles run`, `heracles labor`, and `heracles mcp serve`.
- Planning, issue authoring, and implementation remain independently runnable stages. A Labor composes them and persists stage transitions.
- The Planning Stage uses a configurable Planner, defaults to a Question Budget of 20, may finish early, and requires user permission to exceed the budget.
- The Planning Stage lazily creates or updates only the domain and technical documentation needed to make the PRD and subsequent implementation unambiguous.
- The Issue Stage uses a separate configurable Issue Author and pauses before publishing its proposed issue set.
- The `to-issues-for-heracles` skill will be included in the repository using a skills.sh-compatible skill layout. It will support global or project installation and will generate tracer-bullet issues compatible with Heracles's GitHub contract.
- Heracles v1 supports GitHub Issues only. GitHub-specific operations sit behind an internal tracker boundary.
- Heracles-compatible issues use `heracles:ready`, `heracles:blocked`, `heracles:in-progress`, `heracles:done`, and `heracles:hitl`. Reasoned TDD exemptions use `heracles:tdd-exempt`.
- Issue dependencies are recorded under `## Blocked by` using full GitHub issue URLs.
- Issues may declare Exclusive Scopes used to prevent unsafe concurrent execution.
- `heracles init` detects the containing Git repository and its GitHub origin. When detected, that repository is the default Issue Tracker and Target Repository.
- Outside a Git repository, `heracles init` fails with actionable guidance unless the user provides a tracker and at least one repository.
- Initialization supports an explicit tracker and repeated repository parameters.
- Project Configuration is stored in portable `heracles.yaml`, discovered upward from the current directory or selected explicitly.
- Project Configuration declares the Issue Tracker, Target Repositories, base branches, verification commands, merge order, Agent Profiles, Agent Role assignments, concurrency, auto-merge policy, and relevant workspace policies.
- Target Repository paths may be absolute or relative to the Project Configuration, supporting single repositories, monorepos, sibling repositories, and unrelated repository locations.
- Agent Profiles support inheritance and may configure provider, model, effort or variant when supported, extra arguments, timeout, environment-variable allowlist, and concurrency limit.
- The initial provider adapters are Codex, Claude Code, OpenCode, and Kimi Code.
- Antigravity remains unsupported until a reliable non-interactive interface and capability contract can be verified.
- Provider adapters expose declared capabilities. `heracles doctor` rejects unsupported model, effort, or other profile settings instead of ignoring them.
- Provider adapters execute real CLIs only during user-initiated workflows, never during repository CI.
- Each issue receives an isolated Issue Workspace containing a temporary Git worktree for each Target Repository.
- Heracles does not switch branches or require clean working trees in the user's original repositories.
- Issue Workspace preservation and cleanup are policy-driven, with failed or blocked attempts preserved by default.
- One issue is processed at a time by default. Configurable concurrency is permitted only for issues with satisfied dependencies, non-overlapping Exclusive Scopes, isolated workspaces, and available Agent Profile capacity.
- Heracles claims a Ready Issue before execution by transitioning its GitHub labels to the claimed state.
- The Implementer must record Red Evidence before the implementation change and Green Evidence after the smallest correct implementation.
- Missing Red or Green Evidence blocks delivery unless the issue has a reasoned TDD Exemption.
- The Reviewer checks issue and PRD compliance, correctness, verification, Red and Green Evidence, YAGNI, and DRY. It may make and verify corrective changes.
- Each touched Target Repository is verified with its configured commands before delivery.
- One issue produces a Change Set containing one linked pull request for each touched Target Repository.
- Automatic merging is disabled by default. When enabled, all configured verification and CI requirements must pass before merging in configured order.
- Cross-repository delivery is explicitly non-atomic. A partial merge failure leaves remaining pull requests open and marks the issue blocked.
- SQLite under `.heracles/` stores authoritative local Execution History, including Labors, stages, issue attempts, Approval Gates, Change Sets, logs, and evidence metadata.
- Human-readable JSONL logs and evidence artifacts are stored alongside SQLite.
- Persistence sits behind an internal repository boundary and supports reliable inspection and resume behavior.
- The MCP server ships in the same binary through `heracles mcp serve`, initially communicates over stdio, and exposes the same high-level capabilities as the CLI without arbitrary shell execution.
- The repository will include clear README documentation, a `.gitignore`, an open-source license, contribution guidance, and example configurations.
- CI/CD will run formatting, vetting or linting, unit tests, integration tests, race-sensitive tests where practical, and cross-platform builds without invoking any real agent CLI.
- Release automation will publish versioned binaries for supported operating systems and architectures.

## Testing Decisions

- Good tests verify externally observable Heracles behavior and workflow rules rather than internal helper structure.
- CLI and MCP contract tests will exercise the same application services and assert equivalent outcomes for initialization, diagnostics, stage execution, Approval Gates, inspection, retry, cancel, and resume operations.
- Labor orchestration tests will use deterministic fake tracker, agent, Git, clock, and persistence adapters to verify state transitions, approval pauses, backlog exhaustion, blocking, retries, and resume behavior.
- Initialization tests will use temporary directories and repositories to verify containing-repository detection, GitHub-origin defaults, explicit trackers, repeated repository parameters, relative paths, absolute paths, upward configuration discovery, and actionable errors.
- Project Configuration tests will verify validation, defaults, profile inheritance, repository topology, merge order, concurrency policy, and rejection of unsupported capabilities.
- Provider adapter contract tests will use fake executable binaries or scripts to verify command construction, prompt delivery, output parsing, timeouts, environment allowlists, and capability validation without invoking real agent CLIs.
- GitHub tracker contract tests will run against deterministic fakes or recorded API-shaped fixtures, not a live backlog, and will verify labels, atomic claiming behavior, full-URL dependencies, comments, and issue-state transitions.
- Real Git integration tests will create temporary repositories and worktrees to verify Issue Workspace isolation, branch behavior, touched-repository detection, preservation, cleanup, and Change Set preparation without modifying developer repositories.
- TDD workflow tests will verify required Red Evidence, required Green Evidence, TDD Exemptions, evidence persistence, reviewer visibility, and blocked delivery when evidence is missing.
- Change Set tests will verify one pull request per touched Target Repository, linked metadata, verification gates, opt-in auto-merge, configured merge order, and partial merge failure behavior.
- Concurrency tests will verify sequential defaults, dependency checks, Exclusive Scope conflicts, workspace isolation, and Agent Profile capacity limits.
- SQLite integration tests will use temporary databases to verify durable Execution History, transactional state transitions, interruption recovery, retries, and resume behavior.
- JSONL and artifact tests will verify that logs and evidence remain human-readable and consistently linked to durable state.
- Skill tests will validate the `to-issues-for-heracles` skill structure and assert that generated issue proposals contain required labels, sections, full-URL dependencies, AFK or HITL classification, acceptance criteria, and Exclusive Scopes when applicable.
- CI workflow tests and checks will never invoke paid or authenticated agent CLIs. Provider behavior in CI is always simulated.
- Prior art from the existing agent queue includes tests for option merging, issue ordering, dependency filtering, provider invocation construction, provider output normalization, logging, branch resumption, touched-repository detection, verification behavior, and issue-closing safeguards. These behaviors should be preserved at higher application and adapter seams where they remain relevant.

## Out of Scope

- A desktop application in the first release. The shared application core and persistence model must support one later.
- Issue trackers other than GitHub Issues in v1.
- Guaranteed atomic merges across multiple Target Repositories.
- Automatic merge enabled by default.
- Arbitrary shell execution through the MCP server.
- Real agent CLI invocations in CI.
- Unlimited or automatic Planning Stage questioning beyond the Question Budget without user permission.
- Executing HITL Issues unattended.
- Supporting Antigravity before its non-interactive CLI behavior is verified.
- Proving an agent's private reasoning process. Heracles enforces auditable red/green evidence and review behavior instead.
- Replacing project-specific test suites, CI checks, or branch protection policies.

## Further Notes

- Heracles is named after the system's purpose: systematically completing a challenging defined backlog until the Labor is finished.
- The existing agent queue is behavioral prior art, not the target architecture. The Go implementation should preserve proven outcomes while removing workspace-specific assumptions and known coupling.
- The first implementation should favor small, deep modules around application orchestration, configuration, provider capabilities, GitHub tracking, Git workspaces, evidence, Change Sets, and persistence.
- The implementation should follow true red/green TDD, YAGNI, and DRY throughout its own development.
- The repository is `davidtobonm/heracles`.
