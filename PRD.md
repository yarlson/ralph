## PRD: Ralph Wiggum Loop Harness in Go (Claude Code Only)

### 1) Summary

Build a **Go-based “Ralph Wiggum loop” harness** that orchestrates autonomous, iterative feature delivery by repeatedly selecting the next **ready** task, delegating implementation to **Claude Code (as the only coding agent)**, verifying outcomes (tests/typecheck/lint), committing changes, updating task status and lightweight memory logs, and repeating until the feature is complete.

The harness **does not “code.”** It is a deterministic **executor + verifier + state manager** for a continuous autonomy loop. Planning/PRDs/task creation can be done by other agents, but **coding execution is exclusively Claude Code**.

---

## 2) Problem Statement

Claude Code can implement meaningful changes but, when run in a single pass, often fails due to:

- context limits and drift,
- partial completion (stopping early),
- inconsistent verification discipline,
- lack of durable state/memory hygiene between iterations,
- insufficient guardrails (infinite loops, repetitive mistakes, unsafe actions).

We need a harness that:

1. **breaks work into small, verifiable tasks**,
2. **runs them one by one** with a fresh Claude Code context each time,
3. **anchors truth in repo state + tests**, not conversational memory,
4. **keeps the loop safe and observable**,
5. provides deterministic, auditable outcomes.

---

## 3) Target Users / Personas

1. **Backend/Platform Engineer (primary)**
   Wants autonomous execution for refactors/features with strict verification.

2. **Tech Lead (secondary)**
   Wants predictable progress, audit logs, and the ability to intervene.

3. **Agent Power User (secondary)**
   Wants budgets/stop conditions, and operational rigor.

---

## 4) Goals

### Product Goals

- **Deterministic progress**: each iteration results in either a verified commit or a clearly recorded failure with actionable feedback.
- **Externalized memory**: no reliance on Claude “remembering”; the harness persists state in files + task store + git history.
- **Safety and boundedness**: budgets, stop conditions, sandbox support, and rollback mechanisms.
- **Claude-only coding**: all code changes are produced by Claude Code; the harness standardizes prompts and verification.
- **Task dependency correctness**: picks only **leaf** tasks that are **ready**.

### Success Metrics

- % iterations that end in **verified commits** (vs. churn).
- Average iterations per completed feature.
- Mean time-to-recovery after a failure (rerun with feedback).
- Zero destructive incidents in sandboxed mode.

---

## 5) Non-Goals

- Writing code changes itself (the harness only orchestrates).
- Being a general-purpose AutoGPT-style planner. Planning can be delegated; harness may facilitate but not own intelligence.
- Solving subjective UX quality without verifiable criteria.
- Long-term semantic memory (vector DB) in MVP (optional later).

---

## 6) Key Concepts

### “Ralph Loop” Contract

Each iteration:

1. Load current feature scope (parent task + progress memory)
2. Find next ready **leaf** task
3. Delegate implementation to **Claude Code** (fresh context)
4. Verify (typecheck/tests/lint) and require fixes until pass (within iteration budget)
5. Commit and update task status + progress log
6. Repeat

### Memory Hygiene

- **Repo state** is source of truth.
- **Short-term memory**: `progress.md` (feature-specific patterns, gotchas).
- **Long-term memory**: `AGENTS.md` in relevant directories (only reusable guidance).

---

## 7) User Workflows

### Workflow A: New Feature (Tasks generated externally; harness consumes)

1. User invokes: `ralph plan` (optional helper) OR imports tasks created elsewhere.
2. Harness persists tasks in task store; writes `parent-task-id`.
3. User invokes: `ralph run`

### Workflow B: Existing Tasks (Just run)

1. User invokes: `ralph init --parent <id>` or `ralph init --search "<term>"`
2. Harness validates structure:
   - dependencies acyclic,
   - at least one ready leaf task exists.

3. User invokes: `ralph run`

### Workflow C: Execution Loop

1. `ralph run` continuously:
   - selects next ready leaf task,
   - executes iteration via Claude Code,
   - commits,
   - marks task complete,
   - continues until all leaf tasks are done or blocked/limits reached.

### Workflow D: Human Intervention

- `ralph status` shows blocked/ready/completed and last failure.
- `ralph pause` / `ralph resume`
- `ralph retry --task <id>` reruns with additional feedback.
- `ralph skip --task <id>` marks skipped (tracked, not silent).
- `ralph revert --iteration <n>` resets branch to earlier commit.

---

## 8) Functional Requirements

### 8.1 Task Model and Storage

**Task fields (minimum):**

- `id`
- `title`
- `description` (standalone; no hidden context)
- `parentId` (optional)
- `dependsOn[]`
- `status`: `open | in_progress | completed | blocked | failed | skipped`
- `ready` (derived; all deps completed)
- `acceptance[]` (verifiable)
- `verify`: commands to run (typecheck/test/lint)
- `labels/tags` (optional: area=ui/backend/db)
- `createdAt/updatedAt`

