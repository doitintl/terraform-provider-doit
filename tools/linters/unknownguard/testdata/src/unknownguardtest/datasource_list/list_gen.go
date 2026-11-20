package datasource_list

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

func ListDataSourceSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"filter": schema.StringAttribute{
				Optional: true,
				Computed: true,
			},
			"max_results": schema.Int64Attribute{
				Optional: true,
				Computed: true,
			},
			"page_token": schema.StringAttribute{
				Optional: true,
				Computed: true,
			},
			"row_count": schema.Int64Attribute{
				Computed: true,
			},
		},
	}
}
