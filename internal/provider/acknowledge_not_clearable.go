package provider

import (
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// acknowledgeNotClearable marks the given attribute paths as explicitly not
// user-clearable. This satisfies the clearableattr linter for Optional+Computed
// attributes that should preserve their API-assigned value when removed from
// config (Category B attributes).
//
// Paths use dotted notation for nested attributes and [*] for list elements:
//
//	acknowledgeNotClearable(s,
//	    "recipients",              // top-level attribute
//	    "config.currency",         // SingleNestedAttribute child
//	    "scopes[*].inverse",       // ListNestedAttribute child
//	)
//
// At runtime, this function validates that every named attribute exists in the
// schema. It panics if an attribute path is not found, which is caught during
// acceptance tests when Schema() is called during provider initialization.
//
// This is a no-op for plan/apply behavior — it only serves as documentation
// and a linter anchor point.
func acknowledgeNotClearable(s schema.Schema, paths ...string) {
	for _, p := range paths {
		if !schemaAttrExists(s, p) {
			panic(fmt.Sprintf(
				"acknowledgeNotClearable: attribute path %q not found in schema; "+
					"remove it or fix the path", p))
		}
	}
}

// schemaAttrExists checks if a dotted attribute path exists in the schema.
func schemaAttrExists(s schema.Schema, path string) bool {
	segments := strings.Split(path, ".")
	attrs := s.Attributes

	for i, seg := range segments {
		// Strip [*] suffix — it's a path convention, not part of the attribute name.
		seg = strings.TrimSuffix(seg, "[*]")

		attr, ok := attrs[seg]
		if !ok {
			return false
		}

		// If this is the last segment, the attribute exists.
		if i == len(segments)-1 {
			return true
		}

		// Descend into nested attributes.
		switch a := attr.(type) {
		case schema.SingleNestedAttribute:
			attrs = a.Attributes
		case schema.ListNestedAttribute:
			attrs = a.NestedObject.Attributes
		case schema.SetNestedAttribute:
			attrs = a.NestedObject.Attributes
		case schema.MapNestedAttribute:
			attrs = a.NestedObject.Attributes
		default:
			// Non-nested attribute but path has more segments.
			return false
		}
	}

	return false
}
