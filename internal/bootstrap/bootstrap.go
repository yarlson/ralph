// Package bootstrap provides initialization pipelines for Ralph.
package bootstrap

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yarlson/ralph/internal/claude"
	"github.com/yarlson/ralph/internal/config"
	"github.com/yarlson/ralph/internal/decomposer"
	"github.com/yarlson/ralph/internal/memory"
	"github.com/yarlson/ralph/internal/runner"
	"github.com/yarlson/ralph/internal/state"
	"github.com/yarlson/ralph/internal/taskstore"
)

// Options configures a bootstrap run.
type Options struct {
	Once          bool
	MaxIterations int
	Parent        string
	Branch        string
	Stream        bool // Stream Claude output to console
}

// RunFromPRD runs the full pipeline: decompose → import → init → run.
func RunFromPRD(ctx context.Context, prdPath, workDir string, cfg *config.Config, opts Options, stdout, stderr io.Writer) error {
	_, _ = fmt.Fprintf(stdout, "Analyzing PRD: %s\n", prdPath)

	// Step 1: Decompose PRD to YAML
	yamlPath, err := decomposePRD(ctx, prdPath, workDir, cfg, stdout)
	if err != nil {
		return err
	}

	// Step 2: Import tasks
	if err := importTasks(yamlPath, cfg, stdout); err != nil {
		return err
	}

	// Step 3: Initialize
	parentTaskID, err := initRalph(workDir, cfg, opts.Parent, stdout)
	if err != nil {
		return err
	}

	// Step 4: Run
	runOpts := runner.Options{
		Once:          opts.Once,
		MaxIterations: opts.MaxIterations,
		Branch:        opts.Branch,
		Stream:        opts.Stream,
	}
	return runner.Run(ctx, workDir, cfg, parentTaskID, runOpts, stdout, stderr)
}

// RunFromYAML runs the pipeline: import → init → run.
func RunFromYAML(ctx context.Context, yamlPath, workDir string, cfg *config.Config, opts Options, stdout, stderr io.Writer) error {
	_, _ = fmt.Fprintf(stdout, "Initializing from YAML: %s\n", yamlPath)

	// Step 1: Import tasks
	if err := importTasks(yamlPath, cfg, stdout); err != nil {
		return err
	}

	// Step 2: Initialize
	parentTaskID, err := initRalph(workDir, cfg, opts.Parent, stdout)
	if err != nil {
		return err
	}

	// Step 3: Run
	runOpts := runner.Options{
		Once:          opts.Once,
		MaxIterations: opts.MaxIterations,
		Branch:        opts.Branch,
		Stream:        opts.Stream,
	}
	return runner.Run(ctx, workDir, cfg, parentTaskID, runOpts, stdout, stderr)
}

