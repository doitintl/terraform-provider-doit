// Package provider implements the doit_report_result data source.
//
// This is a hand-written data source (not auto-generated) because the report
// result contains dynamic types that cannot be represented in Terraform's
// static type system. Specifically, report rows are [][]*Value where each
// Value can be a string, number, or null — and Terraform does not support
// list(list(dynamic)).
//
// The entire "result" field is serialized as a JSON string so that the
// original types are preserved. Users parse it with jsondecode() in HCL.
package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	openapi_types "github.com/oapi-codegen/runtime/types"
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
			" result_json. Use Terraform's jsondecode() to parse the results." +
			"\n\nNote: Report results are dynamic — they change over time as new" +
			" billing data is ingested. Every terraform plan will re-execute the" +
			" report." +
			"\n\nThe result_json field contains the full result object, including the" +
			" schema (column definitions), rows (data), forecastRows (forecast data)," +
			" secondaryRows (secondary time range data), and cacheHit (whether results" +
			" were served from cache).",
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
					iso8601DurationValidator{},
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
					dateValidator{},
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
					dateValidator{},
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

	// If ID or any query parameter is unknown, return early
	// with all computed attributes set to unknown.
	if data.Id.IsUnknown() || data.TimeRange.IsUnknown() || data.StartDate.IsUnknown() || data.EndDate.IsUnknown() {
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
		params.TimeRange = new(data.TimeRange.ValueString())
	}
	if !data.StartDate.IsNull() && !data.StartDate.IsUnknown() {
		startDate, err := time.Parse(time.DateOnly, data.StartDate.ValueString())
		if err != nil {
			resp.Diagnostics.AddError(
				"Invalid Start Date",
				fmt.Sprintf("Could not parse start_date as yyyy-mm-dd: %s", err.Error()),
			)
			return
		}
		params.StartDate = &openapi_types.Date{Time: startDate}
	}
	if !data.EndDate.IsNull() && !data.EndDate.IsUnknown() {
		endDate, err := time.Parse(time.DateOnly, data.EndDate.ValueString())
		if err != nil {
			resp.Diagnostics.AddError(
				"Invalid End Date",
				fmt.Sprintf("Could not parse end_date as yyyy-mm-dd: %s", err.Error()),
			)
			return
		}
		params.EndDate = &openapi_types.Date{Time: endDate}
	}

	// Call the API. The GetReportResponse type is a flat struct (no allOf
	// composition), and Value is a json.RawMessage union that correctly
	// handles mixed-type cells (string, number, null).
	reportResp, err := d.client.GetReportWithResponse(ctx, data.Id.ValueString(), params)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Report Results",
			"Could not read results for report ID "+data.Id.ValueString()+": "+err.Error(),
		)
		return
	}

	if reportResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Reading Report Results",
			fmt.Sprintf("Could not read results for report ID %s, status: %d, body: %s",
				data.Id.ValueString(), reportResp.StatusCode(), string(reportResp.Body)),
		)
		return
	}

	if reportResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Report Results",
			"Received empty response body for report ID "+data.Id.ValueString(),
		)
		return
	}

	report := reportResp.JSON200

	// Map metadata to typed attributes
	data.ReportName = types.StringPointerValue(report.ReportName)

	// Map result to JSON string
	if report.Result != nil {
		// Extract convenience attributes from the result
		data.CacheHit = types.BoolPointerValue(report.Result.CacheHit)

		if report.Result.Rows != nil {
			data.RowCount = types.Int64Value(int64(len(*report.Result.Rows)))
		} else {
			data.RowCount = types.Int64Value(0)
		}

		// Serialize the entire result object as JSON. The Value union type
		// implements MarshalJSON via json.RawMessage, so strings stay strings
		// and numbers stay numbers in the output.
		resultJSON, marshalErr := json.Marshal(report.Result)
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
