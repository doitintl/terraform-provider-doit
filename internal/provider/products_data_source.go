package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_products"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*productsDataSource)(nil)

func NewProductsDataSource() datasource.DataSource {
	return &productsDataSource{}
}

type productsDataSource struct {
	client *models.ClientWithResponses
}

type productsDataSourceModel struct {
	datasource_products.ProductsModel
}

func (d *productsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_products"
}

func (d *productsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_products.ProductsDataSourceSchema(ctx)
}

func (d *productsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *productsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data productsDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If platform is unknown (depends on an unresolved resource), we cannot
	// determine the correct product list. Return unknown list to prevent
	// Terraform from using potentially incorrect data during planning.
	if data.Platform.IsUnknown() {
		data.Products = types.ListUnknown(datasource_products.ProductsValue{}.Type(ctx))
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	// Build query parameters from optional inputs
	params := &models.ListProductsParams{}
	if !data.Platform.IsNull() {
		platformVal := data.Platform.ValueString()
		params.Platform = &platformVal
	}

	apiResp, err := d.client.ListProductsWithResponse(ctx, params)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Products",
			fmt.Sprintf("Unable to read products: %v", err),
		)
		return
	}

	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Products",
			fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	if apiResp.JSON200.Products != nil && len(*apiResp.JSON200.Products) > 0 {
		prodVals := make([]datasource_products.ProductsValue, 0, len(*apiResp.JSON200.Products))
		for _, p := range *apiResp.JSON200.Products {
			prodVal, diags := datasource_products.NewProductsValue(
				datasource_products.ProductsValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"display_name": types.StringPointerValue(p.DisplayName),
					"id":           types.StringPointerValue(p.Id),
					"platform":     types.StringPointerValue(p.Platform),
				},
			)
			resp.Diagnostics.Append(diags...)
			prodVals = append(prodVals, prodVal)
		}

		prodList, diags := types.ListValueFrom(ctx, datasource_products.ProductsValue{}.Type(ctx), prodVals)
		resp.Diagnostics.Append(diags...)
		data.Products = prodList
	} else {
		emptyList, diags := types.ListValueFrom(ctx, datasource_products.ProductsValue{}.Type(ctx), []datasource_products.ProductsValue{})
		resp.Diagnostics.Append(diags...)
		data.Products = emptyList
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
