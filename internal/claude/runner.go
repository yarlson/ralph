// Package claude provides integration with Claude Code subprocess execution.
package claude

import "context"

// ClaudeRequest contains the parameters for invoking Claude Code.
type ClaudeRequest struct {
	// Cwd is the working directory for the Claude subprocess (typically repo root).
	Cwd string `json:"cwd"`

	// SystemPrompt is the system prompt text to pass via --system-prompt.
	SystemPrompt string `json:"system_prompt,omitempty"`

	// AllowedTools lists the tools Claude may use (e.g., ["Read", "Edit", "Bash"]).
	// Passed via --allowedTools flag.
	AllowedTools []string `json:"allowed_tools,omitempty"`

	// Prompt is the user message content to pass via -p flag.
	Prompt string `json:"prompt"`

	// Continue indicates whether to continue an existing session (--continue flag).
	Continue bool `json:"continue,omitempty"`

	// ExtraArgs are additional CLI arguments to pass to Claude.
	ExtraArgs []string `json:"extra_args,omitempty"`

	// Env contains additional environment variables for the subprocess.
	Env map[string]string `json:"env,omitempty"`
}

// ClaudeResponse contains the results from a Claude Code invocation.
type ClaudeResponse struct {
	// SessionID is the session identifier from system/init event.
	SessionID string `json:"session_id"`

	// Model is the model name from system/init event.
	Model string `json:"model,omitempty"`

	// Version is the claude_code_version from system/init event.
	Version string `json:"version,omitempty"`

	// FinalText is the authoritative final output from result event.
	FinalText string `json:"final_text,omitempty"`

	// StreamText is the accumulated text from assistant/message events.
	// Used as fallback if FinalText is empty.
	StreamText string `json:"stream_text,omitempty"`

	// Usage contains token usage statistics from the result event.
	Usage ClaudeUsage `json:"usage"`

	// TotalCostUSD is the total cost from result event (for budget tracking).
	TotalCostUSD float64 `json:"total_cost_usd,omitempty"`

	// PermissionDenials lists any permission denials from result event.
	PermissionDenials []string `json:"permission_denials,omitempty"`

	// RawEventsPath is the path to the saved NDJSON log file for audit/replay.
	RawEventsPath string `json:"raw_events_path,omitempty"`
}

// ClaudeUsage contains token usage statistics from Claude.
type ClaudeUsage struct {
	// InputTokens is the number of input tokens consumed.
	InputTokens int `json:"input_tokens,omitempty"`

	// OutputTokens is the number of output tokens generated.
	OutputTokens int `json:"output_tokens,omitempty"`

	// CacheCreationTokens is the number of tokens used for cache creation.
	CacheCreationTokens int `json:"cache_creation_tokens,omitempty"`

	// CacheReadTokens is the number of tokens read from cache.
	CacheReadTokens int `json:"cache_read_tokens,omitempty"`
}

// Runner is the interface for executing Claude Code as a subprocess.
// Implementations handle command building, process execution, NDJSON parsing,
// and result extraction.
type Runner interface {
	// Run executes Claude Code with the given request and returns the response.
	// The context can be used for cancellation/timeout.
	// Returns an error if the subprocess fails or parsing fails to extract a result.
	Run(ctx context.Context, req ClaudeRequest) (*ClaudeResponse, error)
}
