package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_rbac_countries"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*rbacCountriesDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*rbacCountriesDataSource)(nil)

func NewRbacCountriesDataSource() datasource.DataSource {
	return &rbacCountriesDataSource{}
}

type rbacCountriesDataSource struct {
	client *models.ClientWithResponses
}

type rbacCountriesDataSourceModel struct {
	datasource_rbac_countries.RbacCountriesModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *rbacCountriesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_rbac_countries"
}

func (d *rbacCountriesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_rbac_countries.RbacCountriesDataSourceSchema(ctx)

	s.Attributes["timeouts"] = timeouts.Attributes(ctx)

	resp.Schema = s
}

func (d *rbacCountriesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*models.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *models.ClientWithResponses, got: %T.", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *rbacCountriesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data rbacCountriesDataSourceModel

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

	apiResp, err := d.client.ListGeographicAccessCountriesWithResponse(ctx, nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Rbac Countries",
			fmt.Sprintf("Unable to read rbac countries: %v", err),
		)
		return
	}

	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Rbac Countries",
			fmt.Sprintf("status: %d, body: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	if len(apiResp.JSON200.Countries) > 0 {
		countryVals := make([]datasource_rbac_countries.CountriesValue, 0, len(apiResp.JSON200.Countries))
		for _, c := range apiResp.JSON200.Countries {
			countryVal, diags := datasource_rbac_countries.NewCountriesValue(
				datasource_rbac_countries.CountriesValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"country_code": types.StringValue(c.CountryCode),
					"name":         types.StringValue(c.Name),
				},
			)
			resp.Diagnostics.Append(diags...)
			countryVals = append(countryVals, countryVal)
		}

		countryList, diags := types.ListValueFrom(ctx, datasource_rbac_countries.CountriesValue{}.Type(ctx), countryVals)
		resp.Diagnostics.Append(diags...)
		data.Countries = countryList
	} else {
		emptyList, diags := types.ListValueFrom(ctx, datasource_rbac_countries.CountriesValue{}.Type(ctx), []datasource_rbac_countries.CountriesValue{})
		resp.Diagnostics.Append(diags...)
		data.Countries = emptyList
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
