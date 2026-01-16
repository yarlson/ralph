package verifier

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCommandRunner(t *testing.T) {
	t.Run("creates runner with default settings", func(t *testing.T) {
		runner := NewCommandRunner("")
		require.NotNil(t, runner)
	})

	t.Run("creates runner with working directory", func(t *testing.T) {
		runner := NewCommandRunner("/tmp")
		require.NotNil(t, runner)
	})
}

func TestCommandRunner_Verify(t *testing.T) {
	t.Run("executes simple command successfully", func(t *testing.T) {
		runner := NewCommandRunner("")
		ctx := context.Background()

		results, err := runner.Verify(ctx, [][]string{{"echo", "hello"}})
		require.NoError(t, err)
		require.Len(t, results, 1)

		assert.True(t, results[0].Passed)
		assert.Equal(t, []string{"echo", "hello"}, results[0].Command)
		assert.Contains(t, results[0].Output, "hello")
		assert.Greater(t, results[0].Duration, time.Duration(0))
	})

	t.Run("captures stderr in output", func(t *testing.T) {
		runner := NewCommandRunner("")
		ctx := context.Background()

		// Command that writes to stderr
		results, err := runner.Verify(ctx, [][]string{{"sh", "-c", "echo error >&2"}})
		require.NoError(t, err)
		require.Len(t, results, 1)

		assert.True(t, results[0].Passed)
		assert.Contains(t, results[0].Output, "error")
	})

	t.Run("reports failure for non-zero exit code", func(t *testing.T) {
		runner := NewCommandRunner("")
		ctx := context.Background()

		results, err := runner.Verify(ctx, [][]string{{"sh", "-c", "exit 1"}})
		require.NoError(t, err)
		require.Len(t, results, 1)

		assert.False(t, results[0].Passed)
	})

	t.Run("executes multiple commands sequentially", func(t *testing.T) {
		runner := NewCommandRunner("")
		ctx := context.Background()

		commands := [][]string{
			{"echo", "first"},
			{"echo", "second"},
			{"echo", "third"},
		}

		results, err := runner.Verify(ctx, commands)
		require.NoError(t, err)
		require.Len(t, results, 3)

		for i, result := range results {
			assert.True(t, result.Passed, "command %d should pass", i)
		}
		assert.Contains(t, results[0].Output, "first")
		assert.Contains(t, results[1].Output, "second")
		assert.Contains(t, results[2].Output, "third")
	})

	t.Run("continues after failed command", func(t *testing.T) {
		runner := NewCommandRunner("")
		ctx := context.Background()

		commands := [][]string{
			{"echo", "first"},
			{"sh", "-c", "exit 1"},
			{"echo", "third"},
		}

		results, err := runner.Verify(ctx, commands)
		require.NoError(t, err)
		require.Len(t, results, 3)

		assert.True(t, results[0].Passed)
		assert.False(t, results[1].Passed)
		assert.True(t, results[2].Passed)
	})

	t.Run("returns empty results for empty commands", func(t *testing.T) {
		runner := NewCommandRunner("")
		ctx := context.Background()

		results, err := runner.Verify(ctx, [][]string{})
		require.NoError(t, err)
		assert.Empty(t, results)
	})

	t.Run("returns error for nil context", func(t *testing.T) {
		runner := NewCommandRunner("")

		//nolint:staticcheck // Testing nil context behavior
		_, err := runner.Verify(nil, [][]string{{"echo", "test"}})
		assert.Error(t, err)
	})
}

func TestCommandRunner_VerifyTask(t *testing.T) {
	t.Run("delegates to Verify", func(t *testing.T) {
		runner := NewCommandRunner("")
		ctx := context.Background()

		results, err := runner.VerifyTask(ctx, [][]string{{"echo", "task"}})
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
		assert.Contains(t, results[0].Output, "task")
	})
}

