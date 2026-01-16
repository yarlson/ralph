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
- Implemented `Task` struct with all required fields: ID, Title, Description, ParentID (\*string), DependsOn, Status, Acceptance, Verify, Labels, CreatedAt, UpdatedAt
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
- Converts YAML field names (parentId) to internal model (\*string ParentID)

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
- `VerifyTask` takes commands directly ([][]string) rather than \*Task to avoid coupling verifier package to taskstore package - keeps interface clean and testable

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

### 2026-01-16: cli-run (ralph run Command)

**What changed:**

- Implemented `ralph run` command in `cmd/run.go`
- Added `--once` flag for single iteration mode (invokes `controller.RunOnce`)
- Added `--max-iterations N` flag to override config limit (0 uses config default)
- Reads parent task ID from `.ralph/parent-task-id` file (requires `ralph init` first)
- Creates all necessary dependencies: TaskStore, ClaudeRunner, Verifier, GitManager, ProgressFile
- Invokes `LoopController.RunLoop` or `RunOnce` based on flags
- Configures budget limits and gutter detection from config
- Implements graceful shutdown via signal handling (SIGINT, SIGTERM) with context cancellation
- Displays formatted run result with outcome, message, iterations, completed/failed tasks, cost, and elapsed time
- Removed stub `newRunCmd` from root.go, updated root_test.go to exclude `run` from stub commands

**Files touched:**

- `cmd/run.go` (new)
- `cmd/run_test.go` (new)
- `cmd/root.go` (removed stub newRunCmd function)
- `cmd/root_test.go` (removed "run" from stub commands list)

**Learnings:**

- Use `signal.Notify` with `context.WithCancel` for graceful shutdown pattern
- `NewSubprocessRunner` takes a single string command, not a slice - extract first element from config
- Use `cmd.OutOrStdout()` and `cmd.ErrOrStderr()` for proper output handling in tests
- Tests change working directory with `os.Chdir` to temp dir for isolation
- TDD approach: wrote 14 tests covering command structure, flags, error cases (no parent-task-id, nonexistent parent), and result formatting

**Outcome**: Success - all 39 cmd tests pass, `go build ./...` and `go test ./...` succeed

### 2026-01-16: cli-status (ralph status Command)

**What changed:**

- Implemented `ralph status` command in `cmd/status.go`
- Reads parent task ID from `.ralph/parent-task-id` file (requires `ralph init` first)
- Uses `reporter.StatusGenerator` to gather status information
- Displays task counts (total, completed, ready, blocked, failed, skipped)
- Shows next selected task with ID and title
- Shows last iteration info (ID, task, outcome, time, log path) if available
- Outputs formatted status using `reporter.FormatStatus()`
- Removed stub `newStatusCmd` from root.go, updated root_test.go to exclude `status` from stub commands

**Files touched:**

- `cmd/status.go` (new)
- `cmd/status_test.go` (new)
- `cmd/root.go` (removed stub newStatusCmd function)
- `cmd/root_test.go` (removed "status" from stub commands list)

**Learnings:**

- Reuse existing `reporter.StatusGenerator` and `reporter.FormatStatus` for consistent status display
- Follow same pattern as init/run commands: load config, read parent-task-id, validate parent task exists
- TDD approach: wrote 12 tests covering command structure, error cases (no parent-task-id, nonexistent parent), various task count scenarios, next task display, last iteration info, and graceful handling of missing data

**Outcome**: Success - all 51 cmd tests pass, `go build ./...` and `go test ./...` succeed

### 2026-01-16: cli-pause-resume (ralph pause/resume Commands)

**What changed:**

- Implemented `ralph pause` command in `cmd/pause.go` that sets the paused flag
- Implemented `ralph resume` command in `cmd/resume.go` that clears the paused flag
- Added `IsPaused(root)`, `SetPaused(root, paused)`, and `PausedFilePath(root)` functions to `internal/state/state.go`
- Paused state stored as `.ralph/state/paused` file (presence = paused, absence = not paused)
- Updated `cmd/run.go` to check paused state before running - returns error if paused
- Removed stub `newPauseCmd()` and `newResumeCmd()` from `cmd/root.go`
- Both commands handle edge cases: pause when already paused (shows message), resume when not paused (shows message)

**Files touched:**

- `internal/state/state.go` (added PausedFile constant, IsPaused, SetPaused, PausedFilePath functions)
- `internal/state/state_test.go` (added 14 new tests for pause functionality)
- `cmd/pause.go` (new)
- `cmd/pause_test.go` (new)
- `cmd/resume.go` (new)
- `cmd/resume_test.go` (new)
- `cmd/run.go` (added paused state check)
- `cmd/run_test.go` (added TestRunCmd_RespectsPausedState, cleaned up unused mocks)
- `cmd/root.go` (removed pause/resume stub commands)
- `cmd/root_test.go` (removed pause/resume from stub commands list)

**Learnings:**

- Simple file-based flag pattern (file existence = state) is clean and atomic - no parsing needed
- Using `errors.Is(err, os.ErrNotExist)` is the idiomatic way to check for missing file
- The run command checks paused state early (before loading config) to fail fast
- TDD approach: wrote 13 tests covering command structure, error cases, setting/clearing flag, and run respecting paused state

**Outcome**: Success - all tests pass (15 state tests, 66 cmd tests), `go build ./...`, `go test ./...`, and `golangci-lint run` succeed

### 2026-01-16: cli-retry-skip (ralph retry/skip Commands)

**What changed:**

- Implemented `ralph retry` command in `cmd/retry.go`
- Implemented `ralph skip` command in `cmd/skip.go`
- `retry --task <id>` resets task status from failed/blocked/in_progress to open
- `retry --feedback <text>` saves feedback to `.ralph/state/feedback-<task-id>.txt`
- `skip --task <id>` marks task as skipped
- `skip --reason <text>` saves skip reason to `.ralph/state/skip-reason-<task-id>.txt`
- Both commands validate task exists and is in appropriate state
- Cannot retry completed or skipped tasks
- Cannot skip completed tasks
- Removed stub `newRetryCmd()` and `newSkipCmd()` from `cmd/root.go`

**Files touched:**

- `cmd/retry.go` (new)
- `cmd/retry_test.go` (new)
- `cmd/skip.go` (new)
- `cmd/skip_test.go` (new)
- `cmd/root.go` (removed retry/skip stub commands)
- `cmd/root_test.go` (removed retry/skip from stub commands list)

**Learnings:**

- Reuse existing `taskstore` package for task retrieval and status updates
- Use `state.StateDirPath()` for feedback/reason file storage
- Tasks can be in states: open, in_progress, completed, blocked, failed, skipped
- Retry allows: failed, blocked, in_progress -> open
- Skip allows: open, in_progress, blocked, failed -> skipped
- Handle edge cases: already open (for retry), already skipped (for skip)
- TDD approach: wrote 20 tests first covering command structure, flags, error cases, state transitions, and feedback/reason file persistence

**Outcome**: Success - all cmd tests pass, `go build ./...`, `go test ./...`, and `golangci-lint run` succeed

### 2026-01-16: cli-report (ralph report Command)

**What changed:**

- Implemented `ralph report` command in `cmd/report.go`
- Added `--output, -o` flag to write report to a file instead of stdout
- Uses `reporter.ReportGenerator` to generate the feature report
- Uses `reporter.FormatReport` to format the report for display
- Creates output directory if it doesn't exist when writing to file
- Displays success message with file path when writing to file
- Removed stub `newReportCmd` from `cmd/root.go` (all CLI commands now implemented)

