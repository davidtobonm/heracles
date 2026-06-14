# Require an end-to-end dogfood release gate

The initial Heracles release is complete only when the full dogfood Labor can:

1. Initialize and configure Heracles.
2. Run the interactive Planner and approve its PRD.
3. Generate and reconcile implementation issues.
4. Implement, review, verify, and deliver them through supported providers.
5. Resume interrupted work and handle HITL blockers.
6. Pass cross-platform CLI tests and produce release binaries.
7. Provide complete README, MCP, skills, and provider setup documentation.

These conditions form one release gate. Passing only individual stages or
shipping partially documented commands does not complete the initial release.
