# Offer fast and complete interactive initialization

Bare interactive `heracles init` will first present an arrow-key menu, with a numbered-input fallback, asking the user to choose Fast Setup or Complete Setup. Fast Setup asks only for provider, model, and effort or variant choices. Complete Setup additionally asks about tracker and repository topology, Planning Question Budget, issue limit and concurrency, automatic merging, and Issue Workspace preservation. Complete Setup asks for a provider for every Agent Role, but after selecting the Implementer provider it offers to reuse that provider for all roles.

Like the Hermes setup flow, prompts are grouped into clear sections, display current values as defaults, and safely support reconfiguring an existing project. Shared topology is written to portable Project Configuration while personal runtime and behavior choices are written to project Preferences. Non-interactive initialization remains available through explicit flags and deterministic defaults.

When `heracles init` discovers an existing project, its first menu offers Fast Reconfigure, Complete Reconfigure, Repair Missing Values, or Cancel. Repair asks only for incomplete or invalid settings. Heracles never silently overwrites existing Project Configuration or Preferences.

Setup first discovers installed Agent Provider CLIs and marks unavailable providers. For providers with a reliable model-list command, it queries and presents available models; otherwise it offers curated known models plus a custom model ID. It presents only effort or variant values supported by the selected provider and validates every choice immediately without launching a paid agent turn.

Heracles never collects provider credentials or launches provider authentication flows. Setup and `heracles doctor` may report that a selected provider is unauthenticated and show actionable guidance, but the user authenticates independently through that provider.

Complete Setup asks for each Target Repository's verification commands and the names of environment variables those commands require. Only the variable-name allowlist is stored in shared Project Configuration. Heracles reads values from the launching environment, reports missing required variables, and never persists their values.

Fast Setup detects likely verification commands from each Target Repository's frameworks, manifests, and technology stack, presents them for confirmation, and writes confirmed commands to Project Configuration. When detection is ambiguous, Heracles leaves verification unset with a prominent warning and lets `heracles doctor` report the gap rather than guessing.

Heracles prefers an established aggregate quality command such as `make check`, `npm run check`, or `just check`; otherwise it detects existing non-mutating format-check, lint or static-analysis, test, and build commands. When a Target Repository has no established verification commands, initialization publishes a dedicated approved **Heracles Project Bootstrap** PRD Issue and creates implementation issues in its Defined Backlog to establish those quality gates, rather than inventing unverified commands or leaving the gap as a warning.

At the end of initialization, Heracles asks whether to run the Project Bootstrap Defined Backlog immediately or publish it for later, defaulting to immediate implementation. Setup is not considered fully ready until the bootstrap quality gates exist.

Each bootstrap issue updates shared Project Configuration in the same Change Set that creates its quality commands. Heracles verifies the newly configured commands before merging, reloads the merged `heracles.yaml`, and finishes initialization by running `heracles doctor`.
