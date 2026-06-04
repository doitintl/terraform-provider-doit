package provider

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math/big"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_cloud_diagrams_schemes"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// Compile-time interface checks.
var _ datasource.DataSource = (*cloudDiagramsSchemesDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*cloudDiagramsSchemesDataSource)(nil)

// NewCloudDiagramsSchemesDataSource creates a new instance of the data source.
func NewCloudDiagramsSchemesDataSource() datasource.DataSource {
	return &cloudDiagramsSchemesDataSource{}
}

// cloudDiagramsSchemesDataSource implements datasource.DataSource for cloud diagram schemes.
type cloudDiagramsSchemesDataSource struct {
	client *models.ClientWithResponses
}

// cloudDiagramsSchemesDataSourceModel is the Terraform state model.
// It wraps the generated model and adds fields the generator cannot produce.
type cloudDiagramsSchemesDataSourceModel struct {
	datasource_cloud_diagrams_schemes.CloudDiagramsSchemesModel
	Id        types.String   `tfsdk:"id"`
	SchemeIds types.List     `tfsdk:"scheme_ids"`
	LayerIds  types.List     `tfsdk:"layer_ids"`
	Timeouts  timeouts.Value `tfsdk:"timeouts"`
}

func (d *cloudDiagramsSchemesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_diagrams_schemes"
}

func (d *cloudDiagramsSchemesDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	genSchema := datasource_cloud_diagrams_schemes.CloudDiagramsSchemesDataSourceSchema(ctx)

	// Computed ID.
	genSchema.Attributes["id"] = schema.StringAttribute{
		Computed:            true,
		Description:         "A deterministic hash of the query parameters, used as the data source identifier.",
		MarkdownDescription: "A deterministic hash of the query parameters, used as the data source identifier.",
	}

	// Request body fields (the generator cannot infer these from the POST body).
	genSchema.Attributes["scheme_ids"] = schema.ListAttribute{
		ElementType:         types.StringType,
		Optional:            true,
		Description:         "IDs of diagrams to load. When omitted, returns all accessible diagrams.",
		MarkdownDescription: "IDs of diagrams to load. When omitted, returns all accessible diagrams.",
	}
	genSchema.Attributes["layer_ids"] = schema.ListAttribute{
		ElementType:         types.StringType,
		Optional:            true,
		Description:         "IDs of layers to load. When omitted, returns all layers for the selected diagrams.",
		MarkdownDescription: "IDs of layers to load. When omitted, returns all layers for the selected diagrams.",
	}

	genSchema.Attributes["timeouts"] = timeouts.Attributes(ctx)

	genSchema.Description = "Retrieves Cloud Diagram data including diagrams, layers, and components."
	genSchema.MarkdownDescription = "Retrieves Cloud Diagram data including diagrams, layers, and components."

	resp.Schema = genSchema
}

