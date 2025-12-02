package provider

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// orderResourceModel maps the resource schema data.
type attributionGroupResourceModel struct {
	Id           types.String   `tfsdk:"id"`
	Name         types.String   `tfsdk:"name"`
	Description  types.String   `tfsdk:"description"`
	Attributions []types.String `tfsdk:"attributions"`
	LastUpdated  types.String   `tfsdk:"last_updated"`
}

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource              = &attributionGroupResource{}
	_ resource.ResourceWithConfigure = &attributionGroupResource{}
)

// NewAttributionGroupResource is a helper function to simplify the provider implementation.
func NewAttributionGroupResource() resource.Resource {
	return &attributionGroupResource{}
}

// attributionGroupResource is the resource implementation.
type attributionGroupResource struct {
	client *Client
}

// Metadata returns the resource type name.
func (r *attributionGroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	log.Println("attribution group Metadata")
	resp.TypeName = req.ProviderTypeName + "_attribution_group"
}

// Schema defines the schema for the resource.
func (r *attributionGroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	log.Print("attributionGroup Schema")
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Attribution group ID",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"last_updated": schema.StringAttribute{
				Description: "Timestamp of the last Terraform update of" +
					"the attribution group.",
				Computed: true,
			},
			"name": schema.StringAttribute{
				Description: "Name of the attribution group",
				Required:    true,
			},
			"description": schema.StringAttribute{
				Description: "Description of the attribution group",
				Optional:    true,
			},
			"attributions": schema.ListAttribute{
				Description: "List of the attributions that are part of the attribution group",
				Required:    true,
				ElementType: types.StringType,
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *attributionGroupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	log.Print("attributionGroup Configure")
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*Client)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

// Create creates the resource and sets the initial Terraform state.
func (r *attributionGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	log.Println("attributionGroup Create")

	// Retrieve values from plan
	var plan attributionGroupResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Generate API request body from plan
	var attributionGroup AttributionGroup
	attributionGroup.Description = plan.Description.ValueString()
	attributionGroup.Name = plan.Name.ValueString()
	var attributions []string
	for _, attribution := range plan.Attributions {
		attributions = append(attributions, attribution.ValueString())
	}
	attributionGroup.Attributions = attributions
	log.Println("attributionGroup---------------------------------------------------")
	log.Println(attributionGroup)

	// Create new attributionGroup
	attributionGroupResponse, err := r.client.CreateAttributionGroup(ctx, attributionGroup)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating attributionGrouppp",
			"Could not create attributionGroup, unexpected error: "+err.Error(),
		)
		return
	}
	log.Println("attributionGroup id---------------------------------------------------")
	log.Println(attributionGroupResponse.Id)
	plan.Id = types.StringValue(attributionGroupResponse.Id)
	plan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

}

// Read refreshes the Terraform state with the latest data.
func (r *attributionGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	log.Print("attributionGroup Read")
	// Get current state
	var state attributionGroupResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	log.Print("state id")
	log.Print(state.Id.ValueString())
	// Get refreshed attributionGroup value from DoiT
	attributionGroup, err := r.client.GetAttributionGroup(ctx, state.Id.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Doit Console AttributionGroup",
			"Could not read Doit Console AttributionGroup ID "+state.Id.ValueString()+": "+err.Error(),
		)
		return
	}
	// state.Id = types.StringValue(attributionGroup.Id)
	state.Description = types.StringValue(attributionGroup.Description)
	state.Name = types.StringValue(attributionGroup.Name)

	// Overwrite components with refreshed state
	state.Attributions = []types.String{}
	for _, attribution := range attributionGroup.Attributions {
		state.Attributions = append(state.Attributions, types.StringValue(attribution))
	}

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	log.Print("state read")
	log.Print(state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *attributionGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	log.Println("attributionGroup Update")
	// Retrieve values from plan
	var plan attributionGroupResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state attributionGroupResourceModel
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Generate API request body from plan
	// Generate API request body from plan
	var attributionGroup AttributionGroup
	attributionGroup.Id = state.Id.ValueString()
	attributionGroup.Description = plan.Description.ValueString()
	attributionGroup.Name = plan.Name.ValueString()
	var attributions []string

	for _, attribution := range plan.Attributions {
		attributions = append(attributions, attribution.ValueString())
	}
	attributionGroup.Attributions = attributions
	log.Println("attributionGroup")
	log.Println(attributionGroup)

	// Update existing attributionGroup
	_, err := r.client.UpdateAttributionGroup(ctx, state.Id.ValueString(), attributionGroup)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating DoiT AttributionGroup",
			"Could not update attributionGroup, unexpected error: "+err.Error(),
		)
		return
	}

	// Fetch updated items from GetAttributionGroup as UpdateAttributionGroup items are not
	// populated.
	attributionGroupResponse, err := r.client.GetAttributionGroup(ctx, state.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Doit Console AttributionGroup",
			"Could not read Doit Console attributionGroup ID "+plan.Id.ValueString()+": "+err.Error(),
		)
		return
	}

	// Update resource state with updated items and timestamp
	plan.Id = types.StringValue(attributionGroupResponse.Id)
	plan.Description = types.StringValue(attributionGroupResponse.Description)
	plan.Name = types.StringValue(attributionGroupResponse.Name)
	plan.Attributions = []types.String{}
	for _, attribution := range attributionGroupResponse.Attributions {
		plan.Attributions = append(plan.Attributions, types.StringValue(attribution))
	}
	plan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.

func (r *attributionGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	log.Println("attributionGroup Delete")
	// Retrieve values from state
	var state attributionGroupResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete existing attributionGroup
	err := r.client.DeleteAttributionGroup(ctx, state.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting DoiT AttributionGroup",
			"Could not delete attributionGroup, unexpected error: "+err.Error(),
		)
		return
	}
}
