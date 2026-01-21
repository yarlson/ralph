// Package decomposer provides PRD to task decomposition using Claude Code.
package decomposer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/yarlson/ralph/internal/claude"
	"github.com/yarlson/ralph/internal/config"
	"github.com/yarlson/ralph/internal/taskstore"
)

// maxValidationRetries is the maximum number of times to retry validation with Claude fixing errors.
const maxValidationRetries = 2

// getSystemPrompt returns the Task Decomposer prompt that instructs Claude how to convert PRDs to tasks.
func getSystemPrompt() string {
	return `You are Task Decomposer, a PRD→Execution Plan agent.

GOAL
Convert an input PRD (Markdown) into a single YAML file at ` + config.DefaultTasksFile + ` containing a hierarchical, dependency-aware task list that is directly executable by autonomous Claude Code sessions.

EXECUTION MODEL (CRITICAL)
- Each leaf task will be executed by a separate Claude Code session running autonomously.
- Claude Code cannot ask questions, make decisions, or request clarification during execution.
- Tasks must be fully self-contained with all context needed for implementation.
- If the PRD has ambiguity, YOU must make the decision now—not defer to a "clarify" task.

CORE PRINCIPLES (DRY / KISS / YAGNI)
- Prefer the smallest task graph that is still complete and testable.
- Avoid speculative tasks ("maybe", "future", "nice to have") unless explicitly in-scope in the PRD.
- Use dependsOn liberally to enforce correct execution order (see DEPENDENCY ORDERING below).
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

TASK ATOMICITY (CRITICAL)
- Each leaf task MUST modify at most 2-3 files. If more files needed, split into multiple tasks.
- One task = one logical unit of work that can be completed in a single Claude Code session.
- If a PRD requirement implies N separate items (e.g., "implement 10 API endpoints"), create N separate leaf tasks, not one task with N acceptance criteria.
- Rule of thumb: if acceptance criteria list > 5 items that each require code changes, the task is too large.

FILE EXPLICITNESS (CRITICAL)
- Every leaf task description MUST specify exact file paths to create or modify.
- Format: "Create cmd/fix.go" or "Modify internal/state/state.go"
- Acceptance criteria SHOULD reference specific files: "cmd/fix.go exists with FixCmd struct"
- Claude Code needs explicit targets—vague descriptions like "create the component" are insufficient.

REQUIRED STEPS FROM PRD (CRITICAL)
- If the PRD includes required steps or procedures, carry them into the leaf task descriptions.
- Preserve explicit steps like reading particular files (e.g., "Read internal/selector/graph.go") and include them verbatim in the task description.
- Treat required steps as mandatory context, not optional guidance.

ID RULES
- Use kebab-case IDs.
- Prefix all IDs with a short stable project slug derived from PRD title (e.g., "acme-onboarding-…").
- IDs must be globally unique within the tasks file.
- parentId and dependsOn must reference valid IDs in the same file.
- No circular dependencies.

ACCEPTANCE & VERIFICATION
- Tests are PART of implementation, not separate tasks. Every implementation task should include writing/updating tests.
- For each leaf task, include 3–5 acceptance criteria that are objectively testable.
- Acceptance criteria must be verifiable by examining code or running tests—no subjective criteria.
- Acceptance criteria for implementation tasks MUST include test requirements (e.g., "Unit tests for X pass", "Test coverage for Y exists").
- Only leaf tasks (tasks with no children) require verify commands. Epic tasks (tasks with children) do NOT need verify commands since they are organizational containers—their children handle verification.
- Add verify commands when the PRD/stack makes them clear (e.g., Go: ["go","test","./..."], JS: ["npm","test"]).
- If the stack/commands are unknown, omit verify rather than guessing.
- Example format with integrated tests:
  acceptance:
    - "cmd/handler.go implements HandleRequest function"
    - "internal/handler/handler_test.go contains tests for HandleRequest"
    - "go test ./internal/handler/... passes"
    - "Error cases are handled and tested"

MAPPING RULES (PRD → TASKS)
- Requirements (Functional + Non-Functional) become tasks. Prioritize P0/P1/P2 by labels, not by expanding scope.
- User Journeys become tasks for "happy path" plus edge/failure handling.
- Analytics/Telemetry becomes explicit instrumentation tasks (events, dashboards, alerts) if PRD includes success metrics.
- Risks & Mitigations become tasks only when mitigation implies concrete work (e.g., rate limiting, caching, audit logging).
- Rollout Plan becomes tasks (feature flagging, phased enablement, migration, monitoring, rollback steps) when applicable.
- Non-goals must NOT generate tasks.

DEPENDENCY ORDERING (STRICT SEQUENCING)
- Scaffolding/infrastructure tasks (project setup, config, CI/CD, database schema) MUST complete before feature work begins. Add dependsOn from all feature tasks to relevant scaffolding tasks.
- Within an epic, order tasks sequentially via dependsOn chains when execution order matters. Do NOT leave sibling tasks independent if one logically precedes another.
- Prefer explicit dependsOn over relying on priority labels or creation order. The task selector only considers tasks whose dependencies are ALL completed.
- Example: if "setup-database" and "implement-user-auth" are both P0, implement-user-auth MUST have dependsOn: [setup-database].
- Rule: A task should only become "ready" (no unmet dependencies) when it is truly safe to execute in isolation.
- Common dependency chains:
  - project-scaffold → core-models → api-endpoints → ui-components
  - ci-setup → all tasks with verify commands
  - database-schema → any task that reads/writes data

LABELS (LIGHTWEIGHT, CONSISTENT)
Use labels sparingly; recommended keys:
- area: e.g., core, api, ui, infra, security, analytics, docs, qa, rollout
- priority: P0 | P1 | P2 (derive from PRD; default P1 if unclear)
- prdSection: e.g., "8.1", "8.2", "12" (when you can map it)

FORBIDDEN PATTERNS
Never generate tasks that:
- Require human decisions: "Decide whether to...", "Choose between...", "Clarify..."
- Are conditional: "If X then implement Y, otherwise Z"
- Are open-ended research: "Investigate...", "Explore options for..."
- Bundle multiple unrelated changes: "Implement A, B, C, and D" (split into 4 tasks)
- Lack file specificity: "Update the handlers" (which handlers? which files?)
- Have vague acceptance: "Code is clean", "Performance is good" (not objectively testable)
- Test-only tasks: "Write tests for...", "Add unit tests", "Create test suite" (tests MUST be part of implementation tasks)
- Validation-only tasks: "Run linter", "Verify build", "Run tests" (verification is part of implementation, not separate)
- Tests and verification MUST be part of implementation tasks, not separate tasks
- Each implementation task includes: code changes + tests + verification in acceptance criteria

AMBIGUITY RESOLUTION
If the PRD is ambiguous or has open questions:
- Make a reasonable decision based on common practice and state it in the task description.
- Example: PRD says "add caching" but doesn't specify where → decide "Use in-memory cache for API responses" and document in task.
- Do NOT create "clarify" or "decide" tasks—Claude Code cannot make decisions during execution.
- Prefer the simpler option when multiple approaches are valid.

QUALITY GATES BEFORE FINAL OUTPUT
Verify mentally that:
- There is exactly one root task and all tasks are reachable under it via parentId.
- Every dependsOn points to an existing task and introduces no cycles.
- Every leaf task specifies files to create/modify.
- Every leaf task has 3-5 testable acceptance criteria.
- No leaf task modifies more than 3 files (split if needed).
- No tasks require decisions, research, or clarification.
- The graph includes tasks for: core delivery, testing/verification, observability (if metrics exist), and rollout (if phased).
- No duplicated tasks; no speculative "future" work outside PRD scope.
- Scaffolding/infrastructure tasks have NO dependencies on feature tasks (they come first).
- Feature tasks depend on their prerequisite scaffolding tasks via dependsOn.
- Sibling tasks within an epic are chained via dependsOn when order matters (not left as independent parallels).

FILE HANDLING
- If you can write files in the environment, save as ` + config.DefaultTasksFile + `.
- If not, output the full YAML content (still YAML only).

BEGIN
Convert the provided PRD.md into ` + config.DefaultTasksFile + ` now.`
}

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

	// OutputPath is the path where tasks.yaml was written.
	OutputPath string
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

	// Determine output path - use WorkDir as base if provided
	outputPath := config.DefaultTasksFile
	if req.WorkDir != "" {
		outputPath = filepath.Join(req.WorkDir, config.DefaultTasksFile)
	}

	// Construct user prompt with PRD content
	userPrompt := fmt.Sprintf("Convert the following PRD into %s:\n\n%s", config.DefaultTasksFile, string(prdContent))

	// Call Claude Code
	claudeReq := claude.ClaudeRequest{
		Cwd:          req.WorkDir,
		SystemPrompt: getSystemPrompt(),
		Prompt:       userPrompt,
		AllowedTools: []string{"Write"}, // Only allow Write tool to create tasks file
	}

	resp, err := d.runner.Run(ctx, claudeReq)
	if err != nil {
		return nil, fmt.Errorf("claude execution failed: %w", err)
	}

	// Try to get YAML content - first check if Claude wrote the file directly
	var yamlContent string
	if fileContent, err := os.ReadFile(outputPath); err == nil {
		// File was created by Claude using Write tool
		yamlContent = string(fileContent)
	} else {
		// File wasn't created, try to extract YAML from response text
		yamlContent = extractYAMLContent(resp)
	}

	if yamlContent == "" {
		return nil, fmt.Errorf("no YAML content found: file not created and no YAML in response")
	}

	// Validate YAML and retry if needed
	validatedYAML, err := d.validateAndRetry(ctx, string(prdContent), yamlContent)
	if err != nil {
		return nil, fmt.Errorf("YAML validation failed: %w", err)
	}

	// Create the tasks directory if it doesn't exist
	tasksDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create tasks directory: %w", err)
	}

	// Write the validated YAML to the output file (overwrites if Claude created it)
	if err := os.WriteFile(outputPath, []byte(validatedYAML), 0644); err != nil {
		return nil, fmt.Errorf("failed to write tasks file: %w", err)
	}

	return &DecomposeResult{
		YAMLContent:   validatedYAML,
		SessionID:     resp.SessionID,
		Model:         resp.Model,
		TotalCostUSD:  resp.TotalCostUSD,
		RawEventsPath: resp.RawEventsPath,
		OutputPath:    outputPath,
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

// fixPromptTemplate is the prompt template for asking Claude to fix YAML validation errors.
const fixPromptTemplate = `The following task YAML was generated from this PRD but has validation errors.
Please fix the YAML and output ONLY the corrected YAML (no explanations).

## Original PRD:
%s

## Failed YAML:
%s

## Validation Errors:
%s

Output the corrected YAML only:`

// validateAndRetry validates YAML content and retries with Claude if there are errors.
// It parses the YAML, converts to tasks, and runs the linter.
// If validation fails, it asks Claude to fix the YAML and retries up to maxValidationRetries times.
func (d *Decomposer) validateAndRetry(ctx context.Context, prdContent, yamlContent string) (string, error) {
	currentYAML := yamlContent

	for attempt := 0; attempt <= maxValidationRetries; attempt++ {
		// Parse YAML
		yamlFile, err := taskstore.ParseYAML([]byte(currentYAML))
		if err != nil {
			// YAML syntax error - ask Claude to fix
			if attempt >= maxValidationRetries {
				return "", fmt.Errorf("validation failed after %d retries: YAML parse error: %w", maxValidationRetries, err)
			}
			fixedYAML, fixErr := d.askClaudeToFix(ctx, prdContent, currentYAML, err.Error())
			if fixErr != nil {
				return "", fixErr
			}
			currentYAML = fixedYAML
			continue
		}

		// Convert YAMLTasks to Tasks for linting
		tasks := make([]*taskstore.Task, 0, len(yamlFile.Tasks))
		for _, yt := range yamlFile.Tasks {
			task := convertYAMLTaskToTask(yt)
			tasks = append(tasks, task)
		}

		// Run linter
		lintResult := taskstore.LintTaskSet(tasks)
		if lintResult.Valid {
			return currentYAML, nil
		}

		// Validation failed - collect errors
		if attempt >= maxValidationRetries {
			return "", fmt.Errorf("validation failed after %d retries: %v", maxValidationRetries, lintResult.Error())
		}

		// Ask Claude to fix
		errMsg := lintResult.Error().Error()
		fixedYAML, fixErr := d.askClaudeToFix(ctx, prdContent, currentYAML, errMsg)
		if fixErr != nil {
			return "", fixErr
		}
		currentYAML = fixedYAML
	}

	return "", fmt.Errorf("validation failed after %d retries", maxValidationRetries)
}

// askClaudeToFix asks Claude to fix YAML validation errors.
func (d *Decomposer) askClaudeToFix(ctx context.Context, prdContent, yamlContent, errorMsg string) (string, error) {
	fixPrompt := fmt.Sprintf(fixPromptTemplate, prdContent, yamlContent, errorMsg)

	req := claude.ClaudeRequest{
		SystemPrompt: getSystemPrompt(),
		Prompt:       fixPrompt,
		AllowedTools: []string{}, // No tools needed for text-only response
	}

	resp, err := d.runner.Run(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to get fixed YAML from Claude: %w", err)
	}

	fixedYAML := extractYAMLContent(resp)
	if fixedYAML == "" {
		// Use raw response if no YAML block found
		fixedYAML = strings.TrimSpace(resp.FinalText)
		if fixedYAML == "" {
			fixedYAML = strings.TrimSpace(resp.StreamText)
		}
	}

	return fixedYAML, nil
}

// convertYAMLTaskToTask converts a YAMLTask to a Task for linting purposes.
func convertYAMLTaskToTask(yt taskstore.YAMLTask) *taskstore.Task {
	now := time.Now()
	task := &taskstore.Task{
		ID:          yt.ID,
		Title:       yt.Title,
		Description: yt.Description,
		DependsOn:   yt.DependsOn,
		Acceptance:  yt.Acceptance,
		Verify:      yt.Verify,
		Labels:      yt.Labels,
		Status:      taskstore.StatusOpen,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if yt.ParentID != "" {
		task.ParentID = &yt.ParentID
	}

	if yt.Status != "" {
		task.Status = taskstore.TaskStatus(yt.Status)
	}

	return task
}
