package provider

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestMapRoleToModel(t *testing.T) {
	ctx := t.Context()

	resp := &models.Role{
		Id:                  "role-123",
		Name:                "FinOps Analyst",
		Description:         "Read-only access to cost reports",
		Type:                "custom",
		Customer:            "cust-456",
		ChildTenantEligible: true,
		Permissions:         []string{"perm-b", "perm-a"},
	}

	var state roleResourceModel
	diags := mapRoleToModel(ctx, resp, &state)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if state.Id.ValueString() != "role-123" {
		t.Errorf("expected id 'role-123', got '%s'", state.Id.ValueString())
	}
	if state.Name.ValueString() != "FinOps Analyst" {
		t.Errorf("expected name 'FinOps Analyst', got '%s'", state.Name.ValueString())
	}
	if state.Description.ValueString() != "Read-only access to cost reports" {
		t.Errorf("expected description 'Read-only access to cost reports', got '%s'", state.Description.ValueString())
	}
	if state.Type.ValueString() != "custom" {
		t.Errorf("expected type 'custom', got '%s'", state.Type.ValueString())
	}
	if state.Customer.ValueString() != "cust-456" {
		t.Errorf("expected customer 'cust-456', got '%s'", state.Customer.ValueString())
	}
	if !state.ChildTenantEligible.ValueBool() {
		t.Error("expected child_tenant_eligible true, got false")
	}

	var perms []string
	diags = state.Permissions.ElementsAs(ctx, &perms, false)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics reading permissions: %v", diags)
	}
	// Order must match the API response (the API preserves input order).
	if !slices.Equal(perms, []string{"perm-b", "perm-a"}) {
		t.Errorf("expected permissions [perm-b perm-a], got %v", perms)
	}
}

func TestMapRoleToModel_EmptyPermissions(t *testing.T) {
	ctx := t.Context()

	resp := &models.Role{
		Id:          "role-456",
		Name:        "no-perms",
		Type:        "custom",
		Permissions: nil, // API returned nil permissions
	}

	var state roleResourceModel
	diags := mapRoleToModel(ctx, resp, &state)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	// Must be an empty known list, not null (listnullread)
	if state.Permissions.IsNull() {
		t.Fatal("expected permissions to be non-null empty list, got null")
	}
	if len(state.Permissions.Elements()) != 0 {
		t.Errorf("expected 0 permissions, got %d", len(state.Permissions.Elements()))
	}
	if state.Description.IsNull() || state.Description.ValueString() != "" {
		t.Errorf("expected description to be empty string, got %v", state.Description)
	}
}

