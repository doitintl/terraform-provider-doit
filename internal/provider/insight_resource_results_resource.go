package provider

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	rr "github.com/doitintl/terraform-provider-doit/internal/provider/resource_insight_resource_results"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type (
	insightResourceResultsResource struct {
		client *models.ClientWithResponses
	}
	insightResourceResultsModel struct {
		rr.InsightResourceResultsModel
		Timeouts timeouts.Value `tfsdk:"timeouts"`
	}
)

var (
	_ resource.Resource                = (*insightResourceResultsResource)(nil)
	_ resource.ResourceWithConfigure   = (*insightResourceResultsResource)(nil)
	_ resource.ResourceWithImportState = (*insightResourceResultsResource)(nil)
)

func NewInsightResourceResultsResource() resource.Resource {
	return &insightResourceResultsResource{}
}

func (r *insightResourceResultsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*models.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *models.ClientWithResponses, got: %T.", req.ProviderData),
		)
		return
	}
	r.client = client
}

func (r *insightResourceResultsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_insight_resource_results"
}

func (r *insightResourceResultsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
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

func (r *insightResourceResultsResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := rr.InsightResourceResultsResourceSchema(ctx)

	s.Attributes["timeouts"] = timeouts.Attributes(ctx, timeouts.Opts{
		Create: true,
		Read:   true,
		Update: true,
		Delete: true,
	})

	// Mark identity fields inside resource_results as RequiresReplace.
	// These fields form the upsert key in the database and are immutable
	// after initial insert — the API silently ignores updates to them.
	// Changing any of these in Terraform triggers a destroy-and-recreate.
	immutableFieldDescs := map[string]string{
		"result_type":    "The discriminator property that determines which fields are populated in the 'result' object. **Immutable after creation** — changing this value will destroy and recreate the resource.",
		"account":        "The cloud account or project ID containing this resource. **Immutable after creation** — changing this value will destroy and recreate the resource.",
		"cloud_provider": "The cloud provider associated with the resource. **Immutable after creation** — changing this value will destroy and recreate the resource.",
		"location":       "The region/zone of the resource (e.g. `eu-west-2`). **Immutable after creation** — changing this value will destroy and recreate the resource.",
		"resource_type":  "What the resource actually is, e.g. `instance`, `disk`, `cache`. **Immutable after creation** — changing this value will destroy and recreate the resource.",
	}
	if rrAttr, ok := s.Attributes["resource_results"].(schema.ListNestedAttribute); ok {
		for field, desc := range immutableFieldDescs {
			if nested, ok := rrAttr.NestedObject.Attributes[field].(schema.StringAttribute); ok {
				nested.PlanModifiers = append(nested.PlanModifiers, stringplanmodifier.RequiresReplace())
				nested.MarkdownDescription = desc
				rrAttr.NestedObject.Attributes[field] = nested
			}
		}
		s.Attributes["resource_results"] = rrAttr
	}

	resp.Schema = s
}

// buildResourceResultsRequest builds the API request body from the plan.
func buildResourceResultsRequest(ctx context.Context, plan *insightResourceResultsModel) (*models.CreateResourceResultsBody, diag.Diagnostics) {
	var diags diag.Diagnostics

	var elements []rr.ResourceResultsValue
	diags.Append(plan.ResourceResults.ElementsAs(ctx, &elements, false)...)
	if diags.HasError() {
		return nil, diags
	}

	apiResults := make(models.ResourceResults, 0, len(elements))
	for _, elem := range elements {
		apiResult := models.ResourceResultRequest{
			ResourceId:    elem.ResourceId.ValueString(),
			Account:       elem.Account.ValueString(),
			CloudProvider: elem.CloudProvider.ValueString(),
			ResultType:    models.ResourceResultRequestResultType(elem.ResultType.ValueString()),
		}

		if !elem.Location.IsNull() && !elem.Location.IsUnknown() {
			v := elem.Location.ValueString()
			apiResult.Location = &v
		}
		if !elem.ExternalId.IsNull() && !elem.ExternalId.IsUnknown() {
			v := elem.ExternalId.ValueString()
			apiResult.ExternalId = &v
		}
		if !elem.ExternalUrl.IsNull() && !elem.ExternalUrl.IsUnknown() {
			v := elem.ExternalUrl.ValueString()
			apiResult.ExternalUrl = &v
		}
		if !elem.ResourceType.IsNull() && !elem.ResourceType.IsUnknown() {
			v := elem.ResourceType.ValueString()
			apiResult.ResourceType = &v
		}

		// Result object
		if !elem.Result.IsNull() && !elem.Result.IsUnknown() {
			result := &models.ResourceResultRequestResult{}
			if !elem.Result.Value.IsNull() && !elem.Result.Value.IsUnknown() {
				val64 := elem.Result.Value.ValueFloat64()
				result.Value = &val64
			}
			if !elem.Result.Current.IsNull() && !elem.Result.Current.IsUnknown() {
				v := elem.Result.Current.ValueString()
				result.Current = &v
			}
			if !elem.Result.Recommendation.IsNull() && !elem.Result.Recommendation.IsUnknown() {
				v := elem.Result.Recommendation.ValueString()
				result.Recommendation = &v
			}
			if !elem.Result.AgentInstalled.IsNull() && !elem.Result.AgentInstalled.IsUnknown() {
				v := elem.Result.AgentInstalled.ValueBool()
				result.AgentInstalled = &v
			}
			if !elem.Result.Critical.IsNull() && !elem.Result.Critical.IsUnknown() {
				v := int(elem.Result.Critical.ValueInt64())
				result.Critical = &v
			}
			if !elem.Result.High.IsNull() && !elem.Result.High.IsUnknown() {
				v := int(elem.Result.High.ValueInt64())
				result.High = &v
			}
			if !elem.Result.Medium.IsNull() && !elem.Result.Medium.IsUnknown() {
				v := int(elem.Result.Medium.ValueInt64())
				result.Medium = &v
			}
			if !elem.Result.Low.IsNull() && !elem.Result.Low.IsUnknown() {
				v := int(elem.Result.Low.ValueInt64())
				result.Low = &v
			}
			apiResult.Result = result
		}

		apiResults = append(apiResults, apiResult)
	}

	return &models.CreateResourceResultsBody{
		ResourceResults: apiResults,
	}, diags
}

// mapRRResponseToModel maps the full API response to the Terraform model.
// Used by Read (and internally by overlay Phase 1).
func mapRRResponseToModel(ctx context.Context, results []models.ResourceResult, state *insightResourceResultsModel) diag.Diagnostics {
	var diags diag.Diagnostics

	if len(results) == 0 {
		emptyList, emptyDiags := types.ListValueFrom(ctx, rr.ResourceResultsType{
			ObjectType: types.ObjectType{AttrTypes: rr.ResourceResultsValue{}.AttributeTypes(ctx)},
		}, []rr.ResourceResultsValue{})
		diags.Append(emptyDiags...)
		state.ResourceResults = emptyList
		state.RowCount = types.Int64Value(0)
		return diags
	}

	resultValues := make([]rr.ResourceResultsValue, 0, len(results))
	for _, apiRR := range results {
		// Build result nested object
		resultObj := rr.NewResultValueNull()
		if apiRR.Result != nil {
			resultAttrs := map[string]attr.Value{
				"agent_installed": types.BoolPointerValue(apiRR.Result.AgentInstalled),
				"critical":        intPtrToInt64Value(apiRR.Result.Critical),
				"current":         types.StringPointerValue(apiRR.Result.Current),
				"high":            intPtrToInt64Value(apiRR.Result.High),
				"low":             intPtrToInt64Value(apiRR.Result.Low),
				"medium":          intPtrToInt64Value(apiRR.Result.Medium),
				"recommendation":  types.StringPointerValue(apiRR.Result.Recommendation),
				"value":           types.Float64PointerValue(apiRR.Result.Value),
			}
			var resultDiags diag.Diagnostics
			resultObj, resultDiags = rr.NewResultValue(
				rr.ResultValue{}.AttributeTypes(ctx),
				resultAttrs,
			)
			diags.Append(resultDiags...)
		}

		// Build enhancement nested object
		enhancementObj := rr.NewEnhancementValueNull()
		if apiRR.Enhancement != nil {
			// Build priority
			priorityObj := rr.NewPriorityValueNull()
			if apiRR.Enhancement.Priority != nil {
				var prDiags diag.Diagnostics
				priorityObj, prDiags = rr.NewPriorityValue(
					rr.PriorityValue{}.AttributeTypes(ctx),
					map[string]attr.Value{
						"priority_score": types.Float64PointerValue(apiRR.Enhancement.Priority.PriorityScore),
						"value":          types.StringPointerValue(apiRR.Enhancement.Priority.Value),
					},
				)
				diags.Append(prDiags...)
			}

			// Build tags
			var tagsVal types.List
			if apiRR.Enhancement.Tags != nil {
				var tagDiags diag.Diagnostics
				tagsVal, tagDiags = types.ListValueFrom(ctx, types.StringType, *apiRR.Enhancement.Tags)
				diags.Append(tagDiags...)
			} else {
				var tagDiags diag.Diagnostics
				tagsVal, tagDiags = types.ListValueFrom(ctx, types.StringType, []string{})
				diags.Append(tagDiags...)
			}

			// lastUpdatedAt
			lastUpdatedAt := types.StringNull()
			if apiRR.Enhancement.LastUpdatedAt != nil {
				lastUpdatedAt = types.StringValue(apiRR.Enhancement.LastUpdatedAt.UTC().Format(time.RFC3339))
			}

			var enhDiags diag.Diagnostics
			enhancementObj, enhDiags = rr.NewEnhancementValue(
				rr.EnhancementValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"last_updated_at": lastUpdatedAt,
					"last_updated_by": types.StringPointerValue(apiRR.Enhancement.LastUpdatedBy),
					"priority":        priorityObj,
					"tags":            tagsVal,
				},
			)
			diags.Append(enhDiags...)
		}

		// Build metadata (empty object)
		metadataObj := rr.NewMetadataValueNull()

		// Severity
		severity := types.StringNull()
		if apiRR.Severity != nil {
			severity = types.StringValue(string(*apiRR.Severity))
		}

		// Resolved
		resolved := types.BoolNull()
		if apiRR.Resolved != nil {
			resolved = types.BoolValue(*apiRR.Resolved)
		}

		rrVal, rrDiags := rr.NewResourceResultsValue(
			rr.ResourceResultsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"account":        types.StringValue(apiRR.Account),
				"cloud_provider": types.StringValue(apiRR.CloudProvider),
				"enhancement":    enhancementObj,
				"external_id":    stringPtrOrNull(apiRR.ExternalId),
				"external_url":   stringPtrOrNull(apiRR.ExternalUrl),
				"location":       stringPtrOrNull(apiRR.Location),
				"metadata":       metadataObj,
				"resolved":       resolved,
				"resource_id":    types.StringValue(apiRR.ResourceId),
				"resource_type":  stringPtrOrNull(apiRR.ResourceType),
				"result":         resultObj,
				"result_type":    types.StringValue(string(apiRR.ResultType)),
				"severity":       severity,
			},
		)
		diags.Append(rrDiags...)
		resultValues = append(resultValues, rrVal)
	}

	rrList, listDiags := types.ListValueFrom(ctx, rr.ResourceResultsType{
		ObjectType: types.ObjectType{AttrTypes: rr.ResourceResultsValue{}.AttributeTypes(ctx)},
	}, resultValues)
	diags.Append(listDiags...)
	state.ResourceResults = rrList
	state.RowCount = types.Int64Value(int64(len(results)))

	return diags
}

