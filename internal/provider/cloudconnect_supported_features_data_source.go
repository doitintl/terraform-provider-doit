package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_cloudconnect_supported_features"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*cloudconnectSupportedFeaturesDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*cloudconnectSupportedFeaturesDataSource)(nil)
)

func NewCloudconnectSupportedFeaturesDataSource() datasource.DataSource {
	return &cloudconnectSupportedFeaturesDataSource{}
}

type (
	cloudconnectSupportedFeaturesDataSource struct {
		client *models.ClientWithResponses
	}
	cloudconnectSupportedFeaturesDataSourceModel struct {
		datasource_cloudconnect_supported_features.CloudconnectSupportedFeaturesModel
		Timeouts timeouts.Value `tfsdk:"timeouts"`
	}
)

func (d *cloudconnectSupportedFeaturesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloudconnect_supported_features"
}

func (d *cloudconnectSupportedFeaturesDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_cloudconnect_supported_features.CloudconnectSupportedFeaturesDataSourceSchema(ctx)

	s.MarkdownDescription = "Retrieve supported features and their permission status for a connected AWS account. The account must belong to the authenticated customer. Azure tenant IDs are described in the public API schema but are not supported by the current service."
	s.Description = "Retrieve supported features and their permission status for a connected AWS account. The account must belong to the authenticated customer. Azure tenant IDs are described in the public API schema but are not supported by the current service."

	s.Attributes["timeouts"] = timeouts.Attributes(ctx)

	resp.Schema = s
}

func (d *cloudconnectSupportedFeaturesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *cloudconnectSupportedFeaturesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data cloudconnectSupportedFeaturesDataSourceModel

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

	if data.AccountId.IsUnknown() {
		data.SupportedFeatures = types.ListUnknown(datasource_cloudconnect_supported_features.SupportedFeaturesValue{}.Type(ctx))
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	accountResp, err := d.client.GetCloudConnectSupportedFeaturesWithResponse(ctx, data.AccountId.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading CloudConnect Supported Features",
			"Could not read CloudConnect supported features for account ID "+data.AccountId.ValueString()+": "+err.Error(),
		)
		return
	}

	if accountResp.StatusCode() != 200 || accountResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading CloudConnect Supported Features",
			fmt.Sprintf("Could not read CloudConnect supported features for account ID %s, status: %d, body: %s",
				data.AccountId.ValueString(), accountResp.StatusCode(), string(accountResp.Body)),
		)
		return
	}

	resp.Diagnostics.Append(mapCloudconnectSupportedFeaturesToModel(ctx, accountResp.JSON200, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
