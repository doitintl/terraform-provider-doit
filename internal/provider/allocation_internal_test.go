package provider

import (
	"context"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
)

// TestToAllocationRuleComponentsListValue_EmptySlice verifies that passing an
// empty-but-non-nil slice does not panic. The function indexes stateComponents[0]
// to get the element type, which would panic with len=0.
// This was caught by Copilot code review on PR #100.
func TestToAllocationRuleComponentsListValue_EmptySlice(t *testing.T) {
	ctx := context.Background()

	// This must not panic
	result, diags := toAllocationRuleComponentsListValue(ctx, []models.AllocationComponent{})

	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if result.IsNull() || result.IsUnknown() {
		t.Error("expected a known, non-null list value for empty components")
	}

	if len(result.Elements()) != 0 {
		t.Errorf("expected 0 elements, got %d", len(result.Elements()))
	}
}
