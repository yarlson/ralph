package stream

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamJSONObjects(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single object",
			input:    `{"text":"hello"}`,
			expected: []string{`{"text":"hello"}`},
		},
		{
			name:     "multiple objects no separator",
			input:    `{"a":1}{"b":2}{"c":3}`,
			expected: []string{`{"a":1}`, `{"b":2}`, `{"c":3}`},
		},
		{
			name:     "newline separated",
			input:    "{\"a\":1}\n{\"b\":2}\n",
			expected: []string{`{"a":1}`, `{"b":2}`},
		},
		{
			name:     "noise between objects",
			input:    `garbage{"a":1}more noise{"b":2}end`,
			expected: []string{`{"a":1}`, `{"b":2}`},
		},
		{
			name:     "nested braces",
			input:    `{"outer":{"inner":1}}`,
			expected: []string{`{"outer":{"inner":1}}`},
		},
		{
			name:     "string with braces",
			input:    `{"text":"hello {world}"}`,
			expected: []string{`{"text":"hello {world}"}`},
		},
		{
			name:     "escaped quotes in string",
			input:    `{"text":"say \"hello\""}`,
			expected: []string{`{"text":"say \"hello\""}`},
		},
		{
			name:     "empty object",
			input:    `{}`,
			expected: []string{`{}`},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var objects []string
			err := JsonObjects(strings.NewReader(tc.input), func(raw []byte) {
				objects = append(objects, string(raw))
			})
			// EOF is expected
			assert.Equal(t, io.EOF, err)
			assert.Equal(t, tc.expected, objects)
		})
	}
}

