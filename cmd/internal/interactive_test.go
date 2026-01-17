package internal

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsInteractive_WithFile(t *testing.T) {
	// Create a temp file (not a TTY)
	f, err := os.CreateTemp("", "test-interactive-*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer func() { _ = os.Remove(f.Name()) }()
	defer func() { _ = f.Close() }()

	// A regular file should not be interactive
	result := IsInteractive(f.Fd())
	assert.False(t, result, "a regular file should not be interactive")
}

func TestIsInteractive_WithInvalidFd(t *testing.T) {
	// An invalid file descriptor should return false
	result := IsInteractive(^uintptr(0)) // Invalid fd
	assert.False(t, result, "an invalid fd should not be interactive")
}

func TestIsInteractive_WithDevNull(t *testing.T) {
	// /dev/null is not a terminal
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Skip("could not open /dev/null")
	}
	defer func() { _ = f.Close() }()

	result := IsInteractive(f.Fd())
	assert.False(t, result, "/dev/null should not be interactive")
}
