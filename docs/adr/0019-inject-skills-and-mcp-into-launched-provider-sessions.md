# Inject skills and MCP into launched provider sessions

Every `heracles labor` launch will prepare a temporary provider-compatible session workspace containing Heracles's bundled `grill-with-docs`, `to-prd-for-heracles`, and `to-issues-for-heracles` skills plus MCP configuration for `heracles mcp serve --config ...`. The selected provider receives these capabilities automatically, so running a Labor never depends on the user manually installing skills or configuring MCP. Heracles does not ship the generic `grill-me` skill because `grill-with-docs` defines the intended Planning behavior. The bundled skills remain skills.sh-compatible for users who want them available outside Heracles-launched sessions.

Heracles also exposes `heracles skills list` and `heracles skills install --project|--global` to install its bundled skills through supported provider conventions. The README documents equivalent skills.sh commands. Neither installation method is required for Heracles-launched Labors.

Skill installation mirrors skills.sh ergonomics: Heracles detects compatible installed providers, targets the sole provider automatically, or shows a checklist with all detected providers selected when several exist. Repeatable `--provider` flags allow explicit targets. Unsupported provider conventions fail clearly, and locally modified skills are never overwritten without confirmation.
