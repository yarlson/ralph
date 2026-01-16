package taskstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// LocalStore implements the Store interface using local JSON files.
// Each task is stored as a separate .json file in the configured directory.
type LocalStore struct {
	dir string
	mu  sync.RWMutex
}

// NewLocalStore creates a new LocalStore that persists tasks in the given directory.
// The directory is created if it does not exist.
func NewLocalStore(dir string) (*LocalStore, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create tasks directory: %w", err)
	}
	return &LocalStore{dir: dir}, nil
}

// taskPath returns the file path for a task with the given ID.
func (s *LocalStore) taskPath(id string) string {
	return filepath.Join(s.dir, id+".json")
}

// Get retrieves a task by its ID.
func (s *LocalStore) Get(id string) (*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.getUnlocked(id)
}

// getUnlocked retrieves a task without acquiring the lock.
// Caller must hold at least a read lock.
func (s *LocalStore) getUnlocked(id string) (*Task, error) {
	data, err := os.ReadFile(s.taskPath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &NotFoundError{ID: id}
		}
		return nil, fmt.Errorf("failed to read task file: %w", err)
	}

	var task Task
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, fmt.Errorf("failed to parse task file: %w", err)
	}

	return &task, nil
}

// List retrieves all tasks from the store.
func (s *LocalStore) List() ([]*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read tasks directory: %w", err)
	}

	var tasks []*Task
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		id := strings.TrimSuffix(entry.Name(), ".json")
		task, err := s.getUnlocked(id)
		if err != nil {
			// Skip files that can't be parsed
			continue
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

// ListByParent retrieves all tasks with the given parent ID.
// If parentID is empty, returns tasks with no parent (root tasks).
func (s *LocalStore) ListByParent(parentID string) ([]*Task, error) {
	allTasks, err := s.List()
	if err != nil {
		return nil, err
	}

	var tasks []*Task
	for _, task := range allTasks {
		if parentID == "" {
			// Looking for root tasks (no parent)
			if task.ParentID == nil {
				tasks = append(tasks, task)
			}
		} else {
			// Looking for tasks with specific parent
			if task.ParentID != nil && *task.ParentID == parentID {
				tasks = append(tasks, task)
			}
		}
	}

	return tasks, nil
}

// Save persists a task to the store.
// If a task with the same ID exists, it is updated.
func (s *LocalStore) Save(task *Task) error {
	// Update the UpdatedAt timestamp
	task.UpdatedAt = time.Now().Truncate(time.Second)

	// Validate before saving
	if err := task.Validate(); err != nil {
		return &ValidationError{ID: task.ID, Reason: err.Error()}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return s.writeTask(task)
}

// writeTask writes a task to disk using atomic write.
// Caller must hold the write lock.
func (s *LocalStore) writeTask(task *Task) error {
	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	taskFile := s.taskPath(task.ID)

	// Atomic write: write to temp file, then rename
	tmpFile := taskFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tmpFile, taskFile); err != nil {
		// Clean up temp file on rename failure
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// UpdateStatus updates only the status of an existing task.
func (s *LocalStore) UpdateStatus(id string, status TaskStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, err := s.getUnlocked(id)
	if err != nil {
		return err
	}

	task.Status = status
	task.UpdatedAt = time.Now().Truncate(time.Second)

	return s.writeTask(task)
}

// Delete removes a task by its ID.
func (s *LocalStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	taskFile := s.taskPath(id)
	if _, err := os.Stat(taskFile); err != nil {
		if os.IsNotExist(err) {
			return &NotFoundError{ID: id}
		}
		return fmt.Errorf("failed to stat task file: %w", err)
	}

	if err := os.Remove(taskFile); err != nil {
		return fmt.Errorf("failed to delete task file: %w", err)
	}

	return nil
}
