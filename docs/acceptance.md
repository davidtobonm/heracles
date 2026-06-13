# End-to-End Acceptance Scenario

Heracles's deterministic acceptance scenario proves that a Labor respects both human decisions and completes only after its defined backlog is empty.

Run:

```sh
go test ./internal/labor -run TestLaborRunsThreeStagesWithDistinctApprovalGates -v
go test ./internal/labor -run TestLaborInterruptionResumesWithoutRepeatingCommittedStages -v
```

The scenario:

1. Starts a Labor from a problem description.
2. Runs Planning and stops at the PRD Approval Gate.
3. Approves the PRD and stops at the distinct issue-publication Approval Gate.
4. Approves publication and runs the Implementation Stage.
5. Receives an exhausted defined backlog and only then marks the Labor completed.

The interruption scenario additionally proves a committed stage is not repeated and a blocked implementation can resume. These tests use deterministic fakes and temporary persistence; they never invoke paid agent CLIs or authenticated GitHub mutations. `make check` runs them with the race detector alongside integration tests, while CI separately verifies every supported release target.
