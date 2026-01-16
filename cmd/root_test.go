package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCommand(t *testing.T) {
	t.Run("executes without error", func(t *testing.T) {
		cmd := NewRootCmd()
		cmd.SetArgs([]string{})
		err := cmd.Execute()
		assert.NoError(t, err)
	})

	t.Run("has --config flag", func(t *testing.T) {
		cmd := NewRootCmd()
		flag := cmd.PersistentFlags().Lookup("config")
		require.NotNil(t, flag, "expected --config flag to exist")
		assert.Equal(t, "ralph.yaml", flag.DefValue)
	})

	t.Run("help shows all subcommands", func(t *testing.T) {
		cmd := NewRootCmd()
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetArgs([]string{"--help"})
		err := cmd.Execute()
		require.NoError(t, err)

		output := buf.String()
		expectedCommands := []string{
			"init",
			"run",
			"status",
			"pause",
			"resume",
			"retry",
			"skip",
			"report",
		}
		for _, subcmd := range expectedCommands {
			assert.True(t, strings.Contains(output, subcmd),
				"expected help to contain '%s'", subcmd)
		}
	})
}

func TestSubcommands(t *testing.T) {
	// Commands that are still stubs (not yet implemented)
	stubCommands := []string{
		"run",
		"status",
		"pause",
		"resume",
		"retry",
		"skip",
		"report",
	}

	for _, name := range stubCommands {
		t.Run(name+" returns not implemented", func(t *testing.T) {
			cmd := NewRootCmd()
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			cmd.SetArgs([]string{name})

			err := cmd.Execute()
			// Stub commands return an error with "not implemented"
			require.Error(t, err)
			assert.Contains(t, err.Error(), "not implemented")
		})
	}
}
