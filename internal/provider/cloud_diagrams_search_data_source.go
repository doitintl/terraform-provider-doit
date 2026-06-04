package provider

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_cloud_diagrams_search"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Compile-time interface checks.
var _ datasource.DataSource = (*cloudDiagramsSearchDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*cloudDiagramsSearchDataSource)(nil)

// NewCloudDiagramsSearchDataSource creates a new instance of the data source.
func NewCloudDiagramsSearchDataSource() datasource.DataSource {
	return &cloudDiagramsSearchDataSource{}
}

// cloudDiagramsSearchDataSource implements datasource.DataSource for cloud diagram search.
type cloudDiagramsSearchDataSource struct {
	client *models.ClientWithResponses
}

// cloudDiagramsSearchDataSourceModel is the Terraform state model.
type cloudDiagramsSearchDataSourceModel struct {
	Id        types.String   `tfsdk:"id"`
	Query     types.String   `tfsdk:"query"`
	SsId      types.String   `tfsdk:"ss_id"`
	From      types.Int64    `tfsdk:"from"`
	Size      types.Int64    `tfsdk:"size"`
	Scheme    types.List     `tfsdk:"scheme"`
	Component types.List     `tfsdk:"component"`
	Prop      types.List     `tfsdk:"prop"`
	Timeouts  timeouts.Value `tfsdk:"timeouts"`
}

func (d *cloudDiagramsSearchDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_diagrams_search"
}

func (d *cloudDiagramsSearchDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	// Start from the generated schema (contains the computed output attributes).
	genSchema := datasource_cloud_diagrams_search.CloudDiagramsSearchDataSourceSchema(ctx)

	// Add the input attributes that the generator cannot produce from the POST
	// request body, and the computed id.
	genSchema.Attributes["id"] = schema.StringAttribute{
		Computed:            true,
		Description:         "A deterministic hash of the query parameters, used as the data source identifier.",
		MarkdownDescription: "A deterministic hash of the query parameters, used as the data source identifier.",
	}
	genSchema.Attributes["query"] = schema.StringAttribute{
		Required:            true,
		Description:         "Search query string.",
		MarkdownDescription: "Search query string.",
	}
	genSchema.Attributes["ss_id"] = schema.StringAttribute{
		Optional:            true,
		Description:         "Limit search to components within this layer.",
		MarkdownDescription: "Limit search to components within this layer.",
	}
	genSchema.Attributes["from"] = schema.Int64Attribute{
		Optional:            true,
		Description:         "Pagination offset (default 0). In auto-pagination mode, sets the starting offset.",
		MarkdownDescription: "Pagination offset (default 0). In auto-pagination mode, sets the starting offset.",
		Validators: []validator.Int64{
			int64validator.AtLeast(0),
		},
	}
	genSchema.Attributes["size"] = schema.Int64Attribute{
		Optional:            true,
		Description:         "Maximum number of results per category. When set, disables auto-pagination and returns a single page.",
		MarkdownDescription: "Maximum number of results per category. When set, disables auto-pagination and returns a single page.",
		Validators: []validator.Int64{
			int64validator.AtLeast(1),
		},
	}

	genSchema.Attributes["timeouts"] = timeouts.Attributes(ctx)

	resp.Schema = genSchema
}

