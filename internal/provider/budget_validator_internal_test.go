package provider

import (
	"testing"
	"time"
)

func TestValidateBudgetStartPeriod(t *testing.T) {
	tests := []struct {
		name          string
		budgetType    string
		timeInterval  string
		startPeriod   time.Time
		expectedError bool
	}{
		{
			name:          "Fixed budget",
			budgetType:    "fixed",
			timeInterval:  "",
			startPeriod:   time.Now(),
			expectedError: false,
		},
		{
			name:          "Recurring Year Valid",
			budgetType:    "recurring",
			timeInterval:  "year",
			startPeriod:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			expectedError: false,
		},
		{
			name:          "Recurring Year Invalid",
			budgetType:    "recurring",
			timeInterval:  "year",
			startPeriod:   time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
			expectedError: true,
		},
		{
			name:          "Recurring Month Valid",
			budgetType:    "recurring",
			timeInterval:  "month",
			startPeriod:   time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC),
			expectedError: false,
		},
		{
			name:          "Recurring Month Invalid",
			budgetType:    "recurring",
			timeInterval:  "month",
			startPeriod:   time.Date(2025, 2, 1, 1, 0, 0, 0, time.UTC),
			expectedError: true,
		},
		{
			name:          "Recurring Week Valid (Monday)",
			budgetType:    "recurring",
			timeInterval:  "week",
			startPeriod:   time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC), // Dec 1 2025 is Monday
			expectedError: false,
		},
		{
			name:          "Recurring Week Invalid (Tuesday)",
			budgetType:    "recurring",
			timeInterval:  "week",
			startPeriod:   time.Date(2025, 12, 2, 0, 0, 0, 0, time.UTC), // Dec 2 2025 is Tuesday
			expectedError: true,
		},
		{
			name:          "Recurring Day Valid",
			budgetType:    "recurring",
			timeInterval:  "day",
			startPeriod:   time.Date(2025, 12, 3, 0, 0, 0, 0, time.UTC),
			expectedError: false,
		},
		{
			name:          "Recurring Day Invalid",
			budgetType:    "recurring",
			timeInterval:  "day",
			startPeriod:   time.Date(2025, 12, 3, 0, 0, 1, 0, time.UTC),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBudgetStartPeriod(tt.budgetType, tt.timeInterval, tt.startPeriod.UnixMilli())
			if (err != nil) != tt.expectedError {
				t.Errorf("validateBudgetStartPeriod() error = %v, expectedError %v", err, tt.expectedError)
			}
		})
	}
}
