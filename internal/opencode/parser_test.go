package opencode

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseNDJSON(t *testing.T) {
	input := strings.Join([]string{
		`{"type":"step_start","sessionID":"sess-123","part":{"type":"step-start"}}`,
		`{"type":"text","sessionID":"sess-123","part":{"type":"text","text":"Hello "}}`,
		`{"type":"text","sessionID":"sess-123","part":{"type":"text","text":"world"}}`,
		`{"type":"step_finish","sessionID":"sess-123","part":{"tokens":{"input":12,"output":4,"cache":{"read":2,"write":1}}}}`,
	}, "\n")

	result, err := ParseNDJSON(strings.NewReader(input))
	require.NoError(t, err)

	assert.Equal(t, "sess-123", result.SessionID)
	assert.Equal(t, "Hello world", result.StreamText)
	assert.Equal(t, "Hello world", result.FinalText)
	assert.Equal(t, 12, result.Usage.InputTokens)
	assert.Equal(t, 4, result.Usage.OutputTokens)
	assert.Equal(t, 1, result.Usage.CacheCreationTokens)
	assert.Equal(t, 2, result.Usage.CacheReadTokens)
}

func TestParseNDJSON_NoText(t *testing.T) {
	input := `{"type":"step_start","sessionID":"sess-123","part":{"type":"step-start"}}`

	_, err := ParseNDJSON(strings.NewReader(input))
	assert.Error(t, err)
}