func TestCommandRunner_Timeout(t *testing.T) {
	t.Run("respects context timeout", func(t *testing.T) {
		runner := NewCommandRunner("")
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Command that would take longer than timeout
		results, err := runner.Verify(ctx, [][]string{{"sleep", "10"}})
		require.NoError(t, err)
		require.Len(t, results, 1)

		assert.False(t, results[0].Passed)
		// Output should indicate timeout or context cancellation
	})

	t.Run("cancellation stops execution", func(t *testing.T) {
		runner := NewCommandRunner("")
		ctx, cancel := context.WithCancel(context.Background())

		// Cancel after short delay
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		start := time.Now()
		results, err := runner.Verify(ctx, [][]string{{"sleep", "10"}})
		elapsed := time.Since(start)

		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
		assert.Less(t, elapsed, 5*time.Second, "should have been cancelled quickly")
	})
}

func TestCommandRunner_WorkingDirectory(t *testing.T) {
	t.Run("runs commands in specified directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		runner := NewCommandRunner(tmpDir)
		ctx := context.Background()

		results, err := runner.Verify(ctx, [][]string{{"pwd"}})
		require.NoError(t, err)
		require.Len(t, results, 1)

		assert.True(t, results[0].Passed)
		assert.Contains(t, results[0].Output, tmpDir)
	})
}

func TestCommandRunner_Allowlist(t *testing.T) {
	t.Run("allows commands when no allowlist set", func(t *testing.T) {
		runner := NewCommandRunner("")
		ctx := context.Background()

		results, err := runner.Verify(ctx, [][]string{{"echo", "test"}})
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("allows commands in allowlist", func(t *testing.T) {
		runner := NewCommandRunner("")
		runner.SetAllowedCommands([]string{"echo", "sh"})
		ctx := context.Background()

		results, err := runner.Verify(ctx, [][]string{{"echo", "allowed"}})
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("blocks commands not in allowlist", func(t *testing.T) {
		runner := NewCommandRunner("")
		runner.SetAllowedCommands([]string{"go"})
		ctx := context.Background()

		results, err := runner.Verify(ctx, [][]string{{"echo", "blocked"}})
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
		assert.Contains(t, strings.ToLower(results[0].Output), "not allowed")
	})

	t.Run("allowlist checks base command only", func(t *testing.T) {
		runner := NewCommandRunner("")
		runner.SetAllowedCommands([]string{"go"})
		ctx := context.Background()

		// go is allowed, so "go version" should work
		results, err := runner.Verify(ctx, [][]string{{"go", "version"}})
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})
}

func TestCommandRunner_OutputSize(t *testing.T) {
	t.Run("captures large output", func(t *testing.T) {
		runner := NewCommandRunner("")
		ctx := context.Background()

		// Generate output that's reasonably large but not excessive for tests
		results, err := runner.Verify(ctx, [][]string{{"sh", "-c", "for i in $(seq 1 100); do echo line$i; done"}})
		require.NoError(t, err)
		require.Len(t, results, 1)

		assert.True(t, results[0].Passed)
		assert.Contains(t, results[0].Output, "line1")
		assert.Contains(t, results[0].Output, "line100")
	})

	t.Run("respects max output size limit", func(t *testing.T) {
		runner := NewCommandRunner("")
		runner.SetMaxOutputSize(100) // Very small limit
		ctx := context.Background()

		// Generate output larger than limit
		results, err := runner.Verify(ctx, [][]string{{"sh", "-c", "for i in $(seq 1 100); do echo line$i; done"}})
		require.NoError(t, err)
		require.Len(t, results, 1)

		assert.True(t, results[0].Passed)
		assert.LessOrEqual(t, len(results[0].Output), 200) // Allow some buffer for truncation message
	})
}

func TestCommandRunner_InvalidCommand(t *testing.T) {
	t.Run("handles non-existent command", func(t *testing.T) {
		runner := NewCommandRunner("")
		ctx := context.Background()

		results, err := runner.Verify(ctx, [][]string{{"nonexistent-command-xyz"}})
		require.NoError(t, err)
		require.Len(t, results, 1)

		assert.False(t, results[0].Passed)
	})

	t.Run("handles empty command", func(t *testing.T) {
		runner := NewCommandRunner("")
		ctx := context.Background()

		results, err := runner.Verify(ctx, [][]string{{}})
		require.NoError(t, err)
		require.Len(t, results, 1)

		assert.False(t, results[0].Passed)
		assert.Contains(t, strings.ToLower(results[0].Output), "empty command")
	})
}
