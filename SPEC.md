# SPEC: PRD Alignment and Completion

## Overview

This specification describes how to align the current Ralph harness implementation with PRD.md by addressing all discrepancies documented in PRD_DISCREPANCIES.md. The work is organized into phases that can be tackled incrementally while maintaining a working system.

## Implementation Phases

### Phase 1: CLI and Configuration Foundation

**Objective**: Fix configuration loading and basic CLI functionality.

#### 1.1 Honor --config Flag

**Current Issue**: `--config` flag is defined but ignored; all commands call `config.LoadConfig(workDir)` directly.

**Implementation**:
- Modify `cmd/root.go`:
  - Store `cfgFile` in a package-level variable or pass through context
  - Pass `cfgFile` to `LoadConfig` if set, otherwise use default path
- Update all command files to respect the config path from root command
- Files to modify: `cmd/root.go`, `internal/config/config.go`

**Acceptance**:
- `ralph --config path/to/ralph.yaml run` uses specified config file
- Default behavior unchanged when flag omitted
- Error message shown if specified config file doesn't exist

#### 1.2 Honor claude.command and claude.args

**Current Issue**: Only first command element is used; args are ignored.

**Implementation**:
- Modify `cmd/run.go` and `internal/claude/exec.go`:
  - Build full command array from `config.Claude.Command` + `config.Claude.Args`
  - Append Claude Code's required flags after base command
- Files to modify: `cmd/run.go`, `internal/claude/exec.go`

**Acceptance**:
- Config like `command: ["claude", "code"]` and `args: ["--preset=fast"]` results in `claude code --preset=fast --output-format=stream-json ...`
- Empty args work correctly
- Unit test verifies command construction

#### 1.3 Add ralph logs Command

**Current Issue**: Command missing from CLI.

**Implementation**:
- Create `cmd/logs.go`:
  - `--iteration N` flag to show specific iteration log
  - List available iterations if no flag provided
  - Read from `.ralph/logs/iteration-<n>.json` and pretty-print
  - Optionally show raw Claude NDJSON from `.ralph/logs/claude/`
- Register command in `cmd/root.go`
- Files to create: `cmd/logs.go`

**Acceptance**:
- `ralph logs` lists all iterations
- `ralph logs --iteration 5` shows iteration 5 details
- Clear error if iteration doesn't exist
- JSON formatted for readability

#### 1.4 Add ralph revert Command

**Current Issue**: Command missing from CLI.

**Implementation**:
- Create `cmd/revert.go`:
  - `--iteration N` flag (required)
  - Reads iteration record to get base commit
  - Uses GitManager to `git reset --hard <commit>`
  - Updates task status back to open if task was completed
  - Warns user about uncommitted changes
- Requires confirmation unless `--force` flag
- Files to create: `cmd/revert.go`

**Acceptance**:
- `ralph revert --iteration 5` resets to before iteration 5
- Requires confirmation prompt
- Updates task status correctly
- Error if iteration doesn't exist or git fails

#### 1.5 Fix ralph pause Checking

**Current Issue**: Pause flag only checked at start of run, not between iterations.

**Implementation**:
- Modify `internal/loop/controller.go`:
  - Check pause flag in main loop before selecting next task
  - Add `checkPaused()` method that reads `.ralph/state/paused`
  - Return early with appropriate message if paused
- Files to modify: `internal/loop/controller.go`

**Acceptance**:
- Running `ralph pause` in another terminal stops loop after current iteration
- Status message shows "paused" state
- `ralph resume` allows loop to continue

---

### Phase 2: Task Storage and Status Management

**Objective**: Fix task status transitions and enable YAML import.

#### 2.1 Implement Task Status Transitions

**Current Issue**: `failed` and `blocked` statuses never set; failures just reset to `open`.

**Implementation**:
- Modify `internal/loop/controller.go`:
  - On verification failure:
    - If attempt < max retries: keep `in_progress`, add feedback
    - If max retries exceeded: set status to `failed`
  - On gutter detection: set status to `blocked`
  - Track attempt count in iteration state or task metadata
