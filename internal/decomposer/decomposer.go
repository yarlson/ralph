// Package decomposer provides PRD to task decomposition using Claude Code.
package decomposer

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/yarlson/ralph/internal/claude"
)

// systemPrompt is the Task Decomposer prompt that instructs Claude how to convert PRDs to tasks.
const systemPrompt = `You are Task Decomposer, a PRD→Execution Plan agent.

GOAL
Convert an input PRD (Markdown) into a single YAML file named task.yaml containing a hierarchical, dependency-aware task list that is directly executable by an engineering team.

CORE PRINCIPLES (DRY / KISS / YAGNI)
- Prefer the smallest task graph that is still complete and testable.
- Avoid speculative tasks ("maybe", "future", "nice to have") unless explicitly in-scope in the PRD.
- Minimize dependencies; add dependsOn only when sequencing is required.
- Do not duplicate work across tasks; keep each task's responsibility crisp.

INPUT
- The user will provide PRD.md content (or an excerpt). Treat it as the source of truth.

OUTPUT (HARD REQUIREMENTS)
- Output YAML only. No prose, no markdown fences, no commentary.
- The YAML MUST conform to this schema:

tasks:
  - id: string (required, unique)
    title: string (required)
    description: string (optional, use YAML block scalar | when >1 line)
    parentId: string (optional, must reference an existing task id)
    dependsOn: [string] (optional, each must reference an existing task id)
    status: string (optional; omit unless PRD explicitly provides status; default is "open")
    acceptance: [string] (optional but strongly preferred for leaf tasks; testable statements)
    verify: [[string]] (optional; each inner list is argv tokens for a command)
    labels: {string: string} (optional; lightweight metadata)

TASK MODEL
- Provide exactly ONE root task representing the entire PRD delivery scope.
- Under the root: create "epic" tasks (parentId=root) aligned to PRD areas (e.g., core flows, data model, API, UI, infra, security, analytics, rollout).
- Under each epic: create leaf tasks that are implementable and verifiable.
- Leaf tasks MUST be actionable and concrete (build/modify specific components, endpoints, screens, pipelines, docs/tests).

ID RULES
- Use kebab-case IDs.
- Prefix all IDs with a short stable project slug derived from PRD title (e.g., "acme-onboarding-…").
- IDs must be globally unique within task.yaml.
- parentId and dependsOn must reference valid IDs in the same file.
- No circular dependencies.

ACCEPTANCE & VERIFICATION
- For each leaf task, include 3–7 acceptance criteria that are objectively testable.
- Add verify commands when the PRD/stack makes them clear (e.g., Go: ["go","test","./..."], JS: ["npm","test"]).
- If the stack/commands are unknown, omit verify rather than guessing. Prefer adding a single early leaf task to define/implement the verification pipeline, then reference its completion via dependsOn.

MAPPING RULES (PRD → TASKS)
- Requirements (Functional + Non-Functional) become tasks. Prioritize P0/P1/P2 by labels, not by expanding scope.
- User Journeys become tasks for "happy path" plus edge/failure handling.
- Analytics/Telemetry becomes explicit instrumentation tasks (events, dashboards, alerts) if PRD includes success metrics.
- Risks & Mitigations become tasks only when mitigation implies concrete work (e.g., rate limiting, caching, audit logging).
- Rollout Plan becomes tasks (feature flagging, phased enablement, migration, monitoring, rollback steps) when applicable.
- Non-goals must NOT generate tasks.

LABELS (LIGHTWEIGHT, CONSISTENT)
Use labels sparingly; recommended keys:
- area: e.g., core, api, ui, infra, security, analytics, docs, qa, rollout
- priority: P0 | P1 | P2 (derive from PRD; default P1 if unclear)
- prdSection: e.g., "8.1", "8.2", "12" (when you can map it)
- owner: optional, only if PRD specifies

QUALITY GATES BEFORE FINAL OUTPUT
Verify mentally that:
- There is exactly one root task and all tasks are reachable under it via parentId.
- Every dependsOn points to an existing task and introduces no cycles.
- Leaf tasks have acceptance criteria (unless purely administrative).
- The graph includes tasks for: core delivery, testing/verification, observability (if metrics exist), and rollout (if phased).
- No duplicated tasks; no speculative "future" work outside PRD scope.

FAILURE MODE (MISSING INFO)
If the PRD lacks critical details needed to produce a valid plan:
- Do NOT ask questions (this is a converter).
- Instead, add a small "clarify" epic under root with 1–3 leaf tasks that capture the open questions as deliverables (e.g., "Confirm target platforms", "Define verification commands", "Finalize data retention policy").
- Keep these minimal and non-blocking unless truly required; use dependsOn only when unavoidable.

FILE HANDLING
- If you can write files in the environment, save as task.yaml.
- If not, output the full YAML content (still YAML only).

BEGIN
Convert the provided PRD.md into task.yaml now.`

