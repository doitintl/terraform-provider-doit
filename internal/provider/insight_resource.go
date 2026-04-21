package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_insight"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type (
	insightResource struct {
		client *models.ClientWithResponses
	}
	insightResourceModel struct {
		resource_insight.InsightModel
		Timeouts timeouts.Value `tfsdk:"timeouts"`
	}
)

// Ensure the implementation satisfies expected interfaces.
var (
	_ resource.Resource                = (*insightResource)(nil)
	_ resource.ResourceWithConfigure   = (*insightResource)(nil)
	_ resource.ResourceWithImportState = (*insightResource)(nil)
)

// NewInsightResource creates a new insight resource instance.
func NewInsightResource() resource.Resource {
	return &insightResource{}
}

// Configure adds the provider configured client to the resource.
func (r *insightResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *insightResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_insight"
}

func (r *insightResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import ID format: sourceID/insightKey
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier with format: sourceID/insightKey. Got: %q", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("source_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("insight_key"), parts[1])...)
}

func (r *insightResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := resource_insight.InsightResourceSchema(ctx)

	s.Description = "Manages an insight in DoiT. An insight represents a recommendation or finding for cloud resources."
	s.MarkdownDescription = s.Description

	// Add UseStateForUnknown to stable Computed-only fields
	for _, field := range []string{"source", "display_status", "source_id", "insight_key"} {
		if attr, ok := s.Attributes[field].(schema.StringAttribute); ok {
			attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
			s.Attributes[field] = attr
		}
	}

	// key should be RequiresReplace since it's the identity
	if attr, ok := s.Attributes["key"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.RequiresReplace())
		s.Attributes["key"] = attr
	}

	s.Attributes["timeouts"] = timeouts.Attributes(ctx, timeouts.Opts{
		Create: true,
		Read:   true,
		Update: true,
		Delete: true,
	})

	resp.Schema = s
}