func (d *cloudDiagramsSchemesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *cloudDiagramsSchemesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data cloudDiagramsSchemesDataSourceModel

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

	// If the config contains any unknown values, return all computed attributes as unknown.
	if !req.Config.Raw.IsFullyKnown() {
		data.Id = types.StringUnknown()
		data.Scheme = types.MapUnknown(datasource_cloud_diagrams_schemes.SchemeValue{}.Type(ctx))
		data.Statussheet = types.MapUnknown(datasource_cloud_diagrams_schemes.StatussheetValue{}.Type(ctx))
		data.Template = datasource_cloud_diagrams_schemes.NewTemplateValueUnknown()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	// Build query params.
	params := &models.GetCloudDiagramComponentsParams{}
	if !data.Components.IsNull() {
		params.Components = data.Components.ValueBoolPointer()
	}
	if !data.External.IsNull() {
		params.External = data.External.ValueBoolPointer()
	}
	if !data.Element.IsNull() {
		params.Element = data.Element.ValueBoolPointer()
	}
	if !data.Link.IsNull() {
		params.Link = data.Link.ValueBoolPointer()
	}
	if !data.Group.IsNull() {
		params.Group = data.Group.ValueBoolPointer()
	}
	if !data.Note.IsNull() {
		params.Note = data.Note.ValueBoolPointer()
	}
	if !data.Combiner.IsNull() {
		params.Combiner = data.Combiner.ValueBoolPointer()
	}
	if !data.SkipEmpty.IsNull() {
		params.SkipEmpty = data.SkipEmpty.ValueBoolPointer()
	}
	if !data.ExcludeDefaultVpc.IsNull() {
		params.ExcludeDefaultVpc = data.ExcludeDefaultVpc.ValueBoolPointer()
	}
	if !data.ExcludeEmptySubnets.IsNull() {
		params.ExcludeEmptySubnets = data.ExcludeEmptySubnets.ValueBoolPointer()
	}
	if !data.AlarmsCount.IsNull() {
		params.AlarmsCount = data.AlarmsCount.ValueBoolPointer()
	}
	if !data.IssuesCount.IsNull() {
		params.IssuesCount = data.IssuesCount.ValueBoolPointer()
	}
	if !data.NodeType.IsNull() {
		params.NodeType = new(models.GetCloudDiagramComponentsParamsNodeType(data.NodeType.ValueString()))
	}
	if !data.Type.IsNull() {
		var typeStrs []string
		resp.Diagnostics.Append(data.Type.ElementsAs(ctx, &typeStrs, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		typeVals := make([]models.GetCloudDiagramComponentsParamsType, 0, len(typeStrs))
		for _, ts := range typeStrs {
			typeVals = append(typeVals, models.GetCloudDiagramComponentsParamsType(ts))
		}
		params.Type = &typeVals
	}

	// Build request body.
	body := models.CloudDiagramsGetRequest{}
	if !data.SchemeIds.IsNull() {
		var ids []string
		resp.Diagnostics.Append(data.SchemeIds.ElementsAs(ctx, &ids, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		body.Scheme = &ids
	}
	if !data.LayerIds.IsNull() {
		var ids []string
		resp.Diagnostics.Append(data.LayerIds.ElementsAs(ctx, &ids, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		body.Statussheet = &ids
	}

	// Make API call.
	apiResp, err := d.client.GetCloudDiagramComponentsWithResponse(ctx, params, body)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Cloud Diagram Schemes",
			fmt.Sprintf("Unable to read cloud diagram schemes: %v", err),
		)
		return
	}

	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Cloud Diagram Schemes",
			fmt.Sprintf("Cloud Diagrams API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	// Map response to state.
	resp.Diagnostics.Append(mapSchemesResponse(ctx, &data, apiResp.JSON200)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Set deterministic ID.
	data.Id = types.StringValue(computeSchemesID(data))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// computeSchemesID creates a deterministic hash from query parameters.
func computeSchemesID(data cloudDiagramsSchemesDataSourceModel) string {
	input := "cloud_diagrams_schemes"
	if !data.SchemeIds.IsNull() {
		input += fmt.Sprintf("\nscheme_ids:%s", data.SchemeIds.String())
	}
	if !data.LayerIds.IsNull() {
		input += fmt.Sprintf("\nlayer_ids:%s", data.LayerIds.String())
	}

	// Include all boolean query params in the hash.
	for _, p := range []struct {
		name string
		val  types.Bool
	}{
		{"alarms_count", data.AlarmsCount},
		{"combiner", data.Combiner},
		{"components", data.Components},
		{"element", data.Element},
		{"exclude_default_vpc", data.ExcludeDefaultVpc},
		{"exclude_empty_subnets", data.ExcludeEmptySubnets},
		{"external", data.External},
		{"group", data.Group},
		{"issues_count", data.IssuesCount},
		{"link", data.Link},
		{"note", data.Note},
		{"skip_empty", data.SkipEmpty},
	} {
		if !p.val.IsNull() {
			input += fmt.Sprintf("\n%s:%v", p.name, p.val.ValueBool())
		}
	}

	if !data.NodeType.IsNull() {
		input += fmt.Sprintf("\nnode_type:%s", data.NodeType.ValueString())
	}
	if !data.Type.IsNull() {
		input += fmt.Sprintf("\ntype:%s", data.Type.String())
	}
	hash := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", hash)
}

// mapSchemesResponse maps the full API response to the Terraform state model.
func mapSchemesResponse(
	ctx context.Context,
	data *cloudDiagramsSchemesDataSourceModel,
	apiResp *models.CloudDiagramsGetResponse,
) diag.Diagnostics {
	var diags diag.Diagnostics

	// Map scheme map.
	diags.Append(mapSchemeMap(ctx, data, apiResp.Scheme)...)
	if diags.HasError() {
		return diags
	}

	// Map statussheet map.
	diags.Append(mapStatussheetMap(ctx, data, apiResp.Statussheet)...)
	if diags.HasError() {
		return diags
	}

	// Template is a free-form map; set to null if absent.
	data.Template = datasource_cloud_diagrams_schemes.NewTemplateValueNull()

	return diags
}

// mapSchemeMap maps the scheme map from the API response.
func mapSchemeMap(
	ctx context.Context,
	data *cloudDiagramsSchemesDataSourceModel,
	scheme *map[string]models.CloudDiagramSchemeResult,
) diag.Diagnostics {
	var diags diag.Diagnostics

	if scheme == nil || len(*scheme) == 0 {
		data.Scheme = types.MapNull(datasource_cloud_diagrams_schemes.SchemeValue{}.Type(ctx))
		return diags
	}

	schemeVals := make(map[string]datasource_cloud_diagrams_schemes.SchemeValue, len(*scheme))
	for key, sr := range *scheme {
		// Map statussheet list.
		ssVals := make([]datasource_cloud_diagrams_schemes.SchemeStatussheetValue, 0, len(sr.Statussheet))
		for _, ss := range sr.Statussheet {
			ssVal, ssDiags := datasource_cloud_diagrams_schemes.NewSchemeStatussheetValue(
				datasource_cloud_diagrams_schemes.SchemeStatussheetValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"account_name": types.StringValue(ss.AccountName),
					"alarms_count": types.Int64Value(int64(ss.AlarmsCount)),
					"color":        types.StringValue(ss.Color),
					"empty":        types.BoolValue(ss.Empty),
					"name":         types.StringValue(ss.Name),
					"ssid":         types.StringValue(ss.Ssid),
				},
			)
			diags.Append(ssDiags...)
			if diags.HasError() {
				return diags
			}
			ssVals = append(ssVals, ssVal)
		}

		ssList, listDiags := types.ListValueFrom(ctx,
			datasource_cloud_diagrams_schemes.SchemeStatussheetValue{}.Type(ctx),
			ssVals,
		)
		diags.Append(listDiags...)
		if diags.HasError() {
			return diags
		}

		val, valDiags := datasource_cloud_diagrams_schemes.NewSchemeValue(
			datasource_cloud_diagrams_schemes.SchemeValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"_id":         types.StringValue(sr.UnderscoreId),
				"color":       types.StringPointerValue(sr.Color),
				"name":        types.StringValue(sr.Name),
				"statussheet": ssList,
				"type":        types.StringValue(string(sr.Type)),
			},
		)
		diags.Append(valDiags...)
		if diags.HasError() {
			return diags
		}
		schemeVals[key] = val
	}

	schemeMap, mapDiags := types.MapValueFrom(ctx,
		datasource_cloud_diagrams_schemes.SchemeValue{}.Type(ctx),
		schemeVals,
	)
	diags.Append(mapDiags...)
	if diags.HasError() {
		return diags
	}
	data.Scheme = schemeMap

	return diags
}

// mapStatussheetMap maps the statussheet (layer) map from the API response.
func mapStatussheetMap(
	ctx context.Context,
	data *cloudDiagramsSchemesDataSourceModel,
	statussheet *map[string]models.CloudDiagramStatussheetData,
) diag.Diagnostics {
	var diags diag.Diagnostics

	if statussheet == nil || len(*statussheet) == 0 {
		data.Statussheet = types.MapNull(datasource_cloud_diagrams_schemes.StatussheetValue{}.Type(ctx))
		return diags
	}

	ssVals := make(map[string]datasource_cloud_diagrams_schemes.StatussheetValue, len(*statussheet))
	for key, ssData := range *statussheet {
		val, valDiags := mapStatussheetEntry(ctx, &ssData)
		diags.Append(valDiags...)
		if diags.HasError() {
			return diags
		}
		ssVals[key] = val
	}

	ssMap, mapDiags := types.MapValueFrom(ctx,
		datasource_cloud_diagrams_schemes.StatussheetValue{}.Type(ctx),
		ssVals,
	)
	diags.Append(mapDiags...)
	if diags.HasError() {
		return diags
	}
	data.Statussheet = ssMap

	return diags
}

// mapStatussheetEntry maps a single statussheet (layer) entry.
func mapStatussheetEntry(
	ctx context.Context,
	ssData *models.CloudDiagramStatussheetData,
) (datasource_cloud_diagrams_schemes.StatussheetValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	attrVals := map[string]attr.Value{}

	// Map statussheet metadata.
	importVal := datasource_cloud_diagrams_schemes.NewImportValueNull()
	if ssData.Statussheet.Import != nil {
		imp := ssData.Statussheet.Import
		var syncedAt basetypes.StringValue
		if imp.SyncedAt != nil {
			syncedAt = types.StringValue(imp.SyncedAt.Format(time.RFC3339))
		} else {
			syncedAt = types.StringNull()
		}

		var importDiags diag.Diagnostics
		importVal, importDiags = datasource_cloud_diagrams_schemes.NewImportValue(
			datasource_cloud_diagrams_schemes.ImportValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"account":       types.StringPointerValue(imp.Account),
				"cloud_id":      types.StringPointerValue(imp.CloudId),
				"error_message": types.StringPointerValue(imp.ErrorMessage),
				"status":        mapEnumPointerValue(imp.Status),
				"synced_at":     syncedAt,
				"type":          mapEnumPointerValue(imp.Type),
			},
		)
		diags.Append(importDiags...)
		if diags.HasError() {
			return datasource_cloud_diagrams_schemes.StatussheetValue{}, diags
		}
	}

	metaVal, metaDiags := datasource_cloud_diagrams_schemes.NewStatussheetStatussheetValue(
		datasource_cloud_diagrams_schemes.StatussheetStatussheetValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"_id":           types.StringValue(ssData.Statussheet.UnderscoreId),
			"import":        importVal,
			"links_version": types.Int64PointerValue(intPtrToInt64Ptr(ssData.Statussheet.LinksVersion)),
			"updated_at":    types.StringValue(ssData.Statussheet.UpdatedAt.Format(time.RFC3339)),
		},
	)
	diags.Append(metaDiags...)
	if diags.HasError() {
		return datasource_cloud_diagrams_schemes.StatussheetValue{}, diags
	}
	attrVals["statussheet"] = metaVal

	// Map component maps (node, element, group, link, attachment, combiner, note).
	nodeMap, nodeDiags := mapNodeMap(ctx, ssData.Node)
	diags.Append(nodeDiags...)
	if diags.HasError() {
		return datasource_cloud_diagrams_schemes.StatussheetValue{}, diags
	}
	attrVals["node"] = nodeMap

	elementMap, elementDiags := mapElementMap(ctx, ssData.Element)
	diags.Append(elementDiags...)
	if diags.HasError() {
		return datasource_cloud_diagrams_schemes.StatussheetValue{}, diags
	}
	attrVals["element"] = elementMap

	groupMap, groupDiags := mapGroupMap(ctx, ssData.Group)
	diags.Append(groupDiags...)
	if diags.HasError() {
		return datasource_cloud_diagrams_schemes.StatussheetValue{}, diags
	}
	attrVals["group"] = groupMap

	linkMap, linkDiags := mapLinkMap(ctx, ssData.Link)
	diags.Append(linkDiags...)
	if diags.HasError() {
		return datasource_cloud_diagrams_schemes.StatussheetValue{}, diags
	}
	attrVals["link"] = linkMap

	attachmentMap, attachmentDiags := mapAttachmentMap(ctx, ssData.Attachment)
	diags.Append(attachmentDiags...)
	if diags.HasError() {
		return datasource_cloud_diagrams_schemes.StatussheetValue{}, diags
	}
	attrVals["attachment"] = attachmentMap

	combinerMap, combinerDiags := mapCombinerMap(ctx, ssData.Combiner)
	diags.Append(combinerDiags...)
	if diags.HasError() {
		return datasource_cloud_diagrams_schemes.StatussheetValue{}, diags
	}
	attrVals["combiner"] = combinerMap

	noteMap, noteDiags := mapNoteMap(ctx, ssData.Note)
	diags.Append(noteDiags...)
	if diags.HasError() {
		return datasource_cloud_diagrams_schemes.StatussheetValue{}, diags
	}
	attrVals["note"] = noteMap

	val, valDiags := datasource_cloud_diagrams_schemes.NewStatussheetValue(
		datasource_cloud_diagrams_schemes.StatussheetValue{}.AttributeTypes(ctx),
		attrVals,
	)
	diags.Append(valDiags...)
	return val, diags
}

// --- Component map helpers ---

// mapIssuesList maps a slice of CloudDiagramIssue to a Terraform list value.
func mapIssuesList(ctx context.Context, issues *[]models.CloudDiagramIssue) (basetypes.ListValue, diag.Diagnostics) {
	issueType := datasource_cloud_diagrams_schemes.IssuesValue{}.Type(ctx)

	if issues == nil || len(*issues) == 0 {
		return types.ListValueFrom(ctx, issueType, []datasource_cloud_diagrams_schemes.IssuesValue{})
	}

	var diags diag.Diagnostics
	vals := make([]datasource_cloud_diagrams_schemes.IssuesValue, 0, len(*issues))
	for _, issue := range *issues {
		val, valDiags := datasource_cloud_diagrams_schemes.NewIssuesValue(
			datasource_cloud_diagrams_schemes.IssuesValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"_id":     types.StringPointerValue(issue.UnderscoreId),
				"comment": types.StringPointerValue(issue.Comment),
				"jira":    types.StringPointerValue(issue.Jira),
				"snoozed": float32PtrToBigFloat(issue.Snoozed),
			},
		)
		diags.Append(valDiags...)
		if diags.HasError() {
			return basetypes.ListValue{}, diags
		}
		vals = append(vals, val)
	}

	list, listDiags := types.ListValueFrom(ctx, issueType, vals)
	diags.Append(listDiags...)
	return list, diags
}

// mapTagsList maps a string slice pointer to a Terraform list value.
func mapTagsList(ctx context.Context, tags *[]string) (basetypes.ListValue, diag.Diagnostics) {
	if tags == nil || len(*tags) == 0 {
		return types.ListValueFrom(ctx, types.StringType, []string{})
	}
	return types.ListValueFrom(ctx, types.StringType, *tags)
}

// mapPropsValue maps the free-form props map to a Terraform PropsValue (always empty object).
func mapSchemesPropsValue(ctx context.Context) datasource_cloud_diagrams_schemes.PropsValue {
	// Props is defined as additionalProperties: true in the spec, which produces
	// an empty SingleNestedAttribute. We return an empty known value.
	return datasource_cloud_diagrams_schemes.NewPropsValueMust(
		datasource_cloud_diagrams_schemes.PropsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{},
	)
}

// --- Node ---

func mapNodeMap(ctx context.Context, nodes *map[string]models.CloudDiagramNode) (basetypes.MapValue, diag.Diagnostics) {
	nodeType := datasource_cloud_diagrams_schemes.NodeValue{}.Type(ctx)

	if nodes == nil || len(*nodes) == 0 {
		return types.MapNull(nodeType), nil
	}

	var diags diag.Diagnostics
	vals := make(map[string]datasource_cloud_diagrams_schemes.NodeValue, len(*nodes))
	for key, n := range *nodes {
		issues, issDiags := mapIssuesList(ctx, n.Issues)
		diags.Append(issDiags...)
		if diags.HasError() {
			return basetypes.MapValue{}, diags
		}

		tags, tagsDiags := mapTagsList(ctx, n.Tags)
		diags.Append(tagsDiags...)
		if diags.HasError() {
			return basetypes.MapValue{}, diags
		}

		infraNode := datasource_cloud_diagrams_schemes.NewInfraNodeValueNull()
		if n.InfraNode != nil {
			var infraDiags diag.Diagnostics
			infraNode, infraDiags = datasource_cloud_diagrams_schemes.NewInfraNodeValue(
				datasource_cloud_diagrams_schemes.InfraNodeValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"_id":       types.StringValue(n.InfraNode.UnderscoreId),
					"scheme_id": types.StringValue(n.InfraNode.SchemeId),
					"ss_id":     types.StringValue(n.InfraNode.SsId),
				},
			)
			diags.Append(infraDiags...)
			if diags.HasError() {
				return basetypes.MapValue{}, diags
			}
		}

		val, valDiags := datasource_cloud_diagrams_schemes.NewNodeValue(
			datasource_cloud_diagrams_schemes.NodeValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"_id":            types.StringValue(n.UnderscoreId),
				"cld_account":    types.StringPointerValue(n.CldAccount),
				"cld_id":         types.StringPointerValue(n.CldId),
				"cld_sync":       types.BoolPointerValue(n.CldSync),
				"cld_type":       mapEnumPointerValue(n.CldType),
				"color":          types.StringPointerValue(n.Color),
				"icon":           types.StringPointerValue(n.Icon),
				"infra_node":     infraNode,
				"instance_count": types.Int64PointerValue(intPtrToInt64Ptr(n.InstanceCount)),
				"issues":         issues,
				"name":           types.StringPointerValue(n.Name),
				"parent":         types.StringPointerValue(n.Parent),
				"props":          mapSchemesPropsValue(ctx),
				"running":        types.BoolPointerValue(n.Running),
				"tags":           tags,
			},
		)
		diags.Append(valDiags...)
		if diags.HasError() {
			return basetypes.MapValue{}, diags
		}
		vals[key] = val
	}

	m, mapDiags := types.MapValueFrom(ctx, nodeType, vals)
	diags.Append(mapDiags...)
	return m, diags
}

// --- Element ---

func mapElementMap(ctx context.Context, elements *map[string]models.CloudDiagramElement) (basetypes.MapValue, diag.Diagnostics) {
	elemType := datasource_cloud_diagrams_schemes.ElementValue{}.Type(ctx)

	if elements == nil || len(*elements) == 0 {
		return types.MapNull(elemType), nil
	}

	var diags diag.Diagnostics
	vals := make(map[string]datasource_cloud_diagrams_schemes.ElementValue, len(*elements))
	for key, e := range *elements {
		issues, issDiags := mapIssuesList(ctx, e.Issues)
		diags.Append(issDiags...)
		if diags.HasError() {
			return basetypes.MapValue{}, diags
		}

		tags, tagsDiags := mapTagsList(ctx, e.Tags)
		diags.Append(tagsDiags...)
		if diags.HasError() {
			return basetypes.MapValue{}, diags
		}

		val, valDiags := datasource_cloud_diagrams_schemes.NewElementValue(
			datasource_cloud_diagrams_schemes.ElementValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"_id":         types.StringValue(e.UnderscoreId),
				"cld_account": types.StringPointerValue(e.CldAccount),
				"cld_id":      types.StringPointerValue(e.CldId),
				"cld_sync":    types.BoolPointerValue(e.CldSync),
				"cld_type":    mapEnumPointerValue(e.CldType),
				"color":       types.StringPointerValue(e.Color),
				"icon":        types.StringPointerValue(e.Icon),
				"issues":      issues,
				"name":        types.StringPointerValue(e.Name),
				"parent":      types.StringPointerValue(e.Parent),
				"props":       mapSchemesPropsValue(ctx),
				"tags":        tags,
			},
		)
		diags.Append(valDiags...)
		if diags.HasError() {
			return basetypes.MapValue{}, diags
		}
		vals[key] = val
	}

	m, mapDiags := types.MapValueFrom(ctx, elemType, vals)
	diags.Append(mapDiags...)
	return m, diags
}

