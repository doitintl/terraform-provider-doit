// Package provider implements the doit_report_result data source.
//
// This is a hand-written data source (not auto-generated) because the API
// response contains dynamic types that cannot be represented in Terraform's
// static type system:
//
//   - The GET /analytics/v1/reports/{id} response uses allOf (not supported
//     by the code generator).
//   - Report rows contain Value objects with mixed types (strings, numbers,
//     nulls) that vary per report.
//   - Rows are nested as [][]Value (list of lists), and Terraform does not
//     support list(list(dynamic)).
//
// Additionally, the generated Go type Value = map[string]interface{} does not
// match the actual API response. The API returns Value cells as primitives
// (strings, numbers, null), not as JSON objects. This means we cannot use the
// typed GetReportWithResponse method — it fails with:
//
//	"json: cannot unmarshal string into Go struct field
//	 RunReportResultResult.result.rows of type map[string]interface{}"
//
// Instead, we use the raw GetReport HTTP method and unmarshal the JSON body
// ourselves into map[string]interface{}, then serialize the "result" field
// as a JSON string. Users can parse it with jsondecode() in HCL.
//
// See also: Investigation in .test/Current Status of DCI API.md (2026-02-20).
package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Compile-time interface checks.
var _ datasource.DataSource = (*reportResultDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*reportResultDataSource)(nil)

// NewReportResultDataSource creates a new instance of the data source.
func NewReportResultDataSource() datasource.DataSource {
	return &reportResultDataSource{}
}

// reportResultDataSource implements datasource.DataSource for report results.
type reportResultDataSource struct {
	client *models.ClientWithResponses
}

// reportResultDataSourceModel is the Terraform state model.
type reportResultDataSourceModel struct {
	// Inputs
	Id        types.String `tfsdk:"id"`
	TimeRange types.String `tfsdk:"time_range"`
	StartDate types.String `tfsdk:"start_date"`
	EndDate   types.String `tfsdk:"end_date"`

	// Outputs
	ResultJSON types.String `tfsdk:"result_json"`
	ReportName types.String `tfsdk:"report_name"`
	CacheHit   types.Bool   `tfsdk:"cache_hit"`
	RowCount   types.Int64  `tfsdk:"row_count"`
}

func (d *reportResultDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_report_result"
}

func (d *reportResultDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches the results of an existing Cloud Analytics report." +
			"\n\nThe report is executed and the results are returned as a JSON string in" +
			" `result_json`. Use Terraform's `jsondecode()` to parse the results." +
			"\n\n**Note:** Report results are dynamic — they change over time as new" +
			" billing data is ingested. Every `terraform plan` will re-execute the" +
			" report." +
			"\n\nThe `result_json` field contains the full result object including:" +
			"\n- `schema`: Column definitions (name and type)" +
			"\n- `rows`: Data rows (each row is an array of values)" +
			"\n- `forecastRows`: Forecast data rows (if applicable)" +
			"\n- `secondaryRows`: Secondary time range rows (if applicable)" +
			"\n- `cacheHit`: Whether results were served from cache",
		MarkdownDescription: "Fetches the results of an existing Cloud Analytics report." +
			"\n\nThe report is executed and the results are returned as a JSON string in" +
			" `result_json`. Use Terraform's `jsondecode()` to parse the results." +
			"\n\n~> **Note:** Report results are dynamic — they change over time as new" +
			" billing data is ingested. Every `terraform plan` will re-execute the" +
			" report." +
			"\n\nThe `result_json` field contains the full result object including:" +
			"\n- `schema`: Column definitions (name and type)" +
			"\n- `rows`: Data rows (each row is an array of values)" +
			"\n- `forecastRows`: Forecast data rows (if applicable)" +
			"\n- `secondaryRows`: Secondary time range rows (if applicable)" +
			"\n- `cacheHit`: Whether results were served from cache",
		Attributes: map[string]schema.Attribute{
			// --- Inputs ---
			"id": schema.StringAttribute{
				Description:         "The ID of the report to fetch results for.",
				MarkdownDescription: "The ID of the report to fetch results for.",
				Required:            true,
			},
			"time_range": schema.StringAttribute{
				Description: "An optional parameter to override the report time settings. " +
					"Value should be represented in the ISO 8601 duration format " +
					"P[n]Y[n]M[n]D (e.g., P7D for 7 days, P1M for 1 month).",
				MarkdownDescription: "An optional parameter to override the report time settings. " +
					"Value should be represented in the ISO 8601 duration format " +
					"`P[n]Y[n]M[n]D` (e.g., `P7D` for 7 days, `P1M` for 1 month).",
				Optional: true,
				Validators: []validator.String{
					// time_range is mutually exclusive with start_date/end_date
					stringvalidator.ConflictsWith(path.MatchRoot("start_date")),
					stringvalidator.ConflictsWith(path.MatchRoot("end_date")),
				},
			},
			"start_date": schema.StringAttribute{
				Description: "An optional parameter to override the report time settings. " +
					"Must be provided together with end_date. Format: yyyy-mm-dd.",
				MarkdownDescription: "An optional parameter to override the report time settings. " +
					"Must be provided together with `end_date`. Format: `yyyy-mm-dd`.",
				Optional: true,
				Validators: []validator.String{
					// start_date requires end_date
					stringvalidator.AlsoRequires(path.MatchRoot("end_date")),
				},
			},
			"end_date": schema.StringAttribute{
				Description: "An optional parameter to override the report time settings. " +
					"Must be provided together with start_date. Format: yyyy-mm-dd.",
				MarkdownDescription: "An optional parameter to override the report time settings. " +
					"Must be provided together with `start_date`. Format: `yyyy-mm-dd`.",
				Optional: true,
				Validators: []validator.String{
					// end_date requires start_date
					stringvalidator.AlsoRequires(path.MatchRoot("start_date")),
				},
			},

			// --- Outputs ---
			"result_json": schema.StringAttribute{
				Description: "The full report result as a JSON string. " +
					"Contains schema (column definitions), rows (data), and metadata. " +
					"Use jsondecode() to parse.",
				MarkdownDescription: "The full report result as a JSON string. " +
					"Contains `schema` (column definitions), `rows` (data), and metadata. " +
					"Use `jsondecode()` to parse.",
				Computed: true,
			},
			"report_name": schema.StringAttribute{
				Description:         "The name of the report.",
				MarkdownDescription: "The name of the report.",
				Computed:            true,
			},
			"cache_hit": schema.BoolAttribute{
				Description:         "If true, results were fetched from the cache.",
				MarkdownDescription: "If true, results were fetched from the cache.",
				Computed:            true,
			},
			"row_count": schema.Int64Attribute{
				Description:         "The number of data rows in the result.",
				MarkdownDescription: "The number of data rows in the result.",
				Computed:            true,
			},
		},
	}
}

