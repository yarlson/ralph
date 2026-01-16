package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/yarlson/go-ralph/internal/config"
	"github.com/yarlson/go-ralph/internal/reporter"
	"github.com/yarlson/go-ralph/internal/state"
	"github.com/yarlson/go-ralph/internal/taskstore"
)

func newReportCmd() *cobra.Command {
	var outputFile string

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate feature report",
		Long:  "Generate and display an end-of-feature summary report with commits, completed tasks, and blocked tasks.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReport(cmd, outputFile)
		},
	}

	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write report to file instead of stdout")

	return cmd
}

func runReport(cmd *cobra.Command, outputFile string) error {
	// Get working directory
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Load configuration
	cfg, err := config.LoadConfig(workDir)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Read parent task ID
	parentIDFile := filepath.Join(workDir, cfg.Tasks.ParentIDFile)
	parentIDBytes, err := os.ReadFile(parentIDFile)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("parent-task-id file not found. Run 'ralph init' first")
		}
		return fmt.Errorf("failed to read parent-task-id: %w", err)
	}
	parentTaskID := string(parentIDBytes)

	// Open task store
	tasksPath := filepath.Join(workDir, cfg.Tasks.Path)
	store, err := taskstore.NewLocalStore(tasksPath)
	if err != nil {
		return fmt.Errorf("failed to open task store: %w", err)
	}

	// Validate parent task exists
	_, err = store.Get(parentTaskID)
	if err != nil {
		return fmt.Errorf("parent task %q not found: %w", parentTaskID, err)
	}

	// Get logs directory
	logsDir := state.LogsDirPath(workDir)

	// Create report generator
	generator := reporter.NewReportGenerator(store, logsDir)

	// Generate report
	report, err := generator.GenerateReport(parentTaskID)
	if err != nil {
		return fmt.Errorf("failed to generate report: %w", err)
	}

	// Format report
	formatted := reporter.FormatReport(report)

	// Output to file or stdout
	if outputFile != "" {
		// Create parent directories if needed
		dir := filepath.Dir(outputFile)
		if dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create output directory: %w", err)
			}
		}

		// Write to file
		if err := os.WriteFile(outputFile, []byte(formatted), 0644); err != nil {
			return fmt.Errorf("failed to write report to file: %w", err)
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Report written to: %s\n", outputFile)
	} else {
		_, _ = fmt.Fprint(cmd.OutOrStdout(), formatted)
	}

	return nil
}