- Add `attempt` field to task or iteration record
- Files to modify: `internal/loop/controller.go`, `internal/taskstore/model.go`

**Acceptance**:
- Tasks failing max retries are marked `failed`
- Tasks hitting gutter detection are marked `blocked`
- `ralph status` shows accurate failed/blocked counts
- Blocked/failed tasks skip selection until manually reset

#### 2.2 Add YAML Import CLI Command

**Current Issue**: YAML import exists but no CLI to use it; docs mention YAML but only JSON works.

**Implementation**:
- Create `cmd/import.go`:
  - Reads YAML file path as argument
  - Calls `taskstore.ImportFromYAML`
  - Validates all tasks before importing
  - Reports errors with task IDs
- Add `--overwrite` flag to replace existing tasks
- Files to create: `cmd/import.go`

**Acceptance**:
- `ralph import tasks.yaml` loads tasks into store
- Validation errors show task IDs and reasons
- Existing tasks are not overwritten unless `--overwrite`
- Import is atomic (all or nothing)

#### 2.3 Implement Task Linter

**Current Issue**: No validation that tasks have required acceptance/verify fields.

**Implementation**:
- Create `internal/taskstore/linter.go`:
  - `LintTask(task)` validates:
    - Description is non-empty
    - Acceptance criteria present (warn if missing)
    - Verify commands present for leaf tasks
    - No dependency cycles
    - All dependsOn tasks exist
  - `LintTaskSet(tasks)` validates entire graph
- Call linter in:
  - `ralph init` to validate before starting
  - `ralph import` to validate before importing
- Files to create: `internal/taskstore/linter.go`
- Files to modify: `cmd/init.go`, `cmd/import.go`

**Acceptance**:
- Init fails if tasks are invalid
- Import fails if tasks are invalid
- Clear error messages identify specific problems
- Unit tests for each validation rule

---

### Phase 3: Prompt Engineering and Context Packaging

**Objective**: Use the full prompt builder with all required context.

#### 3.1 Integrate Iteration Prompt Builder

**Current Issue**: Loop uses minimal prompt; `internal/prompt/iteration.go` unused.

**Implementation**:
- Modify `internal/loop/controller.go`:
  - Replace minimal prompt with `prompt.BuildIterationPrompt`
  - Pass task, config, memory manager, git manager to builder
- Modify `internal/prompt/iteration.go`:
  - Include system prompt from PRD section 12:
    - "Implement only this task"
    - "Do not claim completion unless verification passes"
    - "Update .ralph/progress.md with learnings"
  - Include task description and acceptance criteria
  - Include verify commands
  - Include Codebase Patterns from progress.md
  - Include `git diff --stat` and changed files list
  - Enforce size limits (configurable, default 100KB)
- Files to modify: `internal/loop/controller.go`, `internal/prompt/iteration.go`

**Acceptance**:
- Generated prompts include all PRD-specified context
- Size limits enforced; context trimmed if needed
- System prompt follows PRD template
- Unit tests verify all sections present

#### 3.2 Integrate Retry Prompt Builder

**Current Issue**: Retry feedback stored but never used; no fix-only prompts.

**Implementation**:
- Modify `internal/loop/controller.go`:
  - On verification failure, call `prompt.BuildRetryPrompt`
  - Pass previous iteration record with failure output
  - Pass user feedback if from `ralph retry --task X --feedback "..."`
- Modify `internal/prompt/retry.go`:
  - Add strict "fix-only" directive
  - Include trimmed verification output (use `verifier/trimmer.go`)
  - Include failure signature for context
  - Append user feedback if provided
- Files to modify: `internal/loop/controller.go`, `internal/prompt/retry.go`, `cmd/retry.go`

**Acceptance**:
- Retry prompts include trimmed failure output
- Fix-only directive present and clear
- User feedback appended when provided
- Trimming respects max lines config (default 100 lines)

#### 3.3 Extract and Use Codebase Patterns

**Current Issue**: Patterns not extracted from progress.md or injected into prompts.

