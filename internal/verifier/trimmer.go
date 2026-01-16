package verifier

import (
	"errors"
	"strings"
)

// TruncationMarker is the marker added when output is truncated.
const TruncationMarker = "... [output truncated]"

// DefaultMaxLines is the default maximum number of lines to keep.
const DefaultMaxLines = 100

// DefaultMaxBytes is the default maximum output size in bytes.
const DefaultMaxBytes = 8192

// TrimOptions configures output trimming behavior.
type TrimOptions struct {
	// MaxLines is the maximum number of lines to keep.
	// If 0, no line limit is applied.
	MaxLines int

	// MaxBytes is the maximum output size in bytes.
	// If 0, no byte limit is applied.
	MaxBytes int
}

// Validate checks that the options are valid.
func (o TrimOptions) Validate() error {
	if o.MaxLines < 0 {
		return errors.New("MaxLines cannot be negative")
	}
	if o.MaxBytes < 0 {
		return errors.New("MaxBytes cannot be negative")
	}
	return nil
}

// DefaultTrimOptions returns sensible default trim options.
func DefaultTrimOptions() TrimOptions {
	return TrimOptions{
		MaxLines: DefaultMaxLines,
		MaxBytes: DefaultMaxBytes,
	}
}

// TrimOutput trims verification output to fit within configured limits.
// It preserves the tail (end) of the output since error messages typically
// appear at the end. If trimmed, a truncation marker is added at the beginning.
func TrimOutput(output string, opts TrimOptions) string {
	if output == "" {
		return ""
	}

	// No limits, return as-is
	if opts.MaxLines <= 0 && opts.MaxBytes <= 0 {
		return output
	}

	result := output

	// Apply line limit first (preserves tail)
	if opts.MaxLines > 0 {
		result = trimToMaxLines(result, opts.MaxLines)
	}

	// Then apply byte limit (preserves tail)
	if opts.MaxBytes > 0 {
		result = trimToMaxBytes(result, opts.MaxBytes)
	}

	return result
}

// trimToMaxLines trims output to at most maxLines, preserving the tail.
func trimToMaxLines(output string, maxLines int) string {
	lines := strings.Split(output, "\n")

	if len(lines) <= maxLines {
		return output
	}

	// Keep the last maxLines lines
	keptLines := lines[len(lines)-maxLines:]
	trimmed := strings.Join(keptLines, "\n")

	return TruncationMarker + "\n" + trimmed
}

// trimToMaxBytes trims output to at most maxBytes, preserving the tail.
func trimToMaxBytes(output string, maxBytes int) string {
	if len(output) <= maxBytes {
		return output
	}

	// Calculate how much content we can keep
	// We need room for the truncation marker at the start
	markerLen := len(TruncationMarker) + 1 // +1 for newline
	contentBytes := maxBytes - markerLen
	if contentBytes <= 0 {
		// If maxBytes is too small even for marker, just return truncated content
		return TruncationMarker
	}

	// Keep the tail (last contentBytes bytes)
	startIdx := max(len(output)-contentBytes, 0)

	// Check if we're already truncated (has marker at start)
	if strings.HasPrefix(output, TruncationMarker) {
		// Already truncated, just trim further
		return TruncationMarker + "\n" + output[startIdx:]
	}

	return TruncationMarker + "\n" + output[startIdx:]
}

// TrimOutputForFeedback formats verification results for inclusion in a retry prompt.
// It only includes failed results and trims their output according to options.
func TrimOutputForFeedback(results []VerificationResult, opts TrimOptions) string {
	var builder strings.Builder

	for _, result := range results {
		if result.Passed {
			continue
		}

		// Add command header
		builder.WriteString("Command: ")
		builder.WriteString(strings.Join(result.Command, " "))
		builder.WriteString("\n")

		// Add trimmed output
		trimmedOutput := TrimOutput(result.Output, opts)
		builder.WriteString("Output:\n")
		builder.WriteString(trimmedOutput)
		builder.WriteString("\n\n")
	}

	return strings.TrimSpace(builder.String())
}
