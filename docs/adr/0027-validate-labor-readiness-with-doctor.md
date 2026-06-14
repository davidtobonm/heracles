# Validate Labor readiness with doctor

`heracles doctor` will validate the conditions required to complete a Labor
without predictable setup failures. It checks repository and Issue Tracker
access, required labels and branches, workspaces, provider CLI installation and
authentication status, provider capabilities and permission-bypass flags, role
profiles and Preferences, MCP configuration with a smoke test, injected skills,
verification commands and required environment variables, auto-merge
permissions, and CI availability.

Doctor reports optional deficiencies as warnings and exits unsuccessfully for
blocking deficiencies. `heracles doctor --json` provides machine-readable
results.

`heracles doctor --fix` may perform safe repairs, including creating required
labels, installing shipped skills, and repairing missing local configuration.
It must not authenticate providers, change secrets, or make destructive
repository changes.

Every `heracles labor` invocation runs a fast, non-mutating Doctor preflight
before starting or resuming work. Blocking findings stop the Labor and warnings
are reported. This preflight cannot be skipped.
