# Representative Workflows

## Start From a Problem

```sh
heracles labor --id launch-v1 --problem "Deliver the approved v1 experience"
heracles inspect labor launch-v1
heracles approve planning launch-v1 --reason "Scope approved"
heracles approve issues launch-v1 --reason "Breakdown approved"
heracles inspect labor launch-v1
```

The first command stops at the Planning Approval Gate. Planning approval advances to the Issue publication gate. Issue approval publishes executable issues and starts the Implementation Stage. The Labor completes only after its defined backlog is empty.

## Enter At a Stage

```sh
heracles plan --id plan-1 --problem "Clarify the migration"
heracles approve planning plan-1

heracles issues --id issues-1 --prd .heracles/planning/plan-1/PRD.md
heracles approve issues issues-1

heracles run
```

The Implementation Stage also accepts the original agent-loop launch overrides:

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

Omit `--limit` to continue until all compatible Ready Issues are completed or the remaining backlog is genuinely blocked.

## Operate Interrupted Work

```sh
heracles list labors
heracles list issues
heracles list gates
heracles list evidence
heracles inspect issue attempt-1
heracles retry attempt-1
heracles resume launch-v1
```

Failed and blocked Issue Workspaces are preserved by default. Retry resumes the preserved issue attempt; resume advances a blocked or interrupted Labor from its latest durable boundary.

## Project Topologies

Validated examples live under `examples/`:

- `single-repository.yaml`: one repository is tracker and target.
- `monorepo.yaml`: one target worktree covers all applications and packages.
- `multiple-repositories.yaml`: sibling backend and frontend targets.
- `separate-issue-tracker.yaml`: a dedicated tracker with relative and absolute target paths.

Use `--config /path/to/heracles.yaml` to operate any topology from outside its directory tree.
