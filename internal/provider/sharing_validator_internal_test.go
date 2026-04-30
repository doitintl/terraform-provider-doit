package provider

import (
	"context"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_sharing"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// buildSharingConfig constructs a tfsdk.Config with a populated permissions list.
// Each entry is (user, role) where role="" means unknown.
func buildSharingConfig(ctx context.Context, t *testing.T, perms []struct{ user, role string }) tfsdk.Config {
	t.Helper()
	schema := resource_sharing.SharingResourceSchema(ctx)

	// Build the framework-level list elements.
	elems := make([]attr.Value, len(perms))
	for i, p := range perms {
		user := types.StringValue(p.user)
		var role attr.Value
		if p.role == "" {
			role = types.StringUnknown()
		} else {
			role = types.StringValue(p.role)
		}
		pv, diags := resource_sharing.NewPermissionsValue(
			resource_sharing.PermissionsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"user": user,
				"role": role,
			},
		)
		if diags.HasError() {
			t.Fatalf("NewPermissionsValue: %v", diags)
		}
		elems[i] = pv
	}

	permsList, listDiags := types.ListValueFrom(
		ctx,
		resource_sharing.PermissionsValue{}.Type(ctx),
		elems,
	)
	if listDiags.HasError() {
		t.Fatalf("ListValueFrom: %v", listDiags)
	}

	// Build a tftypes.Value matching the schema with all attributes null except permissions.
	schemaType := schema.Type().TerraformType(ctx)
	objType, ok := schemaType.(tftypes.Object)
	if !ok {
		t.Fatalf("expected schema to be tftypes.Object, got %T", schemaType)
	}

	attrValues := make(map[string]tftypes.Value, len(objType.AttributeTypes))
	for name, attrType := range objType.AttributeTypes {
		attrValues[name] = tftypes.NewValue(attrType, nil) // null
	}

	// Marshal the permissions list into a tftypes.Value for the Raw field.
	permsTFValue, err := permsList.ToTerraformValue(ctx)
	if err != nil {
		t.Fatalf("ToTerraformValue: %v", err)
	}
	attrValues["permissions"] = permsTFValue

	rawValue := tftypes.NewValue(schemaType, attrValues)
	return tfsdk.Config{
		Schema: schema,
		Raw:    rawValue,
	}
}

// TestSharingOwnerValidator_UnknownRole verifies that when a role is
// unknown (e.g., from a variable), the validator does NOT error with "Missing Owner".
func TestSharingOwnerValidator_UnknownRole(t *testing.T) {
	ctx := context.Background()
	config := buildSharingConfig(ctx, t, []struct{ user, role string }{
		{"user@example.com", ""}, // role unknown — simulates var.role during plan
	})

	v := sharingOwnerValidator{}
	req := resource.ValidateConfigRequest{Config: config}
	resp := &resource.ValidateConfigResponse{}
	v.ValidateResource(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("expected no errors when role is unknown (from variable), got: %v", resp.Diagnostics)
	}
}

// TestSharingOwnerValidator_KnownOwner verifies that a known "owner" role passes.
func TestSharingOwnerValidator_KnownOwner(t *testing.T) {
	ctx := context.Background()
	config := buildSharingConfig(ctx, t, []struct{ user, role string }{
		{"owner@example.com", "owner"},
	})

	v := sharingOwnerValidator{}
	req := resource.ValidateConfigRequest{Config: config}
	resp := &resource.ValidateConfigResponse{}
	v.ValidateResource(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("expected no errors when owner is present, got: %v", resp.Diagnostics)
	}
}

// TestSharingOwnerValidator_NoOwnerKnown verifies that if all roles are known
// and none is "owner", the validator errors.
func TestSharingOwnerValidator_NoOwnerKnown(t *testing.T) {
	ctx := context.Background()
	config := buildSharingConfig(ctx, t, []struct{ user, role string }{
		{"viewer@example.com", "viewer"},
	})

	v := sharingOwnerValidator{}
	req := resource.ValidateConfigRequest{Config: config}
	resp := &resource.ValidateConfigResponse{}
	v.ValidateResource(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected 'Missing Owner' error when no owner is present, but got no error")
	}
	found := false
	for _, d := range resp.Diagnostics {
		if d.Summary() == "Missing Owner" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'Missing Owner' error, got: %v", resp.Diagnostics)
	}
}

