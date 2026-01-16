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
| `ralph import` | Import tasks from a YAML file into task store              |
| `ralph run`    | Run the iteration loop                                     |
| `ralph status` | Show current status (task counts, next task, last outcome) |
| `ralph pause`  | Pause the iteration loop                                   |
| `ralph resume` | Resume the iteration loop                                  |
| `ralph retry`  | Retry a failed task                                        |
| `ralph skip`   | Skip a task                                                |
| `ralph report` | Generate end-of-feature summary report                     |
| `ralph revert` | Revert to state before a specific iteration                |
| `ralph logs`   | Display iteration logs                                     |

### ralph init

Initialize ralph by setting the parent task ID and validating the task graph.

```bash
ralph init --parent <id>     # Set parent task by ID
ralph init --search "<term>" # Find parent task by title search
```

### ralph import

Import tasks from a YAML file into the task store.

```bash
ralph import tasks.yaml            # Import tasks from file
ralph import tasks.yaml --overwrite # Update existing tasks
```

### ralph run

Execute the iteration loop until all tasks are done or limits are reached.

```bash
ralph run                       # Run continuously
ralph run --once                # Run only a single iteration
ralph run --max-iterations 10   # Limit iterations (overrides config)
ralph run --branch <name>       # Use specific branch name
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

### ralph revert

Revert to the state before a specific iteration (git reset --hard).

```bash
ralph revert --iteration <id>         # Revert to state before iteration
ralph revert --iteration <id> --force # Skip confirmation prompt
```

### ralph logs

Display iteration logs.

```bash
ralph logs                      # Show all iteration logs
ralph logs --iteration <id>     # Show logs for specific iteration
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
  max_progress_bytes: 1048576
  max_recent_iterations: 20

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
  max_retries: 2
  max_verification_retries: 2
  gutter:
    max_same_failure: 3
    max_churn_commits: 2
    max_oscillations: 2
    enable_content_hash: true

safety:
  sandbox: false
  allowed_commands:
    - "npm"
    - "go"
    - "git"
```

### Configuration Options

| Section                           | Option | Description                             |
| --------------------------------- | ------ | --------------------------------------- |
| `repo.root`                       |        | Repository root directory               |
| `repo.branch_prefix`              |        | Git branch prefix for features          |
| `tasks.backend`                   |        | Task storage backend (`local`)          |
| `tasks.path`                      |        | Path to task storage directory          |
| `tasks.parent_id_file`            |        | File storing current parent task ID     |
| `memory.progress_file`            |        | Feature progress log file               |
| `memory.archive_dir`              |        | Archive directory for old progress      |
| `memory.max_progress_bytes`       |        | Max size before pruning                 |
| `memory.max_recent_iterations`    |        | Number of recent iterations to preserve |
| `claude.command`                  |        | Claude Code CLI command                 |
| `claude.args`                     |        | Additional arguments for Claude Code    |
| `verification.commands`           |        | Commands to run for verification        |
| `loop.max_iterations`             |        | Maximum iterations per run              |
| `loop.max_minutes_per_iteration`  |        | Time limit per iteration                |
| `loop.max_retries`                |        | Max retries for failed tasks            |
| `loop.max_verification_retries`   |        | Max in-iteration fix attempts           |
| `loop.gutter.max_same_failure`    |        | Stop after N same failures              |
| `loop.gutter.max_churn_commits`   |        | Stop after N churn commits              |
| `loop.gutter.max_oscillations`    |        | Stop after N file oscillations          |
| `loop.gutter.enable_content_hash` |        | Use content hashing for oscillation     |
| `safety.sandbox`                  |        | Enable sandbox mode                     |
| `safety.allowed_commands`         |        | Allowlist for shell commands            |

### Environment Variables

| Variable         | Description                                                  |
| ---------------- | ------------------------------------------------------------ |
| `CLAUDE_API_KEY` | API key for Claude Code (required by Claude Code subprocess) |

## Task Definition

Tasks are defined in YAML files in `.ralph/tasks/`:

```yaml
tasks:
  - id: task-1
    title: "My Task"
    description: "Detailed standalone description"
    parentId: parent-task-id
    dependsOn:
      - other-task-id
    status: open
    acceptance:
      - "Criterion 1"
      - "Criterion 2"
    verify:
      - ["go", "test", "./..."]
    labels:
      area: core
      priority: P0
```

### Task Fields

| Field         | Description                                                        |
| ------------- | ------------------------------------------------------------------ |
| `id`          | Unique identifier                                                  |
| `title`       | Short summary                                                      |
| `description` | Detailed standalone description                                    |
| `parentId`    | Optional parent task ID                                            |
| `dependsOn`   | Task IDs that must complete first                                  |
| `status`      | `open`, `in_progress`, `completed`, `blocked`, `failed`, `skipped` |
| `acceptance`  | Verifiable acceptance criteria                                     |
| `verify`      | Task-specific verification commands                                |
| `labels`      | Optional categorization                                            |

## Storage

Ralph stores state in the `.ralph/` directory:

| Path                 | Description                               |
| -------------------- | ----------------------------------------- |
| `.ralph/tasks/`      | Task store (YAML files)                   |
| `.ralph/progress.md` | Feature progress log                      |
| `.ralph/state/`      | Session IDs, pause state, budget tracking |
| `.ralph/logs/`       | Iteration logs (JSON and text)            |
| `.ralph/archive/`    | Archived progress files                   |

## Troubleshooting

### Configuration and CLI Flag Issues

Configuration loading may have edge cases. Ensure your `ralph.yaml` is valid YAML and follows the documented structure. Run `ralph --help` for command-specific flags.

### Progress File Growth

The progress file is automatically pruned when it exceeds `max_progress_bytes`. Recent iterations (configured by `max_recent_iterations`) are preserved along with headers and pattern sections.

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
