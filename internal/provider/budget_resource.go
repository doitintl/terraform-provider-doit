package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_budget"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

const budgetSchemaVersion = 1

type (
	budgetResource struct {
		client *models.ClientWithResponses
	}
	budgetResourceModel struct {
		resource_budget.BudgetModel
		Timeouts timeouts.Value `tfsdk:"timeouts"`
	}
)

// Ensure the implementation satisfies expected interfaces.
var (
	_ resource.Resource                     = (*budgetResource)(nil)
	_ resource.ResourceWithConfigure        = (*budgetResource)(nil)
	_ resource.ResourceWithUpgradeState     = (*budgetResource)(nil)
	_ resource.ResourceWithImportState      = (*budgetResource)(nil)
	_ resource.ResourceWithConfigValidators = (*budgetResource)(nil)
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
			"Unexpected Resource Configure Type",
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

	// Add UseStateForUnknown to stable Computed-only fields so they don't
	// show as "(known after apply)" on every plan that modifies the resource.
	if attr, ok := s.Attributes["id"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["id"] = attr
	}
	if attr, ok := s.Attributes["create_time"].(schema.Int64Attribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, int64planmodifier.UseStateForUnknown())
		s.Attributes["create_time"] = attr
	}

	// Classify Optional+Computed attributes (clearableattr).
	// See: https://github.com/doitintl/terraform-provider-doit/issues/233

	// Category B: API-computed defaults or legacy fields — not clearable.
	if attr, ok := s.Attributes["seasonal_amounts"].(schema.ListNestedAttribute); ok { //nolint:clearableattr // optional list, API returns empty list
		s.Attributes["seasonal_amounts"] = attr
	}
	if attr, ok := s.Attributes["currency"].(schema.StringAttribute); ok { //nolint:clearableattr // API defaults to org currency
		s.Attributes["currency"] = attr
	}
	if attr, ok := s.Attributes["scope"].(schema.ListAttribute); ok { //nolint:clearableattr // legacy alias list, API always returns a value
		s.Attributes["scope"] = attr
	}
	if attr, ok := s.Attributes["type"].(schema.StringAttribute); ok { //nolint:clearableattr // API defaults budget type
		s.Attributes["type"] = attr
	}
	if attr, ok := s.Attributes["time_interval"].(schema.StringAttribute); ok { //nolint:clearableattr // API defaults time interval
		s.Attributes["time_interval"] = attr
	}
	if attr, ok := s.Attributes["amount"].(schema.Float64Attribute); ok { //nolint:clearableattr // API-computed for use_prev_spend budgets
		s.Attributes["amount"] = attr
	}
	if attr, ok := s.Attributes["public"].(schema.BoolAttribute); ok { //nolint:clearableattr // API defaults to false
		s.Attributes["public"] = attr
	}
	if attr, ok := s.Attributes["start_period"].(schema.Int64Attribute); ok { //nolint:clearableattr // API defaults to current period
		s.Attributes["start_period"] = attr
	}

	// Category B: nested recipients_slack_channels — API-populated Slack metadata.
	if slackAttr, ok := s.Attributes["recipients_slack_channels"].(schema.ListNestedAttribute); ok {
		if attr, ok := slackAttr.NestedObject.Attributes["customer_id"].(schema.StringAttribute); ok { //nolint:clearableattr // API-populated
			slackAttr.NestedObject.Attributes["customer_id"] = attr
		}
		if attr, ok := slackAttr.NestedObject.Attributes["id"].(schema.StringAttribute); ok { //nolint:clearableattr // API-populated
			slackAttr.NestedObject.Attributes["id"] = attr
		}
		if attr, ok := slackAttr.NestedObject.Attributes["name"].(schema.StringAttribute); ok { //nolint:clearableattr // API-populated
			slackAttr.NestedObject.Attributes["name"] = attr
		}
		if attr, ok := slackAttr.NestedObject.Attributes["shared"].(schema.BoolAttribute); ok { //nolint:clearableattr // API-populated
			slackAttr.NestedObject.Attributes["shared"] = attr
		}
		if attr, ok := slackAttr.NestedObject.Attributes["type"].(schema.StringAttribute); ok { //nolint:clearableattr // API-populated
			slackAttr.NestedObject.Attributes["type"] = attr
		}
		if attr, ok := slackAttr.NestedObject.Attributes["workspace"].(schema.StringAttribute); ok { //nolint:clearableattr // API-populated
			slackAttr.NestedObject.Attributes["workspace"] = attr
		}
		s.Attributes["recipients_slack_channels"] = slackAttr
	}

	// Category B: nested collaborators — identity/API defaults.
	if collabAttr, ok := s.Attributes["collaborators"].(schema.ListNestedAttribute); ok {
		if attr, ok := collabAttr.NestedObject.Attributes["email"].(schema.StringAttribute); ok { //nolint:clearableattr // identity field
			collabAttr.NestedObject.Attributes["email"] = attr
		}
		if attr, ok := collabAttr.NestedObject.Attributes["role"].(schema.StringAttribute); ok { //nolint:clearableattr // API defaults role
			collabAttr.NestedObject.Attributes["role"] = attr
		}
		s.Attributes["collaborators"] = collabAttr
	}

	// Category B: nested alerts — identity field.
	if alertsAttr, ok := s.Attributes["alerts"].(schema.ListNestedAttribute); ok {
		if attr, ok := alertsAttr.NestedObject.Attributes["percentage"].(schema.Float64Attribute); ok { //nolint:clearableattr // identity field within list
			alertsAttr.NestedObject.Attributes["percentage"] = attr
		}
		s.Attributes["alerts"] = alertsAttr
	}

	// Category B: API-defaulted — not clearable.
	if attr, ok := s.Attributes["name"].(schema.StringAttribute); ok { //nolint:clearableattr // API auto-generates name when omitted
		s.Attributes["name"] = attr
	}
	if attr, ok := s.Attributes["end_period"].(schema.Int64Attribute); ok { //nolint:clearableattr // API rejects clearing (endPeriod=0/null); only clears implicitly via type→recurring
		s.Attributes["end_period"] = attr
	}
	if attr, ok := s.Attributes["recipients"].(schema.ListAttribute); ok { //nolint:clearableattr // API auto-assigns creator's email on create
		s.Attributes["recipients"] = attr
	}

	// Nested scopes classification.
	if scopesAttr, ok := s.Attributes["scopes"].(schema.ListNestedAttribute); ok {
		if attr, ok := scopesAttr.NestedObject.Attributes["inverse"].(schema.BoolAttribute); ok { //nolint:clearableattr // API defaults to false
			scopesAttr.NestedObject.Attributes["inverse"] = attr
		}
		if attr, ok := scopesAttr.NestedObject.Attributes["values"].(schema.ListAttribute); ok {
			attr.PlanModifiers = append(attr.PlanModifiers, useNullForUnknownListWhenConfigNull())
			scopesAttr.NestedObject.Attributes["values"] = attr
		}
		s.Attributes["scopes"] = scopesAttr
	}

	s.Attributes["timeouts"] = timeouts.Attributes(ctx, timeouts.Opts{
		Create: true,
		Read:   true,
		Update: true,
		Delete: true,
	})

	resp.Schema = s
}

