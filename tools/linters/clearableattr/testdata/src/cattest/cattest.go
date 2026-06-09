package cattest

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// --- BAD: Schema() with unclassified Optional+Computed fields ---

type badResource struct{}

func (r *badResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) { // want `Optional\+Computed attribute .* has no clearable classification` `Optional\+Computed attribute .* has no clearable classification` `Optional\+Computed attribute .* has no clearable classification` `Optional\+Computed attribute .* has no clearable classification`
	s := BadResourceSchema(ctx) // want `Optional\+Computed attribute .* has no clearable classification`

	// id gets UseStateForUnknown — not relevant to this linter.
	if attr, ok := s.Attributes["id"].(schema.StringAttribute); ok { // want `Optional\+Computed attribute .* has no clearable classification`
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["id"] = attr
	}

	resp.Schema = s // want `Optional\+Computed attribute .* has no clearable classification`
}

// --- GOOD: Schema() with all O+C fields classified via modifier ---

type goodResource struct{}

func (r *goodResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := GoodResourceSchema(ctx)

	// description: clearable.
	if attr, ok := s.Attributes["description"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, useEmptyForUnknownWhenConfigNull())
		s.Attributes["description"] = attr
	}

	// folder_id: clearable.
	if attr, ok := s.Attributes["folder_id"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, useEmptyForUnknownWhenConfigNull())
		s.Attributes["folder_id"] = attr
	}

	// public: clearable + UseStateForUnknown.
	if attr, ok := s.Attributes["public"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers,
			useEmptyForUnknownWhenConfigNull(),
			stringplanmodifier.UseStateForUnknown(),
		)
		s.Attributes["public"] = attr
	}

	// config.currency: clearable via direct nested modifier.
	if attr, ok := s.Attributes["config"].(schema.SingleNestedAttribute); ok {
		if nested, ok := attr.Attributes["currency"].(schema.StringAttribute); ok {
			nested.PlanModifiers = append(nested.PlanModifiers, useEmptyForUnknownWhenConfigNull())
			attr.Attributes["currency"] = nested
		}
		s.Attributes["config"] = attr
	}

	// results[*].external_id, external_url, metadata: clearable via loop.
	if attr, ok := s.Attributes["results"].(schema.ListNestedAttribute); ok {
		for _, field := range []string{"external_id", "external_url", "metadata"} {
			if nested, ok := attr.NestedObject.Attributes[field].(schema.StringAttribute); ok {
				nested.PlanModifiers = append(nested.PlanModifiers, useEmptyForUnknownWhenConfigNull())
				attr.NestedObject.Attributes[field] = nested
			}
		}
		s.Attributes["results"] = attr
	}

	resp.Schema = s
}

// --- GOOD: Schema() where O+C attribute is overridden to Required ---

type overrideResource struct{}

func (r *overrideResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := OverrideResourceSchema(ctx)

	// description: overridden from O+C to Required — no longer needs classification.
	if attr, ok := s.Attributes["description"].(schema.StringAttribute); ok {
		attr.Required = true
		attr.Optional = false
		attr.Computed = false
		s.Attributes["description"] = attr
	}

	// folder_id: overridden from O+C to Required.
	if attr, ok := s.Attributes["folder_id"].(schema.StringAttribute); ok {
		attr.Required = true
		attr.Optional = false
		attr.Computed = false
		s.Attributes["folder_id"] = attr
	}

	// public: overridden from O+C to Optional (no Computed).
	if attr, ok := s.Attributes["public"].(schema.StringAttribute); ok {
		attr.Optional = true
		attr.Computed = false
		s.Attributes["public"] = attr
	}

	// config: delete to avoid nested findings.
	delete(s.Attributes, "config")
	delete(s.Attributes, "results")

	resp.Schema = s
}

// --- GOOD: Schema() with Cat B attributes acknowledged via allowlist ---

type acknowledgedResource struct{}

func (r *acknowledgedResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := AcknowledgedResourceSchema(ctx)

	// Cat A: clearable via modifier.
	if attr, ok := s.Attributes["description"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, useEmptyForUnknownWhenConfigNull())
		s.Attributes["description"] = attr
	}

	// Cat B: acknowledged as not clearable — replaces no-op if-blocks.
	acknowledgeNotClearable(s,
		"folder_id",              // API-assigned
		"public",                 // API defaults to false
		"config.currency",        // API defaults to org currency
		"results[*].external_id", // API-assigned identity
		"results[*].external_url", // API-assigned
		"results[*].metadata",    // API-assigned
	)

	resp.Schema = s
}

// useEmptyForUnknownWhenConfigNull is a stub for the plan modifier.
func useEmptyForUnknownWhenConfigNull() stringplanmodifier.PlanModifier {
	return stringplanmodifier.PlanModifier{}
}

// acknowledgeNotClearable is a stub for the runtime helper.
func acknowledgeNotClearable(_ schema.Schema, _ ...string) {}