// buildInsightRequest constructs the API request body from the Terraform plan.
func buildInsightRequest(ctx context.Context, plan *insightResourceModel) (*models.InsightRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	req := models.InsightRequest{
		Key:              plan.Key.ValueString(),
		Title:            plan.Title.ValueString(),
		ShortDescription: plan.ShortDescription.ValueString(),
		CloudProvider:    plan.CloudProvider.ValueString(),
	}

	// Categories (Required)
	var categories []string
	diags.Append(plan.Categories.ElementsAs(ctx, &categories, false)...)
	if diags.HasError() {
		return nil, diags
	}
	createCategories := make([]models.CreateCategory, len(categories))
	for i, c := range categories {
		createCategories[i] = models.CreateCategory(c)
	}
	req.Categories = createCategories

	// Optional fields
	if !plan.DetailedDescriptionMdx.IsNull() && !plan.DetailedDescriptionMdx.IsUnknown() {
		v := plan.DetailedDescriptionMdx.ValueString()
		req.DetailedDescriptionMdx = &v
	}
	if !plan.EasyWinDescription.IsNull() && !plan.EasyWinDescription.IsUnknown() {
		v := plan.EasyWinDescription.ValueString()
		req.EasyWinDescription = &v
	}
	if !plan.ReportUrl.IsNull() && !plan.ReportUrl.IsUnknown() {
		v := plan.ReportUrl.ValueString()
		req.ReportUrl = &v
	}
	if !plan.CloudFlowTemplateId.IsNull() && !plan.CloudFlowTemplateId.IsUnknown() {
		v := plan.CloudFlowTemplateId.ValueString()
		req.CloudFlowTemplateId = &v
	}

	// Tags
	if !plan.Tags.IsNull() && !plan.Tags.IsUnknown() {
		var tags []string
		diags.Append(plan.Tags.ElementsAs(ctx, &tags, false)...)
		if diags.HasError() {
			return nil, diags
		}
		req.Tags = &tags
	}

	// Resource Results (Required)
	if !plan.ResourceResults.IsNull() && !plan.ResourceResults.IsUnknown() {
		var resourceResults []resource_insight.ResourceResultsValue
		diags.Append(plan.ResourceResults.ElementsAs(ctx, &resourceResults, false)...)
		if diags.HasError() {
			return nil, diags
		}

		apiResults := make(models.ResourceResults, 0, len(resourceResults))
		for _, rr := range resourceResults {
			apiResult := models.ResourceResult{
				ResourceId:    rr.ResourceId.ValueString(),
				Account:       rr.Account.ValueString(),
				CloudProvider: rr.CloudProvider.ValueString(),
				ResultType:    models.ResourceResultResultType(rr.ResultType.ValueString()),
			}

			// Optional resource result fields
			if !rr.Location.IsNull() && !rr.Location.IsUnknown() {
				v := rr.Location.ValueString()
				apiResult.Location = &v
			}
			if !rr.ExternalId.IsNull() && !rr.ExternalId.IsUnknown() {
				v := rr.ExternalId.ValueString()
				apiResult.ExternalId = &v
			}
			if !rr.ExternalUrl.IsNull() && !rr.ExternalUrl.IsUnknown() {
				v := rr.ExternalUrl.ValueString()
				apiResult.ExternalUrl = &v
			}
			if !rr.ResourceType.IsNull() && !rr.ResourceType.IsUnknown() {
				v := rr.ResourceType.ValueString()
				apiResult.ResourceType = &v
			}
			if !rr.Severity.IsNull() && !rr.Severity.IsUnknown() {
				v := models.ResourceResultSeverity(rr.Severity.ValueString())
				apiResult.Severity = &v
			}

			// Result object
			if !rr.Result.IsNull() && !rr.Result.IsUnknown() {
				result := &models.ResourceResultResult{}
				if !rr.Result.Value.IsNull() && !rr.Result.Value.IsUnknown() {
					val64 := rr.Result.Value.ValueFloat64()
					result.Value = &val64
				}
				if !rr.Result.Current.IsNull() && !rr.Result.Current.IsUnknown() {
					v := rr.Result.Current.ValueString()
					result.Current = &v
				}
				if !rr.Result.Recommendation.IsNull() && !rr.Result.Recommendation.IsUnknown() {
					v := rr.Result.Recommendation.ValueString()
					result.Recommendation = &v
				}
				apiResult.Result = result
			}

			apiResults = append(apiResults, apiResult)
		}
		req.ResourceResults = apiResults
	}

	return &req, diags
}

// overlayInsightComputedFields uses the plan-first overlay pattern.
// It preserves user-configured values from the plan and only overlays
// computed fields from the API response.
func overlayInsightComputedFields(ctx context.Context, apiResp *models.InsightResponse, plan *insightResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	// Phase 1: Build fully-resolved state from API response.
	resolved := insightResourceModel{Timeouts: plan.Timeouts}
	mapDiags := mapInsightRespToResourceModel(ctx, apiResp, &resolved)
	diags.Append(mapDiags...)
	if diags.HasError() {
		return diags
	}

	// Phase 2: Overlay computed-only fields — always from resolved.
	plan.Source = resolved.Source
	plan.SourceId = resolved.SourceId
	plan.InsightKey = resolved.InsightKey
	plan.DisplayStatus = resolved.DisplayStatus
	plan.LastStatusChange = resolved.LastStatusChange
	plan.LastUpdated = resolved.LastUpdated
	plan.Summary = resolved.Summary

	// Optional+Computed: resolve only when unknown
	if plan.CloudFlowTemplateId.IsUnknown() {
		plan.CloudFlowTemplateId = resolved.CloudFlowTemplateId
	}
	if plan.DetailedDescriptionMdx.IsUnknown() {
		plan.DetailedDescriptionMdx = resolved.DetailedDescriptionMdx
	}
	if plan.EasyWinDescription.IsUnknown() {
		plan.EasyWinDescription = resolved.EasyWinDescription
	}
	if plan.ReportUrl.IsUnknown() {
		plan.ReportUrl = resolved.ReportUrl
	}
	if plan.Tags.IsUnknown() {
		plan.Tags = resolved.Tags
	}

	// ResourceResults: the API response doesn't include resource_results, so the
	// resolved model has an empty list. We can't use overlayListElements here.
	// Instead, walk the plan's resource_results directly and resolve unknowns
	// to null defaults (for Optional+Computed fields the user omitted).
	if plan.ResourceResults.IsUnknown() {
		plan.ResourceResults = resolved.ResourceResults
	} else if !plan.ResourceResults.IsNull() {
		diags.Append(resolveResourceResultUnknowns(ctx, &plan.ResourceResults)...)
	}

	return diags
}

