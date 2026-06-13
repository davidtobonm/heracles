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
