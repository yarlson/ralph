package taskstore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImportFromYAML_ValidTasks(t *testing.T) {
	// Create a temp directory for the store
	dir := t.TempDir()
	store, err := NewLocalStore(dir)
	require.NoError(t, err)

	// Create a YAML file with valid tasks
	yamlContent := `tasks:
  - id: task-1
    title: "First Task"
    description: "This is the first task"
    status: open
    acceptance:
      - "Criterion 1"
    verify:
      - ["go", "test", "./..."]
    labels:
      area: core
  - id: task-2
    title: "Second Task"
    parentId: task-1
    dependsOn:
      - task-1
    status: completed
    acceptance:
      - "Criterion 2"
`
	yamlFile := filepath.Join(t.TempDir(), "tasks.yaml")
	require.NoError(t, os.WriteFile(yamlFile, []byte(yamlContent), 0644))

	// Import tasks
	result, err := ImportFromYAML(store, yamlFile)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Imported)
	assert.Empty(t, result.Errors)

	// Verify tasks were imported
	tasks, err := store.List()
	require.NoError(t, err)
	assert.Len(t, tasks, 2)

	// Check task-1
	task1, err := store.Get("task-1")
	require.NoError(t, err)
	assert.Equal(t, "First Task", task1.Title)
	assert.Equal(t, "This is the first task", task1.Description)
	assert.Equal(t, StatusOpen, task1.Status)
	assert.Nil(t, task1.ParentID)
	assert.Empty(t, task1.DependsOn)
	assert.Equal(t, []string{"Criterion 1"}, task1.Acceptance)
	assert.Equal(t, [][]string{{"go", "test", "./..."}}, task1.Verify)
	assert.Equal(t, "core", task1.Labels["area"])

	// Check task-2
	task2, err := store.Get("task-2")
	require.NoError(t, err)
	assert.Equal(t, "Second Task", task2.Title)
	assert.Equal(t, StatusCompleted, task2.Status)
	require.NotNil(t, task2.ParentID)
	assert.Equal(t, "task-1", *task2.ParentID)
	assert.Equal(t, []string{"task-1"}, task2.DependsOn)
}

func TestImportFromYAML_DefaultsApplied(t *testing.T) {
	dir := t.TempDir()
	store, err := NewLocalStore(dir)
	require.NoError(t, err)

	// Task with minimal fields (status defaults to "open")
	yamlContent := `tasks:
  - id: minimal-task
    title: "Minimal Task"
`
	yamlFile := filepath.Join(t.TempDir(), "tasks.yaml")
	require.NoError(t, os.WriteFile(yamlFile, []byte(yamlContent), 0644))

	result, err := ImportFromYAML(store, yamlFile)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Imported)

	task, err := store.Get("minimal-task")
	require.NoError(t, err)
	assert.Equal(t, StatusOpen, task.Status) // Default status
	assert.False(t, task.CreatedAt.IsZero())
	assert.False(t, task.UpdatedAt.IsZero())
}

func TestImportFromYAML_ValidationErrors(t *testing.T) {
	dir := t.TempDir()
	store, err := NewLocalStore(dir)
	require.NoError(t, err)

	// Task missing required fields
	yamlContent := `tasks:
  - id: valid-task
    title: "Valid Task"
  - id: ""
    title: "Missing ID"
  - id: missing-title
    title: ""
  - id: invalid-status
    title: "Invalid Status"
    status: bogus
`
	yamlFile := filepath.Join(t.TempDir(), "tasks.yaml")
	require.NoError(t, os.WriteFile(yamlFile, []byte(yamlContent), 0644))

	result, err := ImportFromYAML(store, yamlFile)
	require.NoError(t, err) // Import completes but with errors
	assert.Equal(t, 1, result.Imported)
	assert.Len(t, result.Errors, 3)

	// Check that error messages include task IDs where available
	var foundMissingID, foundMissingTitle, foundInvalidStatus bool
	for _, importErr := range result.Errors {
		if importErr.ID == "" {
			foundMissingID = true
			assert.Contains(t, importErr.Reason, "id is required")
		}
		if importErr.ID == "missing-title" {
			foundMissingTitle = true
			assert.Contains(t, importErr.Reason, "title is required")
		}
		if importErr.ID == "invalid-status" {
			foundInvalidStatus = true
			assert.Contains(t, importErr.Reason, "invalid")
		}
	}
	assert.True(t, foundMissingID, "should report missing ID error")
	assert.True(t, foundMissingTitle, "should report missing title error")
	assert.True(t, foundInvalidStatus, "should report invalid status error")

	// Only valid task should be saved
	tasks, err := store.List()
	require.NoError(t, err)
	assert.Len(t, tasks, 1)
}

func TestImportFromYAML_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	store, err := NewLocalStore(dir)
	require.NoError(t, err)

	_, err = ImportFromYAML(store, "/nonexistent/path/tasks.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read YAML file")
}

func TestImportFromYAML_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	store, err := NewLocalStore(dir)
	require.NoError(t, err)

	yamlContent := `tasks:
  - id: task-1
    title: "Task
    invalid yaml here
`
	yamlFile := filepath.Join(t.TempDir(), "tasks.yaml")
	require.NoError(t, os.WriteFile(yamlFile, []byte(yamlContent), 0644))

	_, err = ImportFromYAML(store, yamlFile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse YAML")
}

