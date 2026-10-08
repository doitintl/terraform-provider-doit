package provider

import (
	"context"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_budget"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func apiCollaborator(email string, role models.CollaboratorRole) models.Collaborator {
	return models.Collaborator{Email: &email, Role: &role}
}

func budgetCollaboratorsList(ctx context.Context, t *testing.T, elems ...resource_budget.CollaboratorsValue) types.List {
	t.Helper()
	list, diags := types.ListValueFrom(ctx, resource_budget.CollaboratorsValue{}.Type(ctx), elems)
	if diags.HasError() {
		t.Fatalf("building collaborators list: %v", diags)
	}
	return list
}

func budgetCollaboratorValue(ctx context.Context, email, role attr.Value) resource_budget.CollaboratorsValue {
	return resource_budget.NewCollaboratorsValueMust(
		resource_budget.CollaboratorsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"email": email, "role": role},
	)
}

// collaboratorPairs flattens a collaborators list into "email/role" strings
// ("<null>" for null attributes) so assertions stay readable.
func collaboratorPairs(ctx context.Context, t *testing.T, list types.List) []string {
	t.Helper()
	if list.IsNull() || list.IsUnknown() {
		t.Fatalf("collaborators must be a known list, got %s", list)
	}
	var elems []resource_budget.CollaboratorsValue
	if diags := list.ElementsAs(ctx, &elems, false); diags.HasError() {
		t.Fatalf("reading collaborators: %v", diags)
	}
	out := make([]string, len(elems))
	for i, e := range elems {
		email, role := "<null>", "<null>"
		if !e.Email.IsNull() {
			email = e.Email.ValueString()
		}
		if !e.Role.IsNull() {
			role = e.Role.ValueString()
		}
		out[i] = email + "/" + role
	}
	return out
}

func assertPairs(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("collaborators = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("collaborators = %v, want %v", got, want)
		}
	}
}

func minimalBudgetAPI(collaborators *[]models.Collaborator) *models.BudgetAPI {
	id := "budget-id"
	return &models.BudgetAPI{
		Id:            &id,
		Name:          "b",
		Currency:      models.CurrencyUSD,
		Collaborators: collaborators,
	}
}

// TestMapBudgetToModelCollaborators covers how the API's collaborators
// response is mapped into state, in particular that an API which starts
// returning an auto-assigned owner (where it used to return nothing) maps to a
// known list, and that nothing maps to null.
func TestMapBudgetToModelCollaborators(t *testing.T) {
	ctx := t.Context()

	tests := []struct {
		name string
		resp *[]models.Collaborator
		want []string
	}{
		{name: "nil maps to empty list", resp: nil, want: []string{}},
		{name: "empty slice maps to empty list", resp: &[]models.Collaborator{}, want: []string{}},
		{
			name: "auto-assigned owner",
			resp: &[]models.Collaborator{apiCollaborator("owner@example.com", "owner")},
			want: []string{"owner@example.com/owner"},
		},
		{
			name: "multiple collaborators keep API order",
			resp: &[]models.Collaborator{
				apiCollaborator("v@example.com", "viewer"),
				apiCollaborator("o@example.com", "owner"),
				apiCollaborator("e@example.com", "editor"),
			},
			want: []string{"v@example.com/viewer", "o@example.com/owner", "e@example.com/editor"},
		},
		{
			name: "nil email and role map to null attributes",
			resp: &[]models.Collaborator{{}},
			want: []string{"<null>/<null>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var state budgetResourceModel
			if diags := mapBudgetToModel(ctx, minimalBudgetAPI(tt.resp), &state); diags.HasError() {
				t.Fatalf("mapBudgetToModel: %v", diags)
			}
			assertPairs(t, collaboratorPairs(ctx, t, state.Collaborators), tt.want)
		})
	}
}

