package internal

import (
	"strings"
)

// ProgressBar returns an ASCII progress bar string for the given percentage.
// The width parameter specifies the inner width of the bar (excluding brackets).
// Percentage values are clamped to 0-100.
//
// Example: ProgressBar(50, 20) returns "[==========          ]"
func ProgressBar(percent, width int) string {
	// Clamp percent to 0-100
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	// Calculate filled portion
	filled := (percent * width) / 100

	// Build the bar
	var sb strings.Builder
	sb.WriteString("[")
	sb.WriteString(strings.Repeat("=", filled))
	sb.WriteString(strings.Repeat(" ", width-filled))
	sb.WriteString("]")

	return sb.String()
}
