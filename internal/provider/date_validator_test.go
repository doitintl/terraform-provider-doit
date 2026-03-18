package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestDateValidator(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		isValid bool
	}{
		// Valid dates
		{"valid date", "2025-01-15", true},
		{"valid leap year", "2024-02-29", true},
		{"valid year start", "2025-01-01", true},
		{"valid year end", "2025-12-31", true},

		// Invalid dates
		{"invalid format slash", "2025/01/15", false},
		{"invalid format dots", "2025.01.15", false},
		{"invalid month", "2025-13-01", false},
		{"invalid day", "2025-01-32", false},
		{"non-leap year feb 29", "2025-02-29", false},
		{"no separators", "20250115", false},
		{"empty string", "", false},
		{"garbage", "hello", false},
		{"timestamp", "2025-01-15T00:00:00Z", false},
	}

	ctx := context.Background()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := validator.StringRequest{
				ConfigValue: types.StringValue(tc.value),
			}
			resp := &validator.StringResponse{}

			v := dateValidator{}
			v.ValidateString(ctx, req, resp)

			if tc.isValid && resp.Diagnostics.HasError() {
				t.Errorf("expected %q to be valid, got error: %s", tc.value, resp.Diagnostics.Errors()[0].Detail())
			}
			if !tc.isValid && !resp.Diagnostics.HasError() {
				t.Errorf("expected %q to be invalid, but got no error", tc.value)
			}
		})
	}
}

func TestDateValidator_NullAndUnknown(t *testing.T) {
	ctx := context.Background()
	v := dateValidator{}

	// Null value should pass without error
	nullReq := validator.StringRequest{ConfigValue: types.StringNull()}
	nullResp := &validator.StringResponse{}
	v.ValidateString(ctx, nullReq, nullResp)
	if nullResp.Diagnostics.HasError() {
		t.Error("expected null value to pass validation")
	}

	// Unknown value should pass without error
	unknownReq := validator.StringRequest{ConfigValue: types.StringUnknown()}
	unknownResp := &validator.StringResponse{}
	v.ValidateString(ctx, unknownReq, unknownResp)
	if unknownResp.Diagnostics.HasError() {
		t.Error("expected unknown value to pass validation")
	}
}
