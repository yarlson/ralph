# Ralph MVP Progress

**Feature**: Ralph Wiggum Loop Harness MVP
**Parent Task**: ralph-mvp
**Started**: 2026-01-16

---

## Codebase Patterns

- **Config loading**: Use Viper with `SetDefault()` for all fields before `ReadInConfig()`. Handle `ConfigFileNotFoundError` gracefully to support running without config file.
- **Struct tags**: Use `mapstructure` tags for Viper unmarshaling, not `yaml` tags.
- **Testing**: Use `t.TempDir()` for isolated test fixtures. Tests should cover: valid config, defaults only, partial override, and invalid input.
- **CLI structure**: Use `NewRootCmd()` pattern returning `*cobra.Command`. Export `Execute()` for main.go. Use `SilenceUsage: true` on root. Stub commands return wrapped `errNotImplemented`.

---

## Iteration Log

### 2026-01-16: project-setup-config (Configuration Loading)

**What changed:**
- Created `internal/config` package with full configuration support
- Implemented `Config` struct matching PRD `ralph.yaml` schema with nested structs: `RepoConfig`, `TasksConfig`, `MemoryConfig`, `ClaudeConfig`, `VerificationConfig`, `LoopConfig`, `GutterConfig`, `SafetyConfig`
- Implemented `LoadConfig(dir)` function using Viper for YAML parsing with sensible defaults
- Added `LoadConfigFromPath(configPath)` helper

**Files touched:**
- `internal/config/config.go` (new)
- `internal/config/config_test.go` (new)
- `go.mod` / `go.sum` (updated with viper, testify dependencies)

**Learnings:**
- Viper requires `mapstructure` struct tags, not `yaml` tags, for `Unmarshal()` to work correctly
- `viper.ConfigFileNotFoundError` type assertion needed to distinguish "no config" from "bad config"
- Nested config structs work well with dot notation in `SetDefault()` (e.g., `loop.gutter.max_same_failure`)

**Outcome**: Success - all 5 tests pass, `go build ./...` succeeds

### 2026-01-16: project-setup-cli-skeleton (Cobra CLI Skeleton)

**What changed:**
- Created `cmd/` package with Cobra-based CLI
- Implemented `NewRootCmd()` returning root command with `--config` flag
- Added 8 stub subcommands: init, run, status, pause, resume, retry, skip, report
- All stubs return `"<cmd>: not implemented"` error
- Created `main.go` invoking `cmd.Execute()`
- Added `Execute()` function for main entrypoint

**Files touched:**
- `cmd/root.go` (new)
- `cmd/root_test.go` (new)
- `main.go` (new)
- `go.mod` / `go.sum` (updated with cobra dependency)

**Learnings:**
- Cobra's `SilenceUsage: true` prevents usage from printing on every error
- Use `cmd.SetOut()` and `cmd.SetErr()` in tests to capture output
- Wrap stub errors with `fmt.Errorf("cmd: %w", errNotImplemented)` for consistent error messages

**Outcome**: Success - all 12 tests pass, `go build ./...` and `go test ./cmd/...` succeed

### 2026-01-16: project-setup-ralph-dir (.ralph Directory Structure)

**What changed:**
- Created `internal/state` package for directory structure management
- Implemented `EnsureRalphDir(root)` function that creates the full `.ralph/` directory tree
- Added path helper functions: `RalphDirPath`, `TasksDirPath`, `StateDirPath`, `LogsDirPath`, `ClaudeLogsDirPath`, `ArchiveDirPath`
- Function is idempotent and creates directories with 0755 permissions

**Files touched:**
- `internal/state/state.go` (new)
- `internal/state/state_test.go` (new)

**Learnings:**
- `os.MkdirAll` is naturally idempotent - it succeeds even if directories already exist
- Checking root existence before creating subdirs gives clearer error messages
- Use `t.TempDir()` for filesystem tests - it auto-cleans up

**Outcome**: Success - all 6 tests pass, `go build ./...`, `go test ./...`, and `golangci-lint run` succeed

### 2026-01-16: task-store-model (Task Model Definition)

**What changed:**
- Created `internal/taskstore` package with Task model and TaskStatus type
- Implemented `Task` struct with all required fields: ID, Title, Description, ParentID (*string), DependsOn, Status, Acceptance, Verify, Labels, CreatedAt, UpdatedAt
- Implemented `TaskStatus` type as string enum with values: open, in_progress, completed, blocked, failed, skipped
- Added `IsValid()` method on TaskStatus for validation
- Added `Validate()` method on Task that checks required fields and valid status
- Full JSON serialization support with appropriate tags and omitempty for optional fields

**Files touched:**
- `internal/taskstore/model.go` (new)
- `internal/taskstore/model_test.go` (new)

**Learnings:**
- Use `*string` for optional parent ID to distinguish "no parent" (nil) from empty string
- Using a map for status validation (`validStatuses`) provides O(1) lookup for `IsValid()`
- JSON tags with `omitempty` keep serialized output clean for optional fields
- TDD approach: wrote 13 tests first covering status validity, JSON serialization, and all validation error cases

**Outcome**: Success - all 13 tests pass, `go build ./...`, `go test ./...`, and `golangci-lint run` succeed

### 2026-01-16: task-store-interface (TaskStore Interface)