**Hierarchy rules:**

- Tasks may form a tree (parent → container → leaf).
- Harness executes only **leaf** tasks.

**Storage backends (pluggable):**

- MVP: **local file store** (JSON/YAML under `.ralph/tasks/`)
- V1: GitHub Issues/Projects, Linear, Jira (optional adapters)

### 8.2 Ready Task Selection Algorithm

Given `parentTaskId`, build descendant set:

1. recursively gather children by `parentId`
2. compute `ready(task)` = all `dependsOn` completed
3. filter:
   - `status=open`
   - `ready=true`
   - `isLeaf=true`

4. choose next task:
   - prefer tasks in same “area” as last completed (heuristic),
   - else deterministic ordering (createdAt, then id).

### 8.3 Iteration Orchestration

Each iteration produces an **Iteration Record**:

- `iterationId`
- `taskId`
- `start/end timestamps`
- `claudeCode` invocation metadata (command, model preset if available)
- `git base commit` and `result commit`
- `verification outputs summary`
- `files changed` (from git diff)
- `outcome`: `success | failed | budget_exceeded | blocked`
- `feedback` appended for next attempt (if failure)

### 8.4 Claude Code Integration (Only)

Harness must integrate with **Claude Code** as the _only_ coding backend.

**Integration mode (MVP):** CLI subprocess execution.

**Claude invocation contract:**

- Harness constructs a single **iteration prompt bundle** and feeds it to Claude Code.
- Claude Code is instructed to:
  1. implement the task,
  2. run verification commands (or allow harness to run them),
  3. fix failures,
  4. update `progress.md` and optional `AGENTS.md`,
  5. stop when done.

**Recommended enforcement:**

- Harness owns verification (authoritative).
- Claude may run tests for speed, but harness always re-runs for trust.

**Result expectations:**

- Claude modifies files in the repo working tree.
- Claude produces a textual summary + learnings + suggested follow-up tasks.

### 8.5 Verification Pipeline (Harness-Owned)

Even if Claude claims success, harness must verify.

Minimum checks (configurable):

- `npm run typecheck` (or equivalent)
- `npm test` (if configured)
- `golangci-lint` / `go test ./...` (if Go project)
- formatters (optional)

**Policy:**

- If verification fails:
  - iteration outcome = failed
  - capture trimmed failing output
  - next iteration prompt includes failing outputs and a strict “fix-only” directive.

### 8.6 Git Discipline

Harness supports:

- dedicated branch per feature: `ralph/<feature-slug>`
- commit only after passing verification
- commit templates:
  - `feat: <task title>`
  - `fix: <task title>`
  - `chore: <task title>`

- optional: squash at end (user-controlled)

### 8.7 Progress and Memory Files

**Short-term feature memory:**

- `.ralph/progress.md` (preferred)
- includes:
  - header: start date, feature name, parent task id
  - “Codebase Patterns” (top)
  - per-iteration entries appended

**Long-term memory:**

- `AGENTS.md` files (only durable patterns).

Harness responsibilities:

- archive old progress on new feature
- keep progress concise (hard cap/prune)
- include “Codebase Patterns” in every Claude iteration.

### 8.8 Guardrails and Stop Conditions

Configurable bounds:

- max iterations per run
- max time per iteration and per run
- max verification retries per iteration
- optional: max cost (if derivable indirectly; otherwise time/iteration cap)

**Gutter detection:**

- same verify failure repeated N times
- repeated churn on same files with no improvement
- oscillation detection (diff churn heuristic)

On gutter detection:

- force stronger prompt constraints (“diagnostic mode”)
- optionally create a “human intervention required” task
- pause the loop if configured.

### 8.9 Status Reporting

Commands:

- `ralph status`
  - counts: completed/ready/blocked
  - next selected task
  - last iteration outcome + log pointers

- `ralph logs --iteration <n>`
- `ralph report` (end-of-feature summary with commits + highlights)

---

## 9) Non-Functional Requirements

### Reliability

- Crash-safe state (atomic writes)
- Resumable runs (`ralph run` continues)

### Security / Safety

- Sandbox-friendly:
  - run in containerized workspace (optional but recommended)
  - restrict shell commands to allowlist

- Secrets hygiene:
  - never print env secrets
  - never include secrets in Claude prompt bundle

### Observability

- JSON structured logs per iteration
- human-readable summaries
- optional OpenTelemetry spans (future)

### Performance

- bounded context packaging
- avoid full repo scans; prefer targeted search

---

## 10) UX / CLI Specification (Go / Cobra)

### Commands (MVP)

- `ralph init`
  - `--parent <id>` or `--search <term>`
  - writes `.ralph/parent-task-id`
  - validates graph