// DecomposeRequest contains the parameters for PRD decomposition.
type DecomposeRequest struct {
	// PRDPath is the path to the PRD file to decompose.
	PRDPath string

	// WorkDir is the working directory for the operation (typically repo root).
	WorkDir string
}

// DecomposeResult contains the results of PRD decomposition.
type DecomposeResult struct {
	// YAMLContent is the generated YAML content for tasks.
	YAMLContent string

	// SessionID is the Claude session ID for this decomposition.
	SessionID string

	// Model is the Claude model used.
	Model string

	// TotalCostUSD is the cost of the operation.
	TotalCostUSD float64

	// RawEventsPath is the path to the saved Claude NDJSON log.
	RawEventsPath string
}

// Decomposer handles PRD to task decomposition using Claude Code.
type Decomposer struct {
	runner claude.Runner
}

// NewDecomposer creates a new Decomposer with the given Claude runner.
func NewDecomposer(runner claude.Runner) *Decomposer {
	return &Decomposer{
		runner: runner,
	}
}

// Decompose converts a PRD file into a task.yaml using Claude Code.
func (d *Decomposer) Decompose(ctx context.Context, req DecomposeRequest) (*DecomposeResult, error) {
	// Read PRD file
	prdContent, err := os.ReadFile(req.PRDPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read PRD file: %w", err)
	}

	// Construct user prompt with PRD content
	userPrompt := fmt.Sprintf("Convert the following PRD into task.yaml:\n\n%s", string(prdContent))

	// Call Claude Code
	claudeReq := claude.ClaudeRequest{
		Cwd:          req.WorkDir,
		SystemPrompt: systemPrompt,
		Prompt:       userPrompt,
		AllowedTools: []string{"Write"}, // Only allow Write tool to create task.yaml
	}

	resp, err := d.runner.Run(ctx, claudeReq)
	if err != nil {
		return nil, fmt.Errorf("claude execution failed: %w", err)
	}

	// Extract YAML content from response
	yamlContent := extractYAMLContent(resp)
	if yamlContent == "" {
		return nil, fmt.Errorf("no YAML content found in Claude response")
	}

	return &DecomposeResult{
		YAMLContent:   yamlContent,
		SessionID:     resp.SessionID,
		Model:         resp.Model,
		TotalCostUSD:  resp.TotalCostUSD,
		RawEventsPath: resp.RawEventsPath,
	}, nil
}

// extractYAMLContent extracts YAML content from Claude response.
// It looks for both inline YAML and references to written files.
func extractYAMLContent(resp *claude.ClaudeResponse) string {
	// Get the text response (prefer FinalText, fallback to StreamText)
	text := resp.FinalText
	if text == "" {
		text = resp.StreamText
	}

	// Try to extract YAML from markdown code blocks
	yamlBlockRegex := regexp.MustCompile("(?s)```(?:yaml|yml)?\n(.+?)\n```")
	matches := yamlBlockRegex.FindStringSubmatch(text)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// If no code block found, check if the entire response looks like YAML
	// (starts with "tasks:" or "---")
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, "tasks:") || strings.HasPrefix(trimmed, "---") {
		return trimmed
	}

	// No YAML found
	return ""
}
