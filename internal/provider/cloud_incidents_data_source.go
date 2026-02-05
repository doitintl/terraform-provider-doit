package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_cloud_incidents"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*cloudIncidentsDataSource)(nil)

func NewCloudIncidentsDataSource() datasource.DataSource {
	return &cloudIncidentsDataSource{}
}

type cloudIncidentsDataSource struct {
	client *models.ClientWithResponses
}

type cloudIncidentsDataSourceModel struct {
	datasource_cloud_incidents.CloudIncidentsModel
}

func (d *cloudIncidentsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_incidents"
}

func (d *cloudIncidentsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_cloud_incidents.CloudIncidentsDataSourceSchema(ctx)
}

func (d *cloudIncidentsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *cloudIncidentsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data cloudIncidentsDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build query parameters
	params := &models.ListKnownIssuesParams{}
	if !data.Filter.IsNull() && !data.Filter.IsUnknown() {
		filter := data.Filter.ValueString()
		params.Filter = &filter
	}
	if !data.MinCreationTime.IsNull() && !data.MinCreationTime.IsUnknown() {
		minCreationTime := data.MinCreationTime.ValueString()
		params.MinCreationTime = &minCreationTime
	}
	if !data.MaxCreationTime.IsNull() && !data.MaxCreationTime.IsUnknown() {
		maxCreationTime := data.MaxCreationTime.ValueString()
		params.MaxCreationTime = &maxCreationTime
	}

	// Cloud incidents can have thousands of records, so we always use manual pagination
	// (no auto-pagination) to avoid very long fetch times.
	if !data.MaxResults.IsNull() && !data.MaxResults.IsUnknown() {
		maxResultsVal := data.MaxResults.ValueInt64()
		params.MaxResults = &maxResultsVal
	}
	if !data.PageToken.IsNull() && !data.PageToken.IsUnknown() {
		pageTokenVal := data.PageToken.ValueString()
		params.PageToken = &pageTokenVal
	}

	apiResp, err := d.client.ListKnownIssuesWithResponse(ctx, params)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Cloud Incidents",
			fmt.Sprintf("Unable to read cloud incidents: %v", err),
		)
		return
	}

	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Cloud Incidents",
			fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	result := apiResp.JSON200
	var allIncidents []models.CloudIncidentListItem
	if result.Incidents != nil {
		allIncidents = *result.Incidents
	}

	// Preserve API's page_token for user to fetch next page
	data.PageToken = types.StringPointerValue(result.PageToken)
	if result.RowCount != nil {
		data.RowCount = types.Int64Value(*result.RowCount)
	} else {
		data.RowCount = types.Int64Value(int64(len(allIncidents)))
	}
	// Preserve max_results null/unknown handling
	if data.MaxResults.IsUnknown() {
		data.MaxResults = types.Int64Null()
	}

	// Map incidents list
	if len(allIncidents) > 0 {
		incidentVals := make([]datasource_cloud_incidents.IncidentsValue, 0, len(allIncidents))
		for _, inc := range allIncidents {
			// Handle optional Platform enum
			var platformVal types.String
			if inc.Platform != nil {
				platformVal = types.StringValue(string(*inc.Platform))
			} else {
				platformVal = types.StringNull()
			}

			// Handle optional Status enum
			var statusVal types.String
			if inc.Status != nil {
				statusVal = types.StringValue(string(*inc.Status))
			} else {
				statusVal = types.StringNull()
			}

			incVal, diags := datasource_cloud_incidents.NewIncidentsValue(
				datasource_cloud_incidents.IncidentsValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"id":          types.StringPointerValue(inc.Id),
					"create_time": types.Int64PointerValue(inc.CreateTime),
					"platform":    platformVal,
					"product":     types.StringPointerValue(inc.Product),
					"status":      statusVal,
					"title":       types.StringPointerValue(inc.Title),
				},
			)
			resp.Diagnostics.Append(diags...)
			incidentVals = append(incidentVals, incVal)
		}

		incidentList, diags := types.ListValueFrom(ctx, datasource_cloud_incidents.IncidentsValue{}.Type(ctx), incidentVals)
		resp.Diagnostics.Append(diags...)
		data.Incidents = incidentList
	} else {
		data.Incidents = types.ListNull(datasource_cloud_incidents.IncidentsValue{}.Type(ctx))
	}

	// Set optional filter params to null if they were unknown
	if data.Filter.IsUnknown() {
		data.Filter = types.StringNull()
	}
	if data.MinCreationTime.IsUnknown() {
		data.MinCreationTime = types.StringNull()
	}
	if data.MaxCreationTime.IsUnknown() {
		data.MaxCreationTime = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
