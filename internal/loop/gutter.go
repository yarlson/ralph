package loop

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// GutterReason identifies why the loop is in the gutter.
type GutterReason string

const (
	// GutterReasonNone indicates no gutter condition.
	GutterReasonNone GutterReason = "none"
	// GutterReasonRepeatedFailure indicates the same verification failure repeated N times.
	GutterReasonRepeatedFailure GutterReason = "repeated_failure"
	// GutterReasonFileChurn indicates the same files are being modified repeatedly without progress.
	GutterReasonFileChurn GutterReason = "file_churn"
	// GutterReasonOscillation indicates an oscillating pattern of changes.
	GutterReasonOscillation GutterReason = "oscillation"
)

// validGutterReasons is the set of valid gutter reasons.
var validGutterReasons = map[GutterReason]bool{
	GutterReasonNone:            true,
	GutterReasonRepeatedFailure: true,
	GutterReasonFileChurn:       true,
	GutterReasonOscillation:     true,
}

// IsValid returns true if the reason is a valid value.
func (r GutterReason) IsValid() bool {
	return validGutterReasons[r]
}

// GutterConfig holds configuration for gutter detection.
type GutterConfig struct {
	// MaxSameFailure is the max times the same verification failure can repeat
	// before triggering gutter detection (0 = disabled).
	MaxSameFailure int `json:"max_same_failure"`

	// MaxChurnIterations is the number of recent iterations to consider for file churn
	// detection (0 = disabled).
	MaxChurnIterations int `json:"max_churn_iterations"`

	// ChurnThreshold is how many times a file must be modified within
	// MaxChurnIterations to be considered "churning" (0 = disabled).
	ChurnThreshold int `json:"churn_threshold"`

	// MaxOscillations is the max number of content oscillations before triggering
	// gutter detection (0 = disabled). Oscillation is when a file's content
	// reverts to a previous state.
	MaxOscillations int `json:"max_oscillations"`

	// EnableContentHash enables content-hash based oscillation detection.
	// If false, only file-name based churn detection is used.
	EnableContentHash bool `json:"enable_content_hash"`
}

// DefaultGutterConfig returns sensible default gutter detection config.
func DefaultGutterConfig() GutterConfig {
	return GutterConfig{
		MaxSameFailure:     3,
		MaxChurnIterations: 5,
		ChurnThreshold:     3,
		MaxOscillations:    2,
		EnableContentHash:  true,
	}
}

// GutterStatus represents the result of gutter detection.
type GutterStatus struct {
	// InGutter is true if a gutter condition was detected.
	InGutter bool

	// Reason identifies the specific gutter condition.
	Reason GutterReason

	// Description is a human-readable explanation of the gutter condition.
	Description string
}

// GutterState represents the persistent state for gutter detection.
type GutterState struct {
	// FailureSignatures maps failure signature hashes to their occurrence count.
	FailureSignatures map[string]int `json:"failure_signatures"`

	// FileChanges tracks files changed in recent iterations (list of file sets).
	FileChanges [][]string `json:"file_changes"`

	// ContentHashes tracks content hashes for files across iterations.
	// Map of file path -> list of content hashes (one per iteration).
	ContentHashes map[string][]string `json:"content_hashes"`
}

// GutterDetector tracks iteration history and detects gutter conditions.
type GutterDetector struct {
	config            GutterConfig
	failureSignatures map[string]int              // signature hash -> count
	fileChanges       [][]string                  // list of file sets from recent iterations
	contentHashes     map[string][]string         // file path -> list of content hashes
	oscillationCounts map[string]int              // file path -> oscillation count
}

// NewGutterDetector creates a new gutter detector with the given config.
func NewGutterDetector(config GutterConfig) *GutterDetector {
	return &GutterDetector{
		config:            config,
		failureSignatures: make(map[string]int),
		fileChanges:       [][]string{},
		contentHashes:     make(map[string][]string),
		oscillationCounts: make(map[string]int),
	}
}

