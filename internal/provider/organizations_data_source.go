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
	var data datasource_organizations.OrganizationsModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := d.client.ListOrganizationsWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Organizations",
			"Could not read Organizations: "+err.Error(),
		)
		return
	}

	if apiResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Reading Organizations",
			fmt.Sprintf("Could not read Organizations, status: %d, body: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	organizationsVal := []datasource_organizations.OrganizationsValue{}

	if apiResp.JSON200 != nil && apiResp.JSON200.Organizations != nil {
		for _, org := range *apiResp.JSON200.Organizations {
			orgAttrs := map[string]attr.Value{
				"id":   types.StringPointerValue(org.Id),
				"name": types.StringPointerValue(org.Name),
			}

			orgVal, diags := datasource_organizations.NewOrganizationsValue(datasource_organizations.OrganizationsValue{}.AttributeTypes(ctx), orgAttrs)
			resp.Diagnostics.Append(diags...)
			organizationsVal = append(organizationsVal, orgVal)
		}
		data.RowCount = types.Int64PointerValue(apiResp.JSON200.RowCount)
	} else {
		data.RowCount = types.Int64Value(0)
	}

	list, listDiags := types.ListValueFrom(ctx, datasource_organizations.OrganizationsValue{}.Type(ctx), organizationsVal)
	resp.Diagnostics.Append(listDiags...)
	data.Organizations = list
	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
