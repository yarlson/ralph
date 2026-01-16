#!/usr/bin/env bash
set -euo pipefail
set -x

read -r -d '' PROMPT <<'EOF' || true
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

have_cmd() { command -v "$1" >/dev/null 2>&1; }

supports_color() {
    [[ -t 1 ]] && have_cmd tput && [[ "$(tput colors 2>/dev/null || echo 0)" -ge 8 ]]
}

if supports_color; then
    C_RESET="$(tput sgr0)"
    C_BOLD="$(tput bold)"
    C_DIM="$(tput dim)"
    C_RED="$(tput setaf 1)"
    C_GREEN="$(tput setaf 2)"
    C_YELLOW="$(tput setaf 3)"
    C_CYAN="$(tput setaf 6)"
else
    C_RESET=""
    C_BOLD=""
    C_DIM=""
    C_RED=""
    C_GREEN=""
    C_YELLOW=""
    C_CYAN=""
fi

banner() {
    local color="$1"
    shift
    local title="$1"
    shift
    local now
    now="$(date '+%Y-%m-%d %H:%M:%S')"
    printf '\n%s%s%s\n' "${color}${C_BOLD}" "================================================================================" "${C_RESET}"
    printf '%s%s%s %s%s\n' "${color}${C_BOLD}" "RALPH" "${C_RESET}" "${color}${C_BOLD}${title}${C_RESET}" "${C_DIM}(${now})${C_RESET}"
    if [[ $# -gt 0 ]]; then
        printf '%s\n' "$*"
    fi
    printf '%s%s%s\n\n' "${color}${C_BOLD}" "================================================================================" "${C_RESET}"
}

fmt_duration() {
    local s="$1"
    local h=$((s / 3600))
    local m=$(((s % 3600) / 60))
    local r=$((s % 60))
    printf '%02d:%02d:%02d' "$h" "$m" "$r"
}

run_id=0

while :; do
    run_id=$((run_id + 1))
    SECONDS=0

    banner "${C_CYAN}" "Starting task run #${run_id}" "Streaming assistant output below…"

    tmp="$(mktemp -t ralph_ndjson.XXXXXX)"

    claude --print \
        --output-format=stream-json \
        --include-partial-messages \
        --verbose \
        --dangerously-skip-permissions \
        -p "$PROMPT" |
        tee "$tmp" |
        stdbuf -oL -eL jq -r '
        select(.type=="assistant")
        | .message.content[]?
        | select(.type=="text")
        | .text
      '

    rc="${PIPESTATUS[0]}"
    if [ "$rc" -ne 0 ]; then
        banner "${C_RED}" "Claude failed (exit code ${rc})" "Aborting."
        rm -f "$tmp"
        exit "$rc"
    fi

    final="$(jq -r 'select(.type=="result") | .result // empty' "$tmp" | tail -n 1)"
    rm -f "$tmp"

    status="$(printf '%s\n' "$final" | tail -n 1)"
    elapsed="$SECONDS"
    elapsed_fmt="$(fmt_duration "$elapsed")"

    banner "${C_GREEN}" "Finished task run #${run_id}" "Time taken: ${elapsed_fmt}"

    if [ "$status" = "RALPH_DONE" ]; then
        say "All tasks are complete. The project is finished."
        exit 0
    fi

    if [ "$status" = "RALPH_BLOCKED" ]; then
        printf '%sRalph is blocked%s (no ready tasks). Review dependencies in tasks.yaml.\n' "${C_YELLOW}${C_BOLD}" "${C_RESET}" >&2
        say "Ralph is blocked. No ready tasks."
        exit 2
    fi

    gic -y
    say "Task completed. Moving to the next one."
done
