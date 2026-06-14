# Make PRD approval trigger background issue generation

Approving a PRD Issue always launches the background Issue Author and publishes implementation issues. `heracles plan` runs the interactive Grilling Session through PRD Issue approval and stops after implementation issues are published. `heracles labor` follows the same path and then continues into the Implementation Stage. This keeps approval behavior consistent while preserving a useful planning-only entry point.

Standalone `heracles issues <prd-issue-url>` accepts one existing GitHub issue labeled `heracles:prd` and `heracles:approved`, generates and publishes its implementation issues in the background, then stops. It rejects unlabeled or unapproved inputs.