**Implementation**:
- Modify `internal/memory/progress.go`:
  - Implement `GetCodebasePatterns()` to extract "## Codebase Patterns" section
  - Parse markdown to find section
  - Return content as string
- Ensure patterns section exists in progress template
- Call in prompt builder to include patterns
- Files to modify: `internal/memory/progress.go`, `internal/prompt/iteration.go`

**Acceptance**:
- `GetCodebasePatterns` extracts patterns section
- Patterns included in every Claude iteration prompt
- Empty patterns section handled gracefully
- Unit tests for extraction

---

### Phase 4: Verification and In-Iteration Retry

**Objective**: Enable config-level verification commands and in-iteration fix loop.

#### 4.1 Use Config-Level Verification Commands

**Current Issue**: `config.Verification.Commands` ignored; only task-level verify used.

**Implementation**:
- Modify `internal/loop/controller.go`:
  - Merge config-level and task-level verify commands
  - Run both: config verification + task-specific verification
  - Config commands run first (typecheck/lint), then task commands (tests)
- Files to modify: `internal/loop/controller.go`

**Acceptance**:
- Both config and task verification commands execute
- All must pass for iteration to succeed
- Clear output showing which command failed
- Config commands optional if not specified

#### 4.2 Implement In-Iteration Retry Loop

**Current Issue**: No fix-only retry within iteration; one failure ends iteration.

**Implementation**:
- Modify `internal/loop/controller.go`:
  - After initial Claude run, if verification fails:
    1. Build retry prompt with failure output
    2. Invoke Claude again with `--continue` (same session)
    3. Re-run verification
    4. Repeat up to `max_verification_retries` (new config option)
  - Track attempt number
  - If all retries exhausted, mark iteration failed
- Add `loop.max_verification_retries` to config (default 2)
- Files to modify: `internal/loop/controller.go`, `internal/config/config.go`

**Acceptance**:
- Claude gets chances to fix verification failures
- Retry prompts are fix-only focused
- Max retries configurable
- Attempt count tracked in iteration record

#### 4.3 Enforce Per-Iteration Time Limit

**Current Issue**: `max_minutes_per_iteration` configured but not enforced.

**Implementation**:
- Modify `internal/loop/controller.go`:
  - Create context with timeout from config
  - Pass to Claude runner
  - Cancel context if timeout exceeded
  - Record timeout in iteration outcome
- Modify `internal/loop/budget.go`:
  - Track per-iteration duration
  - Update `CheckBudget` to enforce iteration time limit
- Files to modify: `internal/loop/controller.go`, `internal/loop/budget.go`

**Acceptance**:
- Claude subprocess killed if iteration exceeds time limit
- Iteration marked as `budget_exceeded`
- Budget JSON tracks iteration durations
- Config default: 20 minutes

---

### Phase 5: Git Discipline and Reporting

**Objective**: Create feature branches and improve reporting.

#### 5.1 Create and Use Feature Branches

**Current Issue**: `EnsureBranch` exists but never called; runs stay on current branch.

**Implementation**:
- Modify `internal/loop/controller.go`:
  - Call `gitManager.EnsureBranch` at start of run
  - Generate branch name from parent task: `ralph/<parent-task-title-slug>`
  - If branch exists, switch to it
  - If not, create from current branch
- Update `ralph init` to suggest branch creation
- Files to modify: `internal/loop/controller.go`, `cmd/init.go`

**Acceptance**:
- `ralph run` creates/switches to feature branch
- Branch name format: `ralph/<feature-name>`
- Existing branches reused
- User can override with `--branch` flag

#### 5.2 Generate Human-Readable Iteration Logs

**Current Issue**: Only JSON logs; no `.txt` human log file.

**Implementation**:
- Modify `internal/loop/record.go`:
  - After saving JSON, generate text summary
  - Write to `.ralph/logs/iteration-<n>.txt`
  - Include:
    - Task title/ID
    - Start/end times and duration
    - Outcome
    - Files changed
    - Verification results (pass/fail)
    - Commit hash if successful
