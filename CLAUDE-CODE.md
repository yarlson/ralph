## Claude Code Execution and Stream-JSON Parsing

### Principal Engineer Handoff Document (Harness Integration)

This document specifies how the Go-based Ralph harness must execute the **real Claude Code binary**, parse `--output-format="stream-json"` reliably, and manage sessions via `--continue`.

It is written to be implementation-ready: command contracts, parsing rules, state files, and failure modes.

---

## 1) Goals and Scope

### Goals

- Execute `claude` as a subprocess from Go.
- Support:
  - **fresh sessions**
  - **continued sessions** (`--continue`) using the same `session_id`

- Parse streaming JSON lines emitted by Claude Code (`--output-format="stream-json"`).
- Extract:
  - `session_id`
  - tool availability / metadata (init)
  - assistant output (final result text)
  - usage and cost telemetry (for budgets)
  - permission denials / errors

- Provide a stable internal interface for the Ralph harness iteration loop.

### Non-goals

- We do not implement Claude tools ourselves (Claude Code owns that).
- We do not depend on Claude’s human-facing `--verbose` formatting; only stream-json lines are authoritative.
- We do not rely on Claude memory beyond `--continue` semantics; repo state remains source of truth.

---

## 2) Claude CLI Contract (Observed)

Example invocation you provided (canonical baseline):

```bash
claude \
  --dangerously-skip-permissions \
  --system-prompt "$(cat ~/prd_planner_system.txt)" \
  --allowedTools "Read,Edit,Bash" \
  --output-format="stream-json" \
  --verbose \
  -p "hello" \
  --continue
```

### Key flags we will use

- `--output-format="stream-json"`: **required** for machine parsing.
- `--continue`: instructs Claude to continue an existing session. In your output, session continuity is achieved by Claude reusing the same `session_id` (it prints it in `system/init`).
- `--system-prompt <string>`: system prompt injection.
- `--allowedTools "<comma-separated>"`: restrict tool surface (recommended).
- `--dangerously-skip-permissions`: bypass permission prompts (recommended only in sandbox or controlled env).
- `--verbose`: emits additional content; still safe if we parse JSON lines only.

### Input prompt

- `-p "<prompt>"` is used for the user message content.

### Working directory matters

Claude’s `system/init` includes `"cwd": "..."`. The harness must set the subprocess working directory to the repo root (or task workspace) to ensure deterministic tool paths.

---

## 3) Stream-JSON Event Model

Claude emits one JSON object per line (NDJSON). You must parse line-by-line from stdout.

From your sample, the important event types:

### 3.1 `system/init`

Example:

```json
{"type":"system","subtype":"init","cwd":"/Users/yar/ttt/ttt/ttt","session_id":"...","tools":[...],"model":"...","claude_code_version":"2.1.9",...}
```

Use this to capture:

- `session_id` (primary handle)
- `cwd` (sanity check)
- `model` (telemetry)
- tool list (optional for auditing)
- `claude_code_version` (for compatibility tracking)
- permission mode (audit)

### 3.2 `assistant/message`

Example:

```json
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"..."}], "usage": {...}}, "session_id":"...", ...}
```

This carries incremental assistant content. You should:

- accumulate any `content[].text` into a streaming buffer, preserving order
- ignore non-text content types unless you explicitly support them later

Also capture `usage` if present.

### 3.3 `result/success` (terminal)

Example:

```json
{"type":"result","subtype":"success","result":"...","session_id":"...","total_cost_usd":0.009631,"usage":{...},"permission_denials":[]}
```

This is the **terminal record** for the command execution and is the authoritative “final text” for the run:

- `result`: final combined assistant output (string)
- `duration_ms`, `num_turns`: telemetry
- `total_cost_usd`, `usage`: budget/accounting inputs
- `permission_denials`: important for safety

There can also be `result/subtype:"error"` (or `is_error:true`) variants; handle generically.

---

## 4) Session Semantics (`--continue`)

### What `--continue` means operationally

- With `--continue`, Claude loads prior session context and continues the same thread.
- The stream includes the same `session_id` in `system/init` and later messages.
- **Your harness must treat `session_id` as the durable session key**.

### Harness-level session policy

For the Ralph loop harness, treat each **task iteration** as:

- Prefer: **fresh Claude session** for coding tasks (aligns with Ralph memory hygiene), unless you deliberately choose to “continue” within a single iteration for multi-turn clarification.
- Allowed: continue within the same iteration to ask clarifying questions or handle follow-ups (like your PRD planner example).
- Not recommended: continuing the same session across multiple leaf tasks, unless you have a strong reason. Repo state already carries continuity; carrying conversational state increases drift risk.

### Required state file

Store per-run session IDs in `.ralph/state/claude-session.json`:

```json
{
  "planner_session_id": "10f6d...",
  "coder_session_id": "abcd...",
  "updated_at": "2026-01-16T..."
}
```

Even if you mostly run fresh sessions, storing IDs is valuable for:

- debugging reproducibility
- optional `--continue` flows
- postmortems

---

## 5) Go Integration Design

### 5.1 Public interface in the harness (proposed)

```go
type ClaudeRunner interface {
  Run(ctx context.Context, req ClaudeRequest) (*ClaudeResponse, error)
}

type ClaudeRequest struct {
  Cwd          string            // repo/workspace directory
  SystemPrompt string            // fully rendered system prompt text
  AllowedTools []string          // e.g. ["Read","Edit","Bash"]
  Prompt       string            // the user message to -p
  Continue     bool              // maps to --continue
  ExtraArgs    []string          // escape hatch
  Env          map[string]string // additional env vars
}

type ClaudeResponse struct {
  SessionID   string
  Model       string
  Version     string // claude_code_version
  FinalText   string // from result.result
  StreamText  string // concatenation of assistant/message text blocks (optional)
  Usage       ClaudeUsage
  TotalCostUSD float64
  PermissionDenials []string
  RawEventsPath string // path to NDJSON log for audit/replay
}
```

### 5.2 Subprocess execution rules

- Use `exec.CommandContext` so cancellation kills the process.
- Set:
  - `cmd.Dir = req.Cwd`
  - `cmd.Env = os.Environ() + req.Env`

- Capture:
  - stdout as stream
  - stderr separately for diagnostics (but do not parse stderr as JSON)

### 5.3 Always log raw NDJSON

Write stdout lines to `.ralph/logs/claude/<timestamp>-<mode>-<task>.ndjson` verbatim.

This is non-negotiable for debugging: you will need to replay parse failures and verify contract changes.

---

## 6) Parsing Requirements (NDJSON)

### 6.1 Parser model

- Read stdout line-by-line using a buffered scanner or `bufio.Reader.ReadString('\n')`.
- Each line must be parsed as independent JSON.
- Do not assume ordering beyond:
  - system/init appears early
  - result/\* appears once at the end

### 6.2 Robustness constraints

- Some lines may be large. `bufio.Scanner` has token limits; set a larger buffer:
  - `scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)` (tune as needed)

- If JSON parsing fails on a line:
  - record parse error
  - keep the raw line in logs
  - attempt to continue parsing subsequent lines
  - fail the run only if terminal `result` is never successfully parsed

### 6.3 Event extraction rules

Maintain a struct:

- `sessionID` from first `system/init` encountered
- `model`, `claude_code_version` from init (if present)
- `streamText` append from each `assistant/message`
- `finalText` from `result.result` (authoritative)
- `usage` and `total_cost_usd` from result if present
- `permission_denials` from result

At end-of-process:

- If `finalText` is empty but `streamText` exists, return `streamText` as fallback (and mark response as “degraded” in logs).
- If neither exists, return error.

### 6.4 Schema tolerance

Treat unknown fields as opaque.
Only bind required fields. Use `map[string]any` or partial structs.

---

## 7) Command Builder (Exact)

### 7.1 Recommended baseline command

For deterministic machine-driven runs:

```bash
claude \
  --output-format="stream-json" \
  --system-prompt "<systemPrompt>" \
  --allowedTools "Read,Edit,Bash" \
  -p "<prompt>"
```

Optional but common:

- `--dangerously-skip-permissions` (sandbox only)
- `--continue` (when continuing a session)
- `--verbose` (not required if you parse JSON only)

### 7.2 Tool allowlisting

Enforce a small, explicit allowlist per mode:

**Planner/PRD mode:** `Read,Bash` (maybe `WebSearch` if you permit)
**Coder mode:** `Read,Edit,Write,Bash,Glob,Grep` (tighten to your environment)

Prefer to include `Write` explicitly if Claude uses it; your sample allowed `Edit` only, but init lists many tools—Claude may still function; nonetheless, your harness should control this intentionally.

---

## 8) State Management in the Harness

