// Package provider — shared plumbing for the async report execution API.
//
// Cloud Analytics reports run as async operations (CMP-44423): the caller
// submits a run, receives 202 with an operation handle, polls until the
// operation reaches a terminal state, then fetches the result from a separate
// endpoint. This file holds the parts both report data sources share; each one
// supplies only its own submit call.
//
// Because no single HTTP request stays open while the report runs, the API's
// ~120s edge timeout does not bound how long a report may take, and neither
// does the provider's request_timeout. The only limit is the caller's Read
// timeout, which covers the whole submit-poll-fetch cycle; when it expires the
// operation is cancelled rather than left running.
package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
	"uuid"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// asyncSubmission is the part of a submit response the shared logic needs,
// normalized so run-by-id and inline runs can share one code path despite
// having different generated response types.
type asyncSubmission struct {
	StatusCode  int
	OperationID string
	Body        []byte
}

// asyncSubmitFunc issues one submit attempt with the given Idempotency-Key.
type asyncSubmitFunc func(ctx context.Context, idempotencyKey string) (asyncSubmission, error)

// asyncOperationID picks the operation ID out of a submit response. A real run
// answers 202, but the generated client models 200 (the dry-run shape) with the
// same envelope, so both are checked rather than assuming which one is set.
func asyncOperationID(envelopes ...*models.AsyncOperationResponse) string {
	for _, envelope := range envelopes {
		if envelope != nil && envelope.OperationId != nil {
			return *envelope.OperationId
		}
	}
	return ""
}

// newIdempotencyKey mints a key for a single submit attempt.
//
// Version 7 is time-ordered, so keys sort by when they were minted — which makes
// correlating one with a log line or an operation record straightforward.
func newIdempotencyKey() string {
	return uuid.NewV7().String()
}

// submitAsyncReport submits a report run and returns the operation ID.
//
// Every submission carries a fresh Idempotency-Key. The key only guards against
// a submission being applied twice; deduplication itself is content-based, so an
// identical config issued concurrently attaches to the operation already in
// flight rather than starting a second one.
func submitAsyncReport(
	ctx context.Context,
	client *models.ClientWithResponses,
	what string,
	submit asyncSubmitFunc,
) (string, diag.Diagnostics) {
	var diags diag.Diagnostics

	idempotencyKey := newIdempotencyKey()

	sub, err := submit(ctx, idempotencyKey)
	if err != nil {
		detail := fmt.Sprintf("Could not submit the %s run: %s", what, err.Error())
		if ctx.Err() != nil {
			// The server may have accepted the submission before we stopped
			// waiting for its response, leaving a run we never learned the ID
			// of. Recover it before reporting the failure, and say what became
			// of it — the user is being told the submit failed, so a run that
			// was created and stopped is part of that story.
			outcome, cleanUpDiags := cleanUpAbandonedSubmit(ctx, client, idempotencyKey, submit)
			diags.Append(cleanUpDiags...)
			if outcome != "" {
				detail += " " + outcome
			}
		}
		diags.AddError("Error Running "+what, detail)
		return "", diags
	}

	if sub.StatusCode != http.StatusAccepted && sub.StatusCode != http.StatusOK {
		diags.AddError(
			"Error Running "+what,
			asyncSubmitErrorDetail(what, sub),
		)
		return "", diags
	}

	// An accepted submission always carries an operation ID, including on a
	// replayed Idempotency-Key. Without one there is nothing to poll or cancel,
	// so report it rather than leaving a run we cannot reach.
	if sub.OperationID == "" {
		diags.AddError(
			"Error Running "+what,
			fmt.Sprintf("The API accepted the %s run but returned no operation ID, status: %d, body: %s. "+
				"The report may be running server-side without the provider being able to track it.",
				what, sub.StatusCode, string(sub.Body)),
		)
		return "", diags
	}

	return sub.OperationID, diags
}

