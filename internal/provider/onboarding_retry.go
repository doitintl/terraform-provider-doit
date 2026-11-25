package provider

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// onboardingRetryInterval is the wait between two attempts of an onboarding
// request that the API refused with a retryable error, such as access that a
// module granted seconds earlier and that has not propagated yet.
const onboardingRetryInterval = 15 * time.Second

// onboardingProblem holds the fields of an RFC 9457 Problem Details body that
// decide whether an onboarding request is repeated.
type onboardingProblem struct {
	Code      string `json:"code"`
	Retryable bool   `json:"retryable"`
}

// onboardingProblemFromError reads the Problem Details body from an error of the
// provider's client. DCIRetryClient.Do returns every status it does not pass
// through as "non-retryable error: <status>, body: <body>".
func onboardingProblemFromError(err error) (onboardingProblem, bool) {
	if err == nil {
		return onboardingProblem{}, false
	}
	_, body, found := strings.Cut(err.Error(), ", body: ")
	if !found {
		return onboardingProblem{}, false
	}
	var problem onboardingProblem
	if json.Unmarshal([]byte(body), &problem) != nil || problem.Code == "" {
		return onboardingProblem{}, false
	}
	return problem, true
}

// retryOnboardingRequest calls call until it returns an error that is not a
// retryable onboarding problem with one of codes, or until ctx is done. It
// returns the error of the last call. call must send a new Idempotency-Key on
// every attempt, so that a cached response is never replayed.
func retryOnboardingRequest(ctx context.Context, codes []string, call func() error) error {
	for {
		err := call()
		problem, ok := onboardingProblemFromError(err)
		if !ok || !problem.Retryable || !slices.Contains(codes, problem.Code) {
			return err
		}

		tflog.Debug(ctx, "Onboarding request refused with a retryable error, retrying", map[string]any{
			"code": problem.Code,
			"wait": onboardingRetryInterval.String(),
		})
		timer := time.NewTimer(onboardingRetryInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return err
		case <-timer.C:
		}
	}
}
