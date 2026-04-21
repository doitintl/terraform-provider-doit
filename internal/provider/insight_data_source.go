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

func (d *insightDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_insight"
}

func (d *insightDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *insightDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_insight.InsightDataSourceSchema(ctx)

	s.MarkdownDescription = "Retrieves a single insight by source ID and insight key."
	s.Description = "Retrieves a single insight by source ID and insight key."

	s.Attributes["timeouts"] = timeouts.Attributes(ctx)

	resp.Schema = s
}

func (d *insightDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
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

	// If inputs are unknown (depend on resources not yet created), set all
	// computed attributes to unknown so consumers don't treat null as a real value during planning.
	if data.SourceId.IsUnknown() || data.InsightKey.IsUnknown() {
		setInsightComputedFieldsUnknown(ctx, &data)
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	sourceID := data.SourceId.ValueString()
	insightKey := data.InsightKey.ValueString()

	apiResp, err := d.client.GetInsightResultWithResponse(ctx, sourceID, insightKey)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Insight",
			fmt.Sprintf("Unable to read insight %s/%s: %v", sourceID, insightKey, err),
		)
		return
	}

	if apiResp.StatusCode() == 404 {
		resp.Diagnostics.AddError(
			"Insight Not Found",
			fmt.Sprintf("Insight with source %q and key %q not found", sourceID, insightKey),
		)
		return
	}

	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Insight",
			fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	mapInsightResponseToModel(ctx, apiResp.JSON200, &data, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// mapInsightResponseToModel maps an InsightResponse to the singular data source model.
func mapInsightResponseToModel(ctx context.Context, insight *models.InsightResponse, data *insightDataSourceModel, diagnostics *diag.Diagnostics) {
	// Map categories list
	categoriesList := mapStringPointerSliceToList(ctx, func() *[]string {
		if insight.Categories == nil {
			return nil
		}
		cats := make([]string, len(*insight.Categories))
		for i, c := range *insight.Categories {
			cats[i] = string(c)
		}
		return &cats
	}(), diagnostics)

	// Map tags list
	tagsList := mapStringPointerSliceToList(ctx, insight.Tags, diagnostics)

	// Map summary nested object
	var summaryVal datasource_insight.SummaryValue
	if insight.Summary != nil {
		var diags diag.Diagnostics
		summaryVal, diags = datasource_insight.NewSummaryValue(
			datasource_insight.SummaryValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"operational_risks":       float32PtrToNumberValue(insight.Summary.OperationalRisks),
				"performance_risks":       float32PtrToNumberValue(insight.Summary.PerformanceRisks),
				"potential_daily_savings": float32PtrToNumberValue(insight.Summary.PotentialDailySavings),
				"reliability_risks":       float32PtrToNumberValue(insight.Summary.ReliabilityRisks),
				"security_risks":          float32PtrToNumberValue(insight.Summary.SecurityRisks),
				"sustainability_risks":    float32PtrToNumberValue(insight.Summary.SustainabilityRisks),
			},
		)
		diagnostics.Append(diags...)
	} else {
		summaryVal = datasource_insight.NewSummaryValueNull()
	}

	// Map last status change nested object
	var lastStatusChangeVal datasource_insight.LastStatusChangeValue
	if insight.LastStatusChange != nil {
		var diags diag.Diagnostics
		lastStatusChangeVal, diags = datasource_insight.NewLastStatusChangeValue(
			datasource_insight.LastStatusChangeValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"last_changed_at": types.StringValue(insight.LastStatusChange.LastChangedAt.String()),
				"user_id":         types.StringValue(insight.LastStatusChange.UserId),
			},
		)
		diagnostics.Append(diags...)
	} else {
		lastStatusChangeVal = datasource_insight.NewLastStatusChangeValueNull()
	}

	// Map display_status enum
	var displayStatusVal types.String
	if insight.DisplayStatus != nil {
		displayStatusVal = types.StringValue(string(*insight.DisplayStatus))
	} else {
		displayStatusVal = types.StringNull()
	}

	// Map cloud_provider
	var cloudProviderVal types.String
	if insight.CloudProvider != nil {
		cloudProviderVal = types.StringValue(*insight.CloudProvider)
	} else {
		cloudProviderVal = types.StringNull()
	}

	// Map source
	var sourceVal types.String
	if insight.Source != nil {
		sourceVal = types.StringValue(*insight.Source)
	} else {
		sourceVal = types.StringNull()
	}

	// Map last_updated
	var lastUpdatedVal types.String
	if insight.LastUpdated != nil {
		lastUpdatedVal = types.StringValue(insight.LastUpdated.String())
	} else {
		lastUpdatedVal = types.StringNull()
	}

	data.Categories = categoriesList
	data.CloudFlowTemplateId = types.StringPointerValue(insight.CloudFlowTemplateId)
	data.CloudProvider = cloudProviderVal
	data.DetailedDescriptionMdx = types.StringPointerValue(insight.DetailedDescriptionMdx)
	data.DisplayStatus = displayStatusVal
	data.EasyWinDescription = types.StringPointerValue(insight.EasyWinDescription)
	data.Key = types.StringPointerValue(insight.Key)
	data.LastStatusChange = lastStatusChangeVal
	data.LastUpdated = lastUpdatedVal
	data.ReportUrl = types.StringPointerValue(insight.ReportUrl)
	data.ShortDescription = types.StringPointerValue(insight.ShortDescription)
	data.Source = sourceVal
	data.Summary = summaryVal
	data.Tags = tagsList
	data.Title = types.StringPointerValue(insight.Title)
}

// setInsightComputedFieldsUnknown sets all computed attributes to unknown for planning.
func setInsightComputedFieldsUnknown(_ context.Context, data *insightDataSourceModel) {
	data.Categories = types.ListUnknown(types.StringType)
	data.CloudFlowTemplateId = types.StringUnknown()
	data.CloudProvider = types.StringUnknown()
	data.DetailedDescriptionMdx = types.StringUnknown()
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
}
