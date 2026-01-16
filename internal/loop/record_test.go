package loop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIterationOutcome_String(t *testing.T) {
	tests := []struct {
		outcome IterationOutcome
		want    string
	}{
		{OutcomeSuccess, "success"},
		{OutcomeFailed, "failed"},
		{OutcomeBudgetExceeded, "budget_exceeded"},
		{OutcomeBlocked, "blocked"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, string(tt.outcome))
		})
	}
}

func TestIterationOutcome_IsValid(t *testing.T) {
	tests := []struct {
		outcome IterationOutcome
		valid   bool
	}{
		{OutcomeSuccess, true},
		{OutcomeFailed, true},
		{OutcomeBudgetExceeded, true},
		{OutcomeBlocked, true},
		{IterationOutcome("invalid"), false},
		{IterationOutcome(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.outcome), func(t *testing.T) {
			assert.Equal(t, tt.valid, tt.outcome.IsValid())
		})
	}
}

func TestIterationRecord_Defaults(t *testing.T) {
	record := IterationRecord{}

	assert.Empty(t, record.IterationID)
	assert.Empty(t, record.TaskID)
	assert.True(t, record.StartTime.IsZero())
	assert.True(t, record.EndTime.IsZero())
	assert.Empty(t, record.ClaudeInvocation)
	assert.Empty(t, record.BaseCommit)
	assert.Empty(t, record.ResultCommit)
	assert.Nil(t, record.VerificationOutputs)
	assert.Nil(t, record.FilesChanged)
	assert.Empty(t, record.Outcome)
	assert.Empty(t, record.Feedback)
}

func TestIterationRecord_AllFields(t *testing.T) {
	now := time.Now()
	later := now.Add(5 * time.Minute)

	record := IterationRecord{
		IterationID: "iter-001",
		TaskID:      "task-123",
		StartTime:   now,
		EndTime:     later,
		ClaudeInvocation: ClaudeInvocationMeta{
			Command:      []string{"claude", "code"},
			Model:        "claude-3-sonnet",
			SessionID:    "sess-abc",
			TotalCostUSD: 0.05,
			InputTokens:  1000,
			OutputTokens: 500,
		},
		BaseCommit:   "abc123",
		ResultCommit: "def456",
		VerificationOutputs: []VerificationOutput{
			{
				Command: []string{"go", "test", "./..."},
				Passed:  true,
				Output:  "PASS",
			},
		},
		FilesChanged: []string{"internal/loop/record.go", "internal/loop/record_test.go"},
		Outcome:      OutcomeSuccess,
		Feedback:     "",
	}

	assert.Equal(t, "iter-001", record.IterationID)
	assert.Equal(t, "task-123", record.TaskID)
	assert.Equal(t, now, record.StartTime)
	assert.Equal(t, later, record.EndTime)
	assert.Equal(t, "claude-3-sonnet", record.ClaudeInvocation.Model)
	assert.Equal(t, "abc123", record.BaseCommit)
	assert.Equal(t, "def456", record.ResultCommit)
	assert.Len(t, record.VerificationOutputs, 1)
	assert.Len(t, record.FilesChanged, 2)
	assert.Equal(t, OutcomeSuccess, record.Outcome)
}

func TestIterationRecord_JSONSerialization(t *testing.T) {
	now := time.Date(2026, 1, 16, 14, 30, 0, 0, time.UTC)

	record := IterationRecord{
		IterationID: "iter-001",
		TaskID:      "task-123",
		StartTime:   now,
		EndTime:     now.Add(5 * time.Minute),
		ClaudeInvocation: ClaudeInvocationMeta{
			Command:      []string{"claude", "code"},
			Model:        "claude-3-sonnet",
			SessionID:    "sess-abc",
			TotalCostUSD: 0.05,
			InputTokens:  1000,
			OutputTokens: 500,
		},
		BaseCommit:   "abc123",
		ResultCommit: "def456",
		VerificationOutputs: []VerificationOutput{
			{Command: []string{"go", "test"}, Passed: true, Output: "PASS"},
		},
		FilesChanged: []string{"file1.go"},
		Outcome:      OutcomeSuccess,
	}

	// Serialize
	data, err := json.MarshalIndent(record, "", "  ")
	require.NoError(t, err)

	// Deserialize
	var decoded IterationRecord
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, record.IterationID, decoded.IterationID)
	assert.Equal(t, record.TaskID, decoded.TaskID)
	assert.Equal(t, record.ClaudeInvocation.Model, decoded.ClaudeInvocation.Model)
	assert.Equal(t, record.Outcome, decoded.Outcome)
}

func TestIterationRecord_Duration(t *testing.T) {
	now := time.Now()
	record := IterationRecord{
		StartTime: now,
		EndTime:   now.Add(5*time.Minute + 30*time.Second),
	}

	duration := record.Duration()
	assert.Equal(t, 5*time.Minute+30*time.Second, duration)
}

func TestIterationRecord_Duration_ZeroTimes(t *testing.T) {
	record := IterationRecord{}

	duration := record.Duration()
	assert.Equal(t, time.Duration(0), duration)
}

