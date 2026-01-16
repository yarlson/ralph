# PRD discrepancies

This list compares PRD.md requirements to the current implementation.

## CLI and config
- `--config` flag is defined but ignored; commands always call `config.LoadConfig(workDir)` from the cwd and never read `cfgFile` (cmd/root.go, cmd/init.go, cmd/run.go, cmd/status.go, cmd/report.go, cmd/retry.go, cmd/skip.go).
- Missing `ralph logs` and `ralph revert` commands (cmd/).
- `ralph pause` sets a flag but the run loop never checks it between iterations; it is only checked at the start of `ralph run` (cmd/pause.go, cmd/run.go, internal/loop/controller.go).
- `ralph retry` stores feedback but the loop never reads it or includes it in prompts (cmd/retry.go, internal/loop/controller.go).
- `claude.command`/`claude.args` are not fully honored; only the first command element is used and args are ignored (cmd/run.go).

## Task storage and status
- Local store writes JSON files; no CLI or automatic YAML import from `.ralph/tasks` even though YAML import exists and README/PRD mention YAML (internal/taskstore/local.go, internal/taskstore/yaml.go, cmd/).
- Task statuses `failed` and `blocked` are never set by the loop; failures reset to `open`, so reports/status do not reflect blocked/failed tasks (internal/loop/controller.go).
- No task linter to enforce acceptance/verify fields (PRD 8.9/14).

## Loop orchestration and prompts
- The iteration uses a minimal prompt and does not use the `internal/prompt` builder; system prompt rules, Codebase Patterns, diff stats, failure output, and retry-specific fix-only prompts are not included (internal/loop/controller.go, internal/prompt/iteration.go, internal/prompt/retry.go).
- No in-iteration verification retry loop; Claude runs once per iteration and a failure ends the iteration without a fix-only retry within budget (internal/loop/controller.go).
- `max_minutes_per_iteration` is configured but not enforced; budget tracker ignores per-iteration timing and no context timeouts are set (internal/loop/budget.go, internal/loop/controller.go).

## Verification and safety
- Config-level `verification.commands` are ignored; only task-level `verify` commands are run (internal/loop/controller.go, internal/config/config.go).
- Trimmed verification output and retry feedback utilities exist but are not used; raw output is stored and no retry prompt uses it (internal/verifier/trimmer.go, internal/loop/controller.go).
- `safety.sandbox` flag is unused; allowed command list only gates verification, not Claude invocations or other shelling (internal/config/config.go, internal/verifier/runner.go).

## Git discipline and reporting
- Feature branch creation is not used; `EnsureBranch` exists but is never called, so runs stay on whatever branch is active (internal/git/manager.go, internal/loop/controller.go).
- Iteration logs are JSON only; no `iteration-<n>.txt` human log file (internal/loop/record.go).
- Report commits do not include commit messages; `CommitInfo.Message` is never populated (internal/reporter/report.go).

## Memory hygiene
- Progress file archive and size pruning exist but are never invoked; old progress is not archived and logs can grow unbounded (internal/memory/archive.go, internal/memory/progress.go, internal/loop/controller.go).
- Codebase Patterns from progress are not read and not injected into prompts, and progress entries omit learnings (internal/memory/progress.go, internal/loop/controller.go).
- No AGENTS.md updates or enforcement in prompts (internal/prompt/iteration.go is unused).
