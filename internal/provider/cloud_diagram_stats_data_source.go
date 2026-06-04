package provider

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_cloud_diagram_stats"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Compile-time interface checks.
var _ datasource.DataSource = (*cloudDiagramStatsDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*cloudDiagramStatsDataSource)(nil)

// NewCloudDiagramStatsDataSource creates a new instance of the data source.
func NewCloudDiagramStatsDataSource() datasource.DataSource {
	return &cloudDiagramStatsDataSource{}
}

// cloudDiagramStatsDataSource implements datasource.DataSource for cloud diagram stats.
type cloudDiagramStatsDataSource struct {
	client *models.ClientWithResponses
}

// cloudDiagramStatsDataSourceModel is the Terraform state model.
type cloudDiagramStatsDataSourceModel struct {
	Id                types.String   `tfsdk:"id"`
	Start             types.String   `tfsdk:"start"`
	End               types.String   `tfsdk:"end"`
	CloudDiagramStats types.Set      `tfsdk:"cloud_diagram_stats"`
	Timeouts          timeouts.Value `tfsdk:"timeouts"`
}

func (d *cloudDiagramStatsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_diagram_stats"
}

func (d *cloudDiagramStatsDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	genSchema := datasource_cloud_diagram_stats.CloudDiagramStatsDataSourceSchema(ctx)

	// Add computed id.
	genSchema.Attributes["id"] = schema.StringAttribute{
		Computed:            true,
		Description:         "A deterministic hash of the query parameters, used as the data source identifier.",
		MarkdownDescription: "A deterministic hash of the query parameters, used as the data source identifier.",
	}

	genSchema.Attributes["timeouts"] = timeouts.Attributes(ctx)

	resp.Schema = genSchema
}

func (d *cloudDiagramStatsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *cloudDiagramStatsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data cloudDiagramStatsDataSourceModel

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

	// If the config contains any unknown values, return all computed attributes as unknown.
	if !req.Config.Raw.IsFullyKnown() {
		data.Id = types.StringUnknown()
		data.CloudDiagramStats = types.SetUnknown(datasource_cloud_diagram_stats.CloudDiagramStatsValue{}.Type(ctx))
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	// Parse the required start/end timestamps.
	startTime, err := time.Parse(time.RFC3339, data.Start.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Start Time",
			fmt.Sprintf("Could not parse 'start' as RFC3339 timestamp: %v", err),
		)
		return
	}

	endTime, err := time.Parse(time.RFC3339, data.End.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid End Time",
			fmt.Sprintf("Could not parse 'end' as RFC3339 timestamp: %v", err),
		)
		return
	}

	params := &models.GetCloudDiagramsStatsParams{
		Start: startTime,
		End:   endTime,
	}

	apiResp, err := d.client.GetCloudDiagramsStatsWithResponse(ctx, params)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Cloud Diagram Stats",
			fmt.Sprintf("Unable to read cloud diagram stats: %v", err),
		)
		return
	}

	if apiResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Reading Cloud Diagram Stats",
			fmt.Sprintf("Cloud Diagram Stats API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Cloud Diagram Stats",
			fmt.Sprintf("Cloud Diagram Stats API returned status 200 but response body could not be parsed: %s", string(apiResp.Body)),
		)
		return
	}

	// Map API response to Terraform state.
	resp.Diagnostics.Append(mapStatsToState(ctx, &data, *apiResp.JSON200)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Set a deterministic ID based on query parameters.
	idInput := fmt.Sprintf("cloud_diagram_stats\nstart:%s\nend:%s", data.Start.ValueString(), data.End.ValueString())
	hash := sha256.Sum256([]byte(idInput))
	data.Id = types.StringValue(fmt.Sprintf("%x", hash))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// mapStatsToState maps the API response to the Terraform state model.
func mapStatsToState(
	ctx context.Context,
	data *cloudDiagramStatsDataSourceModel,
	stats []models.CloudDiagramStats,
) diag.Diagnostics {
	var diags diag.Diagnostics

	statsVals := make([]datasource_cloud_diagram_stats.CloudDiagramStatsValue, 0, len(stats))
	for _, s := range stats {
		// Map changes list.
		changesList, changeDiags := mapStatsChanges(ctx, s.Changes)
		diags.Append(changeDiags...)
		if diags.HasError() {
			return diags
		}

		// Map import object.
		importVal, importDiags := mapStatsImport(ctx, s.Import)
		diags.Append(importDiags...)
		if diags.HasError() {
			return diags
		}

		// Map the diagram type.
		var diagramType types.String
		if s.Type != nil {
			diagramType = types.StringValue(string(*s.Type))
		} else {
			diagramType = types.StringNull()
		}

		val, valDiags := datasource_cloud_diagram_stats.NewCloudDiagramStatsValue(
			datasource_cloud_diagram_stats.CloudDiagramStatsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"_id":          types.StringPointerValue(s.UnderscoreId),
				"account_id":   types.StringPointerValue(s.AccountId),
				"account_name": types.StringPointerValue(s.AccountName),
				"account_type": types.StringPointerValue(s.AccountType),
				"changes":      changesList,
				"import":       importVal,
				"name":         types.StringPointerValue(s.Name),
				"ss_id":        types.StringPointerValue(s.SsId),
				"type":         diagramType,
			},
		)
		diags.Append(valDiags...)
		if diags.HasError() {
			return diags
		}
		statsVals = append(statsVals, val)
	}

	statsList, listDiags := types.SetValueFrom(ctx, datasource_cloud_diagram_stats.CloudDiagramStatsValue{}.Type(ctx), statsVals)
	diags.Append(listDiags...)
	if diags.HasError() {
		return diags
	}
	data.CloudDiagramStats = statsList

	return diags
}

