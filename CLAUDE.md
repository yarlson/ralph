# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Ralph is a Go-based harness that orchestrates Claude Code for autonomous, iterative feature delivery. It executes a "Ralph Wiggum loop": select ready task → delegate to Claude Code → verify → commit → repeat.

The harness does not write code—it is a deterministic executor, verifier, and state manager. Claude Code is the only coding agent.

## Build and Development Commands

```bash
# Build
go build ./...

# Run tests
go test ./...

# Run a single test
go test -run TestName ./path/to/package

# Lint (uses golangci-lint)
golangci-lint run

# Format
gofmt -w .
```

## Architecture

### Core Components (in `internal/`)

- **TaskStore**: Task persistence and retrieval (`.ralph/tasks/`)
- **Selector**: Ready leaf task selection with dependency resolution
- **ClaudeRunner**: Claude Code subprocess execution and NDJSON stream parsing
- **Verifier**: Runs verification commands (typecheck, test, lint)
- **GitManager**: Branch creation, commits after verified changes
- **MemoryManager**: Progress file (`.ralph/progress.md`) and AGENTS.md management
- **LoopController**: Iteration orchestration, budgets, gutter detection
- **Reporter**: Status and report generation

### Claude Code Integration

ClaudeRunner executes Claude Code as a subprocess with `--output-format="stream-json"`. Key parsing:
- `system/init`: Extract `session_id`, `model`, `cwd`
- `assistant/message`: Accumulate streamed text
- `result/success` or `result/error`: Terminal event with `result`, `total_cost_usd`, `usage`

Raw NDJSON logs go to `.ralph/logs/claude/`.

### State Files

- `.ralph/tasks/`: Task store (JSON)
- `.ralph/state/claude-session.json`: Session IDs
- `.ralph/state/budget.json`: Cost and iteration tracking
- `.ralph/progress.md`: Feature-specific patterns and iteration history
- `.ralph/logs/`: Iteration logs and Claude NDJSON output

## Code Style

Follow Effective Go and Go Proverbs. Key principles:

- **Interfaces on consumers**: Define interfaces where they're used, not where types are implemented
- **Accept interfaces, return structs**: Functions should accept interface parameters and return concrete types
- **Small interfaces**: Prefer single-method interfaces; compose larger behaviors
- **DRY/KISS/YAGNI**: No premature abstraction; solve the problem at hand
- **Error handling**: Return errors; don't panic. Wrap errors with context using `fmt.Errorf("context: %w", err)`
- **Testing**: Use `testify/assert` and `testify/require`. Table-driven tests preferred.

## Test-Driven Development (TDD)

**TDD is mandatory for all new code.** Follow the red-green-refactor cycle:

1. **Red**: Write a failing test first that defines the expected behavior
2. **Green**: Write the minimum code necessary to make the test pass
3. **Refactor**: Clean up the code while keeping tests green

Rules:
- No production code without a corresponding test written first
- Run tests after each change to verify the cycle
- Commit tests and implementation together

## CLI

Built with Cobra. Commands: `ralph init`, `ralph run`, `ralph status`, `ralph pause`, `ralph resume`, `ralph retry`, `ralph skip`, `ralph report`.

Configuration: `ralph.yaml` at repo root.