// --- Group ---

func mapGroupMap(ctx context.Context, groups *map[string]models.CloudDiagramGroup) (basetypes.MapValue, diag.Diagnostics) {
	groupType := datasource_cloud_diagrams_schemes.GroupValue{}.Type(ctx)

	if groups == nil || len(*groups) == 0 {
		return types.MapNull(groupType), nil
	}

	var diags diag.Diagnostics
	vals := make(map[string]datasource_cloud_diagrams_schemes.GroupValue, len(*groups))
	for key, g := range *groups {
		issues, issDiags := mapIssuesList(ctx, g.Issues)
		diags.Append(issDiags...)
		if diags.HasError() {
			return basetypes.MapValue{}, diags
		}

		items, itemsDiags := mapGroupItemsList(ctx, g.Items)
		diags.Append(itemsDiags...)
		if diags.HasError() {
			return basetypes.MapValue{}, diags
		}

		tags, tagsDiags := mapTagsList(ctx, g.Tags)
		diags.Append(tagsDiags...)
		if diags.HasError() {
			return basetypes.MapValue{}, diags
		}

		val, valDiags := datasource_cloud_diagrams_schemes.NewGroupValue(
			datasource_cloud_diagrams_schemes.GroupValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"_id":         types.StringValue(g.UnderscoreId),
				"cld_account": types.StringPointerValue(g.CldAccount),
				"cld_id":      types.StringPointerValue(g.CldId),
				"cld_sync":    types.BoolPointerValue(g.CldSync),
				"cld_type":    mapEnumPointerValue(g.CldType),
				"color":       types.StringPointerValue(g.Color),
				"group_type":  mapEnumPointerValue(g.GroupType),
				"icon":        types.StringPointerValue(g.Icon),
				"issues":      issues,
				"items":       items,
				"name":        types.StringPointerValue(g.Name),
				"props":       mapSchemesPropsValue(ctx),
				"tags":        tags,
			},
		)
		diags.Append(valDiags...)
		if diags.HasError() {
			return basetypes.MapValue{}, diags
		}
		vals[key] = val
	}

	m, mapDiags := types.MapValueFrom(ctx, groupType, vals)
	diags.Append(mapDiags...)
	return m, diags
}

