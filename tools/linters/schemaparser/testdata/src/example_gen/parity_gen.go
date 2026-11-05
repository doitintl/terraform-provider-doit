package example_gen

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// ParityResourceSchema holds, for each attribute property the parser reads,
// three attributes that must parse identically:
//
//	<property>_lit      the reference form, written as a literal here
//	<property>_full     a placeholder the provider replaces with a literal
//	<property>_inplace  a placeholder the provider edits in place
//
// The placeholders start in a different state than the final one so that a
// parser that ignores the override leaves them visibly different.
func ParityResourceSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"required_lit": schema.StringAttribute{Required: true},
			"required_full": schema.StringAttribute{
				Computed: true,
			},
			"required_inplace": schema.StringAttribute{
				Computed: true,
			},

			"optional_lit": schema.StringAttribute{Optional: true},
			"optional_full": schema.StringAttribute{
				Required: true,
			},
			"optional_inplace": schema.StringAttribute{
				Required: true,
			},

			"computed_lit": schema.StringAttribute{Computed: true},
			"computed_full": schema.StringAttribute{
				Optional: true,
			},
			"computed_inplace": schema.StringAttribute{
				Optional: true,
			},

			"optional_computed_lit": schema.StringAttribute{
				Optional: true,
				Computed: true,
			},
			"optional_computed_full": schema.StringAttribute{
				Required: true,
			},
			"optional_computed_inplace": schema.StringAttribute{
				Required: true,
			},

			"plan_modifiers_lit": schema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"plan_modifiers_full": schema.StringAttribute{
				Optional: true,
				Computed: true,
			},
			"plan_modifiers_inplace": schema.StringAttribute{
				Optional: true,
				Computed: true,
			},

			"default_lit": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("v"),
			},
			"default_full": schema.StringAttribute{
				Optional: true,
				Computed: true,
			},
		},
	}
}
