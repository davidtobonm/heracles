# Heracles

Heracles coordinates agent-driven software delivery from an understood problem to an emptied implementation backlog.

## Language

**Labor**:
An end-to-end delivery workflow that moves through planning, issue creation, and implementation until its defined backlog is empty.
_Avoid_: Full run, pipeline

**Planning Stage**:
The part of a Labor where an agent clarifies the problem and produces the necessary product and domain documentation.
_Avoid_: Discovery run

**Issue Stage**:
The part of a Labor where an approved product requirements document is converted into implementation-ready issues.
_Avoid_: Ticket generation

**Implementation Stage**:
The part of a Labor where ready issues are implemented, reviewed, and resolved.
_Avoid_: Agent queue

**Approval Gate**:
A required human decision that permits a Labor to proceed from one stage to the next.
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

**Claimed Issue**:
A Ready Issue reserved by one active Labor to prevent concurrent execution.
_Avoid_: In-progress ticket

**Blocked Issue**:
An issue that cannot proceed because a dependency, failed execution, or required decision remains unresolved.
_Avoid_: Failed issue

**HITL Issue**:
An issue that requires human interaction and must not be executed unattended.
_Avoid_: Manual issue

**Issue Tracker**:
The GitHub repository whose issues define and track work for a Labor. It may be separate from every target repository.
_Avoid_: Planning repo, backlog repo

**Planner**:
The agent responsible for clarifying a problem, lazily maintaining necessary domain documentation, and producing a product requirements document.
_Avoid_: Main agent

**Issue Author**:
The agent responsible for converting an approved product requirements document into Heracles-compatible tracer-bullet issues.
_Avoid_: Ticket writer

**Question Budget**:
The expected maximum number of questions for a Planning Stage. It guides the Planner toward focus but may be exceeded with explicit user permission.
_Avoid_: Question limit

**Agent Profile**:
A reusable configuration describing which agent CLI to invoke and the supported runtime settings to apply.
_Avoid_: Provider configuration

**Agent Role**:
A responsibility within a Labor assigned an Agent Profile: Planner, Issue Author, Implementer, or Reviewer.
_Avoid_: Agent type

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