func TestImportFromYAML_EmptyTasks(t *testing.T) {
	dir := t.TempDir()
	store, err := NewLocalStore(dir)
	require.NoError(t, err)

	yamlContent := `tasks: []`
	yamlFile := filepath.Join(t.TempDir(), "tasks.yaml")
	require.NoError(t, os.WriteFile(yamlFile, []byte(yamlContent), 0644))

	result, err := ImportFromYAML(store, yamlFile)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Imported)
	assert.Empty(t, result.Errors)
}

func TestImportFromYAML_UpdatesExistingTasks(t *testing.T) {
	dir := t.TempDir()
	store, err := NewLocalStore(dir)
	require.NoError(t, err)

	// First import
	yamlContent := `tasks:
  - id: task-1
    title: "Original Title"
    status: open
`
	yamlFile := filepath.Join(t.TempDir(), "tasks.yaml")
	require.NoError(t, os.WriteFile(yamlFile, []byte(yamlContent), 0644))

	result, err := ImportFromYAML(store, yamlFile)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Imported)

	// Second import with updated title
	yamlContent2 := `tasks:
  - id: task-1
    title: "Updated Title"
    status: completed
`
	yamlFile2 := filepath.Join(t.TempDir(), "tasks2.yaml")
	require.NoError(t, os.WriteFile(yamlFile2, []byte(yamlContent2), 0644))

	result2, err := ImportFromYAML(store, yamlFile2)
	require.NoError(t, err)
	assert.Equal(t, 1, result2.Imported)

	// Verify task was updated
	task, err := store.Get("task-1")
	require.NoError(t, err)
	assert.Equal(t, "Updated Title", task.Title)
	assert.Equal(t, StatusCompleted, task.Status)
}

func TestImportFromYAML_PreservesTimestamps(t *testing.T) {
	dir := t.TempDir()
	store, err := NewLocalStore(dir)
	require.NoError(t, err)

	yamlContent := `tasks:
  - id: task-1
    title: "Task 1"
`
	yamlFile := filepath.Join(t.TempDir(), "tasks.yaml")
	require.NoError(t, os.WriteFile(yamlFile, []byte(yamlContent), 0644))

	result, err := ImportFromYAML(store, yamlFile)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Imported)

	task, err := store.Get("task-1")
	require.NoError(t, err)

	// Timestamps should be set
	assert.False(t, task.CreatedAt.IsZero())
	assert.False(t, task.UpdatedAt.IsZero())

	// CreatedAt and UpdatedAt should be the same on initial import
	assert.Equal(t, task.CreatedAt, task.UpdatedAt)
}

func TestImportFromYAML_ComplexDependsOn(t *testing.T) {
	dir := t.TempDir()
	store, err := NewLocalStore(dir)
	require.NoError(t, err)

	yamlContent := `tasks:
  - id: parent
    title: "Parent Task"
  - id: child-1
    title: "Child 1"
    parentId: parent
    dependsOn: []
  - id: child-2
    title: "Child 2"
    parentId: parent
    dependsOn:
      - child-1
  - id: grandchild
    title: "Grandchild"
    parentId: child-2
    dependsOn:
      - child-1
      - child-2
`
	yamlFile := filepath.Join(t.TempDir(), "tasks.yaml")
	require.NoError(t, os.WriteFile(yamlFile, []byte(yamlContent), 0644))

	result, err := ImportFromYAML(store, yamlFile)
	require.NoError(t, err)
	assert.Equal(t, 4, result.Imported)

	grandchild, err := store.Get("grandchild")
	require.NoError(t, err)
	assert.Equal(t, []string{"child-1", "child-2"}, grandchild.DependsOn)
}

func TestParseYAML_Valid(t *testing.T) {
	yamlContent := []byte(`tasks:
  - id: task-1
    title: "First Task"
    description: "Description here"
    status: open
    acceptance:
      - "Criterion 1"
    verify:
      - ["go", "test", "./..."]
    labels:
      area: core
  - id: task-2
    title: "Second Task"
    parentId: task-1
    dependsOn:
      - task-1
    status: completed
`)

	result, err := ParseYAML(yamlContent)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Tasks, 2)

	// Check first task
	assert.Equal(t, "task-1", result.Tasks[0].ID)
	assert.Equal(t, "First Task", result.Tasks[0].Title)
	assert.Equal(t, "Description here", result.Tasks[0].Description)
	assert.Equal(t, "open", result.Tasks[0].Status)
	assert.Equal(t, []string{"Criterion 1"}, result.Tasks[0].Acceptance)
	assert.Equal(t, [][]string{{"go", "test", "./..."}}, result.Tasks[0].Verify)
	assert.Equal(t, "core", result.Tasks[0].Labels["area"])

	// Check second task
	assert.Equal(t, "task-2", result.Tasks[1].ID)
	assert.Equal(t, "Second Task", result.Tasks[1].Title)
	assert.Equal(t, "task-1", result.Tasks[1].ParentID)
	assert.Equal(t, []string{"task-1"}, result.Tasks[1].DependsOn)
	assert.Equal(t, "completed", result.Tasks[1].Status)
}

func TestParseYAML_Invalid(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent []byte
	}{
		{
			name: "malformed YAML syntax",
			yamlContent: []byte(`tasks:
  - id: task-1
    title: "Task
    invalid yaml here
`),
		},
		{
			name:        "invalid YAML structure",
			yamlContent: []byte(`not: valid: yaml: structure`),
		},
		{
			name: "invalid indentation",
			yamlContent: []byte(`tasks:
- id: task-1
  title: "Task"
    extra_indent: bad
`),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ParseYAML(tc.yamlContent)
			assert.Error(t, err)
			assert.Nil(t, result)
		})
	}
}
