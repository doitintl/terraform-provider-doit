package example_provider

import (
	"context"

	"example_gen"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

type parityResource struct{}

// Schema rewrites every *_full attribute as a full replacement literal and
// every *_inplace attribute with modify-in-place edits, so that each ends up
// equal to its *_lit counterpart in the generated schema.
//
// There is no default_inplace: the parser does not read `attr.Default = ...`
// in modify-in-place blocks and the provider never writes one. The override
// forms for Default are covered by default_full only.
func (r *parityResource) Schema(ctx context.Context, req schemaRequest, resp *schemaResponse) {
	s := example_gen.ParityResourceSchema(ctx)

	s.Attributes["required_full"] = schema.StringAttribute{Required: true}
	s.Attributes["optional_full"] = schema.StringAttribute{Optional: true}
	s.Attributes["computed_full"] = schema.StringAttribute{Computed: true}
	s.Attributes["optional_computed_full"] = schema.StringAttribute{
		Optional: true,
		Computed: true,
	}
	s.Attributes["plan_modifiers_full"] = schema.StringAttribute{
		Optional: true,
		Computed: true,
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.UseStateForUnknown(),
			stringplanmodifier.RequiresReplace(),
		},
	}
	s.Attributes["default_full"] = schema.StringAttribute{
		Optional: true,
		Computed: true,
		Default:  stringdefault.StaticString("v"),
	}

	if attr, ok := s.Attributes["required_inplace"].(schema.StringAttribute); ok {
		attr.Required = true
		attr.Computed = false
		s.Attributes["required_inplace"] = attr
	}
	if attr, ok := s.Attributes["optional_inplace"].(schema.StringAttribute); ok {
		attr.Required = false
		attr.Optional = true
		s.Attributes["optional_inplace"] = attr
	}
	if attr, ok := s.Attributes["computed_inplace"].(schema.StringAttribute); ok {
		attr.Optional = false
		attr.Computed = true
		s.Attributes["computed_inplace"] = attr
	}
	if attr, ok := s.Attributes["optional_computed_inplace"].(schema.StringAttribute); ok {
		attr.Required = false
		attr.Optional = true
		attr.Computed = true
		s.Attributes["optional_computed_inplace"] = attr
	}
	if attr, ok := s.Attributes["plan_modifiers_inplace"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.RequiresReplace())
		s.Attributes["plan_modifiers_inplace"] = attr
	}

	resp.Schema = s
}
