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

func TestSchemaParserOverrides(t *testing.T) {
	testdata := analysistest.TestData()

	// Run on both the generated schema package and the provider package.
	// The provider imports example_gen and applies overrides in Schema().
	results := analysistest.RunWithSuggestedFixes(t, testdata, schemaparser.Analyzer, "example_gen", "example_provider")

	// Find the result for example_provider (which has overrides applied).
	for _, r := range results {
		if r.Pass.Pkg.Name() != "example_provider" {
			continue
		}
		if r.Result == nil {
			t.Fatal("expected non-nil result for example_provider")
		}
		facts, ok := r.Result.(*schemaparser.SchemaFacts)
		if !ok {
			t.Fatalf("expected *SchemaFacts, got %T", r.Result)
		}

		schema, ok := facts.Schemas["ExampleResourceSchema"]
		if !ok {
			t.Fatal("expected to find ExampleResourceSchema in merged facts")
		}

		// Verify overrides.
		tests := []struct {
			field string
			want  schemaparser.FieldClass
		}{
			// Pattern 1: Full replacement — id changed from ComputedOnly to Required.
			{"id", schemaparser.Required},
			// Pattern 3: Modify-in-place — amount changed from OptionalComputed to Required.
			{"amount", schemaparser.Required},
			// Pattern 3b: No classification change — name stays Required.
			{"name", schemaparser.Required},
			// Unchanged fields.
			{"create_time", schemaparser.ComputedOnly},
			{"description", schemaparser.Optional},
			{"currency", schemaparser.OptionalComputed},
			// Pattern 4: New Computed-only field.
			{"display_name", schemaparser.ComputedOnly},
			// Pattern 5: New Optional field.
			{"phone", schemaparser.Optional},
		}

		for _, tt := range tests {
			t.Run(tt.field, func(t *testing.T) {
				info, ok := schema.Attrs[tt.field]
				if !ok {
					t.Fatalf("attribute %q not found in merged schema", tt.field)
				}
				if info.Class != tt.want {
					t.Errorf("attribute %q: got class %s, want %s", tt.field, info.Class, tt.want)
				}
			})
		}

		// Pattern 2: Deletion — update_time should be gone.
		t.Run("update_time_deleted", func(t *testing.T) {
			if _, ok := schema.Attrs["update_time"]; ok {
				t.Error("expected update_time to be deleted from schema")
			}
		})
	}
}
