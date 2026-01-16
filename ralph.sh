#!/usr/bin/env bash
set -euo pipefail

PROMPT="$(cat <<'EOF'
Read PRD.md, tasks.yaml, and .ralph/progress.md.

Pick exactly ONE task from tasks.yaml:
- completed: false
- all dependsOn tasks have completed: true
- if multiple, pick highest importance (else first in file)

Implement ONLY that task.

Verify (must pass before completion):
- npm run typecheck if present
- npm test if present
- go test ./... if Go repo or Go code touched

If verification fails: fix and re-run until it passes.

Then:
- Mark that task completed: true in tasks.yaml (only that task)
- Append to .ralph/progress.md: what changed, files touched, learnings/gotchas
- Update CLAUDE.md only with durable guidance (no task-specific notes)

Print a brief summary.

If ALL tasks in tasks.yaml are completed, print RALPH_DONE as the LAST line and stop.
If no ready tasks exist but some are incomplete, print RALPH_BLOCKED as the LAST line and stop (include blocked task ids/titles above it).
Do not ask the user questions. Stop.
EOF
)"

while :; do
  tmp="$(mktemp -t ralph_ndjson.XXXXXX)"

  # Stream assistant text live, but also keep the full NDJSON for extracting the terminal result.
  claude --print \
    --output-format=stream-json \
    --include-partial-messages \
    --verbose \
    --dangerously-skip-permissions \
    -p "$PROMPT" \
    | tee "$tmp" \
    | stdbuf -oL -eL jq -r '
        select(.type=="assistant")
        | .message.content[]?
        | select(.type=="text")
        | .text
      '

  rc="${PIPESTATUS[0]}"
  if [ "$rc" -ne 0 ]; then
    echo "claude exited with code $rc" >&2
    rm -f "$tmp"
    exit "$rc"
  fi

  # Get the final aggregated text from the terminal result event (authoritative).
  final="$(jq -r 'select(.type=="result") | .result // empty' "$tmp" | tail -n 1)"
  rm -f "$tmp"

  status="$(printf '%s\n' "$final" | tail -n 1)"
  if [ "$status" = "RALPH_DONE" ]; then
    exit 0
  fi
  if [ "$status" = "RALPH_BLOCKED" ]; then
    echo "Ralph blocked (no ready tasks)." >&2
    exit 2
  fi
  gic -y
done