**Files touched:**

- `cmd/report.go` (new)
- `cmd/report_test.go` (new)
- `cmd/root.go` (removed stub newReportCmd, cleaned up unused imports)
- `cmd/root_test.go` (removed stub commands test section)

**Learnings:**

- Follow same pattern as init/run/status commands: load config, read parent-task-id, validate parent task exists
- Reuse existing `reporter.ReportGenerator` and `reporter.FormatReport` for consistent report generation
- When writing to file, use `os.MkdirAll` on the directory path to ensure parent directories exist
- Tasks require `CreatedAt` and `UpdatedAt` fields in tests - add `time.Now()` to test fixtures
- TDD approach: wrote 10 tests first covering command structure, flags, error cases, report display, blocked tasks, iteration stats, file output, directory creation, and empty reports

**Outcome**: Success - all tests pass, `go build ./...` and `go test ./...` succeed

### 2026-01-16: prompt-packager-iteration (Iteration Prompt Builder)

**What changed:**

- Created `internal/prompt` package for prompt packaging for Claude Code iterations
- Implemented `IterationContext` struct with all required context fields: Task, CodebasePatterns, DiffStat, ChangedFiles, FailureOutput, UserFeedback, IsRetry
- Implemented `SizeOptions` struct with configurable size limits: MaxPromptBytes, MaxPatternsBytes, MaxDiffBytes, MaxFailureBytes
- Implemented `Builder` struct with `NewBuilder(opts)` constructor
- Implemented `BuildSystemPrompt()` that returns harness instructions for Claude (role, rules, completion criteria)
- Implemented `BuildUserPrompt(ctx)` that builds the user prompt with:
  - Task title and description
  - Acceptance criteria (if present)
  - Verification commands (if present)
  - Codebase Patterns (if present, with size truncation)
  - Git status with diff stat and changed files (if present, with size truncation)
  - Instructions section
- Implemented `Build(ctx)` convenience method returning both system and user prompts
- Implemented `truncateWithMarker()` helper that truncates strings to max bytes with "... [truncated]" marker
- Added `DefaultSizeOptions()` with sensible defaults (8KB prompt, 2KB patterns, 1KB diff, 2KB failure)
- Added `Validate()` method on SizeOptions to check for negative values

**Files touched:**

- `internal/prompt/iteration.go` (new)
- `internal/prompt/iteration_test.go` (new)

**Learnings:**

- Use `strings.Builder` with `fmt.Fprintf(&sb, ...)` for efficient string building with linter compliance
- Zero value for size limits means unlimited (no truncation) - useful for testing without limits
- Separate system prompt (harness instructions) from user prompt (task context) for clear Claude invocation
- TDD approach: wrote 20 tests first covering struct defaults, all fields, validation, builder construction, system prompt content, user prompt with minimal/full task, size limits, empty sections, and convenience Build method

**Outcome**: Success - all 20 prompt tests pass, `go build ./...`, `go test ./...`, and `golangci-lint run` succeed

### 2026-01-16: prompt-packager-retry (Retry Prompt Builder)

**What changed:**

- Implemented `RetryContext` struct in `internal/prompt/retry.go` with fields: Task, FailureOutput, FailureSignature, UserFeedback, AttemptNumber
- Implemented `BuildRetrySystemPrompt()` method that returns retry-specific harness instructions emphasizing fix-only approach
- Implemented `BuildRetryPrompt(ctx)` method that builds the retry user prompt with:
  - Retry header with task title and attempt number (if > 0)
  - Fix-only directive emphasized at the top
  - Verification failure section with trimmed output (uses `truncateWithMarker`)
  - Failure signature for debugging/tracking context
  - User feedback section (if provided)
  - Task description and acceptance criteria for reference
  - Verification commands to run
  - Fix-focused instructions
- Implemented `BuildRetry(ctx)` convenience method returning both system and user prompts
- Reuses existing `truncateWithMarker()` from iteration.go for failure output truncation

**Files touched:**

- `internal/prompt/retry.go` (new)
- `internal/prompt/retry_test.go` (new)

**Learnings:**

- Retry prompts should have stronger "fix-only" directives than initial prompts - repeated emphasis helps focus Claude on minimal fixes
- Separate `BuildRetrySystemPrompt()` from `BuildSystemPrompt()` allows different harness instructions for retries vs initial attempts
- Include attempt number in retry prompts to give context on how many times the task has been attempted
- Zero value for AttemptNumber (0) means not set - don't display "attempt 0" in output
- TDD approach: wrote 16 tests first covering nil task, minimal context, failure output, failure signature, user feedback, all context, fix-only directive, truncation, attempt number, and system prompt differences

**Outcome**: Success - all 35 prompt tests pass, `go build ./...`, `go test ./...`, and `golangci-lint run` succeed

### 2026-01-16: MVP Completion (All Tasks Completed)

**What changed:**

- All 37 leaf tasks have been completed
- All 12 container/parent tasks marked as completed in tasks.yaml
- Ralph Wiggum Loop Harness MVP is now fully implemented

**Summary of completed components:**

1. **Project Setup**: Go module, Cobra CLI skeleton, configuration loading, .ralph directory structure
2. **Task Store**: Task model, Store interface, LocalStore implementation, YAML import
3. **Selector**: Dependency graph builder, ready task computation, next task selection with area preference
4. **Claude Runner**: Runner interface, NDJSON parser, subprocess execution, session state management
5. **Verifier**: Verifier interface, command runner, output trimmer
6. **Git Manager**: Manager interface, shell implementation, commit message templates
7. **Memory Manager**: Progress file management, progress archiving
8. **Loop Controller**: Iteration records, budget tracking, gutter detection, main loop orchestrator
9. **Reporter**: Status generation, feature report generation
10. **CLI Commands**: init, run, status, pause, resume, retry, skip, report
11. **Prompt Packager**: Iteration prompt builder, retry prompt builder

**Files touched:**

- `tasks.yaml` (marked all container tasks as completed)

**Learnings:**

- Container tasks in task hierarchy are organizational groupings - only leaf tasks are executable
- When all leaf tasks under a container are completed, the container should be marked completed
- The Ralph harness is designed to execute only leaf tasks, following PRD section 8.2

**Outcome**: MVP Complete - all tests pass, all tasks completed

### 2026-01-16: ralph-align-config-flag (Honor --config Flag)

**What changed:**

- Added `GetConfigFile()` function in `cmd/root.go` to expose the cfgFile flag to subcommands
- Implemented `LoadConfigFromPath(configPath)` in `internal/config/config.go` to load config from a specific file path
- Implemented `LoadConfigWithFile(workDir, configFile)` helper that uses configFile if provided, otherwise falls back to default LoadConfig
- Updated all commands (init, run, status, pause, resume, retry, skip, report) to use `config.LoadConfigWithFile(workDir, GetConfigFile())`
- `LoadConfigFromPath` checks if file exists first and returns defaults if not found (handles missing files gracefully)
- Added comprehensive tests for `LoadConfigFromPath` and `LoadConfigWithFile` covering valid files, non-existent files, invalid YAML, and fallback behavior

**Files touched:**

