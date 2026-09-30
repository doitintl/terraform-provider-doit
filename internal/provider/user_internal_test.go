package provider

import (
	"strings"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestUserResource_PhoneValidators verifies that the generated regex validators
// for phone and phone_extension provide informative diagnostic messages detailing
// the required regular expression pattern when validation fails.
func TestUserResource_PhoneValidators(t *testing.T) {
	t.Parallel()

	r := NewUserResource()
	var resp resource.SchemaResponse
	r.Schema(t.Context(), resource.SchemaRequest{}, &resp)

	phoneAttr, ok := resp.Schema.Attributes["phone"].(schema.StringAttribute)
	if !ok {
		t.Fatalf("expected phone attribute to be schema.StringAttribute")
	}

	phoneExtAttr, ok := resp.Schema.Attributes["phone_extension"].(schema.StringAttribute)
	if !ok {
		t.Fatalf("expected phone_extension attribute to be schema.StringAttribute")
	}

	t.Run("phone validator detail includes regex pattern", func(t *testing.T) {
		t.Parallel()
		if len(phoneAttr.Validators) == 0 {
			t.Fatalf("expected validators on phone attribute")
		}

		for _, v := range phoneAttr.Validators {
			var vResp validator.StringResponse
			v.ValidateString(t.Context(), validator.StringRequest{
				Path:        path.Root("phone"),
				ConfigValue: types.StringValue("invalid-phone"),
			}, &vResp)

			if !vResp.Diagnostics.HasError() {
				t.Fatalf("expected error for invalid phone, got none")
			}
			err := vResp.Diagnostics.Errors()[0]
			if !strings.Contains(err.Detail(), `^\+[0-9]+$`) {
				t.Errorf("expected diagnostic detail to include regex pattern, got: %s", err.Detail())
			}
		}
	})

	t.Run("phone extension validator detail includes regex pattern", func(t *testing.T) {
		t.Parallel()
		if len(phoneExtAttr.Validators) == 0 {
			t.Fatalf("expected validators on phone_extension attribute")
		}

		for _, v := range phoneExtAttr.Validators {
			var vResp validator.StringResponse
			v.ValidateString(t.Context(), validator.StringRequest{
				Path:        path.Root("phone_extension"),
				ConfigValue: types.StringValue("12345"), // 5 digits, rejects 8-15
			}, &vResp)

			if !vResp.Diagnostics.HasError() {
				t.Fatalf("expected error for invalid phone_extension, got none")
			}
			err := vResp.Diagnostics.Errors()[0]
			if !strings.Contains(err.Detail(), `^[0-9]{8,15}$`) {
				t.Errorf("expected diagnostic detail to include regex pattern, got: %s", err.Detail())
			}
		}
	})

	t.Run("valid phone and extension pass validation", func(t *testing.T) {
		t.Parallel()

		for _, v := range phoneAttr.Validators {
			var vResp validator.StringResponse
			v.ValidateString(t.Context(), validator.StringRequest{
				Path:        path.Root("phone"),
				ConfigValue: types.StringValue("+44"),
			}, &vResp)

			if vResp.Diagnostics.HasError() {
				t.Errorf("expected valid phone to pass, got: %s", vResp.Diagnostics.Errors()[0].Detail())
			}
		}

		for _, v := range phoneExtAttr.Validators {
			var vResp validator.StringResponse
			v.ValidateString(t.Context(), validator.StringRequest{
				Path:        path.Root("phone_extension"),
				ConfigValue: types.StringValue("5551234567"),
			}, &vResp)

			if vResp.Diagnostics.HasError() {
				t.Errorf("expected valid phone_extension to pass, got: %s", vResp.Diagnostics.Errors()[0].Detail())
			}
		}
	})
}

// TestOverlayUserComputedFields_PhoneAndLanguage verifies that Optional+Computed
// fields (phone, phone_extension, language) adopt the resolved API values when
// unknown (e.g. omitted by user on create), but preserve user-specified values
// when known.
func TestOverlayUserComputedFields_PhoneAndLanguage(t *testing.T) {
	t.Parallel()

	phone := "+44"
	phoneExt := "5551234567"
	lang := models.UserListItemLanguageEn

	user := &models.UserListItem{
		Phone:          &phone,
		PhoneExtension: &phoneExt,
		Language:       &lang,
	}

	t.Run("overlay when plan fields are unknown", func(t *testing.T) {
		t.Parallel()
		plan := userResourceModel{
			Phone:          types.StringUnknown(),
			PhoneExtension: types.StringUnknown(),
			Language:       types.StringUnknown(),
		}

		overlayUserComputedFields(user, &plan)

		if plan.Phone.ValueString() != "+44" {
			t.Errorf("got phone %q, want %q", plan.Phone.ValueString(), "+44")
		}
		if plan.PhoneExtension.ValueString() != "5551234567" {
			t.Errorf("got phone_extension %q, want %q", plan.PhoneExtension.ValueString(), "5551234567")
		}
		if plan.Language.ValueString() != "en" {
			t.Errorf("got language %q, want %q", plan.Language.ValueString(), "en")
		}
	})

	t.Run("preserve plan when fields are explicitly set (known)", func(t *testing.T) {
		t.Parallel()
		plan := userResourceModel{
			Phone:          types.StringValue("+1"),
			PhoneExtension: types.StringValue("9876543210"),
			Language:       types.StringValue("ja"),
		}

		overlayUserComputedFields(user, &plan)

		if plan.Phone.ValueString() != "+1" {
			t.Errorf("got phone %q, want %q", plan.Phone.ValueString(), "+1")
		}
		if plan.PhoneExtension.ValueString() != "9876543210" {
			t.Errorf("got phone_extension %q, want %q", plan.PhoneExtension.ValueString(), "9876543210")
		}
		if plan.Language.ValueString() != "ja" {
			t.Errorf("got language %q, want %q", plan.Language.ValueString(), "ja")
		}
	})
}
