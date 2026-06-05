package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestStatussheetComponentIDsValidator(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Schema matching the statussheet data source ID attributes.
	testSchema := schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Required: true},
			"node_ids": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
			},
			"element_ids": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
			},
			"group_ids": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
			},
			"link_ids": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
			},
			"attachment_ids": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
			},
			"combiner_ids": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
			},
			"note_ids": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
			},
		},
	}

	// tftypes schema for building tftypes.Value.
	tfSchema := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"id":             tftypes.String,
			"node_ids":       tftypes.List{ElementType: tftypes.String},
			"element_ids":    tftypes.List{ElementType: tftypes.String},
			"group_ids":      tftypes.List{ElementType: tftypes.String},
			"link_ids":       tftypes.List{ElementType: tftypes.String},
			"attachment_ids": tftypes.List{ElementType: tftypes.String},
			"combiner_ids":   tftypes.List{ElementType: tftypes.String},
			"note_ids":       tftypes.List{ElementType: tftypes.String},
		},
	}

	tests := []struct {
		name      string
		values    map[string]tftypes.Value
		expectErr bool
	}{
		{
			name: "all null - should error",
			values: map[string]tftypes.Value{
				"id":             tftypes.NewValue(tftypes.String, "layer-123"),
				"node_ids":       tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
				"element_ids":    tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
				"group_ids":      tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
				"link_ids":       tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
				"attachment_ids": tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
				"combiner_ids":   tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
				"note_ids":       tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
			},
			expectErr: true,
		},
		{
			name: "one empty list - should error",
			values: map[string]tftypes.Value{
				"id":             tftypes.NewValue(tftypes.String, "layer-123"),
				"node_ids":       tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{}),
				"element_ids":    tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
				"group_ids":      tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
				"link_ids":       tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
				"attachment_ids": tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
				"combiner_ids":   tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
				"note_ids":       tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
			},
			expectErr: true,
		},
		{
			name: "all empty lists - should error",
			values: map[string]tftypes.Value{
				"id":             tftypes.NewValue(tftypes.String, "layer-123"),
				"node_ids":       tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{}),
				"element_ids":    tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{}),
				"group_ids":      tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{}),
				"link_ids":       tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{}),
				"attachment_ids": tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{}),
				"combiner_ids":   tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{}),
				"note_ids":       tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{}),
			},
			expectErr: true,
		},
		{
			name: "one non-empty list - should pass",
			values: map[string]tftypes.Value{
				"id": tftypes.NewValue(tftypes.String, "layer-123"),
				"node_ids": tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{
					tftypes.NewValue(tftypes.String, "abc123"),
				}),
				"element_ids":    tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
				"group_ids":      tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
				"link_ids":       tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
				"attachment_ids": tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
				"combiner_ids":   tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
				"note_ids":       tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
			},
			expectErr: false,
		},
		{
			name: "multiple non-empty lists - should pass",
			values: map[string]tftypes.Value{
				"id": tftypes.NewValue(tftypes.String, "layer-123"),
				"node_ids": tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{
					tftypes.NewValue(tftypes.String, "abc123"),
				}),
				"element_ids": tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
				"group_ids": tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{
					tftypes.NewValue(tftypes.String, "def456"),
				}),
				"link_ids":       tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
				"attachment_ids": tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
				"combiner_ids":   tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
				"note_ids":       tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
			},
			expectErr: false,
		},
		{
			name: "unknown list - should skip validation (pass)",
			values: map[string]tftypes.Value{
				"id":             tftypes.NewValue(tftypes.String, "layer-123"),
				"node_ids":       tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, tftypes.UnknownValue),
				"element_ids":    tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
				"group_ids":      tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
				"link_ids":       tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
				"attachment_ids": tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
				"combiner_ids":   tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
				"note_ids":       tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
			},
			expectErr: false,
		},
		{
			name: "non-empty note_ids only - should pass",
			values: map[string]tftypes.Value{
				"id":             tftypes.NewValue(tftypes.String, "layer-123"),
				"node_ids":       tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
				"element_ids":    tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
				"group_ids":      tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
				"link_ids":       tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
				"attachment_ids": tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
				"combiner_ids":   tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
				"note_ids": tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{
					tftypes.NewValue(tftypes.String, "note-1"),
				}),
			},
			expectErr: false,
		},
	}

	v := statussheetComponentIDsValidator{}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			raw := tftypes.NewValue(tfSchema, tc.values)

			config := tfsdk.Config{
				Schema: testSchema,
				Raw:    raw,
			}

			req := datasource.ValidateConfigRequest{Config: config}
			resp := &datasource.ValidateConfigResponse{}

			v.ValidateDataSource(ctx, req, resp)

			if tc.expectErr && !resp.Diagnostics.HasError() {
				t.Error("expected validation error but got none")
			}
			if !tc.expectErr && resp.Diagnostics.HasError() {
				t.Errorf("expected no error but got: %s", resp.Diagnostics.Errors())
			}
		})
	}
}
