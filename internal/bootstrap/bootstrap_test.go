package bootstrap

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockDecomposer implements the Decomposer interface for testing
type MockDecomposer struct {
	DecomposeFunc func(ctx context.Context) (*DecomposeResultInfo, error)
	Called        bool
	CallOrder     *[]string
}

func (m *MockDecomposer) Decompose(ctx context.Context) (*DecomposeResultInfo, error) {
	m.Called = true
	if m.CallOrder != nil {
		*m.CallOrder = append(*m.CallOrder, "decompose")
	}
	if m.DecomposeFunc != nil {
		return m.DecomposeFunc(ctx)
	}
	return &DecomposeResultInfo{TaskCount: 5, YAMLPath: "tasks.yaml"}, nil
}

// MockImporter implements the Importer interface for testing
type MockImporter struct {
	ImportFunc func() (*ImportResultInfo, error)
	Called     bool
	CallOrder  *[]string
}

func (m *MockImporter) Import() (*ImportResultInfo, error) {
	m.Called = true
	if m.CallOrder != nil {
		*m.CallOrder = append(*m.CallOrder, "import")
	}
	if m.ImportFunc != nil {
		return m.ImportFunc()
	}
	return &ImportResultInfo{TaskCount: 5}, nil
}

// MockInitializer implements the Initializer interface for testing
type MockInitializer struct {
	InitFunc  func() error
	Called    bool
	CallOrder *[]string
}

func (m *MockInitializer) Init() error {
	m.Called = true
	if m.CallOrder != nil {
		*m.CallOrder = append(*m.CallOrder, "init")
	}
	if m.InitFunc != nil {
		return m.InitFunc()
	}
	return nil
}

// MockRunner implements the Runner interface for testing
type MockRunner struct {
	RunFunc   func(ctx context.Context) error
	Called    bool
	CallOrder *[]string
}

func (m *MockRunner) Run(ctx context.Context) error {
	m.Called = true
	if m.CallOrder != nil {
		*m.CallOrder = append(*m.CallOrder, "run")
	}
	if m.RunFunc != nil {
		return m.RunFunc(ctx)
	}
	return nil
}

// PRDPipeline tests

func TestPRDPipeline_ExecutionOrder(t *testing.T) {
	t.Run("executes decompose, import, init, run in sequence", func(t *testing.T) {
		callOrder := make([]string, 0, 4)

		decomposer := &MockDecomposer{CallOrder: &callOrder}
		importer := &MockImporter{CallOrder: &callOrder}
		initializer := &MockInitializer{CallOrder: &callOrder}
		runner := &MockRunner{CallOrder: &callOrder}

		var buf bytes.Buffer
		pipeline := NewPRDPipeline(decomposer, importer, initializer, runner, &buf)

		err := pipeline.Execute(context.Background(), "test.md")
		require.NoError(t, err)

		require.Equal(t, []string{"decompose", "import", "init", "run"}, callOrder)
	})
}

func TestPRDPipeline_AnalyzingMessage(t *testing.T) {
	t.Run("shows 'Analyzing PRD' message with filename", func(t *testing.T) {
		var buf bytes.Buffer
		pipeline := NewPRDPipeline(
			&MockDecomposer{},
			&MockImporter{},
			&MockInitializer{},
			&MockRunner{},
			&buf,
		)

		err := pipeline.Execute(context.Background(), "my-feature.md")
		require.NoError(t, err)

		assert.Contains(t, buf.String(), "Analyzing PRD")
		assert.Contains(t, buf.String(), "my-feature.md")
	})
}

func TestPRDPipeline_TaskCountSummary(t *testing.T) {
	t.Run("shows task count summary after decomposition", func(t *testing.T) {
		decomposer := &MockDecomposer{
			DecomposeFunc: func(ctx context.Context) (*DecomposeResultInfo, error) {
				return &DecomposeResultInfo{TaskCount: 10, YAMLPath: "tasks.yaml"}, nil
			},
		}

		var buf bytes.Buffer
		pipeline := NewPRDPipeline(
			decomposer,
			&MockImporter{},
			&MockInitializer{},
			&MockRunner{},
			&buf,
		)

		err := pipeline.Execute(context.Background(), "prd.md")
		require.NoError(t, err)

		assert.Contains(t, buf.String(), "10")
		assert.Contains(t, buf.String(), "task")
	})
}

