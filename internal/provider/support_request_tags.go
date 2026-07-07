package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func (r *supportRequestTagsResource) populateState(ctx context.Context, state *supportRequestTagsResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	ticketResp, err := r.client.IdOfTicketGetWithResponse(ctx, state.TicketId.ValueInt64())
	if err != nil {
		diags.AddError(
			"Error Reading Support Request Tags",
			"Could not read support request: "+err.Error(),
		)
		return diags
	}

	if ticketResp.StatusCode() == 404 {
		state.Id = types.StringNull()
		return diags
	}

	if ticketResp.StatusCode() != 200 {
		diags.AddError(
			"Error Reading Support Request Tags",
			"Unexpected status code "+strconv.Itoa(ticketResp.StatusCode())+": "+string(ticketResp.Body),
		)
		return diags
	}

	if ticketResp.JSON200 == nil {
		diags.AddError(
			"Error Reading Support Request Tags",
			"Received empty response body",
		)
		return diags
	}
	return diags
}

func tagsToStringSlice(set types.Set, diags *diag.Diagnostics) []string {
	if set.IsNull() || set.IsUnknown() {
		return nil
	}

	var result []string
	for _, elem := range set.Elements() {
		s, ok := elem.(types.String)
		if !ok {
			diags.AddError("Invalid Tag Element", "Expected types.String in tags set")
			return nil
		}
		result = append(result, s.ValueString())
	}
	return result
}

func diffTags(old, updated []string) (toAdd, toRemove []string) {
	oldMap := make(map[string]bool, len(old))
	for _, t := range old {
		oldMap[t] = true
	}

	updatedMap := make(map[string]bool, len(updated))
	for _, t := range updated {
		updatedMap[t] = true
	}

	for _, t := range updated {
		if !oldMap[t] {
			toAdd = append(toAdd, t)
		}
	}

	for _, t := range old {
		if !updatedMap[t] {
			toRemove = append(toRemove, t)
		}
	}

	return toAdd, toRemove
}

func (r *supportRequestTagsResource) syncTags(ctx context.Context, ticketId int64, oldTags, desiredTags []string, diags *diag.Diagnostics) {
	toAdd, toRemove := diffTags(oldTags, desiredTags)

	if len(toRemove) > 0 {
		removeReq := models.IdOfTicketTagsRemoveJSONRequestBody{
			Tags: toRemove,
		}
		removeResp, err := r.client.IdOfTicketTagsRemoveWithResponse(ctx, ticketId, removeReq)
		if err != nil {
			diags.AddError("Error Syncing Tags", "Could not remove tags: "+err.Error())
			return
		}
		if removeResp.StatusCode() != 200 {
			diags.AddError(
				"Error Syncing Tags",
				fmt.Sprintf("Could not remove tags, status: %d, body: %s", removeResp.StatusCode(), string(removeResp.Body)),
			)
			return
		}
	}

	if len(toAdd) > 0 {
		addReq := models.IdOfTicketTagsAddJSONRequestBody{
			Tags: toAdd,
		}
		addResp, err := r.client.IdOfTicketTagsAddWithResponse(ctx, ticketId, addReq)
		if err != nil {
			diags.AddError("Error Syncing Tags", "Could not add tags: "+err.Error())
			return
		}
		if addResp.StatusCode() != 200 {
			diags.AddError(
				"Error Syncing Tags",
				fmt.Sprintf("Could not add tags, status: %d, body: %s", addResp.StatusCode(), string(addResp.Body)),
			)
			return
		}
	}
}
