package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	cmdinternal "github.com/yarlson/ralph/cmd/internal"
	"github.com/yarlson/ralph/internal/config"
	"github.com/yarlson/ralph/internal/memory"
	"github.com/yarlson/ralph/internal/state"
	"github.com/yarlson/ralph/internal/taskstore"
)

// yamlImporter wraps the actual task import for the YAML pipeline.
type yamlImporter struct {
	workDir  string
	yamlPath string
	cfg      *config.Config
	output   io.Writer
}

func (i *yamlImporter) Import() (*cmdinternal.ImportResultInfo, error) {
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

// yamlInitializer wraps the actual init command for the YAML pipeline.
type yamlInitializer struct {
	workDir  string
	cfg      *config.Config
	output   io.Writer
	parentID string
}

func (i *yamlInitializer) Init() error {
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

	// Initialize progress file
	progressPath := filepath.Join(i.workDir, i.cfg.Memory.ProgressFile)
	progressFile := memory.NewProgressFile(progressPath)
	if !progressFile.Exists() {
		if err := progressFile.Init(parentTask.Title, parentTaskID); err != nil {
			return fmt.Errorf("init failed: %w", err)
		}
	}

	_, _ = fmt.Fprintf(i.output, "✓ Initialized with parent task: %s (%s)\n\n", parentTask.Title, parentTaskID)

	return nil
}

// yamlRunner wraps the actual run command for the YAML pipeline.
type yamlRunner struct {
	cmd           *cobra.Command
	once          bool
	maxIterations int
	branch        string
}

func (r *yamlRunner) Run(ctx context.Context) error {
	return runRun(r.cmd, r.once, r.maxIterations, r.branch)
}

// runBootstrapFromYAML runs the YAML bootstrap pipeline (import→init→run).
func runBootstrapFromYAML(cmd *cobra.Command, yamlPath string, opts *cmdinternal.BootstrapOptions) error {
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
	importerAdapter := &yamlImporter{
		workDir:  workDir,
		yamlPath: yamlPath,
		cfg:      cfg,
		output:   output,
	}

	initializerAdapter := &yamlInitializer{
		workDir:  workDir,
		cfg:      cfg,
		output:   output,
		parentID: parentID,
	}

	runnerAdapter := &yamlRunner{
		cmd:           cmd,
		once:          once,
		maxIterations: maxIterations,
		branch:        branch,
	}

	// Create and execute pipeline
	pipeline := cmdinternal.NewYAMLPipeline(
		importerAdapter,
		initializerAdapter,
		runnerAdapter,
		output,
	)

	return pipeline.Execute(cmd.Context(), yamlPath)
}