**What changed:**
- Defined `Store` interface in `internal/taskstore/store.go` with all required CRUD methods
- Implemented `NotFoundError` type that wraps `ErrNotFound` sentinel with task ID
- Implemented `ValidationError` type that wraps `ErrValidation` sentinel with task ID and reason
- Both error types implement `Unwrap()` for use with `errors.Is()` and `errors.As()`

**Files touched:**
- `internal/taskstore/store.go` (new)
- `internal/taskstore/store_test.go` (new)

**Learnings:**
- Go error wrapping pattern: define sentinel errors (`var ErrNotFound = errors.New(...)`) and wrap them in typed errors that implement `Unwrap()` returning the sentinel
- Use `errors.Join()` in tests to verify that `errors.Is()` and `errors.As()` work through wrapped error chains
- Interface naming: using `Store` instead of `TaskStore` since the package is already `taskstore`, avoiding stutter (`taskstore.TaskStore` vs `taskstore.Store`)

**Outcome**: Success - all 16 tests pass, `go build ./...`, `go test ./...`, and `golangci-lint run` succeed

### 2026-01-16: task-store-local (Local File Store Implementation)

**What changed:**
- Implemented `LocalStore` struct in `internal/taskstore/local.go` that implements the `Store` interface
- Tasks persisted as individual JSON files in configured directory (`.ralph/tasks/{id}.json`)
- Atomic writes using temp file + rename pattern to prevent corruption
- Concurrent access safety using `sync.RWMutex` (read lock for Get/List, write lock for Save/Update/Delete)
- `NewLocalStore(dir)` constructor creates directory if not exists

**Files touched:**
- `internal/taskstore/local.go` (new)
- `internal/taskstore/local_test.go` (new)

**Learnings:**
- Use `sync.RWMutex` with `RLock()` for reads and `Lock()` for writes to allow concurrent reads
- Internal `getUnlocked()` helper allows code reuse when caller already holds a lock (e.g., `UpdateStatus` calls it after acquiring write lock)
- Atomic write pattern: write to `.tmp` file, then `os.Rename()` - rename is atomic on POSIX systems
- When cleanup code can't meaningfully handle errors (like removing temp file after rename failed), use explicit ignore: `_ = os.Remove(tmpFile)`

**Outcome**: Success - all 37 tests pass, `go build ./...`, `go test ./...`, and `golangci-lint run` succeed

### 2026-01-16: task-store-yaml-import (YAML Task Import)

**What changed:**
- Implemented `ImportFromYAML(store, path)` function in `internal/taskstore/yaml.go`
- Created `YAMLTask` and `YAMLFile` structs for YAML parsing with appropriate tags
- Created `ImportResult` and `ImportError` types to report import results
- Function validates all tasks before importing, reports errors with task IDs
- Defaults status to "open" if not specified in YAML
- Updates existing tasks if task ID already exists in store
- Converts YAML field names (parentId) to internal model (*string ParentID)

**Files touched:**
- `internal/taskstore/yaml.go` (new)
- `internal/taskstore/yaml_test.go` (new)

**Learnings:**
- Use `gopkg.in/yaml.v3` directly for YAML parsing (simpler than Viper for this use case)
- YAML struct tags use camelCase field names (parentId, dependsOn) to match tasks.yaml format
- Separate YAML struct from internal model allows for field name translation and default application
- TDD approach: wrote 9 comprehensive tests covering valid imports, defaults, validation errors, file errors, empty files, updates, and complex dependencies

**Outcome**: Success - all 46 tests pass, `go build ./...`, `go test ./...`, and `golangci-lint run` succeed

### 2026-01-16: selector-dependency-graph (Dependency Graph Builder)

**What changed:**
- Created `internal/selector` package with dependency graph implementation
- Implemented `Graph` struct with nodes, edges (dependencies), and reverseEdges (dependents)
- Implemented `BuildGraph(tasks)` function that constructs graph from task list and validates all dependencies exist
- Implemented `DetectCycle()` using DFS with coloring (white/gray/black) to find cycles and return cycle path
- Implemented `TopologicalSort()` using Kahn's algorithm for ordering tasks by dependencies
- Added helper methods: `Nodes()`, `HasNode()`, `Dependencies()`, `Dependents()`

**Files touched:**
- `internal/selector/graph.go` (new)
- `internal/selector/graph_test.go` (new)

**Learnings:**
- DFS cycle detection with coloring: white=unvisited, gray=in current path, black=fully explored; back-edge to gray node indicates cycle
- Kahn's algorithm for topo sort: start with nodes that have no dependencies (inDegree=0), process them, decrement dependents' inDegree, repeat
- Graph edges point from task to its dependencies (what it depends ON), reverse edges track what depends on a given task
- Return copies of slices from graph methods to prevent external mutation of internal state

**Outcome**: Success - all 15 selector tests pass, `go build ./...` and `go test ./...` succeed

### 2026-01-16: selector-ready-computation (Ready Task Computation)

**What changed:**
- Implemented `ComputeReady(tasks, graph)` function that computes ready status for each task
- A task is ready if all its dependencies (from dependsOn) are completed
- Implemented `IsLeaf(tasks, taskID)` function to identify leaf tasks (tasks with no children)
- Implemented `GetReadyLeaves(tasks, graph)` function that filters tasks by: status=open AND ready=true AND isLeaf=true

**Files touched:**
- `internal/selector/ready.go` (new)
- `internal/selector/ready_test.go` (new)

