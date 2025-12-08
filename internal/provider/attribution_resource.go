package provider

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// orderResourceModel maps the resource schema data.
type attributionResourceModel struct {
	Id          types.String               `tfsdk:"id"`
	Name        types.String               `tfsdk:"name"`
	Description types.String               `tfsdk:"description"`
	Formula     types.String               `tfsdk:"formula"`
	Components  []attibutionComponentModel `tfsdk:"components"`
	LastUpdated types.String               `tfsdk:"last_updated"`
}

// orderComponentModel maps order item data.
type attibutionComponentModel struct {
	TypeComponent types.String   `tfsdk:"type"`
	Key           types.String   `tfsdk:"key"`
	Values        []types.String `tfsdk:"values"`
}

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource              = &attributionResource{}
	_ resource.ResourceWithConfigure = &attributionResource{}
)

// NewattributionResource is a helper function to simplify the provider implementation.
func NewAttributionResource() resource.Resource {
	return &attributionResource{}
}

// attributionResource is the resource implementation.
type attributionResource struct {
	client *Client
}

// Metadata returns the resource type name.
func (r *attributionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	log.Print(" attribution Metadata")
	resp.TypeName = req.ProviderTypeName + "_attribution"
}

// Schema defines the schema for the resource.
func (r *attributionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	log.Print(" attribution Schema")
	resp.Schema = schema.Schema{
		Description:         "Attributions allow you to group and filter costs based on dimensions like projects, labels, and more.",
		MarkdownDescription: "Attributions allow you to group and filter costs based on dimensions like projects, labels, and more.",
		DeprecationMessage: `Attributions and the associated resources are deprecated and no longer supported.
Please consider switching to allocations before upgrading to the next major version of the provider to prevent breaking changes.
The attribution resource will be removed in the next major version of the provider.`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "attribution ID, identifying the attribution",
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
				Description: "Attribution Name",
				Required:    true,
			},
			"description": schema.StringAttribute{
				Description: "Attribution description",
				Optional:    true,
			},
			"formula": schema.StringAttribute{
				Description: "Attribution formula (A is first component, B is second component, C is third component, etc.)",
				Optional:    true,
			},
			"components": schema.ListNestedAttribute{
				Description: "List of Attribution filters",
				Required:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"type": schema.StringAttribute{
							Description: `Type of the component. Possible values are:
'datetime'
'fixed' (Used for AWS account IDs and Google project IDs)
'optional'
'label'
'tag'
'project_label'
'system_label'
'attribution'
'attribution_group'
'gke'
'gke_label'`,
							Required: true,
						},
						"key": schema.StringAttribute{
							Description: "Key of a dimension. Examples: \"service_id\", \"cloud_provider\", \"sku_description\"",
							Required:    true,
						},
						"values": schema.ListAttribute{
							Description: "Value of the dimension. Examples: \"152E-C115-5142\", \"google-cloud\", \"team-a\"",
							Required:    true,
							ElementType: types.StringType,
						},
					},
				},
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *attributionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	log.Print(" attribution Configure")
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*Clients)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *Clients, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client.OldClient
}

// Create creates the resource and sets the initial Terraform state.
func (r *attributionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	resp.Diagnostics.Append(
		diag.NewWarningDiagnostic("doit_attribution resource is deprecated", "use doit_allocation instead"),
	)
	log.Println(" attribution Create")

	// Retrieve values from plan
	var plan attributionResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Generate API request body from plan
	var attribution Attribution
	attribution.Description = plan.Description.ValueString()
	attribution.Name = plan.Name.ValueString()
	attribution.Formula = plan.Formula.ValueString()
	var components []Component

	for _, component := range plan.Components {
		var values []string
		for _, value := range component.Values {
			values = append(values, value.ValueString())
		}
		components = append(components, Component{
			TypeComponent: component.TypeComponent.ValueString(),
			Key:           component.Key.ValueString(),
			Values:        values})
	}
	attribution.Components = components
	log.Println("attribution---------------------------------------------------")
	log.Println(attribution)

	// Create new attribution
	attributionResponse, err := r.client.CreateAttribution(ctx, attribution)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating attribution",
			"Could not create attribution, unexpected error: "+err.Error(),
		)
		return
	}
	log.Println("attribution id---------------------------------------------------")
	log.Println(attributionResponse.Id)
	plan.Id = types.StringValue(attributionResponse.Id)
	plan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

}

