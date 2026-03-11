package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type (
	labelAssignmentsResource struct {
		client *models.ClientWithResponses
	}
	labelAssignmentsResourceModel struct {
		Id          types.String `tfsdk:"id"`
		LabelId     types.String `tfsdk:"label_id"`
		Assignments types.Set    `tfsdk:"assignments"`
	}
)

// assignmentObject represents a single assignment for internal diff logic.
type assignmentObject struct {
	ObjectId   string
	ObjectType string
}

// Ensure the implementation satisfies expected interfaces.
var (
	_ resource.Resource                = (*labelAssignmentsResource)(nil)
	_ resource.ResourceWithConfigure   = (*labelAssignmentsResource)(nil)
	_ resource.ResourceWithImportState = (*labelAssignmentsResource)(nil)
)

// assignmentAttrTypes returns the attribute types for the assignment object.
func assignmentAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"object_id":   types.StringType,
		"object_type": types.StringType,
	}
}

// NewLabelAssignmentsResource creates a new label assignments resource instance.
func NewLabelAssignmentsResource() resource.Resource {
	return &labelAssignmentsResource{}
}

func (r *labelAssignmentsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*models.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *models.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *labelAssignmentsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_label_assignments"
}

func (r *labelAssignmentsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Manages the set of objects (reports, budgets, alerts, etc.) assigned to a label.",
		MarkdownDescription: "Manages the set of objects (reports, budgets, alerts, etc.) assigned to a label.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				Description:         "Same as `label_id`.",
				MarkdownDescription: "Same as `label_id`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"label_id": schema.StringAttribute{
				Required:            true,
				Description:         "The ID of the label to manage assignments for.",
				MarkdownDescription: "The ID of the label to manage assignments for.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"assignments": schema.SetNestedAttribute{
				Required:            true,
				Description:         "Set of objects assigned to the label.",
				MarkdownDescription: "Set of objects assigned to the label.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"object_id": schema.StringAttribute{
							Required:            true,
							Description:         "The ID of the object to assign.",
							MarkdownDescription: "The ID of the object to assign.",
						},
						"object_type": schema.StringAttribute{
							Required:            true,
							Description:         "The type of the object.\nPossible values: `alert`, `allocation`, `budget`, `metric`, `report`, `annotation`",
							MarkdownDescription: "The type of the object.\nPossible values: `alert`, `allocation`, `budget`, `metric`, `report`, `annotation`",
							Validators: []validator.String{
								stringvalidator.OneOf(
									"alert",
									"allocation",
									"budget",
									"metric",
									"report",
									"annotation",
								),
							},
						},
					},
				},
			},
		},
	}
}

func (r *labelAssignmentsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan labelAssignmentsResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	assignments := r.extractAssignments(plan.Assignments, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if len(assignments) > 0 {
		apiObjs := toAPIAssignments(assignments)
		apiReq := models.AssignObjectsToLabelJSONRequestBody{
			Add: &apiObjs,
		}

		assignResp, err := r.client.AssignObjectsToLabelWithResponse(ctx, plan.LabelId.ValueString(), apiReq)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Creating Label Assignments",
				"Could not assign objects to label, unexpected error: "+err.Error(),
			)
			return
		}

		if assignResp.StatusCode() != 200 {
			resp.Diagnostics.AddError(
				"Error Creating Label Assignments",
				fmt.Sprintf("Could not assign objects to label, status: %d, body: %s", assignResp.StatusCode(), string(assignResp.Body)),
			)
			return
		}
	}

	plan.Id = plan.LabelId

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *labelAssignmentsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state labelAssignmentsResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	assignResp, err := r.client.GetLabelAssignmentsWithResponse(ctx, state.LabelId.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Label Assignments",
			"Could not read label assignments: "+err.Error(),
		)
		return
	}

	// If the label itself is gone, remove from state
	if assignResp.StatusCode() == 404 {
		resp.State.RemoveResource(ctx)
		return
	}

	if assignResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Reading Label Assignments",
			fmt.Sprintf("Unexpected status code %d: %s", assignResp.StatusCode(), string(assignResp.Body)),
		)
		return
	}

	if assignResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Label Assignments",
			"Received empty response body",
		)
		return
	}

	// Map API response to state
	state.Assignments = r.apiAssignmentsToSet(ctx, assignResp.JSON200.Assignments, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *labelAssignmentsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan labelAssignmentsResourceModel
	var state labelAssignmentsResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	oldAssignments := r.extractAssignments(state.Assignments, &resp.Diagnostics)
	newAssignments := r.extractAssignments(plan.Assignments, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	toAdd, toRemove := diffAssignments(oldAssignments, newAssignments)

	if len(toAdd) > 0 || len(toRemove) > 0 {
		apiReq := models.AssignObjectsToLabelJSONRequestBody{}

		if len(toAdd) > 0 {
			addObjs := toAPIAssignments(toAdd)
			apiReq.Add = &addObjs
		}
		if len(toRemove) > 0 {
			removeObjs := toAPIAssignments(toRemove)
			apiReq.Remove = &removeObjs
		}

		updateResp, err := r.client.AssignObjectsToLabelWithResponse(ctx, plan.LabelId.ValueString(), apiReq)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Updating Label Assignments",
				"Could not update label assignments, unexpected error: "+err.Error(),
			)
			return
		}

		if updateResp.StatusCode() != 200 {
			resp.Diagnostics.AddError(
				"Error Updating Label Assignments",
				fmt.Sprintf("Could not update label assignments, status: %d, body: %s", updateResp.StatusCode(), string(updateResp.Body)),
			)
			return
		}
	}

	plan.Id = plan.LabelId

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *labelAssignmentsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state labelAssignmentsResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	assignments := r.extractAssignments(state.Assignments, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if len(assignments) > 0 {
		apiObjs := toAPIAssignments(assignments)
		apiReq := models.AssignObjectsToLabelJSONRequestBody{
			Remove: &apiObjs,
		}

		removeResp, err := r.client.AssignObjectsToLabelWithResponse(ctx, state.LabelId.ValueString(), apiReq)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Deleting Label Assignments",
				"Could not remove objects from label, unexpected error: "+err.Error(),
			)
			return
		}

		// Treat 404 as success (label already gone)
		if removeResp.StatusCode() != 200 && removeResp.StatusCode() != 204 && removeResp.StatusCode() != 404 {
			resp.Diagnostics.AddError(
				"Error Deleting Label Assignments",
				fmt.Sprintf("Could not remove objects from label, status: %d, body: %s", removeResp.StatusCode(), string(removeResp.Body)),
			)
			return
		}
	}
}

