---
name: to-issues-for-heracles
description: Converts an approved PRD into tracer-bullet Heracles-compatible GitHub issue proposals. Use when decomposing approved product scope for Heracles or preparing an executable issue backlog.
---

# To Issues For Heracles

## Quick Start

Read the approved PRD and relevant repository context. Propose the smallest set of end-to-end tracer-bullet issues that can independently deliver observable value.

Pause for approval before publishing any issue.

## Proposal Contract

Every proposal must:

- classify as `AFK` or `HITL`
- identify covered user stories
- describe one end-to-end outcome under `## What to build`
- include testable bullets under `## Acceptance criteria`
- use full GitHub issue URLs under `## Blocked by`, or `- None`
- include `## Exclusive Scopes` when concurrent execution would be unsafe, or `- None`
- explain any TDD exemption; otherwise require Red and Green Evidence

Use `heracles:ready` for AFK work; Heracles resolves its dependencies before execution. Use `heracles:hitl` for human-dependent work. Reserve `heracles:blocked` for a failure or decision that requires attention.

## Required Issue Body

```md
## Type
AFK or HITL

## User stories covered
1, 2

## What to build
One tracer-bullet outcome.

## Acceptance criteria
- Observable result

## Blocked by
- Full GitHub issue URLs or None

## Exclusive Scopes
- Shared scope or None
```

## Publication Checklist

1. Verify every issue is independently understandable and executable.
2. Verify dependencies are minimal and use full GitHub issue URLs.
3. Verify unsafe concurrent work declares Exclusive Scopes.
4. Present the complete proposal set for approval.
5. Publish only after approval with the correct Heracles state labels.
