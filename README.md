# Ralph

A Go-based harness that orchestrates [Claude Code](https://claude.ai) for autonomous, iterative feature delivery. Ralph executes a "Ralph Wiggum loop": select ready task → delegate to Claude Code → verify → commit → repeat.

- **Autonomous iteration loop**: Continuously picks ready tasks, delegates to Claude Code, verifies results, and commits changes
- **Task dependency management**: Validates acyclic task graphs and selects only ready leaf tasks
- **PRD decomposition**: Automatically convert PRD markdown files into hierarchical task graphs
- **Verification pipeline**: Runs configurable verification commands (tests, typecheck, lint) before committing
- **Git discipline**: Dedicated branches per feature with automatic commits
- **Memory hygiene**: Maintains progress logs without relying on conversational memory
- **Guardrails**: Budget limits, gutter detection (stuck loops), and sandbox support

## Prerequisites

- Go 1.25.5 or later
- [Claude Code](https://claude.ai) CLI installed and configured (`claude` command in PATH)
- Git

## Install

```bash
go install github.com/yarlson/ralph@latest
```

Or build from source:

```bash
git clone https://github.com/yarlson/ralph.git
cd ralph
go build ./...
```

## Quickstart

### Option 1: Start from a PRD

```bash
ralph docs/prd.md
```

This will:

1. Decompose the PRD into tasks using Claude
2. Import tasks into the store
3. Initialize with the root task
4. Start the iteration loop

### Option 2: Start from a Task YAML

```bash
ralph tasks.yaml
```

### Option 3: Run with Existing Tasks

```bash
ralph
```

If no parent task is set, ralph will prompt you to select a root task interactively.

## Usage

### Main Command

```bash
ralph [file]
```

The main command accepts an optional file argument:

- A PRD `.md` file to decompose into tasks
- A task `.yaml` file to import tasks

| Flag               | Short | Description                                |
| ------------------ | ----- | ------------------------------------------ |
| `--once`           | `-1`  | Run only a single iteration                |
| `--max-iterations` | `-n`  | Maximum iterations (0 uses config default) |
| `--parent`         | `-p`  | Explicit parent task ID                    |
| `--branch`         | `-b`  | Git branch override                        |
| `--dry-run`        |       | Show what would be done                    |
| `--config`         |       | Config file path (default: `ralph.yaml`)   |

### Status Command

Display task counts, next selected task, and last iteration outcome:

```bash
ralph status
```

### Fix Command

Fix failed tasks or undo iterations:

```bash
ralph fix                                    # Interactive mode (requires TTY)
ralph fix --list                             # List fixable issues
ralph fix --retry <task-id>                  # Retry a failed task
ralph fix --retry <task-id> --feedback "hint" # Retry with feedback
ralph fix --skip <task-id>                   # Skip a task
ralph fix --skip <task-id> --reason "reason" # Skip with reason
ralph fix --undo <iteration-id>              # Undo an iteration
ralph fix --force                            # Skip confirmation prompts
```

| Flag         | Short | Description                |
| ------------ | ----- | -------------------------- |
| `--retry`    | `-r`  | Task ID to retry           |
| `--skip`     | `-s`  | Task ID to skip            |
| `--undo`     | `-u`  | Iteration ID to undo       |
| `--feedback` | `-f`  | Feedback message for retry |
| `--reason`   |       | Reason for skipping        |
| `--force`    |       | Skip confirmation prompts  |
| `--list`     | `-l`  | List fixable issues        |

## Configuration

Ralph uses a `ralph.yaml` configuration file. If not present, sensible defaults are used.

### Example Configuration

```yaml
repo:
  root: "."
  branch_prefix: "ralph/"

tasks:
  backend: "local"
  path: ".ralph/tasks"
  parent_id_file: ".ralph/parent-task-id"

memory:
  progress_file: ".ralph/progress.md"
  archive_dir: ".ralph/archive"
  max_progress_bytes: 1048576 # 1MB
  max_recent_iterations: 20

claude:
  command: ["claude"]
  args: []

verification:
  commands:
    - ["go", "test", "./..."]

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
    max_churn_iterations: 5
    churn_threshold: 3

safety:
  sandbox: false
  allowed_commands:
    - "npm"
    - "go"
    - "git"
```

### Configuration Options

| Section                           | Option | Description                         | Default                 |
| --------------------------------- | ------ | ----------------------------------- | ----------------------- |
| `repo.root`                       |        | Repository root directory           | `.`                     |
| `repo.branch_prefix`              |        | Git branch prefix for features      | `ralph/`                |
| `tasks.backend`                   |        | Task storage backend                | `local`                 |
| `tasks.path`                      |        | Path to task storage directory      | `.ralph/tasks`          |
| `tasks.parent_id_file`            |        | File storing current parent task ID | `.ralph/parent-task-id` |
| `memory.progress_file`            |        | Feature progress log file           | `.ralph/progress.md`    |
| `memory.archive_dir`              |        | Archive directory for old progress  | `.ralph/archive`        |
| `memory.max_progress_bytes`       |        | Max size before pruning             | `1048576` (1MB)         |
| `memory.max_recent_iterations`    |        | Iterations to preserve when pruning | `20`                    |
| `claude.command`                  |        | Claude Code CLI command             | `["claude"]`            |
| `claude.args`                     |        | Additional arguments for Claude     | `[]`                    |
| `verification.commands`           |        | Commands to run for verification    | `[]`                    |
| `loop.max_iterations`             |        | Maximum iterations per run          | `50`                    |
| `loop.max_minutes_per_iteration`  |        | Time limit per iteration            | `20`                    |
| `loop.max_retries`                |        | Max retries for failed tasks        | `2`                     |
| `loop.max_verification_retries`   |        | Max in-iteration fix attempts       | `2`                     |
| `loop.gutter.max_same_failure`    |        | Stop after N same failures          | `3`                     |
| `loop.gutter.max_churn_commits`   |        | Stop after N churn commits          | `2`                     |
| `loop.gutter.max_oscillations`    |        | Stop after N file oscillations      | `2`                     |
| `loop.gutter.enable_content_hash` |        | Use content hashing for oscillation | `true`                  |
| `safety.sandbox`                  |        | Enable sandbox mode                 | `false`                 |
| `safety.allowed_commands`         |        | Allowlist for shell commands        | `["npm", "go", "git"]`  |

### Environment Variables

| Variable         | Required | Description                        |
| ---------------- | -------- | ---------------------------------- |
| `CLAUDE_API_KEY` | Yes      | API key for Claude Code subprocess |

## Task Definition

Tasks are defined in YAML files:

```yaml
tasks:
  - id: feature-root
    title: "My Feature"
    description: "Root task for the feature"
    status: open

  - id: feature-task-1
    title: "Implement component"
    description: "Create the main component"
    parentId: feature-root
    dependsOn: []
    status: open
    acceptance:
      - "Component exists at src/component.ts"
      - "Component exports main function"
    verify:
      - ["go", "test", "./..."]
    labels:
      area: core
      priority: P0
```

### Task Fields

| Field         | Required | Description                                                        |
| ------------- | -------- | ------------------------------------------------------------------ |
| `id`          | Yes      | Unique identifier (kebab-case)                                     |
| `title`       | Yes      | Short summary                                                      |
| `description` | No       | Detailed standalone description                                    |
| `parentId`    | No       | Parent task ID for hierarchy                                       |
| `dependsOn`   | No       | Task IDs that must complete first                                  |
| `status`      | Yes      | `open`, `in_progress`, `completed`, `blocked`, `failed`, `skipped` |
| `acceptance`  | No       | Verifiable acceptance criteria                                     |
| `verify`      | No       | Task-specific verification commands                                |
| `labels`      | No       | Key-value metadata (e.g., area, priority)                          |

## Storage

Ralph stores state in the `.ralph/` directory:

| Path                 | Description                               |
| -------------------- | ----------------------------------------- |
| `.ralph/tasks/`      | Task store (YAML files)                   |
| `.ralph/progress.md` | Feature progress log                      |
| `.ralph/state/`      | Session IDs, pause state, budget tracking |
| `.ralph/logs/`       | Iteration logs                            |
| `.ralph/archive/`    | Archived progress files                   |

## Troubleshooting

### Configuration and CLI Flag Issues

Ensure `ralph.yaml` is valid YAML and follows the documented structure. Run `ralph --help` for command-specific flags.

### Progress File Growth

The progress file is automatically pruned when it exceeds `max_progress_bytes` (default 1MB). Recent iterations (configured by `max_recent_iterations`) are preserved.

### No Ready Tasks

If ralph reports no ready tasks, check that:

- Tasks have `status: open`
- All `dependsOn` tasks are completed
- The parent task has leaf descendants

## Development

### Build

```bash
go build ./...
```

### Test

```bash
go test ./...
```

### Lint

```bash
golangci-lint run
```

### Format

```bash
gofmt -w .
```

## Contributing

Contributions are welcome. Please ensure tests pass before submitting pull requests:

```bash
go test ./...
```

## License

[MIT](LICENSE)
