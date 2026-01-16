package claude

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SessionMode represents the type of Claude session (planner or coder).
type SessionMode string

const (
	// SessionModePlanner is used for planning/thinking sessions.
	SessionModePlanner SessionMode = "planner"
	// SessionModeCoder is used for coding/implementation sessions.
	SessionModeCoder SessionMode = "coder"
)

// SessionState holds persistent session information for Claude Code invocations.
// It tracks separate sessions for planner and coder modes to allow --continue
// to resume the appropriate context.
type SessionState struct {
	// PlannerSessionID is the session ID for planning mode invocations.
	PlannerSessionID string `json:"planner_session_id,omitempty"`

	// CoderSessionID is the session ID for coder mode invocations.
	CoderSessionID string `json:"coder_session_id,omitempty"`

	// UpdatedAt is the timestamp of the last session update.
	UpdatedAt time.Time `json:"updated_at"`
}

// UpdatePlannerSession updates the planner session ID and sets UpdatedAt.
func (s *SessionState) UpdatePlannerSession(sessionID string) {
	s.PlannerSessionID = sessionID
	s.UpdatedAt = time.Now()
}

// UpdateCoderSession updates the coder session ID and sets UpdatedAt.
func (s *SessionState) UpdateCoderSession(sessionID string) {
	s.CoderSessionID = sessionID
	s.UpdatedAt = time.Now()
}

// GetSessionForMode returns the session ID for the specified mode.
func (s *SessionState) GetSessionForMode(mode SessionMode) string {
	switch mode {
	case SessionModePlanner:
		return s.PlannerSessionID
	case SessionModeCoder:
		return s.CoderSessionID
	default:
		return ""
	}
}

// UpdateSessionForMode updates the session ID for the specified mode.
func (s *SessionState) UpdateSessionForMode(mode SessionMode, sessionID string) {
	switch mode {
	case SessionModePlanner:
		s.UpdatePlannerSession(sessionID)
	case SessionModeCoder:
		s.UpdateCoderSession(sessionID)
	}
}

// LoadSession loads session state from the specified file path.
// Returns an empty SessionState if the file doesn't exist.
// Returns an error if the file exists but cannot be read or parsed.
func LoadSession(path string) (*SessionState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// File doesn't exist - return empty state
			return &SessionState{}, nil
		}
		return nil, fmt.Errorf("read session state from %s: %w", path, err)
	}

	var state SessionState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse session state from %s: %w", path, err)
	}

	return &state, nil
}

// SaveSession saves session state to the specified file path.
// Creates parent directories if they don't exist.
func SaveSession(path string, state *SessionState) error {
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create session state directory %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session state: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write session state to %s: %w", path, err)
	}

	return nil
}

// DetectSessionFork detects if a session has forked (new ID returned when --continue was used).
// A fork occurs when:
// - currentID is non-empty (we expected to continue a session)
// - newID is non-empty (we got a new session)
// - currentID != newID (they're different)
func DetectSessionFork(currentID, newID string) bool {
	if currentID == "" || newID == "" {
		return false
	}
	return currentID != newID
}