// cleanUpAbandonedSubmit recovers and cancels an operation whose submission was
// accepted but whose response never reached us.
//
// Replaying the original Idempotency-Key is what makes this possible: the API
// answers a replay with the operation the first attempt created, so the ID can
// be recovered and the run stopped. The replay runs on a context detached from
// the expired one, since the whole point is that ctx is already dead.
//
// If the server never actually received the first attempt, the replay is treated
// as a fresh submission and creates an operation — which is then cancelled here
// anyway. That trades a briefly-created run for the guarantee that nothing is
// left executing, which is the better failure mode of the two.
// It returns a phrase describing what became of the recovered operation, empty
// when there was nothing to recover.
func cleanUpAbandonedSubmit(
	ctx context.Context,
	client *models.ClientWithResponses,
	idempotencyKey string,
	submit asyncSubmitFunc,
) (string, diag.Diagnostics) {
	var diags diag.Diagnostics

	recoverCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), asyncCancelGrace)
	defer stop()

	sub, err := submit(recoverCtx, idempotencyKey)
	if err != nil || sub.OperationID == "" {
		// Nothing recoverable: either the submission never landed, or the
		// replay itself failed. Both are logged rather than warned about,
		// since in the common case there is no operation to worry about.
		tflog.Debug(ctx, "Could not determine whether an abandoned submission created an operation", map[string]any{
			"error": err,
		})
		return "", diags
	}

	outcome, cancelDiags := cancelAsyncOperation(ctx, client, sub.OperationID)
	return outcome, append(diags, cancelDiags...)
}

// cancelAsyncOperation issues a best-effort cancel for operationID on a context
// detached from ctx, because every caller reaches here with ctx already expired
// or cancelled — a cancel issued on it would fail immediately.
//
// It returns a phrase describing what became of the operation, empty when
// cancellation did not demonstrably happen, plus a warning in that case.
//
// Note that operations coalesce by config content, so concurrent reads of the
// same report share one, and cancelling it fails every caller awaiting it. This
// is intentional API behavior, confirmed with the API team: the 202 carries no
// signal distinguishing "created" from "attached", so ownership is not something
// a client can establish. Worth re-checking before widening where this is
// called from.
func cancelAsyncOperation(
	ctx context.Context,
	client *models.ClientWithResponses,
	operationID string,
) (string, diag.Diagnostics) {
	var diags diag.Diagnostics

	cancelCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), asyncCancelGrace)
	defer stop()

	cancelResp, err := client.CancelAsyncOperationWithResponse(cancelCtx, operationID,
		&models.CancelAsyncOperationParams{IdempotencyKey: newIdempotencyKey()})

	switch {
	case err != nil:
		diags.AddWarning(
			"Could Not Cancel Report Operation",
			fmt.Sprintf("Operation %s could not be canceled: %s. "+
				"It may still be running; cancel it from the DoiT console if needed.",
				operationID, err.Error()),
		)
		return "", diags

	case cancelResp.StatusCode() != http.StatusOK:
		diags.AddWarning(
			"Could Not Cancel Report Operation",
			fmt.Sprintf("Operation %s could not be canceled, status: %d, body: %s. "+
				"It may still be running; cancel it from the DoiT console if needed.",
				operationID, cancelResp.StatusCode(), string(cancelResp.Body)),
		)
		return "", diags
	}

	// Cancelling is idempotent: an operation that already reached a terminal
	// state comes back unchanged rather than as "canceled". Report the state the
	// API returned instead of assuming the cancel took effect.
	outcome := fmt.Sprintf("A cancellation request was accepted for operation %s.", operationID)
	if cancelResp.JSON200 != nil && cancelResp.JSON200.Status != nil {
		status := *cancelResp.JSON200.Status
		if status == models.AsyncOperationResponseStatusCanceled {
			outcome = fmt.Sprintf("Operation %s was canceled.", operationID)
		} else {
			outcome = fmt.Sprintf("Operation %s had already finished (%s) and was left as-is.",
				operationID, status)
		}
	}

	tflog.Debug(ctx, "Requested cancellation of async report operation", map[string]any{
		"operation_id": operationID,
		"outcome":      outcome,
	})
	return outcome, diags
}