func (d *cloudDiagramsSearchDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

// defaultAutoPageSize is the page size used during auto-pagination.
const defaultAutoPageSize = 100

func (d *cloudDiagramsSearchDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data cloudDiagramsSearchDataSourceModel

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

	// If the config contains any unknown values (e.g., query depends on an
	// unresolved resource during plan), return all computed attributes as unknown.
	if !req.Config.Raw.IsFullyKnown() {
		data.Id = types.StringUnknown()
		data.Scheme = types.ListUnknown(datasource_cloud_diagrams_search.SchemeValue{}.Type(ctx))
		data.Component = types.ListUnknown(datasource_cloud_diagrams_search.ComponentValue{}.Type(ctx))
		data.Prop = types.ListUnknown(datasource_cloud_diagrams_search.PropValue{}.Type(ctx))
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	// Determine pagination mode.
	// Setting `size` switches to manual mode (single API call), matching
	// the `max_results` behavior of other paginated data sources.
	userControlsPagination := !data.Size.IsNull() && !data.Size.IsUnknown()

	var allScheme []models.CloudDiagramSchemeSearchItem
	var allComponent []models.CloudDiagramComponentSearchItem
	var allProp []models.CloudDiagramComponentSearchItem

	if userControlsPagination {
		// Manual pagination: single API call with user-provided values.
		body := d.buildRequestBody(data)
		apiResp, err := d.client.SearchCloudDiagramsWithResponse(ctx, body)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Searching Cloud Diagrams",
				fmt.Sprintf("Unable to search cloud diagrams: %v", err),
			)
			return
		}

		if apiResp.StatusCode() != 200 {
			resp.Diagnostics.AddError(
				"Error Searching Cloud Diagrams",
				fmt.Sprintf("Cloud Diagrams Search API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
			)
			return
		}

		if apiResp.JSON200 == nil {
			resp.Diagnostics.AddError(
				"Error Searching Cloud Diagrams",
				fmt.Sprintf("Cloud Diagrams Search API returned status 200 but response body could not be parsed: %s", string(apiResp.Body)),
			)
			return
		}

		if apiResp.JSON200.Scheme != nil {
			allScheme = *apiResp.JSON200.Scheme
		}
		if apiResp.JSON200.Component != nil {
			allComponent = *apiResp.JSON200.Component
		}
		if apiResp.JSON200.Prop != nil {
			allProp = *apiResp.JSON200.Prop
		}
	} else {
		// Auto-pagination: fetch all pages using the default page size.
		// Honor user-provided `from` as starting offset.
		pageSize := defaultAutoPageSize

		offset := 0
		if !data.From.IsNull() {
			offset = int(data.From.ValueInt64())
		}
		for {
			body := models.SearchCloudDiagramsJSONRequestBody{
				Query: data.Query.ValueString(),
				From:  new(offset),
				Size:  new(pageSize),
			}
			if !data.SsId.IsNull() {
				body.SsId = new(data.SsId.ValueString())
			}

			apiResp, err := d.client.SearchCloudDiagramsWithResponse(ctx, body)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error Searching Cloud Diagrams",
					fmt.Sprintf("Unable to search cloud diagrams (offset %d): %v", offset, err),
				)
				return
			}

			if apiResp.StatusCode() != 200 {
				resp.Diagnostics.AddError(
					"Error Searching Cloud Diagrams",
					fmt.Sprintf("Cloud Diagrams Search API returned status %d (offset %d): %s", apiResp.StatusCode(), offset, string(apiResp.Body)),
				)
				return
			}

			if apiResp.JSON200 == nil {
				resp.Diagnostics.AddError(
					"Error Searching Cloud Diagrams",
					fmt.Sprintf("Cloud Diagrams Search API returned status 200 but response body could not be parsed (offset %d): %s", offset, string(apiResp.Body)),
				)
				return
			}

			var pageScheme []models.CloudDiagramSchemeSearchItem
			var pageComponent []models.CloudDiagramComponentSearchItem
			var pageProp []models.CloudDiagramComponentSearchItem

			if apiResp.JSON200.Scheme != nil {
				pageScheme = *apiResp.JSON200.Scheme
			}
			if apiResp.JSON200.Component != nil {
				pageComponent = *apiResp.JSON200.Component
			}
			if apiResp.JSON200.Prop != nil {
				pageProp = *apiResp.JSON200.Prop
			}

			allScheme = append(allScheme, pageScheme...)
			allComponent = append(allComponent, pageComponent...)
			allProp = append(allProp, pageProp...)

			// Stop when all three categories are exhausted.
			if len(pageScheme) < pageSize && len(pageComponent) < pageSize && len(pageProp) < pageSize {
				break
			}

			offset += pageSize
		}
	}

	// Map API response to Terraform state.
	resp.Diagnostics.Append(d.mapToState(ctx, &data, allScheme, allComponent, allProp)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Set a deterministic ID based on query parameters.
	idInput := data.Query.ValueString()
	if !data.SsId.IsNull() {
		idInput += "\nss_id:" + data.SsId.ValueString()
	}
	if !data.From.IsNull() {
		idInput += fmt.Sprintf("\nfrom:%d", data.From.ValueInt64())
	}
	if !data.Size.IsNull() {
		idInput += fmt.Sprintf("\nsize:%d", data.Size.ValueInt64())
	}
	hash := sha256.Sum256([]byte(idInput))
	data.Id = types.StringValue(fmt.Sprintf("%x", hash))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// buildRequestBody creates the API request body from the Terraform model for manual pagination.
func (d *cloudDiagramsSearchDataSource) buildRequestBody(data cloudDiagramsSearchDataSourceModel) models.SearchCloudDiagramsJSONRequestBody {
	body := models.SearchCloudDiagramsJSONRequestBody{
		Query: data.Query.ValueString(),
	}

	if !data.SsId.IsNull() {
		body.SsId = new(data.SsId.ValueString())
	}

	if !data.From.IsNull() {
		body.From = new(int(data.From.ValueInt64()))
	}

	if !data.Size.IsNull() {
		body.Size = new(int(data.Size.ValueInt64()))
	}

	return body
}

// mapToState maps the API response slices to the Terraform state model.
func (d *cloudDiagramsSearchDataSource) mapToState(
	ctx context.Context,
	data *cloudDiagramsSearchDataSourceModel,
	schemes []models.CloudDiagramSchemeSearchItem,
	components []models.CloudDiagramComponentSearchItem,
	props []models.CloudDiagramComponentSearchItem,
) diag.Diagnostics {
	var diags diag.Diagnostics

	// Map scheme results.
	schemeVals := make([]datasource_cloud_diagrams_search.SchemeValue, 0, len(schemes))
	for _, s := range schemes {
		val, valDiags := datasource_cloud_diagrams_search.NewSchemeValue(
			datasource_cloud_diagrams_search.SchemeValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"_id":          types.StringValue(s.UnderscoreId),
				"account_name": types.StringPointerValue(s.AccountName),
				"name":         types.StringPointerValue(s.Name),
				"scheme":       types.StringPointerValue(s.Scheme),
				"scheme_id":    types.StringPointerValue(s.SchemeId),
				"ss_id":        types.StringPointerValue(s.SsId),
				"status":       types.StringPointerValue(s.Status),
				"type":         types.StringValue(string(s.Type)),
			},
		)
		diags.Append(valDiags...)
		if diags.HasError() {
			return diags
		}
		schemeVals = append(schemeVals, val)
	}

	schemeList, listDiags := types.ListValueFrom(ctx, datasource_cloud_diagrams_search.SchemeValue{}.Type(ctx), schemeVals)
	diags.Append(listDiags...)
	if diags.HasError() {
		return diags
	}
	data.Scheme = schemeList

	// Map component results.
	componentVals, mapDiags := mapComponentSearchItems(ctx, components)
	diags.Append(mapDiags...)
	if diags.HasError() {
		return diags
	}

	componentList, listDiags := types.ListValueFrom(ctx, datasource_cloud_diagrams_search.ComponentValue{}.Type(ctx), componentVals)
	diags.Append(listDiags...)
	if diags.HasError() {
		return diags
	}
	data.Component = componentList

	// Map prop results (same API type, different generated value type).
	propVals, mapDiags := mapPropSearchItems(ctx, props)
	diags.Append(mapDiags...)
	if diags.HasError() {
		return diags
	}

	propList, listDiags := types.ListValueFrom(ctx, datasource_cloud_diagrams_search.PropValue{}.Type(ctx), propVals)
	diags.Append(listDiags...)
	if diags.HasError() {
		return diags
	}
	data.Prop = propList

	return diags
}

