package usftest

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
)

// --- BAD: Schema method missing UseStateForUnknown on id and create_time ---

type badResource struct{}

func (r *badResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) { // want `Computed-only field "create_time" should have UseStateForUnknown` `Computed-only field "id" should have UseStateForUnknown`
	s := BadResourceSchema(ctx)
	resp.Schema = s
}

// --- GOOD: Schema method adds UseStateForUnknown to id and create_time ---

type goodResource struct{}

func (r *goodResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := GoodResourceSchema(ctx)

	attr := s.Attributes["id"].(schema.StringAttribute)
	attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
	s.Attributes["id"] = attr

	ctAttr := s.Attributes["create_time"].(schema.Int64Attribute)
	ctAttr.PlanModifiers = append(ctAttr.PlanModifiers, int64planmodifier.UseStateForUnknown())
	s.Attributes["create_time"] = ctAttr

	resp.Schema = s
}
