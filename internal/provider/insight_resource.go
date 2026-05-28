package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_insight"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
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

	// Add UseStateForUnknown to stable Computed-only fields
	for _, field := range []string{"source", "source_id", "insight_key"} {
		if attr, ok := s.Attributes[field].(schema.StringAttribute); ok {
			attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
			s.Attributes[field] = attr
		}
	}

	// Validate source_id — the API only accepts "public-api" today
	if attr, ok := s.Attributes["source_id"].(schema.StringAttribute); ok {
		attr.Validators = append(attr.Validators, stringvalidator.OneOf(
			string(models.PostInsightResultParamsSourceIDPublicApi),
		))
		s.Attributes["source_id"] = attr
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

// toInsightRequest constructs the API request body from the Terraform plan.
// Resource results are now managed by the separate doit_insight_resource_results resource.
func (plan *insightResourceModel) toInsightRequest(ctx context.Context) (*models.InsightMetadataRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	req := models.InsightMetadataRequest{
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

	// Status (Optional+Computed) — sets the display status inline
	if !plan.Status.IsNull() && !plan.Status.IsUnknown() {
		v := models.DisplayStatus(plan.Status.ValueString())
		req.Status = &v
	}

	// DismissalDetails (Optional+Computed) — required when status is "dismissed"
	if !plan.DismissalDetails.IsNull() && !plan.DismissalDetails.IsUnknown() {
		dd := models.DismissalDetails{}
		if !plan.DismissalDetails.Comment.IsNull() && !plan.DismissalDetails.Comment.IsUnknown() {
			v := plan.DismissalDetails.Comment.ValueString()
			dd.Comment = &v
		}
		if !plan.DismissalDetails.Reason.IsNull() && !plan.DismissalDetails.Reason.IsUnknown() {
			v := models.DismissalDetailsReason(plan.DismissalDetails.Reason.ValueString())
			dd.Reason = &v
		}
		req.DismissalDetails = &dd
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
	plan.DisplayStatus = resolved.DisplayStatus
	plan.LastStatusChange = resolved.LastStatusChange
	plan.LastUpdated = resolved.LastUpdated
	plan.Summary = resolved.Summary
	plan.Tags = resolved.Tags

	// Optional+Computed: resolve only when unknown
	if plan.SourceId.IsUnknown() {
		plan.SourceId = resolved.SourceId
	}
	if plan.InsightKey.IsUnknown() {
		plan.InsightKey = resolved.InsightKey
	}
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
	if plan.Status.IsUnknown() {
		plan.Status = resolved.Status
	}
	if plan.DismissalDetails.IsUnknown() {
		plan.DismissalDetails = resolved.DismissalDetails
	} else if !plan.DismissalDetails.IsNull() {
		overlayInsightDismissalDetails(&resolved.DismissalDetails, &plan.DismissalDetails)
	}

	return diags
}

// overlayInsightDismissalDetails resolves Unknown subfields in the "dismissal_details" SingleNestedAttribute.
func overlayInsightDismissalDetails(resolved, plan *resource_insight.DismissalDetailsValue) {
	if plan.Comment.IsUnknown() {
		plan.Comment = resolved.Comment
	}
	if plan.Reason.IsUnknown() {
		plan.Reason = resolved.Reason
	}
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

	apiReq, buildDiags := plan.toInsightRequest(ctx)
	resp.Diagnostics.Append(buildDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// sourceID is always "public-api" for API-created insights
	sourceID := models.PostInsightResultParamsSourceIDPublicApi
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

	diags := r.populateState(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Handle externally deleted resource (populateState sets InsightKey to null on 404)
	if state.InsightKey.IsNull() {
		resp.State.RemoveResource(ctx)
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

	apiReq, buildDiags := plan.toInsightRequest(ctx)
	resp.Diagnostics.Append(buildDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	sourceID := models.PostInsightResultParamsSourceID(plan.SourceId.ValueString())
	if sourceID == "" {
		sourceID = models.PostInsightResultParamsSourceIDPublicApi
	}
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

	sourceID := models.DeleteInsightResultParamsSourceID(state.SourceId.ValueString())
	if sourceID == "" {
		sourceID = models.DeleteInsightResultParamsSourceIDPublicApi
	}
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
		// status mirrors display_status — it's the user-facing write field
		state.Status = types.StringValue(string(*resp.DisplayStatus))
	} else {
		state.DisplayStatus = types.StringNull()
		state.Status = types.StringNull()
	}

	// Use stringPtrOrNull to normalize empty strings ("") to null.
	// The API returns "" for unset optional fields due to Go's zero-value
	// serialization (domain uses string, not *string). This keeps the insight
	// resource consistent with the resource results resource.
	state.DetailedDescriptionMdx = stringPtrOrNull(resp.DetailedDescriptionMdx)
	state.EasyWinDescription = stringPtrOrNull(resp.EasyWinDescription)
	state.ReportUrl = stringPtrOrNull(resp.ReportUrl)
	state.CloudFlowTemplateId = stringPtrOrNull(resp.CloudFlowTemplateId)

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
		var catDiags diag.Diagnostics
		state.Categories, catDiags = types.ListValueFrom(ctx, types.StringType, []string{})
		diags.Append(catDiags...)
	}

	// Tags
	if resp.Tags != nil {
		tagList, tagDiags := types.ListValueFrom(ctx, types.StringType, *resp.Tags)
		diags.Append(tagDiags...)
		state.Tags = tagList
	} else {
		var tagDiags diag.Diagnostics
		state.Tags, tagDiags = types.ListValueFrom(ctx, types.StringType, []string{})
		diags.Append(tagDiags...)
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

	// DismissalDetails (Optional+Computed nested object)
	if resp.DismissalDetails != nil {
		reasonVal := types.StringNull()
		if resp.DismissalDetails.Reason != nil {
			reasonVal = types.StringValue(string(*resp.DismissalDetails.Reason))
		}
		commentVal := types.StringNull()
		if resp.DismissalDetails.Comment != nil {
			commentVal = types.StringValue(*resp.DismissalDetails.Comment)
		}

		ddVal, ddDiags := resource_insight.NewDismissalDetailsValue(
			resource_insight.DismissalDetailsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"reason":  reasonVal,
				"comment": commentVal,
			},
		)
		diags.Append(ddDiags...)
		state.DismissalDetails = ddVal
	} else {
		state.DismissalDetails = resource_insight.NewDismissalDetailsValueNull()
	}

	return diags
}
