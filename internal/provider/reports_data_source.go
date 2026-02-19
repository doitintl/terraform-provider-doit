package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_reports"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*reportsDataSource)(nil)

func NewReportsDataSource() datasource.DataSource {
	return &reportsDataSource{}
}

type reportsDataSource struct {
	client *models.ClientWithResponses
}

type reportsDataSourceModel struct {
	datasource_reports.ReportsModel
}

func (d *reportsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_reports"
}

func (d *reportsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_reports.ReportsDataSourceSchema(ctx)
}

func (d *reportsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *reportsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data reportsDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If any filter/pagination input is unknown, return unknown list
	if data.Filter.IsUnknown() || data.MinCreationTime.IsUnknown() || data.MaxCreationTime.IsUnknown() || data.MaxResults.IsUnknown() || data.PageToken.IsUnknown() {
		data.Reports = types.ListUnknown(datasource_reports.ReportsValue{}.Type(ctx))
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}
	// Build query parameters
	params := &models.ListReportsParams{}
	if !data.Filter.IsNull() {
		filter := data.Filter.ValueString()
		params.Filter = &filter
	}
	if !data.MinCreationTime.IsNull() {
		minCreationTime := data.MinCreationTime.ValueString()
		params.MinCreationTime = &minCreationTime
	}
	if !data.MaxCreationTime.IsNull() {
		maxCreationTime := data.MaxCreationTime.ValueString()
		params.MaxCreationTime = &maxCreationTime
	}

	// Smart pagination: honor user-provided values, otherwise auto-paginate
	userControlsPagination := !data.MaxResults.IsNull()

	var allReports []models.Report

	if userControlsPagination {
		// Manual mode: single API call with user's params
		maxResultsVal := data.MaxResults.ValueString()
		params.MaxResults = &maxResultsVal
		if !data.PageToken.IsNull() {
			pageTokenVal := data.PageToken.ValueString()
			params.PageToken = &pageTokenVal
		}

		apiResp, err := d.client.ListReportsWithResponse(ctx, params)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Reading Reports",
				fmt.Sprintf("Unable to read reports: %v", err),
			)
			return
		}

		if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
			resp.Diagnostics.AddError(
				"Error Reading Reports",
				fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
			)
			return
		}

		result := apiResp.JSON200
		if result.Reports != nil {
			allReports = *result.Reports
		}

		// Preserve API's page_token for user to fetch next page
		data.PageToken = types.StringPointerValue(result.PageToken)
		if result.RowCount != nil {
			data.RowCount = types.Int64Value(*result.RowCount)
		} else {
			data.RowCount = types.Int64Value(int64(len(allReports)))
		}
		// max_results is already set by user, no change needed
	} else {
		// Auto mode: fetch all pages, honoring user-provided page_token as starting point
		if !data.PageToken.IsNull() {
			pageTokenVal := data.PageToken.ValueString()
			params.PageToken = &pageTokenVal
		}
		for {
			apiResp, err := d.client.ListReportsWithResponse(ctx, params)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error Reading Reports",
					fmt.Sprintf("Unable to read reports: %v", err),
				)
				return
			}

			if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
				resp.Diagnostics.AddError(
					"Error Reading Reports",
					fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
				)
				return
			}

			result := apiResp.JSON200
			if result.Reports != nil {
				allReports = append(allReports, *result.Reports...)
			}

			if result.PageToken == nil || *result.PageToken == "" {
				break
			}
			params.PageToken = result.PageToken
		}

		// Auto mode: set counts based on what we fetched
		data.RowCount = types.Int64Value(int64(len(allReports)))
		data.PageToken = types.StringNull()
		// max_results was not set; preserve null
	}

	// Map reports list
	if len(allReports) > 0 {
		reportVals := make([]datasource_reports.ReportsValue, 0, len(allReports))
		for _, report := range allReports {
			// Handle optional Type enum
			var typeVal types.String
			if report.Type != nil {
				typeVal = types.StringValue(string(*report.Type))
			} else {
				typeVal = types.StringNull()
			}

			reportVal, diags := datasource_reports.NewReportsValue(
				datasource_reports.ReportsValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"id":          types.StringPointerValue(report.Id),
					"report_name": types.StringPointerValue(report.ReportName),
					"owner":       types.StringPointerValue(report.Owner),
					"type":        typeVal,
					"create_time": types.Int64PointerValue(report.CreateTime),
					"update_time": types.Int64PointerValue(report.UpdateTime),
					"url_ui":      types.StringPointerValue(report.UrlUI),
				},
			)
			resp.Diagnostics.Append(diags...)
			reportVals = append(reportVals, reportVal)
		}

		reportList, diags := types.ListValueFrom(ctx, datasource_reports.ReportsValue{}.Type(ctx), reportVals)
		resp.Diagnostics.Append(diags...)
		data.Reports = reportList
	} else {
		emptyList, diags := types.ListValueFrom(ctx, datasource_reports.ReportsValue{}.Type(ctx), []datasource_reports.ReportsValue{})
		resp.Diagnostics.Append(diags...)
		data.Reports = emptyList
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