### 8.1 Files

- `.ralph/state/claude-session.json`
- `.ralph/logs/claude/*.ndjson`
- `.ralph/logs/claude/*.stderr` (optional)

### 8.2 Session usage policy

- If `req.Continue == true`:
  - you must pass `--continue`
  - you should expect same `session_id` in init
  - if a new session_id appears, treat it as “session fork” and update state

### 8.3 Concurrency policy

Do not run multiple Claude subprocesses concurrently in the same workspace branch unless you implement isolation:

- separate branches per worker
- separate working directories
- separate `.ralph/state` files

MVP: single-threaded loop only.

---

## 9) Error Handling and Retries

### 9.1 Categories

1. **Process failure**: non-zero exit code, killed by context timeout.
2. **Parse failure**: NDJSON malformed; missing `result`.
3. **Permission denials**: `permission_denials` non-empty.
4. **Tool errors**: Claude tool failures surfaced in text; treat as normal unless they block verification.
5. **Budget exceeded**: harness stops based on time/iterations/cost.

### 9.2 Retry policy (recommended)

- Do not automatically retry Claude invocation on semantic failures (it can worsen churn).
- You may retry **once** on:
  - transient parse issues (rare)
  - process crash

- If permission denials occur:
  - fail fast unless your policy allows permission prompts (you are bypassing permissions, so denials should be actionable signal).

---

## 10) Budget and Telemetry

Use `result.total_cost_usd` (if present) as the accounting source.
Maintain `.ralph/state/budget.json`:

```json
{
  "total_cost_usd": 1.234,
  "max_cost_usd": 10.0,
  "iterations": 17,
  "max_iterations": 50
}
```

If `total_cost_usd` is absent for some reason:

- fall back to time/iterations budgets.

---

## 11) Security Considerations

- `--dangerously-skip-permissions` is acceptable only when:
  - run in a sandbox (container/VM)
  - command allowlist is enforced for verification steps
  - repository contains no secrets (or secrets are masked)

- Never embed secrets in:
  - system prompt
  - user prompt
  - logged NDJSON (mask via env injection rather than prompt content)

---

## 12) Concrete Implementation Sketch (Parsing)

This is not full code, but enough to implement correctly.

Core loop:

- spawn process
- read stdout lines
- write each line to NDJSON log
- parse JSON and update accumulator
- wait for exit
- validate final result

Parsing strategy:

- partial struct:
  - `Type string 'json:"type"'`
  - `Subtype string 'json:"subtype"'`

- switch on `Type`:
  - `system` + `subtype=init`: parse init fields
  - `assistant`: parse `message.content[]` and append text
  - `result`: parse `result`, `total_cost_usd`, `usage`, `permission_denials`

---

## 13) Harness-Specific Recommendation: Session Use in Ralph

For the Ralph loop harness (task-by-task autonomy):

- **Default**: do not use `--continue` across tasks. Use fresh sessions and rely on repo state + progress.md patterns.
- **Use `--continue` only**:
  - within a single planning flow where Claude asks clarifying questions
  - within a single task iteration if Claude needs additional user answers (AskUserQuestion) and you choose to keep that conversational state

This keeps the “Ralph hygiene” property: prompt memory stays clean, while the environment (git/files) persists.

---

## 14) Test Plan (Must-Haves)

1. **Happy path**: parse init + assistant + result success; extract session_id and final result.
2. **Large lines**: ensure scanner buffer handles long JSON.
3. **Missing result**: process exits but no result line → return error with log path.
4. **Permission denials**: capture and fail fast.
5. **Continue**: run two invocations with `--continue`, verify same session_id (or detect fork).
6. **Timeout**: kill process via context; ensure cleanup and log preserved.

---

## 15) Deliverable Checklist

- [ ] `internal/claude/runner.go` (subprocess runner)
- [ ] `internal/claude/ndjson_parser.go` (event parsing)
- [ ] `.ralph/logs/claude/` raw NDJSON logging
- [ ] `.ralph/state/claude-session.json` session persistence
- [ ] CLI flags mapping (`--continue`, tool allowlist, system prompt)
- [ ] Unit tests for parser with captured fixtures (use your sample NDJSON)

---

If you want the next step, I can produce:

- a precise **Go struct schema** for the events (minimal + tolerant),
- a reference implementation of the runner/parser (production-grade),
- and a “Claude prompt packager” spec that inserts verification failures and progress patterns without exceeding caps.
