package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	cmdinternal "github.com/yarlson/ralph/cmd/internal"
	"github.com/yarlson/ralph/internal/claude"
	"github.com/yarlson/ralph/internal/config"
	"github.com/yarlson/ralph/internal/decomposer"
	"github.com/yarlson/ralph/internal/state"
	"github.com/yarlson/ralph/internal/taskstore"
)

// realDecomposer wraps the actual decomposer for the PRD pipeline.
type realDecomposer struct {
	prdPath string
	workDir string
	cfg     *config.Config
	output  io.Writer
	result  *decomposer.DecomposeResult
}

func (d *realDecomposer) Decompose(ctx context.Context) (*cmdinternal.DecomposeResultInfo, error) {
	// Create Claude logs directory
	claudeLogsDir := filepath.Join(d.workDir, ".ralph", "logs", "claude")
	if err := os.MkdirAll(claudeLogsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create Claude logs directory: %w", err)
	}

	// Create Claude runner
	claudeCommand := "claude"
	var claudeArgs []string
	if len(d.cfg.Claude.Command) > 0 {
		claudeCommand = d.cfg.Claude.Command[0]
		if len(d.cfg.Claude.Command) > 1 {
			claudeArgs = append(claudeArgs, d.cfg.Claude.Command[1:]...)
		}
	}
	claudeArgs = append(claudeArgs, d.cfg.Claude.Args...)

	runner := claude.NewSubprocessRunner(claudeCommand, claudeLogsDir)
	if len(claudeArgs) > 0 {
		runner = runner.WithBaseArgs(claudeArgs)
	}

	// Create decomposer
	dec := decomposer.NewDecomposer(runner)

	// Set up context with timeout
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// Run decomposition
	_, _ = fmt.Fprintf(d.output, "Using Claude to analyze and generate tasks...\n")

	req := decomposer.DecomposeRequest{
		PRDPath: d.prdPath,
		WorkDir: d.workDir,
	}

	result, err := dec.Decompose(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("decomposition failed: %w", err)
	}

	d.result = result

	// Write YAML to tasks.yaml
	outputPath := filepath.Join(d.workDir, "tasks.yaml")
	if err := os.WriteFile(outputPath, []byte(result.YAMLContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to write tasks file: %w", err)
	}

	// Count tasks in generated YAML
	taskCount := countTasksInYAML(result.YAMLContent)

	_, _ = fmt.Fprintf(d.output, "✓ Generated tasks: %s\n", outputPath)
	_, _ = fmt.Fprintf(d.output, "  Session: %s\n", result.SessionID)
	_, _ = fmt.Fprintf(d.output, "  Model: %s\n", result.Model)
	_, _ = fmt.Fprintf(d.output, "  Cost: $%.4f\n\n", result.TotalCostUSD)

	return &cmdinternal.DecomposeResultInfo{
		TaskCount: taskCount,
		YAMLPath:  outputPath,
	}, nil
}

// realImporter wraps the actual task import for the PRD pipeline.
type realImporter struct {
	workDir  string
	yamlPath string
	cfg      *config.Config
	output   io.Writer
}

func (i *realImporter) Import() (*cmdinternal.ImportResultInfo, error) {
	_, _ = fmt.Fprintf(i.output, "Importing tasks into store...\n")

	// Create task store
	store, err := taskstore.NewLocalStore(i.cfg.Tasks.Path)
	if err != nil {
		return nil, fmt.Errorf("import failed: %w", err)
	}

	// Import tasks
	result, err := taskstore.ImportFromYAML(store, i.yamlPath)
	if err != nil {
		return nil, fmt.Errorf("import failed: %w", err)
	}

	_, _ = fmt.Fprintf(i.output, "✓ Imported %d task(s)\n", result.Imported)

	if len(result.Errors) > 0 {
		_, _ = fmt.Fprintf(i.output, "\n%d error(s) occurred during import:\n", len(result.Errors))
		for _, impErr := range result.Errors {
			_, _ = fmt.Fprintf(i.output, "  - Task %q: %s\n", impErr.ID, impErr.Reason)
		}
	}

	// Validate the complete task set
	allTasks, err := store.List()
	if err != nil {
		return nil, fmt.Errorf("import failed: %w", err)
	}

	lintResult := taskstore.LintTaskSet(allTasks)
	if !lintResult.Valid {
		if err := lintResult.Error(); err != nil {
			return nil, fmt.Errorf("import failed: task validation failed:\n%w", err)
		}
	}

	_, _ = fmt.Fprintln(i.output)

	return &cmdinternal.ImportResultInfo{
		TaskCount: result.Imported,
	}, nil
}

// realInitializer wraps the actual init command for the PRD pipeline.
type realInitializer struct {
	workDir  string
	cfg      *config.Config
	output   io.Writer
	parentID string
}

func (i *realInitializer) Init() error {
	_, _ = fmt.Fprintf(i.output, "Initializing ralph...\n")

	// Open task store
	tasksPath := filepath.Join(i.workDir, i.cfg.Tasks.Path)
	store, err := taskstore.NewLocalStore(tasksPath)
	if err != nil {
		return fmt.Errorf("init failed: %w", err)
	}

	// Get root tasks to find the first one (or use explicit parent)
	var parentTaskID string
	if i.parentID != "" {
		parentTaskID = i.parentID
	} else {
		rootTasks, err := store.ListByParent("")
		if err != nil {
			return fmt.Errorf("init failed: %w", err)
		}
		if len(rootTasks) == 0 {
			return fmt.Errorf("init failed: no root tasks found")
		}
		// Use the first root task
		parentTaskID = rootTasks[0].ID
	}

	// Validate parent task exists
	parentTask, err := store.Get(parentTaskID)
	if err != nil {
		return fmt.Errorf("init failed: parent task %q not found", parentTaskID)
	}

	// Ensure state directory exists
	if err := state.EnsureRalphDir(i.workDir); err != nil {
		return fmt.Errorf("init failed: %w", err)
	}

	// Write parent-task-id file
	parentIDFile := filepath.Join(i.workDir, i.cfg.Tasks.ParentIDFile)
	if err := os.WriteFile(parentIDFile, []byte(parentTaskID), 0644); err != nil {
		return fmt.Errorf("init failed: %w", err)
	}

	// Store in state directory
	if err := state.SetStoredParentTaskID(i.workDir, parentTaskID); err != nil {
		return fmt.Errorf("init failed: %w", err)
	}

	_, _ = fmt.Fprintf(i.output, "✓ Initialized with parent task: %s (%s)\n\n", parentTask.Title, parentTaskID)

	return nil
}

// realRunner wraps the actual run command for the PRD pipeline.
type realRunner struct {
	cmd           *cobra.Command
	once          bool
	maxIterations int
	branch        string
}

func (r *realRunner) Run(ctx context.Context) error {
	return runRun(r.cmd, r.once, r.maxIterations, r.branch)
}

// runBootstrapFromPRD runs the full PRD bootstrap pipeline.
func runBootstrapFromPRD(cmd *cobra.Command, prdPath string, opts *cmdinternal.BootstrapOptions) error {
	// Get working directory
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Load configuration
	cfg, err := config.LoadConfigWithFile(workDir, GetConfigFile())
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Ensure .ralph directory exists
	if err := state.EnsureRalphDir(workDir); err != nil {
		return fmt.Errorf("failed to create .ralph directory: %w", err)
	}

	output := cmd.OutOrStdout()
	yamlPath := filepath.Join(workDir, "tasks.yaml")

	// Get options
	var once bool
	var maxIterations int
	var branch string
	var parentID string
	if opts != nil {
		once = opts.Once
		maxIterations = opts.MaxIterations
		branch = opts.Branch
		parentID = opts.Parent
	}

	// Create pipeline components
	decomposerAdapter := &realDecomposer{
		prdPath: prdPath,
		workDir: workDir,
		cfg:     cfg,
		output:  output,
	}

	importerAdapter := &realImporter{
		workDir:  workDir,
		yamlPath: yamlPath,
		cfg:      cfg,
		output:   output,
	}

	initializerAdapter := &realInitializer{
		workDir:  workDir,
		cfg:      cfg,
		output:   output,
		parentID: parentID,
	}

	runnerAdapter := &realRunner{
		cmd:           cmd,
		once:          once,
		maxIterations: maxIterations,
		branch:        branch,
	}

	// Create and execute pipeline
	pipeline := cmdinternal.NewPRDPipeline(
		decomposerAdapter,
		importerAdapter,
		initializerAdapter,
		runnerAdapter,
		output,
	)

	return pipeline.Execute(cmd.Context(), prdPath)
}

// countTasksInYAML counts the number of tasks in YAML content by counting "- id:" patterns.
func countTasksInYAML(content string) int {
	count := 0
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- id:") || strings.HasPrefix(trimmed, "id:") {
			count++
		}
	}
	return count
}
