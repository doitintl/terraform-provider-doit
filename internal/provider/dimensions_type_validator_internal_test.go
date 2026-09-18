package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestDeprecatedDimensionsTypeValidator(t *testing.T) {
	t.Parallel()

	testPath := path.Root("config").AtName("type")
	tests := []struct {
		name       string
		value      types.String
		wantDetail string
	}{
		{
			name:  "null",
			value: types.StringNull(),
		},
		{
			name:  "unknown",
			value: types.StringUnknown(),
		},
		{
			name:  "ordinary dimension type",
			value: types.StringValue("fixed"),
		},
		{
			name:  "allocation",
			value: types.StringValue("allocation"),
		},
		{
			name:  "allocation rule",
			value: types.StringValue("allocation_rule"),
		},
		{
			name:       "attribution",
			value:      types.StringValue("attribution"),
			wantDetail: `Dimension type "attribution" is deprecated and will soon no longer be supported by the DoiT API. Use "allocation_rule" instead.`,
		},
		{
			name:       "attribution group",
			value:      types.StringValue("attribution_group"),
			wantDetail: `Dimension type "attribution_group" is deprecated and will soon no longer be supported by the DoiT API. Use "allocation" instead.`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			request := validator.StringRequest{
				Path:        testPath,
				ConfigValue: test.value,
			}
			response := &validator.StringResponse{}

			deprecatedDimensionsTypeValidator{}.ValidateString(t.Context(), request, response)

			if response.Diagnostics.HasError() {
				t.Fatalf("unexpected errors: %v", response.Diagnostics.Errors())
			}

			wantWarnings := 0
			if test.wantDetail != "" {
				wantWarnings = 1
			}
			if got := response.Diagnostics.WarningsCount(); got != wantWarnings {
				t.Fatalf("warnings = %d, want %d: %v", got, wantWarnings, response.Diagnostics)
			}
			if wantWarnings == 0 {
				return
			}

			diagnostic := response.Diagnostics.Warnings()[0]
			if got := diagnostic.Severity(); got != diag.SeverityWarning {
				t.Errorf("severity = %v, want %v", got, diag.SeverityWarning)
			}
			if got := diagnostic.Summary(); got != deprecatedDimensionsTypeWarningSummary {
				t.Errorf("summary = %q, want %q", got, deprecatedDimensionsTypeWarningSummary)
			}
			if got := diagnostic.Detail(); got != test.wantDetail {
				t.Errorf("detail = %q, want %q", got, test.wantDetail)
			}

			withPath, ok := diagnostic.(diag.DiagnosticWithPath)
			if !ok {
				t.Fatalf("diagnostic is not associated with an attribute path: %T", diagnostic)
			}
			if got := withPath.Path(); !got.Equal(testPath) {
				t.Errorf("path = %s, want %s", got, testPath)
			}
		})
	}
}

func TestDeprecatedDimensionsTypeValidatorResourceSchemaWiring(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	resourceSchemas := map[string]struct {
		schema resourceschema.Schema
		paths  []string
	}{
		"alert": {
			schema: resourceSchemaForTest(ctx, &alertResource{}),
			paths:  []string{"config.scopes[*].type"},
		},
		"budget": {
			schema: resourceSchemaForTest(ctx, &budgetResource{}),
			paths:  []string{"scopes[*].type"},
		},
		"report": {
			schema: resourceSchemaForTest(ctx, &reportResource{}),
			paths:  reportDimensionsTypeAttributePaths,
		},
	}

	for name, test := range resourceSchemas {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			for _, attributePath := range test.paths {
				attribute := resourceStringAttributeAtPath(t, test.schema.Attributes, attributePath)
				assertHasDeprecatedDimensionsTypeValidator(t, attribute.Validators)
			}
		})
	}
}