// TestOverlayBudgetCollaborators covers the Create/Update overlay: an omitted
// (Unknown) collaborators list resolves from the API response, while a
// user-configured list always wins.
func TestOverlayBudgetCollaborators(t *testing.T) {
	ctx := t.Context()
	owner := apiCollaborator("api-owner@example.com", "owner")
	elementType := resource_budget.CollaboratorsValue{}.Type(ctx)

	t.Run("omitted with API-assigned owner resolves to that owner", func(t *testing.T) {
		plan := budgetResourceModel{Collaborators: types.ListUnknown(elementType)}
		if diags := overlayBudgetComputedFields(ctx, minimalBudgetAPI(&[]models.Collaborator{owner}), &plan); diags.HasError() {
			t.Fatalf("overlay: %v", diags)
		}
		assertPairs(t, collaboratorPairs(ctx, t, plan.Collaborators), []string{"api-owner@example.com/owner"})
	})

	t.Run("omitted with API returning nothing resolves to a known empty list", func(t *testing.T) {
		plan := budgetResourceModel{Collaborators: types.ListUnknown(elementType)}
		if diags := overlayBudgetComputedFields(ctx, minimalBudgetAPI(nil), &plan); diags.HasError() {
			t.Fatalf("overlay: %v", diags)
		}
		assertPairs(t, collaboratorPairs(ctx, t, plan.Collaborators), []string{})
	})

	t.Run("configured list wins over the API response", func(t *testing.T) {
		configured := budgetCollaboratorsList(ctx, t,
			budgetCollaboratorValue(ctx, types.StringValue("me@example.com"), types.StringValue("owner")))
		plan := budgetResourceModel{Collaborators: configured}
		resp := &[]models.Collaborator{owner, apiCollaborator("extra@example.com", "viewer")}
		if diags := overlayBudgetComputedFields(ctx, minimalBudgetAPI(resp), &plan); diags.HasError() {
			t.Fatalf("overlay: %v", diags)
		}
		assertPairs(t, collaboratorPairs(ctx, t, plan.Collaborators), []string{"me@example.com/owner"})
	})

	t.Run("configured list with unknown role resolves only that attribute", func(t *testing.T) {
		configured := budgetCollaboratorsList(ctx, t,
			budgetCollaboratorValue(ctx, types.StringValue("api-owner@example.com"), types.StringUnknown()))
		plan := budgetResourceModel{Collaborators: configured}
		if diags := overlayBudgetComputedFields(ctx, minimalBudgetAPI(&[]models.Collaborator{owner}), &plan); diags.HasError() {
			t.Fatalf("overlay: %v", diags)
		}
		assertPairs(t, collaboratorPairs(ctx, t, plan.Collaborators), []string{"api-owner@example.com/owner"})
	})
}

// TestBudgetCollaboratorsLegacyStateConverges documents the upgrade path for a
// budget whose state was written while the API returned no owner: refreshing
// against an API that now returns one yields a one-element list, and an
// overlay of an omitted config against that API leaves state and plan equal.
func TestBudgetCollaboratorsLegacyStateConverges(t *testing.T) {
	ctx := t.Context()
	owner := apiCollaborator("api-owner@example.com", "owner")

	// State written before the upstream fix: no collaborators returned.
	var state budgetResourceModel
	if diags := mapBudgetToModel(ctx, minimalBudgetAPI(nil), &state); diags.HasError() {
		t.Fatalf("legacy mapBudgetToModel: %v", diags)
	}
	assertPairs(t, collaboratorPairs(ctx, t, state.Collaborators), []string{})

	// Refresh after the fix: the owner appears in state.
	if diags := mapBudgetToModel(ctx, minimalBudgetAPI(&[]models.Collaborator{owner}), &state); diags.HasError() {
		t.Fatalf("refresh mapBudgetToModel: %v", diags)
	}
	refreshed := collaboratorPairs(ctx, t, state.Collaborators)
	assertPairs(t, refreshed, []string{"api-owner@example.com/owner"})

	// An update with collaborators omitted carries the refreshed state forward
	// (Optional+Computed prior-state copy) and the overlay must not alter it.
	plan := budgetResourceModel{Collaborators: state.Collaborators}
	if diags := overlayBudgetComputedFields(ctx, minimalBudgetAPI(&[]models.Collaborator{owner}), &plan); diags.HasError() {
		t.Fatalf("overlay: %v", diags)
	}
	assertPairs(t, collaboratorPairs(ctx, t, plan.Collaborators), refreshed)
}