// resolveResourceResultUnknowns walks the plan's resource_results list and resolves
// any Unknown values to their null defaults. This is used during Create/Update where
// the API response doesn't include resource_results, so there's no "resolved" to overlay.
func resolveResourceResultUnknowns(ctx context.Context, list *types.List) diag.Diagnostics {
	var diags diag.Diagnostics
	var elems []resource_insight.ResourceResultsValue

	diags.Append(list.ElementsAs(ctx, &elems, true)...)
	if diags.HasError() {
		return diags
	}

	modified := false
	for i := range elems {
		if resolveOneResourceResult(&elems[i]) {
			modified = true
		}
	}

	if modified {
		newList, rebuildDiags := types.ListValueFrom(ctx, list.ElementType(ctx), elems)
		diags.Append(rebuildDiags...)
		if !rebuildDiags.HasError() {
			*list = newList
		}
	}
	return diags
}

// resolveOneResourceResult resolves all Unknown Optional+Computed fields in a single
// resource_results element to their null/false defaults. Returns true if any field was modified.
func resolveOneResourceResult(rr *resource_insight.ResourceResultsValue) bool {
	modified := false
	if rr.Enhancement.IsUnknown() {
		rr.Enhancement = resource_insight.NewEnhancementValueNull()
		modified = true
	}
	if rr.ExternalId.IsUnknown() {
		rr.ExternalId = types.StringNull()
		modified = true
	}
	if rr.ExternalUrl.IsUnknown() {
		rr.ExternalUrl = types.StringNull()
		modified = true
	}
	if rr.Location.IsUnknown() {
		rr.Location = types.StringNull()
		modified = true
	}
	if rr.Metadata.IsUnknown() {
		rr.Metadata = resource_insight.NewMetadataValueNull()
		modified = true
	}
	if rr.Resolved.IsUnknown() {
		rr.Resolved = types.BoolNull()
		modified = true
	}
	if rr.ResourceType.IsUnknown() {
		rr.ResourceType = types.StringNull()
		modified = true
	}
	if rr.Severity.IsUnknown() {
		rr.Severity = types.StringNull()
		modified = true
	}

	// Nested result object
	if rr.Result.IsUnknown() {
		rr.Result = resource_insight.NewResultValueNull()
		modified = true
	} else if !rr.Result.IsNull() {
		if resolveOneResultValue(&rr.Result) {
			modified = true
		}
	}
	return modified
}

// resolveOneResultValue resolves Unknown Optional+Computed fields in the nested result object.
func resolveOneResultValue(r *resource_insight.ResultValue) bool {
	modified := false
	if r.AgentInstalled.IsUnknown() {
		r.AgentInstalled = types.BoolNull()
		modified = true
	}
	if r.Critical.IsUnknown() {
		r.Critical = types.Int64Null()
		modified = true
	}
	if r.Current.IsUnknown() {
		r.Current = types.StringNull()
		modified = true
	}
	if r.High.IsUnknown() {
		r.High = types.Int64Null()
		modified = true
	}
	if r.Low.IsUnknown() {
		r.Low = types.Int64Null()
		modified = true
	}
	if r.Medium.IsUnknown() {
		r.Medium = types.Int64Null()
		modified = true
	}
	if r.Recommendation.IsUnknown() {
		r.Recommendation = types.StringNull()
		modified = true
	}
	if r.Value.IsUnknown() {
		r.Value = types.Float64Null()
		modified = true
	}
	return modified
}

