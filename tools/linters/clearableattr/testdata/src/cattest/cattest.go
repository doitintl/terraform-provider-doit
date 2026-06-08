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
		attr.PlanModifiers = append(attr.PlanModifiers, useNullForUnknownWhenConfigNull())
		s.Attributes["description"] = attr
	}

	// folder_id: clearable.
	if attr, ok := s.Attributes["folder_id"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, useNullForUnknownWhenConfigNull())
		s.Attributes["folder_id"] = attr
	}

	// public: clearable + UseStateForUnknown.
	if attr, ok := s.Attributes["public"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers,
			useNullForUnknownWhenConfigNull(),
			stringplanmodifier.UseStateForUnknown(),
		)
		s.Attributes["public"] = attr
	}

	// config.currency: clearable via direct nested modifier.
	if attr, ok := s.Attributes["config"].(schema.SingleNestedAttribute); ok {
		if nested, ok := attr.Attributes["currency"].(schema.StringAttribute); ok {
			nested.PlanModifiers = append(nested.PlanModifiers, useNullForUnknownWhenConfigNull())
			attr.Attributes["currency"] = nested
		}
		s.Attributes["config"] = attr
	}

	// results[*].external_id, external_url, metadata: clearable via loop.
	if attr, ok := s.Attributes["results"].(schema.ListNestedAttribute); ok {
		for _, field := range []string{"external_id", "external_url", "metadata"} {
			if nested, ok := attr.NestedObject.Attributes[field].(schema.StringAttribute); ok {
				nested.PlanModifiers = append(nested.PlanModifiers, useNullForUnknownWhenConfigNull())
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

// useNullForUnknownWhenConfigNull is a stub for the plan modifier.
func useNullForUnknownWhenConfigNull() stringplanmodifier.PlanModifier {
	return stringplanmodifier.PlanModifier{}
}
