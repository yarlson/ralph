package opencode

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yarlson/ralph/internal/claude"
)

func TestBuildArgs_DefaultModelVariant(t *testing.T) {
	req := claude.ClaudeRequest{
		Prompt: "Hello",
	}
	args := buildArgs(req, []string{})

	modelIndex := indexOf(args, "--model")
	require.NotEqual(t, -1, modelIndex)
	require.Less(t, modelIndex+1, len(args))
	assert.Equal(t, defaultModel, args[modelIndex+1])

	variantIndex := indexOf(args, "--variant")
	require.NotEqual(t, -1, variantIndex)
	require.Less(t, variantIndex+1, len(args))
	assert.Equal(t, defaultVariant, args[variantIndex+1])
}

func TestBuildArgs_RespectsExtraArgsModelVariant(t *testing.T) {
	req := claude.ClaudeRequest{
		Prompt:    "Hello",
		ExtraArgs: []string{"--model", "opencode/gpt-5.1-codex", "--variant", "high"},
	}
	args := buildArgs(req, []string{})

	modelIndex := indexOf(args, "--model")
	require.NotEqual(t, -1, modelIndex)
	require.Less(t, modelIndex+1, len(args))
	assert.Equal(t, "opencode/gpt-5.1-codex", args[modelIndex+1])

	variantIndex := indexOf(args, "--variant")
	require.NotEqual(t, -1, variantIndex)
	require.Less(t, variantIndex+1, len(args))
	assert.Equal(t, "high", args[variantIndex+1])

	assert.NotContains(t, args, defaultModel)
	assert.NotContains(t, args, defaultVariant)
}

func indexOf(slice []string, item string) int {
	for i, s := range slice {
		if s == item {
			return i
		}
	}
	return -1
}
