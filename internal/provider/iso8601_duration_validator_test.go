package provider

import (
	"testing"
)

func TestIsValidISO8601DateDuration(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		// Valid durations
		{"P7D", true},
		{"P1M", true},
		{"P1Y", true},
		{"P1Y2M3D", true},
		{"P1Y2M", true},
		{"P2M3D", true},
		{"P365D", true},
		{"P12M", true},
		{"P100Y", true},

		// Invalid durations
		{"", false},
		{"P", false},         // No components
		{"7D", false},        // Missing P prefix
		{"P1H", false},       // H is not a date designator
		{"PT1H", false},      // Time components not allowed
		{"P1Y2M3D4H", false}, // Mixed
		{"P1M2Y", false},     // Wrong order
		{"PD", false},        // Missing number
		{"P1D2D", false},     // Duplicate designator
		{"hello", false},     // Garbage
		{"P-1D", false},      // Negative
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := isValidISO8601DateDuration(tc.input)
			if got != tc.valid {
				t.Errorf("isValidISO8601DateDuration(%q) = %v, want %v", tc.input, got, tc.valid)
			}
		})
	}
}