func decomposePRD(ctx context.Context, prdPath, workDir string, cfg *config.Config, output io.Writer) (string, error) {
	claudeLogsDir := filepath.Join(workDir, ".ralph", "logs", "claude")
	if err := os.MkdirAll(claudeLogsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create Claude logs directory: %w", err)
	}

	claudeCommand := "claude"
	var claudeArgs []string
	if len(cfg.Claude.Command) > 0 {
		claudeCommand = cfg.Claude.Command[0]
		if len(cfg.Claude.Command) > 1 {
			claudeArgs = append(claudeArgs, cfg.Claude.Command[1:]...)
		}
	}
	claudeArgs = append(claudeArgs, cfg.Claude.Args...)

	claudeRunner := claude.NewSubprocessRunner(claudeCommand, claudeLogsDir)
	if len(claudeArgs) > 0 {
		claudeRunner = claudeRunner.WithBaseArgs(claudeArgs)
	}

	dec := decomposer.NewDecomposer(claudeRunner)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	_, _ = fmt.Fprintf(output, "Using Claude to analyze and generate tasks...\n")

	req := decomposer.DecomposeRequest{
		PRDPath: prdPath,
		WorkDir: workDir,
	}

	result, err := dec.Decompose(ctx, req)
	if err != nil {
		return "", fmt.Errorf("decomposition failed: %w", err)
	}

	outputPath := filepath.Join(workDir, "tasks.yaml")
	if err := os.WriteFile(outputPath, []byte(result.YAMLContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write tasks file: %w", err)
	}

	taskCount := countTasksInYAML(result.YAMLContent)

	_, _ = fmt.Fprintf(output, "✓ Generated %d tasks: %s\n", taskCount, outputPath)
	_, _ = fmt.Fprintf(output, "  Session: %s\n", result.SessionID)
	_, _ = fmt.Fprintf(output, "  Model: %s\n", result.Model)
	_, _ = fmt.Fprintf(output, "  Cost: $%.4f\n\n", result.TotalCostUSD)

	return outputPath, nil
}

func importTasks(yamlPath string, cfg *config.Config, output io.Writer) error {
	_, _ = fmt.Fprintf(output, "Importing tasks into store...\n")

	store, err := taskstore.NewLocalStore(cfg.Tasks.Path)
	if err != nil {
		return fmt.Errorf("import failed: %w", err)
	}

	result, err := taskstore.ImportFromYAML(store, yamlPath)
	if err != nil {
		return fmt.Errorf("import failed: %w", err)
	}

	_, _ = fmt.Fprintf(output, "✓ Imported %d task(s)\n", result.Imported)

	if len(result.Errors) > 0 {
		_, _ = fmt.Fprintf(output, "\n%d error(s) occurred during import:\n", len(result.Errors))
		for _, impErr := range result.Errors {
			_, _ = fmt.Fprintf(output, "  - Task %q: %s\n", impErr.ID, impErr.Reason)
		}
	}

	allTasks, err := store.List()
	if err != nil {
		return fmt.Errorf("import failed: %w", err)
	}

	lintResult := taskstore.LintTaskSet(allTasks)
	if !lintResult.Valid {
		if err := lintResult.Error(); err != nil {
			return fmt.Errorf("import failed: task validation failed:\n%w", err)
		}
	}

	_, _ = fmt.Fprintln(output)
	return nil
}

func initRalph(workDir string, cfg *config.Config, parentID string, output io.Writer) (string, error) {
	_, _ = fmt.Fprintf(output, "Initializing ralph...\n")

	tasksPath := filepath.Join(workDir, cfg.Tasks.Path)
	store, err := taskstore.NewLocalStore(tasksPath)
	if err != nil {
		return "", fmt.Errorf("init failed: %w", err)
	}

	var parentTaskID string
	if parentID != "" {
		parentTaskID = parentID
	} else {
		rootTasks, err := store.ListByParent("")
		if err != nil {
			return "", fmt.Errorf("init failed: %w", err)
		}
		if len(rootTasks) == 0 {
			return "", fmt.Errorf("init failed: no root tasks found")
		}
		parentTaskID = rootTasks[0].ID
	}

	parentTask, err := store.Get(parentTaskID)
	if err != nil {
		return "", fmt.Errorf("init failed: parent task %q not found", parentTaskID)
	}

	if err := state.EnsureRalphDir(workDir); err != nil {
		return "", fmt.Errorf("init failed: %w", err)
	}

	parentIDFile := filepath.Join(workDir, cfg.Tasks.ParentIDFile)
	if err := os.WriteFile(parentIDFile, []byte(parentTaskID), 0644); err != nil {
		return "", fmt.Errorf("init failed: %w", err)
	}

	if err := state.SetStoredParentTaskID(workDir, parentTaskID); err != nil {
		return "", fmt.Errorf("init failed: %w", err)
	}

	progressPath := filepath.Join(workDir, cfg.Memory.ProgressFile)
	progressFile := memory.NewProgressFile(progressPath)
	if !progressFile.Exists() {
		if err := progressFile.Init(parentTask.Title, parentTaskID); err != nil {
			return "", fmt.Errorf("init failed: %w", err)
		}
	}

	_, _ = fmt.Fprintf(output, "✓ Initialized with parent task: %s (%s)\n\n", parentTask.Title, parentTaskID)

	return parentTaskID, nil
}

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