- `cmd/root.go` (added GetConfigFile function)
- `internal/config/config.go` (added LoadConfigFromPath and LoadConfigWithFile functions, added os import)
- `cmd/init.go`, `cmd/run.go`, `cmd/status.go`, `cmd/report.go`, `cmd/retry.go`, `cmd/skip.go` (updated to use LoadConfigWithFile)
- `internal/config/config_test.go` (added 6 new tests)

**Learnings:**

- When using `viper.SetConfigFile(path)`, must check file existence first with `os.Stat()` because Viper returns an error for missing files (unlike `viper.AddConfigPath()` which gracefully handles missing files)
- Using an empty string for configFile parameter triggers the fallback to default behavior - clean pattern for optional config file override
- The `GetConfigFile()` helper makes the cfgFile accessible to all subcommands without needing to pass it through context or global mutable state
- All config loading now goes through `LoadConfigWithFile` which centralizes the logic and ensures consistent behavior across all commands

**Outcome**: Success - all tests pass (11 config tests including 6 new ones), go build succeeds, acceptance criteria met

### 2026-01-16: ralph-align-claude-args (Honor claude.command and claude.args)

**What changed:**

- Modified `internal/claude/exec.go` to support base arguments that are prepended before Claude-specific flags
- Added `baseArgs []string` field to `SubprocessRunner` struct
- Implemented `WithBaseArgs(baseArgs []string)` method on `SubprocessRunner` for fluent configuration
- Modified `buildArgs(req ClaudeRequest, baseArgs []string)` to accept and prepend base arguments
- Updated `cmd/run.go` to parse both `config.Claude.Command` and `config.Claude.Args`:
  - If `command` has multiple parts (e.g., `["claude", "code"]`), the first becomes the binary and rest become base args
  - All elements from `config.Claude.Args` are appended to base args
- Updated all existing `buildArgs()` test calls to pass empty `[]string{}` for backward compatibility
- Added 3 new tests: `TestBuildArgs_WithBaseArgs`, `TestBuildArgs_WithBaseArgsAndOtherOptions`, `TestSubprocessRunner_WithBaseArgs`

**Files touched:**

- `internal/claude/exec.go` (modified SubprocessRunner struct, added WithBaseArgs method, modified buildArgs signature)
- `internal/claude/exec_test.go` (updated 6 existing tests, added 3 new tests)
- `cmd/run.go` (modified Claude runner creation to use config.Claude.Command and config.Claude.Args)

**Learnings:**

- Using a fluent builder pattern (`WithBaseArgs()`) allows clean configuration without changing the constructor signature
- Base args must be prepended before Claude-specific flags (`--output-format`, `--system-prompt`, etc.) to maintain correct command structure
- Config's `command` field can serve dual purpose: first element as binary, rest as base args - clean pattern for commands like `["claude", "code"]`
- When modifying function signatures in production code, all test callers must be updated - use search/grep to find all call sites
- Empty slice `[]string{}` as default parameter value maintains backward compatibility for existing tests

**Outcome**: Success - all tests pass (52 claude tests including 3 new ones), acceptance criteria met: config command and args both used, empty args handled correctly, unit tests verify command construction

### 2026-01-16: ralph-align-task-status (Implement Task Status Transitions)

**What changed:**

- Added `AttemptNumber` field to `IterationRecord` struct to track retry attempts
- Added `MaxRetries` field to `LoopConfig` with default value of 2
- Added `maxRetries` and `taskAttempts` fields to `Controller` struct for tracking per-task retry counts
- Implemented `SetMaxRetries()` method to configure max retries per task
- Implemented `handleTaskFailure()` helper method that sets task status to:
  - `failed` when attempts exceed maxRetries (max retries exhausted)
  - `open` when attempts are within limit (still have retries left)
- Modified `runIteration()` to:
  - Track attempt number at start of each iteration
  - Set attempt number in iteration record
  - Call `handleTaskFailure()` on all failure paths (Claude error, no changes, verification failure, commit failure)
  - Clear attempt counter on success
- Modified `RunLoop()` to set `blocked` status on gutter detection by finding in-progress task
- Updated `cmd/run.go` to call `SetMaxRetries()` with config value

**Files touched:**

- `internal/loop/record.go` (added AttemptNumber field)
- `internal/config/config.go` (added MaxRetries to LoopConfig, set default to 2)
- `internal/loop/controller.go` (added retry tracking, status transition logic)
- `cmd/run.go` (added SetMaxRetries call)
- `tasks.yaml` (marked ralph-align-task-status as completed)

**Learnings:**

- maxRetries represents number of retries allowed after initial attempt (maxRetries=2 means 3 total attempts)
- Use `>` comparison (not `>=`) when checking if attempts exceed maxRetries
- Tracking attempt count in Controller state allows per-task retry tracking without modifying Task model
- Clear attempt counter on success to reset for potential future failures
- Gutter detection should mark in-progress tasks as blocked, not just return outcome
- Task status transitions: open -> in_progress -> (on failure: open/failed based on retries OR on success: completed)

**Outcome**: Success - all tests pass, acceptance criteria met: tasks failing max retries marked failed, gutter detection sets blocked status, retry attempt tracking implemented

### 2026-01-16: ralph-align-logs-cmd (Add ralph logs Command)

**What changed:**

- Implemented `ralph logs` command in `cmd/logs.go`
- Added `--iteration <id>` flag to show specific iteration details
- Without flag, lists all available iterations sorted by time (newest first)
- Registered command in `cmd/root.go` (added between status and pause commands)
- Created comprehensive test suite with 13 tests in `cmd/logs_test.go`

**Files touched:**

- `cmd/logs.go` (new)
- `cmd/logs_test.go` (new)
- `cmd/root.go` (added newLogsCmd() registration)

**Learnings:**

- Use `os.Stat()` to check file existence before calling `loop.LoadRecord()` to provide clearer "not found" error messages instead of letting the file read error propagate
- When listing iterations, skip non-iteration files and invalid JSON files gracefully by continuing the loop rather than failing
- Sort iterations by `StartTime` for display (newest first) using `sort.Slice` with `After()` comparison
- Format iteration details with sections: header, timing, Claude info, git commits, files changed, verification results, feedback
- For failed verification output, show last 10 lines with truncation marker to keep output readable
- Reuse `loop.LoadRecord()` and iteration record structures instead of reimplementing JSON parsing
- Follow the CLI pattern from other commands: use `cmd.OutOrStdout()` for output to enable test capture

**Outcome**: Success - all tests pass, all acceptance criteria met: lists all iterations, shows specific iteration details, clear error on missing iteration, formatted for readability

### 2026-01-16: ralph-align-pause-check (Fix ralph pause Checking)

**What changed:**

- Added `WorkDir` field to `ControllerDeps` and `Controller` structs to enable pause state checking
- Implemented `checkPaused()` method in `Controller` that reads the pause flag file using `state.IsPaused()`
- Added pause check in main `RunLoop()` method before selecting next task (after context check, before budget check)
- Updated `cmd/run.go` to pass `workDir` to controller dependencies
- Loop now checks pause state between iterations and returns `RunOutcomePaused` with appropriate message
- Added comprehensive test `TestController_RunLoop_ChecksPauseBetweenIterations` that verifies pause detection between iterations

**Files touched:**

