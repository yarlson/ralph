package provider

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalize(t *testing.T) {
	t.Run("defaults to claude", func(t *testing.T) {
		value, err := Normalize("")
		require.NoError(t, err)
		assert.Equal(t, Claude, value)
	})

	t.Run("accepts opencode", func(t *testing.T) {
		value, err := Normalize("OpenCode")
		require.NoError(t, err)
		assert.Equal(t, OpenCode, value)
	})

	t.Run("rejects unknown", func(t *testing.T) {
		_, err := Normalize("unknown")
		assert.Error(t, err)
	})
}

func TestResolve(t *testing.T) {
	t.Run("cli overrides config", func(t *testing.T) {
		value, err := Resolve("opencode", "claude")
		require.NoError(t, err)
		assert.Equal(t, OpenCode, value)
	})

	t.Run("uses config when cli empty", func(t *testing.T) {
		value, err := Resolve("", "opencode")
		require.NoError(t, err)
		assert.Equal(t, OpenCode, value)
	})
}
