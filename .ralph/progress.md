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
