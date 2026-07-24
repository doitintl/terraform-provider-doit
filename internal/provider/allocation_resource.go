package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_allocation"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

type (
	allocationResource struct {
		client *models.ClientWithResponses
	}
	allocationResourceModel struct {
		resource_allocation.AllocationModel
		Timeouts timeouts.Value `tfsdk:"timeouts"`
	}
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                     = (*allocationResource)(nil)
	_ resource.ResourceWithConfigure        = (*allocationResource)(nil)
	_ resource.ResourceWithImportState      = (*allocationResource)(nil)
	_ resource.ResourceWithConfigValidators = (*allocationResource)(nil)
	_ resource.ResourceWithModifyPlan       = (*allocationResource)(nil)
)

// NewAllocationResource creates a new allocation resource instance.
func NewAllocationResource() resource.Resource {
	return &allocationResource{}
}

// Configure adds the provider configured client to the resource.
func (r *allocationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *allocationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *allocationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_allocation"
}

func (r *allocationResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := resource_allocation.AllocationResourceSchema(ctx)

	// Inject validator for rules attribute to enforce 'name' is required for 'create'/'update' actions.
	// See allocationRulesValidator for context on why this workaround is needed.
	if rules, ok := s.Attributes["rules"]; ok {
		if listAttr, ok := rules.(schema.ListNestedAttribute); ok {
			listAttr.Validators = append(listAttr.Validators, allocationRulesValidator{})

			// Also inject components validator into rules[].components
			if components, ok := listAttr.NestedObject.Attributes["components"]; ok {
				if compListAttr, ok := components.(schema.ListNestedAttribute); ok {
					compListAttr.Validators = append(compListAttr.Validators, allocationComponentsValidator{})
					listAttr.NestedObject.Attributes["components"] = compListAttr
				}
			}

			s.Attributes["rules"] = listAttr
		}
	}

	// Inject components validator into rule.components
	if rule, ok := s.Attributes["rule"]; ok {
		if singleAttr, ok := rule.(schema.SingleNestedAttribute); ok {
			if components, ok := singleAttr.Attributes["components"]; ok {
				if compListAttr, ok := components.(schema.ListNestedAttribute); ok {
					compListAttr.Validators = append(compListAttr.Validators, allocationComponentsValidator{})
					singleAttr.Attributes["components"] = compListAttr
				}
			}
			s.Attributes["rule"] = singleAttr
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
	if attr, ok := s.Attributes["allocation_type"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["allocation_type"] = attr
	}
	if attr, ok := s.Attributes["anomaly_detection"].(schema.BoolAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, boolplanmodifier.UseStateForUnknown())
		s.Attributes["anomaly_detection"] = attr
	}
	if attr, ok := s.Attributes["type"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["type"] = attr
	}

	// Category B: rule nested attributes are rule-type-dependent.
	// For "select" rules, name/description/formula are API-computed from the
	// source allocation. For "create" rules, they are user-authored. Since
	// schema-level modifiers fire at plan time and can't distinguish rule types,
	// we must classify as Cat B to avoid drift on select rules. This means users
	// cannot clear these fields on create rules via config removal — they must
	// explicitly set them to "" instead.
	acknowledgeNotClearable(s,
		"rules[*].id",          // API-assigned on create, preserved when config null
		"rules[*].name",        // API-computed for select rules
		"rules[*].description", // API-computed for select rules
		"rules[*].formula",     // API-computed for select rules
	)

	// Category A: user-authored label for unallocated costs in group allocations.
	if attr, ok := s.Attributes["unallocated_costs"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, useEmptyForUnknownWhenConfigNull())
		s.Attributes["unallocated_costs"] = attr
	}

	s.Attributes["timeouts"] = timeouts.Attributes(ctx, timeouts.Opts{
		Create: true,
		Read:   true,
		Update: true,
		Delete: true,
	})

	resp.Schema = s
}

func (r *allocationResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(
			path.MatchRoot("rule"),
			path.MatchRoot("rules"),
		),
		resourcevalidator.RequiredTogether(
			path.MatchRoot("rules"),
			path.MatchRoot("unallocated_costs"),
		),
		resourcevalidator.Conflicting(
			path.MatchRoot("rule"),
			path.MatchRoot("unallocated_costs"),
		),
	}
}

