package volatiletest

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func PlannedResourceSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"description": schema.StringAttribute{Optional: true, Computed: true},
			"etag":        schema.StringAttribute{Computed: true},
			"update_time": schema.StringAttribute{Computed: true},
		},
	}
}

func StableResourceSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"description": schema.StringAttribute{Optional: true, Computed: true},
			"id":          schema.StringAttribute{Computed: true},
			"create_time": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"update_time": schema.StringAttribute{Computed: true},
		},
	}
}

func UnplannedResourceSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"description": schema.StringAttribute{Optional: true, Computed: true},
			"etag":        schema.StringAttribute{Computed: true},
		},
	}
}

func NoPathsResourceSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"description": schema.StringAttribute{Optional: true, Computed: true},
			"etag":        schema.StringAttribute{Computed: true},
		},
	}
}

func PartialResourceSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"description": schema.StringAttribute{Optional: true, Computed: true},
			"etag":        schema.StringAttribute{Computed: true},
			"records":     schema.Int64Attribute{Computed: true},
			"update_time": schema.StringAttribute{Computed: true},
		},
	}
}

func NoClearingResourceSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"description": schema.StringAttribute{Optional: true, Computed: true},
			"etag":        schema.StringAttribute{Computed: true},
			"update_time": schema.StringAttribute{Computed: true},
		},
	}
}
