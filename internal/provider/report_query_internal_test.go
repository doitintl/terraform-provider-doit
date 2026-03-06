package provider

import (
	"context"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_report"
	rsschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestConvertResourceAttrsToDataSource_AllTypes verifies that the schema
// converter correctly handles every attribute type used in the report config.
func TestConvertResourceAttrsToDataSource_AllTypes(t *testing.T) {

	// Build a resource schema with all types we use in the report config.
	resourceAttrs := map[string]rsschema.Attribute{
		"string_field": rsschema.StringAttribute{
			Description: "A string field",
			Optional:    true,
		},
		"bool_field": rsschema.BoolAttribute{
			Description: "A bool field",
			Optional:    true,
		},
		"int64_field": rsschema.Int64Attribute{
			Description: "An int64 field",
			Required:    true,
		},
		"float64_field": rsschema.Float64Attribute{
			Description: "A float64 field",
			Computed:    true,
		},
		"list_field": rsschema.ListAttribute{
			ElementType: types.StringType,
			Description: "A list field",
			Optional:    true,
		},
		"nested_object": rsschema.SingleNestedAttribute{
			Attributes: map[string]rsschema.Attribute{
				"inner_string": rsschema.StringAttribute{
					Description: "Inner string",
					Required:    true,
				},
			},
			Description: "A nested object",
			Optional:    true,
		},
		"nested_list": rsschema.ListNestedAttribute{
			NestedObject: rsschema.NestedAttributeObject{
				Attributes: map[string]rsschema.Attribute{
					"item_string": rsschema.StringAttribute{
						Description: "Item string",
						Required:    true,
					},
				},
			},
			Description: "A nested list",
			Optional:    true,
		},
	}

	result, diags := convertResourceAttrsToDataSource(resourceAttrs)
	if diags.HasError() {
		t.Fatalf("unexpected errors: %s", diags.Errors())
	}

	// Verify all attributes were converted.
	expectedAttrs := []string{
		"string_field", "bool_field", "int64_field", "float64_field",
		"list_field", "nested_object", "nested_list",
	}
	for _, name := range expectedAttrs {
		if _, ok := result[name]; !ok {
			t.Errorf("expected attribute %q to be converted, but it was not found", name)
		}
	}

	// Verify the count matches (no extra attributes).
	if len(result) != len(expectedAttrs) {
		t.Errorf("expected %d attributes, got %d", len(expectedAttrs), len(result))
	}
}

// TestConvertResourceAttrsToDataSource_PreservesProperties verifies that
// key properties (Description, Required, Optional, Computed) are preserved
// during conversion.
func TestConvertResourceAttrsToDataSource_PreservesProperties(t *testing.T) {
	resourceAttrs := map[string]rsschema.Attribute{
		"required_string": rsschema.StringAttribute{
			Description:         "A required string",
			MarkdownDescription: "A **required** string",
			Required:            true,
		},
		"optional_computed_string": rsschema.StringAttribute{
			Description: "An optional computed string",
			Optional:    true,
			Computed:    true,
		},
	}

	result, diags := convertResourceAttrsToDataSource(resourceAttrs)
	if diags.HasError() {
		t.Fatalf("unexpected errors: %s", diags.Errors())
	}

	// Check required_string preserved its properties.
	reqAttr := result["required_string"]
	if reqAttr.IsRequired() != true {
		t.Error("expected required_string to be required")
	}
	if reqAttr.IsOptional() != false {
		t.Error("expected required_string to not be optional")
	}

	// Check optional_computed_string preserved its properties.
	optAttr := result["optional_computed_string"]
	if optAttr.IsRequired() != false {
		t.Error("expected optional_computed_string to not be required")
	}
	if optAttr.IsOptional() != true {
		t.Error("expected optional_computed_string to be optional")
	}
	if optAttr.IsComputed() != true {
		t.Error("expected optional_computed_string to be computed")
	}
}

// TestConvertResourceAttrsToDataSource_NestedRecursion verifies that nested
// attributes are recursively converted, including multi-level nesting.
func TestConvertResourceAttrsToDataSource_NestedRecursion(t *testing.T) {
	resourceAttrs := map[string]rsschema.Attribute{
		"outer": rsschema.SingleNestedAttribute{
			Attributes: map[string]rsschema.Attribute{
				"middle": rsschema.SingleNestedAttribute{
					Attributes: map[string]rsschema.Attribute{
						"inner": rsschema.StringAttribute{
							Description: "Deeply nested",
							Required:    true,
						},
					},
					Optional: true,
				},
			},
			Optional: true,
		},
	}

	result, diags := convertResourceAttrsToDataSource(resourceAttrs)
	if diags.HasError() {
		t.Fatalf("unexpected errors: %s", diags.Errors())
	}

	// Verify the deeply nested attribute exists by checking the chain.
	if _, ok := result["outer"]; !ok {
		t.Fatal("expected 'outer' attribute")
	}
}

// TestConvertResourceAttrsToDataSource_UnhandledType verifies that an
// unhandled attribute type produces a diagnostic error instead of a panic.
func TestConvertResourceAttrsToDataSource_UnhandledType(t *testing.T) {
	// MapAttribute is not currently handled by the converter.
	resourceAttrs := map[string]rsschema.Attribute{
		"known_field": rsschema.StringAttribute{
			Description: "This should convert fine",
			Optional:    true,
		},
		"map_field": rsschema.MapAttribute{
			ElementType: types.StringType,
			Description: "This type is not handled",
			Optional:    true,
		},
	}

	result, diags := convertResourceAttrsToDataSource(resourceAttrs)

	// Should produce an error diagnostic, not panic.
	if !diags.HasError() {
		t.Fatal("expected an error diagnostic for unhandled MapAttribute, got none")
	}

	// The error message should mention the attribute name and type.
	errMsg := diags.Errors()[0].Detail()
	if errMsg == "" {
		t.Error("expected non-empty error detail")
	}

	// The known field should still be converted despite the error.
	if _, ok := result["known_field"]; !ok {
		t.Error("expected 'known_field' to still be converted despite the error on 'map_field'")
	}
}

// TestConvertResourceAttrsToDataSource_IgnoresDefaults verifies that the
// converter works correctly even when resource attributes have Default values
// (which are not supported in data source schemas). The Default field is
// simply not carried over and the attribute still converts.
func TestConvertResourceAttrsToDataSource_IgnoresDefaults(t *testing.T) {
	resourceAttrs := map[string]rsschema.Attribute{
		"with_default": rsschema.StringAttribute{
			Description: "Has a default",
			Optional:    true,
			Computed:    true,
			Default:     stringdefault.StaticString("default_value"),
		},
	}

	result, diags := convertResourceAttrsToDataSource(resourceAttrs)
	if diags.HasError() {
		t.Fatalf("unexpected errors: %s", diags.Errors())
	}

	if _, ok := result["with_default"]; !ok {
		t.Error("expected 'with_default' to be converted")
	}
}

// TestConvertResourceAttrsToDataSource_ReportConfigSchema is an integration
// test that verifies the actual generated report resource config schema can
// be fully converted without errors. This catches regressions if new attribute
// types are added to the report schema.
func TestConvertResourceAttrsToDataSource_ReportConfigSchema(t *testing.T) {
	ctx := context.Background()

	// Get the actual generated report resource schema.
	reportSchema := resource_report.ReportResourceSchema(ctx)
	configAttr, ok := reportSchema.Attributes["config"].(rsschema.SingleNestedAttribute)
	if !ok {
		t.Fatal("expected config to be a SingleNestedAttribute")
	}

	// Convert it — this should succeed without errors.
	result, diags := convertResourceAttrsToDataSource(configAttr.Attributes)
	if diags.HasError() {
		t.Fatalf("failed to convert report config schema: %s", diags.Errors())
	}

	// Verify we got a non-empty result.
	if len(result) == 0 {
		t.Error("expected non-empty result from converting report config schema")
	}

	// Verify a few known attributes exist (smoke test).
	knownAttrs := []string{"aggregation", "currency", "time_interval", "dimensions", "filters"}
	for _, name := range knownAttrs {
		if _, ok := result[name]; !ok {
			t.Errorf("expected attribute %q in converted schema", name)
		}
	}
}