// mapGroupItemsList maps group items. Groups use CloudDiagramGroupItem (same shape as CombinerItem).
func mapGroupItemsList(ctx context.Context, items *[]models.CloudDiagramGroupItem) (basetypes.ListValue, diag.Diagnostics) {
	itemType := datasource_cloud_diagrams_schemes.ItemsValue{}.Type(ctx)

	if items == nil || len(*items) == 0 {
		return types.ListValueFrom(ctx, itemType, []datasource_cloud_diagrams_schemes.ItemsValue{})
	}

	var diags diag.Diagnostics
	vals := make([]datasource_cloud_diagrams_schemes.ItemsValue, 0, len(*items))
	for _, item := range *items {
		val, valDiags := datasource_cloud_diagrams_schemes.NewItemsValue(
			datasource_cloud_diagrams_schemes.ItemsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"_id":  types.StringValue(item.UnderscoreId),
				"type": types.StringValue(string(item.Type)),
			},
		)
		diags.Append(valDiags...)
		if diags.HasError() {
			return basetypes.ListValue{}, diags
		}
		vals = append(vals, val)
	}

	list, listDiags := types.ListValueFrom(ctx, itemType, vals)
	diags.Append(listDiags...)
	return list, diags
}

// --- Link ---

func mapLinkMap(ctx context.Context, links *map[string]models.CloudDiagramLink) (basetypes.MapValue, diag.Diagnostics) {
	linkType := datasource_cloud_diagrams_schemes.LinkValue{}.Type(ctx)

	if links == nil || len(*links) == 0 {
		return types.MapNull(linkType), nil
	}

	var diags diag.Diagnostics
	vals := make(map[string]datasource_cloud_diagrams_schemes.LinkValue, len(*links))
	for key, l := range *links {
		issues, issDiags := mapIssuesList(ctx, l.Issues)
		diags.Append(issDiags...)
		if diags.HasError() {
			return basetypes.MapValue{}, diags
		}

		tags, tagsDiags := mapTagsList(ctx, l.Tags)
		diags.Append(tagsDiags...)
		if diags.HasError() {
			return basetypes.MapValue{}, diags
		}

		val, valDiags := datasource_cloud_diagrams_schemes.NewLinkValue(
			datasource_cloud_diagrams_schemes.LinkValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"_id":             types.StringValue(l.UnderscoreId),
				"cld_account":     types.StringPointerValue(l.CldAccount),
				"cld_id":          types.StringPointerValue(l.CldId),
				"cld_sync":        types.BoolPointerValue(l.CldSync),
				"cld_type":        mapEnumPointerValue(l.CldType),
				"connection_type": mapEnumPointerValue(l.ConnectionType),
				"issues":          issues,
				"name":            types.StringPointerValue(l.Name),
				"owner_ss_id":     types.StringPointerValue(l.OwnerSsId),
				"props":           mapSchemesPropsValue(ctx),
				"tags":            tags,
			},
		)
		diags.Append(valDiags...)
		if diags.HasError() {
			return basetypes.MapValue{}, diags
		}
		vals[key] = val
	}

	m, mapDiags := types.MapValueFrom(ctx, linkType, vals)
	diags.Append(mapDiags...)
	return m, diags
}

