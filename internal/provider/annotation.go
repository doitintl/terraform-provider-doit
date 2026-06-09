// Package provider implements the DoiT Terraform provider.
package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// populateState fetches the annotation from the API and populates the Terraform state.
// On 404, state.Id is set to null to signal Terraform to remove the resource from state.
func (r *annotationResource) populateState(ctx context.Context, state *annotationResourceModel) diag.Diagnostics {
	annotationResp, err := r.client.GetAnnotationWithResponse(ctx, state.Id.ValueString())
	if err != nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Annotation", "Could not read annotation ID "+state.Id.ValueString()+": "+err.Error()),
		}
	}

	if annotationResp.StatusCode() == 404 {
		state.Id = types.StringNull()
		return nil
	}

	if annotationResp.StatusCode() != 200 {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Annotation", fmt.Sprintf("Unexpected status code %d for annotation ID %s: %s", annotationResp.StatusCode(), state.Id.ValueString(), string(annotationResp.Body))),
		}
	}

	if annotationResp.JSON200 == nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Annotation", "Received empty response body for annotation ID "+state.Id.ValueString()),
		}
	}

	return mapAnnotationToModel(ctx, annotationResp.JSON200, state)
}

// mapAnnotationToModel maps the API response to the Terraform model.
func mapAnnotationToModel(ctx context.Context, resp *models.AnnotationListItem, state *annotationResourceModel) (diags diag.Diagnostics) {
	state.Id = types.StringValue(resp.Id)
	state.Content = types.StringValue(resp.Content)

	// Check if the timestamp has changed semantically before overwriting to preserve user formatting
	existingTime, err := time.Parse(time.RFC3339, state.Timestamp.ValueString())
	if err == nil && existingTime.Equal(resp.Timestamp) {
		// Keep the existing string to avoid diffs if the time is the same
	} else {
		state.Timestamp = types.StringValue(resp.Timestamp.UTC().Format(time.RFC3339))
	}

	if resp.CreateTime != nil {
		state.CreateTime = types.StringValue(resp.CreateTime.Format(time.RFC3339))
	} else {
		state.CreateTime = types.StringNull()
	}

	if resp.UpdateTime != nil {
		state.UpdateTime = types.StringValue(resp.UpdateTime.Format(time.RFC3339))
	} else {
		state.UpdateTime = types.StringNull()
	}

	// Map labels — API returns []LabelInfo, but we store just the IDs.
	if resp.Labels != nil && len(*resp.Labels) > 0 {
		labelIDs := make([]string, len(*resp.Labels))
		for i, label := range *resp.Labels {
			labelIDs[i] = label.Id
		}
		labelsList, d := types.ListValueFrom(ctx, types.StringType, labelIDs)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}
		state.Labels = labelsList
	} else {
		var emptyDiags diag.Diagnostics
		state.Labels, emptyDiags = types.ListValueFrom(ctx, types.StringType, []string{})
		diags.Append(emptyDiags...)
	}

	// Map reports.
	if resp.Reports != nil && len(*resp.Reports) > 0 {
		reportsList, d := types.ListValueFrom(ctx, types.StringType, *resp.Reports)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}
		state.Reports = reportsList
	} else {
		var emptyDiags diag.Diagnostics
		state.Reports, emptyDiags = types.ListValueFrom(ctx, types.StringType, []string{})
		diags.Append(emptyDiags...)
	}

	return diags
}

