---
name: to-prd-for-heracles
description: Drafts and publishes a durable PRD Issue from a clarified Heracles Grilling Session, embedding a revision marker and managing heracles:prd, heracles:review, and heracles:approved labels. Use after grill-with-docs to publish or revise the PRD.
---

# To PRD For Heracles

## Quick Start

Using the clarified understanding from the Grilling Session, draft a PRD
covering the problem, target users, scope, out-of-scope items, and success
criteria. Keep it tracer-bullet sized: the smallest PRD that lets Heracles
generate an executable, independently valuable Defined Backlog.

## Publishing the PRD Issue

- Publish the PRD as **one** Issue in the configured Issue Tracker, labeled
  `heracles:prd` and `heracles:review`.
- Embed a SHA-256 revision marker (`<!-- heracles:prd-revision:<hash> -->`)
  at the end of the Issue body, computed over the body content excluding the
  marker itself.
- The PRD Issue keeps **one durable URL** across revisions: edit the same
  Issue rather than creating a new one. Recompute and replace the revision
  marker on every edit.

## Handoff to Heracles

After publishing or revising the PRD Issue, run:

```
heracles plan --id <session-id> --prd-issue <issue-url> --prd <local-path-to-prd.md>
```

using the current Planning Stage ID and a local Markdown file containing the
exact PRD body you published (without the revision marker comment), so
Heracles can hand the approved PRD to the Issue Author.

## Approval Protocol

- If the user approves inside this session, edit the PRD Issue's labels to
  remove `heracles:review` and add `heracles:approved`, then run:
  `heracles approve planning <session-id>`. This records the Planning
  Approval Gate decision and starts background issue generation.
- If the user approves through another Control Surface, wait until you
  observe `heracles:approved` on the PRD Issue, then run the same
  `heracles approve planning` command.
- If the user requests changes, revise the same PRD Issue (recomputing the
  revision marker) and continue the session.
