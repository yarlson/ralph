package verifier

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTrimOutputDefaults(t *testing.T) {
	opts := TrimOptions{}
	assert.Equal(t, 0, opts.MaxLines)
	assert.Equal(t, 0, opts.MaxBytes)
}

func TestTrimOutput_EmptyInput(t *testing.T) {
	result := TrimOutput("", TrimOptions{})
	assert.Equal(t, "", result)
}

func TestTrimOutput_NoLimits(t *testing.T) {
	input := "line1\nline2\nline3"
	result := TrimOutput(input, TrimOptions{})
	assert.Equal(t, input, result)
}

func TestTrimOutput_UnderLimits(t *testing.T) {
	input := "line1\nline2\nline3"
	result := TrimOutput(input, TrimOptions{MaxLines: 10, MaxBytes: 1000})
	assert.Equal(t, input, result)
}

func TestTrimOutput_MaxLinesPreservesTail(t *testing.T) {
	input := "line1\nline2\nline3\nline4\nline5"
	result := TrimOutput(input, TrimOptions{MaxLines: 3})

	// Should preserve the last 3 lines (tail), which is where errors typically appear
	assert.Contains(t, result, "line3")
	assert.Contains(t, result, "line4")
	assert.Contains(t, result, "line5")
	assert.NotContains(t, result, "line1")
	assert.NotContains(t, result, "line2")
}

func TestTrimOutput_MaxLinesAddsTruncationMarker(t *testing.T) {
	input := "line1\nline2\nline3\nline4\nline5"
	result := TrimOutput(input, TrimOptions{MaxLines: 3})

	// Should have truncation marker at the beginning
	assert.True(t, strings.HasPrefix(result, TruncationMarker), "result should start with truncation marker")
}

func TestTrimOutput_MaxBytesPreservesTail(t *testing.T) {
	input := strings.Repeat("A", 100) + "\n" + strings.Repeat("B", 100) + "\n" + strings.Repeat("C", 100)
	result := TrimOutput(input, TrimOptions{MaxBytes: 150})

	// Should preserve the tail (end) of the output
	assert.True(t, strings.HasSuffix(result, strings.Repeat("C", 100)), "result should end with C's")
	assert.Contains(t, result, TruncationMarker)
}

func TestTrimOutput_MaxBytesRespectsBoundary(t *testing.T) {
	input := "12345678901234567890"
	result := TrimOutput(input, TrimOptions{MaxBytes: 10})

	// Truncation marker + remaining content should be around MaxBytes
	// (we allow some slack for the marker)
	assert.LessOrEqual(t, len(result), 10+len(TruncationMarker)+1)
}

func TestTrimOutput_MaxLinesExactLimit(t *testing.T) {
	input := "line1\nline2\nline3"
	result := TrimOutput(input, TrimOptions{MaxLines: 3})

	// Exactly at limit, no truncation needed
	assert.Equal(t, input, result)
	assert.NotContains(t, result, TruncationMarker)
}

func TestTrimOutput_MaxBytesExactLimit(t *testing.T) {
	input := "12345"
	result := TrimOutput(input, TrimOptions{MaxBytes: 5})

	// Exactly at limit, no truncation needed
	assert.Equal(t, input, result)
	assert.NotContains(t, result, TruncationMarker)
}

func TestTrimOutput_BothLimitsLinesMoreRestrictive(t *testing.T) {
	// Each line is 10 bytes + newline
	input := "0123456789\n0123456789\n0123456789\n0123456789\n0123456789"
	result := TrimOutput(input, TrimOptions{MaxLines: 2, MaxBytes: 1000})

	// Lines limit (2) is more restrictive
	lines := strings.Split(strings.TrimPrefix(result, TruncationMarker+"\n"), "\n")
	// Should have 2 lines of content
	contentLines := 0
	for _, line := range lines {
		if line != "" {
			contentLines++
		}
	}
	assert.LessOrEqual(t, contentLines, 2)
}

func TestTrimOutput_BothLimitsBytesMoreRestrictive(t *testing.T) {
	// Each line is 10 bytes + newline
	input := "0123456789\n0123456789\n0123456789\n0123456789\n0123456789"
	result := TrimOutput(input, TrimOptions{MaxLines: 100, MaxBytes: 25})

	// Bytes limit is more restrictive
	assert.LessOrEqual(t, len(result), 25+len(TruncationMarker)+1)
}

func TestTrimOutput_SingleLine(t *testing.T) {
	input := "single line with no newline"
	result := TrimOutput(input, TrimOptions{MaxLines: 1})

	assert.Equal(t, input, result)
}