func (d *reportResultDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *reportResultDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data reportResultDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If ID is unknown (depends on a resource not yet created), return early
	// with all computed attributes set to unknown.
	if data.Id.IsUnknown() {
		data.ResultJSON = types.StringUnknown()
		data.ReportName = types.StringUnknown()
		data.CacheHit = types.BoolUnknown()
		data.RowCount = types.Int64Unknown()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	// Build query parameters from optional time range overrides
	params := &models.GetReportParams{}
	if !data.TimeRange.IsNull() && !data.TimeRange.IsUnknown() {
		v := data.TimeRange.ValueString()
		params.TimeRange = &v
	}
	if !data.StartDate.IsNull() && !data.StartDate.IsUnknown() {
		v := data.StartDate.ValueString()
		params.StartDate = &v
	}
	if !data.EndDate.IsNull() && !data.EndDate.IsUnknown() {
		v := data.EndDate.ValueString()
		params.EndDate = &v
	}

	// We use the raw GetReport method instead of GetReportWithResponse because
	// the generated Go type Value = map[string]interface{} does not match the
	// actual API response. The API returns Value cells as primitives (strings,
	// numbers, null), not as JSON objects. GetReportWithResponse internally
	// tries to unmarshal into the typed struct and fails with:
	//   "json: cannot unmarshal string into Go struct field
	//    RunReportResultResult.result.rows of type map[string]interface{}"
	//
	// By using the raw HTTP response, we bypass the broken type parser and
	// unmarshal the JSON ourselves into map[string]interface{}.
	httpResp, err := d.client.GetReport(ctx, data.Id.ValueString(), params)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Report Results",
			"Could not read results for report ID "+data.Id.ValueString()+": "+err.Error(),
		)
		return
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Report Results",
			fmt.Sprintf("Could not read response body for report ID %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	if httpResp.StatusCode != 200 {
		resp.Diagnostics.AddError(
			"Error Reading Report Results",
			fmt.Sprintf("Could not read results for report ID %s, status: %d, body: %s",
				data.Id.ValueString(), httpResp.StatusCode, string(body)),
		)
		return
	}

	// Parse the raw JSON body ourselves to correctly handle the mixed types
	// in Value cells (strings, numbers, nulls).
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		resp.Diagnostics.AddError(
			"Error Parsing Report Results",
			fmt.Sprintf("Could not parse API response for report ID %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	// Extract report metadata
	if reportName, ok := raw["reportName"].(string); ok {
		data.ReportName = types.StringValue(reportName)
	} else {
		data.ReportName = types.StringNull()
	}

	// Extract result object and serialize it as JSON string
	if result, ok := raw["result"].(map[string]interface{}); ok {
		// Extract convenience attributes
		if cacheHit, ok := result["cacheHit"].(bool); ok {
			data.CacheHit = types.BoolValue(cacheHit)
		} else {
			data.CacheHit = types.BoolNull()
		}

		if rows, ok := result["rows"].([]interface{}); ok {
			data.RowCount = types.Int64Value(int64(len(rows)))
		} else {
			data.RowCount = types.Int64Value(0)
		}

		// Serialize the entire result object as JSON. This preserves the
		// original types (strings stay strings, numbers stay numbers) from
		// the API response. Users can parse with jsondecode() in HCL.
		resultJSON, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			resp.Diagnostics.AddError(
				"Error Serializing Report Results",
				fmt.Sprintf("Could not serialize report results to JSON: %v", marshalErr),
			)
			return
		}
		data.ResultJSON = types.StringValue(string(resultJSON))
	} else {
		// No result object in the response
		data.CacheHit = types.BoolNull()
		data.RowCount = types.Int64Value(0)
		data.ResultJSON = types.StringValue("{}")
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