- `internal/loop/controller.go` (added WorkDir field, checkPaused method, pause check in loop)
- `internal/loop/controller_test.go` (added mockClaudeRunnerWithCallback, new test)
- `cmd/run.go` (updated ControllerDeps to include WorkDir)
- `tasks.yaml` (marked ralph-align-pause-check as completed)

**Learnings:**

- The pause check must occur **in the loop** itself, not just at the start of the run command, to enable mid-execution pausing
- Pause check should be positioned after context cancellation check but before budget/gutter checks for proper priority
- Controller needs access to `workDir` to call `state.IsPaused()` - this required adding WorkDir to both ControllerDeps and Controller structs
- When testing behavior that requires callback during execution, create a custom mock with callback function support
- The pause flag file pattern (file presence = paused) is simple and atomic - no parsing needed

**Outcome**: Success - all tests pass, acceptance criteria met: pause flag checked between iterations, loop stops after current iteration when paused, appropriate message shown

### 2026-01-16: ralph-align-iteration-prompt (Integrate Iteration Prompt Builder)

**What changed:**

- Modified `internal/loop/controller.go` to replace minimal prompt building with full `prompt.Builder` integration
- Changed `buildPrompt()` method signature to return both system and user prompts, accept context parameter
- Updated `buildPrompt()` to:
  - Extract codebase patterns from progress file using `progressFile.GetCodebasePatterns()`
  - Get git diff stat and changed files when uncommitted changes exist
  - Build `prompt.IterationContext` with task, patterns, diff stat, and changed files
  - Use `prompt.NewBuilder()` with default size options to build prompts
- Updated `runIteration()` to handle new `buildPrompt()` signature and set both `SystemPrompt` and `Prompt` in `ClaudeRequest`
- Added error handling for prompt building failures

**Files touched:**

- `internal/loop/controller.go` (modified buildPrompt signature and implementation, updated runIteration)
- `tasks.yaml` (marked ralph-align-iteration-prompt as completed)
- `.ralph/progress.md` (this entry)

**Learnings:**

- The prompt builder follows builder pattern with `NewBuilder(opts)` constructor accepting optional size options
- `BuildSystemPrompt()` returns harness instructions, `BuildUserPrompt()` returns task context - separation keeps concerns clean
- Git diff stat should only be retrieved when there are actual changes (`HasChanges()` check first) to avoid unnecessary git commands
- Progress file's `GetCodebasePatterns()` returns `(string, error)` - must handle both return values
- Default size options (8KB prompt, 2KB patterns, 1KB diff, 2KB failure) provide reasonable truncation limits
- The system prompt includes crucial directives: implement only this task, run verification, don't commit, update progress.md

**Outcome**: Success - all tests pass, prompt builder fully integrated, generated prompts include all PRD-specified context (task details, acceptance criteria, verification commands, codebase patterns, git status)

### 2026-01-16: ralph-align-retry-prompt (Integrate Retry Prompt Builder)

**What changed:**

- Modified `internal/loop/controller.go` to integrate retry prompt builder for failed iterations
- Split `buildPrompt()` into `buildInitialPrompt()` and `buildRetryPrompt()` methods
- `buildRetryPrompt()` detects retry attempts (attemptNumber > 1) and uses `prompt.BuildRetry()` instead of `prompt.Build()`
- Loads user feedback from `.ralph/state/feedback-<task-id>.txt` if it exists
- Loads most recent failed iteration record to get failure output and compute failure signature
- Trims failure output using `verifier.TrimOutputForFeedback()` with default trim options (100 lines, 8KB)
- Moved `LoadAllIterationRecords()` from `internal/reporter` to `internal/loop/record.go` for code reuse
- Updated `internal/reporter/report.go` and tests to use `loop.LoadAllIterationRecords()`

**Files touched:**

- `internal/loop/controller.go` (added buildRetryPrompt, split buildPrompt, added imports)
- `internal/loop/record.go` (added LoadAllIterationRecords function)
- `internal/reporter/report.go` (updated to use loop.LoadAllIterationRecords, removed duplicate function, removed unused imports)
- `internal/reporter/report_test.go` (updated references to loop.LoadAllIterationRecords)
- `tasks.yaml` (marked ralph-align-retry-prompt as completed)

**Learnings:**

- Retry detection is based on `attemptNumber` stored in `taskAttempts` map - when > 1, it's a retry
- User feedback from `ralph retry --feedback "..."` is stored in `.ralph/state/feedback-<task-id>.txt` and loaded for retry prompts
- Previous failure output is loaded from the most recent failed iteration record in logs directory
- `ComputeFailureSignature()` from gutter.go computes SHA256 hash of verification failures for tracking
- `verifier.TrimOutputForFeedback()` formats and trims failure output preserving the tail (where errors usually appear)
- Moving shared utility functions to the package where the types are defined (loop.IterationRecord → loop package) reduces coupling and improves reusability
- The retry prompt builder emphasizes fix-only approach and includes failure context, user feedback, and attempt number

**Outcome**: Success - all tests pass, retry prompts now include trimmed failure output, fix-only directive, user feedback when provided, and failure signature for tracking

### 2026-01-16: ralph-align-codebase-patterns (Extract and Use Codebase Patterns)

**What changed:**

- Verified that `GetCodebasePatterns()` is already fully implemented in `internal/memory/progress.go` (lines 146-158)
- Confirmed patterns extraction is integrated into the loop controller in `buildInitialPrompt()` (line 509 of `controller.go`)
- Verified patterns are included in Claude prompts via the prompt builder (lines 159-165 of `iteration.go`)
- Confirmed comprehensive test coverage exists in `progress_test.go` (line 192) with tests for extraction, empty sections, and missing files

**Files touched:**

- `tasks.yaml` (marked ralph-align-codebase-patterns as completed)
- `.ralph/progress.md` (this entry)

**Learnings:**

- The codebase patterns feature was already fully implemented during the iteration prompt builder integration task
- `GetCodebasePatterns()` uses `extractSection()` helper with `strings.Cut()` for clean section extraction between markers
- Patterns are optional in prompts - the builder checks `if ctx.CodebasePatterns != ""` before adding the section
- The prompt builder truncates patterns using `truncateWithMarker()` respecting `MaxPatternsBytes` size limit (default 2KB)
- All acceptance criteria were already met: extraction works, patterns included in prompts, empty sections handled gracefully, unit tests exist

**Outcome**: Success - task was already complete, all verification commands pass (go test ./internal/memory/...)

### 2026-01-16: ralph-align-retry-loop (Implement In-Iteration Retry Loop)

**What changed:**

- Added `MaxVerificationRetries` field to `LoopConfig` with default value of 2
- Added `maxVerificationRetries` field to `Controller` struct
- Implemented `SetMaxVerificationRetries()` method for configuration
- Modified `runIteration()` to implement in-iteration verification retry loop:
  - After verification fails, builds retry prompt with failure context using `buildRetryPromptForVerificationFailure()`
  - Invokes Claude again with `Continue: true` to fix in same session
  - Re-runs verification after each fix attempt
  - Repeats up to `maxVerificationRetries` times (default 2)
  - Accumulates Claude costs and token usage across all retry attempts within iteration
  - Updates changed files list after each retry
- Implemented `buildRetryPromptForVerificationFailure()` helper method that:
  - Loads user feedback from `.ralph/state/feedback-<task-id>.txt` if exists
  - Computes failure signature from current verification results
  - Trims failure output using `verifier.TrimOutputForFeedback()`
  - Builds retry context and generates retry prompts