// stringPtrOrNull converts *string to types.String, treating nil and empty
// strings as null. This prevents null vs "" drift between the overlay (which
// resolves omitted Optional+Computed fields to null) and the read path (where
// the API returns "" for unset optional strings).
func stringPtrOrNull(s *string) types.String {
	if s == nil || *s == "" {
		return types.StringNull()
	}
	return types.StringValue(*s)
}

// intPtrToInt64Value converts *int to types.Int64, returning Null if nil.
func intPtrToInt64Value(i *int) types.Int64 {
	if i == nil {
		return types.Int64Null()
	}
	return types.Int64Value(int64(*i))
}

// overlayRRComputedFields uses the plan-first overlay pattern.
func overlayRRComputedFields(ctx context.Context, results []models.ResourceResult, plan *insightResourceResultsModel) diag.Diagnostics {
	var diags diag.Diagnostics

	// Phase 1: Build fully-resolved state from API response.
	resolved := insightResourceResultsModel{Timeouts: plan.Timeouts}
	resolved.SourceId = plan.SourceId
	resolved.InsightKey = plan.InsightKey
	resolved.MaxResults = plan.MaxResults
	diags.Append(mapRRResponseToModel(ctx, results, &resolved)...)
	if diags.HasError() {
		return diags
	}

	// Phase 2: Overlay computed-only fields.
	plan.RowCount = resolved.RowCount
	plan.PageToken = resolved.PageToken

	// Phase 3: For resource_results, walk each element and overlay
	// computed fields (severity, resolved, enhancement) from the API response
	// while preserving user-provided values. Uses key-based matching by
	// resource_id since the API may return elements in a different order.
	if plan.ResourceResults.IsUnknown() {
		plan.ResourceResults = resolved.ResourceResults
		return diags
	}

	diags.Append(overlayListElementsByKey(ctx, &resolved.ResourceResults, &plan.ResourceResults, overlayResourceResult)...)

	return diags
}

