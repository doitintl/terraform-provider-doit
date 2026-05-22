package errfmttest

import "fmt"

// Stubs
type diagList struct{}

func (d diagList) AddError(summary, detail string) {}

type httpResponse struct{ Body []byte }

func (r httpResponse) StatusCode() int { return 200 }

type respObj struct{ Diagnostics diagList }

// --- BAD: Generic error path missing both status and body ---

func badMissingBoth(apiResp httpResponse, resp *respObj) {
	if apiResp.StatusCode() != 200 {
		resp.Diagnostics.AddError( // want "must include both apiResp.StatusCode.*and apiResp.Body"
			"Error",
			"something went wrong",
		)
	}
}

// --- BAD: Generic error path has status but missing body ---

func badMissingBody(apiResp httpResponse, resp *respObj) {
	if apiResp.StatusCode() != 200 {
		resp.Diagnostics.AddError( // want "includes status but is missing apiResp.Body"
			"Error",
			fmt.Sprintf("status: %d", apiResp.StatusCode()),
		)
	}
}

// --- BAD: Generic error path has body but missing status ---

func badMissingStatus(apiResp httpResponse, resp *respObj) {
	if apiResp.StatusCode() != 200 {
		resp.Diagnostics.AddError( // want "includes body but is missing apiResp.StatusCode"
			"Error",
			fmt.Sprintf("body: %s", string(apiResp.Body)),
		)
	}
}

// --- GOOD: Generic error path with both status and body ---

func goodBoth(apiResp httpResponse, resp *respObj) {
	if apiResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error",
			fmt.Sprintf("status: %d, body: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
	}
}

// --- GOOD: Compound condition with both status and body ---

func goodCompound(deleteResp httpResponse, resp *respObj) {
	if deleteResp.StatusCode() != 200 && deleteResp.StatusCode() != 204 && deleteResp.StatusCode() != 404 {
		resp.Diagnostics.AddError(
			"Error",
			fmt.Sprintf("status: %d, body: %s", deleteResp.StatusCode(), string(deleteResp.Body)),
		)
	}
}

// --- GOOD: Specific status check (== 404) with semantic message ---
// Not flagged because the status is already known from the condition.

func good404Semantic(apiResp httpResponse, resp *respObj) {
	if apiResp.StatusCode() == 404 {
		resp.Diagnostics.AddError(
			"Not Found",
			"The resource was not found",
		)
	}
}

// --- GOOD: No StatusCode check — non-HTTP error, not flagged ---

func goodNonHTTP(resp *respObj) {
	resp.Diagnostics.AddError(
		"Error",
		"unexpected error: something failed",
	)
}
