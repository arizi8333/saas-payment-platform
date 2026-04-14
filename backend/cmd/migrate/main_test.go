package main

import (
	"testing"
)

func TestRepeat(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		count    int
		expected string
	}{
		{
			name:     "single character repeated",
			input:    "-",
			count:    5,
			expected: "-----",
		},
		{
			name:     "zero repetitions",
			input:    "-",
			count:    0,
			expected: "",
		},
		{
			name:     "multi-char string repeated",
			input:    "ab",
			count:    3,
			expected: "ababab",
		},
		{
			name:     "empty string repeated",
			input:    "",
			count:    10,
			expected: "",
		},
		{
			name:     "negative count",
			input:    "-",
			count:    -1,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := repeat(tt.input, tt.count)
			if got != tt.expected {
				t.Errorf("repeat(%q, %d) = %q, want %q", tt.input, tt.count, got, tt.expected)
			}
		})
	}
}

func TestPrintUsage(t *testing.T) {
	// printUsage writes to stderr; verify it doesn't panic.
	printUsage()
}
