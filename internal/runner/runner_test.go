package runner

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yarlson/ralph/internal/config"
	"github.com/yarlson/ralph/internal/taskstore"
)

func TestRun_WritesProgressOutput(t *testing.T) {
	workDir := t.TempDir()

	originalDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(workDir))
	defer func() { _ = os.Chdir(originalDir) }()

	runCmd(t, workDir, "git", "init")
	runCmd(t, workDir, "git", "config", "user.email", "test@example.com")
	runCmd(t, workDir, "git", "config", "user.name", "Test User")
	runCmd(t, workDir, "git", "config", "commit.gpgsign", "false")

	mockClaude := filepath.Join(workDir, "mock-claude.sh")
	script := `#!/bin/bash
echo '{"type":"system","subtype":"init","session_id":"test-session","model":"test-model"}'
echo '{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"working"}]}}'
echo "change" >> output.txt
echo '{"type":"result","subtype":"success","result":"done","total_cost_usd":0.0100,"usage":{"input_tokens":1,"output_tokens":1}}'
`
	require.NoError(t, os.WriteFile(mockClaude, []byte(script), 0755))

	cfg, err := config.LoadConfig(workDir)
	require.NoError(t, err)
	cfg.Claude.Command = []string{mockClaude}
	cfg.Claude.Args = nil

	tasksPath := filepath.Join(workDir, config.DefaultTasksPath)
	store, err := taskstore.NewLocalStore(tasksPath)
	require.NoError(t, err)

	now := time.Now().Truncate(time.Second)
	parent := &taskstore.Task{
		ID:        "parent-task",
		Title:     "Parent Task",
		Status:    taskstore.StatusOpen,
		CreatedAt: now,
		UpdatedAt: now,
	}
	child := &taskstore.Task{
		ID:        "child-task",
		Title:     "Child Task",
		ParentID:  &parent.ID,
		Status:    taskstore.StatusOpen,
		Verify:    [][]string{{"echo", "ok"}},
		CreatedAt: now,
		UpdatedAt: now,
	}

	require.NoError(t, store.Save(parent))
	require.NoError(t, store.Save(child))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	opts := Options{Once: true}

	err = Run(context.Background(), workDir, cfg, parent.ID, opts, &stdout, &stderr)
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "▶ Task: Child Task")
	assert.Contains(t, output, "⏳ Invoking agent")
	assert.Contains(t, output, "📝 Committed:")
}

func runCmd(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE=Thu, 07 Apr 2005 22:13:13 +0200", "GIT_COMMITTER_DATE=Thu, 07 Apr 2005 22:13:13 +0200")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command failed: %s %v\n%s", name, args, string(output))
	}
}
