# Persist local Execution History in SQLite

Heracles will persist local Execution History in SQLite under `.heracles/`, with human-readable JSONL logs and evidence artifacts stored alongside it. GitHub labels and comments remain the shared remote backlog state, while SQLite is authoritative for local execution and resume behavior. Persistence will sit behind an internal repository boundary for reuse by every Control Surface.
