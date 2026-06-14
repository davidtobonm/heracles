# Keep Labor attached through delivery

`heracles labor` will remain attached across the complete workflow. After the user approves the PRD Issue inside the interactive Planner session, the Planner exits, Heracles launches the Issue Author in the background, publishes implementation issues, and begins implementing newly eligible Ready Issues in the same process. The command streams concise human-readable stage transitions, issue outcomes, pull request links, limit progress, and a final summary until the Labor completes, reaches its issue limit, becomes blocked, or is interrupted. `--verbose` additionally displays provider and verification output, `--quiet` displays only the final summary, and `--json` emits newline-delimited structured events. Full output and evidence remain persisted regardless of display mode, and interrupted work remains resumable.

Background Issue Author, Implementer, and Reviewer output is normalized into status and result events by default. Under `--verbose`, provider output is prefixed with the semantic issue ID so concurrent work remains readable, and Heracles never represents provider output as private reasoning. Raw provider streams remain persisted as evidence.

On the first `Ctrl-C`, Heracles stops at the next durable boundary, preserves active Issue Workspaces and provider session identities, and marks the Labor interrupted and resumable. A second `Ctrl-C` immediately terminates child providers while preserving the latest durable state. Explicit `heracles cancel <labor-id>` remains a distinct irreversible workflow cancellation.

`heracles cancel <labor-id>` requires explicit confirmation, stops active
providers, marks the local Labor state cancelled, and cleans disposable
workspaces. It leaves GitHub issues, branches, and pull requests unchanged. A
cancelled Labor cannot resume, but a new Labor may reconcile work for the same
approved PRD.

Heracles does not daemonize Labors or provide its own detached execution mode.
A Labor remains attached to its controlling terminal. Users who need a Labor to
survive a terminal disconnect run Heracles inside an external terminal
multiplexer such as `tmux` or `screen`. Heracles does not detect, recommend,
create, or manage terminal multiplexer sessions.
