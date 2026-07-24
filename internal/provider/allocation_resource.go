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
// Pass 1 matches by exact name. Pass 2 uses tiered content-based matching for renamed rules:
//   - Tier 1: formula + components both match (pure rename)
//   - Tier 2: components only match (rename + formula change)
//   - Tier 3: formula only match (rename + component change)
//
// Each tier requires a unique 1:1 match. If all three fields change simultaneously (name,
// formula, and components), the rule cannot be matched and a warning is emitted for
// action="update" rules — set the id attribute explicitly in that case.
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

	// Collect state rule candidates with known non-empty IDs.
	// All actions are included (not just "create"/"update") because after ImportState,
	// fillAllocationCommon defaults every rule to action="select" since the API doesn't
	// return the action field. Filtering on action would prevent ID recovery on reapply.
	// Plan-side gating (isInlineRuleNeedingID) restricts matching to "create"/"update"
	// plan rules, so genuine "select" plan rules won't consume candidates.
	type inlineStateRule struct {
		stateIdx int
		id       string
		name     string
		formula  string
		used     bool
	}

	var inlineCandidates []inlineStateRule
	for i, sRule := range stateRules {
		if sRule.Id.IsNull() || sRule.Id.IsUnknown() || sRule.Id.ValueString() == "" {
			continue
		}
		inlineCandidates = append(inlineCandidates, inlineStateRule{
			stateIdx: i,
			id:       sRule.Id.ValueString(),
			name:     sRule.Name.ValueString(),
			formula:  sRule.Formula.ValueString(),
			used:     false,
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

	// Pass 2: Tiered content-based matching for renamed rules.
	// When a rule is renamed, Pass 1 (name match) misses it. We try three tiers of
	// decreasing specificity, each requiring a unique 1:1 match. This ensures any
	// single-field change (name, formula, or components) still produces an in-place update.
	type tierMatch struct {
		planIdx int
		candIdx int
	}

	commitMatches := func(matches []tierMatch) {
		for _, m := range matches {
			cand := &inlineCandidates[m.candIdx]
			cand.used = true
			planRules[m.planIdx].Id = types.StringValue(cand.id)
			planModified = true
		}
	}

	runTier := func(matchFn func(planIdx, candIdx int) bool) []tierMatch {
		type tentative struct {
			candIdx int
			count   int
		}
		planHits := map[int]*tentative{}
		for i := range planRules {
			if !isInlineRuleNeedingID(planRules[i]) {
				continue
			}
			for cIdx := range inlineCandidates {
				if inlineCandidates[cIdx].used {
					continue
				}
				if matchFn(i, cIdx) {
					if h, ok := planHits[i]; ok {
						h.count++
					} else {
						planHits[i] = &tentative{candIdx: cIdx, count: 1}
					}
				}
			}
		}
		// Only keep plan rules with exactly 1 candidate match.
		var unique []tierMatch
		for pIdx, h := range planHits {
			if h.count == 1 {
				unique = append(unique, tierMatch{planIdx: pIdx, candIdx: h.candIdx})
			}
		}
		// Count how many plan rules claimed each candidate. Drop any candidate
		// claimed by more than one plan rule — neither claim is a true 1:1 match.
		candClaims := map[int]int{}
		for _, m := range unique {
			candClaims[m.candIdx]++
		}
		var result []tierMatch
		for _, m := range unique {
			if candClaims[m.candIdx] == 1 {
				result = append(result, m)
			}
		}
		return result
	}

	// Tier 1: Formula + Components match (pure rename — strongest signal)
	commitMatches(runTier(func(pIdx, cIdx int) bool {
		return planRules[pIdx].Formula.Equal(stateRules[inlineCandidates[cIdx].stateIdx].Formula) &&
			planRules[pIdx].Components.Equal(stateRules[inlineCandidates[cIdx].stateIdx].Components)
	}))

	// Tier 2: Components only match (rename + formula change)
	commitMatches(runTier(func(pIdx, cIdx int) bool {
		pc := planRules[pIdx].Components
		if pc.IsNull() || pc.IsUnknown() {
			return false
		}
		return pc.Equal(stateRules[inlineCandidates[cIdx].stateIdx].Components)
	}))

	// Tier 3: Formula only match (rename + component change — weakest signal)
	commitMatches(runTier(func(pIdx, cIdx int) bool {
		pf := planRules[pIdx].Formula
		if pf.IsNull() || pf.IsUnknown() || pf.ValueString() == "" {
			return false
		}
		return pf.Equal(stateRules[inlineCandidates[cIdx].stateIdx].Formula)
	}))

	// Error on unmatched action="update" rules — the API requires an ID for updates.
	for i := range planRules {
		if planRules[i].Action.ValueString() == "update" && isInlineRuleNeedingID(planRules[i]) {
			resp.Diagnostics.AddError(
				"Unmatched allocation rule",
				fmt.Sprintf("Rule %q has action \"update\" but could not be matched to an existing rule by name, formula, or components. "+
					"Either set the rule's id attribute explicitly to target an existing rule, "+
					"or change the action to \"create\" to create a new rule.",
					planRules[i].Name.ValueString()),
			)
		}
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
