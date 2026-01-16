package verifier

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

// DefaultMaxOutputSize is the default maximum output size in bytes (1MB).
const DefaultMaxOutputSize = 1024 * 1024

// CommandRunner implements the Verifier interface by executing commands as subprocesses.
type CommandRunner struct {
	workDir         string
	allowedCommands map[string]bool
	maxOutputSize   int
}

// NewCommandRunner creates a new CommandRunner with the specified working directory.
// If workDir is empty, commands will run in the current working directory.
func NewCommandRunner(workDir string) *CommandRunner {
	return &CommandRunner{
		workDir:       workDir,
		maxOutputSize: DefaultMaxOutputSize,
	}
}

// SetAllowedCommands sets the allowlist of commands that can be executed.
// If set, only commands whose base name is in the list will be allowed.
// If not set (nil or empty), all commands are allowed.
func (r *CommandRunner) SetAllowedCommands(commands []string) {
	if len(commands) == 0 {
		r.allowedCommands = nil
		return
	}
	r.allowedCommands = make(map[string]bool, len(commands))
	for _, cmd := range commands {
		r.allowedCommands[cmd] = true
	}
}

// SetMaxOutputSize sets the maximum output size in bytes.
// Output exceeding this limit will be truncated.
func (r *CommandRunner) SetMaxOutputSize(size int) {
	r.maxOutputSize = size
}

// Verify executes the given commands sequentially and returns results for each.
// Commands are executed in order, and execution continues even if a command fails.
func (r *CommandRunner) Verify(ctx context.Context, commands [][]string) ([]VerificationResult, error) {
	if ctx == nil {
		return nil, errors.New("context cannot be nil")
	}

	results := make([]VerificationResult, 0, len(commands))

	for _, cmdArgs := range commands {
		result := r.runCommand(ctx, cmdArgs)
		results = append(results, result)
	}

	return results, nil
}

// VerifyTask is a convenience method that delegates to Verify.
func (r *CommandRunner) VerifyTask(ctx context.Context, commands [][]string) ([]VerificationResult, error) {
	return r.Verify(ctx, commands)
}

// runCommand executes a single command and returns the result.
func (r *CommandRunner) runCommand(ctx context.Context, cmdArgs []string) VerificationResult {
	start := time.Now()

	// Handle empty command
	if len(cmdArgs) == 0 {
		return VerificationResult{
			Passed:   false,
			Command:  cmdArgs,
			Output:   "error: empty command",
			Duration: time.Since(start),
		}
	}

	baseName := cmdArgs[0]

	// Check allowlist if configured
	if r.allowedCommands != nil {
		if !r.allowedCommands[baseName] {
			return VerificationResult{
				Passed:   false,
				Command:  cmdArgs,
				Output:   fmt.Sprintf("error: command %q is not allowed", baseName),
				Duration: time.Since(start),
			}
		}
	}

	// Create command with context
	cmd := exec.CommandContext(ctx, baseName, cmdArgs[1:]...)

	// Set working directory if specified
	if r.workDir != "" {
		cmd.Dir = r.workDir
	}

	// Capture combined stdout and stderr
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	// Execute command
	err := cmd.Run()
	duration := time.Since(start)

	// Get output, potentially truncated
	outputStr := r.truncateOutput(output.String())

	// Determine if command passed (exit code 0)
	passed := err == nil

	return VerificationResult{
		Passed:   passed,
		Command:  cmdArgs,
		Output:   outputStr,
		Duration: duration,
	}
}

// truncateOutput truncates the output if it exceeds maxOutputSize.
func (r *CommandRunner) truncateOutput(output string) string {
	if r.maxOutputSize <= 0 || len(output) <= r.maxOutputSize {
		return output
	}

	// Truncate and add marker
	truncateMsg := "\n... [output truncated]"
	return output[:r.maxOutputSize] + truncateMsg
}

// Ensure CommandRunner implements Verifier interface.
var _ Verifier = (*CommandRunner)(nil)
