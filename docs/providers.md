# Provider Capabilities

Agent Roles select inherited Agent Profiles. Run `heracles doctor` before starting work; it validates executables and rejects unsupported settings without invoking a paid agent session.

| Provider | Executable | Model | Effort | Variant |
| --- | --- | --- | --- | --- |
| Codex | `codex` | yes | `low`, `medium`, `high`, `xhigh` | no |
| Claude Code | `claude` | yes | `low`, `medium`, `high`, `max` | no |
| OpenCode | `opencode` | yes | no | yes |
| Kimi Code | `kimi` | yes | no | no |
| OpenClaw | `openclaw` | yes | `low`, `medium`, `high` | no |
| Hermes | `hermes` | yes | no | yes |

Every profile may set `timeout`, `extra_args`, `env_allowlist`, and `concurrency`. A child profile inherits unspecified settings through `extends`; changing provider clears inherited provider-specific model, effort, and variant settings.

Every provider launches with its verified full-permission bypass flags so unattended Agent Roles never stop for interactive tool approval; see [Agent Profiles And Diagnostics](../README.md#agent-profiles-and-diagnostics) for the one-time disclosure shown before the first Labor that launches each provider.

Official CLI references:

- [Codex CLI](https://developers.openai.com/codex/cli/reference/)
- [Claude Code CLI](https://code.claude.com/docs/en/cli-reference)
- [OpenCode CLI](https://opencode.ai/docs/cli/)
- [Kimi CLI](https://moonshotai.github.io/kimi-cli/en/reference/kimi-command.html)
- OpenClaw CLI: consult `openclaw --help` and your distribution's documentation.
- Hermes CLI: consult `hermes --help` and your distribution's documentation.

Heracles runs providers only during user-initiated workflows. Repository CI tests adapter construction and output parsing with deterministic fakes.