// mapStatsChanges maps the changes array from the API response.
func mapStatsChanges(
	ctx context.Context,
	changes *[]models.CloudDiagramStatsChange,
) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := datasource_cloud_diagram_stats.ChangesValue{}.Type(ctx)

	if changes == nil || len(*changes) == 0 {
		emptyList, listDiags := types.ListValueFrom(ctx, elemType, []datasource_cloud_diagram_stats.ChangesValue{})
		diags.Append(listDiags...)
		return emptyList, diags
	}

	vals := make([]datasource_cloud_diagram_stats.ChangesValue, 0, len(*changes))
	for _, c := range *changes {
		// Map change type enum.
		var changeType types.String
		if c.Type != nil {
			changeType = types.StringValue(string(*c.Type))
		} else {
			changeType = types.StringNull()
		}

		// Map count.
		var count types.Int64
		if c.Count != nil {
			count = types.Int64Value(int64(*c.Count))
		} else {
			count = types.Int64Null()
		}

		val, valDiags := datasource_cloud_diagram_stats.NewChangesValue(
			datasource_cloud_diagram_stats.ChangesValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"type":    changeType,
				"service": types.StringPointerValue(c.Service),
				"count":   count,
			},
		)
		diags.Append(valDiags...)
		if diags.HasError() {
			return types.ListNull(elemType), diags
		}
		vals = append(vals, val)
	}

	result, listDiags := types.ListValueFrom(ctx, elemType, vals)
	diags.Append(listDiags...)
	return result, diags
}

// mapStatsImport maps the import object from the API response.
func mapStatsImport(
	ctx context.Context,
	importState *models.CloudDiagramImportState,
) (datasource_cloud_diagram_stats.ImportValue, diag.Diagnostics) {
	if importState == nil {
		return datasource_cloud_diagram_stats.NewImportValueNull(), nil
	}

	// Map status enum.
	var status types.String
	if importState.Status != nil {
		status = types.StringValue(string(*importState.Status))
	} else {
		status = types.StringNull()
	}

	// Map type enum.
	var importType types.String
	if importState.Type != nil {
		importType = types.StringValue(string(*importState.Type))
	} else {
		importType = types.StringNull()
	}

	// Map synced_at timestamp.
	var syncedAt types.String
	if importState.SyncedAt != nil {
		syncedAt = types.StringValue(importState.SyncedAt.Format(time.RFC3339))
	} else {
		syncedAt = types.StringNull()
	}

	return datasource_cloud_diagram_stats.NewImportValue(
		datasource_cloud_diagram_stats.ImportValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"account":       types.StringPointerValue(importState.Account),
			"cloud_id":      types.StringPointerValue(importState.CloudId),
			"error_message": types.StringPointerValue(importState.ErrorMessage),
			"status":        status,
			"synced_at":     syncedAt,
			"type":          importType,
		},
	)
}