// overlayAnnotationComputedFields uses the two-phase overlay pattern to reconcile
// the Terraform plan with the API response after Create/Update.
//
// Phase 1 (Resolve): Build a fully-resolved state from the API response using
// mapAnnotationToModel — the same mapping function used by Read.
//
// Phase 2 (Overlay): Walk the plan field-by-field. Known values are preserved.
// Unknown values are replaced with the resolved counterpart.
func overlayAnnotationComputedFields(ctx context.Context, apiResp *models.AnnotationListItem, plan *annotationResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	// Phase 1: Build fully-resolved state from API response.
	resolved := *plan
	diags.Append(mapAnnotationToModel(ctx, apiResp, &resolved)...)
	if diags.HasError() {
		return diags
	}

	// Phase 2: Overlay known plan values on top of resolved state.

	// ── Computed-only fields: always from resolved ──
	plan.Id = resolved.Id
	plan.CreateTime = resolved.CreateTime
	// update_time: Computed-only but changes server-side on every Update.
	// On Create the plan value is Unknown → overlay fills from API.
	// On Update the framework copies the prior state (Known) into the plan.
	// Unconditional assignment would cause "inconsistent result" because the
	// provider would return a newer timestamp than what was in the plan.
	if plan.UpdateTime.IsUnknown() { //nolint:overlaycheck // guarded to avoid inconsistent result on Update
		plan.UpdateTime = resolved.UpdateTime
	}

	// ── Content, Timestamp: Required — never touch ──

	// ── Labels: Optional+Computed clearable list ──
	if plan.Labels.IsUnknown() {
		plan.Labels = resolved.Labels
	}

	// ── Reports: Optional+Computed clearable list ──
	if plan.Reports.IsUnknown() {
		plan.Reports = resolved.Reports
	}

	return diags
}

// toCreateRequest converts the TF model to a CreateAnnotationRequest.
func (plan *annotationResourceModel) toCreateRequest(ctx context.Context) (models.CreateAnnotationRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	timestamp, err := time.Parse(time.RFC3339, plan.Timestamp.ValueString())
	if err != nil {
		diags.AddError(
			"Error Parsing Timestamp",
			fmt.Sprintf("Could not parse timestamp '%s': %s", plan.Timestamp.ValueString(), err.Error()),
		)
		return models.CreateAnnotationRequest{}, diags
	}

	req := models.CreateAnnotationRequest{
		Content:   plan.Content.ValueString(),
		Timestamp: timestamp,
	}

	// Handle optional labels list
	if !plan.Labels.IsNull() && !plan.Labels.IsUnknown() {
		var labels []string
		diags.Append(plan.Labels.ElementsAs(ctx, &labels, false)...)
		if diags.HasError() {
			return req, diags
		}
		req.Labels = &labels
	}

	// Handle optional reports list
	if !plan.Reports.IsNull() && !plan.Reports.IsUnknown() {
		var reports []string
		diags.Append(plan.Reports.ElementsAs(ctx, &reports, false)...)
		if diags.HasError() {
			return req, diags
		}
		req.Reports = &reports
	}

	return req, diags
}

// toUpdateRequest converts the TF model to an UpdateAnnotationRequest.
func (plan *annotationResourceModel) toUpdateRequest(ctx context.Context) (models.UpdateAnnotationRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	timestamp, err := time.Parse(time.RFC3339, plan.Timestamp.ValueString())
	if err != nil {
		diags.AddError(
			"Error Parsing Timestamp",
			fmt.Sprintf("Could not parse timestamp '%s': %s", plan.Timestamp.ValueString(), err.Error()),
		)
		return models.UpdateAnnotationRequest{}, diags
	}

	req := models.UpdateAnnotationRequest{
		Content:   new(plan.Content.ValueString()),
		Timestamp: &timestamp,
	}

	// Handle optional labels list
	if !plan.Labels.IsNull() && !plan.Labels.IsUnknown() {
		var labels []string
		diags.Append(plan.Labels.ElementsAs(ctx, &labels, false)...)
		if diags.HasError() {
			return req, diags
		}
		req.Labels = &labels
	}

	// Handle optional reports list
	if !plan.Reports.IsNull() && !plan.Reports.IsUnknown() {
		var reports []string
		diags.Append(plan.Reports.ElementsAs(ctx, &reports, false)...)
		if diags.HasError() {
			return req, diags
		}
		req.Reports = &reports
	}

	return req, diags
}