func (r *budgetResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		budgetTypeEndPeriodValidator{},
		budgetAlertsLengthValidator{},
		budgetRecipientsMinLengthValidator{},
		budgetScopeMutuallyExclusiveValidator{},
		budgetCollaboratorsOwnerValidator{},
		// Warn when legacy [... N/A] NullFallback sentinels are used in scope values.
		budgetScopeNAValidator{},
	}
}

func (r *budgetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan budgetResourceModel

	// Read Terraform plan data into the model
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

	// Plan-first state pattern: keep all user-configured values from the plan
	// exactly as-is, and only overlay Computed-only fields from the API response.
	// This prevents "Provider produced inconsistent result" errors caused by the
	// API normalizing user-provided values (stripping sentinels, renaming types).
	// Read and ImportState still use mapBudgetToModel for the full API response.
	resp.Diagnostics.Append(overlayBudgetComputedFields(ctx, budgetResp.JSON201, &plan)...)
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

	readTimeout, diags := state.Timeouts.Read(ctx, 2*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

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

	updateTimeout, diags := plan.Timeouts.Update(ctx, 5*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

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

	// Plan-first state pattern: preserve user's plan values, overlay computed fields only.
	plan.Id = state.Id
	resp.Diagnostics.Append(overlayBudgetComputedFields(ctx, updateResp.JSON200, &plan)...)
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

	deleteTimeout, diags := state.Timeouts.Delete(ctx, 2*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

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