- `ralph run`
  - executes loop
  - `--once` single iteration
  - `--max-iterations N`

- `ralph status`
- `ralph pause` / `ralph resume`
- `ralph retry --task <id>`
- `ralph skip --task <id>`
- `ralph report`

### Config

`ralph.yaml` + env overrides.

Example:

```yaml
repo:
  root: .
  branch_prefix: "ralph/"
tasks:
  backend: "local"
  path: ".ralph/tasks"
  parent_id_file: ".ralph/parent-task-id"
memory:
  progress_file: ".ralph/progress.md"
  archive_dir: ".ralph/archive"
claude:
  # Claude Code CLI invocation
  command: ["claude", "code"]
  # optional preset/profile if Claude supports it in your environment
  args: []
verification:
  commands:
    - ["npm", "run", "typecheck"]
    - ["npm", "test"]
loop:
  max_iterations: 50
  max_minutes_per_iteration: 20
  gutter:
    max_same_failure: 3
    max_churn_commits: 2
safety:
  sandbox: true
  allowed_commands:
    - "npm"
    - "go"
    - "git"
```

---

## 11) System Design

### 11.1 High-Level Architecture

Components:

1. **TaskStore**
2. **Selector**
3. **ClaudeRunner** (only coding runner)
4. **Verifier**
5. **GitManager**
6. **MemoryManager**
7. **LoopController**
8. **Reporter**

### 11.2 Iteration Flow (Detailed)

1. Load config
2. Resolve parent task
3. Fetch descendant tasks
4. Compute ready leaf tasks
5. Choose task
6. Prepare Claude prompt bundle:
   - task description + acceptance + verify commands
   - Codebase Patterns from progress
   - last failure outputs (if retry)
   - minimal repo facts (branch, diff stat, relevant file paths)

7. Invoke Claude Code
8. Ensure changes exist (git diff)
9. Run verification pipeline
10. If pass:
    - append progress entry
    - update AGENTS.md if changed
    - commit
    - mark task completed

11. If fail:
    - capture trimmed outputs
    - record failure signature
    - prepare feedback for next attempt

12. Continue/stop based on budgets and readiness.

### 11.3 Context Packaging Strategy

Bounded inputs:

- task description (always)
- Codebase Patterns (always)
- last failure tail (N lines)
- `git diff --stat` + changed files list
- optional targeted greps (harness-run)

Hard caps:

- max bytes for logs
- max file list entries
- max failure output lines

---

## 12) Claude Prompt Protocol (Harness-Owned)

Harness enforces a stable iteration template.

**Required Claude instructions:**

- Implement only this task.
- Do not claim completion unless verification commands pass.
- Prefer minimal, surgical changes.
- Update `.ralph/progress.md` (append) with:
  - what changed
  - files touched
  - learnings (patterns/gotchas)

- Update `AGENTS.md` only with durable guidance.

**Commit policy:**

- Preferred: Claude does not commit; harness commits after verification.
- Alternate: Claude commits with exact message; harness validates.

---

## 13) MVP Deliverables

- Local TaskStore
- Dependency validation (acyclic)
- Ready leaf selection
- Claude Code subprocess runner
- Verification runner
- Git branching + commit
- Progress management + archive
- Loop control: max iterations/time
- Status + report commands

---

## 14) Risks and Mitigations

1. Infinite loops / runaway usage
   - budgets, gutter detection, `--once`

2. Thrashing / oscillation
   - churn detection, fix-only retry mode, failure signatures

3. Unsafe actions
   - command allowlist, sandbox mode

4. Tasks not self-contained
   - task linter (must include acceptance + verify + files)

5. Over-contexting
   - strict caps, prefer repo facts, always fresh iteration context

---

## 15) Acceptance Criteria (System-Level)

A feature run is successful if:

- `ralph run` can complete all ready leaf tasks under a parent task
- each completed task yields a verified git commit
- verification commands pass on each completion
- progress log contains per-iteration entries
- final report lists commits, completed tasks, and any blocked tasks with reasons

---

## 16) Implementation Notes for Go

- CLI: Cobra
- State dir: `.ralph/`
- Task graph: topo sort + cycle detection
- Logs:
  - `.ralph/logs/iteration-<n>.json`
  - `.ralph/logs/iteration-<n>.txt`

- Git integration:
  - shell out to `git` (MVP) for fidelity

- Verification:
  - stream to files; include tails in next iteration prompt.

---

## 17) Assumptions

- Claude Code modifies files directly in the working tree.
- Tasks contain verifiable criteria and commands.
- Harness is the enforcement point: verify, commit, record, and decide.

---

If you want the next deliverable: I can produce the **Go package boundaries + interfaces** (TaskStore, Selector, ClaudeRunner, Verifier, GitManager, LoopController), plus the **exact Claude iteration prompt template** and a **task linter spec** to reject vague tasks before the loop runs.
