package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestCustomerURLSlugValidator(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		isValid bool
	}{
		// Valid cases
		{"valid 3 chars min", "abc", true},
		{"valid 3 chars with hyphen", "a-b", true},
		{"valid 3 chars numbers", "123", true},
		{"valid 3 chars mixed", "a1b", true},
		{"valid intermediate length", "my-company", true},
		{"valid intermediate alphanumeric", "acme-corp-1", true},
		{"valid 12 chars max", "abcdefghijkl", true},
		{"valid 12 chars with hyphens", "a-b-c-d-e-12", true},
		{"valid empty string (clears slug)", "", true},

		// Invalid cases - length
		{"invalid too short 1 char", "a", false},
		{"invalid too short 2 chars", "ab", false},
		{"invalid too short 2 numbers", "12", false},
		{"invalid too long 13 chars", "abcdefghijklm", false},
		{"invalid too long description", "my-very-long-company-slug", false},

		// Invalid cases - character set
		{"invalid uppercase first", "Abc", false},
		{"invalid uppercase all", "ABCDE", false},
		{"invalid uppercase middle", "abcDef", false},
		{"invalid underscore", "abc_def", false},
		{"invalid dot", "abc.def", false},
		{"invalid space", "abc def", false},
		{"invalid symbol", "abc@def", false},

		// Invalid cases - start / end
		{"invalid leading hyphen", "-abcdef", false},
		{"invalid trailing hyphen", "abcdef-", false},
		{"invalid leading and trailing hyphen", "-abcdef-", false},
		{"invalid only hyphens 3 chars", "---", false},
	}

	ctx := context.Background()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := validator.StringRequest{
				ConfigValue: types.StringValue(tc.value),
			}
			resp := &validator.StringResponse{}

			v := customerURLSlug()
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

func TestCustomerURLSlugValidator_NullAndUnknown(t *testing.T) {
	ctx := context.Background()
	v := customerURLSlug()

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
