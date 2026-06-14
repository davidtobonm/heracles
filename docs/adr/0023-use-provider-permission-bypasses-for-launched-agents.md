# Use provider permission bypasses for launched agents

Heracles-launched Planner, Issue Author, Implementer, and Reviewer sessions will use each Agent Provider's verified full-permission bypass, such as Claude Code's `--dangerously-skip-permissions`, so interactive and background roles can inspect repositories, update documentation, use Heracles MCP tools, and complete delivery without repeated provider-native approval prompts. Provider adapters must encode and test the exact bypass contract, and `heracles doctor` rejects a provider or mode whose required non-interactive or permission-bypass behavior has not been verified.

Before the first full-permission interactive Labor in a project, Heracles shows a concise warning and requires acknowledgment, then remembers acceptance in project Preferences. Users may require confirmation on every launch through `safety.confirm-full-permissions` or bypass prompts explicitly with `--yes` for automation.
