package schemaparser_test

import (
	"reflect"
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

		// Category C: ModifyPlan calls requiresReplaceWhenCleared for config.count.
		t.Run("config.count_requires_replace_on_clear", func(t *testing.T) {
			config, ok := schema.Attrs["config"]
			if !ok || config.NestedAttrs == nil {
				t.Fatal("expected config with nested attributes")
			}
			count, ok := config.NestedAttrs["count"]
			if !ok {
				t.Fatal("expected config.count attribute")
			}
			if !count.RequiresReplaceOnClear {
				t.Error("expected config.count.RequiresReplaceOnClear = true")
			}
		})

		// Negative control: currency must not be marked replace-on-clear.
		t.Run("currency_not_replace_on_clear", func(t *testing.T) {
			if info, ok := schema.Attrs["currency"]; ok && info.RequiresReplaceOnClear {
				t.Error("expected currency.RequiresReplaceOnClear = false")
			}
		})
	}
}

// TestSchemaParserLiteralOverrideParity requires an attribute property to parse
// the same way whether it is written as a literal in a generated schema, as a
// full replacement in Schema(), or as an in-place edit in Schema(). The
// fixtures live in example_gen/parity_gen.go and example_provider/parity.go.
func TestSchemaParserLiteralOverrideParity(t *testing.T) {
	testdata := analysistest.TestData()
	results := analysistest.RunWithSuggestedFixes(t, testdata, schemaparser.Analyzer, "example_gen", "example_provider")

	var schema *schemaparser.SchemaInfo
	for _, r := range results {
		if r.Pass.Pkg.Name() != "example_provider" {
			continue
		}
		facts, ok := r.Result.(*schemaparser.SchemaFacts)
		if !ok {
			t.Fatalf("expected *SchemaFacts, got %T", r.Result)
		}
		schema = facts.Schemas["ParityResourceSchema"]
	}
	if schema == nil {
		t.Fatal("expected to find ParityResourceSchema in merged facts")
	}

	properties := []struct {
		name    string
		class   schemaparser.FieldClass
		modsLen int
		// inplace is false for properties the parser does not read from
		// modify-in-place blocks.
		inplace bool
	}{
		{"required", schemaparser.Required, 0, true},
		{"optional", schemaparser.Optional, 0, true},
		{"computed", schemaparser.ComputedOnly, 0, true},
		{"optional_computed", schemaparser.OptionalComputed, 0, true},
		{"plan_modifiers", schemaparser.OptionalComputed, 2, true},
		{"default", schemaparser.OptionalComputed, 0, false},
	}

	for _, p := range properties {
		t.Run(p.name, func(t *testing.T) {
			lit, ok := schema.Attrs[p.name+"_lit"]
			if !ok {
				t.Fatalf("attribute %s_lit not found", p.name)
			}

			// Guard against a vacuous comparison: the reference form must
			// itself carry the property under test.
			if lit.Class != p.class {
				t.Errorf("%s_lit: got class %s, want %s", p.name, lit.Class, p.class)
			}
			if len(lit.PlanModifiers) != p.modsLen {
				t.Errorf("%s_lit: got %d plan modifiers, want %d", p.name, len(lit.PlanModifiers), p.modsLen)
			}
			if lit.HasDefault != (p.name == "default") {
				t.Errorf("%s_lit: got HasDefault=%v", p.name, lit.HasDefault)
			}

			forms := []string{"full"}
			if p.inplace {
				forms = append(forms, "inplace")
			}
			for _, form := range forms {
				got, ok := schema.Attrs[p.name+"_"+form]
				if !ok {
					t.Fatalf("attribute %s_%s not found", p.name, form)
				}
				if !reflect.DeepEqual(lit, got) {
					t.Errorf("%s_%s differs from %s_lit:\n got: %+v\nwant: %+v", p.name, form, p.name, got, lit)
				}
			}
		})
	}
}
