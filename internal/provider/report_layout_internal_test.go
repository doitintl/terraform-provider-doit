package provider

import (
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rsschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestReportLayoutValidation(t *testing.T) {
	ctx := t.Context()
	var resourceSchema resource.SchemaResponse
	NewReportResource().Schema(ctx, resource.SchemaRequest{}, &resourceSchema)
	if resourceSchema.Diagnostics.HasError() {
		t.Fatal(resourceSchema.Diagnostics)
	}
	resourceConfig := resourceSchema.Schema.Attributes["config"].(rsschema.SingleNestedAttribute)
	resourceLayout := resourceConfig.Attributes["layout"].(rsschema.StringAttribute)

	var querySchema datasource.SchemaResponse
	NewReportQueryDataSource().Schema(ctx, datasource.SchemaRequest{}, &querySchema)
	if querySchema.Diagnostics.HasError() {
		t.Fatal(querySchema.Diagnostics)
	}
	queryConfig := querySchema.Schema.Attributes["config"].(dsschema.SingleNestedAttribute)
	queryLayout := queryConfig.Attributes["layout"].(dsschema.StringAttribute)

	for surface, validators := range map[string][]validator.String{
		"resource": resourceLayout.Validators,
		"query":    queryLayout.Validators,
	} {
		t.Run(surface, func(t *testing.T) {
			for _, tc := range []struct {
				layout    string
				wantError bool
			}{
				{layout: "sankey_chart"},
				{layout: "column_and_line_chart"},
				{layout: "trend_board"},
				{layout: "table"},
				{layout: "column_chart"},
				{layout: "csv_export", wantError: true},
				{layout: "sheets_export", wantError: true},
				{layout: "unknown_chart", wantError: true},
			} {
				t.Run(tc.layout, func(t *testing.T) {
					var resp validator.StringResponse
					for _, v := range validators {
						v.ValidateString(t.Context(), validator.StringRequest{
							Path:        path.Root("config").AtName("layout"),
							ConfigValue: types.StringValue(tc.layout),
						}, &resp)
					}
					if resp.Diagnostics.HasError() != tc.wantError {
						t.Fatalf("validation errors = %v, want error = %v", resp.Diagnostics, tc.wantError)
					}
				})
			}
		})
	}
}

func TestReportLayoutRoundTrip(t *testing.T) {
	for _, layout := range []string{"sankey_chart", "column_and_line_chart", "trend_board"} {
		t.Run(layout, func(t *testing.T) {
			ctx := t.Context()
			apiReport := &models.ExternalReport{Config: &models.ExternalConfig{Layout: new(models.ExternalRenderer(layout))}}
			var state reportResourceModel
			if diags := mapReportToModel(ctx, apiReport, &state); diags.HasError() {
				t.Fatal(diags)
			}
			if state.Config.Layout.ValueString() != layout {
				t.Fatalf("read layout = %q, want %q", state.Config.Layout.ValueString(), layout)
			}
			request, diags := toExternalConfig(ctx, state.Config)
			if diags.HasError() {
				t.Fatal(diags)
			}
			if request.Layout == nil || string(*request.Layout) != layout {
				t.Fatalf("request layout = %v, want %q", request.Layout, layout)
			}
			var data reportDataSourceModel
			if diags := (&reportDataSource{}).populateState(ctx, &data, apiReport); diags.HasError() {
				t.Fatal(diags)
			}
			if data.Config.Layout.ValueString() != layout {
				t.Fatalf("data source layout = %q, want %q", data.Config.Layout.ValueString(), layout)
			}
		})
	}
}