// mapPropsValue maps the nested props object from an API component search item.
func mapPropsValue(ctx context.Context, apiProps *models.CloudDiagramComponentSearchItemProps) (datasource_cloud_diagrams_search.PropsValue, diag.Diagnostics) {
	if apiProps == nil {
		return datasource_cloud_diagrams_search.NewPropsValueNull(), nil
	}

	return datasource_cloud_diagrams_search.NewPropsValue(
		datasource_cloud_diagrams_search.PropsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"service_type": types.StringPointerValue(apiProps.ServiceType),
		},
	)
}

// mapNodeType converts an optional enum pointer to a Terraform string value.
func mapNodeType(nt *models.CloudDiagramComponentSearchItemNodeType) types.String {
	if nt != nil {
		return types.StringValue(string(*nt))
	}
	return types.StringNull()
}

// mapComponentSearchItems maps a slice of API component search items to ComponentValue types.
func mapComponentSearchItems(
	ctx context.Context,
	items []models.CloudDiagramComponentSearchItem,
) ([]datasource_cloud_diagrams_search.ComponentValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	vals := make([]datasource_cloud_diagrams_search.ComponentValue, 0, len(items))
	for _, c := range items {
		propsVal, propsDiags := mapPropsValue(ctx, c.Props)
		diags.Append(propsDiags...)
		if diags.HasError() {
			return nil, diags
		}

		val, valDiags := datasource_cloud_diagrams_search.NewComponentValue(
			datasource_cloud_diagrams_search.ComponentValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"_id":          types.StringValue(c.UnderscoreId),
				"account_name": types.StringPointerValue(c.AccountName),
				"color":        types.StringPointerValue(c.Color),
				"group_type":   types.StringPointerValue(c.GroupType),
				"icon":         types.StringPointerValue(c.Icon),
				"name":         types.StringPointerValue(c.Name),
				"node_type":    mapNodeType(c.NodeType),
				"props":        propsVal,
				"scheme_id":    types.StringPointerValue(c.SchemeId),
				"ss_id":        types.StringPointerValue(c.SsId),
				"type":         types.StringValue(string(c.Type)),
			},
		)
		diags.Append(valDiags...)
		if diags.HasError() {
			return nil, diags
		}
		vals = append(vals, val)
	}

	return vals, diags
}