- Files to modify: `internal/loop/record.go`

**Acceptance**:
- Both `.json` and `.txt` logs created per iteration
- Text log is human-readable
- Text log includes key metrics
- `ralph logs` can show text version

#### 5.3 Include Commit Messages in Reports

**Current Issue**: `CommitInfo.Message` never populated in reports.

**Implementation**:
- Modify `internal/git/manager.go`:
  - Add `GetCommitMessage(hash)` method
  - Shell out: `git log -1 --format=%B <hash>`
- Modify `internal/reporter/report.go`:
  - Call `GetCommitMessage` for each commit
  - Populate `CommitInfo.Message`
- Files to modify: `internal/git/manager.go`, `internal/reporter/report.go`

**Acceptance**:
- Reports include commit messages
- Messages formatted cleanly
- Errors handled if commit missing

---

### Phase 6: Memory Hygiene and Long-Term State

**Objective**: Archive progress, enforce size limits, manage AGENTS.md.

#### 6.1 Archive Old Progress on Feature Switch

**Current Issue**: Archive function exists but never called.

**Implementation**:
- Modify `cmd/init.go`:
  - When setting new parent task (different from previous):
    - Call `memoryManager.ArchiveProgress()`
    - Create new progress.md with fresh header
  - Store previous parent ID in state to detect changes
- Files to modify: `cmd/init.go`, `internal/memory/archive.go`

**Acceptance**:
- Old progress moved to `.ralph/archive/progress-TIMESTAMP.md`
- New progress.md created with header
- Feature name and parent ID in header
- Archive only happens on parent task change

#### 6.2 Enforce Progress File Size Limits

**Current Issue**: No pruning; logs can grow unbounded.

**Implementation**:
- Modify `internal/memory/progress.go`:
  - After appending iteration, check file size
  - If exceeds config limit (default 1MB), call `EnforceMaxSize`
  - Prune oldest entries while preserving:
    - Header and Codebase Patterns (always keep)
    - Most recent N iterations (configurable, default 20)
- Add `memory.max_progress_bytes` to config
- Files to modify: `internal/memory/progress.go`, `internal/config/config.go`

**Acceptance**:
- Progress file stays under size limit
- Header and patterns never pruned
- Recent iterations preserved
- Pruning note added when triggered

#### 6.3 Support AGENTS.md Updates

**Current Issue**: No AGENTS.md creation or prompt enforcement.

**Implementation**:
- Modify `internal/prompt/iteration.go`:
  - Add instruction: "Update AGENTS.md only with durable, reusable patterns"
  - Specify format and location (relevant directories)
- Create `internal/memory/agents.go`:
  - `FindAgentsMd()` searches cwd and subdirectories
  - `ReadAgentsMd()` returns content for prompt inclusion
  - Include AGENTS.md excerpts in prompt if found
- Files to create: `internal/memory/agents.go`
- Files to modify: `internal/prompt/iteration.go`

**Acceptance**:
- System prompt includes AGENTS.md guidance
- Existing AGENTS.md files read and included in context
- No automatic creation; Claude decides when to add
- Unit tests for file discovery

---

### Phase 7: Safety and Robustness

**Objective**: Enable sandbox mode, command allowlists, better gutter detection.

#### 7.1 Implement Sandbox Mode Guard

**Current Issue**: `safety.sandbox` flag unused.

**Implementation**:
- Modify `internal/claude/exec.go` and `internal/verifier/runner.go`:
  - If `config.Safety.Sandbox` is true:
    - Check command against allowlist before execution
    - Reject commands not in `config.Safety.AllowedCommands`
  - Apply to both Claude invocations (via tool restrictions) and verifier
- Use Claude Code's `--allowedTools` flag to restrict tools
- Files to modify: `internal/claude/exec.go`, `internal/verifier/runner.go`

**Acceptance**:
- Sandbox mode blocks disallowed commands
- Clear error message when blocked
- Allowlist configurable in ralph.yaml
- Default allowlist: `["npm", "go", "git"]`

