package provider

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type (
	supportRequestTagsResource struct {
		client *models.ClientWithResponses
	}
	supportRequestTagsResourceModel struct {
		Id       types.String   `tfsdk:"id"`
		TicketId types.Int64    `tfsdk:"ticket_id"`
		Tags     types.Set      `tfsdk:"tags"`
		Timeouts timeouts.Value `tfsdk:"timeouts"`
	}
)

var (
	_ resource.Resource                = (*supportRequestTagsResource)(nil)
	_ resource.ResourceWithConfigure   = (*supportRequestTagsResource)(nil)
	_ resource.ResourceWithImportState = (*supportRequestTagsResource)(nil)
)

func NewSupportRequestTagsResource() resource.Resource {
	return &supportRequestTagsResource{}
}

func (r *supportRequestTagsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*models.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *models.ClientWithResponses, got: %T.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *supportRequestTagsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_support_request_tags"
}

func (r *supportRequestTagsResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Manages tags on a DoiT support request.",
		MarkdownDescription: "Manages tags on a DoiT support request.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				Description:         "Same as `ticket_id` (stringified).",
				MarkdownDescription: "Same as `ticket_id` (stringified).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"ticket_id": schema.Int64Attribute{
				Required:            true,
				Description:         "The ID of the support request to manage tags for.",
				MarkdownDescription: "The ID of the support request to manage tags for.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"tags": schema.SetAttribute{
				ElementType:         types.StringType,
				Required:            true,
				Description:         "Set of tags to apply to the support request. Tags are normalized (trimmed and lowercased) by the API.",
				MarkdownDescription: "Set of tags to apply to the support request. Tags are normalized (trimmed and lowercased) by the API.",
			},
			"timeouts": timeouts.Attributes(ctx, timeouts.Opts{
				Create: true,
				Read:   true,
				Update: true,
				Delete: true,
			}),
		},
	}
}

func (r *supportRequestTagsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan supportRequestTagsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, diags := plan.Timeouts.Create(ctx, 5*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	ticketId := plan.TicketId.ValueInt64()

	tags := tagsToStringSlice(plan.Tags, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if len(tags) > 0 {
		addReq := models.IdOfTicketTagsAddJSONRequestBody{
			Tags: tags,
		}
		addResp, err := r.client.IdOfTicketTagsAddWithResponse(ctx, ticketId, addReq)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Creating Support Request Tags",
				"Could not add tags: "+err.Error(),
			)
			return
		}
		if addResp.StatusCode() != 200 {
			resp.Diagnostics.AddError(
				"Error Creating Support Request Tags",
				fmt.Sprintf("Could not add tags, status: %d, body: %s", addResp.StatusCode(), string(addResp.Body)),
			)
			return
		}
	}

	plan.Id = types.StringValue(strconv.FormatInt(ticketId, 10))

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *supportRequestTagsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state supportRequestTagsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	readTimeout, diags := state.Timeouts.Read(ctx, 2*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	resp.Diagnostics.Append(r.populateState(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if state.Id.IsNull() {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *supportRequestTagsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan supportRequestTagsResourceModel
	var state supportRequestTagsResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateTimeout, diags := plan.Timeouts.Update(ctx, 5*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	oldTags := tagsToStringSlice(state.Tags, &resp.Diagnostics)
	newTags := tagsToStringSlice(plan.Tags, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	ticketId := plan.TicketId.ValueInt64()
	r.syncTags(ctx, ticketId, oldTags, newTags, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	plan.Id = state.Id

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *supportRequestTagsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state supportRequestTagsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, diags := state.Timeouts.Delete(ctx, 2*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	tags := tagsToStringSlice(state.Tags, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if len(tags) > 0 {
		removeReq := models.IdOfTicketTagsRemoveJSONRequestBody{
			Tags: tags,
		}

		removeResp, err := r.client.IdOfTicketTagsRemoveWithResponse(ctx, state.TicketId.ValueInt64(), removeReq)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Deleting Support Request Tags",
				"Could not remove tags from support request: "+err.Error(),
			)
			return
		}

		if removeResp.StatusCode() != 200 && removeResp.StatusCode() != 204 && removeResp.StatusCode() != 404 {
			resp.Diagnostics.AddError(
				"Error Deleting Support Request Tags",
				fmt.Sprintf("Could not remove tags, status: %d, body: %s", removeResp.StatusCode(), string(removeResp.Body)),
			)
			return
		}
	}
}

func (r *supportRequestTagsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	ticketId, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected numeric ticket ID, got: %q", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("ticket_id"), ticketId)...)
}