// --- Attachment ---

func mapAttachmentMap(ctx context.Context, attachments *map[string]models.CloudDiagramAttachment) (basetypes.MapValue, diag.Diagnostics) {
	attType := datasource_cloud_diagrams_schemes.AttachmentValue{}.Type(ctx)

	if attachments == nil || len(*attachments) == 0 {
		return types.MapNull(attType), nil
	}

	var diags diag.Diagnostics
	vals := make(map[string]datasource_cloud_diagrams_schemes.AttachmentValue, len(*attachments))
	for key, a := range *attachments {
		issues, issDiags := mapIssuesList(ctx, a.Issues)
		diags.Append(issDiags...)
		if diags.HasError() {
			return basetypes.MapValue{}, diags
		}

		tags, tagsDiags := mapTagsList(ctx, a.Tags)
		diags.Append(tagsDiags...)
		if diags.HasError() {
			return basetypes.MapValue{}, diags
		}

		val, valDiags := datasource_cloud_diagrams_schemes.NewAttachmentValue(
			datasource_cloud_diagrams_schemes.AttachmentValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"_id":         types.StringValue(a.UnderscoreId),
				"cld_account": types.StringPointerValue(a.CldAccount),
				"cld_id":      types.StringPointerValue(a.CldId),
				"cld_sync":    types.BoolPointerValue(a.CldSync),
				"cld_type":    mapEnumPointerValue(a.CldType),
				"color":       types.StringPointerValue(a.Color),
				"group":       types.StringPointerValue(a.Group),
				"icon":        types.StringPointerValue(a.Icon),
				"issues":      issues,
				"name":        types.StringPointerValue(a.Name),
				"props":       mapSchemesPropsValue(ctx),
				"tags":        tags,
			},
		)
		diags.Append(valDiags...)
		if diags.HasError() {
			return basetypes.MapValue{}, diags
		}
		vals[key] = val
	}

	m, mapDiags := types.MapValueFrom(ctx, attType, vals)
	diags.Append(mapDiags...)
	return m, diags
}

