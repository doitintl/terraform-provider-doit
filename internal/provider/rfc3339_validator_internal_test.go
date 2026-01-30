package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestRfc3339Validator(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		expectError bool
		errorMatch  string
	}{
		{
			name:        "valid - standard UTC timestamp",
			value:       "2024-06-15T12:00:00Z",
			expectError: false,
		},
		{
			name:        "valid - timestamp with positive offset",
			value:       "2024-06-15T12:00:00+02:00",
			expectError: false,
		},
		{
			name:        "valid - timestamp with negative offset",
			value:       "2024-06-15T12:00:00-08:00",
			expectError: false,
		},
		{
			name:        "valid - with milliseconds",
			value:       "2024-06-15T12:00:00.123Z",
			expectError: false,
		},
		{
			name:        "invalid - date only",
			value:       "2024-06-15",
			expectError: true,
			errorMatch:  "Invalid RFC3339 Timestamp",
		},
		{
			name:        "invalid - time only",
			value:       "12:00:00",
			expectError: true,
			errorMatch:  "Invalid RFC3339 Timestamp",
		},
		{
			name:        "invalid - no timezone",
			value:       "2024-06-15T12:00:00",
			expectError: true,
			errorMatch:  "Invalid RFC3339 Timestamp",
		},
		{
			name:        "invalid - garbage",
			value:       "not-a-date",
			expectError: true,
			errorMatch:  "Invalid RFC3339 Timestamp",
		},
		{
			name:        "invalid - empty string",
			value:       "",
			expectError: true,
			errorMatch:  "Invalid RFC3339 Timestamp",
		},
		{
			name:        "invalid - ISO 8601 without T separator",
			value:       "2024-06-15 12:00:00Z",
			expectError: true,
			errorMatch:  "Invalid RFC3339 Timestamp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := rfc3339Validator{}
			ctx := context.Background()

			req := validator.StringRequest{
				Path:        path.Root("timestamp"),
				ConfigValue: types.StringValue(tt.value),
			}
			resp := &validator.StringResponse{}

			v.ValidateString(ctx, req, resp)

			hasError := resp.Diagnostics.HasError()
			if hasError != tt.expectError {
				t.Errorf("expected error=%v, got error=%v, diagnostics: %v", tt.expectError, hasError, resp.Diagnostics)
			}

			if tt.expectError && tt.errorMatch != "" {
				found := false
				for _, diag := range resp.Diagnostics {
					if diag.Summary() == tt.errorMatch {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error summary %q, got: %v", tt.errorMatch, resp.Diagnostics)
				}
			}
		})
	}
}

func TestRfc3339Validator_NullAndUnknown(t *testing.T) {
	v := rfc3339Validator{}
	ctx := context.Background()

	t.Run("null value skips validation", func(t *testing.T) {
		req := validator.StringRequest{
			Path:        path.Root("timestamp"),
			ConfigValue: types.StringNull(),
		}
		resp := &validator.StringResponse{}
		v.ValidateString(ctx, req, resp)

		if resp.Diagnostics.HasError() {
			t.Errorf("expected no error for null value, got: %v", resp.Diagnostics)
		}
	})

	t.Run("unknown value skips validation", func(t *testing.T) {
		req := validator.StringRequest{
			Path:        path.Root("timestamp"),
			ConfigValue: types.StringUnknown(),
		}
		resp := &validator.StringResponse{}
		v.ValidateString(ctx, req, resp)

		if resp.Diagnostics.HasError() {
			t.Errorf("expected no error for unknown value, got: %v", resp.Diagnostics)
		}
	})
}
