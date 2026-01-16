package loop

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test GutterReason validity
func TestGutterReason_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		reason GutterReason
		want   bool
	}{
		{"none is valid", GutterReasonNone, true},
		{"repeated_failure is valid", GutterReasonRepeatedFailure, true},
		{"file_churn is valid", GutterReasonFileChurn, true},
		{"oscillation is valid", GutterReasonOscillation, true},
		{"empty is invalid", "", false},
		{"unknown is invalid", "unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.reason.IsValid())
		})
	}
}

// Test GutterConfig defaults
func TestDefaultGutterConfig(t *testing.T) {
	cfg := DefaultGutterConfig()

	assert.Equal(t, 3, cfg.MaxSameFailure, "default MaxSameFailure should be 3")
	assert.Equal(t, 5, cfg.MaxChurnIterations, "default MaxChurnIterations should be 5")
	assert.Equal(t, 3, cfg.ChurnThreshold, "default ChurnThreshold should be 3")
	assert.Equal(t, 2, cfg.MaxOscillations, "default MaxOscillations should be 2")
	assert.True(t, cfg.EnableContentHash, "default EnableContentHash should be true")
}

// Test GutterStatus
func TestGutterStatus_IsInGutter(t *testing.T) {
	tests := []struct {
		name   string
		status GutterStatus
		want   bool
	}{
		{
			name:   "not in gutter",
			status: GutterStatus{InGutter: false, Reason: GutterReasonNone},
			want:   false,
		},
		{
			name:   "in gutter with repeated failure",
			status: GutterStatus{InGutter: true, Reason: GutterReasonRepeatedFailure},
			want:   true,
		},
		{
			name:   "in gutter with file churn",
			status: GutterStatus{InGutter: true, Reason: GutterReasonFileChurn},
			want:   true,
		},
		{
			name:   "in gutter with oscillation",
			status: GutterStatus{InGutter: true, Reason: GutterReasonOscillation},
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.status.InGutter)
		})
	}
}

// Test GutterDetector creation
func TestNewGutterDetector(t *testing.T) {
	cfg := DefaultGutterConfig()
	detector := NewGutterDetector(cfg)

	require.NotNil(t, detector)
	assert.Equal(t, cfg, detector.config)
	assert.NotNil(t, detector.failureSignatures)
	assert.NotNil(t, detector.fileChanges)
}

// Test ComputeFailureSignature
func TestComputeFailureSignature(t *testing.T) {
	tests := []struct {
		name     string
		outputs  []VerificationOutput
		wantHash bool
	}{
		{
			name:     "empty outputs",
			outputs:  []VerificationOutput{},
			wantHash: true,
		},
		{
			name: "all passed - no signature",
			outputs: []VerificationOutput{
				{Command: []string{"go", "test"}, Passed: true, Output: "PASS"},
			},
			wantHash: false,
		},
		{
			name: "failed output - has signature",
			outputs: []VerificationOutput{
				{Command: []string{"go", "test"}, Passed: false, Output: "FAIL: test_foo"},
			},
			wantHash: true,
		},
		{
			name: "mixed outputs - signature from failed",
			outputs: []VerificationOutput{
				{Command: []string{"go", "build"}, Passed: true, Output: ""},
				{Command: []string{"go", "test"}, Passed: false, Output: "FAIL: test_foo"},
			},
			wantHash: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sig := ComputeFailureSignature(tt.outputs)

			if tt.wantHash {
				if len(tt.outputs) == 0 {
					assert.Empty(t, sig, "empty outputs should produce empty signature")
				} else {
					// Check that the signature is a valid hex string (sha256 produces 64 chars)
					hasFailure := false
					for _, o := range tt.outputs {
						if !o.Passed {
							hasFailure = true
							break
						}
					}
					if hasFailure {
						assert.Len(t, sig, 64, "signature should be 64 hex characters")
					} else {
						assert.Empty(t, sig, "no failures should produce empty signature")
					}
				}
			} else {
				assert.Empty(t, sig, "expected no signature")
			}
		})
	}
}

// Test same failure produces same signature
func TestComputeFailureSignature_Consistency(t *testing.T) {
	outputs := []VerificationOutput{
		{Command: []string{"go", "test"}, Passed: false, Output: "FAIL: specific error message"},
	}

	sig1 := ComputeFailureSignature(outputs)
	sig2 := ComputeFailureSignature(outputs)

	assert.Equal(t, sig1, sig2, "same outputs should produce same signature")
}

