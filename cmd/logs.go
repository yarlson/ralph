package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/yarlson/ralph/internal/loop"
	"github.com/yarlson/ralph/internal/state"
)

func newLogsCmd() *cobra.Command {
	var iterationID string

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Show iteration logs",
		Long:  "Display iteration logs. Use --iteration to show a specific iteration, or list all available iterations.",
		RunE: func(cmd *cobra.Command, args []string) error {
			warnDeprecated(cmd.ErrOrStderr(), "logs")
			return runLogs(cmd, iterationID)
		},
	}

	cmd.Flags().StringVar(&iterationID, "iteration", "", "Show specific iteration log by ID")

	return cmd
}

func runLogs(cmd *cobra.Command, iterationID string) error {
	// Get working directory
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	logsDir := state.LogsDirPath(workDir)

	// Check if logs directory exists
	if _, err := os.Stat(logsDir); os.IsNotExist(err) {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "No logs found. Run 'ralph run' to create iterations.\n")
		return nil
	}

	// If iteration ID provided, show that specific iteration
	if iterationID != "" {
		return showIteration(cmd, logsDir, iterationID)
	}

	// Otherwise, list all available iterations
	return listIterations(cmd, logsDir)
}

func showIteration(cmd *cobra.Command, logsDir, iterationID string) error {
	// Find the iteration file
	filename := fmt.Sprintf("iteration-%s.json", iterationID)
	path := filepath.Join(logsDir, filename)

	// Check if file exists first
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("iteration %q not found", iterationID)
	}

	// Load the iteration record
	record, err := loop.LoadRecord(path)
	if err != nil {
		return fmt.Errorf("failed to load iteration: %w", err)
	}

	// Format and display
	output := formatIterationRecord(record)
	_, _ = fmt.Fprint(cmd.OutOrStdout(), output)

	return nil
}

func listIterations(cmd *cobra.Command, logsDir string) error {
	// Read all iteration files
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		return fmt.Errorf("failed to read logs directory: %w", err)
	}

	// Filter for iteration files
	var records []*loop.IterationRecord
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasPrefix(entry.Name(), "iteration-") || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(logsDir, entry.Name())
		record, err := loop.LoadRecord(path)
		if err != nil {
			// Skip files that can't be loaded
			continue
		}
		records = append(records, record)
	}

	if len(records) == 0 {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "No iterations found.\n")
		return nil
	}

	// Sort by start time (newest first)
	sort.Slice(records, func(i, j int) bool {
		return records[i].StartTime.After(records[j].StartTime)
	})

	// Display list
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Available iterations:\n\n")
	for _, record := range records {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s - Task: %s (%s) - %s\n",
			record.IterationID,
			record.TaskID,
			record.Outcome,
			record.StartTime.Format("2006-01-02 15:04:05"))
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nUse --iteration <id> to view details.\n")

	return nil
}

func formatIterationRecord(record *loop.IterationRecord) string {
	var sb strings.Builder

	// Header
	fmt.Fprintf(&sb, "Iteration: %s\n", record.IterationID)
	fmt.Fprintf(&sb, "Task: %s\n", record.TaskID)
	fmt.Fprintf(&sb, "Outcome: %s\n", record.Outcome)
	fmt.Fprintf(&sb, "Duration: %s\n", record.Duration())
	fmt.Fprintf(&sb, "\n")

	// Timing
	fmt.Fprintf(&sb, "Start: %s\n", record.StartTime.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&sb, "End: %s\n", record.EndTime.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&sb, "\n")

	// Attempt number if present
	if record.AttemptNumber > 0 {
		fmt.Fprintf(&sb, "Attempt: %d\n\n", record.AttemptNumber)
	}

	// Claude invocation
	if record.ClaudeInvocation.Model != "" || record.ClaudeInvocation.SessionID != "" {
		fmt.Fprintf(&sb, "Claude:\n")
		if record.ClaudeInvocation.Model != "" {
			fmt.Fprintf(&sb, "  Model: %s\n", record.ClaudeInvocation.Model)
		}
		if record.ClaudeInvocation.SessionID != "" {
			fmt.Fprintf(&sb, "  Session: %s\n", record.ClaudeInvocation.SessionID)
		}
		if record.ClaudeInvocation.TotalCostUSD > 0 {
			fmt.Fprintf(&sb, "  Cost: $%.4f\n", record.ClaudeInvocation.TotalCostUSD)
		}
		if record.ClaudeInvocation.InputTokens > 0 || record.ClaudeInvocation.OutputTokens > 0 {
			fmt.Fprintf(&sb, "  Tokens: %d in / %d out\n",
				record.ClaudeInvocation.InputTokens,
				record.ClaudeInvocation.OutputTokens)
		}
		fmt.Fprintf(&sb, "\n")
	}

	// Git commits
	if record.BaseCommit != "" {
		fmt.Fprintf(&sb, "Base commit: %s\n", record.BaseCommit)
	}
	if record.ResultCommit != "" {
		fmt.Fprintf(&sb, "Result commit: %s\n", record.ResultCommit)
	}
	if record.BaseCommit != "" || record.ResultCommit != "" {
		fmt.Fprintf(&sb, "\n")
	}

	// Files changed
	if len(record.FilesChanged) > 0 {
		fmt.Fprintf(&sb, "Files changed:\n")
		for _, file := range record.FilesChanged {
			fmt.Fprintf(&sb, "  - %s\n", file)
		}
		fmt.Fprintf(&sb, "\n")
	}

	// Verification results
	if len(record.VerificationOutputs) > 0 {
		fmt.Fprintf(&sb, "Verification:\n")
		for _, v := range record.VerificationOutputs {
			status := "PASS"
			if !v.Passed {
				status = "FAIL"
			}
			fmt.Fprintf(&sb, "  [%s] %s (%.2fs)\n", status, formatCommand(v.Command), v.Duration.Seconds())
			if !v.Passed && v.Output != "" {
				// Show first few lines of failed output
				lines := strings.Split(v.Output, "\n")
				maxLines := 10
				if len(lines) > maxLines {
					lines = lines[len(lines)-maxLines:]
					fmt.Fprintf(&sb, "    ... (output truncated)\n")
				}
				for _, line := range lines {
					if line != "" {
						fmt.Fprintf(&sb, "    %s\n", line)
					}
				}
			}
		}
		fmt.Fprintf(&sb, "\n")
	}

	// Feedback
	if record.Feedback != "" {
		fmt.Fprintf(&sb, "Feedback:\n%s\n\n", record.Feedback)
	}

	// Raw JSON option
	fmt.Fprintf(&sb, "---\n")
	fmt.Fprintf(&sb, "To view raw JSON, use: cat %s\n", fmt.Sprintf(".ralph/logs/iteration-%s.json", record.IterationID))

	return sb.String()
}

func formatCommand(cmd []string) string {
	if len(cmd) == 0 {
		return ""
	}
	return strings.Join(cmd, " ")
}
