package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_service_quotas"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*serviceQuotasDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*serviceQuotasDataSource)(nil)

func NewServiceQuotasDataSource() datasource.DataSource {
	return &serviceQuotasDataSource{}
}

type serviceQuotasDataSource struct {
	client *models.ClientWithResponses
}

type serviceQuotasDataSourceModel struct {
	datasource_service_quotas.ServiceQuotasModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *serviceQuotasDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_quotas"
}

func (d *serviceQuotasDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
	d.client = client
}

func (d *serviceQuotasDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_service_quotas.ServiceQuotasDataSourceSchema(ctx)
	s.Attributes["timeouts"] = timeouts.Attributes(ctx)
	resp.Schema = s
}

func (d *serviceQuotasDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data serviceQuotasDataSourceModel
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

	// Composite inputs (config as a whole) require IsFullyKnown to catch unknown elements.
	if !req.Config.Raw.IsFullyKnown() {
		data.Items = types.ListUnknown(datasource_service_quotas.ItemsValue{}.Type(ctx))
		data.RowCount = types.Int64Unknown()
		data.PageToken = types.StringUnknown()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	// Build query parameters from user-provided filter fields
	params := &models.ListServiceQuotasParams{}
	if !data.CloudProvider.IsNull() {
		params.CloudProvider = new(models.ListServiceQuotasParamsCloudProvider(data.CloudProvider.ValueString()))
	}
	if !data.MinUtilizationPercent.IsNull() {
		params.MinUtilizationPercent = new(data.MinUtilizationPercent.ValueFloat64())
	}

	// Smart pagination: honor user-provided values, otherwise auto-paginate
	userControlsPagination := !data.MaxResults.IsNull()

	var allQuotas []models.ServiceQuota

	if userControlsPagination {
		// Manual mode: single API call with user's params
		params.MaxResults = new(int32(data.MaxResults.ValueInt64())) //nolint:gosec // G115: bounded to [1,200] by int64validator.Between in the generated schema
		if !data.PageToken.IsNull() {
			params.PageToken = new(data.PageToken.ValueString())
		}

		apiResp, err := d.client.ListServiceQuotasWithResponse(ctx, params)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Reading Service Quotas",
				fmt.Sprintf("Unable to read service quotas: %v", err),
			)
			return
		}
		if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
			resp.Diagnostics.AddError(
				"Error Reading Service Quotas",
				fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
			)
			return
		}

		result := apiResp.JSON200
		allQuotas = result.Items

		// Preserve API's page_token for the user to fetch the next page
		data.PageToken = types.StringPointerValue(result.PageToken)
		data.RowCount = types.Int64Value(result.RowCount)
		// max_results is already set by the user, no change needed
	} else {
		// Auto mode: fetch all pages, honoring a user-provided page_token as the starting point
		if !data.PageToken.IsNull() {
			params.PageToken = new(data.PageToken.ValueString())
		}
		for {
			apiResp, err := d.client.ListServiceQuotasWithResponse(ctx, params)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error Reading Service Quotas",
					fmt.Sprintf("Unable to read service quotas: %v", err),
				)
				return
			}
			if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
				resp.Diagnostics.AddError(
					"Error Reading Service Quotas",
					fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
				)
				return
			}

			result := apiResp.JSON200
			allQuotas = append(allQuotas, result.Items...)

			if result.PageToken == nil || *result.PageToken == "" {
				break
			}
			params.PageToken = result.PageToken
		}

		// Auto mode: set counts based on what was fetched; no more pages remain
		data.RowCount = types.Int64Value(int64(len(allQuotas)))
		data.PageToken = types.StringNull()
		// max_results was not set by the user; normalize to null
		data.MaxResults = types.Int64Null()
	}

	// Map service quotas to the Terraform model
	resp.Diagnostics.Append(mapServiceQuotasToModel(ctx, allQuotas, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// mapServiceQuotasToModel maps a slice of ServiceQuota to the list model's Items field.
func mapServiceQuotasToModel(ctx context.Context, quotas []models.ServiceQuota, data *serviceQuotasDataSourceModel) (diags diag.Diagnostics) {
	if len(quotas) == 0 {
		emptyList, d := types.ListValueFrom(ctx, datasource_service_quotas.ItemsValue{}.Type(ctx), []datasource_service_quotas.ItemsValue{})
		diags.Append(d...)
		data.Items = emptyList
		return diags
	}

	itemVals := make([]datasource_service_quotas.ItemsValue, 0, len(quotas))
	for _, quota := range quotas {
		val, d := mapServiceQuotaToItemsValue(ctx, quota)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}
		itemVals = append(itemVals, val)
	}

	itemsList, d := types.ListValueFrom(ctx, datasource_service_quotas.ItemsValue{}.Type(ctx), itemVals)
	diags.Append(d...)
	data.Items = itemsList
	return diags
}

// mapServiceQuotaToItemsValue maps a single ServiceQuota to the generated ItemsValue type.
func mapServiceQuotaToItemsValue(ctx context.Context, quota models.ServiceQuota) (datasource_service_quotas.ItemsValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	resourceVal, d := datasource_service_quotas.NewResourceValue(
		datasource_service_quotas.ResourceValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"id":   types.StringValue(quota.Resource.Id),
			"name": types.StringPointerValue(quota.Resource.Name),
			"type": types.StringValue(string(quota.Resource.Type)),
		},
	)
	diags.Append(d...)

	itemVal, d := datasource_service_quotas.NewItemsValue(
		datasource_service_quotas.ItemsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"cloud_provider":      types.StringValue(string(quota.CloudProvider)),
			"limit":               types.Float64Value(quota.Limit),
			"observed_at":         types.StringValue(quota.ObservedAt.UTC().Format(time.RFC3339)),
			"quota":               types.StringValue(quota.Quota),
			"region":              types.StringPointerValue(quota.Region),
			"resource":            resourceVal,
			"service":             types.StringValue(quota.Service),
			"status":              types.StringValue(string(quota.Status)),
			"usage":               types.Float64Value(quota.Usage),
			"utilization_percent": types.Float64Value(quota.UtilizationPercent),
		},
	)
	diags.Append(d...)

	return itemVal, diags
}