**Learnings:**
- Build status lookup map first for O(1) dependency status checks
- IsLeaf checks parentID references across all tasks - a task is a leaf if no other task has it as parentID
- GetReadyLeaves applies three filters sequentially: open status, ready (all deps completed), and leaf (no children)
- TDD approach: wrote 19 comprehensive tests covering no dependencies, completed/incomplete dependencies, transitive deps, leaf detection, and complex hierarchy scenarios

**Outcome**: Success - all 34 selector tests pass, `go build ./...` and `go test ./...` succeed

### 2026-01-16: selector-select-next (Next Task Selection)

**What changed:**
- Implemented `SelectNext(tasks, graph, parentID, lastCompleted)` function in `internal/selector/selector.go`
- Function gathers descendants of the parent task, computes ready leaves, and selects the next task
- Selection heuristics: 1) prefer tasks with same "area" label as last completed task, 2) deterministic ordering by createdAt then ID
- Added helper functions: `getDescendants()` (BFS traversal), `getReadyLeavesFromSubset()`, `sortTasksDeterministically()`, `getArea()`

**Files touched:**
- `internal/selector/selector.go` (new)
- `internal/selector/selector_test.go` (new)

**Learnings:**
- Use BFS with parent-to-children map to efficiently gather all descendants of a parent task
- Area preference heuristic: if multiple ready leaves exist and last completed task has an "area" label, prefer tasks with matching area
- Deterministic ordering uses `sort.Slice` with two-level comparison: first by CreatedAt, then by ID for tie-breaking
- TDD approach: wrote 17 comprehensive tests covering empty/single/multiple leaves, area preference, fallback ordering, deep hierarchies, and dependencies

**Outcome**: Success - all 49 selector tests pass, `go build ./...`, `go test ./...`, and `golangci-lint run` succeed

### 2026-01-16: claude-runner-interface (ClaudeRunner Interface)

**What changed:**
- Created `internal/claude` package for Claude Code integration
- Implemented `ClaudeRequest` struct with fields: Cwd, SystemPrompt, AllowedTools, Prompt, Continue, ExtraArgs, Env
- Implemented `ClaudeResponse` struct with fields: SessionID, Model, Version, FinalText, StreamText, Usage, TotalCostUSD, PermissionDenials, RawEventsPath
- Implemented `ClaudeUsage` struct for token usage statistics: InputTokens, OutputTokens, CacheCreationTokens, CacheReadTokens
- Defined `Runner` interface with `Run(ctx, req) (*ClaudeResponse, error)` method
- All types have JSON tags for logging and serialization

**Files touched:**
- `internal/claude/runner.go` (new)
- `internal/claude/runner_test.go` (new)

**Learnings:**
- Interface naming: `Runner` instead of `ClaudeRunner` to avoid stutter in package (`claude.Runner` vs `claude.ClaudeRunner`)
- The CLAUDE-CODE.md spec defines the exact contract for request/response types - follow it closely
- Context parameter in interface allows for cancellation/timeout support
- TDD approach: wrote 10 tests first covering struct defaults, all fields, JSON serialization, and interface implementation via mock

**Outcome**: Success - all 10 tests pass, `go build ./...`, `go test ./...`, and `golangci-lint run` succeed

### 2026-01-16: claude-runner-ndjson-parser (NDJSON Stream Parser)

**What changed:**
- Implemented NDJSON parser in `internal/claude/parser.go` for Claude Code's stream-json output
- Created `ParseResult` struct to hold extracted data: SessionID, Model, Version, Cwd, FinalText, StreamText, Usage, TotalCostUSD, DurationMS, NumTurns, IsError, PermissionDenials, ParseErrors
- Created internal event structs for parsing: `baseEvent`, `initEvent`, `assistantEvent`, `contentBlock`, `usageBlock`, `resultEvent`
- `ParseNDJSON(io.Reader)` function parses line-by-line with configurable scanner buffer (64KB initial, 10MB max)
- Handles event types: system/init, assistant/message, result/success, result/error
- Gracefully handles parse errors - continues parsing and records errors, fails only if no terminal result

**Files touched:**
- `internal/claude/parser.go` (new)
- `internal/claude/parser_test.go` (new)

**Learnings:**
- Use `bufio.Scanner.Buffer()` to configure scanner for large lines - default token limit is too small for large JSON
- Parse each line independently into `baseEvent` first to determine type, then unmarshal into specific struct
- Accumulate text from assistant/message content blocks using `strings.Builder` for efficiency
- Record parse errors in result rather than failing immediately - allows degraded parsing when possible
- TDD approach: wrote 19 tests covering empty input, system/init, assistant messages, result success/error, permission denials, malformed lines, large lines, and edge cases

**Outcome**: Success - all 29 claude tests pass, `go build ./...`, `go test ./...`, and `golangci-lint run` succeed

### 2026-01-16: claude-runner-subprocess (Subprocess Execution)

**What changed:**
- Implemented `SubprocessRunner` struct in `internal/claude/exec.go` that implements the `Runner` interface
- `NewSubprocessRunner(command, logsDir)` constructor for creating runner instances with configurable Claude binary and log directory
- `Run(ctx, req)` method executes Claude Code as subprocess with proper argument building
- `buildArgs(req)` helper constructs CLI flags: --output-format=stream-json, --system-prompt, --allowedTools, --continue, -p
- Sets working directory from `req.Cwd` and merges environment variables from `req.Env`
- Uses `io.TeeReader` to stream stdout to both NDJSON parser and log file simultaneously
- `generateLogFilename(taskID)` creates unique timestamped log filenames with sanitized task IDs
- Handles context cancellation to kill subprocess on timeout
- Raw NDJSON logs saved to configured logs directory

