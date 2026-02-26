package provider

import (
	"context"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_report"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestReportTimestampValidator_NoCustomTimeRange verifies that the validator
// produces no errors/warnings when custom_time_range is not set.
// This tests whether GetAttribute on missing nested paths generates diagnostics
// that would confuse users if propagated.
func TestReportTimestampValidator_NoCustomTimeRange(t *testing.T) {
	ctx := context.Background()
	schema := resource_report.ReportResourceSchema(ctx)

	// Create an empty config with the report schema (no attributes set).
	config := tfsdk.Config{Schema: schema}

	// Test what GetAttribute returns for a missing nested path.
	var val types.String
	diags := config.GetAttribute(ctx, path.Root("config").AtName("custom_time_range").AtName("from"), &val)

	t.Logf("GetAttribute on missing path: hasError=%v, count=%d, isNull=%v, isUnknown=%v",
		diags.HasError(), len(diags), val.IsNull(), val.IsUnknown())
	for i, d := range diags {
		t.Logf("  diag[%d]: severity=%s summary=%q detail=%q", i, d.Severity(), d.Summary(), d.Detail())
	}

	// Now run the full validator and verify it produces clean output.
	v := reportTimestampValidator{}
	req := resource.ValidateConfigRequest{Config: config}
	resp := &resource.ValidateConfigResponse{}
	v.ValidateResource(ctx, req, resp)

	t.Logf("ValidateResource diagnostics: hasError=%v, count=%d", resp.Diagnostics.HasError(), len(resp.Diagnostics))
	for i, d := range resp.Diagnostics {
		t.Logf("  resp diag[%d]: severity=%s summary=%q", i, d.Severity(), d.Summary())
	}

	if resp.Diagnostics.HasError() {
		t.Fatalf("expected no errors when custom_time_range is not set, got: %v", resp.Diagnostics)
	}
	if len(resp.Diagnostics) > 0 {
		t.Errorf("expected zero diagnostics when custom_time_range is not set, got %d", len(resp.Diagnostics))
	}
}