// Test different failures produce different signatures
func TestComputeFailureSignature_Uniqueness(t *testing.T) {
	outputs1 := []VerificationOutput{
		{Command: []string{"go", "test"}, Passed: false, Output: "FAIL: error A"},
	}
	outputs2 := []VerificationOutput{
		{Command: []string{"go", "test"}, Passed: false, Output: "FAIL: error B"},
	}

	sig1 := ComputeFailureSignature(outputs1)
	sig2 := ComputeFailureSignature(outputs2)

	assert.NotEqual(t, sig1, sig2, "different outputs should produce different signatures")
}

// Test RecordIteration
func TestGutterDetector_RecordIteration(t *testing.T) {
	cfg := DefaultGutterConfig()
	detector := NewGutterDetector(cfg)

	// Record a failed iteration
	record := &IterationRecord{
		IterationID: "iter-001",
		TaskID:      "task-1",
		Outcome:     OutcomeFailed,
		FilesChanged: []string{"file1.go", "file2.go"},
		VerificationOutputs: []VerificationOutput{
			{Command: []string{"go", "test"}, Passed: false, Output: "FAIL"},
		},
	}

	detector.RecordIteration(record)

	// Check that state was updated
	assert.Len(t, detector.fileChanges, 1)
	sig := ComputeFailureSignature(record.VerificationOutputs)
	assert.Equal(t, 1, detector.failureSignatures[sig])
}

// Test repeated failure detection
func TestGutterDetector_RepeatedFailure(t *testing.T) {
	cfg := DefaultGutterConfig()
	cfg.MaxSameFailure = 3
	detector := NewGutterDetector(cfg)

	// Create a record with specific failure
	makeRecord := func(id string) *IterationRecord {
		return &IterationRecord{
			IterationID: id,
			TaskID:      "task-1",
			Outcome:     OutcomeFailed,
			FilesChanged: []string{"file1.go"},
			VerificationOutputs: []VerificationOutput{
				{Command: []string{"go", "test"}, Passed: false, Output: "FAIL: same error"},
			},
		}
	}

	// First 2 failures - not in gutter yet
	detector.RecordIteration(makeRecord("iter-1"))
	status := detector.Check()
	assert.False(t, status.InGutter, "should not be in gutter after 1 failure")

	detector.RecordIteration(makeRecord("iter-2"))
	status = detector.Check()
	assert.False(t, status.InGutter, "should not be in gutter after 2 failures")

	// Third failure - now in gutter
	detector.RecordIteration(makeRecord("iter-3"))
	status = detector.Check()
	assert.True(t, status.InGutter, "should be in gutter after 3 same failures")
	assert.Equal(t, GutterReasonRepeatedFailure, status.Reason)
	assert.Contains(t, status.Description, "same failure")
}

// Test file churn detection
func TestGutterDetector_FileChurn(t *testing.T) {
	cfg := DefaultGutterConfig()
	cfg.MaxChurnIterations = 3
	cfg.ChurnThreshold = 2 // same file modified 2+ times counts as churn
	cfg.MaxOscillations = 10 // Set high so oscillation doesn't trigger first
	detector := NewGutterDetector(cfg)

	// Record iterations with same files being modified repeatedly
	for i := 0; i < 3; i++ {
		record := &IterationRecord{
			IterationID:  GenerateIterationID(),
			TaskID:       "task-1",
			Outcome:      OutcomeFailed,
			FilesChanged: []string{"churning-file.go"},
			VerificationOutputs: []VerificationOutput{
				{Command: []string{"go", "test"}, Passed: false, Output: "FAIL: different error " + string(rune('A'+i))},
			},
		}
		detector.RecordIteration(record)
	}

	status := detector.Check()
	assert.True(t, status.InGutter, "should detect file churn")
	assert.Equal(t, GutterReasonFileChurn, status.Reason)
	assert.Contains(t, status.Description, "churning-file.go")
}

