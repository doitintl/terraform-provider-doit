package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// fetchCurrentTags reads the ticket's current visible tags via GET /tags.
// notFound is true when the ticket does not exist (404).
func (r *supportRequestTagsResource) fetchCurrentTags(ctx context.Context, ticketID int64) (tags []string, notFound bool, diags diag.Diagnostics) {
	tagsResp, err := r.client.ListTicketTagsWithResponse(ctx, ticketID)
	if err != nil {
		diags.AddError(
			"Error Reading Support Request Tags",
			"Could not read tags: "+err.Error(),
		)
		return nil, false, diags
	}

	if tagsResp.StatusCode() == 404 {
		return nil, true, diags
	}

	if tagsResp.StatusCode() != 200 {
		diags.AddError(
			"Error Reading Support Request Tags",
			"Unexpected status code "+strconv.Itoa(tagsResp.StatusCode())+": "+string(tagsResp.Body),
		)
		return nil, false, diags
	}

	if tagsResp.JSON200 == nil {
		diags.AddError(
			"Error Reading Support Request Tags",
			"Received empty response body",
		)
		return nil, false, diags
	}

	if tagsResp.JSON200.Tags != nil {
		tags = *tagsResp.JSON200.Tags
	}
	return tags, false, diags
}

func (r *supportRequestTagsResource) populateState(ctx context.Context, state *supportRequestTagsResourceModel) diag.Diagnostics {
	apiTags, notFound, diags := r.fetchCurrentTags(ctx, state.TicketId.ValueInt64())
	if diags.HasError() {
		return diags
	}
	if notFound {
		state.Id = types.StringNull()
		return diags
	}

	// Preserve the user's tag representation when the API returns a normalized
	// (trim + lowercase) form of the same tag, to avoid false drift. This mirrors
	// the Read-path reconciliation pattern used by normalizeDimensionsType and
	// mergeSentinelValues: the plan is the source of truth for user-configured
	// values, so Read only surfaces genuine external changes.
	priorTags := tagsToStringSlice(state.Tags, &diags)
	if diags.HasError() {
		return diags
	}
	reconciled := reconcileTags(apiTags, priorTags)

	tagsSet, setDiags := types.SetValueFrom(ctx, types.StringType, reconciled)
	diags.Append(setDiags...)
	if diags.HasError() {
		return diags
	}

	state.Tags = tagsSet
	return diags
}

// normalizeTag applies the same normalization the API performs on submitted
// tags (trim + lowercase). See the TagsRequest schema in the OpenAPI spec.
func normalizeTag(tag string) string {
	return strings.ToLower(strings.TrimSpace(tag))
}

// reconcileTags returns the tag set to store in state after a Read. For each tag
// the API returns, if a prior-state tag normalizes to the same value, the user's
// original representation is preserved (avoiding drift from trim/lowercase
// normalization). Tags present in the API response but not in prior state (e.g.
// on import, or added externally) are taken as-is. Tags dropped by the API are
// not carried over, so genuine external removals are still detected as drift.
func reconcileTags(apiTags, priorTags []string) []string {
	byNormalized := make(map[string]string, len(priorTags))
	for _, t := range priorTags {
		byNormalized[normalizeTag(t)] = t
	}

	result := make([]string, 0, len(apiTags))
	for _, a := range apiTags {
		if original, ok := byNormalized[normalizeTag(a)]; ok {
			result = append(result, original)
		} else {
			result = append(result, a)
		}
	}
	return result
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
