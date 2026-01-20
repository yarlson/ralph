package opencode

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

	"github.com/yarlson/ralph/internal/claude"
	"github.com/yarlson/ralph/internal/stream"
)

// SubprocessRunner executes OpenCode as a subprocess and parses its NDJSON output.
type SubprocessRunner struct {
	command      string
	baseArgs     []string
	logsDir      string
	TaskID       string
	streamOutput bool
	streamOpts   stream.Options
}

// NewSubprocessRunner creates a new SubprocessRunner with the given command and logs directory.
func NewSubprocessRunner(command, logsDir string) *SubprocessRunner {
	return &SubprocessRunner{
		command:  command,
		baseArgs: []string{},
		logsDir:  logsDir,
	}
}

// WithBaseArgs sets the base arguments to prepend before OpenCode-specific flags.
func (r *SubprocessRunner) WithBaseArgs(baseArgs []string) *SubprocessRunner {
	r.baseArgs = baseArgs
	return r
}

// WithStreamOutput enables real-time streaming of OpenCode's text output to stdout.
func (r *SubprocessRunner) WithStreamOutput(opts stream.Options) *SubprocessRunner {
	r.streamOutput = true
	r.streamOpts = opts
	return r
}

// Run executes OpenCode with the given request and returns the response.
func (r *SubprocessRunner) Run(ctx context.Context, req claude.ClaudeRequest) (*claude.ClaudeResponse, error) {
	args := buildArgs(req, r.baseArgs)

	cmd := exec.CommandContext(ctx, r.command, args...)
	if req.Cwd != "" {
		cmd.Dir = req.Cwd
	}

	if len(req.Env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range req.Env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}

	logFilename := generateLogFilename(r.TaskID)
	logPath := filepath.Join(r.logsDir, logFilename)
	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file %s: %w", logPath, err)
	}
	defer func() { _ = logFile.Close() }()

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command %s: %w", r.command, err)
	}

	var stdoutBuf bytes.Buffer
	writers := []io.Writer{logFile, &stdoutBuf}

	var streamPipeWriter *io.PipeWriter
	var streamDone chan struct{}
	if r.streamOutput {
		var streamPipeReader *io.PipeReader
		streamPipeReader, streamPipeWriter = io.Pipe()
		writers = append(writers, streamPipeWriter)
		streamDone = make(chan struct{})

		go func() {
			defer close(streamDone)
			processor := stream.NewProcessor(os.Stdout, r.streamOpts)
			_ = processor.Process(streamPipeReader)
		}()
	}

	multiWriter := io.MultiWriter(writers...)
	_, copyErr := io.Copy(multiWriter, stdoutPipe)

	if streamPipeWriter != nil {
		_ = streamPipeWriter.Close()
		<-streamDone
	}

	waitErr := cmd.Wait()

	if ctx.Err() != nil {
		return nil, fmt.Errorf("command cancelled: %w", ctx.Err())
	}

	if copyErr != nil {
		return nil, fmt.Errorf("error reading stdout: %w", copyErr)
	}

	if waitErr != nil {
		stderr := strings.TrimSpace(stderrBuf.String())
		if stderr != "" {
			return nil, fmt.Errorf("command failed: %w, stderr: %s", waitErr, stderr)
		}
		return nil, fmt.Errorf("command failed: %w", waitErr)
	}

	parseResult, err := ParseNDJSON(&stdoutBuf)
	if err != nil {
		return nil, fmt.Errorf("failed to parse NDJSON output: %w", err)
	}

	response := &claude.ClaudeResponse{
		SessionID:     parseResult.SessionID,
		FinalText:     parseResult.FinalText,
		StreamText:    parseResult.StreamText,
		Usage:         parseResult.Usage,
		RawEventsPath: logPath,
	}

	return response, nil
}

func buildArgs(req claude.ClaudeRequest, baseArgs []string) []string {
	args := append([]string{}, baseArgs...)

	args = append(args, "--format", "json")

	if req.Continue {
		args = append(args, "--continue")
	}

	args = append(args, req.ExtraArgs...)
	args = append(args, buildPrompt(req))

	return args
}

func buildPrompt(req claude.ClaudeRequest) string {
	var builder strings.Builder
	var system []string

	if req.SystemPrompt != "" {
		system = append(system, req.SystemPrompt)
	}
	if len(req.AllowedTools) > 0 {
		system = append(system, fmt.Sprintf("Allowed tools: %s.", strings.Join(req.AllowedTools, ", ")))
	}

	if len(system) > 0 {
		builder.WriteString("SYSTEM:\n")
		builder.WriteString(strings.Join(system, "\n"))
		builder.WriteString("\n\nUSER:\n")
	}

	builder.WriteString(req.Prompt)
	return builder.String()
}

var invalidFilenameChars = regexp.MustCompile(`[/\\:*?"<>|\s]`)

func generateLogFilename(taskID string) string {
	timestamp := time.Now().Format("20060102-150405")
	if taskID == "" {
		taskID = "opencode"
	}
	safeTaskID := invalidFilenameChars.ReplaceAllString(taskID, "-")
	return fmt.Sprintf("%s-%s.ndjson", timestamp, safeTaskID)
}