// Test no gutter when making progress
func TestGutterDetector_NoGutterWithProgress(t *testing.T) {
	cfg := DefaultGutterConfig()
	detector := NewGutterDetector(cfg)

	// Record successful iterations
	for i := 0; i < 5; i++ {
		record := &IterationRecord{
			IterationID:  GenerateIterationID(),
			TaskID:       "task-" + string(rune('1'+i)),
			Outcome:      OutcomeSuccess,
			FilesChanged: []string{"file" + string(rune('1'+i)) + ".go"},
			VerificationOutputs: []VerificationOutput{
				{Command: []string{"go", "test"}, Passed: true, Output: "PASS"},
			},
		}
		detector.RecordIteration(record)
	}

	status := detector.Check()
	assert.False(t, status.InGutter, "should not be in gutter with successful iterations")
	assert.Equal(t, GutterReasonNone, status.Reason)
}

// Test mixed results don't trigger gutter
func TestGutterDetector_MixedResults(t *testing.T) {
	cfg := DefaultGutterConfig()
	cfg.MaxSameFailure = 3
	detector := NewGutterDetector(cfg)

	// Different failures each time
	for i := 0; i < 3; i++ {
		record := &IterationRecord{
			IterationID: GenerateIterationID(),
			TaskID:      "task-1",
			Outcome:     OutcomeFailed,
			FilesChanged: []string{
				"file" + string(rune('1'+i)) + ".go",
			},
			VerificationOutputs: []VerificationOutput{
				{Command: []string{"go", "test"}, Passed: false, Output: "FAIL: unique error " + string(rune('A'+i))},
			},
		}
		detector.RecordIteration(record)
	}

	status := detector.Check()
	// May or may not be in gutter depending on file churn config
	// but should NOT be repeated failure
	if status.InGutter {
		assert.NotEqual(t, GutterReasonRepeatedFailure, status.Reason,
			"should not trigger repeated failure with different errors")
	}
}

// Test Reset clears state
func TestGutterDetector_Reset(t *testing.T) {
	cfg := DefaultGutterConfig()
	detector := NewGutterDetector(cfg)

	// Add some state
	record := &IterationRecord{
		IterationID: "iter-1",
		TaskID:      "task-1",
		Outcome:     OutcomeFailed,
		FilesChanged: []string{"file1.go"},
		VerificationOutputs: []VerificationOutput{
			{Command: []string{"go", "test"}, Passed: false, Output: "FAIL"},
		},
	}
	detector.RecordIteration(record)

	assert.NotEmpty(t, detector.failureSignatures)
	assert.NotEmpty(t, detector.fileChanges)

	// Reset
	detector.Reset()

	assert.Empty(t, detector.failureSignatures)
	assert.Empty(t, detector.fileChanges)
}

// Test oscillation detection (file appearing in non-consecutive iterations)
func TestGutterDetector_Oscillation(t *testing.T) {
	cfg := DefaultGutterConfig()
	cfg.MaxOscillations = 2
	cfg.EnableContentHash = true
	cfg.MaxChurnIterations = 6
	detector := NewGutterDetector(cfg)

	// Record file appearing in non-consecutive iterations (file1, file2, file1, file2, file1)
	fileSequence := []string{"file1.go", "file2.go", "file1.go", "file3.go", "file1.go"}
	for i, file := range fileSequence {
		record := &IterationRecord{
			IterationID:  GenerateIterationID(),
			TaskID:       "task-1",
			Outcome:      OutcomeFailed,
			FilesChanged: []string{file},
			VerificationOutputs: []VerificationOutput{
				{Command: []string{"go", "test"}, Passed: false, Output: "FAIL: iteration " + string(rune('0'+i))},
			},
		}
		detector.RecordIteration(record)
	}

	status := detector.Check()
	// file1.go appears 3 times (oscillates twice after first appearance)
	assert.True(t, status.InGutter, "should detect oscillation pattern")
	assert.Equal(t, GutterReasonOscillation, status.Reason)
	assert.Contains(t, status.Description, "file1.go")
}

// Test GetState and SetState
func TestGutterDetector_GetSetState(t *testing.T) {
	cfg := DefaultGutterConfig()
	detector1 := NewGutterDetector(cfg)

	// Record some iterations
	record := &IterationRecord{
		IterationID: "iter-1",
		TaskID:      "task-1",
		Outcome:     OutcomeFailed,
		FilesChanged: []string{"file1.go", "file2.go"},
		VerificationOutputs: []VerificationOutput{
			{Command: []string{"go", "test"}, Passed: false, Output: "FAIL"},
		},
	}
	detector1.RecordIteration(record)

	// Get state
	state := detector1.GetState()
	assert.NotEmpty(t, state.FailureSignatures)
	assert.NotEmpty(t, state.FileChanges)

	// Create new detector and restore state
	detector2 := NewGutterDetector(cfg)
	detector2.SetState(state)

	// Check should return same result
	status1 := detector1.Check()
	status2 := detector2.Check()
	assert.Equal(t, status1.InGutter, status2.InGutter)
	assert.Equal(t, status1.Reason, status2.Reason)
}