// --- Combiner ---

func mapCombinerMap(ctx context.Context, combiners *map[string]models.CloudDiagramCombiner) (basetypes.MapValue, diag.Diagnostics) {
	cmbType := datasource_cloud_diagrams_schemes.CombinerValue{}.Type(ctx)

	if combiners == nil || len(*combiners) == 0 {
		return types.MapNull(cmbType), nil
	}

	var diags diag.Diagnostics
	vals := make(map[string]datasource_cloud_diagrams_schemes.CombinerValue, len(*combiners))
	for key, c := range *combiners {
		items, itemsDiags := mapCombinerItemsList(ctx, c.Items)
		diags.Append(itemsDiags...)
		if diags.HasError() {
			return basetypes.MapValue{}, diags
		}

		val, valDiags := datasource_cloud_diagrams_schemes.NewCombinerValue(
			datasource_cloud_diagrams_schemes.CombinerValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"_id":   types.StringValue(c.UnderscoreId),
				"color": types.StringPointerValue(c.Color),
				"icon":  types.StringPointerValue(c.Icon),
				"items": items,
				"name":  types.StringPointerValue(c.Name),
			},
		)
		diags.Append(valDiags...)
		if diags.HasError() {
			return basetypes.MapValue{}, diags
		}
		vals[key] = val
	}

	m, mapDiags := types.MapValueFrom(ctx, cmbType, vals)
	diags.Append(mapDiags...)
	return m, diags
}

