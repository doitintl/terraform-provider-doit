package volatiletest

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func PlannedResourceSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"description": schema.StringAttribute{Optional: true, Computed: true},
			"etag":        schema.StringAttribute{Computed: true},
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

func NoEtagResourceSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"description": schema.StringAttribute{Optional: true, Computed: true},
			"update_time": schema.StringAttribute{Computed: true},
		},
	}
}

func NoClearingResourceSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"description": schema.StringAttribute{Optional: true, Computed: true},
			"etag":        schema.StringAttribute{Computed: true},
		},
	}
}
