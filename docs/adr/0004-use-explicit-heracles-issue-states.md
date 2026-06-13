# Use explicit Heracles issue states

Heracles-compatible issues will use the labels `heracles:ready`, `heracles:blocked`, `heracles:in-progress`, `heracles:done`, and `heracles:hitl`. A Labor must claim a Ready Issue by applying `heracles:in-progress` before execution, and dependencies will be recorded under `## Blocked by` using full issue URLs so they can cross repository boundaries.
