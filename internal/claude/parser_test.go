package claude

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseNDJSON_EmptyInput(t *testing.T) {
	reader := strings.NewReader("")
	result, err := ParseNDJSON(reader)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no terminal result")
	assert.Nil(t, result)
}

func TestParseNDJSON_SystemInit(t *testing.T) {
	input := `{"type":"system","subtype":"init","cwd":"/repo","session_id":"sess-123","model":"claude-opus-4-5-20251101","claude_code_version":"2.1.9","tools":["Read","Edit"]}
{"type":"result","subtype":"success","result":"done","session_id":"sess-123","total_cost_usd":0.01,"usage":{"input_tokens":100,"output_tokens":50}}`

	reader := strings.NewReader(input)
	result, err := ParseNDJSON(reader)
	require.NoError(t, err)

	assert.Equal(t, "sess-123", result.SessionID)
	assert.Equal(t, "claude-opus-4-5-20251101", result.Model)
	assert.Equal(t, "2.1.9", result.Version)
	assert.Equal(t, "/repo", result.Cwd)
}

func TestParseNDJSON_AssistantMessage(t *testing.T) {
	input := `{"type":"system","subtype":"init","session_id":"sess-123"}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Hello, "},{"type":"text","text":"world!"}]},"session_id":"sess-123"}
{"type":"result","subtype":"success","result":"final text","session_id":"sess-123"}`

	reader := strings.NewReader(input)
	result, err := ParseNDJSON(reader)
	require.NoError(t, err)

	assert.Equal(t, "Hello, world!", result.StreamText)
	assert.Equal(t, "final text", result.FinalText)
}

func TestParseNDJSON_MultipleAssistantMessages(t *testing.T) {
	input := `{"type":"system","subtype":"init","session_id":"sess-123"}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"First "}]},"session_id":"sess-123"}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Second"}]},"session_id":"sess-123"}
{"type":"result","subtype":"success","result":"final","session_id":"sess-123"}`

	reader := strings.NewReader(input)
	result, err := ParseNDJSON(reader)
	require.NoError(t, err)

	assert.Equal(t, "First Second", result.StreamText)
}

func TestParseNDJSON_ResultSuccess(t *testing.T) {
	input := `{"type":"system","subtype":"init","session_id":"sess-123"}
{"type":"result","subtype":"success","result":"The final output text","session_id":"sess-123","total_cost_usd":0.009631,"usage":{"input_tokens":500,"output_tokens":200,"cache_creation_tokens":10,"cache_read_tokens":5},"duration_ms":5000,"num_turns":3}`

	reader := strings.NewReader(input)
	result, err := ParseNDJSON(reader)
	require.NoError(t, err)

	assert.Equal(t, "The final output text", result.FinalText)
	assert.Equal(t, 0.009631, result.TotalCostUSD)
	assert.Equal(t, 500, result.Usage.InputTokens)
	assert.Equal(t, 200, result.Usage.OutputTokens)
	assert.Equal(t, 10, result.Usage.CacheCreationTokens)
	assert.Equal(t, 5, result.Usage.CacheReadTokens)
	assert.Equal(t, 5000, result.DurationMS)
	assert.Equal(t, 3, result.NumTurns)
}

func TestParseNDJSON_ResultError(t *testing.T) {
	input := `{"type":"system","subtype":"init","session_id":"sess-123"}
{"type":"result","subtype":"error","is_error":true,"result":"Something went wrong","session_id":"sess-123"}`

	reader := strings.NewReader(input)
	result, err := ParseNDJSON(reader)
	require.NoError(t, err)

	assert.True(t, result.IsError)
	assert.Equal(t, "Something went wrong", result.FinalText)
}

func TestParseNDJSON_PermissionDenials(t *testing.T) {
	input := `{"type":"system","subtype":"init","session_id":"sess-123"}
{"type":"result","subtype":"success","result":"partial","session_id":"sess-123","permission_denials":["edit /etc/passwd","run rm -rf /"]}`

	reader := strings.NewReader(input)
	result, err := ParseNDJSON(reader)
	require.NoError(t, err)

	require.Len(t, result.PermissionDenials, 2)
	assert.Equal(t, "edit /etc/passwd", result.PermissionDenials[0])
	assert.Equal(t, "run rm -rf /", result.PermissionDenials[1])
}

func TestParseNDJSON_NonTextContent(t *testing.T) {
	// Parser should ignore non-text content types
	input := `{"type":"system","subtype":"init","session_id":"sess-123"}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Hello"},{"type":"tool_use","name":"read","input":{}},{"type":"text","text":" World"}]},"session_id":"sess-123"}
{"type":"result","subtype":"success","result":"done","session_id":"sess-123"}`

	reader := strings.NewReader(input)
	result, err := ParseNDJSON(reader)
	require.NoError(t, err)

	assert.Equal(t, "Hello World", result.StreamText)
}

func TestParseNDJSON_MalformedLine(t *testing.T) {
	// Parser should continue on malformed lines and succeed if terminal result is found
	input := `{"type":"system","subtype":"init","session_id":"sess-123"}
this is not valid json
{"type":"result","subtype":"success","result":"done","session_id":"sess-123"}`

	reader := strings.NewReader(input)
	result, err := ParseNDJSON(reader)
	require.NoError(t, err)

	assert.Equal(t, "done", result.FinalText)
	require.Len(t, result.ParseErrors, 1)
	assert.Contains(t, result.ParseErrors[0], "line 2")
}

