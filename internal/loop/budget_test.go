package loop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBudgetLimits_Defaults(t *testing.T) {
	limits := DefaultBudgetLimits()

	assert.Equal(t, 50, limits.MaxIterations)
	assert.Equal(t, 0, limits.MaxTimeMinutes)          // 0 means unlimited
	assert.Equal(t, float64(0), limits.MaxCostUSD)     // 0 means unlimited
	assert.Equal(t, 20, limits.MaxMinutesPerIteration) // default per-iteration timeout
}

func TestBudgetLimits_AllFields(t *testing.T) {
	limits := BudgetLimits{
		MaxIterations:          100,
		MaxTimeMinutes:         60,
		MaxCostUSD:             50.0,
		MaxMinutesPerIteration: 30,
	}

	assert.Equal(t, 100, limits.MaxIterations)
	assert.Equal(t, 60, limits.MaxTimeMinutes)
	assert.Equal(t, 50.0, limits.MaxCostUSD)
	assert.Equal(t, 30, limits.MaxMinutesPerIteration)
}

func TestBudgetState_Defaults(t *testing.T) {
	state := &BudgetState{}

	assert.Equal(t, 0, state.Iterations)
	assert.Equal(t, float64(0), state.TotalCostUSD)
	assert.True(t, state.StartTime.IsZero())
}

func TestBudgetState_JSONSerialization(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	state := &BudgetState{
		Iterations:   5,
		TotalCostUSD: 1.23,
		StartTime:    now,
	}

	data, err := json.Marshal(state)
	require.NoError(t, err)

	var restored BudgetState
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	assert.Equal(t, state.Iterations, restored.Iterations)
	assert.Equal(t, state.TotalCostUSD, restored.TotalCostUSD)
	assert.Equal(t, state.StartTime.Unix(), restored.StartTime.Unix())
}

func TestBudgetTracker_NewTracker(t *testing.T) {
	limits := DefaultBudgetLimits()
	tracker := NewBudgetTracker(limits)

	assert.NotNil(t, tracker)
	assert.Equal(t, limits, tracker.limits)
	assert.Equal(t, 0, tracker.state.Iterations)
	assert.Equal(t, float64(0), tracker.state.TotalCostUSD)
}

func TestBudgetTracker_RecordIteration(t *testing.T) {
	limits := DefaultBudgetLimits()
	tracker := NewBudgetTracker(limits)

	// Record first iteration
	tracker.RecordIteration(0.5)
	assert.Equal(t, 1, tracker.state.Iterations)
	assert.Equal(t, 0.5, tracker.state.TotalCostUSD)

	// Record second iteration
	tracker.RecordIteration(0.75)
	assert.Equal(t, 2, tracker.state.Iterations)
	assert.Equal(t, 1.25, tracker.state.TotalCostUSD)
}

func TestBudgetTracker_CheckBudget_UnderAllLimits(t *testing.T) {
	limits := BudgetLimits{
		MaxIterations:  10,
		MaxTimeMinutes: 60,
		MaxCostUSD:     100.0,
	}
	tracker := NewBudgetTracker(limits)
	tracker.state.Iterations = 5
	tracker.state.TotalCostUSD = 50.0
	tracker.state.StartTime = time.Now()

	status := tracker.CheckBudget()

	assert.True(t, status.CanContinue)
	assert.Empty(t, status.Reason)
	assert.Equal(t, BudgetReasonNone, status.ReasonCode)
}

func TestBudgetTracker_CheckBudget_IterationsExceeded(t *testing.T) {
	limits := BudgetLimits{
		MaxIterations: 5,
	}
	tracker := NewBudgetTracker(limits)
	tracker.state.Iterations = 5

	status := tracker.CheckBudget()

	assert.False(t, status.CanContinue)
	assert.Contains(t, status.Reason, "iteration")
	assert.Equal(t, BudgetReasonIterations, status.ReasonCode)
}

