package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_insight_resource_results"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*insightResourceResultsDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*insightResourceResultsDataSource)(nil)

func NewInsightResourceResultsDataSource() datasource.DataSource {
	return &insightResourceResultsDataSource{}
}

type insightResourceResultsDataSource struct {
	client *models.ClientWithResponses
}

type insightResourceResultsDataSourceModel struct {
	datasource_insight_resource_results.InsightResourceResultsModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (ds *insightResourceResultsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_insight_resource_results"
}

func (ds *insightResourceResultsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (ds *insightResourceResultsDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_insight_resource_results.InsightResourceResultsDataSourceSchema(ctx)
	s.Attributes["timeouts"] = timeouts.Attributes(ctx)
	resp.Schema = s
}

func (ds *insightResourceResultsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data insightResourceResultsDataSourceModel
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

	// Guard against any unknown inputs (source_id, insight_key, max_results,
	// page_token). Using IsFullyKnown catches derived values that aren't
	// resolved yet during planning — prevents calling ValueString/ValueInt64
	// on unknowns. Only computed outputs are set to unknown; max_results is
	// a pure user input and left untouched.
	if !req.Config.Raw.IsFullyKnown() {
		data.ResourceResults = types.ListUnknown(datasource_insight_resource_results.ResourceResultsValue{}.Type(ctx))
		data.RowCount = types.Int64Unknown()
		data.PageToken = types.StringUnknown()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	sourceID := data.SourceId.ValueString()
	insightKey := data.InsightKey.ValueString()

	params := &models.GetInsightResourceResultsParams{}

	// Smart pagination: honor user-provided values, otherwise auto-paginate
	userControlsPagination := !data.MaxResults.IsNull()

	var allResults []models.ResourceResult

	if userControlsPagination {
		// Manual mode: single API call with user's params
		params.MaxResults = new(int(data.MaxResults.ValueInt64()))
		if !data.PageToken.IsNull() {
			params.PageToken = new(data.PageToken.ValueString())
		}

		apiResp, err := ds.client.GetInsightResourceResultsWithResponse(ctx, sourceID, insightKey, params)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Reading Insight Resource Results",
				fmt.Sprintf("Unable to read resource results: %v", err),
			)
			return
		}
		if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
			resp.Diagnostics.AddError(
				"Error Reading Insight Resource Results",
				fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
			)
			return
		}

		result := apiResp.JSON200
		allResults = result.ResourceResults

		// Preserve API's page_token and row_count
		data.PageToken = types.StringPointerValue(result.PageToken)
		data.RowCount = types.Int64Value(int64(result.RowCount))
	} else {
		// Auto mode: fetch all pages
		if !data.PageToken.IsNull() {
			params.PageToken = new(data.PageToken.ValueString())
		}
		for {
			apiResp, err := ds.client.GetInsightResourceResultsWithResponse(ctx, sourceID, insightKey, params)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error Reading Insight Resource Results",
					fmt.Sprintf("Unable to read resource results: %v", err),
				)
				return
			}
			if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
				resp.Diagnostics.AddError(
					"Error Reading Insight Resource Results",
					fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
				)
				return
			}

			result := apiResp.JSON200
			allResults = append(allResults, result.ResourceResults...)

			if result.PageToken == nil || *result.PageToken == "" {
				break
			}
			params.PageToken = result.PageToken
		}

		// Auto mode: set counts
		data.RowCount = types.Int64Value(int64(len(allResults)))
		data.PageToken = types.StringNull()
	}

	// Map resource results to Terraform model
	resp.Diagnostics.Append(mapResourceResultsListToModel(ctx, allResults, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// mapResourceResultsListToModel maps a slice of ResourceResult to the data source model.
func mapResourceResultsListToModel(ctx context.Context, results []models.ResourceResult, data *insightResourceResultsDataSourceModel) (diags diag.Diagnostics) {
	if len(results) == 0 {
		emptyList, d := types.ListValueFrom(ctx, datasource_insight_resource_results.ResourceResultsValue{}.Type(ctx), []datasource_insight_resource_results.ResourceResultsValue{})
		diags.Append(d...)
		data.ResourceResults = emptyList
		return diags
	}

	resultVals := make([]datasource_insight_resource_results.ResourceResultsValue, 0, len(results))
	for _, rr := range results {
		val, d := mapResourceResultToValue(ctx, &rr)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}
		resultVals = append(resultVals, val)
	}

	resultsList, d := types.ListValueFrom(ctx, datasource_insight_resource_results.ResourceResultsValue{}.Type(ctx), resultVals)
	diags.Append(d...)
	data.ResourceResults = resultsList
	return diags
}

// mapResourceResultToValue maps a single ResourceResult to the generated ResourceResultsValue type.
func mapResourceResultToValue(ctx context.Context, rr *models.ResourceResult) (datasource_insight_resource_results.ResourceResultsValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	// Enhancement
	var enhancementVal datasource_insight_resource_results.EnhancementValue
	if rr.Enhancement != nil {
		// Priority sub-object
		var priorityVal datasource_insight_resource_results.PriorityValue
		if rr.Enhancement.Priority != nil {
			var d diag.Diagnostics
			priorityVal, d = datasource_insight_resource_results.NewPriorityValue(
				datasource_insight_resource_results.PriorityValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"priority_score": types.Float64PointerValue(rr.Enhancement.Priority.PriorityScore),
					"value":          types.StringPointerValue(rr.Enhancement.Priority.Value),
				},
			)
			diags.Append(d...)
		} else {
			priorityVal = datasource_insight_resource_results.NewPriorityValueNull()
		}

		// Tags
		var tagsVal types.List
		if rr.Enhancement.Tags != nil {
			var d diag.Diagnostics
			tagsVal, d = types.ListValueFrom(ctx, types.StringType, *rr.Enhancement.Tags)
			diags.Append(d...)
		} else {
			var d diag.Diagnostics
			tagsVal, d = types.ListValueFrom(ctx, types.StringType, []string{})
			diags.Append(d...)
		}

		// LastUpdatedAt
		var lastUpdatedAtVal types.String
		if rr.Enhancement.LastUpdatedAt != nil {
			lastUpdatedAtVal = types.StringValue(rr.Enhancement.LastUpdatedAt.UTC().Format(time.RFC3339))
		} else {
			lastUpdatedAtVal = types.StringNull()
		}

		var d diag.Diagnostics
		enhancementVal, d = datasource_insight_resource_results.NewEnhancementValue(
			datasource_insight_resource_results.EnhancementValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"last_updated_at": lastUpdatedAtVal,
				"last_updated_by": types.StringPointerValue(rr.Enhancement.LastUpdatedBy),
				"priority":        priorityVal,
				"tags":            tagsVal,
			},
		)
		diags.Append(d...)
	} else {
		enhancementVal = datasource_insight_resource_results.NewEnhancementValueNull()
	}

	// Result sub-object
	var resultVal datasource_insight_resource_results.ResultValue
	if rr.Result != nil {
		var d diag.Diagnostics
		resultVal, d = datasource_insight_resource_results.NewResultValue(
			datasource_insight_resource_results.ResultValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"agent_installed": types.BoolPointerValue(rr.Result.AgentInstalled),
				"critical":        intPtrToInt64Value(rr.Result.Critical),
				"current":         types.StringPointerValue(rr.Result.Current),
				"high":            intPtrToInt64Value(rr.Result.High),
				"low":             intPtrToInt64Value(rr.Result.Low),
				"medium":          intPtrToInt64Value(rr.Result.Medium),
				"recommendation":  types.StringPointerValue(rr.Result.Recommendation),
				"value":           types.Float64PointerValue(rr.Result.Value),
			},
		)
		diags.Append(d...)
	} else {
		resultVal = datasource_insight_resource_results.NewResultValueNull()
	}

	// Metadata — free-form JSON, serialized from API response.
	metadataVal := mapFreeformJSON(rr.Metadata)

	// Severity
	var severityVal types.String
	if rr.Severity != nil {
		severityVal = types.StringValue(string(*rr.Severity))
	} else {
		severityVal = types.StringNull()
	}

	rrVal, d := datasource_insight_resource_results.NewResourceResultsValue(
		datasource_insight_resource_results.ResourceResultsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"account":        types.StringValue(rr.Account),
			"cloud_provider": types.StringValue(rr.CloudProvider),
			"enhancement":    enhancementVal,
			"external_id":    types.StringPointerValue(rr.ExternalId),
			"external_url":   types.StringPointerValue(rr.ExternalUrl),
			"location":       types.StringPointerValue(rr.Location),
			"metadata":       metadataVal,
			"resolved":       types.BoolPointerValue(rr.Resolved),
			"resource_id":    types.StringValue(rr.ResourceId),
			"resource_type":  types.StringPointerValue(rr.ResourceType),
			"result":         resultVal,
			"result_type":    types.StringValue(string(rr.ResultType)),
			"severity":       severityVal,
		},
	)
	diags.Append(d...)

	return rrVal, diags
}
