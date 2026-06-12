package provider

import (
	"context"
	"fmt"
	"time"

	ds "github.com/doitintl/terraform-provider-doit/internal/provider/datasource_cloud_diagrams_export"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// Compile-time interface checks.
var _ datasource.DataSource = (*cloudDiagramsExportDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*cloudDiagramsExportDataSource)(nil)

// NewCloudDiagramsExportDataSource creates a new instance of the data source.
func NewCloudDiagramsExportDataSource() datasource.DataSource {
	return &cloudDiagramsExportDataSource{}
}

// cloudDiagramsExportDataSource implements datasource.DataSource.
type cloudDiagramsExportDataSource struct {
	client *models.ClientWithResponses
}

// cloudDiagramsExportDataSourceModel is the Terraform state model.
type cloudDiagramsExportDataSourceModel struct {
	ds.CloudDiagramsExportModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *cloudDiagramsExportDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_diagrams_export"
}

func (d *cloudDiagramsExportDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := ds.CloudDiagramsExportDataSourceSchema(ctx)
	s.Attributes["timeouts"] = timeouts.Attributes(ctx)
	s.Description = "Exports the full content of a Cloud Diagram layer as a structured document with anonymized component IDs."
	s.MarkdownDescription = "Exports the full content of a Cloud Diagram layer as a structured document with anonymized component IDs."
	resp.Schema = s
}

func (d *cloudDiagramsExportDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *cloudDiagramsExportDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data cloudDiagramsExportDataSourceModel

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

	if !req.Config.Raw.IsFullyKnown() {
		data.Attachments = types.ListUnknown(ds.AttachmentsValue{}.Type(ctx))
		data.Combiners = types.ListUnknown(ds.CombinersValue{}.Type(ctx))
		data.Elements = types.ListUnknown(ds.ElementsValue{}.Type(ctx))
		data.Groups = types.ListUnknown(ds.GroupsValue{}.Type(ctx))
		data.Links = types.ListUnknown(ds.LinksValue{}.Type(ctx))
		data.Nodes = types.ListUnknown(ds.NodesValue{}.Type(ctx))
		data.Notes = types.ListUnknown(ds.NotesValue{}.Type(ctx))
		data.Metadata = ds.NewMetadataValueUnknown()
		data.Statussheet = jsontypes.NewNormalizedUnknown()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	layerID := data.Id.ValueString()

	apiResp, err := d.client.ExportCloudDiagramJsonWithResponse(ctx, layerID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Cloud Diagram Export",
			fmt.Sprintf("Unable to export diagram: %v", err),
		)
		return
	}

	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Cloud Diagram Export",
			fmt.Sprintf("Cloud Diagrams API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	// Map API response to Terraform state.
	resp.Diagnostics.Append(mapExportToState(ctx, &data, apiResp.JSON200)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// mapExportToState maps the API response to the Terraform state model.
func mapExportToState(
	ctx context.Context,
	data *cloudDiagramsExportDataSourceModel,
	export *models.CloudDiagramExportJsonResponse,
) diag.Diagnostics {
	var diags diag.Diagnostics

	// Statussheet (free-form JSON).
	data.Statussheet = mapFreeformJSON(export.Statussheet)

	// Metadata.
	connectionsMap, mapDiags := types.MapValueFrom(ctx, types.StringType, map[string]string{})
	diags.Append(mapDiags...)
	if export.Metadata.Connections != nil && len(*export.Metadata.Connections) > 0 {
		connectionsMap, mapDiags = types.MapValueFrom(ctx, types.StringType, *export.Metadata.Connections)
		diags.Append(mapDiags...)
		if diags.HasError() {
			return diags
		}
	}

	metadataVal, metaDiags := ds.NewMetadataValue(
		ds.MetadataValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"connections": connectionsMap,
			"date":        types.StringValue(export.Metadata.Date.UTC().Format(time.RFC3339)),
			"user":        types.StringValue(export.Metadata.User),
			"version":     types.StringValue(export.Metadata.Version),
		},
	)
	diags.Append(metaDiags...)
	if diags.HasError() {
		return diags
	}
	data.Metadata = metadataVal

	// Component lists.
	nodeList, nodeDiags := mapExportNodeList(ctx, export.Nodes)
	diags.Append(nodeDiags...)
	if diags.HasError() {
		return diags
	}
	data.Nodes = nodeList

	elementList, elemDiags := mapExportElementList(ctx, export.Elements)
	diags.Append(elemDiags...)
	if diags.HasError() {
		return diags
	}
	data.Elements = elementList

	groupList, groupDiags := mapExportGroupList(ctx, export.Groups)
	diags.Append(groupDiags...)
	if diags.HasError() {
		return diags
	}
	data.Groups = groupList

	linkList, linkDiags := mapExportLinkList(ctx, export.Links)
	diags.Append(linkDiags...)
	if diags.HasError() {
		return diags
	}
	data.Links = linkList

	attachmentList, attDiags := mapExportAttachmentList(ctx, export.Attachments)
	diags.Append(attDiags...)
	if diags.HasError() {
		return diags
	}
	data.Attachments = attachmentList

	combinerList, cmbDiags := mapExportCombinerList(ctx, export.Combiners)
	diags.Append(cmbDiags...)
	if diags.HasError() {
		return diags
	}
	data.Combiners = combinerList

	noteList, noteDiags := mapExportNoteList(ctx, export.Notes)
	diags.Append(noteDiags...)
	if diags.HasError() {
		return diags
	}
	data.Notes = noteList

	return diags
}

// --- Shared helpers ---

// mapExportIssuesList maps a slice of CloudDiagramIssue to a Terraform list value.
func mapExportIssuesList(ctx context.Context, issues *[]models.CloudDiagramIssue) (basetypes.ListValue, diag.Diagnostics) {
	issueType := ds.IssuesValue{}.Type(ctx)

	if issues == nil || len(*issues) == 0 {
		return types.ListValueFrom(ctx, issueType, []ds.IssuesValue{})
	}

	var diags diag.Diagnostics
	vals := make([]ds.IssuesValue, 0, len(*issues))
	for _, issue := range *issues {
		val, valDiags := ds.NewIssuesValue(
			ds.IssuesValue{}.AttributeTypes(ctx),
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

// mapExportTagsList maps a string slice pointer to a Terraform list value.
func mapExportTagsList(ctx context.Context, tags *[]string) (basetypes.ListValue, diag.Diagnostics) {
	if tags == nil || len(*tags) == 0 {
		return types.ListValueFrom(ctx, types.StringType, []string{})
	}
	return types.ListValueFrom(ctx, types.StringType, *tags)
}

// --- Node ---

func mapExportNodeList(ctx context.Context, nodes *[]models.CloudDiagramNode) (basetypes.ListValue, diag.Diagnostics) {
	nodeType := ds.NodesValue{}.Type(ctx)

	if nodes == nil || len(*nodes) == 0 {
		return types.ListValueFrom(ctx, nodeType, []ds.NodesValue{})
	}

	var diags diag.Diagnostics
	vals := make([]ds.NodesValue, 0, len(*nodes))
	for _, n := range *nodes {
		issues, issDiags := mapExportIssuesList(ctx, n.Issues)
		diags.Append(issDiags...)
		if diags.HasError() {
			return basetypes.ListValue{}, diags
		}

		tags, tagsDiags := mapExportTagsList(ctx, n.Tags)
		diags.Append(tagsDiags...)
		if diags.HasError() {
			return basetypes.ListValue{}, diags
		}

		infraNode := ds.NewInfraNodeValueNull()
		if n.InfraNode != nil {
			var infraDiags diag.Diagnostics
			infraNode, infraDiags = ds.NewInfraNodeValue(
				ds.InfraNodeValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"_id":       types.StringValue(n.InfraNode.UnderscoreId),
					"scheme_id": types.StringValue(n.InfraNode.SchemeId),
					"ss_id":     types.StringValue(n.InfraNode.SsId),
				},
			)
			diags.Append(infraDiags...)
			if diags.HasError() {
				return basetypes.ListValue{}, diags
			}
		}

		val, valDiags := ds.NewNodesValue(
			ds.NodesValue{}.AttributeTypes(ctx),
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
				"props":          mapFreeformJSON(n.Props),
				"running":        types.BoolPointerValue(n.Running),
				"tags":           tags,
			},
		)
		diags.Append(valDiags...)
		if diags.HasError() {
			return basetypes.ListValue{}, diags
		}
		vals = append(vals, val)
	}

	list, listDiags := types.ListValueFrom(ctx, nodeType, vals)
	diags.Append(listDiags...)
	return list, diags
}

// --- Element ---

func mapExportElementList(ctx context.Context, elements *[]models.CloudDiagramElement) (basetypes.ListValue, diag.Diagnostics) {
	elemType := ds.ElementsValue{}.Type(ctx)

	if elements == nil || len(*elements) == 0 {
		return types.ListValueFrom(ctx, elemType, []ds.ElementsValue{})
	}

	var diags diag.Diagnostics
	vals := make([]ds.ElementsValue, 0, len(*elements))
	for _, e := range *elements {
		issues, issDiags := mapExportIssuesList(ctx, e.Issues)
		diags.Append(issDiags...)
		if diags.HasError() {
			return basetypes.ListValue{}, diags
		}

		tags, tagsDiags := mapExportTagsList(ctx, e.Tags)
		diags.Append(tagsDiags...)
		if diags.HasError() {
			return basetypes.ListValue{}, diags
		}

		val, valDiags := ds.NewElementsValue(
			ds.ElementsValue{}.AttributeTypes(ctx),
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
				"props":       mapFreeformJSON(e.Props),
				"tags":        tags,
			},
		)
		diags.Append(valDiags...)
		if diags.HasError() {
			return basetypes.ListValue{}, diags
		}
		vals = append(vals, val)
	}

	list, listDiags := types.ListValueFrom(ctx, elemType, vals)
	diags.Append(listDiags...)
	return list, diags
}

// --- Group ---

func mapExportGroupList(ctx context.Context, groups *[]models.CloudDiagramGroup) (basetypes.ListValue, diag.Diagnostics) {
	groupType := ds.GroupsValue{}.Type(ctx)

	if groups == nil || len(*groups) == 0 {
		return types.ListValueFrom(ctx, groupType, []ds.GroupsValue{})
	}

	var diags diag.Diagnostics
	vals := make([]ds.GroupsValue, 0, len(*groups))
	for _, g := range *groups {
		issues, issDiags := mapExportIssuesList(ctx, g.Issues)
		diags.Append(issDiags...)
		if diags.HasError() {
			return basetypes.ListValue{}, diags
		}

		items, itemsDiags := mapExportGroupItemsList(ctx, g.Items)
		diags.Append(itemsDiags...)
		if diags.HasError() {
			return basetypes.ListValue{}, diags
		}

		tags, tagsDiags := mapExportTagsList(ctx, g.Tags)
		diags.Append(tagsDiags...)
		if diags.HasError() {
			return basetypes.ListValue{}, diags
		}

		val, valDiags := ds.NewGroupsValue(
			ds.GroupsValue{}.AttributeTypes(ctx),
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
				"props":       mapFreeformJSON(g.Props),
				"tags":        tags,
			},
		)
		diags.Append(valDiags...)
		if diags.HasError() {
			return basetypes.ListValue{}, diags
		}
		vals = append(vals, val)
	}

	list, listDiags := types.ListValueFrom(ctx, groupType, vals)
	diags.Append(listDiags...)
	return list, diags
}

func mapExportGroupItemsList(ctx context.Context, items *[]models.CloudDiagramGroupItem) (basetypes.ListValue, diag.Diagnostics) {
	itemType := ds.ItemsValue{}.Type(ctx)

	if items == nil || len(*items) == 0 {
		return types.ListValueFrom(ctx, itemType, []ds.ItemsValue{})
	}

	var diags diag.Diagnostics
	vals := make([]ds.ItemsValue, 0, len(*items))
	for _, item := range *items {
		val, valDiags := ds.NewItemsValue(
			ds.ItemsValue{}.AttributeTypes(ctx),
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

func mapExportLinkList(ctx context.Context, links *[]models.CloudDiagramLink) (basetypes.ListValue, diag.Diagnostics) {
	linkType := ds.LinksValue{}.Type(ctx)

	if links == nil || len(*links) == 0 {
		return types.ListValueFrom(ctx, linkType, []ds.LinksValue{})
	}

	var diags diag.Diagnostics
	vals := make([]ds.LinksValue, 0, len(*links))
	for _, l := range *links {
		issues, issDiags := mapExportIssuesList(ctx, l.Issues)
		diags.Append(issDiags...)
		if diags.HasError() {
			return basetypes.ListValue{}, diags
		}

		tags, tagsDiags := mapExportTagsList(ctx, l.Tags)
		diags.Append(tagsDiags...)
		if diags.HasError() {
			return basetypes.ListValue{}, diags
		}

		originVal, originDiags := ds.NewOriginValue(
			ds.OriginValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"_id":       types.StringValue(l.Origin.UnderscoreId),
				"type":      types.StringValue(string(l.Origin.Type)),
				"scheme_id": types.StringValue(l.Origin.SchemeId),
				"ss_id":     types.StringValue(l.Origin.SsId),
			},
		)
		diags.Append(originDiags...)
		if diags.HasError() {
			return basetypes.ListValue{}, diags
		}

		destVal, destDiags := ds.NewDestinationValue(
			ds.DestinationValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"_id":       types.StringValue(l.Destination.UnderscoreId),
				"type":      types.StringValue(string(l.Destination.Type)),
				"scheme_id": types.StringValue(l.Destination.SchemeId),
				"ss_id":     types.StringValue(l.Destination.SsId),
			},
		)
		diags.Append(destDiags...)
		if diags.HasError() {
			return basetypes.ListValue{}, diags
		}

		val, valDiags := ds.NewLinksValue(
			ds.LinksValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"_id":             types.StringValue(l.UnderscoreId),
				"cld_account":     types.StringPointerValue(l.CldAccount),
				"cld_id":          types.StringPointerValue(l.CldId),
				"cld_sync":        types.BoolPointerValue(l.CldSync),
				"cld_type":        mapEnumPointerValue(l.CldType),
				"connection_type": mapEnumPointerValue(l.ConnectionType),
				"destination":     destVal,
				"issues":          issues,
				"name":            types.StringPointerValue(l.Name),
				"origin":          originVal,
				"owner_ss_id":     types.StringPointerValue(l.OwnerSsId),
				"props":           mapFreeformJSON(l.Props),
				"tags":            tags,
			},
		)
		diags.Append(valDiags...)
		if diags.HasError() {
			return basetypes.ListValue{}, diags
		}
		vals = append(vals, val)
	}

	list, listDiags := types.ListValueFrom(ctx, linkType, vals)
	diags.Append(listDiags...)
	return list, diags
}

// --- Attachment ---

func mapExportAttachmentList(ctx context.Context, attachments *[]models.CloudDiagramAttachment) (basetypes.ListValue, diag.Diagnostics) {
	attType := ds.AttachmentsValue{}.Type(ctx)

	if attachments == nil || len(*attachments) == 0 {
		return types.ListValueFrom(ctx, attType, []ds.AttachmentsValue{})
	}

	var diags diag.Diagnostics
	vals := make([]ds.AttachmentsValue, 0, len(*attachments))
	for _, a := range *attachments {
		issues, issDiags := mapExportIssuesList(ctx, a.Issues)
		diags.Append(issDiags...)
		if diags.HasError() {
			return basetypes.ListValue{}, diags
		}

		tags, tagsDiags := mapExportTagsList(ctx, a.Tags)
		diags.Append(tagsDiags...)
		if diags.HasError() {
			return basetypes.ListValue{}, diags
		}

		val, valDiags := ds.NewAttachmentsValue(
			ds.AttachmentsValue{}.AttributeTypes(ctx),
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
				"props":       mapFreeformJSON(a.Props),
				"tags":        tags,
			},
		)
		diags.Append(valDiags...)
		if diags.HasError() {
			return basetypes.ListValue{}, diags
		}
		vals = append(vals, val)
	}

	list, listDiags := types.ListValueFrom(ctx, attType, vals)
	diags.Append(listDiags...)
	return list, diags
}

// --- Combiner ---

func mapExportCombinerList(ctx context.Context, combiners *[]models.CloudDiagramCombiner) (basetypes.ListValue, diag.Diagnostics) {
	cmbType := ds.CombinersValue{}.Type(ctx)

	if combiners == nil || len(*combiners) == 0 {
		return types.ListValueFrom(ctx, cmbType, []ds.CombinersValue{})
	}

	var diags diag.Diagnostics
	vals := make([]ds.CombinersValue, 0, len(*combiners))
	for _, c := range *combiners {
		items, itemsDiags := mapExportCombinerItemsList(ctx, c.Items)
		diags.Append(itemsDiags...)
		if diags.HasError() {
			return basetypes.ListValue{}, diags
		}

		val, valDiags := ds.NewCombinersValue(
			ds.CombinersValue{}.AttributeTypes(ctx),
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
			return basetypes.ListValue{}, diags
		}
		vals = append(vals, val)
	}

	list, listDiags := types.ListValueFrom(ctx, cmbType, vals)
	diags.Append(listDiags...)
	return list, diags
}

func mapExportCombinerItemsList(ctx context.Context, items *[]models.CloudDiagramCombinerItem) (basetypes.ListValue, diag.Diagnostics) {
	itemType := ds.ItemsValue{}.Type(ctx)

	if items == nil || len(*items) == 0 {
		return types.ListValueFrom(ctx, itemType, []ds.ItemsValue{})
	}

	var diags diag.Diagnostics
	vals := make([]ds.ItemsValue, 0, len(*items))
	for _, item := range *items {
		val, valDiags := ds.NewItemsValue(
			ds.ItemsValue{}.AttributeTypes(ctx),
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

func mapExportNoteList(ctx context.Context, notes *[]models.CloudDiagramNote) (basetypes.ListValue, diag.Diagnostics) {
	noteType := ds.NotesValue{}.Type(ctx)

	if notes == nil || len(*notes) == 0 {
		return types.ListValueFrom(ctx, noteType, []ds.NotesValue{})
	}

	var diags diag.Diagnostics
	vals := make([]ds.NotesValue, 0, len(*notes))
	for _, n := range *notes {
		val, valDiags := ds.NewNotesValue(
			ds.NotesValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"_id":       types.StringValue(n.UnderscoreId),
				"color":     types.StringPointerValue(n.Color),
				"font_size": ssFloat32PtrToBigFloat(n.FontSize),
				"text":      types.StringPointerValue(n.Text),
			},
		)
		diags.Append(valDiags...)
		if diags.HasError() {
			return basetypes.ListValue{}, diags
		}
		vals = append(vals, val)
	}

	list, listDiags := types.ListValueFrom(ctx, noteType, vals)
	diags.Append(listDiags...)
	return list, diags
}