// overlayResourceResult resolves Unknown subfields from the resolved element.
// Known values are never touched — the user's plan is the source of truth.
func overlayResourceResult(_ context.Context, resolved, plan *rr.ResourceResultsValue) diag.Diagnostics {
	// Computed-only: always from resolved
	plan.Severity = resolved.Severity
	plan.Resolved = resolved.Resolved
	plan.Enhancement = resolved.Enhancement

	// Optional+Computed scalars: resolve only when Unknown
	if plan.ExternalId.IsUnknown() {
		plan.ExternalId = resolved.ExternalId
	}
	if plan.ExternalUrl.IsUnknown() {
		plan.ExternalUrl = resolved.ExternalUrl
	}
	if plan.Location.IsUnknown() {
		plan.Location = resolved.Location
	}
	if plan.ResourceType.IsUnknown() {
		plan.ResourceType = resolved.ResourceType
	}
	if plan.Metadata.IsUnknown() {
		plan.Metadata = resolved.Metadata
	}

	// Nested result: resolve unknowns in subfields
	if plan.Result.IsUnknown() {
		plan.Result = resolved.Result
	} else if !plan.Result.IsNull() {
		overlayResultNested(&resolved.Result, &plan.Result)
	}

	return nil
}

// overlayResultNested resolves Unknown subfields in the nested result object.
func overlayResultNested(resolved, plan *rr.ResultValue) {
	if plan.AgentInstalled.IsUnknown() {
		plan.AgentInstalled = resolved.AgentInstalled
	}
	if plan.Critical.IsUnknown() {
		plan.Critical = resolved.Critical
	}
	if plan.Current.IsUnknown() {
		plan.Current = resolved.Current
	}
	if plan.High.IsUnknown() {
		plan.High = resolved.High
	}
	if plan.Low.IsUnknown() {
		plan.Low = resolved.Low
	}
	if plan.Medium.IsUnknown() {
		plan.Medium = resolved.Medium
	}
	if plan.Recommendation.IsUnknown() {
		plan.Recommendation = resolved.Recommendation
	}
	if plan.Value.IsUnknown() {
		plan.Value = resolved.Value
	}
}

