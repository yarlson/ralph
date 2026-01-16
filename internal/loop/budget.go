package loop

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// BudgetReasonCode identifies why a budget check failed.
type BudgetReasonCode string

const (
	// BudgetReasonNone indicates no budget limit was exceeded.
	BudgetReasonNone BudgetReasonCode = "none"
	// BudgetReasonIterations indicates the iteration limit was exceeded.
	BudgetReasonIterations BudgetReasonCode = "iterations"
	// BudgetReasonTime indicates the time limit was exceeded.
	BudgetReasonTime BudgetReasonCode = "time"
	// BudgetReasonCost indicates the cost limit was exceeded.
	BudgetReasonCost BudgetReasonCode = "cost"
)

// BudgetLimits defines the configurable limits for budget tracking.
type BudgetLimits struct {
	// MaxIterations is the maximum number of iterations allowed (0 = unlimited).
	MaxIterations int `json:"max_iterations"`

	// MaxTimeMinutes is the maximum total time in minutes (0 = unlimited).
	MaxTimeMinutes int `json:"max_time_minutes"`

	// MaxCostUSD is the maximum total cost in USD (0 = unlimited).
	MaxCostUSD float64 `json:"max_cost_usd"`

	// MaxMinutesPerIteration is the maximum time per iteration in minutes.
	MaxMinutesPerIteration int `json:"max_minutes_per_iteration"`
}

// BudgetState tracks the current budget consumption.
type BudgetState struct {
	// Iterations is the number of iterations completed.
	Iterations int `json:"iterations"`

	// TotalCostUSD is the total cost incurred so far.
	TotalCostUSD float64 `json:"total_cost_usd"`

	// StartTime is when the budget tracking started.
	StartTime time.Time `json:"start_time"`
}

// BudgetStatus represents the result of a budget check.
type BudgetStatus struct {
	// CanContinue indicates whether the loop can continue.
	CanContinue bool

	// Reason is a human-readable explanation if CanContinue is false.
	Reason string

	// ReasonCode identifies the specific budget limit that was exceeded.
	ReasonCode BudgetReasonCode
}

// BudgetTracker tracks budget consumption and enforces limits.
type BudgetTracker struct {
	limits BudgetLimits
	state  BudgetState
}

// DefaultBudgetLimits returns sensible default budget limits.
func DefaultBudgetLimits() BudgetLimits {
	return BudgetLimits{
		MaxIterations:          50,
		MaxTimeMinutes:         0,  // unlimited
		MaxCostUSD:             0,  // unlimited
		MaxMinutesPerIteration: 20, // 20 minutes per iteration
	}
}

// NewBudgetTracker creates a new budget tracker with the given limits.
func NewBudgetTracker(limits BudgetLimits) *BudgetTracker {
	return &BudgetTracker{
		limits: limits,
		state:  BudgetState{},
	}
}

// RecordIteration records a completed iteration with its cost.
func (bt *BudgetTracker) RecordIteration(costUSD float64) {
	// Set start time on first iteration
	if bt.state.StartTime.IsZero() {
		bt.state.StartTime = time.Now()
	}

	bt.state.Iterations++
	bt.state.TotalCostUSD += costUSD
}

// CheckBudget checks if the current budget consumption is within limits.
// Returns a BudgetStatus indicating whether the loop can continue.
func (bt *BudgetTracker) CheckBudget() BudgetStatus {
	// Check iteration limit
	if bt.limits.MaxIterations > 0 && bt.state.Iterations >= bt.limits.MaxIterations {
		return BudgetStatus{
			CanContinue: false,
			Reason:      fmt.Sprintf("max iteration limit reached (%d/%d)", bt.state.Iterations, bt.limits.MaxIterations),
			ReasonCode:  BudgetReasonIterations,
		}
	}

	// Check time limit
	if bt.limits.MaxTimeMinutes > 0 && !bt.state.StartTime.IsZero() {
		elapsed := time.Since(bt.state.StartTime)
		maxDuration := time.Duration(bt.limits.MaxTimeMinutes) * time.Minute
		if elapsed >= maxDuration {
			return BudgetStatus{
				CanContinue: false,
				Reason:      fmt.Sprintf("max time limit exceeded (%.1f/%.1f minutes)", elapsed.Minutes(), float64(bt.limits.MaxTimeMinutes)),
				ReasonCode:  BudgetReasonTime,
			}
		}
	}

	// Check cost limit
	if bt.limits.MaxCostUSD > 0 && bt.state.TotalCostUSD >= bt.limits.MaxCostUSD {
		return BudgetStatus{
			CanContinue: false,
			Reason:      fmt.Sprintf("max cost limit exceeded ($%.2f/$%.2f)", bt.state.TotalCostUSD, bt.limits.MaxCostUSD),
			ReasonCode:  BudgetReasonCost,
		}
	}

	return BudgetStatus{
		CanContinue: true,
		Reason:      "",
		ReasonCode:  BudgetReasonNone,
	}
}

// GetState returns a copy of the current budget state.
func (bt *BudgetTracker) GetState() BudgetState {
	return bt.state
}

// SetState sets the budget state (used for loading persisted state).
func (bt *BudgetTracker) SetState(state BudgetState) {
	bt.state = state
}

// Reset resets the budget state for a new run.
func (bt *BudgetTracker) Reset() {
	bt.state = BudgetState{
		StartTime: time.Now(),
	}
}

// ElapsedTime returns the time elapsed since the budget tracking started.
func (bt *BudgetTracker) ElapsedTime() time.Duration {
	if bt.state.StartTime.IsZero() {
		return 0
	}
	return time.Since(bt.state.StartTime)
}

// SaveBudget saves the budget state to a file.
func SaveBudget(path string, state *BudgetState) error {
	if state == nil {
		return errors.New("state cannot be nil")
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal budget state: %w", err)
	}

	// Write to file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write budget state: %w", err)
	}

	return nil
}

// LoadBudget loads the budget state from a file.
// If the file does not exist, returns an empty state (not an error).
func LoadBudget(path string) (*BudgetState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Return empty state if file doesn't exist
			return &BudgetState{}, nil
		}
		return nil, fmt.Errorf("failed to read budget state: %w", err)
	}

	var state BudgetState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal budget state: %w", err)
	}

	return &state, nil
}
