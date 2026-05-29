package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_custom_themes"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*customThemesDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*customThemesDataSource)(nil)

func NewCustomThemesDataSource() datasource.DataSource {
	return &customThemesDataSource{}
}

type (
	customThemesDataSource struct {
		client *models.ClientWithResponses
	}
	customThemesDataSourceModel struct {
		datasource_custom_themes.CustomThemesModel
		Timeouts timeouts.Value `tfsdk:"timeouts"`
	}
)

func (d *customThemesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_custom_themes"
}

func (d *customThemesDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_custom_themes.CustomThemesDataSourceSchema(ctx)

	s.Description = "List all custom color themes."
	s.MarkdownDescription = s.Description

	s.Attributes["timeouts"] = timeouts.Attributes(ctx)

	resp.Schema = s
}

func (d *customThemesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*models.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *models.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *customThemesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data customThemesDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	readTimeout, diags := data.Timeouts.Read(ctx, 2*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	// List API has no inputs to check for unknown — no pagination params.

	apiResp, err := d.client.ListCustomThemesWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Custom Themes",
			fmt.Sprintf("Unable to read custom themes: %v", err),
		)
		return
	}

	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Custom Themes",
			fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	result := apiResp.JSON200

	// Deterministic rowCount fallback
	var allThemes []models.CustomTheme
	if result.Themes != nil {
		allThemes = *result.Themes
	}

	if result.RowCount != nil {
		data.RowCount = types.Int64Value(int64(*result.RowCount))
	} else {
		data.RowCount = types.Int64Value(int64(len(allThemes)))
	}

	// Map themes list
	if len(allThemes) > 0 {
		themeVals := make([]datasource_custom_themes.ThemesValue, 0, len(allThemes))
		for _, theme := range allThemes {
			colorsVal, colorsDiags := dsThemesColorsToModel(ctx, theme.Colors)
			resp.Diagnostics.Append(colorsDiags...)
			if resp.Diagnostics.HasError() {
				return
			}

			var createTime, updateTime types.String
			if theme.CreateTime != nil {
				createTime = types.StringValue(theme.CreateTime.Format(time.RFC3339))
			} else {
				createTime = types.StringNull()
			}
			if theme.UpdateTime != nil {
				updateTime = types.StringValue(theme.UpdateTime.Format(time.RFC3339))
			} else {
				updateTime = types.StringNull()
			}

			themeVal, themeValDiags := datasource_custom_themes.NewThemesValue(
				datasource_custom_themes.ThemesValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"id":            types.StringValue(theme.Id),
					"name":          types.StringValue(theme.Name),
					"primary_color": types.StringValue(theme.PrimaryColor),
					"create_time":   createTime,
					"update_time":   updateTime,
					"colors":        colorsVal,
				},
			)
			resp.Diagnostics.Append(themeValDiags...)
			themeVals = append(themeVals, themeVal)
		}

		themeList, listDiags := types.ListValueFrom(ctx, datasource_custom_themes.ThemesValue{}.Type(ctx), themeVals)
		resp.Diagnostics.Append(listDiags...)
		data.Themes = themeList
	} else {
		emptyList, listDiags := types.ListValueFrom(ctx, datasource_custom_themes.ThemesValue{}.Type(ctx), []datasource_custom_themes.ThemesValue{})
		resp.Diagnostics.Append(listDiags...)
		data.Themes = emptyList
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// dsThemesColorsToModel converts an API ThemeColors response into the list data source's ColorsValue type.
func dsThemesColorsToModel(ctx context.Context, tc models.ThemeColors) (datasource_custom_themes.ColorsValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	lightVals := make([]attr.Value, len(tc.Light))
	for i, c := range tc.Light {
		lightVals[i] = types.StringValue(c)
	}

	darkVals := make([]attr.Value, len(tc.Dark))
	for i, c := range tc.Dark {
		darkVals[i] = types.StringValue(c)
	}

	lightList, lightDiags := types.ListValue(types.StringType, lightVals)
	diags.Append(lightDiags...)
	darkList, darkDiags := types.ListValue(types.StringType, darkVals)
	diags.Append(darkDiags...)

	if diags.HasError() {
		return datasource_custom_themes.NewColorsValueUnknown(), diags
	}

	colorsVal, colorsDiags := datasource_custom_themes.NewColorsValue(
		datasource_custom_themes.ColorsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"light": lightList,
			"dark":  darkList,
		},
	)
	diags.Append(colorsDiags...)

	return colorsVal, diags
}
