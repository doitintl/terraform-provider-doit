// Package drifttest is a test fixture for the defaultdrift analyzer.
package drifttest

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure imports are used.
var _ context.Context

// DriftTestResourceSchema returns a test schema with defaulted fields.
func DriftTestResourceSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"name": schema.StringAttribute{
				Required: true,
			},
			// Optional+Computed WITH default — PointerValue is risky.
			"description": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(""),
			},
			"metric": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("cost"),
			},
			"growth_per_period": schema.Float64Attribute{
				Optional: true,
				Computed: true,
				Default:  float64default.StaticFloat64(0),
			},
			"use_prev_spend": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"max_results": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Default:  int64default.StaticInt64(1000),
			},
			// Optional+Computed WITHOUT default — PointerValue is fine.
			"folder_id": schema.StringAttribute{
				Optional: true,
				Computed: true,
			},
			// Computed-only — PointerValue is fine.
			"create_time": schema.Int64Attribute{
				Computed: true,
			},
			// Nested attribute with defaulted fields.
			"config": schema.SingleNestedAttribute{
				Optional: true,
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{
						Optional: true,
						Computed: true,
						Default:  booldefault.StaticBool(false),
					},
					"sort_order": schema.StringAttribute{
						Optional: true,
						Computed: true,
						Default:  stringdefault.StaticString("desc"),
					},
					// No default — PointerValue is fine.
					"label": schema.StringAttribute{
						Optional: true,
						Computed: true,
					},
				},
			},
			// List nested with defaulted fields.
			"items": schema.ListNestedAttribute{
				Optional: true,
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required: true,
						},
						"include_null": schema.BoolAttribute{
							Optional: true,
							Computed: true,
							Default:  booldefault.StaticBool(false),
						},
					},
				},
			},
		},
	}
}

// DriftTestModel is the Terraform model for the test resource.
type DriftTestModel struct {
	Id              types.String
	Name            types.String
	Description     types.String
	Metric          types.String
	GrowthPerPeriod types.Float64
	UsePrevSpend    types.Bool
	MaxResults      types.Int64
	FolderId        types.String
	CreateTime      types.Int64
}

// ConfigModel is the nested config model.
type ConfigModel struct {
	Enabled   types.Bool
	SortOrder types.String
	Label     types.String
}

// ItemModel is the nested list item model.
type ItemModel struct {
	Name        types.String
	IncludeNull types.Bool
}

// ApiResponse is a mock API response with pointer fields.
type ApiResponse struct {
	Id              *string
	Name            string
	Description     *string
	Metric          *string
	GrowthPerPeriod *float64
	UsePrevSpend    *bool
	MaxResults      *int64
	FolderId        *string
	CreateTime      *int64
}

// ConfigResponse is a mock API response for nested config.
type ConfigResponse struct {
	Enabled   *bool
	SortOrder *string
	Label     *string
}

// ItemResponse is a mock API response for list items.
type ItemResponse struct {
	Name        string
	IncludeNull *bool
}