func mapCombinerItemsList(ctx context.Context, items *[]models.CloudDiagramCombinerItem) (basetypes.ListValue, diag.Diagnostics) {
	itemType := datasource_cloud_diagrams_schemes.ItemsValue{}.Type(ctx)

	if items == nil || len(*items) == 0 {
		return types.ListValueFrom(ctx, itemType, []datasource_cloud_diagrams_schemes.ItemsValue{})
	}

	var diags diag.Diagnostics
	vals := make([]datasource_cloud_diagrams_schemes.ItemsValue, 0, len(*items))
	for _, item := range *items {
		val, valDiags := datasource_cloud_diagrams_schemes.NewItemsValue(
			datasource_cloud_diagrams_schemes.ItemsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"_id":  types.StringValue(item.UnderscoreId),
				"type": types.StringValue(string(item.Type)),
			},
		)
		diags.Append(valDiags...)
		if diags.HasError() {
			return basetypes.ListValue{}, diags
		}
		vals = append(vals, val)
	}

	list, listDiags := types.ListValueFrom(ctx, itemType, vals)
	diags.Append(listDiags...)
	return list, diags
}

// --- Note ---

func mapNoteMap(ctx context.Context, notes *map[string]models.CloudDiagramNote) (basetypes.MapValue, diag.Diagnostics) {
	noteType := datasource_cloud_diagrams_schemes.NoteValue{}.Type(ctx)

	if notes == nil || len(*notes) == 0 {
		return types.MapNull(noteType), nil
	}

	var diags diag.Diagnostics
	vals := make(map[string]datasource_cloud_diagrams_schemes.NoteValue, len(*notes))
	for key, n := range *notes {
		val, valDiags := datasource_cloud_diagrams_schemes.NewNoteValue(
			datasource_cloud_diagrams_schemes.NoteValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"_id":       types.StringValue(n.UnderscoreId),
				"color":     types.StringPointerValue(n.Color),
				"font_size": float32PtrToBigFloat(n.FontSize),
				"text":      types.StringPointerValue(n.Text),
			},
		)
		diags.Append(valDiags...)
		if diags.HasError() {
			return basetypes.MapValue{}, diags
		}
		vals[key] = val
	}

	m, mapDiags := types.MapValueFrom(ctx, noteType, vals)
	diags.Append(mapDiags...)
	return m, diags
}

// --- Utility helpers ---

// mapEnumPointerValue converts a typed enum pointer to types.String.
// Works for any ~string enum type.
func mapEnumPointerValue[T ~string](v *T) basetypes.StringValue {
	if v != nil {
		return types.StringValue(string(*v))
	}
	return types.StringNull()
}

// intPtrToInt64Ptr converts *int to *int64.
func intPtrToInt64Ptr(v *int) *int64 {
	if v == nil {
		return nil
	}
	return new(int64(*v))
}

// float32PtrToBigFloat converts a *float32 to a NumberValue via big.Float.
func float32PtrToBigFloat(v *float32) basetypes.NumberValue {
	if v == nil {
		return types.NumberNull()
	}
	return types.NumberValue(new(big.Float).SetFloat64(float64(*v)))
}
