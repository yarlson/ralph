// Package verifier provides verification command execution for the Ralph harness.
package verifier

import (
	"context"
	"time"
)

// VerificationResult contains the outcome of running a single verification command.
type VerificationResult struct {
	// Passed indicates whether the command exited successfully (exit code 0).
	Passed bool `json:"passed"`

	// Command is the command that was executed (e.g., ["go", "test", "./..."]).
	Command []string `json:"command"`

	// Output is the combined stdout/stderr output from the command.
	Output string `json:"output"`

	// Duration is how long the command took to execute.
	Duration time.Duration `json:"duration"`
}

// Verifier defines the interface for running verification commands.
type Verifier interface {
	// Verify runs the given commands and returns results for each.
	// Commands are executed sequentially in the order provided.
	// The context can be used to set timeouts or cancel execution.
	Verify(ctx context.Context, commands [][]string) ([]VerificationResult, error)

	// VerifyTask runs verification commands for a specific task.
	// This is a convenience method that takes task verify commands directly.
	// The context can be used to set timeouts or cancel execution.
	VerifyTask(ctx context.Context, commands [][]string) ([]VerificationResult, error)
}
