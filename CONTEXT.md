# Heracles

Heracles coordinates agent-driven software delivery from an understood problem to an emptied implementation backlog.

## Language

**Labor**:
An end-to-end delivery workflow that moves through planning, issue creation, and implementation until its defined backlog is empty.
_Avoid_: Full run, pipeline

**Planning Stage**:
The part of a Labor where a Planner conducts a Grilling Session, clarifies the problem, and produces the necessary product and domain documentation.
_Avoid_: Discovery run

**Issue Stage**:
The background part of a Labor where an approved PRD Issue is converted into published implementation-ready issues.
_Avoid_: Ticket generation

**Implementation Stage**:
The part of a Labor where ready issues are implemented, reviewed, and resolved.
_Avoid_: Agent queue

**Approval Gate**:
A required human decision that permits a Labor to proceed at a defined controlled transition without requiring the active Grilling Session to end.
_Avoid_: Checkpoint

**Target Repository**:
A Git repository that a Labor is permitted to inspect and change.
_Avoid_: Code repo

**Issue Workspace**:
An isolated working area containing one temporary worktree for each target repository involved in an issue.
_Avoid_: Issue branch

**Change Set**:
The linked pull requests produced across one or more target repositories to deliver a single issue.
_Avoid_: Cross-repository PR

**Ready Issue**:
An implementation-ready issue that Heracles may claim and execute without human interaction.
_Avoid_: Actionable issue, queued issue

**Defined Backlog**:
The implementation issues linked to one PRD Issue that a Labor is responsible for completing.
_Avoid_: All ready issues, tracker backlog

**Claimed Issue**:
A Ready Issue reserved by one active Labor to prevent concurrent execution.
_Avoid_: In-progress ticket

**Review Issue**:
An issue whose verified Change Set awaits human pull-request review or merge before it can be completed.
_Avoid_: Done issue, blocked issue

**Blocked Issue**:
An issue that cannot proceed because a dependency, failed execution, or required decision remains unresolved.
_Avoid_: Failed issue

**Obsolete Issue**:
An untouched implementation issue that no longer belongs to the current approved revision of its Parent PRD and must not be executed.
_Avoid_: Deleted issue, cancelled issue

**Blocked Labor**:
A Labor that cannot advance without exceptional clarification or operator action.
_Avoid_: Paused Labor

**HITL Issue**:
An issue that requires human interaction and must not be executed unattended.
_Avoid_: Manual issue

**Issue Tracker**:
The GitHub repository whose issues define and track work for a Labor. It may be separate from every target repository.
_Avoid_: Planning repo, backlog repo

**PRD Issue**:
A GitHub issue published by the Planner from the Grilling Session, containing the product requirements that govern the subsequent Issue Stage. It keeps one identity across revisions and exposes whether it is under review or approved.
_Avoid_: Local PRD, planning artifact

**Planner**:
The agent responsible for clarifying a problem, lazily maintaining necessary domain documentation, and producing a product requirements document.
_Avoid_: Main agent

**Issue Author**:
The agent responsible for converting an approved PRD Issue into Heracles-compatible tracer-bullet issues without requiring an interactive review session.
_Avoid_: Ticket writer

**Question Budget**:
The expected maximum number of questions for a Grilling Session. It guides the Planner toward focus but may be exceeded with explicit user permission.
_Avoid_: Question limit

**Grilling Session**:
A focused Planning Stage interview in which the Planner challenges assumptions and resolves ambiguity with the user before producing a product requirements document.
_Avoid_: Grill me session, questionnaire

**Agent Profile**:
A reusable configuration describing which agent CLI to invoke and the supported runtime settings to apply.
_Avoid_: Provider configuration

**Agent Role**:
A responsibility within a Labor assigned an Agent Profile: Planner, Issue Author, Implementer, or Reviewer.
_Avoid_: Agent type

**Preference**:
A user-selected default for an Agent Role or Labor behavior that overrides portable Project Configuration without changing it.
_Avoid_: Project setting, profile

**Red Evidence**:
A recorded verification command and failing result that demonstrates the intended behavior was not satisfied before implementation.
_Avoid_: Failing test log

**Green Evidence**:
A recorded verification command and passing result that demonstrates the implemented behavior is satisfied.
_Avoid_: Passing test log

**TDD Exemption**:
An explicit, reasoned exception allowing an issue to proceed without Red Evidence and Green Evidence.
_Avoid_: Skip tests

**Control Surface**:
An interface through which a user or client operates Heracles, such as the CLI, MCP server, or a future desktop application.
_Avoid_: Frontend

**Exclusive Scope**:
A declared area of a target repository that must not be changed by concurrent issues.
_Avoid_: File lock

**Execution History**:
The durable local record of Labors, stage transitions, issue attempts, Approval Gates, Change Sets, logs, and evidence.
_Avoid_: Run logs

**Project Configuration**:
The portable declaration of an Issue Tracker, Target Repositories, Agent Profiles, and Labor policies used by Heracles.
_Avoid_: Runner config