func TestParseNDJSON_MultipleMalformedLines(t *testing.T) {
	input := `{"type":"system","subtype":"init","session_id":"sess-123"}
bad line 1
bad line 2
{"type":"result","subtype":"success","result":"done","session_id":"sess-123"}`

	reader := strings.NewReader(input)
	result, err := ParseNDJSON(reader)
	require.NoError(t, err)

	require.Len(t, result.ParseErrors, 2)
}

func TestParseNDJSON_NoResult(t *testing.T) {
	input := `{"type":"system","subtype":"init","session_id":"sess-123"}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Hello"}]},"session_id":"sess-123"}`

	reader := strings.NewReader(input)
	result, err := ParseNDJSON(reader)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no terminal result")
	assert.Nil(t, result)
}

func TestParseNDJSON_UnknownEventType(t *testing.T) {
	// Parser should ignore unknown event types
	input := `{"type":"system","subtype":"init","session_id":"sess-123"}
{"type":"unknown","foo":"bar"}
{"type":"result","subtype":"success","result":"done","session_id":"sess-123"}`

	reader := strings.NewReader(input)
	result, err := ParseNDJSON(reader)
	require.NoError(t, err)

	assert.Equal(t, "done", result.FinalText)
}

func TestParseNDJSON_LargeLine(t *testing.T) {
	// Generate a large text content to ensure scanner buffer handles it
	largeText := strings.Repeat("a", 100000) // 100KB of text
	input := `{"type":"system","subtype":"init","session_id":"sess-123"}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"` + largeText + `"}]},"session_id":"sess-123"}
{"type":"result","subtype":"success","result":"done","session_id":"sess-123"}`

	reader := strings.NewReader(input)
	result, err := ParseNDJSON(reader)
	require.NoError(t, err)

	assert.Equal(t, largeText, result.StreamText)
}

func TestParseNDJSON_FallbackToStreamText(t *testing.T) {
	// If FinalText is empty but StreamText exists, FinalText should be populated
	input := `{"type":"system","subtype":"init","session_id":"sess-123"}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"streamed content"}]},"session_id":"sess-123"}
{"type":"result","subtype":"success","result":"","session_id":"sess-123"}`

	reader := strings.NewReader(input)
	result, err := ParseNDJSON(reader)
	require.NoError(t, err)

	assert.Equal(t, "streamed content", result.StreamText)
	// FinalText stays empty as returned from result
	assert.Equal(t, "", result.FinalText)
}

func TestParseNDJSON_EmptyLines(t *testing.T) {
	// Parser should handle empty lines gracefully
	input := `{"type":"system","subtype":"init","session_id":"sess-123"}

{"type":"result","subtype":"success","result":"done","session_id":"sess-123"}
`

	reader := strings.NewReader(input)
	result, err := ParseNDJSON(reader)
	require.NoError(t, err)

	assert.Equal(t, "done", result.FinalText)
}

func TestParseNDJSON_SystemSubtypes(t *testing.T) {
	// Non-init system subtypes should be ignored
	input := `{"type":"system","subtype":"init","session_id":"sess-123"}
{"type":"system","subtype":"other","foo":"bar"}
{"type":"result","subtype":"success","result":"done","session_id":"sess-123"}`

	reader := strings.NewReader(input)
	result, err := ParseNDJSON(reader)
	require.NoError(t, err)

	assert.Equal(t, "sess-123", result.SessionID)
}

func TestParseNDJSON_UsageInAssistantMessage(t *testing.T) {
	input := `{"type":"system","subtype":"init","session_id":"sess-123"}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Hello"}],"usage":{"input_tokens":100,"output_tokens":50}},"session_id":"sess-123"}
{"type":"result","subtype":"success","result":"done","session_id":"sess-123","usage":{"input_tokens":200,"output_tokens":100}}`

	reader := strings.NewReader(input)
	result, err := ParseNDJSON(reader)
	require.NoError(t, err)

	// Final usage from result should take precedence
	assert.Equal(t, 200, result.Usage.InputTokens)
	assert.Equal(t, 100, result.Usage.OutputTokens)
}

func TestParseResult_StructFields(t *testing.T) {
	pr := &ParseResult{
		SessionID:  "test-session",
		Model:      "claude-opus-4-5-20251101",
		Version:    "2.1.9",
		Cwd:        "/test/dir",
		FinalText:  "final",
		StreamText: "streamed",
	}

	assert.Equal(t, "test-session", pr.SessionID)
	assert.Equal(t, "claude-opus-4-5-20251101", pr.Model)
	assert.Equal(t, "2.1.9", pr.Version)
	assert.Equal(t, "/test/dir", pr.Cwd)
	assert.Equal(t, "final", pr.FinalText)
	assert.Equal(t, "streamed", pr.StreamText)
}

func TestParseResult_UsageFields(t *testing.T) {
	usage := ClaudeUsage{
		InputTokens:        100,
		OutputTokens:       50,
		CacheCreationTokens: 10,
		CacheReadTokens:     5,
	}

	assert.Equal(t, 100, usage.InputTokens)
	assert.Equal(t, 50, usage.OutputTokens)
	assert.Equal(t, 10, usage.CacheCreationTokens)
	assert.Equal(t, 5, usage.CacheReadTokens)
}
