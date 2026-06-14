# Workflow Contracts

## Issue Contract

An unattended issue is open, labeled `heracles:ready`, not labeled `heracles:hitl`, and has no unresolved full-URL dependencies under `## Blocked by`. Heracles claims it with `heracles:in-progress`, reports failures with `heracles:blocked`, and records delivery with `heracles:done`. `heracles:tdd-exempt` requires a written reason.

Issue proposals include:

- `## Type`: `AFK` or `HITL`
- `## User stories covered`
- `## What to build`
- `## Acceptance criteria`
- `## Blocked by`: full GitHub issue URLs or `None`
- `## Exclusive Scopes`: shared concurrency scopes or `None`

## Evidence Contract

Normal delivery requires ordered, artifact-linked Red and Green Evidence:

- Red Evidence records a real failing command before implementation.
- Green Evidence records a passing command after the smallest correct change.
- Both records include repository, command, exit code, output, timing, and artifact path.
- A TDD Exemption bypasses evidence only when it contains a reason.

The Reviewer receives the issue, approved PRD, changes, evidence, exemption, and verification context; checks correctness, YAGNI, and DRY; and verifies any corrective changes.

## Change Set Contract

One completed issue produces one Change Set with exactly one pull request per touched Target Repository. Pull request bodies link the issue and related pull requests and include review summary, QA steps, and evidence references. Auto-merge is enabled by default; when a repository does not permit it, that pull request falls back to review mode and `heracles resume` reconciles the merge later. When enabled, required CI passes before ordered merges; a partial failure leaves remaining pull requests open and blocks the issue.

Requested changes or a code-classified CI failure start a correction cycle in the preserved Issue Workspace, up to 3 attempts before the issue is left blocked for operator attention. Infrastructure-classified CI failures (e.g. runner outages) are retried without counting against this limit.

## Durable State Contract

SQLite under `.heracles/history.db` is authoritative local Execution History. JSONL logs and artifacts remain human-readable. Planning, issue publication, issue attempts, Change Sets, Approval Gates, and Labors persist before returning control so interruption can resume from a committed boundary.

A project-level lock under `.heracles/` ensures only one Labor mutates a project at a time; a concurrent `heracles run`/`labor`/`resume` fails immediately, and a stale lock from a killed process is detected and cleared automatically. State and preferences carry a schema version: a compatible version migrates automatically (with a backup); a breaking migration requires confirmation; a newer schema version is rejected by an older binary without being rewritten.
