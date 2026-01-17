package internal

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileType_String(t *testing.T) {
	tests := []struct {
		name     string
		ft       FileType
		expected string
	}{
		{"unknown", FileTypeUnknown, "unknown"},
		{"prd", FileTypePRD, "prd"},
		{"tasks", FileTypeTasks, "tasks"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.ft.String())
		})
	}
}

func TestDetectFileType_PRDPatterns(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected FileType
	}{
		{
			name:     "PRD with objectives section",
			content:  "# Product Spec\n\n## Objectives\n\nBuild a great product.",
			expected: FileTypePRD,
		},
		{
			name:     "PRD with requirements section",
			content:  "# Spec\n\n## Requirements\n\n- Feature A\n- Feature B",
			expected: FileTypePRD,
		},
		{
			name:     "PRD with user stories section",
			content:  "# Document\n\n## User Stories\n\nAs a user I want...",
			expected: FileTypePRD,
		},
		{
			name:     "PRD with acceptance criteria section",
			content:  "# Feature\n\n## Acceptance Criteria\n\n- Must work",
			expected: FileTypePRD,
		},
		{
			name:     "PRD with overview section",
			content:  "# Product\n\n## Overview\n\nThis is the overview.",
			expected: FileTypePRD,
		},
		{
			name:     "PRD case insensitive",
			content:  "# doc\n\n## OBJECTIVES\n\nSome text",
			expected: FileTypePRD,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectFileType(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDetectFileType_TaskPatterns(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected FileType
	}{
		{
			name:     "YAML task with id field",
			content:  "id: task-001\ntitle: Do something\nstatus: open",
			expected: FileTypeTasks,
		},
		{
			name:     "YAML task with tasks array",
			content:  "tasks:\n  - id: task-001\n    title: Task one",
			expected: FileTypeTasks,
		},
		{
			name:     "YAML with status field",
			content:  "title: Some task\nstatus: in_progress\ndescription: Do stuff",
			expected: FileTypeTasks,
		},
		{
			name:     "YAML with depends_on field",
			content:  "id: task-002\ntitle: Task two\ndepends_on:\n  - task-001",
			expected: FileTypeTasks,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectFileType(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDetectFileType_Unknown(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"empty content", ""},
		{"random text", "Hello world, this is just some text."},
		{"code snippet", "func main() { fmt.Println(\"hello\") }"},
		{"markdown without PRD markers", "# Title\n\nSome paragraph text."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectFileType(tt.content)
			assert.Equal(t, FileTypeUnknown, result)
		})
	}
}

func TestFileType_Constants(t *testing.T) {
	// Verify constants have expected values
	assert.Equal(t, FileType(0), FileTypeUnknown)
	assert.Equal(t, FileType(1), FileTypePRD)
	assert.Equal(t, FileType(2), FileTypeTasks)
}

// YAMLPipeline tests

func TestYAMLPipeline_ExecutionOrder(t *testing.T) {
	t.Run("executes import, init, run in sequence (skips decompose)", func(t *testing.T) {
		callOrder := make([]string, 0, 3)

		importer := &MockImporter{CallOrder: &callOrder}
		initializer := &MockInitializer{CallOrder: &callOrder}
		runner := &MockRunner{CallOrder: &callOrder}

		var buf bytes.Buffer
		pipeline := NewYAMLPipeline(importer, initializer, runner, &buf)

		err := pipeline.Execute(context.Background(), "tasks.yaml")
		require.NoError(t, err)

		// Should NOT have decompose in the call order
		require.Equal(t, []string{"import", "init", "run"}, callOrder)
	})
}

func TestYAMLPipeline_InitializingMessage(t *testing.T) {
	t.Run("shows 'Initializing from' message with filename", func(t *testing.T) {
		var buf bytes.Buffer
		pipeline := NewYAMLPipeline(
			&MockImporter{},
			&MockInitializer{},
			&MockRunner{},
			&buf,
		)

		err := pipeline.Execute(context.Background(), "my-tasks.yaml")
		require.NoError(t, err)

		assert.Contains(t, buf.String(), "Initializing")
		assert.Contains(t, buf.String(), "my-tasks.yaml")
	})
}

func TestYAMLPipeline_StopsOnImportError(t *testing.T) {
	t.Run("stops and reports error if import fails", func(t *testing.T) {
		importer := &MockImporter{
			ImportFunc: func() (*ImportResultInfo, error) {
				return nil, errors.New("import failed: validation error")
			},
		}
		initializer := &MockInitializer{}
		runner := &MockRunner{}

		var buf bytes.Buffer
		pipeline := NewYAMLPipeline(importer, initializer, runner, &buf)

		err := pipeline.Execute(context.Background(), "test.yaml")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "import failed")
		assert.False(t, initializer.Called)
		assert.False(t, runner.Called)
	})
}

func TestYAMLPipeline_StopsOnInitError(t *testing.T) {
	t.Run("stops and reports error if init fails", func(t *testing.T) {
		initializer := &MockInitializer{
			InitFunc: func() error {
				return errors.New("init failed: no ready tasks")
			},
		}
		runner := &MockRunner{}

		var buf bytes.Buffer
		pipeline := NewYAMLPipeline(
			&MockImporter{},
			initializer,
			runner,
			&buf,
		)

		err := pipeline.Execute(context.Background(), "test.yaml")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "init failed")
		assert.False(t, runner.Called)
	})
}

func TestYAMLPipeline_StopsOnRunError(t *testing.T) {
	t.Run("stops and reports error if run fails", func(t *testing.T) {
		runner := &MockRunner{
			RunFunc: func(ctx context.Context) error {
				return errors.New("run failed: gutter detected")
			},
		}

		var buf bytes.Buffer
		pipeline := NewYAMLPipeline(
			&MockImporter{},
			&MockInitializer{},
			runner,
			&buf,
		)

		err := pipeline.Execute(context.Background(), "test.yaml")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "run failed")
	})
}

func TestYAMLPipeline_AllStagesCalled(t *testing.T) {
	t.Run("all stages are called on success", func(t *testing.T) {
		importer := &MockImporter{}
		initializer := &MockInitializer{}
		runner := &MockRunner{}

		var buf bytes.Buffer
		pipeline := NewYAMLPipeline(importer, initializer, runner, &buf)

		err := pipeline.Execute(context.Background(), "test.yaml")

		require.NoError(t, err)
		assert.True(t, importer.Called)
		assert.True(t, initializer.Called)
		assert.True(t, runner.Called)
	})
}

func TestYAMLPipeline_TaskCountSummary(t *testing.T) {
	t.Run("shows task count summary after import", func(t *testing.T) {
		importer := &MockImporter{
			ImportFunc: func() (*ImportResultInfo, error) {
				return &ImportResultInfo{TaskCount: 8}, nil
			},
		}

		var buf bytes.Buffer
		pipeline := NewYAMLPipeline(
			importer,
			&MockInitializer{},
			&MockRunner{},
			&buf,
		)

		err := pipeline.Execute(context.Background(), "tasks.yaml")
		require.NoError(t, err)

		assert.Contains(t, buf.String(), "8")
		assert.Contains(t, buf.String(), "task")
	})
}