func TestOverlayRoleComputedFields(t *testing.T) {
	ctx := t.Context()

	apiResp := &models.Role{
		Id:                  "role-overlay",
		Name:                "server-name",
		Description:         "server-desc",
		Type:                "custom",
		Customer:            "cust-overlay",
		ChildTenantEligible: false,
		Permissions:         []string{"perm-a"},
	}

	// Scenario 1: Create overlay (Computed and omitted Optional+Computed fields unknown)
	plan := roleResourceModel{
		Id:                  types.StringUnknown(),
		Name:                types.StringValue("user-name"),
		Description:         types.StringUnknown(),
		Type:                types.StringUnknown(),
		Customer:            types.StringUnknown(),
		ChildTenantEligible: types.BoolUnknown(),
		Permissions:         types.ListUnknown(types.StringType),
	}

	diags := overlayRoleComputedFields(ctx, apiResp, &plan)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics on create overlay: %v", diags)
	}

	if plan.Id.ValueString() != "role-overlay" {
		t.Errorf("expected Id 'role-overlay', got '%s'", plan.Id.ValueString())
	}
	if plan.Name.ValueString() != "user-name" {
		t.Errorf("expected Name to remain 'user-name', got '%s'", plan.Name.ValueString())
	}
	if plan.Description.ValueString() != "server-desc" {
		t.Errorf("expected Description 'server-desc', got '%s'", plan.Description.ValueString())
	}
	if plan.Type.ValueString() != "custom" {
		t.Errorf("expected Type 'custom', got '%s'", plan.Type.ValueString())
	}
	if plan.Customer.ValueString() != "cust-overlay" {
		t.Errorf("expected Customer 'cust-overlay', got '%s'", plan.Customer.ValueString())
	}
	if plan.ChildTenantEligible.IsUnknown() || plan.ChildTenantEligible.ValueBool() {
		t.Errorf("expected ChildTenantEligible false, got %v", plan.ChildTenantEligible)
	}
	if plan.Permissions.IsUnknown() || len(plan.Permissions.Elements()) != 1 {
		t.Errorf("expected 1 resolved permission, got %v", plan.Permissions)
	}

	// Scenario 2: Update overlay with known user values (must be preserved)
	knownPerms, diags := types.ListValueFrom(ctx, types.StringType, []string{"perm-x", "perm-y"})
	if diags.HasError() {
		t.Fatalf("failed to create permissions list: %v", diags)
	}
	planUpdate := roleResourceModel{
		Id:                  types.StringValue("role-overlay"),
		Name:                types.StringValue("updated-user-name"),
		Description:         types.StringValue("user-desc"),
		Type:                types.StringValue("custom"),
		Customer:            types.StringValue("cust-overlay"),
		ChildTenantEligible: types.BoolValue(false),
		Permissions:         knownPerms,
	}

	diags = overlayRoleComputedFields(ctx, apiResp, &planUpdate)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics on update overlay: %v", diags)
	}

	if planUpdate.Name.ValueString() != "updated-user-name" {
		t.Errorf("expected Name to preserve plan value, got '%s'", planUpdate.Name.ValueString())
	}
	if planUpdate.Description.ValueString() != "user-desc" {
		t.Errorf("expected Description to preserve known user value, got '%s'", planUpdate.Description.ValueString())
	}
	if !planUpdate.Permissions.Equal(knownPerms) {
		t.Errorf("expected Permissions to preserve known user value, got %v", planUpdate.Permissions)
	}
}

func TestRoleToCreateRequest(t *testing.T) {
	ctx := t.Context()

	permsList, diags := types.ListValueFrom(ctx, types.StringType, []string{"perm-a", "perm-b"})
	if diags.HasError() {
		t.Fatalf("failed to create permissions list: %v", diags)
	}

	t.Run("all fields set", func(t *testing.T) {
		plan := roleResourceModel{
			Name:        types.StringValue("my-role"),
			Description: types.StringValue("My role"),
			Permissions: permsList,
		}

		req, diags := plan.toCreateRequest(ctx)
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}

		if req.Name != "my-role" {
			t.Errorf("expected name 'my-role', got '%s'", req.Name)
		}
		if req.Description == nil || *req.Description != "My role" {
			t.Errorf("expected description 'My role', got %v", req.Description)
		}
		if req.Permissions == nil || !slices.Equal(*req.Permissions, []string{"perm-a", "perm-b"}) {
			t.Errorf("expected permissions [perm-a perm-b], got %v", req.Permissions)
		}
	})

	t.Run("optional fields unknown are omitted", func(t *testing.T) {
		plan := roleResourceModel{
			Name:        types.StringValue("my-role"),
			Description: types.StringUnknown(),
			Permissions: types.ListUnknown(types.StringType),
		}

		req, diags := plan.toCreateRequest(ctx)
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}

		if req.Description != nil {
			t.Errorf("expected description to be omitted, got %q", *req.Description)
		}
		if req.Permissions != nil {
			t.Errorf("expected permissions to be omitted, got %v", *req.Permissions)
		}
	})
}

