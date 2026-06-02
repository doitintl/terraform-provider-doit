// Package guardtest is a test fixture for the requestguard analyzer.
package guardtest

import (
	"context"

	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure imports are used.
var _ context.Context

// GuardTestResourceSchema returns a test schema with various field types.
func GuardTestResourceSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			// Computed-only: IsUnknown() is legitimate.
			"id": schema.StringAttribute{
				Computed: true,
			},
			// Required: IsUnknown() is dead code.
			"name": schema.StringAttribute{
				Required: true,
			},
			// Optional (no Computed): IsUnknown() is dead code.
			"label": schema.StringAttribute{
				Optional: true,
			},
			// Optional+Computed WITH default: IsUnknown() is dead code.
			"metric": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("cost"),
			},
			// Optional+Computed WITHOUT default: IsUnknown() is needed.
			"folder_id": schema.StringAttribute{
				Optional: true,
				Computed: true,
			},
			// Nested attribute with mixed fields.
			"config": schema.SingleNestedAttribute{
				Optional: true,
				Computed: true,
				Attributes: map[string]schema.Attribute{
					// Required nested field.
					"type": schema.StringAttribute{
						Required: true,
					},
					// Optional+Computed WITH default nested field.
					"case_insensitive": schema.BoolAttribute{
						Optional: true,
						Computed: true,
						Default:  booldefault.StaticBool(false),
					},
					// Optional+Computed WITHOUT default nested field.
					"currency": schema.StringAttribute{
						Optional: true,
						Computed: true,
					},
					// 2-level nested attribute.
					"filter": schema.SingleNestedAttribute{
						Optional: true,
						Computed: true,
						Attributes: map[string]schema.Attribute{
							// Required at 2nd nesting level.
							"operator": schema.StringAttribute{
								Required: true,
							},
							// Optional+Computed without default at 2nd nesting level.
							"mode": schema.StringAttribute{
								Optional: true,
								Computed: true,
							},
						},
					},
					// 2-level nested attribute with a defaulted field.
					"display_settings": schema.SingleNestedAttribute{
						Optional: true,
						Computed: true,
						Attributes: map[string]schema.Attribute{
							// Optional+Computed WITH default at 2nd nesting level.
							"theme_id": schema.StringAttribute{
								Optional: true,
								Computed: true,
								Default:  stringdefault.StaticString("default"),
							},
						},
					},
				},
			},
		},
	}
}

// GuardTestModel is the Terraform model.
type GuardTestModel struct {
	Id       types.String
	Name     types.String
	Label    types.String
	Metric   types.String
	FolderId types.String
	Config   ConfigValue
}

// ConfigValue is the nested config model.
type ConfigValue struct {
	Type             types.String
	CaseInsensitive  types.Bool
	Currency         types.String
	Filter           FilterValue
	DisplaySettings  DisplaySettingsValue
}

// FilterValue is the 2nd-level nested model.
type FilterValue struct {
	Operator types.String
	Mode     types.String
}

// DisplaySettingsValue is the 2nd-level nested model for display settings.
type DisplaySettingsValue struct {
	ThemeId types.String
}



// guardTestResource is a mock resource.
type guardTestResource struct{}

// Schema wires the resource type to the schema.
func (r *guardTestResource) Schema(ctx context.Context) schema.Schema {
	return GuardTestResourceSchema(ctx)
}

// ApiRequest is a mock API request type.
type ApiRequest struct {
	Name     *string
	Label    *string
	Metric   *string
	FolderId *string
	Config   *ConfigRequest
}

// ConfigRequest is a mock nested API request type.
type ConfigRequest struct {
	Type            *string
	CaseInsensitive *bool
	Currency        *string
	Filter          *FilterRequest
}

// FilterRequest is a mock 2nd-level nested request type.
type FilterRequest struct {
	Operator *string
	Mode     *string
}

// DisplaySettingsRequest is a mock 2nd-level nested request type.
type DisplaySettingsRequest struct {
	ThemeId *string
}

// --- Data source fixtures ---

// searchDataSourceModel is a data source model (not a resource model).
// It tests the DataSourceModel → DataSourceSchema derivation path.
type searchDataSourceModel struct {
	Query types.String
	SsId  types.String
}

// searchDataSource is a non-model service struct (data source type).
type searchDataSource struct{}

// SearchDataSourceSchema defines the data source schema (simulated _gen.go).
func SearchDataSourceSchema(ctx context.Context) dsschema.Schema {
	return dsschema.Schema{
		Attributes: map[string]dsschema.Attribute{
			"query": dsschema.StringAttribute{
				Required: true,
			},
		},
	}
}

// Schema wires the data source type to the schema.
func (d *searchDataSource) Schema(ctx context.Context) dsschema.Schema {
	s := SearchDataSourceSchema(ctx)
	// Add a manual attribute (simulates runtime override like cloud_diagram_search).
	s.Attributes["ss_id"] = dsschema.StringAttribute{
		Optional: true,
	}
	return s
}

// SearchRequest is a mock API request for the data source.
type SearchRequest struct {
	Query *string
	SsId  *string
}
