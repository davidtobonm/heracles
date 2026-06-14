---
name: grill-with-docs
description: Runs an interactive Grilling Session that explores Target Repositories and existing documentation, asking one clarifying question at a time within a Question Budget. Use at the start of Heracles Planning, before drafting a PRD.
---

# Grill With Docs

## Quick Start

Read the Problem statement, Target Repositories, and any existing
documentation provided in the session brief. Explore the repositories and
documents before asking questions; do not ask about anything the docs or
code already answer.

## Grilling Protocol

- Ask exactly **one** clarifying question at a time, then wait for the
  answer before asking the next.
- Prefer questions that resolve ambiguity blocking a tracer-bullet PRD:
  scope boundaries, target users, success criteria, constraints, and
  out-of-scope items.
- Track how many questions you have asked against the session's Question
  Budget.
- Skip questions whose answers are already evident from the Target
  Repositories or existing documentation — cite what you found instead of
  asking.

## Reaching the Question Budget

When you reach the Question Budget:

1. Stop asking new questions.
2. Summarize what remains uncertain.
3. Ask the user whether you may exceed the budget by a small number of
   questions, or should proceed directly to drafting the PRD with what you
   already know.

## Handoff

Once clarification is complete (or the user chooses to proceed), use the
`to-prd-for-heracles` skill to draft, publish, and revise the durable PRD
Issue. Do not draft the PRD yourself in this skill — Grilling produces the
clarified understanding that `to-prd-for-heracles` turns into a PRD.
