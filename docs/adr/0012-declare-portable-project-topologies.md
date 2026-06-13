# Declare portable project topologies

Heracles projects will use a portable `heracles.yaml` discovered by searching upward from the current directory or selected explicitly with `--config`. `heracles init` defaults to the containing Git repository as both Issue Tracker and Target Repository, using its GitHub origin; outside a Git repository it fails with guidance to provide `--tracker` and one or more repeatable `--repo` arguments. Explicit repository paths may be relative to the configuration file or absolute, supporting monorepos, sibling repositories, and unrelated locations.
