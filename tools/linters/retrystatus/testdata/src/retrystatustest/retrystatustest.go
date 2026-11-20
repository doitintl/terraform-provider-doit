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
	Body                      []byte
	HTTPResponse              *http.Response
	JSON200                   *problem
	JSON403                   *problem
	JSON404                   *problem
	JSON4xx                   *problem
	JSON5xx                   *problem
	ApplicationproblemJSON412 *problem
	ApplicationproblemJSON500 *problem
}

func (r *response) StatusCode() int { return 0 }

func (r *response) GetJSON403() *problem { return r.JSON403 } // want `field JSON403 is never seen by resource code`
func (r *response) GetJSON404() *problem { return r.JSON404 }

// unrelated has the same member names but is not a generated response.
type unrelated struct {
	JSON403 *problem
}

func (u *unrelated) StatusCode() int { return 0 }

func useUnrelated(u *unrelated) {
	_ = u.StatusCode() == http.StatusForbidden
	_ = u.JSON403
}

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

	_ = r.GetJSON404()
	_ = r.GetJSON403()              // want `field GetJSON403 is never seen by resource code`
	_ = r.JSON403                   // want `field JSON403 is never seen by resource code`
	_ = r.JSON5xx                   // want `field JSON5xx is never seen by resource code`
	_ = r.ApplicationproblemJSON500 // want `field ApplicationproblemJSON500 is never seen by resource code`
}

func hint(status int) string {
	if status == http.StatusBadRequest { // want `status 400 is never seen by resource code`
		return "bad request"
	}
	if status == http.StatusNotFound { // passed through
		return "not found"
	}
	return ""
}

func hintSwitch(status int) {
	switch status {
	case http.StatusNotFound:
	case http.StatusForbidden: // want `status 403 is never seen by resource code`
	}
}

func outer(status int) { inner(status) }

func inner(code int) {
	_ = code == http.StatusConflict // want `status 409 is never seen by resource code`
}

// unfed is never called with a status, so its comparison is not flagged.
func unfed(n int) bool { return n == http.StatusBadRequest }

func plain(u *unrelated) { unfedToo(u.StatusCode()) }

func unfedToo(n int) bool { return n == http.StatusBadRequest }

func useParams(r *response) {
	_ = hint(r.StatusCode())
	_ = hint(r.StatusCode()) // second call site: the callee is still reported once
	hintSwitch(r.StatusCode())
	outer(r.StatusCode())

	status := r.StatusCode()
	_ = status == http.StatusForbidden // want `status 403 is never seen by resource code`
	_ = hint(status)

	var viaVar = r.StatusCode()
	_ = viaVar != 502 // want `status 502 is never seen by resource code`

	_ = unfed(200)
}

// Overwritten variables no longer hold the response status.
func useOverwritten(r *response) {
	status := r.StatusCode()
	status = 400
	_ = status == http.StatusBadRequest // reassigned: not a response status

	_ = overwrittenParam(r.StatusCode())
}

func overwrittenParam(status int) bool {
	status = 400
	return status == http.StatusBadRequest // reassigned: not a response status
}

type helper struct{}

func (h *helper) methodHint(status int) bool {
	return status == http.StatusTeapot // want `status 418 is never seen by resource code`
}

func useMethodExpr(r *response, h *helper) {
	_ = (*helper).methodHint(h, r.StatusCode())
}
