package taskstore

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// YAMLTask represents a task as defined in a YAML file.
// Field names match the YAML structure (e.g., parentId instead of parent_id).
type YAMLTask struct {
	ID          string            `yaml:"id"`
	Title       string            `yaml:"title"`
	Description string            `yaml:"description,omitempty"`
	ParentID    string            `yaml:"parentId,omitempty"`
	DependsOn   []string          `yaml:"dependsOn,omitempty"`
	Status      string            `yaml:"status,omitempty"`
	Acceptance  []string          `yaml:"acceptance,omitempty"`
	Verify      [][]string        `yaml:"verify,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
}

// YAMLFile represents the structure of a tasks YAML file.
type YAMLFile struct {
	Tasks []YAMLTask `yaml:"tasks"`
}

// ImportError represents an error that occurred during import of a specific task.
type ImportError struct {
	ID     string
	Reason string
}

// ImportResult contains the results of a YAML import operation.
type ImportResult struct {
	Imported int
	Errors   []ImportError
}

// ImportFromYAML reads tasks from a YAML file and imports them into the store.
// Tasks that fail validation are skipped and reported in the result.
// Existing tasks with matching IDs are updated.
func ImportFromYAML(store Store, path string) (*ImportResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read YAML file: %w", err)
	}

	var yamlFile YAMLFile
	if err := yaml.Unmarshal(data, &yamlFile); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	result := &ImportResult{}

	for _, yt := range yamlFile.Tasks {
		task, err := convertYAMLTask(yt)
		if err != nil {
			result.Errors = append(result.Errors, ImportError{
				ID:     yt.ID,
				Reason: err.Error(),
			})
			continue
		}

		if err := store.Save(task); err != nil {
			result.Errors = append(result.Errors, ImportError{
				ID:     task.ID,
				Reason: err.Error(),
			})
			continue
		}

		result.Imported++
	}

	return result, nil
}

// convertYAMLTask converts a YAMLTask to a Task, applying defaults and validation.
func convertYAMLTask(yt YAMLTask) (*Task, error) {
	now := time.Now().Truncate(time.Second)

	task := &Task{
		ID:          yt.ID,
		Title:       yt.Title,
		Description: yt.Description,
		DependsOn:   yt.DependsOn,
		Acceptance:  yt.Acceptance,
		Verify:      yt.Verify,
		Labels:      yt.Labels,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Handle optional ParentID
	if yt.ParentID != "" {
		task.ParentID = &yt.ParentID
	}

	// Default status to "open" if not specified
	if yt.Status == "" {
		task.Status = StatusOpen
	} else {
		task.Status = TaskStatus(yt.Status)
	}

	// Validate the task
	if err := task.Validate(); err != nil {
		return nil, err
	}

	return task, nil
}