// restoreUserResourceResultValues restores user-provided values from the plan
// after mapResourceResultsToModel has populated state from API response.
// This prevents precision drift (e.g. float32→float64 for result.value) by
// preserving the exact values the user wrote in HCL.
//
// Strategy: walk plan and API lists element-by-element (matched by index),
// and for each Required/user-provided field, restore the plan value.
func restoreUserResourceResultValues(ctx context.Context, planRR, apiRR *types.List) diag.Diagnostics {
	var diags diag.Diagnostics
	var planElems []resource_insight.ResourceResultsValue
	var apiElems []resource_insight.ResourceResultsValue

	diags.Append(planRR.ElementsAs(ctx, &planElems, true)...)
	diags.Append(apiRR.ElementsAs(ctx, &apiElems, true)...)
	if diags.HasError() {
		return diags
	}

	modified := false
	for i := range apiElems {
		if i >= len(planElems) {
			break
		}
		p := &planElems[i]
		a := &apiElems[i]

		// Restore user-provided Required fields
		if !p.Account.IsNull() && !p.Account.IsUnknown() {
			a.Account = p.Account
		}
		if !p.CloudProvider.IsNull() && !p.CloudProvider.IsUnknown() {
			a.CloudProvider = p.CloudProvider
		}
		if !p.ResourceId.IsNull() && !p.ResourceId.IsUnknown() {
			a.ResourceId = p.ResourceId
		}
		if !p.ResultType.IsNull() && !p.ResultType.IsUnknown() {
			a.ResultType = p.ResultType
		}

		// Restore user-provided Optional fields
		if !p.Location.IsNull() && !p.Location.IsUnknown() {
			a.Location = p.Location
		}
		if !p.ExternalId.IsNull() && !p.ExternalId.IsUnknown() {
			a.ExternalId = p.ExternalId
		}
		if !p.ExternalUrl.IsNull() && !p.ExternalUrl.IsUnknown() {
			a.ExternalUrl = p.ExternalUrl
		}
		if !p.ResourceType.IsNull() && !p.ResourceType.IsUnknown() {
			a.ResourceType = p.ResourceType
		}
		if !p.Severity.IsNull() && !p.Severity.IsUnknown() {
			a.Severity = p.Severity
		}

		// Restore nested result — particularly result.value which suffers float32 drift
		if !p.Result.IsNull() && !p.Result.IsUnknown() && !a.Result.IsNull() {
			if !p.Result.Value.IsNull() && !p.Result.Value.IsUnknown() {
				a.Result.Value = p.Result.Value
			}
			if !p.Result.Current.IsNull() && !p.Result.Current.IsUnknown() {
				a.Result.Current = p.Result.Current
			}
			if !p.Result.Recommendation.IsNull() && !p.Result.Recommendation.IsUnknown() {
				a.Result.Recommendation = p.Result.Recommendation
			}
		}

		modified = true
	}

	if modified {
		newList, rebuildDiags := types.ListValueFrom(ctx, apiRR.ElementType(ctx), apiElems)
		diags.Append(rebuildDiags...)
		if !rebuildDiags.HasError() {
			*apiRR = newList
		}
	}
	return diags
}

