package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_cloud_incident"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*cloudIncidentDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*cloudIncidentDataSource)(nil)

func NewCloudIncidentDataSource() datasource.DataSource {
	return &cloudIncidentDataSource{}
}

type cloudIncidentDataSource struct {
	client *models.ClientWithResponses
}

func (ds *cloudIncidentDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_incident"
}

func (ds *cloudIncidentDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
	ds.client = client
}

func (ds *cloudIncidentDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_cloud_incident.CloudIncidentDataSourceSchema(ctx)
}

func (ds *cloudIncidentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state datasource_cloud_incident.CloudIncidentModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.Id.ValueString()
	incidentResp, err := ds.client.GetKnownIssueWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Error reading cloud incident", err.Error())
		return
	}
	if incidentResp.StatusCode() == 404 {
		resp.Diagnostics.AddError("Cloud incident not found", fmt.Sprintf("Cloud incident with ID %s not found", id))
		return
	}
	if incidentResp.StatusCode() != 200 || incidentResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error reading cloud incident",
			fmt.Sprintf("Unexpected status: %d, body: %s", incidentResp.StatusCode(), string(incidentResp.Body)),
		)
		return
	}

	incident := incidentResp.JSON200

	// Map fields from the anonymous struct response
	state.Id = types.StringPointerValue(incident.Id)
	state.CreateTime = types.Int64PointerValue(incident.CreateTime)
	state.Description = types.StringPointerValue(incident.Description)
	state.Product = types.StringPointerValue(incident.Product)
	state.Summary = types.StringPointerValue(incident.Summary)
	state.Symptoms = types.StringPointerValue(incident.Symptoms)
	state.Title = types.StringPointerValue(incident.Title)
	state.Workaround = types.StringPointerValue(incident.Workaround)

	if incident.Platform != nil {
		state.Platform = types.StringValue(string(*incident.Platform))
	} else {
		state.Platform = types.StringNull()
	}

	if incident.Status != nil {
		state.Status = types.StringValue(string(*incident.Status))
	} else {
		state.Status = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
