package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

const deprecatedDimensionsTypeWarningSummary = "Deprecated Dimension Type"

var (
	_ validator.String = deprecatedDimensionsTypeValidator{}

	reportDimensionsTypeAttributePaths = []string{
		"config.dimensions[*].type",
		"config.filters[*].type",
		"config.group[*].type",
	}
)

// deprecatedDimensionsTypeValidator warns users when they configure a legacy
// attribution dimension type that has been superseded by allocations.
type deprecatedDimensionsTypeValidator struct{}

func (v deprecatedDimensionsTypeValidator) Description(_ context.Context) string {
	return "Warns when a deprecated attribution dimension type is configured."
}

func (v deprecatedDimensionsTypeValidator) MarkdownDescription(_ context.Context) string {
	return "Warns when a deprecated attribution dimension type is configured."
}

func (v deprecatedDimensionsTypeValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := req.ConfigValue.ValueString()
	var replacement string
	switch value {
	case "attribution":
		replacement = "allocation_rule"
	case "attribution_group":
		replacement = "allocation"
	default:
		return
	}

	resp.Diagnostics.AddAttributeWarning(
		req.Path,
		deprecatedDimensionsTypeWarningSummary,
		fmt.Sprintf(
			"Dimension type %q is deprecated and will soon no longer be supported by the DoiT API. Use %q instead.",
			value,
			replacement,
		),
	)
}

// appendDimensionsTypeDeprecationValidators adds the shared warning validator
// to generated resource schema attributes without modifying generated files.
// Paths use dotted notation and [*] for nested collection elements.
func appendDimensionsTypeDeprecationValidators(attributes map[string]schema.Attribute, attributePaths ...string) {
	for _, attributePath := range attributePaths {
		if appendDimensionsTypeDeprecationValidatorAtPath(attributes, strings.Split(attributePath, ".")) {
			continue
		}

		panic(fmt.Sprintf(
			"appendDimensionsTypeDeprecationValidators: string attribute path %q not found in schema",
			attributePath,
		))
	}
}

func appendDimensionsTypeDeprecationValidatorAtPath(attributes map[string]schema.Attribute, segments []string) bool {
	name := strings.TrimSuffix(segments[0], "[*]")
	attribute, ok := attributes[name]
	if !ok {
		return false
	}

	if len(segments) == 1 {
		stringAttribute, ok := attribute.(schema.StringAttribute)
		if !ok {
			return false
		}

		stringAttribute.Validators = append(stringAttribute.Validators, deprecatedDimensionsTypeValidator{})
		attributes[name] = stringAttribute
		return true
	}

	switch nestedAttribute := attribute.(type) {
	case schema.SingleNestedAttribute:
		return appendDimensionsTypeDeprecationValidatorAtPath(nestedAttribute.Attributes, segments[1:])
	case schema.ListNestedAttribute:
		return appendDimensionsTypeDeprecationValidatorAtPath(nestedAttribute.NestedObject.Attributes, segments[1:])
	case schema.SetNestedAttribute:
		return appendDimensionsTypeDeprecationValidatorAtPath(nestedAttribute.NestedObject.Attributes, segments[1:])
	case schema.MapNestedAttribute:
		return appendDimensionsTypeDeprecationValidatorAtPath(nestedAttribute.NestedObject.Attributes, segments[1:])
	default:
		return false
	}
}
