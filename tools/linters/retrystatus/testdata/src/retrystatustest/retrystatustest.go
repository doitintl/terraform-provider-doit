package retrystatustest

import "net/http"

type DCIRetryClient struct{}

// Do mirrors the status switch of the production client: the case clause that
// returns the response is the pass-through set.
func (c *DCIRetryClient) Do(resp *http.Response) (*http.Response, error) {
	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		return nil, nil
	case http.StatusOK, http.StatusCreated,
		http.StatusNotFound,
		http.StatusPreconditionFailed,
		425:
		return resp, nil
	default:
		return nil, nil
	}
}

type problem struct{}

type response struct {
	JSON200                   *problem
	JSON403                   *problem
	JSON404                   *problem
	JSON4xx                   *problem
	JSON5xx                   *problem
	ApplicationproblemJSON412 *problem
	ApplicationproblemJSON500 *problem
}

func (r *response) StatusCode() int { return 0 }

func use(r *response) {
	_ = r.StatusCode() == http.StatusNotFound // passed through
	_ = r.StatusCode() != 412                 // passed through
	_ = r.StatusCode() >= 400                 // range check is valid
	_ = r.StatusCode() == http.StatusOK

	_ = r.StatusCode() == http.StatusForbidden // want `status 403 is never seen by resource code`
	_ = 500 == r.StatusCode()                  // want `status 500 is never seen by resource code`
	_ = r.StatusCode() != 429                  // want `status 429 is never seen by resource code`

	switch r.StatusCode() {
	case http.StatusOK, http.StatusNotFound:
	case http.StatusConflict: // want `status 409 is never seen by resource code`
	}

	_ = r.JSON200
	_ = r.JSON404
	_ = r.JSON4xx // class contains passed-through statuses
	_ = r.ApplicationproblemJSON412

	_ = r.JSON403                   // want `field JSON403 is never seen by resource code`
	_ = r.JSON5xx                   // want `field JSON5xx is never seen by resource code`
	_ = r.ApplicationproblemJSON500 // want `field ApplicationproblemJSON500 is never seen by resource code`
}
