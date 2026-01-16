# Ralph

A Go-based harness that orchestrates [Claude Code](https://claude.ai) for autonomous, iterative feature delivery. Ralph executes a "Ralph Wiggum loop": select ready task → delegate to Claude Code → verify → commit → repeat.

- **Autonomous iteration loop**: Continuously picks ready tasks, delegates to Claude Code, verifies results, and commits changes
- **Task dependency management**: Validates acyclic task graphs and selects only ready leaf tasks
- **Verification pipeline**: Runs configurable verification commands (tests, typecheck, lint) before committing
- **Git discipline**: Dedicated branches per feature with commit templates and verification gates
- **Memory hygiene**: Maintains progress logs and patterns without relying on conversational memory
- **Guardrails**: Budget limits, gutter detection, pause/resume, and sandbox support

## Prerequisites

- Go 1.25.5 or later
- [Claude Code](https://claude.ai) CLI installed and configured
- Git

## Install

```bash
go install github.com/yarlson/go-ralph@latest
```

Or build from source:

```bash
git clone https://github.com/yarlson/go-ralph.git
cd go-ralph
go build ./...
```

## Quickstart

1. **Create a task file** in `.ralph/tasks/` (YAML format) with your tasks

2. **Initialize ralph** with a parent task:

```bash
ralph init --parent <task-id>
# or search by title
ralph init --search "feature name"
```

3. **Run the loop**:

```bash
ralph run
```

Ralph will continuously select ready leaf tasks, delegate to Claude Code, verify, commit, and repeat until all tasks are complete or limits are reached.

## Usage

### Commands

| Command        | Description                                                |
| -------------- | ---------------------------------------------------------- |
| `ralph init`   | Initialize ralph for a feature                             |
| `ralph run`    | Run the iteration loop                                     |
| `ralph status` | Show current status (task counts, next task, last outcome) |
| `ralph pause`  | Pause the iteration loop                                   |
| `ralph resume` | Resume the iteration loop                                  |
| `ralph retry`  | Retry a failed task                                        |
| `ralph skip`   | Skip a task                                                |
| `ralph report` | Generate end-of-feature summary report                     |

### ralph init

Initialize ralph by setting the parent task ID and validating the task graph.

```bash
ralph init --parent <id>    # Set parent task by ID
ralph init --search "<term>" # Find parent task by title search
```

### ralph run

Execute the iteration loop until all tasks are done or limits are reached.

```bash
ralph run                    # Run continuously
ralph run --once            # Run only a single iteration
ralph run --max-iterations 10 # Limit iterations (overrides config)
```

### ralph status

Display task counts, next selected task, and last iteration outcome.

```bash
ralph status
```

### ralph pause / resume

Control the iteration loop.

```bash
ralph pause   # Stop after current iteration
ralph resume  # Allow loop to continue
```

### ralph retry

Reset a task to open status and optionally add feedback for the next attempt.

```bash
ralph retry --task <id>
ralph retry --task <id> --feedback "additional context for retry"
```

### ralph skip

Mark a task as skipped so the loop can continue.

```bash
ralph skip --task <id>
ralph skip --task <id> --reason "reason for skipping"
```

### ralph report

Generate and display an end-of-feature summary report.

```bash
ralph report                 # Output to stdout
ralph report -o report.md    # Write to file
```

### Global Flags

| Flag       | Description      | Default      |
| ---------- | ---------------- | ------------ |
| `--config` | Config file path | `ralph.yaml` |

## Configuration

Ralph uses a `ralph.yaml` configuration file. Example:

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
  command: ["claude", "code"]
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

### Configuration Options

| Section                          | Option                              | Description |
| -------------------------------- | ----------------------------------- | ----------- |
| `repo.branch_prefix`             | Git branch prefix for features      |
| `tasks.path`                     | Path to task storage directory      |
| `tasks.parent_id_file`           | File storing current parent task ID |
| `memory.progress_file`           | Feature progress log file           |
| `claude.command`                 | Claude Code CLI command             |
| `verification.commands`          | Commands to run for verification    |
| `loop.max_iterations`            | Maximum iterations per run          |
| `loop.max_minutes_per_iteration` | Time limit per iteration            |
| `loop.gutter.max_same_failure`   | Stop after N same failures          |
| `loop.gutter.max_churn_commits`  | Stop after N churn commits          |
| `safety.allowed_commands`        | Allowlist for shell commands        |

### Environment Variables

| Variable         | Description                                                  |
| ---------------- | ------------------------------------------------------------ |
| `CLAUDE_API_KEY` | API key for Claude Code (required by Claude Code subprocess) |

## Storage

Ralph stores state in the `.ralph/` directory:

| Path                 | Description                               |
| -------------------- | ----------------------------------------- |
| `.ralph/tasks/`      | Task store (YAML files)                   |
| `.ralph/progress.md` | Feature progress log                      |
| `.ralph/state/`      | Session IDs, pause state, budget tracking |
| `.ralph/logs/`       | Iteration logs                            |

## Troubleshooting

Not documented. Check `ralph.yaml` and command help (`ralph --help`) for details.

## Development

### Build

```bash
go build ./...
```

### Test

```bash
go test ./...
```

### Format

```bash
gofmt -w .
```

### Lint

```bash
golangci-lint run
```

## Contributing

Not documented. Check the repository for contribution guidelines.

## License

Not documented. Check the repository for license information.