// asyncSubmitErrorDetail builds the detail for a failed submit, always
// including status and body (required by the errformat linter, and the only
// reliable error content — the shared error schema is a bare {"error": string}
// that most of these endpoints do not actually use).
func asyncSubmitErrorDetail(what string, sub asyncSubmission) string {
	detail := fmt.Sprintf("Could not submit the %s run, status: %d, body: %s",
		what, sub.StatusCode, string(sub.Body))
	if sub.StatusCode == http.StatusNotFound {
		detail += "\n\nA 404 here means either the report does not exist, or this account " +
			"cannot reach the async report execution API."
	}
	return detail
}

// awaitAsyncReport polls operationID to a terminal state and returns its result.
//
// If ctx expires or is cancelled while the operation is still running, the
// operation is cancelled server-side before returning, so an abandoned run does
// not keep burning query capacity. This covers both the Read timeout elapsing
// and the user interrupting Terraform.
func awaitAsyncReport(
	ctx context.Context,
	client *models.ClientWithResponses,
	what, operationID string,
) (*models.GetAsyncOperationResults200Response, diag.Diagnostics) {
	var diags diag.Diagnostics

	for {
		pollResp, err := client.GetAsyncOperationWithResponse(ctx, operationID)
		if err != nil {
			if ctx.Err() != nil {
				return nil, asyncRunAborted(ctx, client, what, operationID)
			}
			diags.AddError(
				"Error Running "+what,
				fmt.Sprintf("Could not poll operation %s: %s", operationID, err.Error()),
			)
			return nil, diags
		}

		if pollResp.StatusCode() != http.StatusOK || pollResp.JSON200 == nil {
			// A 404 is the one status here with a non-obvious cause, so it gets
			// an explanation appended to the raw status and body.
			var hint string
			if pollResp.StatusCode() == http.StatusNotFound {
				hint = "\n\nThe operation no longer exists — it may have expired, " +
					"or it belongs to a different user or tenant."
			}
			diags.AddError(
				"Error Running "+what,
				fmt.Sprintf("Could not poll operation %s, status: %d, body: %s%s",
					operationID, pollResp.StatusCode(), string(pollResp.Body), hint),
			)
			return nil, diags
		}

		switch pollResp.JSON200.Status {
		case models.AsyncOperationPollResponseStatusSucceeded:
			return fetchAsyncReportResults(ctx, client, what, operationID)

		case models.AsyncOperationPollResponseStatusFailed:
			diags.AddError(
				"Error Running "+what,
				asyncFailureDetail(operationID, pollResp.JSON200.Error),
			)
			return nil, diags

		case models.AsyncOperationPollResponseStatusCanceled:
			diags.AddError(
				"Error Running "+what,
				fmt.Sprintf("Operation %s was canceled before it produced a result.", operationID),
			)
			return nil, diags

		case models.AsyncOperationPollResponseStatusPending,
			models.AsyncOperationPollResponseStatusRunning:
			// Non-terminal; fall through to the wait below.

		default:
			// Generated enums are plain strings with no unmarshal validation, so
			// an unexpected value lands here rather than being rejected earlier.
			// Treating it as non-terminal would poll until the read timeout and
			// then cancel, turning a contract violation into a silent stall.
			diags.AddError(
				"Error Running "+what,
				fmt.Sprintf("Operation %s reported an unrecognized status %q, status: %d, body: %s",
					operationID, pollResp.JSON200.Status, pollResp.StatusCode(), string(pollResp.Body)),
			)
			return nil, diags
		}

		// Still pending or running — wait, honoring the API's own guidance.
		wait := asyncPollInitialInterval
		if pollResp.Headers200 != nil && pollResp.Headers200.RetryAfter != nil {
			if suggested, ok := parseRetryAfterBounded(
				*pollResp.Headers200.RetryAfter, time.Now(),
				asyncPollInitialInterval, asyncPollMaxInterval,
			); ok {
				wait = suggested
			}
		}

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, asyncRunAborted(ctx, client, what, operationID)
		case <-timer.C:
		}
	}
}

