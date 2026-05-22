package datasource_test

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

func TestDataSourceSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required: true,
			},
			"name": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}
