// Package fix provides task fixing operations (retry, skip, undo).
package fix

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/yarlson/ralph/internal/git"
	"github.com/yarlson/ralph/internal/loop"
	"github.com/yarlson/ralph/internal/state"
	"github.com/yarlson/ralph/internal/taskstore"
)

// Issue represents a fixable issue (failed or blocked task).
type Issue struct {
	TaskID   string
	Title    string
	Status   string
	Attempts int
}

// Iteration represents an iteration record for display.
type Iteration struct {
	IterationID string
	TaskID      string
	Outcome     string
}

// UndoInfo contains information for undo confirmation.
type UndoInfo struct {
	IterationID           string
	CommitToResetTo       string
	TaskToReopen          string
	FilesToRevert         []string
	HasUncommittedChanges bool
}

// Service provides fix operations.
type Service struct {
	store    *taskstore.LocalStore
	logsDir  string
	stateDir string
	workDir  string
}

// NewService creates a new fix service.
func NewService(store *taskstore.LocalStore, logsDir, stateDir, workDir string) *Service {
	return &Service{
		store:    store,
		logsDir:  logsDir,
		stateDir: stateDir,
		workDir:  workDir,
	}
}

// Retry resets a failed task to open status.
func (s *Service) Retry(taskID, feedback string) error {
	task, err := s.store.Get(taskID)
	if err != nil {
		var notFoundErr *taskstore.NotFoundError
		if errors.As(err, &notFoundErr) {
			return fmt.Errorf("task %q not found", taskID)
		}
		return fmt.Errorf("failed to get task: %w", err)
	}

	switch task.Status {
	case taskstore.StatusFailed:
		// OK to retry
	case taskstore.StatusOpen:
		return nil // Already open, no-op
	case taskstore.StatusCompleted:
		return fmt.Errorf("cannot retry task %q: task is completed", taskID)
	default:
		return fmt.Errorf("cannot retry task %q: task status is %q (must be failed or open)", taskID, task.Status)
	}

	if err := s.store.UpdateStatus(taskID, taskstore.StatusOpen); err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

	if feedback != "" {
		if err := state.EnsureRalphDir(s.workDir); err != nil {
			return fmt.Errorf("failed to ensure .ralph directory: %w", err)
		}
		feedbackFile := filepath.Join(s.stateDir, fmt.Sprintf("feedback-%s.txt", taskID))
		if err := os.WriteFile(feedbackFile, []byte(feedback), 0644); err != nil {
			return fmt.Errorf("failed to write feedback file: %w", err)
		}
	}

	return nil
}

// IsAlreadyOpen returns true if retry was a no-op because task is already open.
func (s *Service) IsAlreadyOpen(taskID string) (bool, error) {
	task, err := s.store.Get(taskID)
	if err != nil {
		return false, err
	}
	return task.Status == taskstore.StatusOpen, nil
}

// Skip marks a task as skipped.
func (s *Service) Skip(taskID, reason string) error {
	task, err := s.store.Get(taskID)
	if err != nil {
		var notFoundErr *taskstore.NotFoundError
		if errors.As(err, &notFoundErr) {
			return fmt.Errorf("task %q not found", taskID)
		}
		return fmt.Errorf("failed to get task: %w", err)
	}

	switch task.Status {
	case taskstore.StatusOpen, taskstore.StatusFailed, taskstore.StatusBlocked:
		// OK to skip
	case taskstore.StatusSkipped:
		return nil // Already skipped, no-op
	case taskstore.StatusCompleted:
		return errors.New("cannot skip completed task")
	default:
		return fmt.Errorf("cannot skip task %q: task status is %q (must be open, failed, or blocked)", taskID, task.Status)
	}

	if err := s.store.UpdateStatus(taskID, taskstore.StatusSkipped); err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

	if reason != "" {
		if err := state.EnsureRalphDir(s.workDir); err != nil {
			return fmt.Errorf("failed to ensure .ralph directory: %w", err)
		}
		reasonFile := filepath.Join(s.stateDir, fmt.Sprintf("skip-reason-%s.txt", taskID))
		if err := os.WriteFile(reasonFile, []byte(reason), 0644); err != nil {
			return fmt.Errorf("failed to write reason file: %w", err)
		}
	}

	return nil
}

// IsAlreadySkipped returns true if skip was a no-op because task is already skipped.
func (s *Service) IsAlreadySkipped(taskID string) (bool, error) {
	task, err := s.store.Get(taskID)
	if err != nil {
		return false, err
	}
	return task.Status == taskstore.StatusSkipped, nil
}

// ListIssues returns all fixable issues (failed and blocked tasks).
func (s *Service) ListIssues() (failed, blocked []Issue, err error) {
	tasks, err := s.store.List()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list tasks: %w", err)
	}

	iterations, _ := loop.LoadAllIterationRecords(s.logsDir)

	for _, task := range tasks {
		switch task.Status {
		case taskstore.StatusFailed:
			failed = append(failed, Issue{
				TaskID:   task.ID,
				Title:    task.Title,
				Status:   string(task.Status),
				Attempts: countTaskAttempts(iterations, task.ID),
			})
		case taskstore.StatusBlocked:
			blocked = append(blocked, Issue{
				TaskID:   task.ID,
				Title:    task.Title,
				Status:   string(task.Status),
				Attempts: countTaskAttempts(iterations, task.ID),
			})
		}
	}

	return failed, blocked, nil
}

