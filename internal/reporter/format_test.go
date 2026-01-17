package reporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProgressBar_ZeroPercent(t *testing.T) {
	result := ProgressBar(0, 20)
	assert.Equal(t, "[                    ]", result)
}

func TestProgressBar_HundredPercent(t *testing.T) {
	result := ProgressBar(100, 20)
	assert.Equal(t, "[====================]", result)
}

func TestProgressBar_FiftyPercent(t *testing.T) {
	result := ProgressBar(50, 20)
	assert.Equal(t, "[==========          ]", result)
}

func TestProgressBar_TwentyFivePercent(t *testing.T) {
	result := ProgressBar(25, 20)
	assert.Equal(t, "[=====               ]", result)
}

func TestProgressBar_SeventyFivePercent(t *testing.T) {
	result := ProgressBar(75, 20)
	assert.Equal(t, "[===============     ]", result)
}

func TestProgressBar_DifferentWidths(t *testing.T) {
	tests := []struct {
		percent  int
		width    int
		expected string
	}{
		{50, 10, "[=====     ]"},
		{50, 4, "[==  ]"},
		{100, 5, "[=====]"},
		{0, 5, "[     ]"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := ProgressBar(tt.percent, tt.width)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProgressBar_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		percent  int
		width    int
		expected string
	}{
		{"negative percent clamped to 0", -10, 10, "[          ]"},
		{"over 100 clamped to 100", 150, 10, "[==========]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ProgressBar(tt.percent, tt.width)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProgressBar_ZeroWidth(t *testing.T) {
	// Zero width should still produce valid output
	result := ProgressBar(50, 0)
	assert.Equal(t, "[]", result)
}

func TestProgressBar_SmallWidth(t *testing.T) {
	result := ProgressBar(50, 2)
	assert.Equal(t, "[= ]", result)
}
