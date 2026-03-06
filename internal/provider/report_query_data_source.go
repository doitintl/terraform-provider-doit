// Package provider implements the doit_report_query data source.
//
// This is a hand-written data source because the query endpoint is POST-based
// and the code generator only supports GET endpoints for data sources. The
// config schema is derived from the generated doit_report resource schema via
// a recursive converter, avoiding ~800 lines of duplication.
//
// Like report_result, the output uses a JSON string for dynamic result data
// since report rows contain mixed types (string, number, null) that cannot
// be represented in Terraform's static type system.
package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_report"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	rsschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Compile-time interface checks.
var _ datasource.DataSource = (*reportQueryDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*reportQueryDataSource)(nil)

// NewReportQueryDataSource creates a new instance of the data source.
func NewReportQueryDataSource() datasource.DataSource {
	return &reportQueryDataSource{}
}

// reportQueryDataSource implements datasource.DataSource for ad-hoc report queries.
type reportQueryDataSource struct {
	client *models.ClientWithResponses
}

// reportQueryDataSourceModel is the Terraform state model.
type reportQueryDataSourceModel struct {
	// Input: reuses the generated ConfigValue type from the report resource.
	Config resource_report.ConfigValue `tfsdk:"config"`

	// Outputs
	ResultJSON types.String `tfsdk:"result_json"`
	CacheHit   types.Bool   `tfsdk:"cache_hit"`
	RowCount   types.Int64  `tfsdk:"row_count"`
}

func (d *reportQueryDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_report_query"
}

