package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_insight"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*insightDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*insightDataSource)(nil)

func NewInsightDataSource() datasource.DataSource {
	return &insightDataSource{}
}

type insightDataSource struct {
	client *models.ClientWithResponses
}

type insightDataSourceModel struct {
	datasource_insight.InsightModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (ds *insightDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_insight"
}

func (ds *insightDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (ds *insightDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_insight.InsightDataSourceSchema(ctx)
	s.Attributes["timeouts"] = timeouts.Attributes(ctx)
	resp.Schema = s
}

func (ds *insightDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data insightDataSourceModel
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

	// If any required input is unknown (depends on a resource not yet created),
	// set all computed attributes to unknown so consumers don't treat null as a
	// real value during planning.
	if data.SourceId.IsUnknown() || data.InsightKey.IsUnknown() {
		data.Categories = types.ListUnknown(types.StringType)
		data.CloudFlowTemplateId = types.StringUnknown()
		data.CloudProvider = types.StringUnknown()
		data.DetailedDescriptionMdx = types.StringUnknown()
		data.DismissalDetails = datasource_insight.NewDismissalDetailsValueUnknown()
		data.DisplayStatus = types.StringUnknown()
		data.EasyWinDescription = types.StringUnknown()
		data.Key = types.StringUnknown()
		data.LastStatusChange = datasource_insight.NewLastStatusChangeValueUnknown()
		data.LastUpdated = types.StringUnknown()
		data.ReportUrl = types.StringUnknown()
		data.ShortDescription = types.StringUnknown()
		data.Source = types.StringUnknown()
		data.Summary = datasource_insight.NewSummaryValueUnknown()
		data.Tags = types.ListUnknown(types.StringType)
		data.Title = types.StringUnknown()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	sourceID := data.SourceId.ValueString()
	insightKey := data.InsightKey.ValueString()

	apiResp, err := ds.client.GetInsightResultWithResponse(ctx, sourceID, insightKey)
	if err != nil {
		resp.Diagnostics.AddError("Error reading insight", err.Error())
		return
	}
	if apiResp.StatusCode() == 404 {
		resp.Diagnostics.AddError(
			"Insight not found",
			fmt.Sprintf("Insight %s/%s not found", sourceID, insightKey),
		)
		return
	}
	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error reading insight",
			fmt.Sprintf("Unexpected status: %d, body: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	resp.Diagnostics.Append(mapInsightRespToDatasourceModel(ctx, apiResp.JSON200, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// mapInsightRespToDatasourceModel maps an InsightResponse to the data source model.
// This is the data source equivalent of mapInsightRespToResourceModel.
func mapInsightRespToDatasourceModel(ctx context.Context, insight *models.InsightResponse, data *insightDataSourceModel) (diags diag.Diagnostics) {
	// Identity fields
	data.Key = types.StringPointerValue(insight.Key)
	if insight.Source != nil {
		data.Source = types.StringValue(*insight.Source)
	} else {
		data.Source = types.StringNull()
	}

	// Core fields
	data.Title = types.StringPointerValue(insight.Title)
	data.ShortDescription = types.StringPointerValue(insight.ShortDescription)
	data.DetailedDescriptionMdx = stringPtrOrNull(insight.DetailedDescriptionMdx)
	data.EasyWinDescription = stringPtrOrNull(insight.EasyWinDescription)
	data.ReportUrl = stringPtrOrNull(insight.ReportUrl)
	data.CloudFlowTemplateId = stringPtrOrNull(insight.CloudFlowTemplateId)

	if insight.CloudProvider != nil {
		data.CloudProvider = types.StringValue(*insight.CloudProvider)
	} else {
		data.CloudProvider = types.StringNull()
	}

	if insight.DisplayStatus != nil {
		data.DisplayStatus = types.StringValue(string(*insight.DisplayStatus))
	} else {
		data.DisplayStatus = types.StringNull()
	}

	// LastUpdated is *time.Time
	if insight.LastUpdated != nil {
		data.LastUpdated = types.StringValue(insight.LastUpdated.UTC().Format(time.RFC3339))
	} else {
		data.LastUpdated = types.StringNull()
	}

	// Categories
	if insight.Categories != nil {
		catStrings := make([]string, len(*insight.Categories))
		for i, c := range *insight.Categories {
			catStrings[i] = string(c)
		}
		catList, catDiags := types.ListValueFrom(ctx, types.StringType, catStrings)
		diags.Append(catDiags...)
		data.Categories = catList
	} else {
		var catDiags diag.Diagnostics
		data.Categories, catDiags = types.ListValueFrom(ctx, types.StringType, []string{})
		diags.Append(catDiags...)
	}

	// Tags
	if insight.Tags != nil {
		tagList, tagDiags := types.ListValueFrom(ctx, types.StringType, *insight.Tags)
		diags.Append(tagDiags...)
		data.Tags = tagList
	} else {
		var tagDiags diag.Diagnostics
		data.Tags, tagDiags = types.ListValueFrom(ctx, types.StringType, []string{})
		diags.Append(tagDiags...)
	}

	// Summary
	if insight.Summary != nil {
		summaryVal, summaryDiags := datasource_insight.NewSummaryValue(
			datasource_insight.SummaryValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"operational_risks":       types.Float64PointerValue(insight.Summary.OperationalRisks),
				"performance_risks":       types.Float64PointerValue(insight.Summary.PerformanceRisks),
				"potential_daily_savings": types.Float64PointerValue(insight.Summary.PotentialDailySavings),
				"reliability_risks":       types.Float64PointerValue(insight.Summary.ReliabilityRisks),
				"security_risks":          types.Float64PointerValue(insight.Summary.SecurityRisks),
				"sustainability_risks":    types.Float64PointerValue(insight.Summary.SustainabilityRisks),
			},
		)
		diags.Append(summaryDiags...)
		data.Summary = summaryVal
	} else {
		data.Summary = datasource_insight.NewSummaryValueNull()
	}

	// LastStatusChange
	if insight.LastStatusChange != nil {
		lscVal, lscDiags := datasource_insight.NewLastStatusChangeValue(
			datasource_insight.LastStatusChangeValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"last_changed_at": types.StringValue(insight.LastStatusChange.LastChangedAt.UTC().Format(time.RFC3339)),
				"user_id":         types.StringValue(insight.LastStatusChange.UserId),
			},
		)
		diags.Append(lscDiags...)
		data.LastStatusChange = lscVal
	} else {
		data.LastStatusChange = datasource_insight.NewLastStatusChangeValueNull()
	}

	// DismissalDetails
	if insight.DismissalDetails != nil {
		reasonVal := types.StringNull()
		if insight.DismissalDetails.Reason != nil {
			reasonVal = types.StringValue(string(*insight.DismissalDetails.Reason))
		}
		commentVal := types.StringNull()
		if insight.DismissalDetails.Comment != nil {
			commentVal = types.StringValue(*insight.DismissalDetails.Comment)
		}
		ddVal, ddDiags := datasource_insight.NewDismissalDetailsValue(
			datasource_insight.DismissalDetailsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"reason":  reasonVal,
				"comment": commentVal,
			},
		)
		diags.Append(ddDiags...)
		data.DismissalDetails = ddVal
	} else {
		data.DismissalDetails = datasource_insight.NewDismissalDetailsValueNull()
	}

	return diags
}
