package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// listRowCountKind is what a list endpoint's rowCount counts. The spec's
// wording is not reliable for this, so the kind of each endpoint is measured
// against the API: one request with a small maxResults compared with a full
// crawl.
type listRowCountKind int

const (
	// listRowCountTotal: rowCount is the number of items across all pages, the
	// same on every page. row_count is that total in every mode.
	listRowCountTotal listRowCountKind = iota
	// listRowCountPage: rowCount is the number of items in the response.
	// row_count is the number of items the read returned.
	listRowCountPage
	// listRowCountTotalUnlessPaged: rowCount is the total, but the API sends it
	// only when a single request saw every item. row_count is null when a
	// single page does not hold every item.
	listRowCountTotalUnlessPaged
)

// listRowCount returns the count the API reported, or fetched when it reported
// none. In auto-pagination mode, apiCount comes from the first page and fetched
// is the number of items read over all pages, so it is computed after the loop.
func listRowCount[T ~int | ~int64](apiCount *T, fetched int) types.Int64 {
	if apiCount != nil {
		return types.Int64Value(int64(*apiCount))
	}
	return types.Int64Value(int64(fetched))
}

// setListPaginationDescriptions replaces the generated descriptions of
// max_results, page_token and row_count, which describe the API's paging, with
// what the data source does. See the Pagination Guide for the contract.
// pageTokenNote, if not empty, is appended to the page_token description for
// what only this endpoint's tokens do, such as expiring.
func setListPaginationDescriptions(s *schema.Schema, kind listRowCountKind, pageTokenNote string) diag.Diagnostics {
	return setPaginationDescriptions(s, kind, false, pageTokenNote)
}

// setSinglePageListPaginationDescriptions is setListPaginationDescriptions for a
// data source that reads a single page even when max_results is omitted.
func setSinglePageListPaginationDescriptions(s *schema.Schema, kind listRowCountKind, pageTokenNote string) diag.Diagnostics {
	return setPaginationDescriptions(s, kind, true, pageTokenNote)
}

func setPaginationDescriptions(s *schema.Schema, kind listRowCountKind, singlePageOnly bool, pageTokenNote string) diag.Diagnostics {
	var diags diag.Diagnostics

	maxResults, okMaxResults := s.Attributes["max_results"].(schema.Int64Attribute)
	pageToken, okPageToken := s.Attributes["page_token"].(schema.StringAttribute)
	rowCount, okRowCount := s.Attributes["row_count"].(schema.Int64Attribute)
	if !okMaxResults || !okPageToken || !okRowCount {
		diags.AddError("Unexpected Schema", "Expected max_results and row_count to be number attributes and page_token to be a string attribute.")
		return diags
	}

	if singlePageOnly {
		maxResults.Description = "The maximum number of items to return. " +
			"A single page is read whether or not it is set; without it, the API's default page size applies."
		pageToken.Description = "Token of the page to read. " +
			"After the read, `page_token` holds the token of the next page, which is null after the last page."
	} else {
		maxResults.Description = "The maximum number of items to return. " +
			"Setting it reads a single page and returns the `page_token` of the next one; omit it to fetch every page."
		pageToken.Description = "Token of the page to start at. " +
			"With `max_results` set, a single page is read and `page_token` then holds the token of the next page, which is null after the last page. " +
			"Without `max_results`, every page from this token on is fetched and `page_token` is null after the read."
	}
	if pageTokenNote != "" {
		pageToken.Description += " " + pageTokenNote
	}
	maxResults.MarkdownDescription = maxResults.Description
	s.Attributes["max_results"] = maxResults
	pageToken.MarkdownDescription = pageToken.Description
	s.Attributes["page_token"] = pageToken

	switch kind {
	case listRowCountTotal:
		rowCount.Description = "The number of items across all pages, as reported by the API. " +
			"It is the same whatever `max_results` and `page_token` are, so with either set it can be larger than the number of items returned. " +
			"If the API reports no count, `row_count` is the number of items this read returned instead."
	case listRowCountPage:
		rowCount.Description = "The number of items this read returned: " +
			"with `max_results` set, the items of that page; without it, the items of every page fetched."
		if singlePageOnly {
			rowCount.Description = "The number of items this read returned, which is the items of one page."
		}
	case listRowCountTotalUnlessPaged:
		rowCount.Description = "The number of items across all pages. " +
			"With `max_results` set, it is known only when the page holds every item, and null otherwise. " +
			"Without `max_results`, it is the number of items fetched, which counts only the items from `page_token` on when a `page_token` is set."
	}
	rowCount.MarkdownDescription = rowCount.Description
	s.Attributes["row_count"] = rowCount

	return diags
}