- Updated `cmd/run.go` to call `SetMaxVerificationRetries()` from config
- Config default set to `loop.max_verification_retries: 2`

**Files touched:**

- `internal/config/config.go` (added MaxVerificationRetries field and default)
- `internal/loop/controller.go` (modified runIteration, added retry loop logic, added buildRetryPromptForVerificationFailure method)
- `cmd/run.go` (added SetMaxVerificationRetries call)
- `tasks.yaml` (marked ralph-align-retry-loop as completed)
- `.ralph/progress.md` (this entry)

**Learnings:**

- In-iteration retries differ from cross-iteration retries: in-iteration retries use `Continue: true` to stay in same Claude session, while cross-iteration retries start fresh
- The `maxVerificationRetries` config controls how many fix attempts Claude gets within a single iteration after initial verification failure
- Need to accumulate costs and token usage across all Claude invocations within the iteration for accurate budget tracking
- The loop condition `verificationAttempt <= c.maxVerificationRetries+1` accounts for: 1 initial attempt + N retries
- Changed files list should be updated after each retry since Claude may modify additional files during fix attempts
- Retry prompt building for in-iteration failures uses current verification results rather than loading from previous iteration records
- The feature enables Claude to fix verification failures immediately without ending the iteration, improving success rate

**Outcome**: Success - all tests pass (go test ./...), feature fully implemented with config support, in-iteration retry loop functional

### 2026-01-16: ralph-align-config-verify (Use Config-Level Verification Commands)

**What changed:**

- Added `configVerifyCommands [][]string` field to Controller struct to hold global verification commands from config
- Implemented `SetConfigVerifyCommands(commands [][]string)` method to configure global verification commands
- Implemented `mergeVerificationCommands(taskVerify [][]string)` helper method that:
  - Returns task commands if no config commands are set
  - Returns config commands if no task commands are set
  - Merges both by prepending config commands before task commands when both are present
- Modified `runIteration()` to call `mergeVerificationCommands()` before running verification
- Updated `cmd/run.go` to call `controller.SetConfigVerifyCommands(cfg.Verification.Commands)` during controller configuration
- Added comprehensive tests: `TestController_MergeVerificationCommands_*`, `TestController_RunIteration_WithConfigVerifyCommands`, `TestController_RunIteration_ConfigVerifyFails`
- Enhanced `mockVerifier` to support `verifyFn` callback for tracking executed commands in tests

**Files touched:**

- `internal/loop/controller.go` (added configVerifyCommands field, SetConfigVerifyCommands method, mergeVerificationCommands helper)
- `cmd/run.go` (added SetConfigVerifyCommands call)
- `internal/loop/controller_test.go` (added verifyFn to mockVerifier, added 5 new tests)
- `tasks.yaml` (marked ralph-align-config-verify as completed)
- `.ralph/progress.md` (this entry)

**Learnings:**

- Config-level verification commands run before task-level commands - this allows global checks (typecheck, lint) to run before task-specific tests
- The merge order is important: config commands first, then task commands - ensures global checks like linting happen before running expensive tests
- Both config and task verification must pass for iteration to succeed - any failure in either set fails the iteration
- Config verification commands are optional - if not specified in config, only task-level verification runs
- The `mergeVerificationCommands` pattern uses Go's `make()` with capacity hint for efficient slice allocation: `make([][]string, 0, len(a)+len(b))`
- Tests need proper Claude response mocks with all required fields (SessionID, Model, Usage, TotalCostUSD) to avoid nil pointer dereferences

**Outcome**: Success - all acceptance criteria met: both config and task commands execute, all must pass, clear output showing which failed, config commands optional

### 2026-01-16: ralph-align-yaml-import (Add YAML Import CLI Command)

**What changed:**

- Implemented `ralph import` command in `cmd/import.go` with full YAML task import functionality
- Added `--overwrite` flag (documented but currently all imports update existing tasks by design)
- Command reads YAML file, validates all tasks, and imports them into the task store using `taskstore.ImportFromYAML`
- Displays import results with count of successfully imported tasks and detailed error messages for failed validations
- Registered command in `cmd/root.go` (added after `newInitCmd()`)
- Created comprehensive test suite with 10 tests in `cmd/import_test.go` covering:
  - Command structure and flags
  - No args error
  - File not found error
  - Invalid YAML parsing error
  - Valid YAML import
  - Validation errors with detailed reporting
  - Overwrite behavior (existing tasks updated)
  - Empty file handling
  - Partial import with mix of valid/invalid tasks

**Files touched:**

- `cmd/import.go` (new)
- `cmd/import_test.go` (new)
- `cmd/root.go` (added newImportCmd registration)
- `tasks.yaml` (marked ralph-align-yaml-import as completed)
- `.ralph/progress.md` (this entry)

**Learnings:**

- The existing `taskstore.ImportFromYAML` function already handles validation and reports errors with task IDs - perfect for CLI integration
- Config field for task store path is `cfg.Tasks.Path`, not `Directory`
- `NewLocalStore` returns `(*LocalStore, error)` - must handle error return
- Import behavior is not truly atomic (as per acceptance criteria) - valid tasks are imported, invalid tasks are skipped and reported. This is by design in the existing `ImportFromYAML` implementation
- The `--overwrite` flag is documented but currently has no effect since `ImportFromYAML` always updates existing tasks via `store.Save()`. This matches the actual behavior described in the implementation
- File existence check before calling ImportFromYAML provides clearer error messages ("file not found" vs "failed to read")
- Using `cmd.OutOrStdout()` for all output enables test capture and verification
- Following TDD: wrote 10 tests first, saw them fail, implemented command, all tests pass

**Outcome**: Success - all 10 import tests pass, all acceptance criteria met: imports tasks from YAML, validates and reports errors with task IDs, existing tasks are updated, command integrated into CLI

### 2026-01-16: ralph-align-iteration-timeout (Enforce Per-Iteration Time Limit)

**What changed:**

- Modified `internal/loop/controller.go` `runIteration()` method to create context with timeout from `MaxMinutesPerIteration` config
- Added timeout context creation at the beginning of `runIteration()`: if `MaxMinutesPerIteration > 0`, creates child context with timeout
- Replaced all `ctx` uses with `iterationCtx` throughout `runIteration()` method for consistent timeout enforcement
- Added timeout detection checks after every operation that could fail: Claude invocation, git operations, verification, commit
- When `iterationCtx.Err() != nil` is detected, iteration completes with `OutcomeBudgetExceeded` and feedback "Iteration timeout exceeded"
- Checks for both `context.DeadlineExceeded` and `context.Canceled` errors using `iterationCtx.Err() != nil`
- Added comprehensive tests: `TestController_RunIteration_TimeoutExceeded` and `TestController_RunLoop_IterationTimeoutFromConfig`
- Created `timeoutMockClaudeRunner` test mock that respects context cancellation

**Files touched:**

- `internal/loop/controller.go` (modified `runIteration()` method)
- `internal/loop/controller_test.go` (added 2 new tests and `timeoutMockClaudeRunner` mock)
- `tasks.yaml` (marked ralph-align-iteration-timeout as completed)
- `.ralph/progress.md` (this entry)

**Learnings:**

