package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func (r *webhookEndpointResource) populateState(ctx context.Context, state *webhookEndpointResourceModel) diag.Diagnostics {
	endpointResp, err := r.client.GetWebhookEndpointWithResponse(ctx, state.Id.ValueString())
	if err != nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Webhook Endpoint", "Could not read webhook endpoint ID "+state.Id.ValueString()+": "+err.Error()),
		}
	}

	if endpointResp.StatusCode() == 404 {
		state.Id = types.StringNull()
		return nil
	}

	if endpointResp.StatusCode() != 200 {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Webhook Endpoint", fmt.Sprintf("Unexpected status code %d for webhook endpoint ID %s: %s", endpointResp.StatusCode(), state.Id.ValueString(), string(endpointResp.Body))),
		}
	}

	if endpointResp.JSON200 == nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Webhook Endpoint", "Received empty response body for webhook endpoint ID "+state.Id.ValueString()),
		}
	}

	return mapWebhookEndpointToModel(ctx, endpointResp.JSON200, state)
}

func mapWebhookEndpointToModel(ctx context.Context, resp *models.WebhookEndpoint, state *webhookEndpointResourceModel) (diags diag.Diagnostics) {
	state.Id = types.StringValue(resp.Id)
	state.Name = types.StringValue(resp.Name)
	state.Url = types.StringValue(resp.Url)
	state.Description = types.StringPointerValue(resp.Description)

	if resp.Status != nil {
		state.Status = types.StringValue(string(*resp.Status))
	} else {
		state.Status = types.StringNull()
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

	if resp.Events != nil && len(*resp.Events) > 0 {
		events := make([]string, len(*resp.Events))
		for i, e := range *resp.Events {
			events[i] = string(e)
		}
		eventsList, d := types.ListValueFrom(ctx, types.StringType, events)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}
		state.Events = eventsList
	} else {
		var d diag.Diagnostics
		state.Events, d = types.ListValueFrom(ctx, types.StringType, []string{})
		diags.Append(d...)
	}

	return diags
}

func overlayWebhookEndpointComputedFields(ctx context.Context, apiResp *models.WebhookEndpoint, plan *webhookEndpointResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	// Phase 1: Build fully-resolved state from API response.
	resolved := *plan
	diags.Append(mapWebhookEndpointToModel(ctx, apiResp, &resolved)...)
	if diags.HasError() {
		return diags
	}

	// Phase 2: Overlay known plan values on top of resolved state.

	// ── Computed-only stable fields: always from resolved ──
	plan.Id = resolved.Id
	plan.CreateTime = resolved.CreateTime

	// ── Computed-only volatile: guard with IsUnknown() ──
	if plan.UpdateTime.IsUnknown() {
		plan.UpdateTime = resolved.UpdateTime //nolint:overlaycheck // guarded to avoid inconsistent result on Update
	}

	// ── Name, Url: Required — never touch ──

	// ── Description: Optional+Computed clearable (Cat A) ──
	if plan.Description.IsUnknown() {
		plan.Description = resolved.Description
	}

	// ── Events: Optional+Computed clearable list (Cat A) ──
	if plan.Events.IsUnknown() {
		plan.Events = resolved.Events
	}

	// ── Status: Computed-only — always from resolved ──
	plan.Status = resolved.Status

	return diags
}

func (plan *webhookEndpointResourceModel) toCreateRequest(ctx context.Context) (models.CreateWebhookEndpointRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	req := models.CreateWebhookEndpointRequest{
		Name: plan.Name.ValueString(),
		Url:  plan.Url.ValueString(),
	}

	if !plan.Description.IsUnknown() {
		req.Description = plan.Description.ValueStringPointer()
	}

	if !plan.Events.IsNull() && !plan.Events.IsUnknown() {
		var eventStrings []string
		diags.Append(plan.Events.ElementsAs(ctx, &eventStrings, false)...)
		if diags.HasError() {
			return req, diags
		}
		events := make([]models.CreateWebhookEndpointRequestEvents, len(eventStrings))
		for i, e := range eventStrings {
			events[i] = models.CreateWebhookEndpointRequestEvents(e)
		}
		req.Events = &events
	}

	return req, diags
}

func (plan *webhookEndpointResourceModel) toUpdateRequest(ctx context.Context) (models.UpdateWebhookEndpointRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	req := models.UpdateWebhookEndpointRequest{
		Name: plan.Name.ValueStringPointer(),
		Url:  plan.Url.ValueStringPointer(),
	}

	if !plan.Description.IsUnknown() {
		req.Description = plan.Description.ValueStringPointer()
	}

	// Events: Optional+Computed clearable list (Cat A)
	// Null or empty list means "subscribe to all events" — send explicit empty list to clear
	if plan.Events.IsNull() || (!plan.Events.IsUnknown() && len(plan.Events.Elements()) == 0) {
		emptyEvents := []models.UpdateWebhookEndpointRequestEvents{}
		req.Events = &emptyEvents
	} else if !plan.Events.IsUnknown() {
		var eventStrings []string
		diags.Append(plan.Events.ElementsAs(ctx, &eventStrings, false)...)
		if diags.HasError() {
			return req, diags
		}
		events := make([]models.UpdateWebhookEndpointRequestEvents, len(eventStrings))
		for i, e := range eventStrings {
			events[i] = models.UpdateWebhookEndpointRequestEvents(e)
		}
		req.Events = &events
	}

	return req, diags
}
