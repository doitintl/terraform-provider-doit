package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_organizations"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*organizationsDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*organizationsDataSource)(nil)

func NewOrganizationsDataSource() datasource.DataSource {
	return &organizationsDataSource{}
}

type organizationsDataSource struct {
	client *models.ClientWithResponses
}

type organizationsDataSourceModel struct {
	datasource_organizations.OrganizationsModel
}

func (d *organizationsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *organizationsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organizations"
}

func (d *organizationsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_organizations.OrganizationsDataSourceSchema(ctx)
}

func (d *organizationsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data organizationsDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := d.client.ListOrganizationsWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to read organizations",
			"Unable to read organizations: "+err.Error(),
		)
		return
	}

	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unable to read organizations",
			fmt.Sprintf("Unable to read organizations, status: %d, body: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	if apiResp.JSON200.Organizations != nil && len(*apiResp.JSON200.Organizations) > 0 {
		organizationsVal := make([]datasource_organizations.OrganizationsValue, 0, len(*apiResp.JSON200.Organizations))
		for _, org := range *apiResp.JSON200.Organizations {
			orgAttrs := map[string]attr.Value{
				"id":   types.StringPointerValue(org.Id),
				"name": types.StringPointerValue(org.Name),
			}

			orgVal, diags := datasource_organizations.NewOrganizationsValue(datasource_organizations.OrganizationsValue{}.AttributeTypes(ctx), orgAttrs)
			resp.Diagnostics.Append(diags...)
			organizationsVal = append(organizationsVal, orgVal)
		}
		list, listDiags := types.ListValueFrom(ctx, datasource_organizations.OrganizationsValue{}.Type(ctx), organizationsVal)
		resp.Diagnostics.Append(listDiags...)
		data.Organizations = list
	} else {
		emptyList, diags := types.ListValueFrom(ctx, datasource_organizations.OrganizationsValue{}.Type(ctx), []datasource_organizations.OrganizationsValue{})
		resp.Diagnostics.Append(diags...)
		data.Organizations = emptyList
	}

	orgCount := int64(0)
	if apiResp.JSON200.Organizations != nil {
		orgCount = int64(len(*apiResp.JSON200.Organizations))
	}

	if apiResp.JSON200.RowCount != nil {
		data.RowCount = types.Int64Value(*apiResp.JSON200.RowCount)
	} else {
		data.RowCount = types.Int64Value(orgCount)
	}
	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