// TestSharingOwnerValidator_MixedUnknownAndOwner verifies that when one role
// is unknown and another is a known "owner", the validator passes (no false-positive
// "Multiple Owners" from the unknown).
func TestSharingOwnerValidator_MixedUnknownAndOwner(t *testing.T) {
	ctx := context.Background()
	config := buildSharingConfig(ctx, t, []struct{ user, role string }{
		{"owner@example.com", "owner"},
		{"dynamic@example.com", ""}, // unknown role
	})

	v := sharingOwnerValidator{}
	req := resource.ValidateConfigRequest{Config: config}
	resp := &resource.ValidateConfigResponse{}
	v.ValidateResource(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("expected no errors with one known owner and one unknown, got: %v", resp.Diagnostics)
	}
}

// TestSharingOwnerValidator_NullPermissions verifies the validator skips
// validation when permissions is null (e.g., during import planning).
func TestSharingOwnerValidator_NullPermissions(t *testing.T) {
	ctx := context.Background()
	schema := resource_sharing.SharingResourceSchema(ctx)

	// Empty config — permissions will be null.
	config := tfsdk.Config{Schema: schema}

	v := sharingOwnerValidator{}
	req := resource.ValidateConfigRequest{Config: config}
	resp := &resource.ValidateConfigResponse{}
	v.ValidateResource(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("expected no errors when permissions is null, got: %v", resp.Diagnostics)
	}
}

// TestSharingOwnerValidator_MultipleOwners verifies the validator rejects
// configs with more than one owner.
func TestSharingOwnerValidator_MultipleOwners(t *testing.T) {
	ctx := context.Background()
	config := buildSharingConfig(ctx, t, []struct{ user, role string }{
		{"owner1@example.com", "owner"},
		{"owner2@example.com", "owner"},
	})

	v := sharingOwnerValidator{}
	req := resource.ValidateConfigRequest{Config: config}
	resp := &resource.ValidateConfigResponse{}
	v.ValidateResource(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected 'Multiple Owners' error, but got no error")
	}
	found := false
	for _, d := range resp.Diagnostics {
		if d.Summary() == "Multiple Owners" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'Multiple Owners' error, got: %v", resp.Diagnostics)
	}
}

// TestSharingOwnerValidator_AllUnknown verifies the validator does not error
// when ALL roles are unknown (all provided via variables).
func TestSharingOwnerValidator_AllUnknown(t *testing.T) {
	ctx := context.Background()
	config := buildSharingConfig(ctx, t, []struct{ user, role string }{
		{"user1@example.com", ""},
		{"user2@example.com", ""},
	})

	v := sharingOwnerValidator{}
	req := resource.ValidateConfigRequest{Config: config}
	resp := &resource.ValidateConfigResponse{}
	v.ValidateResource(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("expected no errors when all roles are unknown, got: %v", resp.Diagnostics)
	}
}

// TestSharingOwnerValidator_EmptyList verifies the validator on an empty
// permissions list uses the path.Root("permissions") attribute error.
func TestSharingOwnerValidator_EmptyList(t *testing.T) {
	ctx := context.Background()
	schema := resource_sharing.SharingResourceSchema(ctx)

	// Build a config with an empty permissions list.
	permsList, listDiags := types.ListValueFrom(
		ctx,
		resource_sharing.PermissionsValue{}.Type(ctx),
		[]attr.Value{},
	)
	if listDiags.HasError() {
		t.Fatalf("ListValueFrom: %v", listDiags)
	}

	schemaType := schema.Type().TerraformType(ctx)
	objType := schemaType.(tftypes.Object)
	attrValues := make(map[string]tftypes.Value, len(objType.AttributeTypes))
	for name, attrType := range objType.AttributeTypes {
		attrValues[name] = tftypes.NewValue(attrType, nil)
	}
	permsTFValue, err := permsList.ToTerraformValue(ctx)
	if err != nil {
		t.Fatalf("ToTerraformValue: %v", err)
	}
	attrValues["permissions"] = permsTFValue

	config := tfsdk.Config{
		Schema: schema,
		Raw:    tftypes.NewValue(schemaType, attrValues),
	}

	v := sharingOwnerValidator{}
	req := resource.ValidateConfigRequest{Config: config}
	resp := &resource.ValidateConfigResponse{}
	v.ValidateResource(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected 'Missing Owner' error with empty permissions list, but got no error")
	}
}