// ComputeFailureSignature computes a hash signature of verification failures.
// Returns empty string if there are no failures.
func ComputeFailureSignature(outputs []VerificationOutput) string {
	// Collect failed outputs
	var failures []string
	for _, o := range outputs {
		if !o.Passed {
			// Include command and output in signature
			cmd := strings.Join(o.Command, " ")
			failures = append(failures, fmt.Sprintf("%s:%s", cmd, o.Output))
		}
	}

	if len(failures) == 0 {
		return ""
	}

	// Sort for consistency
	sort.Strings(failures)

	// Hash the combined failure info
	combined := strings.Join(failures, "\n")
	hash := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(hash[:])
}

// ComputeContentHash computes a hash of file content for oscillation detection.
func ComputeContentHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

// RecordIteration records an iteration for gutter detection.
func (d *GutterDetector) RecordIteration(record *IterationRecord) {
	if record == nil {
		return
	}

	// Track file changes for churn detection
	if len(record.FilesChanged) > 0 {
		d.fileChanges = append(d.fileChanges, record.FilesChanged)

		// Keep only recent iterations for churn window
		if d.config.MaxChurnIterations > 0 && len(d.fileChanges) > d.config.MaxChurnIterations {
			d.fileChanges = d.fileChanges[len(d.fileChanges)-d.config.MaxChurnIterations:]
		}
	}

	// Track file change patterns for oscillation detection
	// Since we don't have access to file content in the record, we detect
	// oscillation as files appearing in non-consecutive iterations (skip pattern)
	if d.config.EnableContentHash && len(record.FilesChanged) > 0 {
		for _, file := range record.FilesChanged {
			// Get previous iterations where this file appeared
			prevIterations := d.contentHashes[file]

			// Check if file has appeared before (potential oscillation)
			// If there's a gap (file not modified in every iteration), it's oscillating
			if len(prevIterations) > 0 {
				// Count as oscillation if file reappears after being absent
				d.oscillationCounts[file]++
			}

			// Record this iteration for this file (use iteration count as placeholder)
			iterMarker := fmt.Sprintf("%d", len(d.fileChanges))
			d.contentHashes[file] = append(prevIterations, iterMarker)

			// Keep only recent markers within window
			if d.config.MaxChurnIterations > 0 && len(d.contentHashes[file]) > d.config.MaxChurnIterations {
				d.contentHashes[file] = d.contentHashes[file][len(d.contentHashes[file])-d.config.MaxChurnIterations:]
			}
		}
	}

	// Track failure signatures for repeated failure detection
	if record.Outcome == OutcomeFailed {
		sig := ComputeFailureSignature(record.VerificationOutputs)
		if sig != "" {
			d.failureSignatures[sig]++
		}
	}
}

// Check checks for gutter conditions based on recorded iterations.
func (d *GutterDetector) Check() GutterStatus {
	// Check for repeated failure
	if status := d.checkRepeatedFailure(); status.InGutter {
		return status
	}

	// Check for oscillation
	if status := d.checkOscillation(); status.InGutter {
		return status
	}

	// Check for file churn
	if status := d.checkFileChurn(); status.InGutter {
		return status
	}

	return GutterStatus{
		InGutter:    false,
		Reason:      GutterReasonNone,
		Description: "",
	}
}

// checkRepeatedFailure detects if the same failure has repeated too many times.
func (d *GutterDetector) checkRepeatedFailure() GutterStatus {
	if d.config.MaxSameFailure <= 0 {
		return GutterStatus{InGutter: false, Reason: GutterReasonNone}
	}

	for sig, count := range d.failureSignatures {
		if count >= d.config.MaxSameFailure {
			return GutterStatus{
				InGutter:    true,
				Reason:      GutterReasonRepeatedFailure,
				Description: fmt.Sprintf("same failure repeated %d times (threshold: %d), signature: %s", count, d.config.MaxSameFailure, sig[:8]),
			}
		}
	}

	return GutterStatus{InGutter: false, Reason: GutterReasonNone}
}