// overlayListElementsByKey is like overlayListElements but matches elements
// by resource_id rather than by position. This is needed because the API may
// return elements in a different order than the user's HCL config.
func overlayListElementsByKey(ctx context.Context, resolved, plan *types.List, overlayFn func(context.Context, *rr.ResourceResultsValue, *rr.ResourceResultsValue) diag.Diagnostics) diag.Diagnostics {
	var diags diag.Diagnostics
	var planElems []rr.ResourceResultsValue
	var resolvedElems []rr.ResourceResultsValue

	diags.Append(plan.ElementsAs(ctx, &planElems, true)...)
	diags.Append(resolved.ElementsAs(ctx, &resolvedElems, true)...)
	if diags.HasError() {
		return diags
	}

	// Build map by resource_id for key-based matching.
	resolvedByID := make(map[string]*rr.ResourceResultsValue, len(resolvedElems))
	for i := range resolvedElems {
		resolvedByID[resolvedElems[i].ResourceId.ValueString()] = &resolvedElems[i]
	}

	for i := range planElems {
		p := &planElems[i]
		r, ok := resolvedByID[p.ResourceId.ValueString()]
		if !ok {
			continue
		}
		diags.Append(overlayFn(ctx, r, p)...)
	}

	// Rebuild the list with overlaid elements.
	newList, rebuildDiags := types.ListValueFrom(ctx, plan.ElementType(ctx), planElems)
	diags.Append(rebuildDiags...)
	if !rebuildDiags.HasError() {
		*plan = newList
	}
	return diags
}

