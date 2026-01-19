# Ralph

Ralph is a Go harness that runs [Claude Code](https://claude.ai) against a repository in a tight loop:

pick a ready task -> ask Claude Code to implement it -> run verification -> commit -> repeat.

It is designed for repos where you want repeatable, auditable agent work: task graphs, verification gates, git commits, and a local state directory you can inspect and version as needed.

## What "Ralph Wiggum loop" means

Ralph is a loop, not a model.

The core idea is simple: don’t stop at the first attempt. Each iteration produces changes, then verification decides whether the task is actually done (tests pass, acceptance criteria met). If not, the next iteration runs with the failures as feedback. Repeat until it passes, or until a safety limit is hit.

The name is a nod to Ralph Wiggum from The Simpsons: well-meaning, occasionally clueless, but persistent. The technique was popularized as a literal shell loop around an AI coding agent; this project turns that idea into a repo-grade harness with task selection, verification, commits, and on-disk state under `.ralph/`.

## What Ralph does

- Runs an iteration loop that selects a ready leaf task and drives Claude Code to completion.
- Enforces task dependencies (rejects cycles, only schedules tasks whose deps are done).
- Can turn a PRD markdown file into a tasks.yaml-style plan (via Claude Code), then run it.
- Runs verification commands before committing (tests, typecheck, lint, custom commands).
- Keeps state on disk under `.ralph/` so the run is reviewable and resumable.
- Has guardrails for churn and repeated failures (gutter detection), plus optional sandboxing.

## Prerequisites

- Go 1.25.5+
- Claude Code CLI installed and configured (`claude` on your PATH)
- Git

## Install

```bash
go install github.com/yarlson/ralph@latest
```

Build from source:

```bash
git clone https://github.com/yarlson/ralph.git
cd ralph
go build ./...
```

## Quick start

Ralph can start from a PRD, a task YAML, or an existing `.ralph/` state.

### 1) Start from a PRD

```bash
ralph docs/prd.md
```

This flow:

1. Uses Claude Code to propose a task graph from the PRD
2. Imports tasks into the task store
3. Sets the PRD root as the current parent task
4. Starts the loop

### 2) Start from a tasks.yaml

```bash
ralph tasks.yaml
```

### 3) Continue with existing state

```bash
ralph
```

If no parent task is set, Ralph will prompt you to select one (TTY required).

## How the loop chooses work

For the current parent task, Ralph selects a task that is:

- `status: open`
- all `dependsOn` tasks are `completed`
- a leaf in the task hierarchy (no incomplete children)

If more than one task is ready, selection follows the project’s configured policy (and is visible via `ralph status`).

## Usage

### Main command

```bash
ralph [file]
```

The optional file can be:

- a PRD `.md` file (decompose into tasks)
- a task `.yaml` file (import tasks)

Flags (run `ralph --help` for the authoritative list):

| Flag               | Short | Description                              |
| ------------------ | ----- | ---------------------------------------- |
| `--once`           | `-1`  | Run a single iteration                   |
| `--max-iterations` | `-n`  | Max iterations (0 uses config default)   |
| `--parent`         | `-p`  | Explicit parent task ID                  |
| `--branch`         | `-b`  | Git branch override                      |
| `--dry-run`        |       | Show what would be done                  |
| `--config`         |       | Config file path (default: `ralph.yaml`) |

### Status

Shows task counts, the next selected task, and the last iteration outcome:

```bash
ralph status
```

### Fix

Fix failed tasks or undo iterations:

```bash
ralph fix                                      # Interactive (TTY)
ralph fix --list                               # List fixable issues
ralph fix --retry <task-id>                    # Retry a failed task
ralph fix --retry <task-id> --feedback "hint"  # Retry with feedback
ralph fix --skip <task-id>                     # Skip a task
ralph fix --skip <task-id> --reason "reason"   # Skip with reason
ralph fix --undo <iteration-id>                # Undo an iteration
ralph fix --force                              # Skip confirmations
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

Ralph reads `ralph.yaml` from the repo root by default. If the file is missing, it uses defaults.

### Example

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

### Selected options

| Section        | Option                      | Meaning                               | Default                 |
| -------------- | --------------------------- | ------------------------------------- | ----------------------- |
| `repo`         | `root`                      | Repository root                       | `.`                     |
| `repo`         | `branch_prefix`             | Prefix for feature branches           | `ralph/`                |
| `tasks`        | `path`                      | Task store directory                  | `.ralph/tasks`          |
| `tasks`        | `parent_id_file`            | Current parent task ID                | `.ralph/parent-task-id` |
| `memory`       | `progress_file`             | Progress log file                     | `.ralph/progress.md`    |
| `memory`       | `max_progress_bytes`        | Prune progress when above this size   | `1048576`               |
| `claude`       | `command`                   | Claude Code executable                | `["claude"]`            |
| `verification` | `commands`                  | Verification commands                 | `[]`                    |
| `loop`         | `max_iterations`            | Hard stop per run                     | `50`                    |
| `loop`         | `max_minutes_per_iteration` | Per-iteration time limit              | `20`                    |
| `loop.gutter`  | `max_same_failure`          | Stop after repeating the same failure | `3`                     |
| `safety`       | `allowed_commands`          | Allowlist for shell commands          | `["npm", "go", "git"]`  |

### Environment variables

Ralph runs Claude Code as a subprocess. Make sure Claude Code itself is authenticated and can run non-interactively in your environment.

| Variable         | Required | Description                        |
| ---------------- | -------- | ---------------------------------- |
| `CLAUDE_API_KEY` | Yes      | API key for Claude Code subprocess |

## Task format

Tasks live in YAML. A minimal example:

```yaml
tasks:
  - id: feature-root
    title: "My feature"
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

### Fields

| Field         | Required | Notes                                                              |
| ------------- | -------- | ------------------------------------------------------------------ |
| `id`          | Yes      | Unique identifier (kebab-case recommended)                         |
| `title`       | Yes      | Short summary                                                      |
| `description` | No       | Standalone description (Claude should not need extra context)      |
| `parentId`    | No       | Parent task ID                                                     |
| `dependsOn`   | No       | Task IDs that must be `completed` first                            |
| `status`      | Yes      | `open`, `in_progress`, `completed`, `blocked`, `failed`, `skipped` |
| `acceptance`  | No       | Verifiable criteria                                                |
| `verify`      | No       | Task-specific verification commands                                |
| `labels`      | No       | Metadata (area, priority, etc.)                                    |

## Local state and files

Ralph stores state under `.ralph/`:

| Path                 | Purpose                                   |
| -------------------- | ----------------------------------------- |
| `.ralph/tasks/`      | Task store (YAML files)                   |
| `.ralph/progress.md` | Progress log                              |
| `.ralph/state/`      | Session IDs, pause state, budget tracking |
| `.ralph/logs/`       | Iteration logs                            |
| `.ralph/archive/`    | Archived progress files                   |

## Operational notes

- Ralph makes commits. Run it in a clean working tree and review diffs as you would with any contributor.
- Verification is your main safety net. Treat `verification.commands` as mandatory, not optional.
- If you are experimenting on a risky repo, enable sandboxing and keep `allowed_commands` tight.

## Troubleshooting

### Config or flags look wrong

Validate `ralph.yaml` as YAML and check the command help output:

```bash
ralph --help
ralph fix --help
```

### Progress file keeps growing

Ralph prunes `.ralph/progress.md` when it exceeds `memory.max_progress_bytes`. It keeps the most recent iterations as configured by `memory.max_recent_iterations`.

### "No ready tasks"

Check:

- tasks you expect to run have `status: open`
- their `dependsOn` tasks are `completed`
- you are on the right parent task (use `ralph status`)

## Development

```bash
go build ./...
go test ./...
golangci-lint run
gofmt -w .
```

## Contributing

PRs are welcome. Please run tests locally:

```bash
go test ./...
```

## License

[MIT](LICENSE)