func TestBudgetTracker_CheckBudget_TimeExceeded(t *testing.T) {
	limits := BudgetLimits{
		MaxIterations:  100,
		MaxTimeMinutes: 1,
	}
	tracker := NewBudgetTracker(limits)
	tracker.state.Iterations = 1
	tracker.state.StartTime = time.Now().Add(-2 * time.Minute) // Started 2 minutes ago

	status := tracker.CheckBudget()

	assert.False(t, status.CanContinue)
	assert.Contains(t, status.Reason, "time")
	assert.Equal(t, BudgetReasonTime, status.ReasonCode)
}

func TestBudgetTracker_CheckBudget_CostExceeded(t *testing.T) {
	limits := BudgetLimits{
		MaxIterations: 100,
		MaxCostUSD:    10.0,
	}
	tracker := NewBudgetTracker(limits)
	tracker.state.Iterations = 5
	tracker.state.TotalCostUSD = 15.0

	status := tracker.CheckBudget()

	assert.False(t, status.CanContinue)
	assert.Contains(t, status.Reason, "cost")
	assert.Equal(t, BudgetReasonCost, status.ReasonCode)
}

func TestBudgetTracker_CheckBudget_ZeroLimits_Unlimited(t *testing.T) {
	// Zero values mean unlimited
	limits := BudgetLimits{
		MaxIterations:  0, // unlimited
		MaxTimeMinutes: 0, // unlimited
		MaxCostUSD:     0, // unlimited
	}
	tracker := NewBudgetTracker(limits)
	tracker.state.Iterations = 1000
	tracker.state.TotalCostUSD = 10000.0
	tracker.state.StartTime = time.Now().Add(-24 * time.Hour)

	status := tracker.CheckBudget()

	assert.True(t, status.CanContinue)
	assert.Empty(t, status.Reason)
}

func TestBudgetTracker_GetState(t *testing.T) {
	limits := DefaultBudgetLimits()
	tracker := NewBudgetTracker(limits)
	tracker.RecordIteration(1.0)
	tracker.RecordIteration(2.0)

	state := tracker.GetState()

	assert.Equal(t, 2, state.Iterations)
	assert.Equal(t, 3.0, state.TotalCostUSD)
}

func TestBudgetTracker_SetState(t *testing.T) {
	limits := DefaultBudgetLimits()
	tracker := NewBudgetTracker(limits)

	now := time.Now()
	state := BudgetState{
		Iterations:   10,
		TotalCostUSD: 25.0,
		StartTime:    now,
	}
	tracker.SetState(state)

	assert.Equal(t, 10, tracker.state.Iterations)
	assert.Equal(t, 25.0, tracker.state.TotalCostUSD)
	assert.Equal(t, now, tracker.state.StartTime)
}

func TestBudgetTracker_Reset(t *testing.T) {
	limits := DefaultBudgetLimits()
	tracker := NewBudgetTracker(limits)
	tracker.RecordIteration(5.0)
	tracker.RecordIteration(5.0)

	tracker.Reset()

	assert.Equal(t, 0, tracker.state.Iterations)
	assert.Equal(t, float64(0), tracker.state.TotalCostUSD)
	assert.False(t, tracker.state.StartTime.IsZero()) // Start time should be reset to now
}

func TestBudgetTracker_ElapsedTime(t *testing.T) {
	limits := DefaultBudgetLimits()
	tracker := NewBudgetTracker(limits)
	tracker.state.StartTime = time.Now().Add(-5 * time.Minute)

	elapsed := tracker.ElapsedTime()

	// Allow some tolerance for test execution time
	assert.GreaterOrEqual(t, elapsed.Minutes(), float64(4))
	assert.LessOrEqual(t, elapsed.Minutes(), float64(6))
}

