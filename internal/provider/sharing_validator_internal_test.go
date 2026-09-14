package provider

import (
	"context"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_sharing"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// buildSharingConfig constructs a tfsdk.Config with a populated permissions list.
// Each entry is (user, role) where role="" means unknown.
func buildSharingConfig(ctx context.Context, t *testing.T, perms []struct{ user, role string }) tfsdk.Config {
	t.Helper()

	// Build the framework-level list elements.
	elems := make([]attr.Value, len(perms))
	for i, p := range perms {
		var role attr.Value
		if p.role == "" {
			role = types.StringUnknown()
		} else {
			role = types.StringValue(p.role)
		}
		elems[i] = buildPermissionValue(ctx, t, types.StringValue(p.user), role)
	}
	return buildSharingConfigWithPermissionValues(ctx, t, elems)
}

func buildPermissionValue(ctx context.Context, t *testing.T, user, role attr.Value) attr.Value {
	t.Helper()
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
	return pv
}

func buildSharingConfigWithPermissionValues(ctx context.Context, t *testing.T, elems []attr.Value) tfsdk.Config {
	t.Helper()
	schema := resource_sharing.SharingResourceSchema(ctx)

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

func TestSharingOwnerValidator_ElementStates(t *testing.T) {
	ctx := t.Context()
	unknownRole := buildPermissionValue(ctx, t, types.StringValue("unknown@example.com"), types.StringUnknown())
	owner := buildPermissionValue(ctx, t, types.StringValue("owner@example.com"), types.StringValue("owner"))
	viewer := buildPermissionValue(ctx, t, types.StringValue("viewer@example.com"), types.StringValue("viewer"))
	nullRole := buildPermissionValue(ctx, t, types.StringValue("null@example.com"), types.StringNull())

	tests := map[string]struct {
		values      []attr.Value
		wantSummary string
	}{
		"null element":             {values: []attr.Value{resource_sharing.NewPermissionsValueNull()}, wantSummary: "Missing Owner"},
		"unknown element":          {values: []attr.Value{resource_sharing.NewPermissionsValueUnknown()}},
		"null role":                {values: []attr.Value{nullRole}, wantSummary: "Missing Owner"},
		"null and unknown role":    {values: []attr.Value{resource_sharing.NewPermissionsValueNull(), unknownRole}},
		"null and known owner":     {values: []attr.Value{resource_sharing.NewPermissionsValueNull(), owner}},
		"null and known non-owner": {values: []attr.Value{resource_sharing.NewPermissionsValueNull(), viewer}, wantSummary: "Missing Owner"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			config := buildSharingConfigWithPermissionValues(ctx, t, test.values)
			resp := &resource.ValidateConfigResponse{}
			sharingOwnerValidator{}.ValidateResource(ctx, resource.ValidateConfigRequest{Config: config}, resp)

			wantCount := 0
			if test.wantSummary != "" {
				wantCount = 1
			}
			if len(resp.Diagnostics) != wantCount {
				t.Fatalf("diagnostic count = %d, want %d: %v", len(resp.Diagnostics), wantCount, resp.Diagnostics)
			}
			if wantCount == 0 {
				return
			}
			diagnostic := resp.Diagnostics[0]
			if diagnostic.Severity() != diag.SeverityError {
				t.Errorf("severity = %s, want error", diagnostic.Severity())
			}
			if diagnostic.Summary() != test.wantSummary {
				t.Errorf("summary = %q, want %q", diagnostic.Summary(), test.wantSummary)
			}
			withPath, ok := diagnostic.(diag.DiagnosticWithPath)
			if !ok {
				t.Fatalf("diagnostic %T has no path", diagnostic)
			}
			if got, want := withPath.Path(), path.Root("permissions"); !got.Equal(want) {
				t.Errorf("path = %s, want %s", got, want)
			}
		})
	}
}

// TestSharingOwnerValidator_UnknownRole verifies that when a role is
// unknown (e.g., from a variable), the validator does NOT error with "Missing Owner".
func TestSharingOwnerValidator_UnknownRole(t *testing.T) {
	ctx := t.Context()
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
	ctx := t.Context()
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
	ctx := t.Context()
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
	ctx := t.Context()
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
	ctx := t.Context()
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
	ctx := t.Context()
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
	ctx := t.Context()
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
	ctx := t.Context()
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
