package claude

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// SubprocessRunner executes Claude Code as a subprocess and parses its NDJSON output.
type SubprocessRunner struct {
	// command is the path to the Claude binary (e.g., "claude" or "/usr/bin/claude").
	command string
	// logsDir is the directory where raw NDJSON logs are saved.
	logsDir string
	// TaskID is an optional task identifier to include in log filenames.
	TaskID string
}

// NewSubprocessRunner creates a new SubprocessRunner with the given command and logs directory.
func NewSubprocessRunner(command, logsDir string) *SubprocessRunner {
	return &SubprocessRunner{
		command: command,
		logsDir: logsDir,
	}
}

// Run executes Claude Code with the given request and returns the response.
// It streams stdout to both the parser and a log file simultaneously.
// The context can be used for cancellation/timeout.
func (r *SubprocessRunner) Run(ctx context.Context, req ClaudeRequest) (*ClaudeResponse, error) {
	// Build command arguments
	args := buildArgs(req)

	// Create the command
	cmd := exec.CommandContext(ctx, r.command, args...)

	// Set working directory if specified
	if req.Cwd != "" {
		cmd.Dir = req.Cwd
	}

	// Set environment variables
	if len(req.Env) > 0 {
		// Start with current environment
		cmd.Env = os.Environ()
		// Add custom environment variables
		for k, v := range req.Env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}

	// Create log file
	logFilename := generateLogFilename(r.TaskID)
	logPath := filepath.Join(r.logsDir, logFilename)

	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file %s: %w", logPath, err)
	}
	defer func() { _ = logFile.Close() }()

	// Capture stdout for parsing and logging
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Capture stderr for error reporting
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	// Start the command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command %s: %w", r.command, err)
	}

	// Tee stdout to both the log file and a buffer for parsing
	var stdoutBuf bytes.Buffer
	teeReader := io.TeeReader(stdoutPipe, &stdoutBuf)

	// Copy stdout to log file while command runs
	_, copyErr := io.Copy(logFile, teeReader)

	// Wait for command to complete
	waitErr := cmd.Wait()

	// Check for context cancellation
	if ctx.Err() != nil {
		return nil, fmt.Errorf("command cancelled: %w", ctx.Err())
	}

	// Check for copy errors
	if copyErr != nil {
		return nil, fmt.Errorf("error reading stdout: %w", copyErr)
	}

	// Check for command errors (non-zero exit) that aren't from parsing
	if waitErr != nil {
		// Include stderr in error message if available
		stderr := strings.TrimSpace(stderrBuf.String())
		if stderr != "" {
			return nil, fmt.Errorf("command failed: %w, stderr: %s", waitErr, stderr)
		}
		return nil, fmt.Errorf("command failed: %w", waitErr)
	}

	// Parse the NDJSON output
	parseResult, err := ParseNDJSON(&stdoutBuf)
	if err != nil {
		return nil, fmt.Errorf("failed to parse NDJSON output: %w", err)
	}

	// Convert ParseResult to ClaudeResponse
	response := &ClaudeResponse{
		SessionID:         parseResult.SessionID,
		Model:             parseResult.Model,
		Version:           parseResult.Version,
		FinalText:         parseResult.FinalText,
		StreamText:        parseResult.StreamText,
		Usage:             parseResult.Usage,
		TotalCostUSD:      parseResult.TotalCostUSD,
		PermissionDenials: parseResult.PermissionDenials,
		RawEventsPath:     logPath,
	}

	return response, nil
}

// buildArgs constructs the command-line arguments for the Claude subprocess.
func buildArgs(req ClaudeRequest) []string {
	var args []string

	// Always use stream-json output format for structured parsing
	args = append(args, "--output-format=stream-json")

	// Add system prompt if specified
	if req.SystemPrompt != "" {
		args = append(args, "--system-prompt", req.SystemPrompt)
	}

	// Add allowed tools if specified
	if len(req.AllowedTools) > 0 {
		args = append(args, "--allowedTools", strings.Join(req.AllowedTools, ","))
	}

	// Add continue flag if specified
	if req.Continue {
		args = append(args, "--continue")
	}

	// Add extra args
	args = append(args, req.ExtraArgs...)

	// Add prompt (must be last with -p flag)
	args = append(args, "-p", req.Prompt)

	return args
}

// sanitizeFilename replaces characters that are invalid in filenames.
var invalidFilenameChars = regexp.MustCompile(`[/\\:*?"<>|\s]`)

// generateLogFilename creates a unique log filename with timestamp and task ID.
func generateLogFilename(taskID string) string {
	timestamp := time.Now().Format("20060102-150405")

	if taskID == "" {
		taskID = "claude"
	}

	// Sanitize task ID for use in filename
	safeTaskID := invalidFilenameChars.ReplaceAllString(taskID, "-")

	return fmt.Sprintf("%s-%s.ndjson", timestamp, safeTaskID)
}