func TestTrimOutput_SingleLineTruncatedByBytes(t *testing.T) {
	// A longer input to ensure truncation happens
	input := "this is a very long single line with lots of content that should definitely get truncated"
	// Need enough bytes for marker + some content but less than input
	result := TrimOutput(input, TrimOptions{MaxBytes: 50})

	assert.Contains(t, result, TruncationMarker)
	// Should preserve the tail
	assert.True(t, strings.HasSuffix(result, "truncated"))
}

func TestTrimOutput_TrailingNewline(t *testing.T) {
	input := "line1\nline2\nline3\n"
	result := TrimOutput(input, TrimOptions{MaxLines: 2})

	// Should handle trailing newline gracefully
	assert.Contains(t, result, "line3")
}

func TestTrimOutput_EmptyLines(t *testing.T) {
	input := "line1\n\nline2\n\nline3"
	result := TrimOutput(input, TrimOptions{MaxLines: 3})

	// Empty lines count as lines
	assert.Contains(t, result, TruncationMarker)
}

func TestTrimOutput_OnlyNewlines(t *testing.T) {
	input := "\n\n\n\n\n"
	result := TrimOutput(input, TrimOptions{MaxLines: 2})

	// Should handle all-newline input
	assert.Contains(t, result, TruncationMarker)
}

func TestTrimOutput_LargeInput(t *testing.T) {
	// Simulate a large test output
	var builder strings.Builder
	for i := 0; i < 1000; i++ {
		builder.WriteString("error: test failed at line ")
		builder.WriteString(string(rune('0' + i%10)))
		builder.WriteString("\n")
	}
	input := builder.String()

	result := TrimOutput(input, TrimOptions{MaxLines: 50, MaxBytes: 2000})

	// Should be within limits
	lines := strings.Split(result, "\n")
	assert.LessOrEqual(t, len(lines), 52) // 50 + marker line + possible empty
	assert.LessOrEqual(t, len(result), 2000+len(TruncationMarker)+1)
}

func TestTrimOutputForFeedback(t *testing.T) {
	results := []VerificationResult{
		{
			Passed:  true,
			Command: []string{"go", "build", "./..."},
			Output:  "success",
		},
		{
			Passed:  false,
			Command: []string{"go", "test", "./..."},
			Output:  "line1\nline2\nline3\nFAIL: TestFoo\nline4\nline5",
		},
	}

	feedback := TrimOutputForFeedback(results, TrimOptions{MaxLines: 3})

	// Should only include failed results
	assert.NotContains(t, feedback, "go build")
	assert.Contains(t, feedback, "go test")

	// Should have truncated output
	assert.Contains(t, feedback, TruncationMarker)
	assert.Contains(t, feedback, "line5")
}

func TestTrimOutputForFeedback_AllPassed(t *testing.T) {
	results := []VerificationResult{
		{Passed: true, Command: []string{"go", "build"}, Output: "ok"},
		{Passed: true, Command: []string{"go", "test"}, Output: "ok"},
	}

	feedback := TrimOutputForFeedback(results, TrimOptions{MaxLines: 10})

	// No failures, so empty feedback
	assert.Equal(t, "", feedback)
}

func TestTrimOutputForFeedback_MultipleFailures(t *testing.T) {
	results := []VerificationResult{
		{Passed: false, Command: []string{"go", "build"}, Output: "build error"},
		{Passed: false, Command: []string{"go", "test"}, Output: "test error"},
	}

	feedback := TrimOutputForFeedback(results, TrimOptions{MaxLines: 10})

	// Should include both failures
	assert.Contains(t, feedback, "go build")
	assert.Contains(t, feedback, "build error")
	assert.Contains(t, feedback, "go test")
	assert.Contains(t, feedback, "test error")
}

func TestTrimOptions_Validate(t *testing.T) {
	tests := []struct {
		name    string
		opts    TrimOptions
		wantErr bool
	}{
		{
			name:    "zero values valid",
			opts:    TrimOptions{},
			wantErr: false,
		},
		{
			name:    "positive values valid",
			opts:    TrimOptions{MaxLines: 100, MaxBytes: 10000},
			wantErr: false,
		},
		{
			name:    "negative MaxLines invalid",
			opts:    TrimOptions{MaxLines: -1},
			wantErr: true,
		},
		{
			name:    "negative MaxBytes invalid",
			opts:    TrimOptions{MaxBytes: -1},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDefaultTrimOptions(t *testing.T) {
	opts := DefaultTrimOptions()

	assert.Greater(t, opts.MaxLines, 0)
	assert.Greater(t, opts.MaxBytes, 0)
	assert.NoError(t, opts.Validate())
}
