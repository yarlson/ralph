package claude

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// ParseResult contains the extracted data from parsing Claude's NDJSON stream.
type ParseResult struct {
	// SessionID is the session identifier from system/init event.
	SessionID string `json:"session_id"`

	// Model is the model name from system/init event.
	Model string `json:"model,omitempty"`

	// Version is the claude_code_version from system/init event.
	Version string `json:"version,omitempty"`

	// Cwd is the working directory from system/init event.
	Cwd string `json:"cwd,omitempty"`

	// FinalText is the authoritative final output from result event.
	FinalText string `json:"final_text,omitempty"`

	// StreamText is the accumulated text from assistant/message events.
	StreamText string `json:"stream_text,omitempty"`

	// Usage contains token usage statistics from the result event.
	Usage ClaudeUsage `json:"usage"`

	// TotalCostUSD is the total cost from result event.
	TotalCostUSD float64 `json:"total_cost_usd,omitempty"`

	// DurationMS is the execution duration in milliseconds.
	DurationMS int `json:"duration_ms,omitempty"`

	// NumTurns is the number of conversation turns.
	NumTurns int `json:"num_turns,omitempty"`

	// IsError indicates if the result was an error.
	IsError bool `json:"is_error,omitempty"`

	// PermissionDenials lists any permission denials from result event.
	PermissionDenials []string `json:"permission_denials,omitempty"`

	// ParseErrors contains any non-fatal parse errors encountered.
	ParseErrors []string `json:"parse_errors,omitempty"`
}

// baseEvent is used for initial type/subtype detection.
type baseEvent struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
}

// initEvent represents system/init event fields.
type initEvent struct {
	SessionID string `json:"session_id"`
	Model     string `json:"model"`
	Version   string `json:"claude_code_version"`
	Cwd       string `json:"cwd"`
}

// assistantEvent represents assistant/message event structure.
type assistantEvent struct {
	Message struct {
		Content []contentBlock `json:"content"`
		Usage   *usageBlock    `json:"usage,omitempty"`
	} `json:"message"`
}

// contentBlock represents a content item in assistant message.
type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// usageBlock represents usage statistics in various events.
type usageBlock struct {
	InputTokens         int `json:"input_tokens"`
	OutputTokens        int `json:"output_tokens"`
	CacheCreationTokens int `json:"cache_creation_tokens"`
	CacheReadTokens     int `json:"cache_read_tokens"`
}

// resultEvent represents result/success or result/error event fields.
type resultEvent struct {
	Result            string     `json:"result"`
	IsError           bool       `json:"is_error"`
	TotalCostUSD      float64    `json:"total_cost_usd"`
	Usage             usageBlock `json:"usage"`
	DurationMS        int        `json:"duration_ms"`
	NumTurns          int        `json:"num_turns"`
	PermissionDenials []string   `json:"permission_denials"`
}

// bufferSize is the initial buffer size for scanner (64KB).
const bufferSize = 64 * 1024

// maxTokenSize is the maximum size of a single line (10MB).
const maxTokenSize = 10 * 1024 * 1024

// ParseNDJSON parses Claude Code's NDJSON stream output from the given reader.
// It extracts session info, accumulated text, and final result.
// Returns an error if no terminal result event is found.
func ParseNDJSON(r io.Reader) (*ParseResult, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, bufferSize), maxTokenSize)

	result := &ParseResult{}
	var streamTextBuilder strings.Builder
	lineNum := 0
	hasTerminalResult := false

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Parse the base event to determine type
		var base baseEvent
		if err := json.Unmarshal([]byte(line), &base); err != nil {
			result.ParseErrors = append(result.ParseErrors,
				fmt.Sprintf("line %d: failed to parse JSON: %v", lineNum, err))
			continue
		}

		switch base.Type {
		case "system":
			if base.Subtype == "init" {
				var init initEvent
				if err := json.Unmarshal([]byte(line), &init); err != nil {
					result.ParseErrors = append(result.ParseErrors,
						fmt.Sprintf("line %d: failed to parse system/init: %v", lineNum, err))
					continue
				}
				result.SessionID = init.SessionID
				result.Model = init.Model
				result.Version = init.Version
				result.Cwd = init.Cwd
			}

		case "assistant":
			var assistant assistantEvent
			if err := json.Unmarshal([]byte(line), &assistant); err != nil {
				result.ParseErrors = append(result.ParseErrors,
					fmt.Sprintf("line %d: failed to parse assistant: %v", lineNum, err))
				continue
			}
			// Accumulate text content
			for _, block := range assistant.Message.Content {
				if block.Type == "text" {
					streamTextBuilder.WriteString(block.Text)
				}
			}

		case "result":
			var res resultEvent
			if err := json.Unmarshal([]byte(line), &res); err != nil {
				result.ParseErrors = append(result.ParseErrors,
					fmt.Sprintf("line %d: failed to parse result: %v", lineNum, err))
				continue
			}
			result.FinalText = res.Result
			result.IsError = res.IsError
			result.TotalCostUSD = res.TotalCostUSD
			result.DurationMS = res.DurationMS
			result.NumTurns = res.NumTurns
			result.PermissionDenials = res.PermissionDenials
			result.Usage = ClaudeUsage{
				InputTokens:         res.Usage.InputTokens,
				OutputTokens:        res.Usage.OutputTokens,
				CacheCreationTokens: res.Usage.CacheCreationTokens,
				CacheReadTokens:     res.Usage.CacheReadTokens,
			}
			hasTerminalResult = true
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanner error: %w", err)
	}

	if !hasTerminalResult {
		return nil, fmt.Errorf("no terminal result event found in stream")
	}

	result.StreamText = streamTextBuilder.String()

	return result, nil
}