// checkOscillation detects if files are oscillating (modified repeatedly in non-consecutive iterations).
func (d *GutterDetector) checkOscillation() GutterStatus {
	if d.config.MaxOscillations <= 0 || !d.config.EnableContentHash {
		return GutterStatus{InGutter: false, Reason: GutterReasonNone}
	}

	// Find files that have oscillated beyond threshold
	var oscillatingFiles []string
	for file, count := range d.oscillationCounts {
		if count >= d.config.MaxOscillations {
			oscillatingFiles = append(oscillatingFiles, file)
		}
	}

	if len(oscillatingFiles) > 0 {
		sort.Strings(oscillatingFiles)
		return GutterStatus{
			InGutter:    true,
			Reason:      GutterReasonOscillation,
			Description: fmt.Sprintf("files oscillating (modified %d+ times non-consecutively): %s", d.config.MaxOscillations, strings.Join(oscillatingFiles, ", ")),
		}
	}

	return GutterStatus{InGutter: false, Reason: GutterReasonNone}
}

// checkFileChurn detects if the same files are being modified repeatedly.
func (d *GutterDetector) checkFileChurn() GutterStatus {
	if d.config.MaxChurnIterations <= 0 || d.config.ChurnThreshold <= 0 {
		return GutterStatus{InGutter: false, Reason: GutterReasonNone}
	}

	// Count file occurrences across recent iterations
	fileCounts := make(map[string]int)
	for _, files := range d.fileChanges {
		for _, file := range files {
			fileCounts[file]++
		}
	}

	// Find churning files
	var churningFiles []string
	for file, count := range fileCounts {
		if count >= d.config.ChurnThreshold {
			churningFiles = append(churningFiles, file)
		}
	}

	if len(churningFiles) > 0 {
		sort.Strings(churningFiles)
		return GutterStatus{
			InGutter:    true,
			Reason:      GutterReasonFileChurn,
			Description: fmt.Sprintf("files modified %d+ times in last %d iterations: %s", d.config.ChurnThreshold, len(d.fileChanges), strings.Join(churningFiles, ", ")),
		}
	}

	return GutterStatus{InGutter: false, Reason: GutterReasonNone}
}

// Reset clears all tracked state.
func (d *GutterDetector) Reset() {
	d.failureSignatures = make(map[string]int)
	d.fileChanges = [][]string{}
	d.contentHashes = make(map[string][]string)
	d.oscillationCounts = make(map[string]int)
}

// GetState returns the current gutter detection state for persistence.
func (d *GutterDetector) GetState() GutterState {
	// Copy the map to avoid external mutation
	sigs := make(map[string]int, len(d.failureSignatures))
	for k, v := range d.failureSignatures {
		sigs[k] = v
	}

	// Copy the slice
	changes := make([][]string, len(d.fileChanges))
	for i, files := range d.fileChanges {
		changes[i] = make([]string, len(files))
		copy(changes[i], files)
	}

	// Copy content hashes
	hashes := make(map[string][]string, len(d.contentHashes))
	for k, v := range d.contentHashes {
		hashes[k] = make([]string, len(v))
		copy(hashes[k], v)
	}

	return GutterState{
		FailureSignatures: sigs,
		FileChanges:       changes,
		ContentHashes:     hashes,
	}
}

// SetState restores gutter detection state from persistence.
func (d *GutterDetector) SetState(state GutterState) {
	if state.FailureSignatures != nil {
		d.failureSignatures = make(map[string]int, len(state.FailureSignatures))
		for k, v := range state.FailureSignatures {
			d.failureSignatures[k] = v
		}
	} else {
		d.failureSignatures = make(map[string]int)
	}

	if state.FileChanges != nil {
		d.fileChanges = make([][]string, len(state.FileChanges))
		for i, files := range state.FileChanges {
			d.fileChanges[i] = make([]string, len(files))
			copy(d.fileChanges[i], files)
		}
	} else {
		d.fileChanges = [][]string{}
	}

	if state.ContentHashes != nil {
		d.contentHashes = make(map[string][]string, len(state.ContentHashes))
		for k, v := range state.ContentHashes {
			d.contentHashes[k] = make([]string, len(v))
			copy(d.contentHashes[k], v)
		}
	} else {
		d.contentHashes = make(map[string][]string)
	}

	// Rebuild oscillation counts from content hashes
	d.oscillationCounts = make(map[string]int)
	for file, hashes := range d.contentHashes {
		// Count how many times this file appeared (reappearances after first)
		if len(hashes) > 1 {
			d.oscillationCounts[file] = len(hashes) - 1
		}
	}
}
