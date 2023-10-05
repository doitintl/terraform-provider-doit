package provider

import (
	"context"

	"fmt"
	"log"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
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
	client *ClientTest
}

// Metadata returns the resource type name.
func (r *attributionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	log.Print("hello attribution Metadata:)")
	resp.TypeName = req.ProviderTypeName + "_attribution"
}

// Schema defines the schema for the resource.
func (r *attributionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	log.Print("hello attribution Schema:)")
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"last_updated": schema.StringAttribute{
				Computed: true,
			},
			"name": schema.StringAttribute{
				Required: true,
			},
			"description": schema.StringAttribute{
				Required: true,
			},
			"formula": schema.StringAttribute{
				Required: true,
			},
			"components": schema.ListNestedAttribute{
				Required: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"type": schema.StringAttribute{
							Required: true,
						},
						"key": schema.StringAttribute{
							Required: true,
						},
						"values": schema.ListAttribute{
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
	log.Print("hello attribution Configure:)")
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*ClientTest)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *ClientTest, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

// Create creates the resource and sets the initial Terraform state.
func (r *attributionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	log.Println("hello attribution Create:)")
	log.Println(r.client.Auth.DoiTAPITOken)
	log.Println("---------------------------------------------------")
	log.Println(r.client.Auth.CustomerContext)

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
	attributionResponse, err := r.client.CreateAttribution(attribution)
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
	log.Print("hello attribution Read:)")
	// Get current state
	var state attributionResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	log.Print("state id::::::::::::::::::::::::::)")
	log.Print(state.Id.ValueString())
	// Get refreshed attribution value from DoiT
	attribution, err := r.client.GetAttribution(state.Id.ValueString())
	if err != nil {
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
	log.Print("state read::::::::::::::::::::::::::)")
	log.Print(state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *attributionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	log.Print("hello attribution Update:)")
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
	_, err := r.client.UpdateAttribution(state.Id.ValueString(), attribution)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating DoiT Attribution",
			"Could not update attribution, unexpected error: "+err.Error(),
		)
		return
	}

	// Fetch updated items from GetAttribution as UpdateAttribution items are not
	// populated.
	attributionResponse, err := r.client.GetAttribution(state.Id.ValueString())
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
	log.Print("hello attribution Delete:)")
	// Retrieve values from state
	var state attributionResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete existing attribution
	err := r.client.DeleteAttribution(state.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting DoiT Attribution",
			"Could not delete attribution, unexpected error: "+err.Error(),
		)
		return
	}
}
