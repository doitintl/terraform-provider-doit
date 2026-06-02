package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_custom_theme"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*customThemeDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*customThemeDataSource)(nil)

func NewCustomThemeDataSource() datasource.DataSource {
	return &customThemeDataSource{}
}

type (
	customThemeDataSource struct {
		client *models.ClientWithResponses
	}
	customThemeDataSourceModel struct {
		datasource_custom_theme.CustomThemeModel
		Timeouts timeouts.Value `tfsdk:"timeouts"`
	}
)

func (d *customThemeDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_custom_theme"
}

func (d *customThemeDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_custom_theme.CustomThemeDataSourceSchema(ctx)

	s.Description = "Retrieve a custom color theme by its ID."
	s.MarkdownDescription = s.Description

	s.Attributes["timeouts"] = timeouts.Attributes(ctx)

	resp.Schema = s
}

func (d *customThemeDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *customThemeDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data customThemeDataSourceModel

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

	// If ID is unknown (depends on a resource not yet created), set all computed
	// attributes to unknown so consumers don't treat null as a real value during planning.
	if data.Id.IsUnknown() {
		data.Name = types.StringUnknown()
		data.PrimaryColor = types.StringUnknown()
		data.CreateTime = types.StringUnknown()
		data.UpdateTime = types.StringUnknown()
		data.Colors = datasource_custom_theme.NewColorsValueUnknown()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	// Call API to get custom theme
	themeResp, err := d.client.GetCustomThemeWithResponse(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Custom Theme",
			"Could not read custom theme ID "+data.Id.ValueString()+": "+err.Error(),
		)
		return
	}

	if themeResp.StatusCode() != 200 || themeResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Custom Theme",
			fmt.Sprintf("Could not read custom theme ID %s, status: %d, body: %s", data.Id.ValueString(), themeResp.StatusCode(), string(themeResp.Body)),
		)
		return
	}

	theme := themeResp.JSON200

	// Map API response to model
	data.Id = types.StringValue(theme.Id)
	data.Name = types.StringValue(theme.Name)
	data.PrimaryColor = types.StringValue(theme.PrimaryColor)

	if theme.CreateTime != nil {
		data.CreateTime = types.StringValue(theme.CreateTime.Format(time.RFC3339))
	} else {
		data.CreateTime = types.StringNull()
	}

	if theme.UpdateTime != nil {
		data.UpdateTime = types.StringValue(theme.UpdateTime.Format(time.RFC3339))
	} else {
		data.UpdateTime = types.StringNull()
	}

	// Map colors nested object
	colorsVal, colorsDiags := dsColorsToModel(ctx, theme.Colors)
	resp.Diagnostics.Append(colorsDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Colors = colorsVal

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// dsColorsToModel converts an API ThemeColors response into the data source's ColorsValue type.
func dsColorsToModel(ctx context.Context, tc models.ThemeColors) (datasource_custom_theme.ColorsValue, diag.Diagnostics) {
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
		return datasource_custom_theme.NewColorsValueUnknown(), diags
	}

	colorsVal, colorsDiags := datasource_custom_theme.NewColorsValue(
		datasource_custom_theme.ColorsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"light": lightList,
			"dark":  darkList,
		},
	)
	diags.Append(colorsDiags...)

	return colorsVal, diags
}
