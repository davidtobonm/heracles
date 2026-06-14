# Scope Labors to PRD-defined backlogs

Every generated implementation issue will include a `## Parent PRD` section with the full PRD Issue URL and carry `heracles:implementation`. A Labor selects only issues linked to its PRD Issue, making that set its Defined Backlog rather than consuming unrelated Ready Issues from a shared tracker. `heracles run <prd-issue-url>` processes that Defined Backlog; bare `heracles run` asks for confirmation before processing all eligible Ready Issues in the tracker.

Starting or resuming a Defined Backlog reconciles remote issue labels with durable local Execution History. Heracles skips `heracles:done` issues, resumes preserved in-progress attempts when safe, and processes the remaining ready or newly unblocked issues. Inconsistent remote and local state is reported for operator resolution rather than guessed.

Issue generation records the approved PRD Issue content revision used to create the Defined Backlog. Re-running issue generation against the same PRD revision performs no mutations. When the approved PRD changes, Heracles updates only still-open issues that are not in progress, creates newly required issues, and leaves in-progress or completed work unchanged.

When a changed PRD makes an untouched issue obsolete, Heracles leaves it open, removes `heracles:ready`, adds `heracles:obsolete`, and comments with the superseding PRD revision. It never automatically closes obsolete issues. In-progress and completed issues remain untouched and are reported.

A PRD revision is the SHA-256 hash of a canonicalized PRD Issue title and body, excluding comments and Heracles-managed labels. Heracles records the revision in local Execution History and embeds `<!-- heracles:prd-revision=<sha256> -->` in generated issues, making unchanged detection deterministic without relying on GitHub edit timestamps.

The Parent PRD URL identifies the Defined Backlog, while each generated issue carries an Issue Author-assigned semantic ID such as `auth-login-flow` in `<!-- heracles:issue-id=auth-login-flow -->`. Heracles reconciles issues by Parent PRD plus semantic ID, allowing titles to change without duplication. Missing or duplicate semantic IDs block publication.

Within an Issue Author proposal set, dependencies reference semantic issue IDs. Heracles validates that every referenced ID exists and that the dependency graph is acyclic, publishes dependencies first, and substitutes each created issue's full GitHub URL into dependent issues under `## Blocked by`. Dependencies on issues outside the proposal set continue to use full GitHub issue URLs.

HITL issues are excluded from unattended implementation while independent AFK work continues. A Labor becomes blocked only when unresolved HITL issues directly or transitively prevent remaining AFK issues from running. If only standalone HITL issues remain, Heracles completes the agent-deliverable Defined Backlog and reports those HITL issues separately.

Attached runs refresh tracker state between implementation batches, so AFK issues unblocked by a human resolving HITL work become eligible in the same invocation. When no AFK work is runnable and unresolved blocking HITL dependencies remain, Heracles marks the Labor blocked and exits rather than polling indefinitely.