- Context timeout should be created at the start of `runIteration()` and passed to all operations (Claude, Git, Verifier) for consistent timeout enforcement
- Use `context.WithTimeout()` with duration from `time.Duration(MaxMinutesPerIteration) * time.Minute`
- Check `iterationCtx.Err() != nil` instead of specifically `context.DeadlineExceeded` to catch both deadline exceeded and manual cancellation
- Timeout handling should mark iteration as `OutcomeBudgetExceeded` (not `OutcomeFailed`) to distinguish timeout from other failures
- Test mocks should respect context cancellation by checking `ctx.Done()` in select statements
- The `defer cancel()` pattern ensures timeout context is cleaned up even if iteration completes before timeout
- Config default `MaxMinutesPerIteration: 20` (20 minutes) is already set in `DefaultBudgetLimits()`

**Outcome**: Success - all tests pass (go test ./...), per-iteration timeout enforcement implemented, Claude subprocess properly cancelled on timeout, iteration marked as budget_exceeded when timeout occurs

### 2026-01-16: ralph-align-task-linter (Implement Task Linter)

**What changed:**

- Created `internal/taskstore/linter.go` with comprehensive task validation functionality
- Implemented `LintTask(task)` function that validates individual tasks for required fields (description)
- Implemented `LintTaskWithWarnings(task)` that returns both errors and warnings (e.g., missing acceptance criteria)
- Implemented `LintTaskSet(tasks)` function that validates entire task graph:
  - Individual task validity (description required, valid status)
  - Parent ID validity (parent task must exist)
  - Dependency existence (all dependsOn tasks must exist)
  - Dependency cycle detection using DFS with coloring
  - Leaf task verification commands (leaf tasks must have verify commands)
- Created `LintResult`, `LintError`, and `LintWarning` types for structured error reporting
- Implemented `detectDependencyCycle()` helper using DFS algorithm (white/gray/black coloring)
- Implemented `isLeafTask()` helper to identify tasks with no children
- Integrated linter into `cmd/init.go` - validates all tasks before allowing init
- Integrated linter into `cmd/import.go` - validates imported tasks after import, displays warnings
- Updated all test fixtures in `cmd/init_test.go` and `cmd/import_test.go` to create valid tasks with descriptions and verify commands

**Files touched:**

- `internal/taskstore/linter.go` (new)
- `internal/taskstore/linter_test.go` (new)
- `cmd/init.go` (added linter call)
- `cmd/import.go` (added linter call and warning display)
- `cmd/init_test.go` (added helper functions, updated fixtures)
- `cmd/import_test.go` (updated test expectations and fixtures)
- `tasks.yaml` (marked ralph-align-task-linter as completed)

**Learnings:**

- Avoided import cycle by implementing cycle detection directly in linter instead of importing selector package (taskstore already imported by selector)
- Used DFS with three-color marking (white=unvisited, gray=in-progress, black=done) for cycle detection - gray back-edge indicates cycle
- Warnings (missing acceptance criteria) are non-fatal and don't prevent task usage, but errors (missing description, missing verify on leaves) fail validation
- Linter runs after import completes to validate the entire task set including cross-task relationships (cycles, dependencies, parent-child)
- Test fixtures needed updating because linter now enforces stricter validation - all leaf tasks must have descriptions and verify commands
- Created helper functions `createValidParentTask()` and `createValidLeafTask()` in tests for consistent valid task creation
- The `LintResult.Error()` method returns nil if valid, or formatted error with all validation failures if invalid

**Outcome**: Success - all tests pass (go test ./..., go build ./..., golangci-lint run), linter fully integrated into init and import commands

### 2026-01-16: ralph-align-feature-branch (Create and Use Feature Branches)

**What changed:**

- Added `branchOverride` field to Controller struct for optional branch name override
- Implemented `slugify()` function to convert task titles to branch-safe names (lowercase, hyphens, alphanumeric only)
- Implemented `SetBranchOverride()` method to allow explicit branch name configuration
- Implemented `ensureFeatureBranch()` method that:
  - Uses branch override if set, otherwise generates branch name from parent task title
  - Calls gitManager.EnsureBranch to create/switch to the feature branch
  - Errors early if parent task doesn't exist
- Modified `RunLoop()` to call `ensureFeatureBranch()` at the start before entering the iteration loop
- Added `--branch` flag to `ralph run` command to allow users to override the auto-generated branch name
- Updated `cmd/run.go` to call `SetBranchOverride()` when --branch flag is provided
- Updated existing `dynamicGitManager` test mock to support `ensureBranchFn`, `getCurrentCommitFn`, `hasChangesFn`, and `commitFn` callbacks
- Created comprehensive test suite with 18+ tests covering:
  - `slugify()` with various inputs (spaces, special chars, empty strings, real task titles)
  - `ensureFeatureBranch()` with auto-generation, override, task not found, git errors
  - `RunLoop()` integration test verifying branch is created before iterations
- Fixed existing tests that were broken by the new branch creation requirement:
  - `TestController_RunLoop_InvalidParentTask` - now expects error outcome when parent doesn't exist
  - `TestController_RunLoop_IterationTimeoutFromConfig` - added parent task to mock store
  - `TestRunCmd_Integration_NoReadyTasks` - initialized git repo with initial commit and disabled GPG signing

**Files touched:**

- `internal/loop/controller.go` (added slugify, ensureFeatureBranch, branchOverride field, SetBranchOverride method)
- `internal/loop/slug_test.go` (new - comprehensive slugify tests)
- `internal/loop/controller_test.go` (added filepath import, updated dynamicGitManager, added 5 new tests, fixed 2 existing tests)
- `cmd/run.go` (added --branch flag, updated runRun signature, added SetBranchOverride call)
- `cmd/run_test.go` (added os/exec import, initialized git repo in integration test)
- `tasks.yaml` (marked ralph-align-feature-branch as completed)
- `.ralph/progress.md` (this entry)

**Learnings:**

- The branch prefix from config (e.g., "ralph/") is prepended by the git.ShellManager's EnsureBranch method, so we just pass the slug
- Branch names must be lowercase alphanumeric with hyphens - no spaces, underscores converted to hyphens, special characters removed
- The `strings.ReplaceAll()` pattern for collapsing multiple hyphens works well with a simple loop
- EnsureBranch is idempotent - it checks if already on the branch, switches if branch exists, creates if doesn't exist
- Calling EnsureBranch early in RunLoop (before the main loop) provides better error handling and clearer failure messages
- The --branch flag allows users to override auto-generation for cases where they want specific branch names
- Test mocks need to be extended carefully - reusing existing mock types with new callback fields is cleaner than creating duplicates
- Git integration tests require: `git init`, `git config user.*`, `git config commit.gpgsign false`, and an initial commit before operations
- When a new requirement affects existing tests, update test expectations rather than working around them - the new behavior is correct

**Outcome**: Success - all tests pass (go test ./..., go build ./..., golangci-lint run), feature branch creation fully integrated, all acceptance criteria met


### 2026-01-16: ralph-align-revert-cmd (Add ralph revert Command)

**What changed:**

- Created `cmd/revert.go` implementing the `ralph revert` command with:
  - `--iteration` flag (required) to specify which iteration to revert
  - `--force` flag to skip confirmation prompt
  - Interactive confirmation prompt showing commit details and warnings
  - Git reset --hard to the base commit from the iteration record
  - Task status update to reset completed tasks back to open
  - Error handling for missing iterations, missing base commits, and git failures
