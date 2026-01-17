# CLAUDE.md

Ralph orchestrates Claude Code for autonomous feature delivery via a loop: select ready task → delegate → verify → commit → repeat.

## Commands

```bash
go build ./...          # Build
go test ./...           # Test all
go test -run TestName ./internal/pkg  # Test specific
golangci-lint run       # Lint
gofmt -w .              # Format
```

## Package Map

| Package    | Location               | Purpose                                         |
| ---------- | ---------------------- | ----------------------------------------------- |
| taskstore  | `internal/taskstore/`  | Task model, YAML persistence, validation        |
| selector   | `internal/selector/`   | Dependency graph, ready leaf selection          |
| claude     | `internal/claude/`     | Subprocess execution, NDJSON streaming          |
| verifier   | `internal/verifier/`   | Test/lint/typecheck runners                     |
| git        | `internal/git/`        | Branch creation, commits                        |
| memory     | `internal/memory/`     | Progress file, AGENTS.md discovery              |
| loop       | `internal/loop/`       | Iteration controller, budgets, gutter detection |
| reporter   | `internal/reporter/`   | Status display, reports                         |
| prompt     | `internal/prompt/`     | Task context building                           |
| config     | `internal/config/`     | ralph.yaml loading                              |
| state      | `internal/state/`      | .ralph directory management                     |
| decomposer | `internal/decomposer/` | PRD → task YAML via Claude                      |

## CLI Commands

`init` · `run` · `status` · `pause` · `resume` · `retry` · `skip` · `report` · `decompose` · `import` · `logs` · `revert`

All in `cmd/` with matching `*_test.go` files. Config: `ralph.yaml`.

## State Files

```
.ralph/
├── tasks/          # Task YAML files
├── state/          # Session, budget, pause flag
├── logs/           # Iteration + Claude NDJSON logs
├── progress.md     # Iteration history
└── archive/        # Old iterations
```

## Task Lifecycle

`open` → `in_progress` → `completed` | `failed` | `skipped`

`blocked` = waiting on dependency. `failed` can retry → `in_progress`.

## Terminology

| Term      | Meaning                                                              |
| --------- | -------------------------------------------------------------------- |
| Leaf task | Task with no children (executable unit)                              |
| Ready     | Open leaf with all dependencies completed                            |
| Iteration | One cycle: select → delegate → verify → commit                       |
| Gutter    | Unproductive loop detection (repeated failures, churn, oscillations) |

## Code Style

**Go idioms**: Effective Go, Go Proverbs.

- Interfaces at call site, not definition site
- Accept interfaces, return structs
- Single-method interfaces; compose for larger behaviors
- Errors returned, not panicked; wrap with `fmt.Errorf("context: %w", err)`

**Don't**: `panic()`, `init()`, global state, mocks outside `*_test.go`

## TDD Required

Red → Green → Refactor. No production code without a failing test first.

```bash
# Cycle
go test -run TestNewFeature ./internal/pkg  # Red: fails
# Write minimal implementation
go test -run TestNewFeature ./internal/pkg  # Green: passes
# Refactor, keep green
```

Use `testify/assert` and `testify/require`. Table-driven tests preferred.

## Principles

**DRY**: Extract only when duplication is proven, not predicted.

**KISS**: Simplest solution that works. No clever abstractions.

**YAGNI**: Solve today's problem. Delete speculative code.
