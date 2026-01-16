package verifier

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerificationResult_DefaultValues(t *testing.T) {
	result := VerificationResult{}

	assert.False(t, result.Passed)
	assert.Nil(t, result.Command)
	assert.Empty(t, result.Output)
	assert.Zero(t, result.Duration)
}

func TestVerificationResult_AllFields(t *testing.T) {
	result := VerificationResult{
		Passed:   true,
		Command:  []string{"go", "test", "./..."},
		Output:   "ok  	github.com/yarlson/ralph/internal/verifier\n",
		Duration: 5 * time.Second,
	}

	assert.True(t, result.Passed)
	assert.Equal(t, []string{"go", "test", "./..."}, result.Command)
	assert.Contains(t, result.Output, "ok")
	assert.Equal(t, 5*time.Second, result.Duration)
}

func TestVerificationResult_JSONSerialization(t *testing.T) {
	result := VerificationResult{
		Passed:   true,
		Command:  []string{"npm", "run", "typecheck"},
		Output:   "All checks passed",
		Duration: 2500 * time.Millisecond,
	}

	// Marshal to JSON
	data, err := json.Marshal(result)
	require.NoError(t, err)

	// Verify JSON contains expected fields
	var parsed map[string]any
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, true, parsed["passed"])
	assert.Equal(t, []any{"npm", "run", "typecheck"}, parsed["command"])
	assert.Equal(t, "All checks passed", parsed["output"])
	// Duration is stored as nanoseconds in JSON
	assert.Equal(t, float64(2500*time.Millisecond), parsed["duration"])

	// Unmarshal back
	var unmarshaled VerificationResult
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, result.Passed, unmarshaled.Passed)
	assert.Equal(t, result.Command, unmarshaled.Command)
	assert.Equal(t, result.Output, unmarshaled.Output)
	assert.Equal(t, result.Duration, unmarshaled.Duration)
}

func TestVerificationResult_FailedWithOutput(t *testing.T) {
	result := VerificationResult{
		Passed:   false,
		Command:  []string{"go", "test", "./..."},
		Output:   "FAIL: TestSomething\nExpected: true\nGot: false",
		Duration: 10 * time.Second,
	}

	assert.False(t, result.Passed)
	assert.Contains(t, result.Output, "FAIL")
}

// MockVerifier implements the Verifier interface for testing.
type MockVerifier struct {
	VerifyFunc     func(ctx context.Context, commands [][]string) ([]VerificationResult, error)
	VerifyTaskFunc func(ctx context.Context, commands [][]string) ([]VerificationResult, error)
}

func (m *MockVerifier) Verify(ctx context.Context, commands [][]string) ([]VerificationResult, error) {
	if m.VerifyFunc != nil {
		return m.VerifyFunc(ctx, commands)
	}
	return nil, nil
}

func (m *MockVerifier) VerifyTask(ctx context.Context, commands [][]string) ([]VerificationResult, error) {
	if m.VerifyTaskFunc != nil {
		return m.VerifyTaskFunc(ctx, commands)
	}
	return nil, nil
}

func TestVerifier_Interface(t *testing.T) {
	// Test that MockVerifier satisfies the Verifier interface
	var _ Verifier = (*MockVerifier)(nil)
}

func TestVerifier_VerifyWithMock(t *testing.T) {
	mock := &MockVerifier{
		VerifyFunc: func(ctx context.Context, commands [][]string) ([]VerificationResult, error) {
			results := make([]VerificationResult, len(commands))
			for i, cmd := range commands {
				results[i] = VerificationResult{
					Passed:   true,
					Command:  cmd,
					Output:   "success",
					Duration: time.Second,
				}
			}
			return results, nil
		},
	}

	ctx := context.Background()
	commands := [][]string{
		{"go", "build", "./..."},
		{"go", "test", "./..."},
	}

	results, err := mock.Verify(ctx, commands)
	require.NoError(t, err)
	require.Len(t, results, 2)

	assert.True(t, results[0].Passed)
	assert.Equal(t, []string{"go", "build", "./..."}, results[0].Command)

	assert.True(t, results[1].Passed)
	assert.Equal(t, []string{"go", "test", "./..."}, results[1].Command)
}

func TestVerifier_VerifyTaskWithMock(t *testing.T) {
	mock := &MockVerifier{
		VerifyTaskFunc: func(ctx context.Context, commands [][]string) ([]VerificationResult, error) {
			return []VerificationResult{
				{Passed: true, Command: commands[0], Output: "ok", Duration: time.Second},
			}, nil
		},
	}

	ctx := context.Background()
	commands := [][]string{{"go", "test", "./internal/verifier/..."}}

	results, err := mock.VerifyTask(ctx, commands)
	require.NoError(t, err)
	require.Len(t, results, 1)

	assert.True(t, results[0].Passed)
}

func TestVerifier_ContextCancellation(t *testing.T) {
	mock := &MockVerifier{
		VerifyFunc: func(ctx context.Context, commands [][]string) ([]VerificationResult, error) {
			// Check if context is cancelled
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				return []VerificationResult{{Passed: true}}, nil
			}
		},
	}

	// Test with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := mock.Verify(ctx, [][]string{{"echo", "test"}})
	assert.ErrorIs(t, err, context.Canceled)
}

func TestVerifier_Timeout(t *testing.T) {
	mock := &MockVerifier{
		VerifyFunc: func(ctx context.Context, commands [][]string) ([]VerificationResult, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(100 * time.Millisecond):
				return []VerificationResult{{Passed: true}}, nil
			}
		},
	}

	// Test with timeout that expires
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := mock.Verify(ctx, [][]string{{"sleep", "1"}})
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}
