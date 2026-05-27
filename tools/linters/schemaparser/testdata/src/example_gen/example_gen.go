// want package:"ExampleResourceSchema"

package example_gen

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func ExampleResourceSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The unique identifier.",
			},
			"create_time": schema.StringAttribute{
				Computed:    true,
				Description: "Creation time.",
			},
			"update_time": schema.StringAttribute{
				Computed:    true,
				Description: "Update time.",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The name.",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "The description.",
			},
			"amount": schema.Float64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "The amount.",
			},
			"currency": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Currency code.",
			},
			"tags": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Description: "Tags.",
			},
			"alerts": schema.ListNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"percentage": schema.Float64Attribute{
							Optional: true,
							Computed: true,
						},
						"triggered": schema.BoolAttribute{
							Computed: true,
						},
						"threshold": schema.Float64Attribute{
							Required: true,
						},
					},
				},
				Optional:    true,
				Computed:    true,
				Description: "List of alerts.",
			},
		},
	}
}

type ExampleModel struct {
	Id          types.String  `tfsdk:"id"`
	CreateTime  types.String  `tfsdk:"create_time"`
	UpdateTime  types.String  `tfsdk:"update_time"`
	Name        types.String  `tfsdk:"name"`
	Description types.String  `tfsdk:"description"`
	Amount      types.Float64 `tfsdk:"amount"`
	Currency    types.String  `tfsdk:"currency"`
	Tags        types.List    `tfsdk:"tags"`
	Alerts      types.List    `tfsdk:"alerts"`
}
