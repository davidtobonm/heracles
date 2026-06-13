# Provider Capabilities

Agent Roles select inherited Agent Profiles. Run `heracles doctor` before starting work; it validates executables and rejects unsupported settings without invoking a paid agent session.

| Provider | Executable | Model | Effort | Variant |
| --- | --- | --- | --- | --- |
| Codex | `codex` | yes | `low`, `medium`, `high`, `xhigh` | no |
| Claude Code | `claude` | yes | `low`, `medium`, `high`, `max` | no |
| OpenCode | `opencode` | yes | no | yes |
| Kimi Code | `kimi` | yes | no | no |

Every profile may set `timeout`, `extra_args`, `env_allowlist`, and `concurrency`. A child profile inherits unspecified settings through `extends`; changing provider clears inherited provider-specific model, effort, and variant settings.

Official CLI references:

- [Codex CLI](https://developers.openai.com/codex/cli/reference/)
- [Claude Code CLI](https://code.claude.com/docs/en/cli-reference)
- [OpenCode CLI](https://opencode.ai/docs/cli/)
- [Kimi CLI](https://moonshotai.github.io/kimi-cli/en/reference/kimi-command.html)

Heracles runs providers only during user-initiated workflows. Repository CI tests adapter construction and output parsing with deterministic fakes.