func (r *insightResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan insightResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, createDiags := plan.Timeouts.Create(ctx, 5*time.Minute)
	resp.Diagnostics.Append(createDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	apiReq, buildDiags := buildInsightRequest(ctx, &plan)
	resp.Diagnostics.Append(buildDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// sourceID is always "public-api" for API-created insights
	sourceID := "public-api"
	insightKey := plan.Key.ValueString()

	createResp, err := r.client.PostInsightResultWithResponse(ctx, sourceID, insightKey, *apiReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Insight",
			"Could not create insight, unexpected error: "+err.Error(),
		)
		return
	}

	if createResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Creating Insight",
			fmt.Sprintf("Could not create insight, status: %d, body: %s", createResp.StatusCode(), string(createResp.Body)),
		)
		return
	}

	if createResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Creating Insight",
			"Could not create insight, empty response",
		)
		return
	}

	// Plan-first overlay pattern
	resp.Diagnostics.Append(overlayInsightComputedFields(ctx, createResp.JSON200, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save the plan's resource_results before the API fetch — we'll restore
	// user-provided values after to avoid precision drift (float32→float64).
	planRR := plan.ResourceResults

	// Fetch resource results from the separate endpoint to populate state
	// consistently with what Read returns (prevents drift on first refresh).
	rrResp, err := r.client.GetInsightResourceResultsWithResponse(ctx, sourceID, insightKey)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Insight Resource Results",
			fmt.Sprintf("Could not read resource results after create for %s/%s: %s", sourceID, insightKey, err.Error()),
		)
		return
	}
	if rrResp.StatusCode() == 200 && rrResp.JSON200 != nil {
		resp.Diagnostics.Append(mapResourceResultsToModel(ctx, rrResp.JSON200, &plan)...)
		if !resp.Diagnostics.HasError() {
			resp.Diagnostics.Append(restoreUserResourceResultValues(ctx, &planRR, &plan.ResourceResults)...)
		}
	}
	// 404 or non-200: keep the overlay's resolved values (safe — overlay already resolved unknowns)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *insightResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state insightResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	readTimeout, readDiags := state.Timeouts.Read(ctx, 2*time.Minute)
	resp.Diagnostics.Append(readDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	sourceID := state.SourceId.ValueString()
	insightKey := state.InsightKey.ValueString()

	// Handle import: if source_id or insight_key is missing, derive from key
	if sourceID == "" {
		sourceID = "public-api"
	}
	if insightKey == "" {
		insightKey = state.Key.ValueString()
	}

	getResp, err := r.client.GetInsightResultWithResponse(ctx, sourceID, insightKey)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Insight",
			fmt.Sprintf("Could not read insight %s/%s: %s", sourceID, insightKey, err.Error()),
		)
		return
	}

	// Handle externally deleted resource
	if getResp.StatusCode() == 404 {
		resp.State.RemoveResource(ctx)
		return
	}

	if getResp.StatusCode() != 200 || getResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Insight",
			fmt.Sprintf("Unexpected status %d for insight %s/%s: %s", getResp.StatusCode(), sourceID, insightKey, string(getResp.Body)),
		)
		return
	}

	// Map full response to state — Read uses full mapping, not overlay
	resp.Diagnostics.Append(mapInsightRespToResourceModel(ctx, getResp.JSON200, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Fetch resource results separately (they are not included in InsightResponse)
	rrResp, err := r.client.GetInsightResourceResultsWithResponse(ctx, sourceID, insightKey)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Insight Resource Results",
			fmt.Sprintf("Could not read resource results for %s/%s: %s", sourceID, insightKey, err.Error()),
		)
		return
	}

	// 404 is OK here — means no resource results (e.g. after all were resolved)
	if rrResp.StatusCode() == 200 && rrResp.JSON200 != nil {
		resp.Diagnostics.Append(mapResourceResultsToModel(ctx, rrResp.JSON200, &state)...)
	} else if rrResp.StatusCode() == 404 {
		emptyList, emptyDiags := types.ListValueFrom(ctx, resource_insight.ResourceResultsType{
			ObjectType: types.ObjectType{AttrTypes: resource_insight.ResourceResultsValue{}.AttributeTypes(ctx)},
		}, []resource_insight.ResourceResultsValue{})
		resp.Diagnostics.Append(emptyDiags...)
		state.ResourceResults = emptyList
	} else if rrResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Reading Insight Resource Results",
			fmt.Sprintf("Unexpected status %d for resource results %s/%s: %s", rrResp.StatusCode(), sourceID, insightKey, string(rrResp.Body)),
		)
		return
	}
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *insightResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan insightResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateTimeout, updateDiags := plan.Timeouts.Update(ctx, 5*time.Minute)
	resp.Diagnostics.Append(updateDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	apiReq, buildDiags := buildInsightRequest(ctx, &plan)
	resp.Diagnostics.Append(buildDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	sourceID := "public-api"
	insightKey := plan.Key.ValueString()

	updateResp, err := r.client.PostInsightResultWithResponse(ctx, sourceID, insightKey, *apiReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Insight",
			"Could not update insight, unexpected error: "+err.Error(),
		)
		return
	}

	if updateResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Updating Insight",
			fmt.Sprintf("Could not update insight, status: %d, body: %s", updateResp.StatusCode(), string(updateResp.Body)),
		)
		return
	}

	if updateResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Updating Insight",
			"Received empty response body",
		)
		return
	}

	// Plan-first overlay pattern
	resp.Diagnostics.Append(overlayInsightComputedFields(ctx, updateResp.JSON200, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save the plan's resource_results before the API fetch.
	planRR := plan.ResourceResults

	// Fetch resource results from the separate endpoint to populate state
	// consistently with what Read returns (prevents drift on first refresh).
	rrResp, err := r.client.GetInsightResourceResultsWithResponse(ctx, sourceID, insightKey)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Insight Resource Results",
			fmt.Sprintf("Could not read resource results after update for %s/%s: %s", sourceID, insightKey, err.Error()),
		)
		return
	}
	if rrResp.StatusCode() == 200 && rrResp.JSON200 != nil {
		resp.Diagnostics.Append(mapResourceResultsToModel(ctx, rrResp.JSON200, &plan)...)
		if !resp.Diagnostics.HasError() {
			resp.Diagnostics.Append(restoreUserResourceResultValues(ctx, &planRR, &plan.ResourceResults)...)
		}
	}
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *insightResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state insightResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, deleteDiags := state.Timeouts.Delete(ctx, 2*time.Minute)
	resp.Diagnostics.Append(deleteDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	sourceID := state.SourceId.ValueString()
	insightKey := state.InsightKey.ValueString()

	deleteResp, err := r.client.DeleteInsightResultWithResponse(ctx, sourceID, insightKey)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Insight",
			"Could not delete insight, unexpected error: "+err.Error(),
		)
		return
	}

	// Treat 404 as success - resource is already gone
	if deleteResp.StatusCode() != 200 && deleteResp.StatusCode() != 204 && deleteResp.StatusCode() != 404 {
		resp.Diagnostics.AddError(
			"Error Deleting Insight",
			fmt.Sprintf("Could not delete insight, status: %d, body: %s", deleteResp.StatusCode(), string(deleteResp.Body)),
		)
		return
	}
}

