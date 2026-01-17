package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/yarlson/ralph/internal/claude"
	"github.com/yarlson/ralph/internal/config"
	"github.com/yarlson/ralph/internal/decomposer"
	"github.com/yarlson/ralph/internal/state"
	"github.com/yarlson/ralph/internal/taskstore"
)

func newDecomposeCmd() *cobra.Command {
	var outputPath string
	var autoImport bool
	var timeout int

	cmd := &cobra.Command{
		Use:   "decompose <prd-file>",
		Short: "Decompose a PRD/SPEC into Ralph tasks",
		Long: `Decompose a PRD or specification document into a hierarchical task graph.

This command uses Claude Code to convert a PRD (Product Requirements Document) or
specification file into a task.yaml file that can be imported into Ralph's task store.

The decomposer will:
  - Read your PRD file (Markdown format)
  - Use Claude to analyze and break it down into tasks
  - Generate a task.yaml with proper hierarchy and dependencies
  - Optionally auto-import the tasks into Ralph's task store

Example:
  ralph decompose docs/prd.md
  ralph decompose docs/prd.md --output my-tasks.yaml
  ralph decompose docs/prd.md --import
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			warnDeprecated(cmd.ErrOrStderr(), "decompose")

			if len(args) != 1 {
				return fmt.Errorf("requires a PRD file path as argument")
			}

			prdPath := args[0]

			// Check if PRD file exists
			if _, err := os.Stat(prdPath); err != nil {
				return fmt.Errorf("PRD file not found: %s", prdPath)
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

			// Ensure .ralph directory exists
			if err := state.EnsureRalphDir(workDir); err != nil {
				return fmt.Errorf("failed to create .ralph directory: %w", err)
			}

			// Create Claude logs directory
			claudeLogsDir := filepath.Join(workDir, ".ralph", "logs", "claude")
			if err := os.MkdirAll(claudeLogsDir, 0755); err != nil {
				return fmt.Errorf("failed to create Claude logs directory: %w", err)
			}

			// Create Claude runner
			claudeCommand := "claude"
			var claudeArgs []string
			if len(cfg.Claude.Command) > 0 {
				claudeCommand = cfg.Claude.Command[0]
				// If command has multiple parts (e.g., ["claude", "code"]),
				// use the first as command and rest as base args
				if len(cfg.Claude.Command) > 1 {
					claudeArgs = append(claudeArgs, cfg.Claude.Command[1:]...)
				}
			}
			// Append configured args
			claudeArgs = append(claudeArgs, cfg.Claude.Args...)

			runner := claude.NewSubprocessRunner(claudeCommand, claudeLogsDir)
			if len(claudeArgs) > 0 {
				runner = runner.WithBaseArgs(claudeArgs)
			}

			// Create decomposer
			dec := decomposer.NewDecomposer(runner)

			// Set up context with timeout
			ctx := context.Background()
			if timeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
				defer cancel()
			}

			// Run decomposition
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Decomposing PRD: %s\n", prdPath)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Using Claude to analyze and generate tasks...\n\n")

			req := decomposer.DecomposeRequest{
				PRDPath: prdPath,
				WorkDir: workDir,
			}

			result, err := dec.Decompose(ctx, req)
			if err != nil {
				return fmt.Errorf("decomposition failed: %w", err)
			}

			// Determine output path
			if outputPath == "" {
				outputPath = "tasks.yaml"
			}

			// Write YAML to file
			if err := os.WriteFile(outputPath, []byte(result.YAMLContent), 0644); err != nil {
				return fmt.Errorf("failed to write tasks file: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✓ Generated tasks: %s\n", outputPath)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Session: %s\n", result.SessionID)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Model: %s\n", result.Model)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Cost: $%.4f\n", result.TotalCostUSD)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Log: %s\n\n", result.RawEventsPath)

			// Auto-import if requested
			if autoImport {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Importing tasks into store...\n")

				// Create task store
				store, err := taskstore.NewLocalStore(cfg.Tasks.Path)
				if err != nil {
					return fmt.Errorf("failed to create task store: %w", err)
				}

				// Import tasks
				importResult, err := taskstore.ImportFromYAML(store, outputPath)
				if err != nil {
					return fmt.Errorf("import failed: %w", err)
				}

				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✓ Imported %d task(s)\n", importResult.Imported)

				if len(importResult.Errors) > 0 {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n%d error(s) occurred during import:\n", len(importResult.Errors))
					for _, impErr := range importResult.Errors {
						_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  - Task %q: %s\n", impErr.ID, impErr.Reason)
					}
				}

				// Validate the complete task set
				allTasks, err := store.List()
				if err != nil {
					return fmt.Errorf("failed to list tasks for validation: %w", err)
				}

				lintResult := taskstore.LintTaskSet(allTasks)
				if !lintResult.Valid {
					if err := lintResult.Error(); err != nil {
						return fmt.Errorf("task validation failed after import:\n%w", err)
					}
				}

				// Display warnings
				if len(lintResult.Warnings) > 0 {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n%d warning(s):\n", len(lintResult.Warnings))
					for _, warning := range lintResult.Warnings {
						_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  - Task %q: %s\n", warning.TaskID, warning.Warning)
					}
				}

				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nYou can now initialize ralph with one of the root tasks:\n")
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  ralph init --parent <task-id>\n")
			} else {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "To import these tasks, run:\n")
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  ralph import %s\n", outputPath)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "output path for generated tasks.yaml (default: tasks.yaml)")
	cmd.Flags().BoolVar(&autoImport, "import", false, "automatically import tasks into Ralph's task store after generation")
	cmd.Flags().IntVar(&timeout, "timeout", 300, "timeout in seconds for Claude execution (default: 300)")

	return cmd
}
