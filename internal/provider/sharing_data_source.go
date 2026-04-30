package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_sharing"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*sharingDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*sharingDataSource)(nil)

func NewSharingDataSource() datasource.DataSource {
	return &sharingDataSource{}
}

type (
	sharingDataSource struct {
		client *models.ClientWithResponses
	}
	sharingDataSourceModel struct {
		datasource_sharing.SharingModel
		Timeouts timeouts.Value `tfsdk:"timeouts"`
	}
)

func (d *sharingDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sharing"
}

func (d *sharingDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_sharing.SharingDataSourceSchema(ctx)

	s.Attributes["timeouts"] = timeouts.Attributes(ctx)

	resp.Schema = s
}

func (d *sharingDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *sharingDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data sharingDataSourceModel

	// Read Terraform configuration data into the model
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

	// If either input is unknown (depends on a resource not yet created),
	// set all computed attributes to unknown so consumers don't treat null as
	// a real value during planning.
	if data.ResourceType.IsUnknown() || data.ResourceId.IsUnknown() {
		data.Id = types.StringUnknown()
		data.Name = types.StringUnknown()
		data.Description = types.StringUnknown()
		data.CreateTime = types.Int64Unknown()
		data.UpdateTime = types.Int64Unknown()
		data.Public = types.StringUnknown()
		data.Permissions = types.ListUnknown(datasource_sharing.PermissionsValue{}.Type(ctx))
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	resType := models.GetResourcePermissionParamsResourceType(data.ResourceType.ValueString())
	resID := data.ResourceId.ValueString()

	getResp, err := d.client.GetResourcePermissionWithResponse(ctx, resType, resID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Resource Sharing",
			fmt.Sprintf("Could not read resource permissions for %s/%s: %s", data.ResourceType.ValueString(), resID, err.Error()),
		)
		return
	}

	if getResp.StatusCode() != 200 || getResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Resource Sharing",
			fmt.Sprintf("Could not read resource permissions for %s/%s, status: %d, body: %s",
				data.ResourceType.ValueString(), resID, getResp.StatusCode(), string(getResp.Body)),
		)
		return
	}

	resp.Diagnostics.Append(mapSharingResponseToDataSourceModel(ctx, getResp.JSON200, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// mapSharingResponseToDataSourceModel maps the API response to the data source model.
func mapSharingResponseToDataSourceModel(ctx context.Context, apiResp *models.ResourcePermissionsResponse, data *sharingDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	data.Id = types.StringPointerValue(apiResp.Id)
	data.Name = types.StringPointerValue(apiResp.Name)
	data.Description = types.StringPointerValue(apiResp.Description)

	if apiResp.CreateTime != nil {
		data.CreateTime = types.Int64Value(*apiResp.CreateTime)
	} else {
		data.CreateTime = types.Int64Null()
	}

	if apiResp.UpdateTime != nil {
		data.UpdateTime = types.Int64Value(*apiResp.UpdateTime)
	} else {
		data.UpdateTime = types.Int64Null()
	}

	// Map permissions using the datasource-specific PermissionsValue type
	data.Permissions = mapDSPermissionsToList(ctx, apiResp.Permissions, &diags)

	// Map public — normalize empty string to null (allocations return "" for "no public access")
	if apiResp.Public != nil && string(*apiResp.Public) != "" {
		data.Public = types.StringValue(string(*apiResp.Public))
	} else {
		data.Public = types.StringNull()
	}

	return diags
}

// mapDSPermissionsToList converts the API permissions array to a Terraform list
// using the datasource_sharing package types.
func mapDSPermissionsToList(ctx context.Context, apiPerms *[]models.ResourcePermission, diags *diag.Diagnostics) types.List {
	elemType := datasource_sharing.PermissionsValue{}.Type(ctx)

	if apiPerms == nil || len(*apiPerms) == 0 {
		emptyList, d := types.ListValueFrom(ctx, elemType, []datasource_sharing.PermissionsValue{})
		diags.Append(d...)
		return emptyList
	}

	vals := make([]datasource_sharing.PermissionsValue, 0, len(*apiPerms))
	for _, p := range *apiPerms {
		var user, role types.String
		if p.User != nil {
			user = types.StringValue(*p.User)
		} else {
			user = types.StringNull()
		}
		if p.Role != nil {
			role = types.StringValue(string(*p.Role))
		} else {
			role = types.StringNull()
		}

		permVal, d := datasource_sharing.NewPermissionsValue(
			datasource_sharing.PermissionsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"user": user,
				"role": role,
			},
		)
		diags.Append(d...)
		if diags.HasError() {
			return types.ListNull(elemType)
		}
		vals = append(vals, permVal)
	}

	listVal, d := types.ListValueFrom(ctx, elemType, vals)
	diags.Append(d...)
	return listVal
}
