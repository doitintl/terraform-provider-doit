package schemaparser_test

import (
	"testing"

	"github.com/doitintl/terraform-provider-doit/tools/linters/schemaparser"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestSchemaParser(t *testing.T) {
	testdata := analysistest.TestData()

	// Run the analyzer. We use RunWithSuggestedFixes to avoid the strict
	// fact-checking that analysistest.Run applies. The schemaparser exports
	// package facts but produces no diagnostics, so we verify facts manually.
	results := analysistest.RunWithSuggestedFixes(t, testdata, schemaparser.Analyzer, "example_gen")

	// Verify the parsed schema facts via the Result.
	for _, r := range results {
		if r.Result == nil {
			continue
		}
		facts, ok := r.Result.(*schemaparser.SchemaFacts)
		if !ok {
			t.Fatalf("expected *SchemaFacts, got %T", r.Result)
		}

		// Check that we found the test schema.
		schema, ok := facts.Schemas["ExampleResourceSchema"]
		if !ok {
			t.Fatal("expected to find ExampleResourceSchema in facts")
		}

		// Verify field classifications.
		tests := []struct {
			field string
			want  schemaparser.FieldClass
			list  bool
		}{
			{"id", schemaparser.ComputedOnly, false},
			{"create_time", schemaparser.ComputedOnly, false},
			{"update_time", schemaparser.ComputedOnly, false},
			{"name", schemaparser.Required, false},
			{"description", schemaparser.Optional, false},
			{"amount", schemaparser.OptionalComputed, false},
			{"currency", schemaparser.OptionalComputed, false},
			{"tags", schemaparser.OptionalComputed, true},
			{"alerts", schemaparser.OptionalComputed, true},
		}

		for _, tt := range tests {
			t.Run(tt.field, func(t *testing.T) {
				info, ok := schema.Attrs[tt.field]
				if !ok {
					t.Fatalf("attribute %q not found in schema", tt.field)
				}
				if info.Class != tt.want {
					t.Errorf("attribute %q: got class %s, want %s", tt.field, info.Class, tt.want)
				}
				if info.IsList != tt.list {
					t.Errorf("attribute %q: got IsList=%v, want %v", tt.field, info.IsList, tt.list)
				}
			})
		}

		// Verify nested attribute classifications.
		alerts, ok := schema.Attrs["alerts"]
		if !ok {
			t.Fatal("expected to find alerts attribute")
		}
		if alerts.NestedAttrs == nil {
			t.Fatal("expected alerts to have nested attributes")
		}

		nestedTests := []struct {
			field string
			want  schemaparser.FieldClass
		}{
			{"percentage", schemaparser.OptionalComputed},
			{"triggered", schemaparser.ComputedOnly},
			{"threshold", schemaparser.Required},
		}

		for _, tt := range nestedTests {
			t.Run("alerts."+tt.field, func(t *testing.T) {
				info, ok := alerts.NestedAttrs[tt.field]
				if !ok {
					t.Fatalf("nested attribute %q not found in alerts", tt.field)
				}
				if info.Class != tt.want {
					t.Errorf("nested attribute %q: got class %s, want %s", tt.field, info.Class, tt.want)
				}
			})
		}
	}
}
