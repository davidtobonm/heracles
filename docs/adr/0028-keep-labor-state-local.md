# Keep Labor state local

Heracles stores durable project-local state under `.heracles/`. Runtime state,
including provider session IDs, logs, workspaces, locks, and resumable execution
data, is ignored by Git and must not contain persisted secret values. A small,
non-secret Labor manifest may be committed when it provides useful project
context.

GitHub remains the Issue Tracker and delivery surface for PRD Issues,
implementation issues, pull requests, CI, reviews, and merges. It is not a
Heracles state or recovery backend, does not override local Labor state, and
cannot be used to reconstruct a lost `.heracles/` Labor.

If local Labor state is lost or corrupted, Heracles reports that Labor as
unrecoverable and leaves all GitHub issues and pull requests unchanged. The
user may start a new Labor against the same approved PRD. That new Labor
reconciles existing implementation work, but it does not resume prior provider
sessions or execution state.
