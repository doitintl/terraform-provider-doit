package provider

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_composer_recipes"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*composerRecipesDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*composerRecipesDataSource)(nil)

func NewComposerRecipesDataSource() datasource.DataSource {
	return &composerRecipesDataSource{}
}

type composerRecipesDataSource struct {
	client *models.ClientWithResponses
}

type composerRecipesDataSourceModel struct {
	datasource_composer_recipes.ComposerRecipesModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *composerRecipesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_composer_recipes"
}

func (d *composerRecipesDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_composer_recipes.ComposerRecipesDataSourceSchema(ctx)
	resp.Diagnostics.Append(setListPaginationDescriptions(ctx, &s, listRowCountTotal,
		"A `page_token` works only with the `sort_by`, `sort_order`, `filter` and `max_results` it was returned for, and expires 5 minutes after it was issued. "+
			"An invalid or expired one fails the read; read again without `page_token` to start over.")...)
	s.Attributes["timeouts"] = timeouts.Attributes(ctx)
	resp.Schema = s
}

func (d *composerRecipesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *composerRecipesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data composerRecipesDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	readTimeout, diags := data.Timeouts.Read(ctx, DefaultReadTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	// The results are not yet known. page_token is one of them in every mode:
	// a read replaces it with the next page's token, or null. The inputs stay
	// as configured.
	if !req.Config.Raw.IsFullyKnown() {
		data.Items = types.ListUnknown(datasource_composer_recipes.ItemsValue{}.Type(ctx))
		data.RowCount = types.Int64Unknown()
		data.PageToken = types.StringUnknown()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	params := &models.ListComposerRecipesParams{}
	if !data.Filter.IsNull() {
		params.Filter = new(data.Filter.ValueString())
	}
	if !data.SortBy.IsNull() {
		params.SortBy = new(models.ListComposerRecipesParamsSortBy(data.SortBy.ValueString()))
	}
	if !data.SortOrder.IsNull() {
		params.SortOrder = new(models.ListComposerRecipesParamsSortOrder(data.SortOrder.ValueString()))
	}

	var allItems []models.ComposerRecipe
	userControlsPagination := !data.MaxResults.IsNull()

	if userControlsPagination {
		params.MaxResults = new(int(data.MaxResults.ValueInt64()))
		if !data.PageToken.IsNull() {
			params.PageToken = new(data.PageToken.ValueString())
		}

		apiResp, err := d.client.ListComposerRecipesWithResponse(ctx, params)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Reading Composer Recipes",
				fmt.Sprintf("Unable to read composer recipes: %v", err),
			)
			return
		}
		if apiResp.StatusCode() != http.StatusOK || apiResp.JSON200 == nil {
			resp.Diagnostics.AddError(
				"Error Reading Composer Recipes",
				fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
			)
			return
		}

		result := apiResp.JSON200
		allItems = result.Items
		data.PageToken = types.StringPointerValue(result.PageToken)
		data.RowCount = listRowCount(&result.RowCount, len(allItems))
	} else {
		if !data.PageToken.IsNull() {
			params.PageToken = new(data.PageToken.ValueString())
		}
		// rowCount is the total across all pages, the same on every page, so it
		// is taken from the first page even when a page_token starts the read
		// after the first item.
		var apiRowCount *int64
		for first := true; ; first = false {
			apiResp, err := d.client.ListComposerRecipesWithResponse(ctx, params)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error Reading Composer Recipes",
					fmt.Sprintf("Unable to read composer recipes: %v", err),
				)
				return
			}
			if apiResp.StatusCode() != http.StatusOK || apiResp.JSON200 == nil {
				resp.Diagnostics.AddError(
					"Error Reading Composer Recipes",
					fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
				)
				return
			}

			result := apiResp.JSON200
			if first {
				apiRowCount = &result.RowCount
			}
			allItems = append(allItems, result.Items...)

			if result.PageToken == nil || *result.PageToken == "" {
				break
			}
			params.PageToken = result.PageToken
		}

		data.RowCount = listRowCount(apiRowCount, len(allItems))
		data.PageToken = types.StringNull()
		data.MaxResults = types.Int64Null()
	}

	itemsList, mapDiags := mapComposerRecipesItemsList(ctx, allItems)
	resp.Diagnostics.Append(mapDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Items = itemsList

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func mapComposerRecipesItemsList(ctx context.Context, items []models.ComposerRecipe) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics

	if len(items) == 0 {
		emptyList, d := types.ListValueFrom(ctx, datasource_composer_recipes.ItemsValue{}.Type(ctx), []datasource_composer_recipes.ItemsValue{})
		diags.Append(d...)
		return emptyList, diags
	}

	vals := make([]datasource_composer_recipes.ItemsValue, 0, len(items))
	for _, item := range items {
		var detectionTime types.String
		if item.DetectionTime != nil {
			detectionTime = types.StringValue(item.DetectionTime.UTC().Format(time.RFC3339Nano))
		} else {
			detectionTime = types.StringNull()
		}

		var estimatedFixTimeMinutes types.Int64
		if item.EstimatedFixTimeMinutes != nil {
			estimatedFixTimeMinutes = types.Int64Value(int64(*item.EstimatedFixTimeMinutes))
		} else {
			estimatedFixTimeMinutes = types.Int64Null()
		}

		var status types.String
		if item.Status != nil {
			status = types.StringValue(string(*item.Status))
		} else {
			status = types.StringNull()
		}

		importantPointsSlice := []string{}
		if item.Metadata.ImportantPoints != nil && len(*item.Metadata.ImportantPoints) > 0 {
			importantPointsSlice = *item.Metadata.ImportantPoints
		}
		importantPointsList, d := types.ListValueFrom(ctx, types.StringType, importantPointsSlice)
		diags.Append(d...)
		if diags.HasError() {
			return types.ListNull(datasource_composer_recipes.ItemsValue{}.Type(ctx)), diags
		}

		metadataVal, metadataDiags := datasource_composer_recipes.NewMetadataValue(
			datasource_composer_recipes.MetadataValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"impacts":            types.StringPointerValue(item.Metadata.Impacts),
				"important_points":   importantPointsList,
				"manual_remediation": types.StringPointerValue(item.Metadata.ManualRemediation),
				"reference":          types.StringPointerValue(item.Metadata.Reference),
			},
		)
		diags.Append(metadataDiags...)
		if diags.HasError() {
			return types.ListNull(datasource_composer_recipes.ItemsValue{}.Type(ctx)), diags
		}

		itemVal, itemDiags := datasource_composer_recipes.NewItemsValue(
			datasource_composer_recipes.ItemsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"active_results_count":       types.Int64Value(int64(item.ActiveResultsCount)),
				"affected_resource_type":     types.StringPointerValue(item.AffectedResourceType),
				"description":                types.StringPointerValue(item.Description),
				"detection_time":             detectionTime,
				"estimated_fix_time_minutes": estimatedFixTimeMinutes,
				"id":                         types.Int64Value(item.Id),
				"is_dismissed":               types.BoolValue(item.IsDismissed),
				"metadata":                   metadataVal,
				"provider":                   types.StringValue(string(item.Provider)),
				"resolved_results_count":     types.Int64Value(int64(item.ResolvedResultsCount)),
				"severity":                   types.StringValue(string(item.Severity)),
				"snoozed_results_count":      types.Int64Value(int64(item.SnoozedResultsCount)),
				"status":                     status,
				"title":                      types.StringValue(item.Title),
				"total_potential_savings":    types.StringPointerValue(item.TotalPotentialSavings),
				"total_results_count":        types.Int64Value(int64(item.TotalResultsCount)),
			},
		)
		diags.Append(itemDiags...)
		if diags.HasError() {
			return types.ListNull(datasource_composer_recipes.ItemsValue{}.Type(ctx)), diags
		}

		vals = append(vals, itemVal)
	}

	resultList, d := types.ListValueFrom(ctx, datasource_composer_recipes.ItemsValue{}.Type(ctx), vals)
	diags.Append(d...)
	return resultList, diags
}
