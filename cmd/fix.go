package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/yarlson/ralph/cmd/tui"
	"github.com/yarlson/ralph/internal/config"
	"github.com/yarlson/ralph/internal/fix"
	"github.com/yarlson/ralph/internal/state"
	"github.com/yarlson/ralph/internal/taskstore"
)

func newFixCmd() *cobra.Command {
	var retryID, skipID, undoID, feedback, reason string
	var force, list bool

	cmd := &cobra.Command{
		Use:   "fix",
		Short: "Fix failed tasks or undo iterations",
		Long: `Fix command provides options to retry failed tasks, skip tasks, or undo iterations.

Examples:
  ralph fix --retry task-123        # Retry a failed task
  ralph fix --skip task-123         # Skip a task
  ralph fix --undo iteration-001    # Undo an iteration
  ralph fix --list                  # List fixable issues`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFix(cmd, retryID, skipID, undoID, feedback, reason, force, list)
		},
	}

	cmd.Flags().StringVarP(&retryID, "retry", "r", "", "task ID to retry")
	cmd.Flags().StringVarP(&skipID, "skip", "s", "", "task ID to skip")
	cmd.Flags().StringVarP(&undoID, "undo", "u", "", "iteration ID to undo")
	cmd.Flags().StringVarP(&feedback, "feedback", "f", "", "feedback message for retry")
	cmd.Flags().StringVar(&reason, "reason", "", "reason for skipping")
	cmd.Flags().BoolVar(&force, "force", false, "skip confirmation prompts")
	cmd.Flags().BoolVarP(&list, "list", "l", false, "list fixable issues")

	return cmd
}

func runFix(cmd *cobra.Command, retryID, skipID, undoID, feedback, reason string, force, list bool) error {
	svc, err := newFixService()
	if err != nil {
		return err
	}

	if list {
		return runFixList(cmd, svc)
	}

	hasActionFlag := retryID != "" || skipID != "" || undoID != ""

	if !hasActionFlag {
		if !tui.IsInteractive(os.Stdin.Fd()) {
			return runFixNonTTYError(cmd, svc)
		}
		return runFixInteractive(cmd, svc, force)
	}

	if retryID != "" {
		return runFixRetry(cmd, svc, retryID, feedback)
	}

	if skipID != "" {
		return runFixSkip(cmd, svc, skipID, reason)
	}

	if undoID != "" {
		return runFixUndo(cmd, svc, undoID, force)
	}

	return nil
}

func newFixService() (*fix.Service, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	cfg, err := config.LoadConfigWithFile(workDir, GetConfigFile())
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	tasksPath := filepath.Join(workDir, cfg.Tasks.Path)
	store, err := taskstore.NewLocalStore(tasksPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open task store: %w", err)
	}

	logsDir := state.LogsDirPath(workDir)
	stateDir := state.StateDirPath(workDir)

	return fix.NewService(store, logsDir, stateDir, workDir), nil
}

func runFixList(cmd *cobra.Command, svc *fix.Service) error {
	failed, blocked, err := svc.ListIssues()
	if err != nil {
		return err
	}

	iterations, _ := svc.ListIterations(10)

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Failed Tasks:")
	if len(failed) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  (none)")
	} else {
		for _, t := range failed {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  - %s: %s\n", t.TaskID, t.Title)
		}
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout())

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Blocked Tasks:")
	if len(blocked) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  (none)")
	} else {
		for _, t := range blocked {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  - %s: %s\n", t.TaskID, t.Title)
		}
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout())

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Recent Iterations:")
	if len(iterations) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  (none)")
	} else {
		for _, i := range iterations {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  - %s: task=%s outcome=%s\n", i.IterationID, i.TaskID, i.Outcome)
		}
	}

	return nil
}

func runFixRetry(cmd *cobra.Command, svc *fix.Service, taskID, feedback string) error {
	alreadyOpen, _ := svc.IsAlreadyOpen(taskID)
	if alreadyOpen {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Task %q is already open\n", taskID)
		return nil
	}

	if err := svc.Retry(taskID, feedback); err != nil {
		return err
	}

	if feedback != "" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Feedback saved for task %q\n", taskID)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Retry initiated: task %q reset to open status\n", taskID)
	return nil
}