- Created `cmd/revert_test.go` with comprehensive test coverage:
  - Test for missing --iteration flag error
  - Test for iteration not found error
  - Test for confirmation requirement (flag structure validation)
  - Full integration test with git repo setup, iteration records, and task status verification
  - Helper functions: `runTestCommand()` and `getTestGitCommit()` for test git operations
- Updated `cmd/root.go` to register the new `revertCmd` subcommand
- Used `exec.Command()` directly for git reset operation to match existing patterns in the codebase

**Files touched:**

- `cmd/revert.go` (new - 162 lines)
- `cmd/revert_test.go` (new - 148 lines)
- `cmd/root.go` (added newRevertCmd() registration)
- `tasks.yaml` (marked ralph-align-revert-cmd as completed)
- `.ralph/progress.md` (this entry)

**Learnings:**

- Test helpers should follow existing patterns - use `exec.Command()` directly rather than creating wrapper functions
- Task validation requires `CreatedAt` and `UpdatedAt` timestamps - all test tasks must include these fields
- Git integration tests need proper setup: `git init -b main`, user config, commit.gpgsign=false, and initial commit
- The `t.TempDir()` and `os.Chdir()` pattern requires saving and restoring the old working directory with defer
- Confirmation prompts use `bufio.NewReader(cmd.InOrStdin())` to support testing with custom input streams
- The `gitResetHard()` function wraps `exec.Command()` for better testability and separation of concerns
- Task status updates only happen when the iteration outcome was success and the task exists in the store
- The `--force` flag is essential for non-interactive use cases and testing
- Error handling should distinguish between "task not found" (acceptable) and other errors (return)

**Outcome**: Success - all tests pass (go test ./...), command fully functional with comprehensive error handling and user experience features (warnings, confirmations, clear messaging)


### 2026-01-16: ralph-align-sandbox (Implement Sandbox Mode Guard)

**What changed:**

- Enhanced `cmd/run.go` to enforce sandbox mode by checking `cfg.Safety.Sandbox` before applying allowed commands to verifier
- Added `SetSandboxMode()` method to `internal/loop/controller.go` to configure sandbox mode with allowed tools list
- Added `sandboxEnabled` and `allowedTools` fields to Controller struct
- Modified both ClaudeRequest creation points (initial and retry) to include `AllowedTools` when sandbox mode is enabled
- Updated `internal/config/config.go` to set default `allowed_commands` to `["npm", "go", "git"]` (was empty)
- Added test `TestConfig_SandboxMode` in `internal/config/config_test.go` covering:
  - Sandbox disabled by default
  - Sandbox enabled with custom allowlist
  - Sandbox enabled with empty allowlist
- Added test `TestController_SetSandboxMode` in `internal/loop/controller_test.go` covering:
  - Sandbox mode disabled by default
  - Can enable sandbox mode with allowed tools
  - Can disable sandbox mode
- Updated `TestLoadConfig_WithDefaults` to verify default allowed_commands is `["npm", "go", "git"]`
- Added test case in `TestCommandRunner_Allowlist` for clearing allowlist with empty slice

**Files touched:**

- `cmd/run.go` (modified - added sandbox mode enforcement check)
- `internal/loop/controller.go` (modified - added SetSandboxMode method, sandboxEnabled/allowedTools fields, applied to ClaudeRequest)
- `internal/config/config.go` (modified - updated default allowed_commands)
- `internal/config/config_test.go` (modified - added TestConfig_SandboxMode, updated defaults test)
- `internal/loop/controller_test.go` (modified - added TestController_SetSandboxMode)
- `internal/verifier/runner_test.go` (modified - added allowlist clearing test case)
- `tasks.yaml` (marked ralph-align-sandbox as completed)
- `.ralph/progress.md` (this entry)

**Learnings:**

- Sandbox mode requires coordination between multiple layers: config, cmd layer (for verifier), and controller (for Claude Code)
- The verifier already had `SetAllowedCommands()` implemented but wasn't gated by sandbox flag
- Claude Code's `--allowedTools` flag is passed via the `AllowedTools` field in `ClaudeRequest`
- Default values in config should match PRD spec expectations - defaulting to common safe commands is better than empty
- Both initial and retry ClaudeRequest invocations need the same sandbox mode restrictions applied
- Testing strategy: config layer tests verify loading/defaults, controller layer tests verify state management, integration happens via cmd layer
- The sandbox mode check should be `cfg.Safety.Sandbox && len(cfg.Safety.AllowedCommands) > 0` to avoid accidentally blocking when empty
- Verifier allowlist enforcement was already implemented correctly, just needed to be conditionally enabled

**Outcome**: Success - all tests pass (go test ./..., go build ./...), sandbox mode fully functional with proper gating and tool restrictions

### 2026-01-16: ralph-align-gutter (Improve Gutter Detection)

**What changed:**

- Enhanced gutter detection with oscillation detection for files modified in non-consecutive iterations
- Added content hash tracking capability to detect file modification patterns
- Extended GutterConfig with MaxOscillations and EnableContentHash options
- Implemented checkOscillation() method to detect files repeatedly appearing after gaps
- Updated GutterState to persist content hashes across sessions
- Added comprehensive tests for oscillation detection and configuration options

**Files touched:**

- `internal/loop/gutter.go` (modified)
- `internal/loop/gutter_test.go` (modified)
- `internal/config/config.go` (modified)

**Learnings:**

- Oscillation detection tracks when files are modified in non-consecutive iterations, indicating "thrashing" behavior
- Since IterationRecord doesn't contain file content, oscillation is detected by tracking file appearance patterns rather than actual content hashes
- The enableContentHash flag controls whether oscillation detection is active
- Oscillation check runs before file churn check to catch more specific patterns first
- Config defaults: MaxOscillations=2, EnableContentHash=true, MaxChurnIterations=5, ChurnThreshold=3
- Tests must disable oscillation (or set high threshold) when specifically testing file churn to avoid interference

**Outcome**: Success - all tests pass (go test ./...), build succeeds (go build ./...), linter passes (golangci-lint run)

### 2026-01-16: ralph-align-text-logs (Generate Human-Readable Iteration Logs)

**What changed:**

- Implemented `GenerateTextLog()` function in `internal/loop/record.go` to create human-readable text summaries of iteration records
- Modified `SaveRecord()` to automatically generate and save both JSON and text log files
- Added text log generation that includes: iteration ID, task ID, start/end times, duration, outcome, base/result commits, files changed, verification results, Claude invocation metadata, feedback, and attempt number
- Text logs are saved as `.ralph/logs/iteration-<id>.txt` alongside JSON logs
- Added comprehensive tests for text log generation and file creation

**Files touched:**

- `internal/loop/record.go` (modified - added GenerateTextLog function, updated SaveRecord)
- `internal/loop/record_test.go` (modified - added TestGenerateTextLog and TestSaveRecord_CreatesTextLog)

**Learnings:**

- Text logs provide human-readable iteration summaries without needing to parse JSON
- JSON remains the source of truth; text log write failures are non-fatal warnings
- Text format includes all key metrics: timing, outcome, file changes, verification status
- Using strings.Builder for efficient string concatenation in log generation
- Verification outputs show as "PASS/FAIL" with command duration when available
- Claude metadata (model, session ID, cost, token counts) included when present

**Outcome**: Success - all tests pass (go test ./...), task verification passes (go test ./internal/loop/...)

### 2026-01-16: ralph-align-commit-messages (Include Commit Messages in Reports)

