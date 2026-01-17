package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProgressBar_ZeroPercent(t *testing.T) {
	result := ProgressBar(0, 20)
	assert.Equal(t, "[                    ]", result)
	assert.Len(t, result, 22) // 20 width + 2 brackets
}

func TestProgressBar_HundredPercent(t *testing.T) {
	result := ProgressBar(100, 20)
	assert.Equal(t, "[====================]", result)
	assert.Len(t, result, 22)
}

func TestProgressBar_FiftyPercent(t *testing.T) {
	result := ProgressBar(50, 20)
	assert.Equal(t, "[==========          ]", result)
	assert.Len(t, result, 22)
}

func TestProgressBar_TwentyFivePercent(t *testing.T) {
	result := ProgressBar(25, 20)
	assert.Equal(t, "[=====               ]", result)
	assert.Len(t, result, 22)
}

func TestProgressBar_SeventyFivePercent(t *testing.T) {
	result := ProgressBar(75, 20)
	assert.Equal(t, "[===============     ]", result)
	assert.Len(t, result, 22)
}

func TestProgressBar_DifferentWidths(t *testing.T) {
	tests := []struct {
		name     string
		percent  int
		width    int
		expected string
	}{
		{"10 width at 50%", 50, 10, "[=====     ]"},
		{"40 width at 25%", 25, 40, "[==========                              ]"},
		{"5 width at 100%", 100, 5, "[=====]"},
		{"5 width at 0%", 0, 5, "[     ]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ProgressBar(tt.percent, tt.width)
			assert.Equal(t, tt.expected, result)
			assert.Len(t, result, tt.width+2) // width + 2 brackets
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
		{"percent over 100 clamped", 150, 10, "[==========]"},
		{"negative percent clamped", -10, 10, "[          ]"},
		{"one percent rounds down", 1, 10, "[          ]"},
		{"99 percent", 99, 10, "[========= ]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ProgressBar(tt.percent, tt.width)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProgressBar_ZeroWidth(t *testing.T) {
	// Zero width should return just brackets
	result := ProgressBar(50, 0)
	assert.Equal(t, "[]", result)
}

func TestProgressBar_SmallWidth(t *testing.T) {
	result := ProgressBar(50, 2)
	assert.Equal(t, "[= ]", result)
}
