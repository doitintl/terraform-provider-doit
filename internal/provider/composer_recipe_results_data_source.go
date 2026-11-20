package provider

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_composer_recipe_results"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*composerRecipeResultsDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*composerRecipeResultsDataSource)(nil)

func NewComposerRecipeResultsDataSource() datasource.DataSource {
	return &composerRecipeResultsDataSource{}
}

type composerRecipeResultsDataSource struct {
	client *models.ClientWithResponses
}

type composerRecipeResultsDataSourceModel struct {
	datasource_composer_recipe_results.ComposerRecipeResultsModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *composerRecipeResultsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_composer_recipe_results"
}

func (d *composerRecipeResultsDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_composer_recipe_results.ComposerRecipeResultsDataSourceSchema(ctx)
	resp.Diagnostics.Append(setListPaginationDescriptions(ctx, &s, listRowCountTotal,
		"A `page_token` works only with the `sort_by`, `sort_order`, `filter` and `max_results` it was returned for, and expires 5 minutes after it was issued. "+
			"An invalid or expired one fails the read; read again without `page_token` to start over.")...)
	s.Attributes["timeouts"] = timeouts.Attributes(ctx)
	resp.Schema = s
}

func (d *composerRecipeResultsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *composerRecipeResultsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data composerRecipeResultsDataSourceModel

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
		data.Items = types.ListUnknown(datasource_composer_recipe_results.ItemsValue{}.Type(ctx))
		data.RowCount = types.Int64Unknown()
		data.PageToken = types.StringUnknown()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	recipeID := data.RecipeId.ValueInt64()

	params := &models.ListComposerRecipeResultsParams{}
	if !data.Filter.IsNull() {
		params.Filter = new(data.Filter.ValueString())
	}
	if !data.SortBy.IsNull() {
		params.SortBy = new(models.ListComposerRecipeResultsParamsSortBy(data.SortBy.ValueString()))
	}
	if !data.SortOrder.IsNull() {
		params.SortOrder = new(models.ListComposerRecipeResultsParamsSortOrder(data.SortOrder.ValueString()))
	}

	var allItems []models.ComposerRecipeResult
	userControlsPagination := !data.MaxResults.IsNull()

	if userControlsPagination {
		params.MaxResults = new(int(data.MaxResults.ValueInt64()))
		if !data.PageToken.IsNull() {
			params.PageToken = new(data.PageToken.ValueString())
		}

		apiResp, err := d.client.ListComposerRecipeResultsWithResponse(ctx, recipeID, params)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Reading Composer Recipe Results",
				fmt.Sprintf("Unable to read composer recipe results for recipe %d: %v", recipeID, err),
			)
			return
		}
		if apiResp.StatusCode() != http.StatusOK || apiResp.JSON200 == nil {
			resp.Diagnostics.AddError(
				"Error Reading Composer Recipe Results",
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
			apiResp, err := d.client.ListComposerRecipeResultsWithResponse(ctx, recipeID, params)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error Reading Composer Recipe Results",
					fmt.Sprintf("Unable to read composer recipe results for recipe %d: %v", recipeID, err),
				)
				return
			}
			if apiResp.StatusCode() != http.StatusOK || apiResp.JSON200 == nil {
				resp.Diagnostics.AddError(
					"Error Reading Composer Recipe Results",
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

	itemsList, mapDiags := mapComposerRecipeResultsItemsList(ctx, allItems)
	resp.Diagnostics.Append(mapDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Items = itemsList

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func mapComposerRecipeResultsItemsList(ctx context.Context, items []models.ComposerRecipeResult) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics

	if len(items) == 0 {
		emptyList, d := types.ListValueFrom(ctx, datasource_composer_recipe_results.ItemsValue{}.Type(ctx), []datasource_composer_recipe_results.ItemsValue{})
		diags.Append(d...)
		return emptyList, diags
	}

	vals := make([]datasource_composer_recipe_results.ItemsValue, 0, len(items))
	for _, item := range items {
		var resolvedTime types.String
		if item.ResolvedTime != nil {
			resolvedTime = types.StringValue(item.ResolvedTime.UTC().Format(time.RFC3339Nano))
		} else {
			resolvedTime = types.StringNull()
		}

		var tagsMap types.Map
		if tags := nullableToPointer(item.Tags); tags != nil {
			var d diag.Diagnostics
			tagsMap, d = types.MapValueFrom(ctx, types.StringType, *tags)
			diags.Append(d...)
			if diags.HasError() {
				return types.ListNull(datasource_composer_recipe_results.ItemsValue{}.Type(ctx)), diags
			}
		} else {
			tagsMap = types.MapNull(types.StringType)
		}

		itemVal, itemDiags := datasource_composer_recipe_results.NewItemsValue(
			datasource_composer_recipe_results.ItemsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"account_id": types.StringValue(item.AccountId),
				"cost_estimation_potential_freed_commitments":              types.Float64PointerValue(item.CostEstimationPotentialFreedCommitments),
				"cost_estimation_potential_increased_commitments_required": types.Float64PointerValue(item.CostEstimationPotentialIncreasedCommitmentsRequired),
				"cost_estimation_potential_increased_costs":                types.Float64PointerValue(item.CostEstimationPotentialIncreasedCosts),
				"cost_estimation_potential_savings":                        types.Float64PointerValue(item.CostEstimationPotentialSavings),
				"detection_time":                                           types.StringValue(item.DetectionTime.UTC().Format(time.RFC3339Nano)),
				"first_flow_id":                                            types.StringPointerValue(item.FirstFlowId),
				"id":                                                       types.Int64Value(item.Id),
				"is_active":                                                types.BoolValue(item.IsActive),
				"issued_resource_id":                                       types.StringValue(item.IssuedResourceId),
				"issued_resource_name":                                     types.StringValue(item.IssuedResourceName),
				"last_flow_id":                                             types.StringPointerValue(item.LastFlowId),
				"provider":                                                 types.StringValue(string(item.Provider)),
				"provider_identifier":                                      types.StringPointerValue(nullableToPointer(item.ProviderIdentifier)),
				"region":                                                   types.StringPointerValue(item.Region),
				"resolved_time":                                            resolvedTime,
				"resource_class":                                           types.StringPointerValue(item.ResourceClass),
				"rule_id":                                                  types.Int64Value(item.RuleId),
				"snoozed":                                                  types.BoolPointerValue(item.Snoozed),
				"solved":                                                   types.BoolValue(item.Solved),
				"tags":                                                     tagsMap,
			},
		)
		diags.Append(itemDiags...)
		if diags.HasError() {
			return types.ListNull(datasource_composer_recipe_results.ItemsValue{}.Type(ctx)), diags
		}

		vals = append(vals, itemVal)
	}

	resultList, d := types.ListValueFrom(ctx, datasource_composer_recipe_results.ItemsValue{}.Type(ctx), vals)
	diags.Append(d...)
	return resultList, diags
}
