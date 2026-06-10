package provider

import (
	"context"
	"fmt"
	"time"

	ds "github.com/doitintl/terraform-provider-doit/internal/provider/datasource_cloud_diagrams_activity_groups"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Compile-time interface checks.
var _ datasource.DataSource = (*cloudDiagramsActivityGroupsDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*cloudDiagramsActivityGroupsDataSource)(nil)

// NewCloudDiagramsActivityGroupsDataSource creates a new instance of the data source.
func NewCloudDiagramsActivityGroupsDataSource() datasource.DataSource {
	return &cloudDiagramsActivityGroupsDataSource{}
}

// cloudDiagramsActivityGroupsDataSource implements datasource.DataSource.
type cloudDiagramsActivityGroupsDataSource struct {
	client *models.ClientWithResponses
}

// cloudDiagramsActivityGroupsDataSourceModel is the Terraform state model.
type cloudDiagramsActivityGroupsDataSourceModel struct {
	ds.CloudDiagramsActivityGroupsModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *cloudDiagramsActivityGroupsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_diagrams_activity_groups"
}

func (d *cloudDiagramsActivityGroupsDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := ds.CloudDiagramsActivityGroupsDataSourceSchema(ctx)
	s.Attributes["timeouts"] = timeouts.Attributes(ctx)
	s.Description = "Retrieves snapshot activity groups for a Cloud Diagram layer."
	s.MarkdownDescription = "Retrieves snapshot activity groups for a Cloud Diagram layer."
	resp.Schema = s
}

func (d *cloudDiagramsActivityGroupsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *cloudDiagramsActivityGroupsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data cloudDiagramsActivityGroupsDataSourceModel

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

	if !req.Config.Raw.IsFullyKnown() {
		data.CloudDiagramsActivityGroups = types.SetUnknown(ds.CloudDiagramsActivityGroupsValue{}.Type(ctx))
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	// Prepare query parameters.
	params := models.ListCloudDiagramActivityGroupsParams{
		SsId: data.SsId.ValueString(),
	}

	if !data.Limit.IsNull() {
		params.Limit = new(int(data.Limit.ValueInt64()))
	}

	if !data.Offset.IsNull() {
		params.Offset = new(int(data.Offset.ValueInt64()))
	}
	if !data.Tags.IsNull() {
		var tags []string
		resp.Diagnostics.Append(data.Tags.ElementsAs(ctx, &tags, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		params.Tags = &tags
	}

	// Call the API.
	apiResp, err := d.client.ListCloudDiagramActivityGroupsWithResponse(ctx, &params)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Cloud Diagram Activity Groups",
			fmt.Sprintf("Unable to list activity groups: %v", err),
		)
		return
	}

	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Cloud Diagram Activity Groups",
			fmt.Sprintf("Cloud Diagrams API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	// Map API response to Terraform state.
	var groupVals []ds.CloudDiagramsActivityGroupsValue
	if len(*apiResp.JSON200) > 0 {
		groupVals = make([]ds.CloudDiagramsActivityGroupsValue, 0, len(*apiResp.JSON200))
		for _, g := range *apiResp.JSON200 {
			itemsList, itemsDiags := mapActivityItems(ctx, g.Items)
			resp.Diagnostics.Append(itemsDiags...)
			if resp.Diagnostics.HasError() {
				return
			}

			tagsList, tagsDiags := mapStringList(ctx, g.Tags)
			resp.Diagnostics.Append(tagsDiags...)
			if resp.Diagnostics.HasError() {
				return
			}

			groupVal, valDiags := ds.NewCloudDiagramsActivityGroupsValue(
				ds.CloudDiagramsActivityGroupsValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"_id":         types.StringValue(g.UnderscoreId),
					"items":       itemsList,
					"snapshot":    types.StringValue(g.Snapshot),
					"statussheet": types.StringValue(g.Statussheet),
					"tags":        tagsList,
					"timestamp":   types.StringValue(g.Timestamp.UTC().Format(time.RFC3339)),
				},
			)
			resp.Diagnostics.Append(valDiags...)
			if resp.Diagnostics.HasError() {
				return
			}
			groupVals = append(groupVals, groupVal)
		}
	} else {
		groupVals = []ds.CloudDiagramsActivityGroupsValue{}
	}

	groupsSet, setDiags := types.SetValueFrom(ctx, ds.CloudDiagramsActivityGroupsValue{}.Type(ctx), groupVals)
	resp.Diagnostics.Append(setDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.CloudDiagramsActivityGroups = groupsSet

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// mapActivityItems maps a slice of CloudDiagramActivityItem to a Terraform list.
func mapActivityItems(ctx context.Context, items *[]models.CloudDiagramActivityItem) (types.List, diag.Diagnostics) {
	itemType := ds.ItemsValue{}.Type(ctx)

	if items == nil || len(*items) == 0 {
		return types.ListValueFrom(ctx, itemType, []ds.ItemsValue{})
	}

	var diags diag.Diagnostics
	vals := make([]ds.ItemsValue, 0, len(*items))
	for _, item := range *items {
		tagsList, tagsDiags := mapStringList(ctx, item.Tags)
		diags.Append(tagsDiags...)
		if diags.HasError() {
			return types.ListNull(itemType), diags
		}

		// Metadata is free-form (additionalProperties: true). Map as JSON string.
		metadataVal := mapFreeformJSON(item.Metadata)

		val, valDiags := ds.NewItemsValue(
			ds.ItemsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"_id":          types.StringValue(item.UnderscoreId),
				"activity":     types.StringValue(string(item.Activity)),
				"group":        types.StringValue(item.Group),
				"group_type":   types.StringPointerValue(item.GroupType),
				"metadata":     metadataVal,
				"service_type": types.StringPointerValue(item.ServiceType),
				"tags":         tagsList,
				"timestamp":    types.StringValue(item.Timestamp.UTC().Format(time.RFC3339)),
			},
		)
		diags.Append(valDiags...)
		if diags.HasError() {
			return types.ListNull(itemType), diags
		}
		vals = append(vals, val)
	}

	list, listDiags := types.ListValueFrom(ctx, itemType, vals)
	diags.Append(listDiags...)
	return list, diags
}

// mapStringList maps a *[]string to a Terraform list of strings.
func mapStringList(ctx context.Context, tags *[]string) (types.List, diag.Diagnostics) {
	if tags == nil || len(*tags) == 0 {
		return types.ListValueFrom(ctx, types.StringType, []string{})
	}
	return types.ListValueFrom(ctx, types.StringType, *tags)
}
