package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_budget"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const budgetSchemaVersion = 1

type (
	budgetResource struct {
		client *models.ClientWithResponses
	}
	budgetResourceModel struct {
		resource_budget.BudgetModel
	}
)

// Ensure the implementation satisfies expected interfaces.
var (
	_ resource.Resource                     = &budgetResource{}
	_ resource.ResourceWithConfigure        = &budgetResource{}
	_ resource.ResourceWithUpgradeState     = &budgetResource{}
	_ resource.ResourceWithImportState      = &budgetResource{}
	_ resource.ResourceWithConfigValidators = &budgetResource{}
)

// NewBudgetResource creates a new budget resource instance.
func NewBudgetResource() resource.Resource {
	return &budgetResource{}
}

// Configure adds the provider configured client to the resource.
func (r *budgetResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*models.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *models.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *budgetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_budget"
}

func (r *budgetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *budgetResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := resource_budget.BudgetResourceSchema(ctx)
	s.Version = budgetSchemaVersion

	// Add manual validator for end_period
	if endPeriod, ok := s.Attributes["end_period"]; ok {
		// We need to type assert to the specific attribute type to access Validators field
		// end_period is an Int64Attribute
		if int64Attr, ok := endPeriod.(schema.Int64Attribute); ok {
			int64Attr.Validators = append(int64Attr.Validators, budgetEndPeriodValidator{})
			s.Attributes["end_period"] = int64Attr
		}
	}

	resp.Schema = s
}

func (r *budgetResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		budgetTypeEndPeriodValidator{},
		budgetAlertsLengthValidator{},
		budgetScopeMutuallyExclusiveValidator{},
	}
}

func (r *budgetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan budgetResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Convert model to API budget type
	budget, diags := plan.toUpdateRequest(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create new budget via API
	budgetResp, err := r.client.CreateBudgetWithResponse(ctx, budget)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Budget",
			"Could not create budget, unexpected error: "+err.Error(),
		)
		return
	}

	if budgetResp.StatusCode() != 201 {
		resp.Diagnostics.AddError(
			"Error Creating Budget",
			fmt.Sprintf("Could not create budget, status: %d, body: %s", budgetResp.StatusCode(), string(budgetResp.Body)),
		)
		return
	}

	if budgetResp.JSON201 == nil {
		resp.Diagnostics.AddError(
			"Error creating budget",
			"Could not create budget, empty response",
		)
		return
	}

	// Map response directly to state
	plan.Id = types.StringPointerValue(budgetResp.JSON201.Id)
	resp.Diagnostics.Append(mapBudgetToModel(ctx, budgetResp.JSON201, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *budgetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state budgetResourceModel

	// Read Terraform prior state
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Populate state
	resp.Diagnostics.Append(r.populateState(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Handle externally deleted resource (populateState sets Id to null on 404)
	if state.Id.IsNull() {
		resp.State.RemoveResource(ctx)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *budgetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan budgetResourceModel

	// Read Terraform plan data
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Convert model to API budget type
	budget, diags := plan.toUpdateRequest(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get the ID from the state
	var state budgetResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	budgetID := state.Id.ValueString()

	// Update budget via API
	updateResp, err := r.client.UpdateBudgetWithResponse(ctx, budgetID, budget)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Budget",
			"Could not update budget, unexpected error: "+err.Error(),
		)
		return
	}

	if updateResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Updating Budget",
			fmt.Sprintf("Could not update budget, status: %d, body: %s", updateResp.StatusCode(), string(updateResp.Body)),
		)
		return
	}

	// Map response directly to state
	if updateResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Updating Budget",
			"Received empty response body",
		)
		return
	}

	plan.Id = state.Id
	diags = mapBudgetToModel(ctx, updateResp.JSON200, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *budgetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state budgetResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete budget via API
	deleteResp, err := r.client.DeleteBudgetWithResponse(ctx, state.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting DoiT Budget",
			"Could not delete budget, unexpected error: "+err.Error(),
		)
		return
	}

	// Treat 404 as success - resource is already gone (deleted outside Terraform)
	if deleteResp.StatusCode() != 200 && deleteResp.StatusCode() != 204 && deleteResp.StatusCode() != 404 {
		resp.Diagnostics.AddError(
			"Error Deleting DoiT Budget",
			fmt.Sprintf("Could not delete budget, status: %d, body: %s", deleteResp.StatusCode(), string(deleteResp.Body)),
		)
		return
	}
}
