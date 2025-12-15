package provider

import (
	"context"
	"fmt"

	"terraform-provider-doit/internal/provider/models"
	"terraform-provider-doit/internal/provider/resource_budget"

	"github.com/hashicorp/terraform-plugin-framework/resource"
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
	_ resource.ResourceWithConfigValidators = &budgetResource{}
)

func NewBudgetResource() resource.Resource {
	return &budgetResource{}
}

// Configure adds the provider configured client to the resource.
func (r *budgetResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = client.NewClient
}

func (r *budgetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_budget"
}

func (r *budgetResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := resource_budget.BudgetResourceSchema(ctx)
	s.Version = budgetSchemaVersion
	resp.Schema = s
}

func (r *budgetResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		budgetTypeEndPeriodValidator{},
		budgetAlertsLengthValidator{},
		budgetScopeMutuallyExclusiveValidator{},
	}
}

func (r *budgetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data budgetResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Convert model to API budget type
	budget, diags := data.toUpdateRequest(ctx)
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
		errorMsg := fmt.Sprintf("Could not create budget, status: %d", budgetResp.StatusCode())
		if len(budgetResp.Body) > 0 {
			errorMsg += fmt.Sprintf(", body: %s", string(budgetResp.Body))
		}
		// Also non-retryable error
		resp.Diagnostics.AddError(
			"Error Creating Budget",
			"non-retryable error: "+errorMsg,
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

	// Map response to model
	data.Id = types.StringPointerValue(budgetResp.JSON201.Id)

	// Read full budget details (including scopes which are missing in Create response)
	resp.Diagnostics.Append(r.populateState(ctx, &data)...)
	resp.Diagnostics.Append(diags...)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *budgetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data budgetResourceModel

	// Read Terraform prior state
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Populate state
	resp.Diagnostics.Append(r.populateState(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *budgetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data budgetResourceModel

	// Read Terraform plan data
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Convert model to API budget type
	budget, diags := data.toUpdateRequest(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Set required ID for update (not present in plan data for Create, but is for Update? No, ID is in state)
	// We need to get the ID from the state, as it might not be in the plan if it's computed?
	// Actually, for Update, the ID is established.
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
		errorMsg := fmt.Sprintf("Could not update budget, status: %d", updateResp.StatusCode())
		// Try to read body for error details
		if len(updateResp.Body) > 0 {
			errorMsg += fmt.Sprintf(", body: %s", string(updateResp.Body))
		}
		resp.Diagnostics.AddError(
			"Error Updating Budget",
			errorMsg,
		)
		return
	}

	// Fetch updated budget from API and populate state
	data.Id = state.Id
	diags = r.populateState(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *budgetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data budgetResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete budget via API
	_, err := r.client.DeleteBudget(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting DoiT Budget",
			"Could not delete budget, unexpected error: "+err.Error(),
		)
		return
	}
}