func TestClaudeInvocationMeta_JSONSerialization(t *testing.T) {
	meta := ClaudeInvocationMeta{
		Command:      []string{"claude", "code", "-p", "test"},
		Model:        "claude-3-sonnet",
		SessionID:    "sess-123",
		TotalCostUSD: 0.123,
		InputTokens:  5000,
		OutputTokens: 2500,
	}

	data, err := json.Marshal(meta)
	require.NoError(t, err)

	var decoded ClaudeInvocationMeta
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, meta, decoded)
}

func TestVerificationOutput_JSONSerialization(t *testing.T) {
	output := VerificationOutput{
		Command:  []string{"go", "test", "./..."},
		Passed:   false,
		Output:   "FAIL: TestSomething",
		Duration: 5 * time.Second,
	}

	data, err := json.Marshal(output)
	require.NoError(t, err)

	var decoded VerificationOutput
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, output.Command, decoded.Command)
	assert.Equal(t, output.Passed, decoded.Passed)
	assert.Equal(t, output.Output, decoded.Output)
}

func TestNewIterationRecord(t *testing.T) {
	record := NewIterationRecord("task-456")

	assert.NotEmpty(t, record.IterationID)
	assert.Equal(t, "task-456", record.TaskID)
	assert.False(t, record.StartTime.IsZero())
	assert.True(t, record.EndTime.IsZero())
}

func TestIterationRecord_Complete(t *testing.T) {
	record := NewIterationRecord("task-456")
	assert.True(t, record.EndTime.IsZero())

	record.Complete(OutcomeSuccess)

	assert.False(t, record.EndTime.IsZero())
	assert.Equal(t, OutcomeSuccess, record.Outcome)
}

func TestIterationRecord_SetFeedback(t *testing.T) {
	record := NewIterationRecord("task-456")
	record.SetFeedback("Verification failed: go test failed")

	assert.Equal(t, "Verification failed: go test failed", record.Feedback)
}

func TestSaveRecord(t *testing.T) {
	dir := t.TempDir()
	record := IterationRecord{
		IterationID: "iter-001",
		TaskID:      "task-123",
		StartTime:   time.Now(),
		EndTime:     time.Now().Add(5 * time.Minute),
		Outcome:     OutcomeSuccess,
	}

	path, err := SaveRecord(dir, &record)
	require.NoError(t, err)

	assert.Contains(t, path, "iteration-iter-001.json")
	assert.FileExists(t, path)

	// Verify content
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var loaded IterationRecord
	err = json.Unmarshal(data, &loaded)
	require.NoError(t, err)

	assert.Equal(t, record.IterationID, loaded.IterationID)
	assert.Equal(t, record.TaskID, loaded.TaskID)
	assert.Equal(t, record.Outcome, loaded.Outcome)
}

func TestSaveRecord_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	logsDir := filepath.Join(dir, "logs", "subdir")

	record := IterationRecord{
		IterationID: "iter-002",
		TaskID:      "task-456",
		Outcome:     OutcomeFailed,
	}

	path, err := SaveRecord(logsDir, &record)
	require.NoError(t, err)

	assert.FileExists(t, path)
}

func TestSaveRecord_NilRecord(t *testing.T) {
	dir := t.TempDir()

	_, err := SaveRecord(dir, nil)
	assert.Error(t, err)
}

func TestLoadRecord(t *testing.T) {
	dir := t.TempDir()

	// Save a record first
	original := &IterationRecord{
		IterationID: "iter-003",
		TaskID:      "task-789",
		StartTime:   time.Date(2026, 1, 16, 14, 0, 0, 0, time.UTC),
		EndTime:     time.Date(2026, 1, 16, 14, 5, 0, 0, time.UTC),
		Outcome:     OutcomeSuccess,
		FilesChanged: []string{"file1.go", "file2.go"},
	}

	path, err := SaveRecord(dir, original)
	require.NoError(t, err)

	// Load it back
	loaded, err := LoadRecord(path)
	require.NoError(t, err)

	assert.Equal(t, original.IterationID, loaded.IterationID)
	assert.Equal(t, original.TaskID, loaded.TaskID)
	assert.Equal(t, original.Outcome, loaded.Outcome)
	assert.Equal(t, original.FilesChanged, loaded.FilesChanged)
}

func TestLoadRecord_NotFound(t *testing.T) {
	_, err := LoadRecord("/nonexistent/path/iteration.json")
	assert.Error(t, err)
}

func TestLoadRecord_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "invalid.json")
	err := os.WriteFile(path, []byte("not valid json"), 0644)
	require.NoError(t, err)

	_, err = LoadRecord(path)
	assert.Error(t, err)
}

func TestGenerateIterationID(t *testing.T) {
	id1 := GenerateIterationID()
	id2 := GenerateIterationID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
}

func TestIterationRecord_AllPassed(t *testing.T) {
	tests := []struct {
		name    string
		outputs []VerificationOutput
		want    bool
	}{
		{
			name:    "empty outputs",
			outputs: nil,
			want:    true,
		},
		{
			name: "all passed",
			outputs: []VerificationOutput{
				{Passed: true},
				{Passed: true},
			},
			want: true,
		},
		{
			name: "one failed",
			outputs: []VerificationOutput{
				{Passed: true},
				{Passed: false},
			},
			want: false,
		},
		{
			name: "all failed",
			outputs: []VerificationOutput{
				{Passed: false},
				{Passed: false},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := IterationRecord{
				VerificationOutputs: tt.outputs,
			}
			assert.Equal(t, tt.want, record.AllPassed())
		})
	}
}