// fetchAllRRResults auto-paginates through all pages of resource results.
func fetchAllRRResults(ctx context.Context, client *models.ClientWithResponses, sourceID, insightKey string) ([]models.ResourceResult, diag.Diagnostics) {
	var diags diag.Diagnostics
	allResults := make([]models.ResourceResult, 0)
	var pageToken *string

	for {
		params := &models.GetInsightResourceResultsParams{
			PageToken: pageToken,
		}

		resp, err := client.GetInsightResourceResultsWithResponse(ctx, sourceID, insightKey, params)
		if err != nil {
			diags.AddError(
				"Error Reading Insight Resource Results",
				fmt.Sprintf("Could not read resource results for %s/%s: %s", sourceID, insightKey, err.Error()),
			)
			return nil, diags
		}

		if resp.StatusCode() == 404 {
			return nil, diags
		}

		if resp.StatusCode() != 200 || resp.JSON200 == nil {
			diags.AddError(
				"Error Reading Insight Resource Results",
				fmt.Sprintf("Unexpected status %d for resource results %s/%s: %s", resp.StatusCode(), sourceID, insightKey, string(resp.Body)),
			)
			return nil, diags
		}

		allResults = append(allResults, resp.JSON200.ResourceResults...)

		if resp.JSON200.PageToken == nil || *resp.JSON200.PageToken == "" {
			break
		}
		pageToken = resp.JSON200.PageToken
	}

	return allResults, diags
}

