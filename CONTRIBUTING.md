# Contributing to Heracles

## Development Setup

Heracles requires Go 1.24 or newer, Git, and GitHub CLI. Clone the repository, then run:

```sh
make check
make build
./bin/heracles --help
```

## Change Expectations

- Keep changes scoped to one observable workflow outcome.
- Add or update tests before changing behavior.
- Preserve the shared application core; CLI and MCP must not duplicate orchestration.
- Never invoke real paid agent CLIs or authenticated GitHub mutations in tests.
- Use deterministic fakes for providers, GitHub, clocks, and interruption scenarios.
- Update README, examples, or reference docs when changing a user-visible contract.

## Pull Requests

Before opening a pull request:

```sh
make check
git diff --check
```

CI verifies formatting, `go vet`, race-enabled tests, integration tests, and CGO-free cross-platform builds. Pull requests should explain the user-visible outcome, verification performed, and any compatibility or migration concern.

## Releases

Maintainers create releases by pushing a `v*` tag. The release workflow reruns all quality gates, builds Linux, macOS, and Windows binaries, generates SHA-256 checksums, and publishes a GitHub release.

Heracles is available under the [MIT License](LICENSE).
