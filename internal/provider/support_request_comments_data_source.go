package provider

import (
	"context"
	"fmt"

	ds "github.com/doitintl/terraform-provider-doit/internal/provider/datasource_support_request_comments"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var _ datasource.DataSource = (*supportRequestCommentsDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*supportRequestCommentsDataSource)(nil)

func NewSupportRequestCommentsDataSource() datasource.DataSource {
	return &supportRequestCommentsDataSource{}
}

type supportRequestCommentsDataSource struct {
	client *models.ClientWithResponses
}

func (d *supportRequestCommentsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_support_request_comments"
}

func (d *supportRequestCommentsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *supportRequestCommentsDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = ds.SupportRequestCommentsDataSourceSchema(ctx)
}

func (d *supportRequestCommentsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ds.SupportRequestCommentsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If ticket_id is unknown (depends on a resource not yet created), return early
	// with unknown comments list to prevent incorrect data usage.
	if data.TicketId.IsUnknown() {
		data.Comments = types.ListUnknown(ds.CommentsValue{}.Type(ctx))
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	ticketId := data.TicketId.ValueInt64()
	commentsResp, err := d.client.IdOfTicketCommentsListWithResponse(ctx, ticketId)
	if err != nil {
		resp.Diagnostics.AddError("Error reading support request comments", err.Error())
		return
	}
	if commentsResp.StatusCode() == 404 {
		resp.Diagnostics.AddError(
			"Support request not found",
			fmt.Sprintf("Support request with ID %d not found", ticketId),
		)
		return
	}
	if commentsResp.StatusCode() != 200 || commentsResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error reading support request comments",
			fmt.Sprintf("Unexpected status: %d, body: %s", commentsResp.StatusCode(), string(commentsResp.Body)),
		)
		return
	}

	result := commentsResp.JSON200

	// Map API comments to Terraform state
	if result.Comments != nil && len(*result.Comments) > 0 {
		commentVals := make([]ds.CommentsValue, 0, len(*result.Comments))
		for _, c := range *result.Comments {
			// Map attachments for this comment
			attachmentsList, attachDiags := mapAttachments(ctx, c.Attachments)
			resp.Diagnostics.Append(attachDiags...)
			if resp.Diagnostics.HasError() {
				return
			}

			commentVal, diags := ds.NewCommentsValue(
				ds.CommentsValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"id":          types.Int64PointerValue(c.Id),
					"author":      types.StringPointerValue(c.Author),
					"body":        types.StringPointerValue(c.Body),
					"created":     types.Int64PointerValue(c.Created),
					"attachments": attachmentsList,
				},
			)
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}
			commentVals = append(commentVals, commentVal)
		}
		var listDiags diag.Diagnostics
		data.Comments, listDiags = types.ListValueFrom(ctx, ds.CommentsValue{}.Type(ctx), commentVals)
		resp.Diagnostics.Append(listDiags...)
	} else {
		// Return empty list, not null — safe for consumers to iterate
		var emptyDiags diag.Diagnostics
		data.Comments, emptyDiags = types.ListValueFrom(ctx, ds.CommentsValue{}.Type(ctx), []ds.CommentsValue{})
		resp.Diagnostics.Append(emptyDiags...)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// mapAttachments converts API attachment items to a Terraform list value.
func mapAttachments(ctx context.Context, attachments *[]models.CommentExtAPIAttachmentsItem) (basetypes.ListValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := ds.AttachmentsValue{}.Type(ctx)

	if attachments == nil || len(*attachments) == 0 {
		emptyList, d := types.ListValueFrom(ctx, elemType, []ds.AttachmentsValue{})
		diags.Append(d...)
		return emptyList, diags
	}

	attachVals := make([]ds.AttachmentsValue, 0, len(*attachments))
	for _, a := range *attachments {
		attachVal, d := ds.NewAttachmentsValue(
			ds.AttachmentsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"id":          types.Int64PointerValue(a.Id),
				"file_name":   types.StringPointerValue(a.FileName),
				"content_url": types.StringPointerValue(a.ContentUrl),
			},
		)
		diags.Append(d...)
		if diags.HasError() {
			return basetypes.ListValue{}, diags
		}
		attachVals = append(attachVals, attachVal)
	}

	list, d := types.ListValueFrom(ctx, elemType, attachVals)
	diags.Append(d...)
	return list, diags
}
