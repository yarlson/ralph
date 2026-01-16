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