**Files touched:**
- `internal/claude/exec.go` (new)
- `internal/claude/exec_test.go` (new)

**Learnings:**
- Use `exec.CommandContext` for context-aware subprocess execution with automatic process kill on cancellation
- `io.TeeReader` allows simultaneous writing to log file while buffering for parser
- Sanitize task IDs for filenames using regex to replace invalid characters (slashes, spaces, etc.)
- For deferred file close with linter compliance, use `defer func() { _ = logFile.Close() }()` pattern
- TDD approach: wrote 20 tests covering argument building, log filename generation, working directory, environment variables, context cancellation, valid NDJSON parsing, error results, and permission denials

**Outcome**: Success - all 49 claude tests pass, `go build ./...`, `go test ./...`, and `golangci-lint run` succeed

### 2026-01-16: claude-runner-session-state (Session State Management)

**What changed:**
- Implemented `SessionState` struct in `internal/claude/session.go` with `PlannerSessionID`, `CoderSessionID`, and `UpdatedAt` fields
- Created `SessionMode` type with constants `SessionModePlanner` and `SessionModeCoder` for tracking separate session contexts
- Implemented `LoadSession(path)` function that loads session state from JSON file (returns empty state if file doesn't exist)
- Implemented `SaveSession(path, state)` function that persists session state to JSON file (creates parent directories if needed)
- Implemented `DetectSessionFork(currentID, newID)` function to detect when Claude returns a new session ID on --continue (fork detection)
- Added helper methods: `UpdatePlannerSession()`, `UpdateCoderSession()`, `GetSessionForMode()`, `UpdateSessionForMode()`

**Files touched:**
- `internal/claude/session.go` (new)
- `internal/claude/session_test.go` (new)

**Learnings:**
- Separate session IDs for planner vs coder allows different --continue contexts for different workflows
- Fork detection is useful when Claude Code starts a new session instead of continuing (e.g., context too long)
- Go's `errors.Is(err, os.ErrNotExist)` is the idiomatic way to check for file-not-found vs other read errors
- Don't use `omitempty` on `time.Time` struct fields (linter warning: has no effect on nested structs)

**Outcome**: Success - all 62 claude tests pass, `go build ./...`, `go test ./...`, and `golangci-lint run` succeed

### 2026-01-16: verifier-interface (Verifier Interface)

**What changed:**
- Created `internal/verifier` package for verification command execution
- Implemented `VerificationResult` struct with fields: Passed (bool), Command ([]string), Output (string), Duration (time.Duration)
- Defined `Verifier` interface with `Verify(ctx, commands [][]string)` and `VerifyTask(ctx, commands [][]string)` methods
- Both methods support context for timeout/cancellation
- All types have JSON tags for logging and serialization

**Files touched:**
- `internal/verifier/verifier.go` (new)
- `internal/verifier/verifier_test.go` (new)

**Learnings:**
- Interface naming: `Verifier` is appropriate here since it's the primary interface in the package
- TDD approach: wrote 9 tests first covering struct defaults, all fields, JSON serialization, interface implementation via mock, and context cancellation/timeout
- `VerifyTask` takes commands directly ([][]string) rather than *Task to avoid coupling verifier package to taskstore package - keeps interface clean and testable

**Outcome**: Success - all 9 verifier tests pass, `go build ./...`, `go test ./...`, and `golangci-lint run` succeed

### 2026-01-16: verifier-command-runner (Command Runner)

**What changed:**
- Implemented `CommandRunner` struct in `internal/verifier/runner.go` that implements the `Verifier` interface
- `NewCommandRunner(workDir)` constructor creates runner with optional working directory
- Commands executed as subprocesses using `exec.CommandContext` for context-aware timeout/cancellation
- Combined stdout/stderr captured in output using `bytes.Buffer`
- Supports command allowlist via `SetAllowedCommands()` - blocked commands return failure with descriptive error
- Supports output size limits via `SetMaxOutputSize()` - truncates large output with marker
- Sequential command execution - continues even after failed commands
- Proper handling of edge cases: empty commands, non-existent commands, context cancellation

**Files touched:**
- `internal/verifier/runner.go` (new)
- `internal/verifier/runner_test.go` (new)

**Learnings:**
- Use `exec.CommandContext` for subprocess execution with automatic process termination on context cancellation
- Allowlist should check base command name only (e.g., "go" in "go test ./...")
- Return failure result with descriptive output rather than error for blocked/invalid commands - keeps sequential execution intact
- Default 1MB max output size is reasonable; truncation preserves beginning and adds marker at end

**Outcome**: Success - all 28 verifier tests pass, `go build ./...`, `go test ./...`, and `golangci-lint run` succeed

### 2026-01-16: verifier-output-trimmer (Output Trimmer for Feedback)

**What changed:**
- Implemented `TrimOutput(output, opts)` function in `internal/verifier/trimmer.go`
- Created `TrimOptions` struct with `MaxLines` and `MaxBytes` configurable limits
- Trimming preserves the tail (end) of output since error messages typically appear at the end
- Added `TruncationMarker` constant ("... [output truncated]") prepended when trimming occurs
- Implemented `TrimOutputForFeedback(results, opts)` helper that formats failed verification results for Claude retry prompts
- Added `DefaultTrimOptions()` returning sensible defaults (100 lines, 8KB)
- Added `Validate()` method on TrimOptions to check for negative values

**Files touched:**
- `internal/verifier/trimmer.go` (new)
- `internal/verifier/trimmer_test.go` (new)

**Learnings:**
- Preserving tail of output is more useful for error feedback than preserving head - errors and failures typically appear at the end of verification output
- When applying both line and byte limits, apply line limit first then byte limit - this gives most predictable results
- Use Go's built-in `max()` function instead of manual if-else for cleaner code (Go 1.21+)
- TDD approach: wrote 25 tests first covering empty input, no limits, under limits, exact limits, both limits with different restrictiveness, single lines, trailing newlines, empty lines, large input, and feedback formatting

**Outcome**: Success - all 40 verifier tests pass, `go build ./...`, `go test ./...`, and `golangci-lint run` succeed

### 2026-01-16: git-manager-interface (GitManager Interface)

**What changed:**
- Created `internal/git` package for Git operations
- Defined `Manager` interface with 7 methods: `EnsureBranch`, `GetCurrentCommit`, `HasChanges`, `GetDiffStat`, `GetChangedFiles`, `Commit`, `GetCurrentBranch`
- All methods accept `context.Context` for cancellation/timeout support
- Implemented 4 sentinel errors: `ErrNotAGitRepo`, `ErrNoChanges`, `ErrBranchExists`, `ErrCommitFailed`
- Implemented `GitError` struct for rich error context with command, output, and wrapped error support
- `GitError` implements `Unwrap()` for use with `errors.Is()` and `errors.As()`

**Files touched:**
- `internal/git/manager.go` (new)
- `internal/git/manager_test.go` (new)

**Learnings:**
- Interface naming: `Manager` avoids stutter in package (`git.Manager` vs `git.GitManager`)
- All interface methods should accept `context.Context` as first parameter for proper cancellation support
- Sentinel errors with typed wrapper errors (like `GitError`) provide both convenient `errors.Is()` checks and detailed error context
- TDD approach: wrote 14 tests first covering interface implementation via mock, all method behaviors, error types, and error wrapping

**Outcome**: Success - all 14 git tests pass, `go build ./...` and `go test ./...` succeed

### 2026-01-16: git-manager-shell (Git Shell Implementation)

**What changed:**
- Implemented `ShellManager` struct in `internal/git/shell.go` that implements the `Manager` interface
- `NewShellManager(workDir, branchPrefix)` constructor creates manager with configurable working directory and branch prefix
- Implemented `runGit(ctx, args...)` helper method that shells out to git binary with proper error handling
- All 7 interface methods implemented: `GetCurrentBranch`, `GetCurrentCommit`, `HasChanges`, `GetDiffStat`, `GetChangedFiles`, `Commit`, `EnsureBranch`
- Uses `exec.CommandContext` for context-aware subprocess execution with automatic cancellation
- Case-insensitive "not a git repository" detection to handle varying git versions
- Branch operations use configured prefix (e.g., "ralph/") prepended to branch names
- `Commit` method stages all changes with `git add -A` before committing

**Files touched:**
- `internal/git/shell.go` (new)
- `internal/git/shell_test.go` (new)

**Learnings:**
- Test setup for git operations requires disabling GPG signing (`git config commit.gpgsign false`) when system has signing configured
- Use `git init -b main` to ensure consistent default branch name across different git configurations
- `git diff --stat` outputs "warning: Not a git repository" (capital N) vs other commands that use lowercase - need case-insensitive matching
- `git status --porcelain` format uses first 2 characters for status codes, followed by space, then filename
- Renamed files in porcelain format show as "old -> new" and need special parsing
- TDD approach: wrote 26 integration tests covering all interface methods, context cancellation/timeout, non-git-repo errors, and edge cases

**Outcome**: Success - all 40 git tests pass, `go build ./...`, `go test ./...`, and `golangci-lint run` succeed

### 2026-01-16: git-manager-commit-template (Commit Message Templates)

**What changed:**
- Implemented commit message formatting in `internal/git/commit.go`
- Created `CommitType` type with constants: `CommitTypeFeat`, `CommitTypeFix`, `CommitTypeChore`
- Implemented `InferCommitType(title)` that analyzes task title keywords to determine commit type:
  - "add", "implement", "create", "new" → feat
  - "fix", "repair", "resolve", "correct" → fix
  - "update", "refactor", "clean", "remove", "rename", "move" → chore
  - Default: chore
- Implemented `FormatCommitMessage(taskTitle, iterationID)` that creates conventional commit format
- Implemented `FormatCommitMessageWithType(commitType, taskTitle, iterationID)` for explicit type override
- Implemented `ParseConventionalCommit(message)` that parses commit type, subject, and body
- Implemented `ValidateCommitMessage(message)` for commit message validation
- Message format: `<type>: <title>\n\nRalph iteration: <iterationID>`

**Files touched:**
- `internal/git/commit.go` (new)
- `internal/git/commit_test.go` (new)

**Learnings:**
- Use `strings.Cut()` instead of `strings.Index()` + manual slicing for cleaner code (Go 1.18+)
- Keyword matching with `strings.HasPrefix` for case-insensitive detection (after lowercasing)
- Conventional commit parsing: split on first colon, then handle optional body after blank line
- TDD approach: wrote 37 tests first covering commit type string conversion, type inference from keywords, message formatting with/without iteration ID, conventional commit parsing, and validation edge cases

**Outcome**: Success - all 59 git tests pass, `go build ./...`, `go test ./...`, and `golangci-lint run` succeed

### 2026-01-16: memory-manager-progress (Progress File Management)

**What changed:**
- Created `internal/memory` package for progress and memory file management
- Implemented `ProgressFile` struct with `NewProgressFile(path)` constructor
- Implemented `IterationEntry` struct for representing iteration log entries
- Implemented `SizeOptions` struct for configuring max size limits
- `Init(featureName, parentTaskID)` creates progress file with standard header (feature name, parent task, start date, Codebase Patterns section, Iteration Log section)
- `AppendIteration(entry)` appends formatted iteration entries to the file
- `IterationEntry.Format(timestamp)` formats entry as markdown with what changed, files touched (optional), learnings (optional), and outcome
- `GetCodebasePatterns()` extracts the Codebase Patterns section using `strings.Cut()`
- `UpdateCodebasePatterns(patterns)` replaces the patterns section preserving other sections
- `EnforceMaxSize(opts)` prunes old iteration entries when file exceeds line limit, keeping most recent entries
- `Exists()` and `Path()` helper methods

**Files touched:**
- `internal/memory/progress.go` (new)
- `internal/memory/progress_test.go` (new)

**Learnings:**
- Use `strings.Cut()` for extracting sections between markers - cleaner than manual Index + slicing
- Use `fmt.Fprintf(&sb, ...)` instead of `sb.WriteString(fmt.Sprintf(...))` for better linter compliance
- Use built-in `min()` and `max()` functions (Go 1.21+) instead of if-else statements
- Pruning iteration entries requires parsing entry boundaries (lines starting with "### ") and keeping entries from the end to preserve most recent work
- TDD approach: wrote 21 comprehensive tests covering init, append, patterns extraction/update, size enforcement, and formatting

**Outcome**: Success - all 21 memory tests pass, `go build ./...`, `go test ./...`, and `golangci-lint run` succeed

### 2026-01-16: memory-manager-archive (Progress Archive)

**What changed:**
- Implemented `ProgressArchive` struct in `internal/memory/archive.go` for archiving progress files
- `NewProgressArchive(archiveDir)` constructor creates archive manager with configurable directory
- `Archive(progressPath)` moves progress file to archive directory with timestamped filename
- Generates unique archive filenames with format: `progress-YYYYMMDD-HHMMSS.md`
- Handles filename collisions by appending counter suffix (e.g., `progress-20260116-143022-1.md`)
- `ListArchives()` returns list of archived files, sorted newest first
- `ArchiveDir()` getter for the archive directory path
- Creates archive directory if it doesn't exist

**Files touched:**
- `internal/memory/archive.go` (new)
- `internal/memory/archive_test.go` (new)

**Learnings:**
- Second-precision timestamps can cause collisions in rapid-fire tests; handle by appending counter suffix
- Use `os.IsNotExist(err)` for file existence checks rather than `errors.Is(err, os.ErrNotExist)` when working with `os.Stat` errors
- Archive pattern: read source, write to destination, then remove source (safer than rename which may fail across filesystems)
- TDD approach: wrote 17 tests first covering archive creation, timestamp in filename, directory creation, error handling, multiple archives, and listing

**Outcome**: Success - all 35 memory tests pass, `go build ./...`, `go test ./...`, and `golangci-lint run` succeed

### 2026-01-16: loop-controller-iteration-record (Iteration Record Model)

**What changed:**
- Created `internal/loop` package for iteration orchestration
- Implemented `IterationRecord` struct with all required fields: IterationID, TaskID, StartTime, EndTime, ClaudeInvocation, BaseCommit, ResultCommit, VerificationOutputs, FilesChanged, Outcome, Feedback
- Implemented `IterationOutcome` type with values: success, failed, budget_exceeded, blocked
- Implemented `ClaudeInvocationMeta` struct for Claude Code invocation metadata (Command, Model, SessionID, TotalCostUSD, InputTokens, OutputTokens)
- Implemented `VerificationOutput` struct mirroring verifier package results
- Added helper functions: `NewIterationRecord(taskID)`, `GenerateIterationID()`, `SaveRecord(dir, record)`, `LoadRecord(path)`
- Added methods: `Duration()`, `Complete(outcome)`, `SetFeedback(feedback)`, `AllPassed()`
- Full JSON serialization support for logging and persistence

**Files touched:**
- `internal/loop/record.go` (new)
- `internal/loop/record_test.go` (new)
- `go.mod` / `go.sum` (added github.com/google/uuid dependency)

**Learnings:**
- Use `github.com/google/uuid` for generating unique IDs; slice first 8 characters for readable iteration IDs
- `SaveRecord` creates directory if not exists using `os.MkdirAll`
- TDD approach: wrote 26 tests first covering outcome validity, record defaults, all fields, JSON serialization, duration calculation, completion, feedback, save/load functionality, and verification pass aggregation

**Outcome**: Success - all 26 loop tests pass, `go build ./...`, `go test ./...`, and `golangci-lint run` succeed

### 2026-01-16: loop-controller-budget (Budget Tracking)

**What changed:**
- Implemented budget tracking in `internal/loop/budget.go`
- Created `BudgetLimits` struct with configurable limits: MaxIterations, MaxTimeMinutes, MaxCostUSD, MaxMinutesPerIteration
- Created `BudgetState` struct for tracking consumption: Iterations, TotalCostUSD, StartTime
- Created `BudgetStatus` struct for check results: CanContinue, Reason, ReasonCode
- Implemented `BudgetTracker` with methods: `NewBudgetTracker()`, `RecordIteration()`, `CheckBudget()`, `GetState()`, `SetState()`, `Reset()`, `ElapsedTime()`
- Implemented `SaveBudget(path, state)` and `LoadBudget(path)` for persistence to .ralph/state/budget.json
- Created `BudgetReasonCode` type with values: none, iterations, time, cost
- `DefaultBudgetLimits()` returns sensible defaults (50 iterations, 20 min per iteration, unlimited time/cost)
- Zero values for limits mean unlimited - allows flexible configuration
- Budget checks evaluate in priority order: iterations, time, cost

**Files touched:**
- `internal/loop/budget.go` (new)
- `internal/loop/budget_test.go` (new)

**Learnings:**
- Zero values for limits allow "unlimited" behavior without special sentinel values
- Start time is set on first RecordIteration() call, not on construction, allowing for lazy initialization
- LoadBudget returns empty state (not error) when file doesn't exist - enables clean first-run behavior
- Budget status includes both human-readable reason and machine-readable reason code for programmatic handling
- TDD approach: wrote 25 tests first covering defaults, limits, tracker operations, persistence, and edge cases

**Outcome**: Success - all 51 loop tests pass, `go build ./...`, `go test ./...`, and `golangci-lint run` succeed

### 2026-01-16: loop-controller-gutter (Gutter Detection)

**What changed:**
- Implemented gutter detection in `internal/loop/gutter.go`
- Created `GutterReason` type with values: none, repeated_failure, file_churn, oscillation
- Created `GutterConfig` struct with configurable thresholds: MaxSameFailure, MaxChurnIterations, ChurnThreshold
- Created `GutterStatus` struct for detection results: InGutter, Reason, Description
- Created `GutterState` struct for persistence: FailureSignatures, FileChanges
- Implemented `GutterDetector` with methods: `NewGutterDetector()`, `RecordIteration()`, `Check()`, `Reset()`, `GetState()`, `SetState()`
- Implemented `ComputeFailureSignature()` that hashes verification failure outputs using SHA256
- Repeated failure detection: tracks failure signature occurrences and triggers when threshold exceeded
- File churn detection: tracks files changed across recent iterations and triggers when same file modified repeatedly
- `DefaultGutterConfig()` returns sensible defaults (3 same failures, 5 churn iterations, 3 churn threshold)
- Zero values for thresholds disable that detection type

**Files touched:**
- `internal/loop/gutter.go` (new)
- `internal/loop/gutter_test.go` (new)

**Learnings:**
- Use SHA256 hash of sorted failure outputs for consistent signature computation
- Churn detection needs to track file changes across a sliding window of recent iterations (not all time)
- Copy maps and slices in GetState/SetState to prevent external mutation of internal state
- TDD approach: wrote 24 tests first covering GutterReason validity, config defaults, detector creation, failure signatures, repeated failure detection, file churn detection, reset, state persistence, and disabled detection

**Outcome**: Success - all 69 loop tests pass, `go build ./...`, `go test ./...`, and `golangci-lint run` succeed

### 2026-01-16: loop-controller-orchestrator (Main Loop Orchestrator)

**What changed:**
- Implemented `Controller` struct in `internal/loop/controller.go` that orchestrates the main iteration loop
- Created `ControllerDeps` struct for dependency injection: TaskStore, Claude Runner, Verifier, Git Manager, ProgressFile
- Created `RunLoopOutcome` type with values: completed, blocked, budget_exceeded, gutter_detected, paused, error
- Created `RunResult` struct containing: Outcome, Message, IterationsRun, CompletedTasks, FailedTasks, Records, TotalCostUSD, ElapsedTime
- Implemented `NewController(deps)` constructor that initializes budget and gutter trackers with defaults
- Implemented `RunLoop(ctx, parentTaskID)` main method that:
  - Checks context cancellation, budget limits, and gutter conditions before each iteration
  - Uses selector.SelectNext to pick the next ready leaf task
  - Runs single iteration: invoke Claude, check for changes, run verification, commit on success
  - Updates task status (in_progress → completed/open)
  - Records iteration in budget and gutter trackers
  - Continues until all tasks completed, blocked, or limits reached
- Implemented `RunOnce(ctx, parentTaskID)` for single iteration mode (--once flag)
- Implemented `runIteration(ctx, task)` that handles:
  - Getting base commit, invoking Claude, checking for changes
  - Running verification commands, formatting feedback on failure
  - Committing changes with formatted message, updating progress file
- Implemented `buildPrompt(task)` to construct Claude prompts with task details and instructions
- Implemented `GetSummary(ctx, parentTaskID)` for status reporting with task counts
- Added `SetBudgetLimits()` and `SetGutterConfig()` for configuration

**Files touched:**
- `internal/loop/controller.go` (new)
- `internal/loop/controller_test.go` (new)

**Learnings:**
- Need to disable gutter detection in tests when testing multi-task scenarios to avoid false positives from same files being changed
- When no tasks exist for a parent, returning "completed" (vacuously true) is reasonable behavior
- Use `dynamicGitManager` mock pattern when different return values are needed per call
- Controller integrates all core components: TaskStore, Selector, ClaudeRunner, Verifier, GitManager, MemoryManager
- TDD approach: wrote 20 tests first covering outcome validity, constructor, loop scenarios (no tasks, success, verification failure, budget exceeded, context cancellation, Claude error, no changes, multiple tasks), run once, summary, dependency graph ordering

**Outcome**: Success - all 95 loop tests pass, `go build ./...`, `go test ./...`, and `golangci-lint run` succeed

### 2026-01-16: reporter-status (Status Generation)

**What changed:**
- Created `internal/reporter` package for status display and report generation
- Implemented `TaskCounts` struct for aggregating task counts (total, completed, ready, blocked, failed, skipped)
- Implemented `LastIterationInfo` struct for summarizing last iteration (iterationID, taskID, taskTitle, outcome, endTime, logPath)
- Implemented `Status` struct combining parent task ID, counts, next task, and last iteration info
- Implemented `StatusGenerator` struct with `NewStatusGenerator(store, logsDir)` constructor
- Implemented `GetStatus(parentTaskID)` method that:
  - Gathers all descendant tasks under parent via BFS traversal
  - Counts tasks by status (completed, blocked, failed, skipped)
  - Uses selector to count ready leaves and find next task
  - Loads last iteration record from logs directory
- Implemented `FindLatestIterationRecord(logsDir)` that scans iteration JSON files and returns the most recent by end time
- Implemented `FormatStatus(status)` for CLI display with markdown-formatted output

**Files touched:**
- `internal/reporter/status.go` (new)
- `internal/reporter/status_test.go` (new)

**Learnings:**
- Reuse selector package's `GetReadyLeaves` and `SelectNext` for consistent ready task logic
- Parse iteration log files from `.ralph/logs/` directory to find latest iteration by comparing EndTime
- BFS traversal with parent-to-children map efficiently gathers all descendants
- TDD approach: wrote 22 tests first covering struct defaults, all fields, generator creation, various task scenarios (no tasks, blocked, skipped, dependencies, deep hierarchy), last iteration loading (single/multiple files), and formatting

**Outcome**: Success - all 22 reporter tests pass, `go build ./...`, `go test ./...`, and `golangci-lint run` succeed

### 2026-01-16: reporter-report (Feature Report Generation)

**What changed:**
- Implemented `Report` struct in `internal/reporter/report.go` with all required fields: ParentTaskID, FeatureName, Commits, CompletedTasks, BlockedTasks, FailedTasks, SkippedTasks, TotalIterations, TotalCostUSD, TotalDuration, StartTime, EndTime
- Implemented helper structs: `CommitInfo`, `TaskSummary`, `BlockedTaskSummary`
- Implemented `ReportGenerator` with `NewReportGenerator(store, logsDir)` constructor
- Implemented `GenerateReport(parentTaskID)` method that:
  - Gathers all descendant tasks under the parent via BFS traversal
  - Categorizes tasks by status (completed, blocked, failed, skipped)
  - Loads all iteration records to calculate totals (iterations, cost, duration)
  - Extracts commits from successful iteration records
  - Computes time range from earliest start to latest end
  - Determines blocked reasons by analyzing dependencies
- Implemented `LoadAllIterationRecords(logsDir)` function that scans iteration JSON files
- Implemented `FormatReport(report)` for CLI display with markdown formatting
- Added `formatDuration(d)` helper for human-readable duration display

**Files touched:**
- `internal/reporter/report.go` (new)
- `internal/reporter/report_test.go` (new)

**Learnings:**
- Reuse `gatherDescendants()` pattern from status.go for BFS traversal
- Time comparison in tests requires `WithinDuration` instead of `Equal` due to monotonic clock differences when times are loaded from JSON
- Blocked reason computation: check dependsOn tasks status, report which dependencies are incomplete
- SkippedTasks should be tracked separately from completed/failed for complete reporting
- TDD approach: wrote 23 tests first covering struct defaults, all fields, generator creation, various task scenarios (no tasks, completed, blocked, failed, skipped, mixed), iteration records, commits extraction, time range, and formatting

**Outcome**: Success - all 45 reporter tests pass, `go build ./...`, `go test ./...`, and `golangci-lint run` succeed

### 2026-01-16: cli-init (ralph init Command)

**What changed:**
- Implemented `ralph init` command in `cmd/init.go`
- Added `--parent <id>` flag to specify parent task ID directly
- Added `--search <term>` flag to find parent task by title substring (case-insensitive)
- Creates `.ralph/` directory structure using `state.EnsureRalphDir`
- Writes parent task ID to `.ralph/parent-task-id` file
- Validates parent task exists in task store
- Validates task graph has no cycles using `selector.BuildGraph` and `graph.DetectCycle`
- Validates at least one ready leaf task exists under the parent
- Updated `root_test.go` to remove `init` from stub commands list

**Files touched:**
- `cmd/init.go` (new)
- `cmd/init_test.go` (new)
- `cmd/root.go` (removed stub newInitCmd function)
- `cmd/root_test.go` (updated stub commands list)

**Learnings:**
- Use `cmd.OutOrStdout()` for writing output in Cobra commands to allow capturing in tests
- Must handle `fmt.Fprintf` return values explicitly to pass errcheck linter (`_, _ = fmt.Fprintf(...)`)
- Tests need to change working directory (`os.Chdir`) to temp dir since config and state operations are relative to cwd
- Reuse `selector.GetReadyLeaves` for consistent ready task logic, then filter to descendants of parent
- BFS traversal with parent-to-children map efficiently gathers all descendants

**Outcome**: Success - all 31 cmd tests pass (16 init tests + 15 root tests), `go build ./...`, `go test ./...`, and `golangci-lint run` succeed