func (d *reportQueryDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	// Get the generated report resource schema and extract its "config" attribute.
	reportSchema := resource_report.ReportResourceSchema(ctx)
	configAttr, ok := reportSchema.Attributes["config"].(rsschema.SingleNestedAttribute)
	if !ok {
		resp.Diagnostics.AddError(
			"Internal Error",
			"Could not convert report config schema to SingleNestedAttribute. Please report this issue to the provider developers.",
		)
		return
	}

	// Convert the resource config attribute to a data source config attribute.
	dsConfigAttrs, convertDiags := convertResourceAttrsToDataSource(configAttr.Attributes)
	resp.Diagnostics.Append(convertDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Schema = dsschema.Schema{
		Description: "Runs an ad-hoc Cloud Analytics query without persisting a report." +
			"\n\nThe query is executed with the provided config and results are returned" +
			" as a JSON string in result_json. Use Terraform's jsondecode() to parse." +
			"\n\nNote: Query results are dynamic — they change over time as new billing" +
			" data is ingested. Every terraform plan will re-execute the query." +
			"\n\nThe result_json field contains the full result object, including the" +
			" schema (column definitions), rows (data), forecastRows (forecast data)," +
			" secondaryRows (secondary time range data), and cacheHit (whether results" +
			" were served from cache).",
		MarkdownDescription: "Runs an ad-hoc Cloud Analytics query without persisting a report." +
			"\n\nThe query is executed with the provided config and results are returned" +
			" as a JSON string in `result_json`. Use Terraform's `jsondecode()` to parse." +
			"\n\n~> **Note:** Query results are dynamic — they change over time as new" +
			" billing data is ingested. Every `terraform plan` will re-execute the query." +
			"\n\nThe `result_json` field contains the full result object, including the following" +
			" keys: `schema` (column definitions, including name and type), `rows` (data" +
			" rows, where each row is an array of values), `forecastRows` (forecast data" +
			" rows, if applicable), `secondaryRows` (secondary time range rows, if" +
			" applicable), and `cacheHit` (whether results were served from cache).",
		Attributes: map[string]dsschema.Attribute{
			// --- Input ---
			"config": dsschema.SingleNestedAttribute{
				Attributes:          dsConfigAttrs,
				CustomType:          configAttr.CustomType,
				Required:            true,
				Description:         "The report configuration. Same structure as the config attribute on the doit_report resource.",
				MarkdownDescription: "The report configuration. Same structure as the `config` attribute on the `doit_report` resource.",
			},

			// --- Outputs ---
			"result_json": dsschema.StringAttribute{
				Description: "The full query result as a JSON string. " +
					"Contains schema (column definitions), rows (data), and metadata. " +
					"Use jsondecode() to parse.",
				MarkdownDescription: "The full query result as a JSON string. " +
					"Contains `schema` (column definitions), `rows` (data), and metadata. " +
					"Use `jsondecode()` to parse.",
				Computed: true,
			},
			"cache_hit": dsschema.BoolAttribute{
				Description:         "If true, results were fetched from the cache.",
				MarkdownDescription: "If true, results were fetched from the cache.",
				Computed:            true,
			},
			"row_count": dsschema.Int64Attribute{
				Description:         "The number of data rows in the result.",
				MarkdownDescription: "The number of data rows in the result.",
				Computed:            true,
			},
		},
	}
}

func (d *reportQueryDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *reportQueryDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data reportQueryDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If the config contains any unknown values (e.g., during a plan where
	// inputs depend on unresolved resources), we cannot make a complete API query.
	// Return all computed attributes as unknown.
	if !req.Config.Raw.IsFullyKnown() {
		data.ResultJSON = types.StringUnknown()
		data.CacheHit = types.BoolUnknown()
		data.RowCount = types.Int64Unknown()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	// Convert the Terraform config to the API ExternalConfig type.
	// This reuses the same mapping function used by the doit_report resource.
	externalConfig, diags := toExternalConfig(ctx, data.Config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Call the query API
	queryResp, err := d.client.QueryWithResponse(ctx, models.QueryJSONRequestBody{
		Config: externalConfig,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Running Query",
			"Could not execute report query: "+err.Error(),
		)
		return
	}

	if queryResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Running Query",
			fmt.Sprintf("Query failed, status: %d, body: %s",
				queryResp.StatusCode(), string(queryResp.Body)),
		)
		return
	}

	if queryResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Running Query",
			"Received empty response body from query API",
		)
		return
	}

	result := queryResp.JSON200

	// Map result to outputs
	if result.Result != nil {
		data.CacheHit = types.BoolPointerValue(result.Result.CacheHit)

		if result.Result.Rows != nil {
			data.RowCount = types.Int64Value(int64(len(*result.Result.Rows)))
		} else {
			data.RowCount = types.Int64Value(0)
		}

		resultJSON, marshalErr := json.Marshal(result.Result)
		if marshalErr != nil {
			resp.Diagnostics.AddError(
				"Error Serializing Query Results",
				fmt.Sprintf("Could not serialize query results to JSON: %v", marshalErr),
			)
			return
		}
		data.ResultJSON = types.StringValue(string(resultJSON))
	} else {
		data.CacheHit = types.BoolNull()
		data.RowCount = types.Int64Value(0)
		data.ResultJSON = types.StringValue("{}")
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// convertResourceAttrsToDataSource recursively converts resource schema
// attributes to data source schema attributes. This allows the query data
// source to reuse the generated report resource config schema without
// duplicating ~800 lines of attribute definitions.
//
// The CustomType fields (e.g., ConfigType, MetricType) are framework-agnostic
// and work identically in both resource and data source contexts.
func convertResourceAttrsToDataSource(attrs map[string]rsschema.Attribute) (map[string]dsschema.Attribute, diag.Diagnostics) {
	var diags diag.Diagnostics
	result := make(map[string]dsschema.Attribute, len(attrs))

	for name, attr := range attrs {
		switch a := attr.(type) {
		case rsschema.StringAttribute:
			result[name] = dsschema.StringAttribute{
				Description:         a.Description,
				MarkdownDescription: a.MarkdownDescription,
				Required:            a.Required,
				Optional:            a.Optional,
				Computed:            a.Computed,
				Sensitive:           a.Sensitive,
				Validators:          a.Validators,
				CustomType:          a.CustomType,
			}
		case rsschema.BoolAttribute:
			result[name] = dsschema.BoolAttribute{
				Description:         a.Description,
				MarkdownDescription: a.MarkdownDescription,
				Required:            a.Required,
				Optional:            a.Optional,
				Computed:            a.Computed,
				Sensitive:           a.Sensitive,
				Validators:          a.Validators,
				CustomType:          a.CustomType,
			}
		case rsschema.Int64Attribute:
			result[name] = dsschema.Int64Attribute{
				Description:         a.Description,
				MarkdownDescription: a.MarkdownDescription,
				Required:            a.Required,
				Optional:            a.Optional,
				Computed:            a.Computed,
				Sensitive:           a.Sensitive,
				Validators:          a.Validators,
				CustomType:          a.CustomType,
			}
		case rsschema.Float64Attribute:
			result[name] = dsschema.Float64Attribute{
				Description:         a.Description,
				MarkdownDescription: a.MarkdownDescription,
				Required:            a.Required,
				Optional:            a.Optional,
				Computed:            a.Computed,
				Sensitive:           a.Sensitive,
				Validators:          a.Validators,
				CustomType:          a.CustomType,
			}
		case rsschema.ListAttribute:
			result[name] = dsschema.ListAttribute{
				ElementType:         a.ElementType,
				Description:         a.Description,
				MarkdownDescription: a.MarkdownDescription,
				Required:            a.Required,
				Optional:            a.Optional,
				Computed:            a.Computed,
				Sensitive:           a.Sensitive,
				Validators:          a.Validators,
				CustomType:          a.CustomType,
			}
		case rsschema.SingleNestedAttribute:
			nestedAttrs, nestedDiags := convertResourceAttrsToDataSource(a.Attributes)
			diags.Append(nestedDiags...)
			result[name] = dsschema.SingleNestedAttribute{
				Attributes:          nestedAttrs,
				CustomType:          a.CustomType,
				Description:         a.Description,
				MarkdownDescription: a.MarkdownDescription,
				Required:            a.Required,
				Optional:            a.Optional,
				Computed:            a.Computed,
				Sensitive:           a.Sensitive,
				Validators:          a.Validators,
			}
		case rsschema.ListNestedAttribute:
			nestedAttrs, nestedDiags := convertResourceAttrsToDataSource(a.NestedObject.Attributes)
			diags.Append(nestedDiags...)
			result[name] = dsschema.ListNestedAttribute{
				NestedObject: dsschema.NestedAttributeObject{
					Attributes: nestedAttrs,
					CustomType: a.NestedObject.CustomType,
					Validators: a.NestedObject.Validators,
				},
				Description:         a.Description,
				MarkdownDescription: a.MarkdownDescription,
				Required:            a.Required,
				Optional:            a.Optional,
				Computed:            a.Computed,
				Sensitive:           a.Sensitive,
				Validators:          a.Validators,
				CustomType:          a.CustomType,
			}
		default:
			diags.AddError(
				"Unsupported Attribute Type in Schema Converter",
				fmt.Sprintf("Attribute %q has type %T which is not handled by convertResourceAttrsToDataSource. "+
					"This is a provider bug — please report it to the provider developers.", name, attr),
			)
		}
	}

	return result, diags
}
