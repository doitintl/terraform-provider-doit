package example_provider

import (
	"context"

	"example_gen"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

type exampleResource struct{}

// Schema method that overrides the generated schema — exercises all 3 patterns.
func (r *exampleResource) Schema(ctx context.Context, req schemaRequest, resp *schemaResponse) {
	s := example_gen.ExampleResourceSchema(ctx)

	// Pattern 1: Full replacement — change id from Computed-only to Required.
	s.Attributes["id"] = schema.StringAttribute{
		Required:    true,
		Description: "The unique identifier (user-provided).",
	}

	// Pattern 2: Deletion — remove update_time.
	delete(s.Attributes, "update_time")

	// Pattern 3: Modify-in-place — change amount from Optional+Computed to Required.
	if attr, ok := s.Attributes["amount"].(schema.Float64Attribute); ok {
		attr.Required = true
		attr.Optional = false
		attr.Computed = false
		s.Attributes["amount"] = attr
	}

	// Pattern 3b: Modify-in-place — no classification change (just PlanModifiers).
	if attr, ok := s.Attributes["name"].(schema.StringAttribute); ok {
		_ = attr // just PlanModifiers, no Required/Optional/Computed changes
		s.Attributes["name"] = attr
	}

	// Pattern 4: Add a new field not in the generated schema.
	s.Attributes["display_name"] = schema.StringAttribute{
		Computed:    true,
		Description: "The display name.",
	}

	// Pattern 5: Add a new Optional field.
	s.Attributes["phone"] = schema.StringAttribute{
		Optional:    true,
		Description: "Phone number.",
	}

	resp.Schema = s
}

// Stubs.
type schemaRequest struct{}
type schemaResponse struct{ Schema schema.Schema }