func runFixSkip(cmd *cobra.Command, svc *fix.Service, taskID, reason string) error {
	alreadySkipped, _ := svc.IsAlreadySkipped(taskID)
	if alreadySkipped {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Task %q is already skipped\n", taskID)
		return nil
	}

	if err := svc.Skip(taskID, reason); err != nil {
		return err
	}

	if reason != "" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Skip reason saved for task %q\n", taskID)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Task %q marked as skipped\n", taskID)
	return nil
}

func runFixUndo(cmd *cobra.Command, svc *fix.Service, iterationID string, force bool) error {
	info, err := svc.GetUndoInfo(cmd.Context(), iterationID)
	if err != nil {
		return err
	}

	if !force {
		confirmInfo := tui.UndoConfirmationInfo{
			IterationID:           info.IterationID,
			CommitToResetTo:       info.CommitToResetTo,
			TaskToReopen:          info.TaskToReopen,
			FilesToRevert:         info.FilesToRevert,
			HasUncommittedChanges: info.HasUncommittedChanges,
		}

		confirmed, err := tui.ConfirmUndo(cmd.OutOrStdout(), cmd.InOrStdin(), confirmInfo)
		if err != nil {
			return err
		}
		if !confirmed {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Undo cancelled.\n")
			return nil
		}
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Reverting to commit %s...\n", info.CommitToResetTo)
	if err := svc.Undo(iterationID); err != nil {
		return err
	}

	if info.TaskToReopen != "" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Task %q reset to open status\n", info.TaskToReopen)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Undo completed: reverted iteration %s\n", iterationID)
	return nil
}

func runFixNonTTYError(cmd *cobra.Command, svc *fix.Service) error {
	failed, _, _ := svc.ListIssues()
	iterations, _ := svc.ListIterations(10)

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Fixable Issues:")
	if len(failed) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  (none)")
	} else {
		for _, t := range failed {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  - %s: %s\n", t.TaskID, t.Title)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "    use: ralph fix --retry %s\n", t.TaskID)
		}
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout())

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Recent Iterations:")
	if len(iterations) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  (none)")
	} else {
		for _, i := range iterations {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  - %s: task=%s outcome=%s\n", i.IterationID, i.TaskID, i.Outcome)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "    use: ralph fix --undo %s\n", i.IterationID)
		}
	}

	return fmt.Errorf("interactive mode requires TTY: use explicit flags (--retry, --skip, --undo, --list)")
}

func runFixInteractive(cmd *cobra.Command, svc *fix.Service, force bool) error {
	failed, blocked, _ := svc.ListIssues()
	iterations, _ := svc.ListIterations(10)

	var issues []tui.FixIssue
	for _, t := range failed {
		issues = append(issues, tui.FixIssue{TaskID: t.TaskID, Title: t.Title, Status: t.Status, Attempts: t.Attempts})
	}
	for _, t := range blocked {
		issues = append(issues, tui.FixIssue{TaskID: t.TaskID, Title: t.Title, Status: t.Status, Attempts: t.Attempts})
	}

	var fixIterations []tui.FixIteration
	for _, i := range iterations {
		fixIterations = append(fixIterations, tui.FixIteration{IterationID: i.IterationID, TaskID: i.TaskID, Outcome: i.Outcome})
	}

	handler := func(action *tui.FixAction) error {
		switch action.Type {
		case tui.FixActionRetry:
			return runFixRetry(cmd, svc, action.TargetID, action.Feedback)
		case tui.FixActionSkip:
			return runFixSkip(cmd, svc, action.TargetID, "")
		case tui.FixActionUndo:
			return runFixUndo(cmd, svc, action.TargetID, force)
		default:
			return fmt.Errorf("unknown action type: %s", action.Type)
		}
	}

	editorFn := func(taskID string) (string, error) {
		return fix.OpenEditorForFeedback(taskID, os.Stdin, os.Stdout, os.Stderr)
	}

	return tui.FixInteractiveModeWithEditor(cmd.OutOrStdout(), cmd.InOrStdin(), issues, fixIterations, handler, editorFn)
}