func TestBudgetTracker_ElapsedTime_NotStarted(t *testing.T) {
	limits := DefaultBudgetLimits()
	tracker := NewBudgetTracker(limits)

	elapsed := tracker.ElapsedTime()

	assert.Equal(t, time.Duration(0), elapsed)
}

func TestSaveBudget(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "budget.json")

	now := time.Now().Truncate(time.Second)
	state := &BudgetState{
		Iterations:   7,
		TotalCostUSD: 12.34,
		StartTime:    now,
	}

	err := SaveBudget(path, state)
	require.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(path)
	require.NoError(t, err)

	// Load and verify content
	loaded, err := LoadBudget(path)
	require.NoError(t, err)
	assert.Equal(t, state.Iterations, loaded.Iterations)
	assert.Equal(t, state.TotalCostUSD, loaded.TotalCostUSD)
	assert.Equal(t, state.StartTime.Unix(), loaded.StartTime.Unix())
}

func TestSaveBudget_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subdir", "budget.json")

	state := &BudgetState{
		Iterations: 1,
	}

	err := SaveBudget(path, state)
	require.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(path)
	require.NoError(t, err)
}

func TestLoadBudget_FileNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nonexistent.json")

	state, err := LoadBudget(path)
	require.NoError(t, err) // Should return empty state, not error
	assert.Equal(t, 0, state.Iterations)
	assert.Equal(t, float64(0), state.TotalCostUSD)
	assert.True(t, state.StartTime.IsZero())
}

func TestLoadBudget_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "invalid.json")

	err := os.WriteFile(path, []byte("not json"), 0644)
	require.NoError(t, err)

	_, err = LoadBudget(path)
	assert.Error(t, err)
}

func TestBudgetStatus_Fields(t *testing.T) {
	status := BudgetStatus{
		CanContinue: false,
		Reason:      "max iterations reached",
		ReasonCode:  BudgetReasonIterations,
	}

	assert.False(t, status.CanContinue)
	assert.Equal(t, "max iterations reached", status.Reason)
	assert.Equal(t, BudgetReasonIterations, status.ReasonCode)
}

func TestBudgetReasonCode_Values(t *testing.T) {
	assert.Equal(t, BudgetReasonCode("none"), BudgetReasonNone)
	assert.Equal(t, BudgetReasonCode("iterations"), BudgetReasonIterations)
	assert.Equal(t, BudgetReasonCode("time"), BudgetReasonTime)
	assert.Equal(t, BudgetReasonCode("cost"), BudgetReasonCost)
}

func TestBudgetTracker_EnsureStartTime(t *testing.T) {
	limits := DefaultBudgetLimits()
	tracker := NewBudgetTracker(limits)

	// Start time should be zero initially
	assert.True(t, tracker.state.StartTime.IsZero())

	// First record should set start time
	tracker.RecordIteration(1.0)
	assert.False(t, tracker.state.StartTime.IsZero())

	startTime := tracker.state.StartTime

	// Subsequent records should not change start time
	time.Sleep(10 * time.Millisecond)
	tracker.RecordIteration(1.0)
	assert.Equal(t, startTime, tracker.state.StartTime)
}

func TestBudgetTracker_CheckBudget_PriorityOrder(t *testing.T) {
	// When multiple limits are exceeded, iterations should be checked first
	limits := BudgetLimits{
		MaxIterations:  5,
		MaxTimeMinutes: 1,
		MaxCostUSD:     1.0,
	}
	tracker := NewBudgetTracker(limits)
	tracker.state.Iterations = 10        // Exceeded
	tracker.state.TotalCostUSD = 100.0   // Exceeded
	tracker.state.StartTime = time.Now() // Not exceeded

	status := tracker.CheckBudget()

	assert.False(t, status.CanContinue)
	assert.Equal(t, BudgetReasonIterations, status.ReasonCode)
}

func TestSaveBudget_NilState(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "budget.json")

	err := SaveBudget(path, nil)
	assert.Error(t, err)
}