// mapInsightRespToResourceModel maps the full InsightResponse to the Terraform resource model.
// Used by Read and ImportState (not Create/Update which use the overlay).
// Named differently from mapInsightResponseToModel in insight_data_source.go to avoid collision.
func mapInsightRespToResourceModel(ctx context.Context, resp *models.InsightResponse, state *insightResourceModel) (diags diag.Diagnostics) {
	// Identity fields
	state.Key = types.StringPointerValue(resp.Key)
	state.SourceId = types.StringPointerValue(resp.Source)
	state.InsightKey = types.StringPointerValue(resp.Key)
	state.Source = types.StringPointerValue(resp.Source)

	// Core fields
	state.Title = types.StringPointerValue(resp.Title)
	state.ShortDescription = types.StringPointerValue(resp.ShortDescription)

	if resp.CloudProvider != nil {
		state.CloudProvider = types.StringValue(*resp.CloudProvider)
	} else {
		state.CloudProvider = types.StringNull()
	}

	if resp.DisplayStatus != nil {
		state.DisplayStatus = types.StringValue(string(*resp.DisplayStatus))
	} else {
		state.DisplayStatus = types.StringNull()
	}

	state.DetailedDescriptionMdx = types.StringPointerValue(resp.DetailedDescriptionMdx)
	state.EasyWinDescription = types.StringPointerValue(resp.EasyWinDescription)
	state.ReportUrl = types.StringPointerValue(resp.ReportUrl)
	state.CloudFlowTemplateId = types.StringPointerValue(resp.CloudFlowTemplateId)

	// LastUpdated is *time.Time in the API model
	if resp.LastUpdated != nil {
		state.LastUpdated = types.StringValue(resp.LastUpdated.UTC().Format(time.RFC3339))
	} else {
		state.LastUpdated = types.StringNull()
	}

	// Categories
	if resp.Categories != nil {
		catStrings := make([]string, len(*resp.Categories))
		for i, c := range *resp.Categories {
			catStrings[i] = string(c)
		}
		catList, catDiags := types.ListValueFrom(ctx, types.StringType, catStrings)
		diags.Append(catDiags...)
		state.Categories = catList
	} else {
		state.Categories, _ = types.ListValueFrom(ctx, types.StringType, []string{})
	}

	// Tags
	if resp.Tags != nil {
		tagList, tagDiags := types.ListValueFrom(ctx, types.StringType, *resp.Tags)
		diags.Append(tagDiags...)
		state.Tags = tagList
	} else {
		state.Tags, _ = types.ListValueFrom(ctx, types.StringType, []string{})
	}

	// Summary (Computed-only nested object)
	if resp.Summary != nil {
		summaryVal, summaryDiags := resource_insight.NewSummaryValue(
			resource_insight.SummaryValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"operational_risks":       types.Float64PointerValue(resp.Summary.OperationalRisks),
				"performance_risks":       types.Float64PointerValue(resp.Summary.PerformanceRisks),
				"potential_daily_savings": types.Float64PointerValue(resp.Summary.PotentialDailySavings),
				"reliability_risks":       types.Float64PointerValue(resp.Summary.ReliabilityRisks),
				"security_risks":          types.Float64PointerValue(resp.Summary.SecurityRisks),
				"sustainability_risks":    types.Float64PointerValue(resp.Summary.SustainabilityRisks),
			},
		)
		diags.Append(summaryDiags...)
		state.Summary = summaryVal
	} else {
		state.Summary = resource_insight.NewSummaryValueNull()
	}

	// LastStatusChange (Computed-only nested object)
	if resp.LastStatusChange != nil {
		lscVal, lscDiags := resource_insight.NewLastStatusChangeValue(
			resource_insight.LastStatusChangeValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"last_changed_at": types.StringValue(resp.LastStatusChange.LastChangedAt.UTC().Format(time.RFC3339)),
				"user_id":         types.StringValue(resp.LastStatusChange.UserId),
			},
		)
		diags.Append(lscDiags...)
		state.LastStatusChange = lscVal
	} else {
		state.LastStatusChange = resource_insight.NewLastStatusChangeValueNull()
	}

	// ResourceResults are not included in InsightResponse — they are fetched
	// separately via GetInsightResourceResultsWithResponse in Read.
	// For Import: set to empty (Read will immediately fetch them).
	if state.ResourceResults.IsNull() || state.ResourceResults.IsUnknown() {
		emptyList, emptyDiags := types.ListValueFrom(ctx, resource_insight.ResourceResultsType{
			ObjectType: types.ObjectType{AttrTypes: resource_insight.ResourceResultsValue{}.AttributeTypes(ctx)},
		}, []resource_insight.ResourceResultsValue{})
		diags.Append(emptyDiags...)
		state.ResourceResults = emptyList
	}

	return diags
}

