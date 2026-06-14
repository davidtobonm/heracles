# Allowlist and redact process environments

Agent Provider processes receive only the selected Agent Profile's environment allowlist plus essential process variables such as `PATH`, `HOME`, and terminal variables. Verification commands use an explicit Target Repository environment policy. Heracles redacts values associated with allowed secret variables from displayed and persisted output and never stores secret values in Preferences, Project Configuration, logs, or Execution History.

Target Repositories declare verification environment variable names through `verify_env_allowlist` in Project Configuration. Heracles reads their current values from the launching environment and fails clearly when required variables are missing.