// Read refreshes the Terraform state with the latest data.
func (r *attributionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	resp.Diagnostics.Append(
		diag.NewWarningDiagnostic("doit_attribution resource is deprecated", "use doit_allocation instead"),
	)
	log.Print(" attribution Read")
	// Get current state
	var state attributionResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	log.Print("state id:::::::::::::::::::::::::")
	log.Print(state.Id.ValueString())
	// Get refreshed attribution value from DoiT
	attribution, err := r.client.GetAttribution(ctx, state.Id.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Doit Console Attribution",
			"Could not read Doit Console Attribution ID "+state.Id.ValueString()+": "+err.Error(),
		)
		return
	}
	state.Id = types.StringValue(attribution.Id)
	state.Description = types.StringValue(attribution.Description)
	state.Formula = types.StringValue(attribution.Formula)
	state.Name = types.StringValue(attribution.Name)

	// Overwrite components with refreshed state
	state.Components = []attibutionComponentModel{}
	for _, component := range attribution.Components {
		values := []types.String{}
		for _, value := range component.Values {
			values = append(values, types.StringValue(value))
		}
		state.Components = append(state.Components, attibutionComponentModel{
			TypeComponent: types.StringValue(component.TypeComponent),
			Key:           types.StringValue(component.Key),
			Values:        values,
		})
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

func (r *attributionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.Append(
		diag.NewWarningDiagnostic("doit_attribution resource is deprecated", "use doit_allocation instead"),
	)
	log.Print("attribution Update")
	// Retrieve values from plan
	var plan attributionResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state attributionResourceModel
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Generate API request body from plan
	// Generate API request body from plan
	var attribution Attribution
	attribution.Id = state.Id.ValueString()
	attribution.Description = plan.Description.ValueString()
	attribution.Name = plan.Name.ValueString()
	attribution.Formula = plan.Formula.ValueString()
	var components []Component

	for _, component := range plan.Components {
		var values []string
		for _, value := range component.Values {
			values = append(values, value.ValueString())
		}
		components = append(components, Component{
			TypeComponent: component.TypeComponent.ValueString(),
			Key:           component.Key.ValueString(),
			Values:        values})
	}
	attribution.Components = components
	log.Println("attribution---------------------------------------------------")
	log.Println(attribution)

	// Update existing attribution
	_, err := r.client.UpdateAttribution(ctx, state.Id.ValueString(), attribution)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating DoiT Attribution",
			"Could not update attribution, unexpected error: "+err.Error(),
		)
		return
	}

	// Fetch updated items from GetAttribution as UpdateAttribution items are not
	// populated.
	attributionResponse, err := r.client.GetAttribution(ctx, state.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Doit Console Attribution",
			"Could not read Doit Console attribution ID "+plan.Id.ValueString()+": "+err.Error(),
		)
		return
	}

	// Update resource state with updated items and timestamp
	plan.Id = types.StringValue(attributionResponse.Id)
	plan.Description = types.StringValue(attributionResponse.Description)
	plan.Formula = types.StringValue(attributionResponse.Formula)
	plan.Name = types.StringValue(attributionResponse.Name)
	plan.Components = []attibutionComponentModel{}
	for _, component := range attributionResponse.Components {
		values := []types.String{}
		for _, value := range component.Values {
			values = append(values, types.StringValue(value))
		}
		plan.Components = append(plan.Components, attibutionComponentModel{
			TypeComponent: types.StringValue(component.TypeComponent),
			Key:           types.StringValue(component.Key),
			Values:        values,
		})
	}
	plan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.

func (r *attributionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.Append(
		diag.NewWarningDiagnostic("doit_attribution resource is deprecated", "use doit_allocation instead"),
	)
	log.Print(" attribution Delete")
	// Retrieve values from state
	var state attributionResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete existing attribution
	err := r.client.DeleteAttribution(ctx, state.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting DoiT Attribution",
			"Could not delete attribution, unexpected error: "+err.Error(),
		)
		return
	}
}
