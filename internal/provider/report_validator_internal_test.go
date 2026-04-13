package provider

import (
	"context"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_report"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ---------------------------------------------------------------------------
// TestReportTimestampValidator_NoCustomTimeRange
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// TestIsNAFallback
// ---------------------------------------------------------------------------

func TestIsNAFallback(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		// Known NullFallback sentinels from the Cloud Analytics keymap.
		{"[Service N/A]", true},
		{"[Folder N/A]", true},
		{"[Customer N/A]", true},
		{"[Project/Account ID N/A]", true},
		{"[Attribution N/A]", true},
		{"[Allocation N/A]", true},
		{"[Value N/A]", true},
		{"[Region N/A]", true},
		// NOT sentinels.
		{"Compute Engine", false},
		{"[N/A]", false}, // NotApplicable constant — no space before N/A.
		{"", false},
		{"[something]", false},
		{"N/A", false},
		{"Service N/A", false},         // Missing leading bracket.
		{"[Service N/A] extra", false}, // Trailing characters after closing bracket.
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			if got := isNAFallback(tt.value); got != tt.want {
				t.Errorf("isNAFallback(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestWarnNAFilterValues
// ---------------------------------------------------------------------------

// buildTestFilter constructs a resource_report.FiltersValue with the given values
// slice. Other fields are set to safe defaults so the constructor does not error.
func buildTestFilter(ctx context.Context, t *testing.T, values []string) resource_report.FiltersValue {
	t.Helper()
	elems := make([]attr.Value, len(values))
	for i, v := range values {
		elems[i] = types.StringValue(v)
	}
	f, diags := resource_report.NewFiltersValue(
		resource_report.FiltersValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"case_insensitive": types.BoolValue(false),
			"id":               types.StringValue("service_description"),
			"include_null":     types.BoolValue(false),
			"inverse":          types.BoolNull(),
			"mode":             types.StringValue("is"),
			"type":             types.StringValue("fixed"),
			"values":           types.ListValueMust(types.StringType, elems),
		},
	)
	if diags.HasError() {
		t.Fatalf("buildTestFilter: %v", diags)
	}
	return f
}

func countWarnings(d diag.Diagnostics) int {
	n := 0
	for _, item := range d {
		if item.Severity() == diag.SeverityWarning {
			n++
		}
	}
	return n
}

func TestWarnNAFilterValues(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		filters      [][]string // outer = filters, inner = values per filter
		wantWarnings int
	}{
		{
			name:         "no filters",
			filters:      nil,
			wantWarnings: 0,
		},
		{
			name:         "real values only",
			filters:      [][]string{{"Compute Engine", "Cloud Storage"}},
			wantWarnings: 0,
		},
		{
			name:         "pure sentinel",
			filters:      [][]string{{"[Service N/A]"}},
			wantWarnings: 1,
		},
		{
			name:         "mixed real and sentinel",
			filters:      [][]string{{"Compute Engine", "[Service N/A]"}},
			wantWarnings: 1,
		},
		{
			name:         "multiple filters, one sentinel",
			filters:      [][]string{{"Compute Engine"}, {"[Folder N/A]"}},
			wantWarnings: 1,
		},
		{
			name:         "multiple filters, each with one sentinel",
			filters:      [][]string{{"[Service N/A]"}, {"[Folder N/A]"}},
			wantWarnings: 2,
		},
		{
			name:         "two sentinels in same filter",
			filters:      [][]string{{"[Service N/A]", "[Folder N/A]"}},
			wantWarnings: 2,
		},
		{
			name:         "[N/A] is not a sentinel (NotApplicable constant, no space)",
			filters:      [][]string{{"[N/A]"}},
			wantWarnings: 0,
		},
		{
			name:         "empty values list",
			filters:      [][]string{{}},
			wantWarnings: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var filterVals []resource_report.FiltersValue
			for _, vals := range tt.filters {
				filterVals = append(filterVals, buildTestFilter(ctx, t, vals))
			}
			var d diag.Diagnostics
			warnNAFilterValues(ctx, filterVals, &d)
			got := countWarnings(d)
			if got != tt.wantWarnings {
				t.Errorf("wantWarnings=%d got=%d diags=%v", tt.wantWarnings, got, d)
			}
			if d.HasError() {
				t.Errorf("unexpected errors: %v", d)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestReportFilterNAValidator_EmptyConfig
// ---------------------------------------------------------------------------

// TestReportFilterNAValidator_EmptyConfig verifies that the validator produces
// no diagnostics when the config has no filters set (null).
func TestReportFilterNAValidator_EmptyConfig(t *testing.T) {
	ctx := context.Background()
	config := tfsdk.Config{Schema: resource_report.ReportResourceSchema(ctx)}
	v := reportFilterNAValidator{}
	req := resource.ValidateConfigRequest{Config: config}
	resp := &resource.ValidateConfigResponse{}
	v.ValidateResource(ctx, req, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("expected no errors with empty config, got: %v", resp.Diagnostics)
	}
	if len(resp.Diagnostics) > 0 {
		t.Errorf("expected zero diagnostics with empty config, got %d", len(resp.Diagnostics))
	}
}

// ---------------------------------------------------------------------------
// TestWarnNASentinels — unknown element handling
// ---------------------------------------------------------------------------

// TestWarnNASentinels_UnknownElement reproduces the "Value Conversion Error"
// crash when a filter/scope values list contains an unknown element (e.g.
// referencing doit_allocation.xxx.id that is planned for update). The function
// checks if the LIST is unknown, but not if individual ELEMENTS are unknown.
// Go []string cannot represent unknown values, so ElementsAs crashes.
func TestWarnNASentinels_UnknownElement(t *testing.T) {
	ctx := context.Background()

	unknownList := types.ListValueMust(types.StringType, []attr.Value{
		types.StringValue("known-allocation-id"),
		types.StringUnknown(), // simulates a cross-resource reference during plan
	})

	var diags diag.Diagnostics
	warnNASentinels(ctx, path.Root("test"), []types.List{unknownList}, &diags)

	if diags.HasError() {
		t.Fatalf("warnNASentinels crashed with unknown element: %v", diags)
	}
	if countWarnings(diags) > 0 {
		t.Errorf("expected no warnings when elements contain unknown values, got %d", countWarnings(diags))
	}
}

// TestWarnNASentinels_AllUnknown reproduces the crash when ALL elements in a
// values list are unknown (e.g. values = [doit_allocation.xxx.id]).
func TestWarnNASentinels_AllUnknown(t *testing.T) {
	ctx := context.Background()

	unknownList := types.ListValueMust(types.StringType, []attr.Value{
		types.StringUnknown(),
	})

	var diags diag.Diagnostics
	warnNASentinels(ctx, path.Root("test"), []types.List{unknownList}, &diags)

	if diags.HasError() {
		t.Fatalf("warnNASentinels crashed with all-unknown elements: %v", diags)
	}
}

// TestWarnNAFilterValues_UnknownElement reproduces the crash through the full
// warnNAFilterValues path (the report validator's entry point).
func TestWarnNAFilterValues_UnknownElement(t *testing.T) {
	ctx := context.Background()

	f, fDiags := resource_report.NewFiltersValue(
		resource_report.FiltersValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"case_insensitive": types.BoolValue(false),
			"id":               types.StringValue("attribution"),
			"include_null":     types.BoolValue(false),
			"inverse":          types.BoolNull(),
			"mode":             types.StringValue("is"),
			"type":             types.StringValue("allocation_rule"),
			"values": types.ListValueMust(types.StringType, []attr.Value{
				types.StringUnknown(), // simulates doit_allocation.xxx.id during plan
			}),
		},
	)
	if fDiags.HasError() {
		t.Fatalf("NewFiltersValue: %v", fDiags)
	}

	var diags diag.Diagnostics
	warnNAFilterValues(ctx, []resource_report.FiltersValue{f}, &diags)

	if diags.HasError() {
		t.Fatalf("warnNAFilterValues crashed with unknown element: %v", diags)
	}
}

// TestWarnNASentinels_KnownValues_StillWarns verifies that the sentinel warning
// still fires for fully-known values containing sentinels (guard against
// false positives from the unknown-element fix).
func TestWarnNASentinels_KnownValues_StillWarns(t *testing.T) {
	ctx := context.Background()

	knownList := types.ListValueMust(types.StringType, []attr.Value{
		types.StringValue("[Service N/A]"),
		types.StringValue("normal-value"),
	})

	var diags diag.Diagnostics
	warnNASentinels(ctx, path.Root("test"), []types.List{knownList}, &diags)

	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if got := countWarnings(diags); got != 1 {
		t.Errorf("expected 1 warning for sentinel value, got %d", got)
	}
}
