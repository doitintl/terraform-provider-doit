package provider

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_cloud_diagrams_statussheet"
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
var _ datasource.DataSource = (*cloudDiagramsStatussheetDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*cloudDiagramsStatussheetDataSource)(nil)

// NewCloudDiagramsStatussheetDataSource creates a new instance of the data source.
func NewCloudDiagramsStatussheetDataSource() datasource.DataSource {
	return &cloudDiagramsStatussheetDataSource{}
}

// cloudDiagramsStatussheetDataSource implements datasource.DataSource.
type cloudDiagramsStatussheetDataSource struct {
	client *models.ClientWithResponses
}

// cloudDiagramsStatussheetDataSourceModel is the Terraform state model.
type cloudDiagramsStatussheetDataSourceModel struct {
	Id         types.String   `tfsdk:"id"`
	P          types.String   `tfsdk:"p"`
	Node       types.Map      `tfsdk:"node"`
	Element    types.Map      `tfsdk:"element"`
	Group      types.Map      `tfsdk:"group"`
	Link       types.Map      `tfsdk:"link"`
	Attachment types.Map      `tfsdk:"attachment"`
	Combiner   types.Map      `tfsdk:"combiner"`
	Note       types.Map      `tfsdk:"note"`
	Timeouts   timeouts.Value `tfsdk:"timeouts"`
}

func (d *cloudDiagramsStatussheetDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_diagrams_statussheet"
}

func (d *cloudDiagramsStatussheetDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	genSchema := datasource_cloud_diagrams_statussheet.CloudDiagramsStatussheetDataSourceSchema(ctx)

	// Override the generated id to be required input (layer ID).
	genSchema.Attributes["id"] = schema.StringAttribute{
		Required:            true,
		Description:         "Layer ID to retrieve components for.",
		MarkdownDescription: "Layer ID to retrieve components for.",
	}

	genSchema.Attributes["timeouts"] = timeouts.Attributes(ctx)

	genSchema.Description = "Retrieves the components of a specific Cloud Diagram layer."
	genSchema.MarkdownDescription = "Retrieves the components of a specific Cloud Diagram layer."

	resp.Schema = genSchema
}

func (d *cloudDiagramsStatussheetDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *cloudDiagramsStatussheetDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data cloudDiagramsStatussheetDataSourceModel

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
		data.Node = types.MapUnknown(datasource_cloud_diagrams_statussheet.NodeValue{}.Type(ctx))
		data.Element = types.MapUnknown(datasource_cloud_diagrams_statussheet.ElementValue{}.Type(ctx))
		data.Group = types.MapUnknown(datasource_cloud_diagrams_statussheet.GroupValue{}.Type(ctx))
		data.Link = types.MapUnknown(datasource_cloud_diagrams_statussheet.LinkValue{}.Type(ctx))
		data.Attachment = types.MapUnknown(datasource_cloud_diagrams_statussheet.AttachmentValue{}.Type(ctx))
		data.Combiner = types.MapUnknown(datasource_cloud_diagrams_statussheet.CombinerValue{}.Type(ctx))
		data.Note = types.MapUnknown(datasource_cloud_diagrams_statussheet.NoteValue{}.Type(ctx))
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	layerID := data.Id.ValueString()

	// Build query params.
	params := &models.GetStatussheetComponentsParams{}
	if !data.P.IsNull() {
		params.P = data.P.ValueStringPointer()
	}

	// Send empty request body (retrieve all components).
	body := models.GetStatussheetComponentsJSONRequestBody{}

	apiResp, err := d.client.GetStatussheetComponentsWithResponse(ctx, layerID, params, body)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Cloud Diagram Statussheet",
			fmt.Sprintf("Unable to read statussheet components: %v", err),
		)
		return
	}

	if apiResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Reading Cloud Diagram Statussheet",
			fmt.Sprintf("Cloud Diagram Statussheet API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Cloud Diagram Statussheet",
			fmt.Sprintf("Cloud Diagram Statussheet API returned status 200 but response body could not be parsed: %s", string(apiResp.Body)),
		)
		return
	}

	// Map API response to Terraform state.
	resp.Diagnostics.Append(mapStatussheetComponentsToState(ctx, &data, apiResp.JSON200)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// mapStatussheetComponentsToState maps the API response to the Terraform state model.
func mapStatussheetComponentsToState(
	ctx context.Context,
	data *cloudDiagramsStatussheetDataSourceModel,
	components *models.CloudDiagramStatussheetComponents,
) diag.Diagnostics {
	var diags diag.Diagnostics

	nodeMap, nodeDiags := mapSSNodeMap(ctx, components.Node)
	diags.Append(nodeDiags...)
	if diags.HasError() {
		return diags
	}
	data.Node = nodeMap

	elementMap, elementDiags := mapSSElementMap(ctx, components.Element)
	diags.Append(elementDiags...)
	if diags.HasError() {
		return diags
	}
	data.Element = elementMap

	groupMap, groupDiags := mapSSGroupMap(ctx, components.Group)
	diags.Append(groupDiags...)
	if diags.HasError() {
		return diags
	}
	data.Group = groupMap

	linkMap, linkDiags := mapSSLinkMap(ctx, components.Link)
	diags.Append(linkDiags...)
	if diags.HasError() {
		return diags
	}
	data.Link = linkMap

	attachmentMap, attachmentDiags := mapSSAttachmentMap(ctx, components.Attachment)
	diags.Append(attachmentDiags...)
	if diags.HasError() {
		return diags
	}
	data.Attachment = attachmentMap

	combinerMap, combinerDiags := mapSSCombinerMap(ctx, components.Combiner)
	diags.Append(combinerDiags...)
	if diags.HasError() {
		return diags
	}
	data.Combiner = combinerMap

	noteMap, noteDiags := mapSSNoteMap(ctx, components.Note)
	diags.Append(noteDiags...)
	if diags.HasError() {
		return diags
	}
	data.Note = noteMap

	return diags
}

// --- Shared helpers ---

// mapSSIssuesList maps a slice of CloudDiagramIssue to a Terraform list value.
func mapSSIssuesList(ctx context.Context, issues *[]models.CloudDiagramIssue) (basetypes.ListValue, diag.Diagnostics) {
	issueType := datasource_cloud_diagrams_statussheet.IssuesValue{}.Type(ctx)

	if issues == nil || len(*issues) == 0 {
		return types.ListValueFrom(ctx, issueType, []datasource_cloud_diagrams_statussheet.IssuesValue{})
	}

	var diags diag.Diagnostics
	vals := make([]datasource_cloud_diagrams_statussheet.IssuesValue, 0, len(*issues))
	for _, issue := range *issues {
		val, valDiags := datasource_cloud_diagrams_statussheet.NewIssuesValue(
			datasource_cloud_diagrams_statussheet.IssuesValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"_id":     types.StringPointerValue(issue.UnderscoreId),
				"comment": types.StringPointerValue(issue.Comment),
				"jira":    types.StringPointerValue(issue.Jira),
				"snoozed": ssFloat32PtrToBigFloat(issue.Snoozed),
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

// mapSSTagsList maps a string slice pointer to a Terraform list value.
func mapSSTagsList(ctx context.Context, tags *[]string) (basetypes.ListValue, diag.Diagnostics) {
	if tags == nil || len(*tags) == 0 {
		return types.ListValueFrom(ctx, types.StringType, []string{})
	}
	return types.ListValueFrom(ctx, types.StringType, *tags)
}

// mapSSPropsValue maps the free-form props map to a known empty PropsValue.
func mapSSPropsValue(ctx context.Context) datasource_cloud_diagrams_statussheet.PropsValue {
	return datasource_cloud_diagrams_statussheet.NewPropsValueMust(
		datasource_cloud_diagrams_statussheet.PropsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{},
	)
}

// ssFloat32PtrToBigFloat converts a *float32 to a NumberValue via big.Float.
func ssFloat32PtrToBigFloat(v *float32) basetypes.NumberValue {
	if v == nil {
		return types.NumberNull()
	}
	return types.NumberValue(new(big.Float).SetFloat64(float64(*v)))
}

// --- Node ---

func mapSSNodeMap(ctx context.Context, nodes *map[string]models.CloudDiagramNode) (basetypes.MapValue, diag.Diagnostics) {
	nodeType := datasource_cloud_diagrams_statussheet.NodeValue{}.Type(ctx)

	if nodes == nil || len(*nodes) == 0 {
		return types.MapNull(nodeType), nil
	}

	var diags diag.Diagnostics
	vals := make(map[string]datasource_cloud_diagrams_statussheet.NodeValue, len(*nodes))
	for key, n := range *nodes {
		issues, issDiags := mapSSIssuesList(ctx, n.Issues)
		diags.Append(issDiags...)
		if diags.HasError() {
			return basetypes.MapValue{}, diags
		}

		tags, tagsDiags := mapSSTagsList(ctx, n.Tags)
		diags.Append(tagsDiags...)
		if diags.HasError() {
			return basetypes.MapValue{}, diags
		}

		infraNode := datasource_cloud_diagrams_statussheet.NewInfraNodeValueNull()
		if n.InfraNode != nil {
			var infraDiags diag.Diagnostics
			infraNode, infraDiags = datasource_cloud_diagrams_statussheet.NewInfraNodeValue(
				datasource_cloud_diagrams_statussheet.InfraNodeValue{}.AttributeTypes(ctx),
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

		val, valDiags := datasource_cloud_diagrams_statussheet.NewNodeValue(
			datasource_cloud_diagrams_statussheet.NodeValue{}.AttributeTypes(ctx),
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
				"props":          mapSSPropsValue(ctx),
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

func mapSSElementMap(ctx context.Context, elements *map[string]models.CloudDiagramElement) (basetypes.MapValue, diag.Diagnostics) {
	elemType := datasource_cloud_diagrams_statussheet.ElementValue{}.Type(ctx)

	if elements == nil || len(*elements) == 0 {
		return types.MapNull(elemType), nil
	}

	var diags diag.Diagnostics
	vals := make(map[string]datasource_cloud_diagrams_statussheet.ElementValue, len(*elements))
	for key, e := range *elements {
		issues, issDiags := mapSSIssuesList(ctx, e.Issues)
		diags.Append(issDiags...)
		if diags.HasError() {
			return basetypes.MapValue{}, diags
		}

		tags, tagsDiags := mapSSTagsList(ctx, e.Tags)
		diags.Append(tagsDiags...)
		if diags.HasError() {
			return basetypes.MapValue{}, diags
		}

		val, valDiags := datasource_cloud_diagrams_statussheet.NewElementValue(
			datasource_cloud_diagrams_statussheet.ElementValue{}.AttributeTypes(ctx),
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
				"props":       mapSSPropsValue(ctx),
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

func mapSSGroupMap(ctx context.Context, groups *map[string]models.CloudDiagramGroup) (basetypes.MapValue, diag.Diagnostics) {
	groupType := datasource_cloud_diagrams_statussheet.GroupValue{}.Type(ctx)

	if groups == nil || len(*groups) == 0 {
		return types.MapNull(groupType), nil
	}

	var diags diag.Diagnostics
	vals := make(map[string]datasource_cloud_diagrams_statussheet.GroupValue, len(*groups))
	for key, g := range *groups {
		issues, issDiags := mapSSIssuesList(ctx, g.Issues)
		diags.Append(issDiags...)
		if diags.HasError() {
			return basetypes.MapValue{}, diags
		}

		items, itemsDiags := mapSSGroupItemsList(ctx, g.Items)
		diags.Append(itemsDiags...)
		if diags.HasError() {
			return basetypes.MapValue{}, diags
		}

		tags, tagsDiags := mapSSTagsList(ctx, g.Tags)
		diags.Append(tagsDiags...)
		if diags.HasError() {
			return basetypes.MapValue{}, diags
		}

		val, valDiags := datasource_cloud_diagrams_statussheet.NewGroupValue(
			datasource_cloud_diagrams_statussheet.GroupValue{}.AttributeTypes(ctx),
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
				"props":       mapSSPropsValue(ctx),
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

func mapSSGroupItemsList(ctx context.Context, items *[]models.CloudDiagramGroupItem) (basetypes.ListValue, diag.Diagnostics) {
	itemType := datasource_cloud_diagrams_statussheet.ItemsValue{}.Type(ctx)

	if items == nil || len(*items) == 0 {
		return types.ListValueFrom(ctx, itemType, []datasource_cloud_diagrams_statussheet.ItemsValue{})
	}

	var diags diag.Diagnostics
	vals := make([]datasource_cloud_diagrams_statussheet.ItemsValue, 0, len(*items))
	for _, item := range *items {
		val, valDiags := datasource_cloud_diagrams_statussheet.NewItemsValue(
			datasource_cloud_diagrams_statussheet.ItemsValue{}.AttributeTypes(ctx),
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

func mapSSLinkMap(ctx context.Context, links *map[string]models.CloudDiagramLink) (basetypes.MapValue, diag.Diagnostics) {
	linkType := datasource_cloud_diagrams_statussheet.LinkValue{}.Type(ctx)

	if links == nil || len(*links) == 0 {
		return types.MapNull(linkType), nil
	}

	var diags diag.Diagnostics
	vals := make(map[string]datasource_cloud_diagrams_statussheet.LinkValue, len(*links))
	for key, l := range *links {
		issues, issDiags := mapSSIssuesList(ctx, l.Issues)
		diags.Append(issDiags...)
		if diags.HasError() {
			return basetypes.MapValue{}, diags
		}

		tags, tagsDiags := mapSSTagsList(ctx, l.Tags)
		diags.Append(tagsDiags...)
		if diags.HasError() {
			return basetypes.MapValue{}, diags
		}

		val, valDiags := datasource_cloud_diagrams_statussheet.NewLinkValue(
			datasource_cloud_diagrams_statussheet.LinkValue{}.AttributeTypes(ctx),
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
				"props":           mapSSPropsValue(ctx),
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

func mapSSAttachmentMap(ctx context.Context, attachments *map[string]models.CloudDiagramAttachment) (basetypes.MapValue, diag.Diagnostics) {
	attType := datasource_cloud_diagrams_statussheet.AttachmentValue{}.Type(ctx)

	if attachments == nil || len(*attachments) == 0 {
		return types.MapNull(attType), nil
	}

	var diags diag.Diagnostics
	vals := make(map[string]datasource_cloud_diagrams_statussheet.AttachmentValue, len(*attachments))
	for key, a := range *attachments {
		issues, issDiags := mapSSIssuesList(ctx, a.Issues)
		diags.Append(issDiags...)
		if diags.HasError() {
			return basetypes.MapValue{}, diags
		}

		tags, tagsDiags := mapSSTagsList(ctx, a.Tags)
		diags.Append(tagsDiags...)
		if diags.HasError() {
			return basetypes.MapValue{}, diags
		}

		val, valDiags := datasource_cloud_diagrams_statussheet.NewAttachmentValue(
			datasource_cloud_diagrams_statussheet.AttachmentValue{}.AttributeTypes(ctx),
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
				"props":       mapSSPropsValue(ctx),
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

func mapSSCombinerMap(ctx context.Context, combiners *map[string]models.CloudDiagramCombiner) (basetypes.MapValue, diag.Diagnostics) {
	cmbType := datasource_cloud_diagrams_statussheet.CombinerValue{}.Type(ctx)

	if combiners == nil || len(*combiners) == 0 {
		return types.MapNull(cmbType), nil
	}

	var diags diag.Diagnostics
	vals := make(map[string]datasource_cloud_diagrams_statussheet.CombinerValue, len(*combiners))
	for key, c := range *combiners {
		items, itemsDiags := mapSSCombinerItemsList(ctx, c.Items)
		diags.Append(itemsDiags...)
		if diags.HasError() {
			return basetypes.MapValue{}, diags
		}

		val, valDiags := datasource_cloud_diagrams_statussheet.NewCombinerValue(
			datasource_cloud_diagrams_statussheet.CombinerValue{}.AttributeTypes(ctx),
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

func mapSSCombinerItemsList(ctx context.Context, items *[]models.CloudDiagramCombinerItem) (basetypes.ListValue, diag.Diagnostics) {
	itemType := datasource_cloud_diagrams_statussheet.ItemsValue{}.Type(ctx)

	if items == nil || len(*items) == 0 {
		return types.ListValueFrom(ctx, itemType, []datasource_cloud_diagrams_statussheet.ItemsValue{})
	}

	var diags diag.Diagnostics
	vals := make([]datasource_cloud_diagrams_statussheet.ItemsValue, 0, len(*items))
	for _, item := range *items {
		val, valDiags := datasource_cloud_diagrams_statussheet.NewItemsValue(
			datasource_cloud_diagrams_statussheet.ItemsValue{}.AttributeTypes(ctx),
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

func mapSSNoteMap(ctx context.Context, notes *map[string]models.CloudDiagramNote) (basetypes.MapValue, diag.Diagnostics) {
	noteType := datasource_cloud_diagrams_statussheet.NoteValue{}.Type(ctx)

	if notes == nil || len(*notes) == 0 {
		return types.MapNull(noteType), nil
	}

	var diags diag.Diagnostics
	vals := make(map[string]datasource_cloud_diagrams_statussheet.NoteValue, len(*notes))
	for key, n := range *notes {
		val, valDiags := datasource_cloud_diagrams_statussheet.NewNoteValue(
			datasource_cloud_diagrams_statussheet.NoteValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"_id":       types.StringValue(n.UnderscoreId),
				"color":     types.StringPointerValue(n.Color),
				"font_size": ssFloat32PtrToBigFloat(n.FontSize),
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
