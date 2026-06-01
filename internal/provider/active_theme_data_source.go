package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_active_theme"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*activeThemeDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*activeThemeDataSource)(nil)

func NewActiveThemeDataSource() datasource.DataSource {
	return &activeThemeDataSource{}
}

type (
	activeThemeDataSource struct {
		client *models.ClientWithResponses
	}
	activeThemeDataSourceModel struct {
		datasource_active_theme.ActiveThemeModel
		Timeouts timeouts.Value `tfsdk:"timeouts"`
	}
)

func (d *activeThemeDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_active_theme"
}

func (d *activeThemeDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_active_theme.ActiveThemeDataSourceSchema(ctx)

	s.Description = "Retrieve the active color theme for the current account."
	s.MarkdownDescription = s.Description

	s.Attributes["timeouts"] = timeouts.Attributes(ctx)

	resp.Schema = s
}

func (d *activeThemeDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *activeThemeDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data activeThemeDataSourceModel

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

	// No inputs to check for unknown — this is a singleton endpoint with no parameters.

	themeResp, err := d.client.GetActiveThemeWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Active Theme",
			fmt.Sprintf("Unable to read active theme: %v", err),
		)
		return
	}

	if themeResp.StatusCode() != 200 || themeResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Active Theme",
			fmt.Sprintf("API returned status %d: %s", themeResp.StatusCode(), string(themeResp.Body)),
		)
		return
	}

	// Map API response to model
	if themeResp.JSON200.ThemeId != "" {
		data.ThemeId = types.StringValue(themeResp.JSON200.ThemeId)
	} else {
		data.ThemeId = types.StringValue("default")
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