func TestPRDPipeline_StopsOnDecomposeError(t *testing.T) {
	t.Run("stops and reports error if decompose fails", func(t *testing.T) {
		decomposer := &MockDecomposer{
			DecomposeFunc: func(ctx context.Context) (*DecomposeResultInfo, error) {
				return nil, errors.New("decomposition failed: Claude error")
			},
		}
		importer := &MockImporter{}
		initializer := &MockInitializer{}
		runner := &MockRunner{}

		var buf bytes.Buffer
		pipeline := NewPRDPipeline(decomposer, importer, initializer, runner, &buf)

		err := pipeline.Execute(context.Background(), "test.md")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "decomposition failed")
		assert.False(t, importer.Called)
		assert.False(t, initializer.Called)
		assert.False(t, runner.Called)
	})
}

func TestPRDPipeline_StopsOnImportError(t *testing.T) {
	t.Run("stops and reports error if import fails", func(t *testing.T) {
		importer := &MockImporter{
			ImportFunc: func() (*ImportResultInfo, error) {
				return nil, errors.New("import failed: validation error")
			},
		}
		initializer := &MockInitializer{}
		runner := &MockRunner{}

		var buf bytes.Buffer
		pipeline := NewPRDPipeline(
			&MockDecomposer{},
			importer,
			initializer,
			runner,
			&buf,
		)

		err := pipeline.Execute(context.Background(), "test.md")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "import failed")
		assert.False(t, initializer.Called)
		assert.False(t, runner.Called)
	})
}

func TestPRDPipeline_StopsOnInitError(t *testing.T) {
	t.Run("stops and reports error if init fails", func(t *testing.T) {
		initializer := &MockInitializer{
			InitFunc: func() error {
				return errors.New("init failed: no ready tasks")
			},
		}
		runner := &MockRunner{}

		var buf bytes.Buffer
		pipeline := NewPRDPipeline(
			&MockDecomposer{},
			&MockImporter{},
			initializer,
			runner,
			&buf,
		)

		err := pipeline.Execute(context.Background(), "test.md")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "init failed")
		assert.False(t, runner.Called)
	})
}

func TestPRDPipeline_StopsOnRunError(t *testing.T) {
	t.Run("stops and reports error if run fails", func(t *testing.T) {
		runner := &MockRunner{
			RunFunc: func(ctx context.Context) error {
				return errors.New("run failed: gutter detected")
			},
		}

		var buf bytes.Buffer
		pipeline := NewPRDPipeline(
			&MockDecomposer{},
			&MockImporter{},
			&MockInitializer{},
			runner,
			&buf,
		)

		err := pipeline.Execute(context.Background(), "test.md")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "run failed")
	})
}

func TestPRDPipeline_AllStagesCalled(t *testing.T) {
	t.Run("all stages are called on success", func(t *testing.T) {
		decomposer := &MockDecomposer{}
		importer := &MockImporter{}
		initializer := &MockInitializer{}
		runner := &MockRunner{}

		var buf bytes.Buffer
		pipeline := NewPRDPipeline(decomposer, importer, initializer, runner, &buf)

		err := pipeline.Execute(context.Background(), "test.md")

		require.NoError(t, err)
		assert.True(t, decomposer.Called)
		assert.True(t, importer.Called)
		assert.True(t, initializer.Called)
		assert.True(t, runner.Called)
	})
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

// Verify interfaces are implemented correctly
func TestInterfaces(t *testing.T) {
	var _ Decomposer = (*MockDecomposer)(nil)
	var _ Importer = (*MockImporter)(nil)
	var _ Initializer = (*MockInitializer)(nil)
	var _ Runner = (*MockRunner)(nil)
}