#### 7.2 Improve Gutter Detection

**Current Issue**: Basic implementation; could be more robust.

**Implementation**:
- Enhance `internal/loop/gutter.go`:
  - Detect oscillation: same files modified back and forth
  - Use content hash, not just file list
  - Detect no-progress: commits made but no verification improvement
  - Track failure signatures across iterations
  - Configurable thresholds for each detection type
- Add gutter detection config options:
  - `gutter.max_same_failure` (exists, default 3)
  - `gutter.max_oscillations` (new, default 2)
  - `gutter.enable_content_hash` (new, default true)
- Files to modify: `internal/loop/gutter.go`, `internal/config/config.go`

**Acceptance**:
- Oscillation detected when file changes revert
- Content hash prevents false positives
- All thresholds configurable
- Unit tests for each detection type

#### 7.3 Add Retry Feedback Support

**Current Issue**: Retry stores feedback but it's never read.

**Implementation**:
- Modify `cmd/retry.go`:
  - Add `--feedback "message"` flag
  - Store feedback in iteration state or task metadata
- Modify `internal/loop/controller.go`:
  - Read feedback when selecting task
  - Pass to retry prompt builder
- Create state file for retry feedback: `.ralph/state/retry-feedback.json`
- Files to modify: `cmd/retry.go`, `internal/loop/controller.go`

**Acceptance**:
- `ralph retry --task X --feedback "Try approach Y"` stores feedback
- Next iteration includes feedback in retry prompt
- Feedback cleared after successful iteration
- Feedback shown in status command

---

## Implementation Order

Recommended order to maintain working system:

1. **Phase 1** (CLI/config foundation) - ensures basic functionality works correctly
2. **Phase 3** (prompt engineering) - biggest impact on Claude success rate
3. **Phase 4** (verification and retry) - enables in-iteration fixes
4. **Phase 2** (task status) - better state tracking
5. **Phase 5** (git and reporting) - improves observability
6. **Phase 6** (memory hygiene) - long-term state management
7. **Phase 7** (safety) - robustness and guardrails

Each phase can be tackled independently, with incremental PRs and tests.

---

## Testing Strategy

For each phase:

1. **Unit Tests**: Test new/modified functions in isolation
2. **Integration Tests**: Test command flows end-to-end
3. **Manual Validation**: Run `ralph run` on sample feature
4. **Regression Tests**: Ensure existing functionality unbroken

Use table-driven tests and `testify/assert` per CLAUDE.md guidelines.

---

## Configuration Updates

Add new config options as phases progress:

```yaml
# ralph.yaml additions

loop:
  max_verification_retries: 2      # Phase 4.2

memory:
  max_progress_bytes: 1048576      # Phase 6.2 (1MB)
  max_recent_iterations: 20        # Phase 6.2

gutter:
  max_oscillations: 2              # Phase 7.2
  enable_content_hash: true        # Phase 7.2

safety:
  sandbox: false                   # Phase 7.1
  allowed_commands:                # Phase 7.1
    - "npm"
    - "go"
    - "git"
```

---

## Success Criteria

The implementation is complete when:

1. All commands in PRD section 10 are implemented and functional
2. All config options in PRD section 10 are honored
3. Task status transitions match PRD section 8.1
4. Prompts include all context from PRD section 12
5. Memory hygiene follows PRD section 8.7
6. Git discipline follows PRD section 8.6
7. Verification follows PRD section 8.5
8. All items in PRD_DISCREPANCIES.md are resolved
9. System-level acceptance criteria from PRD section 15 pass
10. `ralph run` completes the MVP feature (self-hosting test)

---

## Self-Hosting Test

Final validation: Use Ralph to implement these phases.

1. Convert this SPEC.md into tasks.yaml
2. Run `ralph import spec-tasks.yaml`
3. Run `ralph init --parent spec-alignment`
4. Run `ralph run`
5. Verify Ralph can complete at least Phase 1 autonomously

This validates the core loop and provides real-world feedback on improvements needed.