**What changed:**

- Added `GetCommitMessage(ctx, hash)` method to `git.Manager` interface in `internal/git/manager.go`
- Implemented `GetCommitMessage` in `ShellManager` using `git log -1 --format=%B` to retrieve full commit message body
- Modified `ReportGenerator` struct to accept `git.Manager` as a dependency
- Updated `NewReportGenerator()` constructor signature to include `gitManager` parameter
- Modified `GenerateReport()` to call `GetCommitMessage` for each commit and populate `CommitInfo.Message` field
- Updated all mock implementations (`mockManager`, `mockGitManager`, `dynamicGitManager`) to implement the new method
- Updated `cmd/report.go` to create `ShellManager` and pass it to `NewReportGenerator`
- Updated all test files to pass `nil` for git manager where git operations are not being tested

**Files touched:**

- `internal/git/manager.go` (modified - added GetCommitMessage to interface)
- `internal/git/shell.go` (modified - implemented GetCommitMessage method)
- `internal/git/manager_test.go` (modified - added GetCommitMessage to mockManager)
- `internal/reporter/report.go` (modified - added gitManager field, updated constructor, populate commit messages)
- `internal/reporter/report_test.go` (modified - updated all NewReportGenerator calls to pass nil)
- `cmd/report.go` (modified - create and pass git manager)
- `internal/loop/controller_test.go` (modified - added GetCommitMessage to mockGitManager and dynamicGitManager)

**Learnings:**

- Git commit messages are retrieved using `git log -1 --format=%B <hash>` which returns the full message body
- The reporter gracefully handles missing git manager (nil check) or commit retrieval errors by leaving message empty
- This prevents report generation from failing if git operations fail
- Mock implementations need to be updated whenever the interface changes to maintain test compatibility
- Passing nil for optional dependencies in tests is idiomatic when those dependencies aren't being tested

**Outcome**: Success - all tests pass (go test ./...), task verification passes (go test ./internal/git/..., go test ./internal/reporter/...)

### 2026-01-16: ralph-align-archive-progress (Archive Old Progress on Feature Switch)

**What changed:**

- Added `GetStoredParentTaskID` and `SetStoredParentTaskID` functions to `internal/state/state.go` to track parent task ID changes in `.ralph/state/parent-task-id`
- Modified `cmd/init.go` to detect parent task ID changes and archive old progress files
- When parent task changes: archives existing progress.md to `.ralph/archive/progress-TIMESTAMP.md` and creates new progress.md with fresh header
- When parent task stays the same: no archiving occurs
- On first initialization: creates progress.md if it doesn't exist

**Files touched:**

- `internal/state/state.go` (added parent task ID state functions)
- `internal/state/state_test.go` (added tests for new functions)
- `cmd/init.go` (added archive logic on parent task change)
- `cmd/init_test.go` (added tests for archiving behavior)
- `tasks.yaml` (marked task as completed)

**Learnings:**

- State files should be stored in `.ralph/state/` directory for session-level data
- The config file already has `parent_id_file` at `.ralph/parent-task-id` for compatibility, but state tracking uses `.ralph/state/parent-task-id`
- Progress archive uses `memory.NewProgressArchive` and `archive.Archive()` which handles timestamping and collision avoidance
- Progress file initialization uses `memory.NewProgressFile` and `Init()` method to create header with feature name and parent task ID
- Archive only happens when parent ID changes (not on same-parent re-init)

**Outcome**: Success - all tests pass, go build succeeds

### 2026-01-16: ralph-align-progress-limits (Enforce Progress File Size Limits)

**What changed:**

- Added `MaxProgressBytes` and `MaxRecentIterations` fields to `MemoryConfig` struct in `internal/config/config.go` with defaults of 1MB and 20 iterations respectively
- Modified `SizeOptions` struct in `internal/memory/progress.go` to use byte-based limits and iteration count instead of line count
- Rewrote `EnforceMaxSize` method to check byte size, preserve header and Codebase Patterns section, and keep at least MaxRecentIterations of most recent entries
- Added pruning note comment when entries are removed
- Added `maxProgressBytes` and `maxRecentIterations` fields to `Controller` struct in `internal/loop/controller.go`
- Added `SetMemoryConfig` method to set memory configuration on controller
- Modified controller's success path to call `EnforceMaxSize` after appending iteration entries
- Updated `cmd/run.go` to call `SetMemoryConfig` with values from config
- Updated all tests to use new `SizeOptions` structure with byte limits and iteration counts

**Files touched:**

- `internal/config/config.go` (modified - added memory config fields and defaults)
- `internal/config/config_test.go` (modified - added assertions for new defaults)
- `internal/memory/progress.go` (modified - changed SizeOptions, rewrote EnforceMaxSize)
- `internal/memory/progress_test.go` (modified - updated tests for byte-based limits)
- `internal/loop/controller.go` (modified - added memory config fields and SetMemoryConfig method, added enforcement after AppendIteration)
- `cmd/run.go` (modified - added SetMemoryConfig call)

**Learnings:**

- Byte-based size limits are more predictable than line counts for preventing unbounded growth
- The algorithm prioritizes staying under byte limit over keeping all MaxRecentIterations entries - it keeps as many recent entries as fit
- Preserving header and Codebase Patterns section is critical since they contain durable knowledge
- The enforcement logic builds test output incrementally to calculate exact sizes before committing
- Setting reasonable defaults (1MB, 20 iterations) balances memory usage with keeping sufficient context
- The Controller doesn't store full config but extracts needed values via setter methods for cleaner dependency management

**Outcome**: Success - all tests pass (go test ./...), build succeeds (go build ./...), config defaults verified

### 2026-01-16: ralph-align-agents-md (Support AGENTS.md Updates)

**What changed:**

- Created `internal/memory/agents.go` with `FindAgentsMd()` and `ReadAgentsMd()` functions
- `FindAgentsMd()` walks directory tree to find all AGENTS.md files, skipping hidden directories
- `ReadAgentsMd()` reads and concatenates all AGENTS.md files with file path headers and truncation at 10KB per file
- Modified `internal/prompt/iteration.go` to add `AgentsContent` field to `IterationContext`
- Updated `BuildSystemPrompt()` to include guidance: "Update AGENTS.md only with durable, reusable patterns (not task-specific)"
- Updated `BuildUserPrompt()` to include existing AGENTS.md content in prompts when available
- AGENTS.md content is shown in "### Existing AGENTS.md Files" section and truncated using same mechanism as patterns

**Files touched:**

- `internal/memory/agents.go` (new)
- `internal/memory/agents_test.go` (new)
- `internal/prompt/iteration.go` (modified)
- `internal/prompt/iteration_test.go` (modified)
- `tasks.yaml` (marked ralph-align-agents-md as completed)

**Learnings:**

- `filepath.WalkDir` is efficient for searching directory trees and allows skipping directories with `filepath.SkipDir`
- Skipping hidden directories (starting with ".") prevents scanning `.git`, `.ralph`, etc. which improves performance
- Reading files in directory walk should be graceful - skip files that can't be read rather than failing entire operation
- Truncation with markers maintains consistency with existing patterns truncation in prompt builder
- Adding AGENTS.md guidance to system prompt (rule 6) and including existing content helps Claude understand when and how to use AGENTS.md files

**Outcome**: Success - all tests pass, `go build ./...` and `go test ./...` succeed