func (r *allocationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan allocationResourceModel

	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
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

	// Generate API request body from state
	allocationReq, diags := plan.toCreateRequest(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save state into Terraform state
	allocationResp, err := r.client.CreateAllocationWithResponse(ctx, allocationReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating allocation",
			"Could not create allocation, unexpected error: "+err.Error(),
		)
		return
	}

	if allocationResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error creating allocation",
			fmt.Sprintf("Could not create allocation, status: %d, body: %s", allocationResp.StatusCode(), string(allocationResp.Body)),
		)
		return
	}

	if allocationResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error creating allocation",
			"Could not create allocation, empty response",
		)
		return
	}

	if allocationResp.JSON200.Id == nil {
		resp.Diagnostics.AddError(
			"Error creating allocation",
			"Could not create allocation, response missing ID",
		)
		return
	}

	// Plan-first state pattern: keep all user-configured values from the plan
	// exactly as-is, and only overlay Computed-only fields from the API response.
	// This prevents "Provider produced inconsistent result" errors caused by the
	// API normalizing user-provided values (stripping sentinels, renaming services).
	// Read and ImportState still use mapAllocationToModel for the full API response.
	resp.Diagnostics.Append(r.overlayAllocationComputedFields(ctx, allocationResp.JSON200, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *allocationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state allocationResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
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

	diags = r.populateState(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Handle externally deleted resource (populateState sets Id to null on 404)
	if state.Id.IsNull() {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *allocationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan allocationResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
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

	// Generate API request body from plan
	allocation, diags := plan.toUpdateRequest(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// We need the allocation ID from state for the API call.
	var stateId types.String
	diags = req.State.GetAttribute(ctx, path.Root("id"), &stateId)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if stateId.IsNull() || stateId.IsUnknown() {
		resp.Diagnostics.AddError(
			"Error updating allocation",
			"Could not update allocation because the resource ID in state is null or unknown.",
		)
		return
	}

	// Update the allocation
	updateResp, err := r.client.UpdateAllocationWithResponse(ctx, stateId.ValueString(), allocation)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating allocation",
			"Could not update allocation, unexpected error: "+err.Error(),
		)
		return
	}

	if updateResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error updating allocation",
			fmt.Sprintf("Could not update allocation, status: %d, body: %s", updateResp.StatusCode(), string(updateResp.Body)),
		)
		return
	}

	if updateResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error updating allocation",
			"Could not update allocation, empty response",
		)
		return
	}

	if updateResp.JSON200.Id == nil {
		resp.Diagnostics.AddError(
			"Error updating allocation",
			"Could not update allocation, response missing ID",
		)
		return
	}

	// Plan-first state pattern: keep all user-configured values from the plan
	// exactly as-is, and only overlay Computed-only fields from the API response.
	resp.Diagnostics.Append(r.overlayAllocationComputedFields(ctx, updateResp.JSON200, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *allocationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state allocationResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
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

	deleteResp, err := r.client.DeleteAllocationWithResponse(ctx, state.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting DoiT Allocation",
			"Could not delete allocation, unexpected error: "+err.Error(),
		)
		return
	}

	// Treat 404 as success - resource is already gone (deleted outside Terraform)
	if deleteResp.StatusCode() != 200 && deleteResp.StatusCode() != 204 && deleteResp.StatusCode() != 404 {
		resp.Diagnostics.AddError(
			"Error Deleting DoiT Allocation",
			fmt.Sprintf("Could not delete allocation, status: %d, body: %s", deleteResp.StatusCode(), string(deleteResp.Body)),
		)
		return
	}
}