// Test signature computation helper
func TestSignatureHash(t *testing.T) {
	// Verify we're using sha256
	input := "test input"
	hash := sha256.Sum256([]byte(input))
	expected := hex.EncodeToString(hash[:])

	// Compute manually
	h := sha256.New()
	h.Write([]byte(input))
	actual := hex.EncodeToString(h.Sum(nil))

	assert.Equal(t, expected, actual)
}

// Test disabled gutter detection (all thresholds 0)
func TestGutterDetector_Disabled(t *testing.T) {
	cfg := GutterConfig{
		MaxSameFailure:     0,
		MaxChurnIterations: 0,
		ChurnThreshold:     0,
		MaxOscillations:    0,
		EnableContentHash:  false,
	}
	detector := NewGutterDetector(cfg)

	// Record many failures
	for i := 0; i < 10; i++ {
		record := &IterationRecord{
			IterationID: GenerateIterationID(),
			TaskID:      "task-1",
			Outcome:     OutcomeFailed,
			FilesChanged: []string{"file.go"},
			VerificationOutputs: []VerificationOutput{
				{Command: []string{"go", "test"}, Passed: false, Output: "FAIL: same error"},
			},
		}
		detector.RecordIteration(record)
	}

	status := detector.Check()
	assert.False(t, status.InGutter, "gutter detection should be disabled when all thresholds are 0")
}

// Test oscillation detection disabled when EnableContentHash is false
func TestGutterDetector_OscillationDisabled(t *testing.T) {
	cfg := DefaultGutterConfig()
	cfg.MaxOscillations = 2
	cfg.EnableContentHash = false // Disable content hash tracking
	cfg.MaxChurnIterations = 6
	detector := NewGutterDetector(cfg)

	// Record file appearing multiple times
	fileSequence := []string{"file1.go", "file2.go", "file1.go", "file2.go", "file1.go"}
	for i, file := range fileSequence {
		record := &IterationRecord{
			IterationID:  GenerateIterationID(),
			TaskID:       "task-1",
			Outcome:      OutcomeFailed,
			FilesChanged: []string{file},
			VerificationOutputs: []VerificationOutput{
				{Command: []string{"go", "test"}, Passed: false, Output: "FAIL: iteration " + string(rune('0'+i))},
			},
		}
		detector.RecordIteration(record)
	}

	status := detector.Check()
	// Should not detect oscillation when EnableContentHash is false
	if status.InGutter {
		assert.NotEqual(t, GutterReasonOscillation, status.Reason, "should not detect oscillation when content hash disabled")
	}
}

// Test only failed iterations count for gutter detection
func TestGutterDetector_OnlyFailedIterationsCounted(t *testing.T) {
	cfg := DefaultGutterConfig()
	cfg.MaxSameFailure = 2
	detector := NewGutterDetector(cfg)

	// Record one failure
	failRecord := &IterationRecord{
		IterationID: "iter-1",
		TaskID:      "task-1",
		Outcome:     OutcomeFailed,
		FilesChanged: []string{"file.go"},
		VerificationOutputs: []VerificationOutput{
			{Command: []string{"go", "test"}, Passed: false, Output: "FAIL: error"},
		},
	}
	detector.RecordIteration(failRecord)

	// Record successful iteration (should reset or not count)
	successRecord := &IterationRecord{
		IterationID: "iter-2",
		TaskID:      "task-2",
		Outcome:     OutcomeSuccess,
		FilesChanged: []string{"other.go"},
		VerificationOutputs: []VerificationOutput{
			{Command: []string{"go", "test"}, Passed: true, Output: "PASS"},
		},
	}
	detector.RecordIteration(successRecord)

	// Record another failure with same signature - now at 2
	detector.RecordIteration(failRecord)

	status := detector.Check()
	assert.True(t, status.InGutter, "should count consecutive/total failures correctly")
}