func TestSanitize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain text",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "preserves newlines",
			input:    "line1\nline2\n",
			expected: "line1\nline2\n",
		},
		{
			name:     "preserves tabs",
			input:    "col1\tcol2",
			expected: "col1\tcol2",
		},
		{
			name:     "removes ANSI color codes",
			input:    "\x1b[31mred\x1b[0m",
			expected: "red",
		},
		{
			name:     "removes ANSI cursor movement",
			input:    "\x1b[2Jhello\x1b[H",
			expected: "hello",
		},
		{
			name:     "removes control chars",
			input:    "hello\x00\x01\x02world",
			expected: "helloworld",
		},
		{
			name:     "keeps unicode",
			input:    "héllo wörld 🎉",
			expected: "héllo wörld 🎉",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := Sanitize(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestExtractText(t *testing.T) {
	tests := []struct {
		name         string
		event        map[string]any
		expectedText string
		expectedMode string
	}{
		{
			name:         "direct text field",
			event:        map[string]any{"text": "hello"},
			expectedText: "hello",
			expectedMode: "message",
		},
		{
			name:         "text_delta field",
			event:        map[string]any{"text_delta": "hello"},
			expectedText: "hello",
			expectedMode: "delta",
		},
		{
			name: "delta object with text",
			event: map[string]any{
				"delta": map[string]any{"text": "hello"},
			},
			expectedText: "hello",
			expectedMode: "delta",
		},
		{
			name: "content_block text",
			event: map[string]any{
				"content_block": map[string]any{"text": "hello"},
			},
			expectedText: "hello",
			expectedMode: "message",
		},
		{
			name: "message with content blocks",
			event: map[string]any{
				"message": map[string]any{
					"content": []any{
						map[string]any{"type": "text", "text": "hello "},
						map[string]any{"type": "text", "text": "world"},
					},
				},
			},
			expectedText: "hello world",
			expectedMode: "message",
		},
		{
			name: "content_block_delta type",
			event: map[string]any{
				"type": "content_block_delta",
				"text": "hello",
			},
			expectedText: "hello",
			expectedMode: "delta",
		},
		{
			name: "wrapped in chunk",
			event: map[string]any{
				"chunk": map[string]any{"text": "hello"},
			},
			expectedText: "hello",
			expectedMode: "message",
		},
		{
			name: "wrapped in part",
			event: map[string]any{
				"part": map[string]any{"text": "hello"},
			},
			expectedText: "hello",
			expectedMode: "message",
		},
		{
			name:         "no text",
			event:        map[string]any{"type": "system", "subtype": "init"},
			expectedText: "",
			expectedMode: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			text, mode := ExtractText(tc.event)
			assert.Equal(t, tc.expectedText, text)
			assert.Equal(t, tc.expectedMode, mode)
		})
	}
}

func TestExtractAnyText(t *testing.T) {
	tests := []struct {
		name     string
		event    map[string]any
		expected string
	}{
		{
			name:     "text field",
			event:    map[string]any{"text": "hello"},
			expected: "hello",
		},
		{
			name:     "output field",
			event:    map[string]any{"output": "result"},
			expected: "result",
		},
		{
			name:     "result field",
			event:    map[string]any{"result": "done"},
			expected: "done",
		},
		{
			name:     "stdout field",
			event:    map[string]any{"stdout": "output"},
			expected: "output",
		},
		{
			name:     "stderr field",
			event:    map[string]any{"stderr": "error"},
			expected: "error",
		},
		{
			name:     "part wrapper",
			event:    map[string]any{"part": map[string]any{"text": "hello"}},
			expected: "hello",
		},
		{
			name:     "empty",
			event:    map[string]any{"other": "value"},
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ExtractAnyText(tc.event)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestProcessor(t *testing.T) {
	t.Run("processes text events", func(t *testing.T) {
		input := `{"type":"assistant","text":"Hello, "}{"type":"assistant","text":"world!"}`
		var out bytes.Buffer
		p := NewProcessor(&out, Options{})

		err := p.Process(strings.NewReader(input))
		require.NoError(t, err)
		assert.Equal(t, "Hello, \nworld!\n", out.String())
	})

	t.Run("processes delta events", func(t *testing.T) {
		input := `{"type":"content_block_delta","text":"chunk1"}{"type":"content_block_delta","text":"chunk2"}`
		var out bytes.Buffer
		p := NewProcessor(&out, Options{})

		err := p.Process(strings.NewReader(input))
		require.NoError(t, err)
		assert.Equal(t, "chunk1chunk2", out.String())
	})

	t.Run("shows tools when enabled", func(t *testing.T) {
		input := `{"type":"tool_use","tool":"Read","text":"content"}`
		var out bytes.Buffer
		p := NewProcessor(&out, Options{ShowTools: true})

		err := p.Process(strings.NewReader(input))
		require.NoError(t, err)
		assert.Contains(t, out.String(), "--- TOOL: Read ---")
		assert.Contains(t, out.String(), "content")
		assert.Contains(t, out.String(), "--- END TOOL ---")
	})

	t.Run("hides tools when disabled", func(t *testing.T) {
		input := `{"type":"tool_use","tool":"Read","text":"content"}`
		var out bytes.Buffer
		p := NewProcessor(&out, Options{ShowTools: false})

		err := p.Process(strings.NewReader(input))
		require.NoError(t, err)
		assert.NotContains(t, out.String(), "TOOL")
	})

	t.Run("debug unhandled events", func(t *testing.T) {
		input := `{"type":"unknown","subtype":"event"}`
		var out, debug bytes.Buffer
		p := NewProcessor(&out, Options{
			DebugUnhandled: true,
			DebugWriter:    &debug,
		})

		err := p.Process(strings.NewReader(input))
		require.NoError(t, err)
		assert.Contains(t, debug.String(), "UNHANDLED")
		assert.Contains(t, debug.String(), "unknown")
	})

	t.Run("sanitizes output", func(t *testing.T) {
		// Use \u001b which is valid JSON escape for ESC character
		input := `{"text":"\u001b[31mcolored\u001b[0m text"}`
		var out bytes.Buffer
		p := NewProcessor(&out, Options{})

		err := p.Process(strings.NewReader(input))
		require.NoError(t, err)
		assert.Equal(t, "colored text\n", out.String())
	})
}