// ModifyPlan implements identity-aware rule ID matching for group allocation in-line rules.
// This matches in-line plan rules (action="create" or "update" without explicit ID) to existing
// state in-line rules by name (and position fallback for renames) so that rule IDs are carried over
// safely without positional/index corruption when rules are reordered or removed.
func (r *allocationResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// Skip if state or plan is null (e.g. resource creation or deletion)
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}

	var stateModel allocationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &stateModel)...)
	var planModel allocationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planModel)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Skip if rules list in state or plan is null or unknown
	if stateModel.Rules.IsNull() || stateModel.Rules.IsUnknown() ||
		planModel.Rules.IsNull() || planModel.Rules.IsUnknown() {
		return
	}

	var stateRules []resource_allocation.RulesValue
	resp.Diagnostics.Append(stateModel.Rules.ElementsAs(ctx, &stateRules, false)...)
	var planRules []resource_allocation.RulesValue
	resp.Diagnostics.Append(planModel.Rules.ElementsAs(ctx, &planRules, true)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Collect in-line state rule candidates.
	// Only in-line state rules (action == "create" or "update" with known non-empty Id)
	// can be matched to in-line plan rules. "select" rules reference standalone allocations
	// and are excluded.
	type inlineStateRule struct {
		id      string
		name    string
		formula string
		used    bool
	}

	var inlineCandidates []inlineStateRule
	for _, sRule := range stateRules {
		if sRule.Id.IsNull() || sRule.Id.IsUnknown() || sRule.Id.ValueString() == "" {
			continue
		}
		if sRule.Action.ValueString() == "select" {
			continue
		}
		inlineCandidates = append(inlineCandidates, inlineStateRule{
			id:      sRule.Id.ValueString(),
			name:    sRule.Name.ValueString(),
			formula: sRule.Formula.ValueString(),
			used:    false,
		})
	}

	isInlineRuleNeedingID := func(pRule resource_allocation.RulesValue) bool {
		action := pRule.Action.ValueString()
		if action != "create" && action != "update" {
			return false
		}
		if !pRule.Id.IsNull() && !pRule.Id.IsUnknown() && pRule.Id.ValueString() != "" {
			return false
		}
		return true
	}

	planModified := false

	// Pass 1: Exact Name Match
	for i := range planRules {
		if !isInlineRuleNeedingID(planRules[i]) {
			continue
		}
		planName := planRules[i].Name.ValueString()
		if planName == "" {
			continue
		}

		for cIdx := range inlineCandidates {
			cand := &inlineCandidates[cIdx]
			if !cand.used && cand.name == planName {
				cand.used = true
				planRules[i].Id = types.StringValue(cand.id)
				planModified = true
				break
			}
		}
	}

	// Pass 2: Formula-Based Match for Renamed Rules
	// When a rule is renamed but keeps the same formula, Pass 1 (name match) misses it.
	// Match by formula instead of list position, which breaks when rules are deleted or
	// reordered. Only match when a formula appears exactly once on each side (unambiguous).
	//
	// Gap: if a user renames AND changes the formula simultaneously, neither pass matches
	// and the rule is treated as new (delete + create). Provide an explicit id in HCL to
	// force an in-place update in that case.
	planByFormula := map[string][]int{}
	for i := range planRules {
		if !isInlineRuleNeedingID(planRules[i]) {
			continue
		}
		f := planRules[i].Formula.ValueString()
		if f == "" || planRules[i].Formula.IsNull() || planRules[i].Formula.IsUnknown() {
			continue
		}
		planByFormula[f] = append(planByFormula[f], i)
	}

	stateByFormula := map[string][]int{}
	for cIdx := range inlineCandidates {
		c := &inlineCandidates[cIdx]
		if c.used || c.formula == "" {
			continue
		}
		stateByFormula[c.formula] = append(stateByFormula[c.formula], cIdx)
	}

	for formula, pIdxs := range planByFormula {
		sIdxs, ok := stateByFormula[formula]
		if !ok || len(pIdxs) != 1 || len(sIdxs) != 1 {
			continue
		}
		cand := &inlineCandidates[sIdxs[0]]
		cand.used = true
		planRules[pIdxs[0]].Id = types.StringValue(cand.id)
		planModified = true
	}

	if !planModified {
		return
	}

	rulesValue, diags := types.ListValueFrom(ctx, resource_allocation.RulesValue{}.Type(ctx), planRules)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	planModel.Rules = rulesValue
	resp.Diagnostics.Append(resp.Plan.Set(ctx, &planModel)...)
}
