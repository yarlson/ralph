package opencode

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/yarlson/ralph/internal/claude"
	"github.com/yarlson/ralph/internal/stream"
)

// ParseResult contains the extracted data from parsing OpenCode NDJSON output.
type ParseResult struct {
	SessionID   string
	FinalText   string
	StreamText  string
	Usage       claude.ClaudeUsage
	ParseErrors []string
}

const (
	opencodeBufferSize   = 64 * 1024
	opencodeMaxTokenSize = 10 * 1024 * 1024
)

// ParseNDJSON parses OpenCode NDJSON output and extracts the final response.
func ParseNDJSON(r io.Reader) (*ParseResult, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, opencodeBufferSize), opencodeMaxTokenSize)

	result := &ParseResult{}
	var streamTextBuilder strings.Builder
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			result.ParseErrors = append(result.ParseErrors, fmt.Sprintf("line %d: invalid JSON: %v", lineNum, err))
			continue
		}

		if result.SessionID == "" {
			if sessionID := getString(event, "sessionID"); sessionID != "" {
				result.SessionID = sessionID
			}
		}

		text, _ := stream.ExtractText(event)
		if text != "" {
			streamTextBuilder.WriteString(text)
		}

		if part, ok := event["part"].(map[string]any); ok {
			if result.SessionID == "" {
				if sessionID := getString(part, "sessionID"); sessionID != "" {
					result.SessionID = sessionID
				}
			}
			applyTokens(&result.Usage, part)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanner error: %w", err)
	}

	result.StreamText = streamTextBuilder.String()
	result.FinalText = result.StreamText

	if result.StreamText == "" {
		return nil, fmt.Errorf("no text events found in stream")
	}

	return result, nil
}

func applyTokens(usage *claude.ClaudeUsage, part map[string]any) {
	tokens, ok := part["tokens"].(map[string]any)
	if !ok {
		return
	}
	usage.InputTokens += getInt(tokens["input"])
	usage.OutputTokens += getInt(tokens["output"])

	cache, ok := tokens["cache"].(map[string]any)
	if !ok {
		return
	}
	usage.CacheReadTokens += getInt(cache["read"])
	usage.CacheCreationTokens += getInt(cache["write"])
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getInt(v any) int {
	switch value := v.(type) {
	case float64:
		return int(value)
	case int:
		return value
	case int64:
		return int(value)
	case json.Number:
		if parsed, err := value.Int64(); err == nil {
			return int(parsed)
		}
	}
	return 0
}