func TestDeprecatedDimensionsTypeValidatorDataSourceSchemaWiring(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	t.Run("dimension", func(t *testing.T) {
		t.Parallel()

		var response datasource.SchemaResponse
		(&dimensionDataSource{}).Schema(ctx, datasource.SchemaRequest{}, &response)
		if response.Diagnostics.HasError() {
			t.Fatalf("building dimension schema: %v", response.Diagnostics.Errors())
		}

		attribute := dataSourceStringAttributeAtPath(t, response.Schema.Attributes, "type")
		assertHasDeprecatedDimensionsTypeValidator(t, attribute.Validators)
	})

	t.Run("report query", func(t *testing.T) {
		t.Parallel()

		var response datasource.SchemaResponse
		(&reportQueryDataSource{}).Schema(ctx, datasource.SchemaRequest{}, &response)
		if response.Diagnostics.HasError() {
			t.Fatalf("building report query schema: %v", response.Diagnostics.Errors())
		}

		for _, attributePath := range reportDimensionsTypeAttributePaths {
			attribute := dataSourceStringAttributeAtPath(t, response.Schema.Attributes, attributePath)
			assertHasDeprecatedDimensionsTypeValidator(t, attribute.Validators)
		}
	})
}

type resourceWithSchema interface {
	Schema(context.Context, resource.SchemaRequest, *resource.SchemaResponse)
}

func resourceSchemaForTest(ctx context.Context, providerResource resourceWithSchema) resourceschema.Schema {
	var response resource.SchemaResponse
	providerResource.Schema(ctx, resource.SchemaRequest{}, &response)
	return response.Schema
}

func resourceStringAttributeAtPath(t *testing.T, attributes map[string]resourceschema.Attribute, attributePath string) resourceschema.StringAttribute {
	t.Helper()

	segments := strings.Split(attributePath, ".")
	for index, segment := range segments {
		name := strings.TrimSuffix(segment, "[*]")
		attribute, ok := attributes[name]
		if !ok {
			t.Fatalf("resource attribute %q not found while resolving %q", name, attributePath)
		}

		if index == len(segments)-1 {
			stringAttribute, ok := attribute.(resourceschema.StringAttribute)
			if !ok {
				t.Fatalf("resource attribute %q has type %T, want schema.StringAttribute", attributePath, attribute)
			}
			return stringAttribute
		}

		switch nestedAttribute := attribute.(type) {
		case resourceschema.SingleNestedAttribute:
			attributes = nestedAttribute.Attributes
		case resourceschema.ListNestedAttribute:
			attributes = nestedAttribute.NestedObject.Attributes
		default:
			t.Fatalf("resource attribute %q has non-nested type %T while resolving %q", name, attribute, attributePath)
		}
	}

	t.Fatalf("resource attribute path %q did not resolve", attributePath)
	return resourceschema.StringAttribute{}
}

func dataSourceStringAttributeAtPath(t *testing.T, attributes map[string]datasourceschema.Attribute, attributePath string) datasourceschema.StringAttribute {
	t.Helper()

	segments := strings.Split(attributePath, ".")
	for index, segment := range segments {
		name := strings.TrimSuffix(segment, "[*]")
		attribute, ok := attributes[name]
		if !ok {
			t.Fatalf("data source attribute %q not found while resolving %q", name, attributePath)
		}

		if index == len(segments)-1 {
			stringAttribute, ok := attribute.(datasourceschema.StringAttribute)
			if !ok {
				t.Fatalf("data source attribute %q has type %T, want schema.StringAttribute", attributePath, attribute)
			}
			return stringAttribute
		}

		switch nestedAttribute := attribute.(type) {
		case datasourceschema.SingleNestedAttribute:
			attributes = nestedAttribute.Attributes
		case datasourceschema.ListNestedAttribute:
			attributes = nestedAttribute.NestedObject.Attributes
		default:
			t.Fatalf("data source attribute %q has non-nested type %T while resolving %q", name, attribute, attributePath)
		}
	}

	t.Fatalf("data source attribute path %q did not resolve", attributePath)
	return datasourceschema.StringAttribute{}
}

func assertHasDeprecatedDimensionsTypeValidator(t *testing.T, validators []validator.String) {
	t.Helper()

	count := 0
	for _, candidate := range validators {
		if _, ok := candidate.(deprecatedDimensionsTypeValidator); ok {
			count++
		}
	}
	if count != 1 {
		t.Errorf("deprecated dimension type validator count = %d, want 1", count)
	}
}
