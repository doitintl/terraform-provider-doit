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
	var replacement, idHint string
	switch value {
	case "attribution":
		replacement = "allocation_rule"
		// The API pairs each type with its own id and rejects a mix
		// (400 for alert and budget scopes, 404 for dimension lookups).
		idHint = ` If this block also sets "id", change it from "attribution" to "allocation_rule" as well.`
	case "attribution_group":
		// The id is the allocation ID, which does not change.
		replacement = "allocation"
	default:
		return
	}

	resp.Diagnostics.AddAttributeWarning(
		req.Path,
		deprecatedDimensionsTypeWarningSummary,
		fmt.Sprintf(
			"Dimension type %q is deprecated and will soon no longer be supported by the DoiT API. Use %q instead.%s",
			value,
			replacement,
			idHint,
		),
	)
}

// appendDimensionsTypeDeprecationValidators adds the shared warning validator
// to generated resource schema attributes without modifying generated files.
// Paths use dotted notation and [*] for nested collection elements.
func appendDimensionsTypeDeprecationValidators(attributes map[string]schema.Attribute, attributePaths ...string) {
	updateStringAttributesAtPaths(attributes, "appendDimensionsTypeDeprecationValidators", func(attribute schema.StringAttribute) schema.StringAttribute {
		attribute.Validators = append(attribute.Validators, deprecatedDimensionsTypeValidator{})
		return attribute
	}, attributePaths...)
}

// updateStringAttributesAtPaths applies update to the string attribute at each
// path and panics when a path does not resolve to a string attribute, so a
// renamed generated attribute fails loudly instead of silently losing its
// hand-written additions. caller names the public helper in the panic message.
func updateStringAttributesAtPaths(
	attributes map[string]schema.Attribute,
	caller string,
	update func(schema.StringAttribute) schema.StringAttribute,
	attributePaths ...string,
) {
	for _, attributePath := range attributePaths {
		if updateStringAttributeAtPath(attributes, strings.Split(attributePath, "."), update) {
			continue
		}

		panic(fmt.Sprintf(
			"%s: string attribute path %q not found in schema",
			caller,
			attributePath,
		))
	}
}

func updateStringAttributeAtPath(attributes map[string]schema.Attribute, segments []string, update func(schema.StringAttribute) schema.StringAttribute) bool {
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

		attributes[name] = update(stringAttribute)
		return true
	}

	switch nestedAttribute := attribute.(type) {
	case schema.SingleNestedAttribute:
		return updateStringAttributeAtPath(nestedAttribute.Attributes, segments[1:], update)
	case schema.ListNestedAttribute:
		return updateStringAttributeAtPath(nestedAttribute.NestedObject.Attributes, segments[1:], update)
	case schema.SetNestedAttribute:
		return updateStringAttributeAtPath(nestedAttribute.NestedObject.Attributes, segments[1:], update)
	case schema.MapNestedAttribute:
		return updateStringAttributeAtPath(nestedAttribute.NestedObject.Attributes, segments[1:], update)
	default:
		return false
	}
}