// mapPropSearchItems maps a slice of API component search items to PropValue types.
// The prop category uses the same API type but a different generated TF value type.
func mapPropSearchItems(
	ctx context.Context,
	items []models.CloudDiagramComponentSearchItem,
) ([]datasource_cloud_diagrams_search.PropValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	vals := make([]datasource_cloud_diagrams_search.PropValue, 0, len(items))
	for _, c := range items {
		propsVal, propsDiags := mapPropsValue(ctx, c.Props)
		diags.Append(propsDiags...)
		if diags.HasError() {
			return nil, diags
		}

		val, valDiags := datasource_cloud_diagrams_search.NewPropValue(
			datasource_cloud_diagrams_search.PropValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"_id":          types.StringValue(c.UnderscoreId),
				"account_name": types.StringPointerValue(c.AccountName),
				"color":        types.StringPointerValue(c.Color),
				"group_type":   types.StringPointerValue(c.GroupType),
				"icon":         types.StringPointerValue(c.Icon),
				"name":         types.StringPointerValue(c.Name),
				"node_type":    mapNodeType(c.NodeType),
				"props":        propsVal,
				"scheme_id":    types.StringPointerValue(c.SchemeId),
				"ss_id":        types.StringPointerValue(c.SsId),
				"type":         types.StringValue(string(c.Type)),
			},
		)
		diags.Append(valDiags...)
		if diags.HasError() {
			return nil, diags
		}
		vals = append(vals, val)
	}

	return vals, diags
}
