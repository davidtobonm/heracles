# Version and migrate configuration and local state

Heracles versions both `heracles.yaml` and `.heracles/` local-state formats.
Backward-compatible format changes are migrated automatically after Heracles
creates a backup.

Breaking migrations stop before Labor execution, show the planned migration,
and require explicit user confirmation. An older Heracles binary never
downgrades or writes a format created by a newer incompatible version.

Each Labor records the Heracles version that created it and the last version
that ran it. A newer binary may resume the Labor only when it declares the
Labor-state schema compatible. If compatibility is uncertain or breaking,
Heracles stops before execution and requires migration or completion with the
original compatible version.
