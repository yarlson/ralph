package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWarnDeprecated(t *testing.T) {
	tests := []struct {
		name            string
		command         string
		wantWarning     bool
		wantReplacement string
	}{
		{
			name:            "init command",
			command:         "init",
			wantWarning:     true,
			wantReplacement: "ralph (auto-initializes) or ralph --parent <id>",
		},
		{
			name:            "run command",
			command:         "run",
			wantWarning:     true,
			wantReplacement: "ralph",
		},
		{
			name:            "decompose command",
			command:         "decompose",
			wantWarning:     true,
			wantReplacement: "ralph <prd.md>",
		},
		{
			name:            "import command",
			command:         "import",
			wantWarning:     true,
			wantReplacement: "ralph <tasks.yaml>",
		},
		{
			name:            "pause command",
			command:         "pause",
			wantWarning:     true,
			wantReplacement: "Ctrl+C to stop. Run ralph to resume",
		},
		{
			name:            "resume command",
			command:         "resume",
			wantWarning:     true,
			wantReplacement: "ralph",
		},
		{
			name:            "retry command",
			command:         "retry",
			wantWarning:     true,
			wantReplacement: "ralph fix --retry <id>",
		},
		{
			name:            "skip command",
			command:         "skip",
			wantWarning:     true,
			wantReplacement: "ralph fix --skip <id>",
		},
		{
			name:            "revert command",
			command:         "revert",
			wantWarning:     true,
			wantReplacement: "ralph fix --undo <id>",
		},
		{
			name:            "logs command",
			command:         "logs",
			wantWarning:     true,
			wantReplacement: "ralph status --log",
		},
		{
			name:            "report command",
			command:         "report",
			wantWarning:     true,
			wantReplacement: "ralph status --report",
		},
		{
			name:        "unknown command",
			command:     "unknown",
			wantWarning: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stderr bytes.Buffer
			shouldContinue := warnDeprecated(&stderr, tt.command)

			if tt.wantWarning {
				assert.True(t, shouldContinue, "warnDeprecated should return true for deprecated commands")
				assert.Contains(t, stderr.String(), "Deprecated:")
				assert.Contains(t, stderr.String(), tt.wantReplacement)
				assert.Contains(t, stderr.String(), "Use "+tt.wantReplacement)
			} else {
				assert.False(t, shouldContinue, "warnDeprecated should return false for unknown commands")
				assert.Empty(t, stderr.String())
			}
		})
	}
}

func TestDeprecationMessagesMapHasAllCommands(t *testing.T) {
	expectedCommands := []string{
		"init",
		"run",
		"decompose",
		"import",
		"pause",
		"resume",
		"retry",
		"skip",
		"revert",
		"logs",
		"report",
	}

	require.Len(t, deprecationMessages, len(expectedCommands), "deprecationMessages should have exactly %d entries", len(expectedCommands))

	for _, cmd := range expectedCommands {
		_, ok := deprecationMessages[cmd]
		assert.True(t, ok, "deprecationMessages should contain %q", cmd)
	}
}