func (r *labelAssignmentsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import ID is the label_id
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("label_id"), req.ID)...)
}

// extractAssignments converts the Terraform set to a slice of assignmentObject.
func (r *labelAssignmentsResource) extractAssignments(set types.Set, diags *diag.Diagnostics) []assignmentObject {
	if set.IsNull() || set.IsUnknown() {
		return nil
	}

	var result []assignmentObject
	elements := set.Elements()
	for _, elem := range elements {
		obj, ok := elem.(types.Object)
		if !ok {
			diags.AddError("Invalid Assignment Element", fmt.Sprintf("Expected types.Object, got %T", elem))
			return nil
		}
		attrs := obj.Attributes()

		objID, ok := attrs["object_id"].(types.String)
		if !ok {
			diags.AddError("Invalid Assignment Element", fmt.Sprintf("Expected types.String for object_id, got %T", attrs["object_id"]))
			return nil
		}
		objType, ok := attrs["object_type"].(types.String)
		if !ok {
			diags.AddError("Invalid Assignment Element", fmt.Sprintf("Expected types.String for object_type, got %T", attrs["object_type"]))
			return nil
		}

		result = append(result, assignmentObject{
			ObjectId:   objID.ValueString(),
			ObjectType: objType.ValueString(),
		})
	}
	return result
}

// apiAssignmentsToSet maps API assignments to a Terraform set value.
func (r *labelAssignmentsResource) apiAssignmentsToSet(ctx context.Context, assignments *[]models.LabelAssignmentObject, diags *diag.Diagnostics) types.Set {
	elemType := types.ObjectType{AttrTypes: assignmentAttrTypes()}

	if assignments == nil || len(*assignments) == 0 {
		emptySet, d := types.SetValue(elemType, []attr.Value{})
		diags.Append(d...)
		return emptySet
	}

	vals := make([]attr.Value, 0, len(*assignments))
	for _, a := range *assignments {
		objVal, d := types.ObjectValue(assignmentAttrTypes(), map[string]attr.Value{
			"object_id":   types.StringValue(a.ObjectId),
			"object_type": types.StringValue(string(a.ObjectType)),
		})
		diags.Append(d...)
		if diags.HasError() {
			return types.SetNull(elemType)
		}
		vals = append(vals, objVal)
	}

	setVal, d := types.SetValue(elemType, vals)
	diags.Append(d...)
	return setVal
}

// toAPIAssignments converts internal assignment objects to API types.
func toAPIAssignments(assignments []assignmentObject) []models.LabelAssignmentObject {
	result := make([]models.LabelAssignmentObject, len(assignments))
	for i, a := range assignments {
		result[i] = models.LabelAssignmentObject{
			ObjectId:   a.ObjectId,
			ObjectType: models.LabelAssignmentObjectObjectType(a.ObjectType),
		}
	}
	return result
}

// diffAssignments computes which assignments to add and remove.
func diffAssignments(old, updated []assignmentObject) (toAdd, toRemove []assignmentObject) {
	oldMap := make(map[assignmentObject]bool, len(old))
	for _, a := range old {
		oldMap[a] = true
	}

	updatedMap := make(map[assignmentObject]bool, len(updated))
	for _, a := range updated {
		updatedMap[a] = true
	}

	for _, a := range updated {
		if !oldMap[a] {
			toAdd = append(toAdd, a)
		}
	}

	for _, a := range old {
		if !updatedMap[a] {
			toRemove = append(toRemove, a)
		}
	}

	return toAdd, toRemove
}
