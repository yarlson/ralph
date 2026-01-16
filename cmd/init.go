package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/yarlson/go-ralph/internal/config"
	"github.com/yarlson/go-ralph/internal/selector"
	"github.com/yarlson/go-ralph/internal/state"
	"github.com/yarlson/go-ralph/internal/taskstore"
)

func newInitCmd() *cobra.Command {
	var parentID string
	var searchTerm string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize ralph for a feature",
		Long:  "Initialize ralph by setting the parent task ID and validating the task graph.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd, parentID, searchTerm)
		},
	}

	cmd.Flags().StringVar(&parentID, "parent", "", "parent task ID to initialize with")
	cmd.Flags().StringVar(&searchTerm, "search", "", "search term to find parent task by title")

	return cmd
}

func runInit(cmd *cobra.Command, parentID, searchTerm string) error {
	// Validate flags
	if parentID == "" && searchTerm == "" {
		return errors.New("either --parent or --search must be specified")
	}
	if parentID != "" && searchTerm != "" {
		return errors.New("cannot specify both --parent and --search")
	}

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

	// Ensure .ralph directory structure exists
	if err := state.EnsureRalphDir(workDir); err != nil {
		return fmt.Errorf("failed to create .ralph directory: %w", err)
	}

	// Open task store
	tasksPath := filepath.Join(workDir, cfg.Tasks.Path)
	store, err := taskstore.NewLocalStore(tasksPath)
	if err != nil {
		return fmt.Errorf("failed to open task store: %w", err)
	}

	// Resolve parent task ID
	resolvedID := parentID
	if searchTerm != "" {
		foundID, err := searchTaskByTitle(store, searchTerm)
		if err != nil {
			return err
		}
		resolvedID = foundID
	}

	// Validate parent task exists
	parentTask, err := store.Get(resolvedID)
	if err != nil {
		var notFoundErr *taskstore.NotFoundError
		if errors.As(err, &notFoundErr) {
			return fmt.Errorf("parent task %q not found", resolvedID)
		}
		return fmt.Errorf("failed to get parent task: %w", err)
	}

	// Load all tasks and validate graph
	allTasks, err := store.List()
	if err != nil {
		return fmt.Errorf("failed to list tasks: %w", err)
	}

	// Lint all tasks
	lintResult := taskstore.LintTaskSet(allTasks)
	if !lintResult.Valid {
		if err := lintResult.Error(); err != nil {
			return fmt.Errorf("task validation failed:\n%w", err)
		}
	}

	// Build dependency graph
	graph, err := selector.BuildGraph(allTasks)
	if err != nil {
		return fmt.Errorf("failed to build dependency graph: %w", err)
	}

	// Check for cycles
	if cycle := graph.DetectCycle(); cycle != nil {
		return fmt.Errorf("task graph contains a cycle: %v", cycle)
	}

	// Check for ready leaves
	readyLeaves := selector.GetReadyLeaves(allTasks, graph)
	// Filter to only descendants of parent
	var descendantLeaves []*taskstore.Task
	descendants := getDescendantIDs(allTasks, resolvedID)
	for _, leaf := range readyLeaves {
		if descendants[leaf.ID] {
			descendantLeaves = append(descendantLeaves, leaf)
		}
	}

	if len(descendantLeaves) == 0 {
		return fmt.Errorf("no ready leaf tasks found under parent %q", resolvedID)
	}

	// Write parent-task-id file
	parentIDFile := filepath.Join(workDir, cfg.Tasks.ParentIDFile)
	if err := os.MkdirAll(filepath.Dir(parentIDFile), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory for parent-task-id: %w", err)
	}
	if err := os.WriteFile(parentIDFile, []byte(resolvedID), 0644); err != nil {
		return fmt.Errorf("failed to write parent-task-id file: %w", err)
	}

	// Print success message
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Initialized ralph for parent task: %s (%s)\n", parentTask.Title, resolvedID)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Found %d ready leaf task(s)\n", len(descendantLeaves))

	return nil
}

// searchTaskByTitle searches for a task by title substring (case-insensitive)
func searchTaskByTitle(store *taskstore.LocalStore, searchTerm string) (string, error) {
	tasks, err := store.List()
	if err != nil {
		return "", fmt.Errorf("failed to list tasks: %w", err)
	}

	searchLower := strings.ToLower(searchTerm)
	var matches []*taskstore.Task

	for _, task := range tasks {
		if strings.Contains(strings.ToLower(task.Title), searchLower) {
			matches = append(matches, task)
		}
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no task found matching %q", searchTerm)
	}

	if len(matches) > 1 {
		var ids []string
		for _, t := range matches {
			ids = append(ids, fmt.Sprintf("%s (%s)", t.ID, t.Title))
		}
		return "", fmt.Errorf("multiple tasks match %q: %s", searchTerm, strings.Join(ids, ", "))
	}

	return matches[0].ID, nil
}

// getDescendantIDs returns a set of all descendant task IDs under the given parent
func getDescendantIDs(tasks []*taskstore.Task, parentID string) map[string]bool {
	// Build parent-to-children map
	children := make(map[string][]string)
	for _, t := range tasks {
		if t.ParentID != nil {
			children[*t.ParentID] = append(children[*t.ParentID], t.ID)
		}
	}

	// BFS to find all descendants
	descendants := make(map[string]bool)
	queue := children[parentID]

	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		descendants[id] = true
		queue = append(queue, children[id]...)
	}

	return descendants
}
