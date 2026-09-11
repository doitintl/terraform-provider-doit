package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_ps4c_gcp_settings"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*ps4cGcpSettingsDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*ps4cGcpSettingsDataSource)(nil)

func NewPs4cGcpSettingsDataSource() datasource.DataSource {
	return &ps4cGcpSettingsDataSource{}
}

type ps4cGcpSettingsDataSource struct {
	client *models.ClientWithResponses
}

type ps4cGcpSettingsDataSourceModel struct {
	datasource_ps4c_gcp_settings.Ps4cGcpSettingsModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *ps4cGcpSettingsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ps4c_gcp_settings"
}

func (d *ps4cGcpSettingsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*models.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *models.ClientWithResponses, got: %T", req.ProviderData),
		)
		return
	}
	d.client = client
}

func (d *ps4cGcpSettingsDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_ps4c_gcp_settings.Ps4cGcpSettingsDataSourceSchema(ctx)

	s.MarkdownDescription = "List PS4C commitment settings across all GCP billing accounts."
	s.Description = "List PS4C commitment settings across all GCP billing accounts."

	s.Attributes["timeouts"] = timeouts.Attributes(ctx)

	resp.Schema = s
}

func (d *ps4cGcpSettingsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ps4cGcpSettingsDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	readTimeout, diags := data.Timeouts.Read(ctx, DefaultReadTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	if data.MaxResults.IsUnknown() || data.PageToken.IsUnknown() {
		data.Items = types.ListUnknown(datasource_ps4c_gcp_settings.ItemsValue{}.Type(ctx))
		data.RowCount = types.Int64Unknown()
		data.PageToken = types.StringUnknown()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	params := &models.ListGcpBillingAccountsSettingsParams{}

	userControlsPagination := !data.MaxResults.IsNull()

	var allItems []models.GcpBillingAccountSettingsItem

	if userControlsPagination {
		params.MaxResults = new(int(data.MaxResults.ValueInt64()))
		if !data.PageToken.IsNull() {
			params.PageToken = new(data.PageToken.ValueString())
		}

		apiResp, err := d.client.ListGcpBillingAccountsSettingsWithResponse(ctx, params)
		if err != nil {
			resp.Diagnostics.AddError("Error Reading PS4C GCP Settings", fmt.Sprintf("Unable to read PS4C GCP settings: %v", err))
			return
		}
		if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
			resp.Diagnostics.AddError("Error Reading PS4C GCP Settings", fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)))
			return
		}

		result := apiResp.JSON200
		allItems = result.Items

		data.PageToken = types.StringPointerValue(nullableToPointer(result.PageToken))
		if rowCount := result.RowCount; rowCount != nil {
			data.RowCount = types.Int64Value(*rowCount)
		} else {
			data.RowCount = types.Int64Value(int64(len(allItems)))
		}
	} else {
		if !data.PageToken.IsNull() {
			params.PageToken = new(data.PageToken.ValueString())
		}
		for {
			apiResp, err := d.client.ListGcpBillingAccountsSettingsWithResponse(ctx, params)
			if err != nil {
				resp.Diagnostics.AddError("Error Reading PS4C GCP Settings", fmt.Sprintf("Unable to read PS4C GCP settings: %v", err))
				return
			}
			if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
				resp.Diagnostics.AddError("Error Reading PS4C GCP Settings", fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)))
				return
			}

			result := apiResp.JSON200
			allItems = append(allItems, result.Items...)

			pageToken := nullableToPointer(result.PageToken)
			if pageToken == nil || *pageToken == "" {
				break
			}
			params.PageToken = pageToken
		}

		data.RowCount = types.Int64Value(int64(len(allItems)))
		data.PageToken = types.StringNull()
	}

	itemsList, itemsDiags := mapGcpBillingAccountsSettingsToItemsList(ctx, allItems)
	resp.Diagnostics.Append(itemsDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Items = itemsList

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
