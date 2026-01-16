package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionState_EmptyState(t *testing.T) {
	state := &SessionState{}

	assert.Empty(t, state.PlannerSessionID)
	assert.Empty(t, state.CoderSessionID)
	assert.True(t, state.UpdatedAt.IsZero())
}

func TestSessionState_AllFields(t *testing.T) {
	now := time.Now()
	state := &SessionState{
		PlannerSessionID: "planner-123",
		CoderSessionID:   "coder-456",
		UpdatedAt:        now,
	}

	assert.Equal(t, "planner-123", state.PlannerSessionID)
	assert.Equal(t, "coder-456", state.CoderSessionID)
	assert.Equal(t, now, state.UpdatedAt)
}

func TestSessionState_JSONSerialization(t *testing.T) {
	now := time.Date(2026, 1, 16, 10, 30, 0, 0, time.UTC)
	state := &SessionState{
		PlannerSessionID: "planner-abc",
		CoderSessionID:   "coder-xyz",
		UpdatedAt:        now,
	}

	// Marshal to JSON
	data, err := json.Marshal(state)
	require.NoError(t, err)

	// Unmarshal back
	var decoded SessionState
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, state.PlannerSessionID, decoded.PlannerSessionID)
	assert.Equal(t, state.CoderSessionID, decoded.CoderSessionID)
	assert.True(t, state.UpdatedAt.Equal(decoded.UpdatedAt))
}

func TestLoadSession_FileNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "claude-session.json")

	state, err := LoadSession(statePath)
	require.NoError(t, err)
	require.NotNil(t, state)

	// Should return empty state when file doesn't exist
	assert.Empty(t, state.PlannerSessionID)
	assert.Empty(t, state.CoderSessionID)
	assert.True(t, state.UpdatedAt.IsZero())
}

func TestLoadSession_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "claude-session.json")

	// Write a valid session file
	original := &SessionState{
		PlannerSessionID: "planner-saved",
		CoderSessionID:   "coder-saved",
		UpdatedAt:        time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
	}
	data, err := json.Marshal(original)
	require.NoError(t, err)
	err = os.WriteFile(statePath, data, 0644)
	require.NoError(t, err)

	// Load it back
	state, err := LoadSession(statePath)
	require.NoError(t, err)

	assert.Equal(t, "planner-saved", state.PlannerSessionID)
	assert.Equal(t, "coder-saved", state.CoderSessionID)
}

func TestLoadSession_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "claude-session.json")

	// Write invalid JSON
	err := os.WriteFile(statePath, []byte("not valid json"), 0644)
	require.NoError(t, err)

	// Should return error
	_, err = LoadSession(statePath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse session state")
}

func TestSaveSession_NewFile(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "claude-session.json")

	state := &SessionState{
		PlannerSessionID: "new-planner",
		CoderSessionID:   "new-coder",
		UpdatedAt:        time.Now(),
	}

	err := SaveSession(statePath, state)
	require.NoError(t, err)

	// Verify file was created
	_, err = os.Stat(statePath)
	require.NoError(t, err)

	// Load and verify contents
	loaded, err := LoadSession(statePath)
	require.NoError(t, err)
	assert.Equal(t, state.PlannerSessionID, loaded.PlannerSessionID)
	assert.Equal(t, state.CoderSessionID, loaded.CoderSessionID)
}

func TestSaveSession_OverwriteExisting(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "claude-session.json")

	// Write initial state
	initial := &SessionState{PlannerSessionID: "old-planner"}
	err := SaveSession(statePath, initial)
	require.NoError(t, err)

	// Overwrite with new state
	updated := &SessionState{
		PlannerSessionID: "new-planner",
		CoderSessionID:   "new-coder",
	}
	err = SaveSession(statePath, updated)
	require.NoError(t, err)

	// Verify new contents
	loaded, err := LoadSession(statePath)
	require.NoError(t, err)
	assert.Equal(t, "new-planner", loaded.PlannerSessionID)
	assert.Equal(t, "new-coder", loaded.CoderSessionID)
}

func TestSaveSession_CreatesParentDir(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "subdir", "deep", "claude-session.json")

	state := &SessionState{CoderSessionID: "nested-coder"}
	err := SaveSession(statePath, state)
	require.NoError(t, err)

	// Verify file exists in nested directory
	loaded, err := LoadSession(statePath)
	require.NoError(t, err)
	assert.Equal(t, "nested-coder", loaded.CoderSessionID)
}

func TestSessionState_UpdatePlannerSession(t *testing.T) {
	state := &SessionState{
		PlannerSessionID: "old-planner",
		CoderSessionID:   "coder",
	}

	state.UpdatePlannerSession("new-planner")

	assert.Equal(t, "new-planner", state.PlannerSessionID)
	assert.Equal(t, "coder", state.CoderSessionID) // Unchanged
	assert.False(t, state.UpdatedAt.IsZero())
}

func TestSessionState_UpdateCoderSession(t *testing.T) {
	state := &SessionState{
		PlannerSessionID: "planner",
		CoderSessionID:   "old-coder",
	}

	state.UpdateCoderSession("new-coder")

	assert.Equal(t, "planner", state.PlannerSessionID) // Unchanged
	assert.Equal(t, "new-coder", state.CoderSessionID)
	assert.False(t, state.UpdatedAt.IsZero())
}

func TestSessionState_DetectFork(t *testing.T) {
	tests := []struct {
		name         string
		currentID    string
		newID        string
		expectForked bool
	}{
		{
			name:         "same session",
			currentID:    "session-123",
			newID:        "session-123",
			expectForked: false,
		},
		{
			name:         "different session (fork)",
			currentID:    "session-123",
			newID:        "session-456",
			expectForked: true,
		},
		{
			name:         "empty current (new session)",
			currentID:    "",
			newID:        "session-123",
			expectForked: false,
		},
		{
			name:         "empty new session",
			currentID:    "session-123",
			newID:        "",
			expectForked: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			forked := DetectSessionFork(tt.currentID, tt.newID)
			assert.Equal(t, tt.expectForked, forked)
		})
	}
}

func TestSessionState_GetSessionForMode(t *testing.T) {
	state := &SessionState{
		PlannerSessionID: "planner-id",
		CoderSessionID:   "coder-id",
	}

	assert.Equal(t, "planner-id", state.GetSessionForMode(SessionModePlanner))
	assert.Equal(t, "coder-id", state.GetSessionForMode(SessionModeCoder))
	assert.Empty(t, state.GetSessionForMode("unknown"))
}

func TestSessionState_UpdateSessionForMode(t *testing.T) {
	state := &SessionState{}

	state.UpdateSessionForMode(SessionModePlanner, "planner-new")
	assert.Equal(t, "planner-new", state.PlannerSessionID)

	state.UpdateSessionForMode(SessionModeCoder, "coder-new")
	assert.Equal(t, "coder-new", state.CoderSessionID)

	// Unknown mode should be no-op
	state.UpdateSessionForMode("unknown", "ignored")
	assert.Equal(t, "planner-new", state.PlannerSessionID)
	assert.Equal(t, "coder-new", state.CoderSessionID)
}
