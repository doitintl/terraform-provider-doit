package provider

import (
	"context"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/oapi-codegen/nullable"
)

func TestNullableComparison_ExplicitNullIsNotNil(t *testing.T) {
	// Assesses that nullable.Nullable[T] is a map type, so an explicit JSON null
	// parses into an initialized, non-nil map.
	var n nullable.Nullable[models.AllocationRule]
	n.SetNull()

	if n == nil {
		t.Errorf("Expected n != nil when set to null, but got nil")
	}

	// This proves that a direct comparison like `resp.Rule != nil` will be TRUE
	// even when Rule is explicitly null. We must use `nullableToPointer(resp.Rule) != nil`
	// to properly detect explicitly specified, non-null values.
}

func TestUnknownPointerAccessor_ReturnsZeroValue(t *testing.T) {
	// Assesses the framework behavior that pointer accessors return a pointer
	// to the zero value when the state is Unknown, not nil.
	unknownStr := types.StringUnknown()
	p := unknownStr.ValueStringPointer()

	if p == nil {
		t.Fatalf("Expected pointer to not be nil for Unknown")
	}

	// Because `p` is not nil, `pointerToNullable(p)` will mark the nullable as specified:
	n := pointerToNullable(p)
	if !n.IsSpecified() {
		t.Errorf("Expected nullable to be marked as specified for Unknown")
	}
	if n.MustGet() != "" {
		t.Errorf("Expected nullable value to be empty string for Unknown, got %q", n.MustGet())
	}
	// This proves why `!IsUnknown()` checks are required for `Optional+Computed` fields
	// before invoking their pointer accessors.
}

func TestMapAllocationToModel_GroupRule_RuleNull(t *testing.T) {
	ctx := context.Background()

	allocType := models.AllocationAllocationType("group")
	var nullRule nullable.Nullable[models.AllocationRule]
	nullRule.SetNull()

	apiResp := &models.Allocation{
		AllocationType: &allocType,
		Rule:           nullRule,
		Rules:          &[]nullable.Nullable[models.GroupAllocationRule]{}, // Empty group allocation
	}

	state := &allocationResourceModel{}
	state.Id = types.StringValue("test-id")

	// Call the mapping function
	diags := mapAllocationToModel(ctx, nil, apiResp, state)
	if diags.HasError() {
		t.Fatalf("Unexpected diagnostics: %v", diags)
	}

	// state.Rules should be correctly mapped as an empty list, not Null.
	// If `resp.Rule != nil` is used instead of `nullableToPointer(resp.Rule) != nil`,
	// state.Rules will incorrectly be mapped as Null.
	if state.Rules.IsNull() {
		t.Errorf("state.Rules is Null, expected empty list")
	}
}
