package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/yarlson/ralph/internal/config"
	"github.com/yarlson/ralph/internal/taskstore"
)

func newImportCmd() *cobra.Command {
	var overwrite bool

	cmd := &cobra.Command{
		Use:   "import <file>",
		Short: "Import tasks from a YAML file",
		Long: `Import tasks from a YAML file into the Ralph task store.

The YAML file should contain a 'tasks' array with task definitions.
Tasks are validated before import. Invalid tasks are skipped and reported.
Existing tasks with matching IDs are updated unless --overwrite is false.

Example YAML format:
  tasks:
    - id: task-1
      title: My Task
      description: Task description
      status: open
      acceptance:
        - Acceptance criterion 1
      verify:
        - ["go", "test", "./..."]
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("requires a YAML file path as argument")
			}

			yamlPath := args[0]

			// Check if file exists
			if _, err := os.Stat(yamlPath); err != nil {
				return fmt.Errorf("file not found: %s", yamlPath)
			}

			// Get working directory
			workDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}

			// Load config
			cfg, err := config.LoadConfigWithFile(workDir, GetConfigFile())
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Create task store
			store, err := taskstore.NewLocalStore(cfg.Tasks.Path)
			if err != nil {
				return fmt.Errorf("failed to create task store: %w", err)
			}

			// Import from YAML
			result, err := taskstore.ImportFromYAML(store, yamlPath)
			if err != nil {
				return fmt.Errorf("import failed: %w", err)
			}

			// Display import results
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Successfully imported %d task(s)\n", result.Imported)

			if len(result.Errors) > 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n%d error(s) occurred during import:\n", len(result.Errors))
				for _, impErr := range result.Errors {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  - Task %q: %s\n", impErr.ID, impErr.Reason)
				}
			}

			// Lint all tasks after import to validate the complete task set
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

			// Display warnings if any
			if len(lintResult.Warnings) > 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n%d warning(s):\n", len(lintResult.Warnings))
				for _, warning := range lintResult.Warnings {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  - Task %q: %s\n", warning.TaskID, warning.Warning)
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "Overwrite existing tasks (currently tasks are always updated if they exist)")

	return cmd
}
