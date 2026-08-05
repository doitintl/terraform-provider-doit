package provider

import (
	"context"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
)

// TestMapServiceQuotasToModel_EmptyReturnsNonNullList verifies the
// listnullread invariant deterministically: an empty (or nil) API result
// must map to a non-null, zero-length items list, never types.ListNull.
// This is tested directly against the mapping function rather than via an
// acceptance test filtered to a "currently empty" live result, since no
// filter combination on the real API is guaranteed to stay empty as tenant
// data changes.
func TestMapServiceQuotasToModel_EmptyReturnsNonNullList(t *testing.T) {
	ctx := context.Background()

	for _, name := range []string{"nil slice", "empty slice"} {
		t.Run(name, func(t *testing.T) {
			var quotas []models.ServiceQuota
			if name == "empty slice" {
				quotas = []models.ServiceQuota{}
			}

			var data serviceQuotasDataSourceModel
			diags := mapServiceQuotasToModel(ctx, quotas, &data)
			if diags.HasError() {
				t.Fatalf("mapServiceQuotasToModel returned errors: %v", diags)
			}

			if data.Items.IsNull() {
				t.Fatal("expected Items to be a non-null empty list, got null")
			}
			if data.Items.IsUnknown() {
				t.Fatal("expected Items to be known, got unknown")
			}
			if got := len(data.Items.Elements()); got != 0 {
				t.Fatalf("expected 0 elements, got %d", got)
			}
		})
	}
}