// fetchAsyncReportResults retrieves the result of a succeeded operation.
//
// 425 is a declared response of this endpoint, so it is retried rather than
// failed on. It is not expected here: the result is stored before the operation
// transitions to succeeded, so by the time this is called the result should
// exist. Retrying briefly costs little and beats failing a report that has
// already finished. The spec documents a Retry-After on a 425 but none is sent,
// so the standard poll interval applies.
func fetchAsyncReportResults(
	ctx context.Context,
	client *models.ClientWithResponses,
	what, operationID string,
) (*models.GetAsyncOperationResults200Response, diag.Diagnostics) {
	var diags diag.Diagnostics

	for {
		resultsResp, err := client.GetAsyncOperationResultsWithResponse(ctx, operationID)
		if err != nil {
			diags.AddError(
				"Error Running "+what,
				fmt.Sprintf("Could not fetch results for operation %s: %s", operationID, err.Error()),
			)
			return nil, diags
		}

		if resultsResp.StatusCode() == http.StatusTooEarly {
			timer := time.NewTimer(asyncPollInitialInterval)
			select {
			case <-ctx.Done():
				// The operation already succeeded, so there is nothing running
				// to cancel — only the result fetch ran out of time.
				timer.Stop()
				diags.AddError(
					"Error Running "+what,
					fmt.Sprintf("Timed out waiting for the result of operation %s to become available: %s. "+
						"The operation succeeded, so retrying shortly should return it.",
						operationID, ctx.Err()),
				)
				return nil, diags
			case <-timer.C:
			}
			continue
		}

		if resultsResp.StatusCode() != http.StatusOK || resultsResp.JSON200 == nil {
			diags.AddError(
				"Error Running "+what,
				fmt.Sprintf("Could not fetch results for operation %s, status: %d, body: %s",
					operationID, resultsResp.StatusCode(), string(resultsResp.Body)),
			)
			return nil, diags
		}

		return resultsResp.JSON200, diags
	}
}

// asyncRunAborted cancels a still-running operation and reports why the wait
// ended, distinguishing an exhausted read timeout from a deliberate interrupt.
// Only the former warrants advice about raising the timeout; offering it to
// someone who pressed Ctrl-C points them at the wrong thing.
//
// A cancel that fails is reported as a warning rather than an error: the reason
// the run stopped is what the user needs to act on, and the warning names the
// operation so it can be cancelled from the console.
func asyncRunAborted(
	ctx context.Context,
	client *models.ClientWithResponses,
	what, operationID string,
) diag.Diagnostics {
	outcome, diags := cancelAsyncOperation(ctx, client, operationID)
	if outcome == "" {
		// Cancellation did not demonstrably happen, so say only what is known.
		// The accompanying warning explains why.
		outcome = fmt.Sprintf("Operation %s may still be running.", operationID)
	}

	if errors.Is(ctx.Err(), context.Canceled) {
		diags.AddError(
			"Canceled Running "+what,
			fmt.Sprintf("Running the %s was interrupted: %s. %s", what, ctx.Err(), outcome),
		)
		return diags
	}

	diags.AddError(
		"Timed Out Running "+what,
		fmt.Sprintf("The %s did not finish within the read timeout: %s. %s\n\n"+
			"Increase the timeout for slow reports, for example:\n\n"+
			"  timeouts = {\n    read = \"30m\"\n  }",
			what, ctx.Err(), outcome),
	)
	return diags
}

// asyncFailureDetail renders a failed operation's error, which carries more
// detail than the HTTP layer does.
func asyncFailureDetail(operationID string, opErr *models.AsyncOperationError) string {
	if opErr == nil {
		return fmt.Sprintf("Operation %s failed, but the API reported no error detail.", operationID)
	}

	detail := fmt.Sprintf("Operation %s failed: %s (code: %s, status: %d)",
		operationID, opErr.Title, opErr.Code, opErr.Status)
	if opErr.Detail != nil && *opErr.Detail != "" {
		detail += "\n\n" + *opErr.Detail
	}
	return detail
}
