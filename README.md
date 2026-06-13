# Heracles

Heracles coordinates agent-driven software delivery from an understood problem to an emptied implementation backlog.

The project is under active development. The current binary foundation provides stable help and version contracts while the Planning, Issue, and Implementation Stages are built from the [product requirements](PRD.md).

## Requirements

- Go 1.24 or newer when building from source
- Git and GitHub CLI for the workflows that will coordinate repositories and issues

## Install From Source

```sh
git clone https://github.com/davidtobonm/heracles.git
cd heracles
make install
heracles --help
```

Versioned release binaries for Linux, macOS, and Windows are published from `v*` tags.

## Develop

```sh
make check
make build
./bin/heracles version
```

CI runs formatting, static analysis, race-enabled tests, and cross-platform builds. Tests use deterministic fake executables and never invoke paid or authenticated agent CLIs.

## Initialize A Project

From anywhere inside a Git repository with a GitHub `origin`:

```sh
heracles init
```

Heracles writes `heracles.yaml` at the repository root and uses that repository as both the Issue Tracker and first Target Repository.

For sibling or unrelated repositories, provide a separate tracker and repeat `--repo`:

```sh
heracles init \
  --tracker acme/delivery-backlog \
  --repo ../backend \
  --repo /absolute/path/to/frontend
```

Repository paths passed as relative paths are stored relative to `heracles.yaml`; absolute paths remain absolute. Project Configuration discovery searches upward from the current directory, and later commands can select a configuration explicitly with `--config`.

## Local Execution History

Heracles stores authoritative local workflow state in `.heracles/history.db`. Human-readable JSONL logs live under `.heracles/logs/`, and evidence artifacts live under `.heracles/artifacts/`. Reopening a project rebuilds JSONL mirrors from committed SQLite events, so interrupted workflows remain inspectable and resumable.

## Agent Profiles And Diagnostics

Agent Roles select reusable, inheritable profiles:

```yaml
agents:
  default_profile: default
  profiles:
    default:
      provider: codex
      model: gpt-5.4
      effort: high
      timeout: 45m
      env_allowlist: [PATH, HOME]
      concurrency: 1
    reviewer:
      extends: default
      provider: claude
      model: sonnet
  roles:
    reviewer: reviewer
```

Heracles supports Codex, Claude Code, OpenCode, and Kimi Code. Provider-specific model, effort, and variant settings are validated instead of silently ignored. Run `heracles doctor` before a Labor to validate the Project Configuration, Target Repositories, GitHub authentication, Agent Profiles, capabilities, and required executables. Diagnostics never invoke a paid agent session.