func TestRoleToUpdateRequest(t *testing.T) {
	ctx := t.Context()

	permsList, diags := types.ListValueFrom(ctx, types.StringType, []string{"perm-a"})
	if diags.HasError() {
		t.Fatalf("failed to create permissions list: %v", diags)
	}
	emptyList, diags := types.ListValueFrom(ctx, types.StringType, []string{})
	if diags.HasError() {
		t.Fatalf("failed to create empty list: %v", diags)
	}

	tests := []struct {
		name            string
		description     types.String
		permissions     types.List
		wantDescription *string
		wantPermissions []string // nil means omitted
	}{
		{
			name:            "set values are sent",
			description:     types.StringValue("desc"),
			permissions:     permsList,
			wantDescription: new("desc"),
			wantPermissions: []string{"perm-a"},
		},
		{
			name:            "null clears with explicit empty values",
			description:     types.StringNull(),
			permissions:     types.ListNull(types.StringType),
			wantDescription: new(""),
			wantPermissions: []string{},
		},
		{
			name:            "explicit empty values clear",
			description:     types.StringValue(""),
			permissions:     emptyList,
			wantDescription: new(""),
			wantPermissions: []string{},
		},
		{
			name:            "unknown values are omitted",
			description:     types.StringUnknown(),
			permissions:     types.ListUnknown(types.StringType),
			wantDescription: nil,
			wantPermissions: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := roleResourceModel{
				Name:        types.StringValue("my-role"),
				Description: tt.description,
				Permissions: tt.permissions,
			}

			req, diags := plan.toUpdateRequest(ctx)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}

			if req.Name == nil || *req.Name != "my-role" {
				t.Errorf("expected name 'my-role', got %v", req.Name)
			}

			switch {
			case tt.wantDescription == nil && req.Description != nil:
				t.Errorf("expected description omitted, got %q", *req.Description)
			case tt.wantDescription != nil && (req.Description == nil || *req.Description != *tt.wantDescription):
				t.Errorf("expected description %q, got %v", *tt.wantDescription, req.Description)
			}

			switch {
			case tt.wantPermissions == nil && req.Permissions != nil:
				t.Errorf("expected permissions omitted, got %v", *req.Permissions)
			case tt.wantPermissions != nil && req.Permissions == nil:
				t.Errorf("expected permissions %v, got omitted", tt.wantPermissions)
			case tt.wantPermissions != nil && !slices.Equal(*req.Permissions, tt.wantPermissions):
				t.Errorf("expected permissions %v, got %v", tt.wantPermissions, *req.Permissions)
			}
		})
	}
}

func TestRole_PopulateState_NotFound(t *testing.T) {
	ctx := t.Context()

	srv := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/iam/v1/roles/missing-id" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error": "Not Found"}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))

	client := newTestAPIClient(t, srv)

	res := &roleResource{client: client}
	state := roleResourceModel{
		Id: types.StringValue("missing-id"),
	}

	diags := res.populateState(ctx, &state)
	if diags.HasError() {
		t.Fatalf("populateState should not return error on 404, got: %v", diags)
	}
	if !state.Id.IsNull() {
		t.Errorf("expected state.Id to be set to null on 404, got '%s'", state.Id.ValueString())
	}
}

func TestRoleNamePattern(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"single character", "a", true},
		{"interior spaces", "FinOps Analyst", true},
		{"interior newline", "line one\nline two", true},
		{"non-ASCII letters", "Rôle für Analysten", true},
		{"leading space", " padded", false},
		{"trailing space", "padded ", false},
		{"leading tab", "\tpadded", false},
		{"trailing newline", "padded\n", false},
		{"leading next line U+0085", "\u0085padded", false},
		{"trailing no-break space U+00A0", "padded\u00a0", false},
		{"leading ogham space U+1680", "\u1680padded", false},
		{"trailing em space U+2003", "padded\u2003", false},
		{"trailing line separator U+2028", "padded\u2028", false},
		{"leading narrow no-break space U+202F", "\u202fpadded", false},
		{"trailing ideographic space U+3000", "padded\u3000", false},
		{"whitespace only", "   ", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := roleNamePattern.MatchString(tt.input); got != tt.valid {
				t.Errorf("roleNamePattern.MatchString(%q) = %v, want %v", tt.input, got, tt.valid)
			}
		})
	}
}