// mapResourceResultsToModel maps the API ResourceResults array to the Terraform model.
// Called from Read after fetching resource results via the separate endpoint.
func mapResourceResultsToModel(ctx context.Context, results *[]models.ResourceResult, state *insightResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	if results == nil || len(*results) == 0 {
		emptyList, emptyDiags := types.ListValueFrom(ctx, resource_insight.ResourceResultsType{
			ObjectType: types.ObjectType{AttrTypes: resource_insight.ResourceResultsValue{}.AttributeTypes(ctx)},
		}, []resource_insight.ResourceResultsValue{})
		diags.Append(emptyDiags...)
		state.ResourceResults = emptyList
		return diags
	}

	resultValues := make([]resource_insight.ResourceResultsValue, 0, len(*results))
	for _, rr := range *results {
		// Build result nested object
		resultObj := resource_insight.NewResultValueNull()
		if rr.Result != nil {
			resultAttrs := map[string]attr.Value{
				"agent_installed": types.BoolNull(),
				"critical":        types.Int64Null(),
				"current":         types.StringPointerValue(rr.Result.Current),
				"high":            types.Int64Null(),
				"low":             types.Int64Null(),
				"medium":          types.Int64Null(),
				"recommendation":  types.StringPointerValue(rr.Result.Recommendation),
				"value":           types.Float64PointerValue(rr.Result.Value),
			}
			var resultDiags diag.Diagnostics
			resultObj, resultDiags = resource_insight.NewResultValue(
				resource_insight.ResultValue{}.AttributeTypes(ctx),
				resultAttrs,
			)
			diags.Append(resultDiags...)
		}

		// Build enhancement nested object (usually null for public-api insights)
		enhancementObj := resource_insight.NewEnhancementValueNull()

		// Build metadata nested object (usually null)
		metadataObj := resource_insight.NewMetadataValueNull()

		rrVal, rrDiags := resource_insight.NewResourceResultsValue(
			resource_insight.ResourceResultsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"account":        types.StringValue(rr.Account),
				"cloud_provider": types.StringValue(rr.CloudProvider),
				"enhancement":    enhancementObj,
				"external_id":    types.StringPointerValue(rr.ExternalId),
				"external_url":   types.StringPointerValue(rr.ExternalUrl),
				"location":       types.StringPointerValue(rr.Location),
				"metadata":       metadataObj,
				"resolved":       types.BoolPointerValue(rr.Resolved),
				"resource_id":    types.StringValue(rr.ResourceId),
				"resource_type":  types.StringPointerValue(rr.ResourceType),
				"result":         resultObj,
				"result_type":    types.StringValue(string(rr.ResultType)),
				"severity":       severityToString(rr.Severity),
			},
		)
		diags.Append(rrDiags...)
		resultValues = append(resultValues, rrVal)
	}

	rrList, listDiags := types.ListValueFrom(ctx, resource_insight.ResourceResultsType{
		ObjectType: types.ObjectType{AttrTypes: resource_insight.ResourceResultsValue{}.AttributeTypes(ctx)},
	}, resultValues)
	diags.Append(listDiags...)
	state.ResourceResults = rrList

	return diags
}

func severityToString(s *models.ResourceResultSeverity) types.String {
	if s == nil {
		return types.StringNull()
	}
	return types.StringValue(string(*s))
}

// planModifier that makes a string attribute require replacement on change.
var _ planmodifier.String = stringRequiresReplace{}

type stringRequiresReplace struct{}

func (m stringRequiresReplace) Description(_ context.Context) string {
	return "If the value of this attribute changes, Terraform will destroy and recreate the resource."
}

func (m stringRequiresReplace) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m stringRequiresReplace) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if !req.StateValue.IsNull() && !req.PlanValue.Equal(req.StateValue) {
		resp.RequiresReplace = true
	}
}