func (r *insightResourceResultsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan insightResourceResultsModel
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

	apiReq, buildDiags := buildResourceResultsRequest(ctx, &plan)
	resp.Diagnostics.Append(buildDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	sourceID := models.PostInsightResourceResultsParamsSourceIDPublicApi
	insightKey := plan.InsightKey.ValueString()

	createResp, err := r.client.PostInsightResourceResultsWithResponse(ctx, sourceID, insightKey, nil, *apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Error Creating Insight Resource Results", err.Error())
		return
	}

	if createResp.StatusCode() != 200 || createResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Creating Insight Resource Results",
			fmt.Sprintf("status: %d, body: %s", createResp.StatusCode(), string(createResp.Body)),
		)
		return
	}

	// Plan-first overlay: keep user values, overlay computed fields from response
	resp.Diagnostics.Append(overlayRRComputedFields(ctx, createResp.JSON200.ResourceResults, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// PageToken from response — always set to null after create (we have all results)
	plan.PageToken = types.StringNull()

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *insightResourceResultsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state insightResourceResultsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Capture prior state ordering before refreshing from the API.
	var priorElems []rr.ResourceResultsValue
	if !state.ResourceResults.IsNull() && !state.ResourceResults.IsUnknown() {
		resp.Diagnostics.Append(state.ResourceResults.ElementsAs(ctx, &priorElems, true)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	readTimeout, diags := state.Timeouts.Read(ctx, 2*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	sourceID := state.SourceId.ValueString()
	insightKey := state.InsightKey.ValueString()

	if sourceID == "" {
		sourceID = "public-api"
	}

	allResults, fetchDiags := fetchAllRRResults(ctx, r.client, sourceID, insightKey)
	resp.Diagnostics.Append(fetchDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// nil means 404 — parent insight was deleted externally
	if allResults == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	// Re-order API results to match the prior state ordering.
	// The API may return results in arbitrary order, but Terraform lists are
	// order-sensitive — without this, every refresh would detect drift.
	allResults = reorderResultsToMatchState(allResults, priorElems)

	resp.Diagnostics.Append(mapRRResponseToModel(ctx, allResults, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.PageToken = types.StringNull()
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *insightResourceResultsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan insightResourceResultsModel
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

	apiReq, buildDiags := buildResourceResultsRequest(ctx, &plan)
	resp.Diagnostics.Append(buildDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	sourceID := models.PostInsightResourceResultsParamsSourceIDPublicApi
	insightKey := plan.InsightKey.ValueString()

	updateResp, err := r.client.PostInsightResourceResultsWithResponse(ctx, sourceID, insightKey, nil, *apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Error Updating Insight Resource Results", err.Error())
		return
	}

	if updateResp.StatusCode() != 200 || updateResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Updating Insight Resource Results",
			fmt.Sprintf("status: %d, body: %s", updateResp.StatusCode(), string(updateResp.Body)),
		)
		return
	}

	resp.Diagnostics.Append(overlayRRComputedFields(ctx, updateResp.JSON200.ResourceResults, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	plan.PageToken = types.StringNull()
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *insightResourceResultsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state insightResourceResultsModel
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

	sourceID := models.PostInsightResourceResultsParamsSourceIDPublicApi
	insightKey := state.InsightKey.ValueString()

	// POST with empty array triggers stale resolution, marking all existing
	// resource results as resolved (effectively clearing them).
	emptyBody := models.CreateResourceResultsBody{
		ResourceResults: models.ResourceResults{},
	}

	delResp, err := r.client.PostInsightResourceResultsWithResponse(ctx, sourceID, insightKey, nil, emptyBody)
	if err != nil {
		resp.Diagnostics.AddError("Error Deleting Insight Resource Results", err.Error())
		return
	}

	// 404 = parent insight already gone, treat as success
	if delResp.StatusCode() != 200 && delResp.StatusCode() != 404 {
		resp.Diagnostics.AddError(
			"Error Deleting Insight Resource Results",
			fmt.Sprintf("non-retryable error: %d, body: %s", delResp.StatusCode(), string(delResp.Body)),
		)
	}
}

// reorderResultsToMatchState re-orders API results to match the prior state's
// element ordering (by resource_id). This prevents false drift caused by the
// API returning results in a different order than the plan/state.
// New results (not in prior state) are appended at the end in stable order.
func reorderResultsToMatchState(apiResults []models.ResourceResult, priorElems []rr.ResourceResultsValue) []models.ResourceResult {
	if len(priorElems) == 0 {
		return apiResults
	}

	// Build a position map from prior state: resource_id -> index
	stateOrder := make(map[string]int, len(priorElems))
	for i, elem := range priorElems {
		stateOrder[elem.ResourceId.ValueString()] = i
	}

	// Sort API results: elements present in prior state come first in their
	// original order, new elements come after in stable (original API) order.
	sort.SliceStable(apiResults, func(i, j int) bool {
		posI, okI := stateOrder[apiResults[i].ResourceId]
		posJ, okJ := stateOrder[apiResults[j].ResourceId]

		switch {
		case okI && okJ:
			return posI < posJ // both in state: preserve state order
		case okI:
			return true // i in state, j is new: i comes first
		case okJ:
			return false // j in state, i is new: j comes first
		default:
			return false // both new: preserve API order (stable sort)
		}
	})

	return apiResults
}