// ListIterations returns recent iterations.
func (s *Service) ListIterations(limit int) ([]Iteration, error) {
	records, err := loop.LoadAllIterationRecords(s.logsDir)
	if err != nil {
		return nil, nil // Empty is OK
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].EndTime.After(records[j].EndTime)
	})

	if len(records) > limit {
		records = records[:limit]
	}

	var result []Iteration
	for _, r := range records {
		result = append(result, Iteration{
			IterationID: r.IterationID,
			TaskID:      r.TaskID,
			Outcome:     string(r.Outcome),
		})
	}

	return result, nil
}

// GetUndoInfo returns information needed for undo confirmation.
func (s *Service) GetUndoInfo(ctx context.Context, iterationID string) (*UndoInfo, error) {
	iterationFile := filepath.Join(s.logsDir, fmt.Sprintf("iteration-%s.json", iterationID))

	if _, err := os.Stat(iterationFile); os.IsNotExist(err) {
		return nil, errors.New("iteration not found")
	}

	record, err := loop.LoadRecord(iterationFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load iteration record: %w", err)
	}

	if record.BaseCommit == "" {
		return nil, fmt.Errorf("iteration %q has no base commit recorded", iterationID)
	}

	gitManager := git.NewShellManager(s.workDir, "")
	hasChanges, _ := gitManager.HasChanges(ctx)

	taskToReopen := ""
	if record.Outcome == loop.OutcomeSuccess && record.TaskID != "" {
		task, err := s.store.Get(record.TaskID)
		if err == nil && task.Status == taskstore.StatusCompleted {
			taskToReopen = record.TaskID
		}
	}

	return &UndoInfo{
		IterationID:           iterationID,
		CommitToResetTo:       record.BaseCommit,
		TaskToReopen:          taskToReopen,
		FilesToRevert:         record.FilesChanged,
		HasUncommittedChanges: hasChanges,
	}, nil
}

// Undo reverts an iteration.
func (s *Service) Undo(iterationID string) error {
	iterationFile := filepath.Join(s.logsDir, fmt.Sprintf("iteration-%s.json", iterationID))

	if _, err := os.Stat(iterationFile); os.IsNotExist(err) {
		return errors.New("iteration not found")
	}

	record, err := loop.LoadRecord(iterationFile)
	if err != nil {
		return fmt.Errorf("failed to load iteration record: %w", err)
	}

	if record.BaseCommit == "" {
		return fmt.Errorf("iteration %q has no base commit recorded", iterationID)
	}

	// Git reset
	cmd := exec.Command("git", "reset", "--hard", record.BaseCommit)
	cmd.Dir = s.workDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git reset failed: %w", err)
	}

	// Reopen task if it was completed
	if record.Outcome == loop.OutcomeSuccess && record.TaskID != "" {
		task, err := s.store.Get(record.TaskID)
		if err == nil && task.Status == taskstore.StatusCompleted {
			if err := s.store.UpdateStatus(record.TaskID, taskstore.StatusOpen); err != nil {
				return fmt.Errorf("failed to update task status: %w", err)
			}
		}
	}

	return nil
}

func countTaskAttempts(iterations []*loop.IterationRecord, taskID string) int {
	count := 0
	for _, iter := range iterations {
		if iter.TaskID == taskID {
			count++
		}
	}
	return count
}

// ParseEditorContent removes comment lines and trims whitespace.
func ParseEditorContent(content string) string {
	var result []byte
	inComment := false
	lineStart := true

	for _, ch := range content {
		if lineStart && ch == '#' {
			inComment = true
		}
		if ch == '\n' {
			if !inComment {
				result = append(result, '\n')
			}
			inComment = false
			lineStart = true
		} else {
			lineStart = false
			if !inComment {
				result = append(result, byte(ch))
			}
		}
	}

	return trimWhitespace(string(result))
}

func trimWhitespace(s string) string {
	start := 0
	end := len(s)

	for start < end && isWhitespace(s[start]) {
		start++
	}

	for end > start && isWhitespace(s[end-1]) {
		end--
	}

	return s[start:end]
}

func isWhitespace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// OpenEditorForFeedback opens the user's editor for feedback input.
func OpenEditorForFeedback(taskID string, stdin io.Reader, stdout, stderr io.Writer) (string, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		for _, e := range []string{"vim", "vi", "nano", "notepad"} {
			if _, err := exec.LookPath(e); err == nil {
				editor = e
				break
			}
		}
	}
	if editor == "" {
		return "", fmt.Errorf("no editor found. Set EDITOR or VISUAL environment variable")
	}

	tmpFile, err := os.CreateTemp("", fmt.Sprintf("ralph-feedback-%s-*.txt", taskID))
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	instructions := fmt.Sprintf(`# Enter feedback for task %s
# Lines starting with # will be ignored.
# Save and close the editor to continue.

`, taskID)
	if _, err := tmpFile.WriteString(instructions); err != nil {
		_ = tmpFile.Close()
		return "", fmt.Errorf("failed to write instructions: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close temporary file: %w", err)
	}

	editorCmd := exec.Command(editor, tmpPath)
	editorCmd.Stdin = stdin
	editorCmd.Stdout = stdout
	editorCmd.Stderr = stderr
	if err := editorCmd.Run(); err != nil {
		return "", fmt.Errorf("editor failed: %w", err)
	}

	content, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", fmt.Errorf("failed to read feedback: %w", err)
	}

	return ParseEditorContent(string(content)), nil
}
