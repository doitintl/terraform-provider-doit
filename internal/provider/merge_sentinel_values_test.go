package provider

import "testing"

// TestMergeSentinelValues verifies the sentinel-restoration merge logic, including
// the ordering-bug regression: a sentinel appearing BEFORE a non-sentinel in
// stateVals must not cause the non-sentinel to be silently discarded.
func TestMergeSentinelValues(t *testing.T) {
	tests := []struct {
		name           string
		apiValues      []string
		stateVals      []string
		apiIncludeNull bool
		want           []string
	}{
		// ------------------------------------------------------------------ //
		// No state to merge from — result equals API values.
		// ------------------------------------------------------------------ //
		{
			name:           "no state, API has values",
			apiValues:      []string{"Cloud Run", "BigQuery"},
			stateVals:      nil,
			apiIncludeNull: false,
			want:           []string{"Cloud Run", "BigQuery"},
		},
		{
			name:           "no state, API empty",
			apiValues:      nil,
			stateVals:      nil,
			apiIncludeNull: false,
			want:           nil,
		},
		// ------------------------------------------------------------------ //
		// Normal case: sentinel was stripped by API, restored from state.
		// ------------------------------------------------------------------ //
		{
			name:           "sentinel stripped by API, restored",
			apiValues:      []string{"Cloud Run"},
			stateVals:      []string{"Cloud Run", "[Service N/A]"},
			apiIncludeNull: true,
			want:           []string{"Cloud Run", "[Service N/A]"},
		},
		{
			name:           "sentinel-only filter stripped to empty, restored",
			apiValues:      []string{},
			stateVals:      []string{"[Service N/A]"},
			apiIncludeNull: true,
			want:           []string{"[Service N/A]"},
		},
		// ------------------------------------------------------------------ //
		// Sentinel NOT restored when apiIncludeNull=false.
		// ------------------------------------------------------------------ //
		{
			name:           "sentinel not restored when apiIncludeNull=false",
			apiValues:      []string{"Cloud Run"},
			stateVals:      []string{"Cloud Run", "[Service N/A]"},
			apiIncludeNull: false,
			want:           []string{"Cloud Run"},
		},
		// ------------------------------------------------------------------ //
		// Legacy full-state fallback: API returned nothing AND a non-sentinel
		// value is missing → restore full state list.
		// ------------------------------------------------------------------ //
		{
			name:           "API returned nothing, non-sentinel lost — full fallback",
			apiValues:      nil,
			stateVals:      []string{"Cloud Run", "BigQuery"},
			apiIncludeNull: false,
			want:           []string{"Cloud Run", "BigQuery"},
		},
		{
			name:           "API returned nothing, only sentinels in state — restored",
			apiValues:      nil,
			stateVals:      []string{"[Service N/A]"},
			apiIncludeNull: true,
			want:           []string{"[Service N/A]"},
		},
		// ------------------------------------------------------------------ //
		// REGRESSION: sentinel before non-sentinel must NOT drop the non-sentinel.
		//
		// Old bug: the loop appended the sentinel first (mergedValues=[sentinel]),
		// then when it saw the non-sentinel it checked len(mergedValues)==0
		// which was now false, so the non-sentinel was silently discarded.
		// ------------------------------------------------------------------ //
		{
			name:           "regression: sentinel before non-sentinel, API empty — full fallback",
			apiValues:      nil,
			stateVals:      []string{"[Service N/A]", "Cloud Run"},
			apiIncludeNull: true,
			// Non-sentinel "Cloud Run" is missing and api returned nothing →
			// full-state fallback preserves both elements.
			want: []string{"[Service N/A]", "Cloud Run"},
		},
		{
			name:           "regression: non-sentinel before sentinel, API empty — full fallback",
			apiValues:      nil,
			stateVals:      []string{"Cloud Run", "[Service N/A]"},
			apiIncludeNull: true,
			want:           []string{"Cloud Run", "[Service N/A]"},
		},
		{
			name:           "regression: sentinel before non-sentinel, API has other value",
			apiValues:      []string{"BigQuery"},
			stateVals:      []string{"[Service N/A]", "Cloud Run"},
			apiIncludeNull: true,
			// API returned BigQuery. Cloud Run disappeared (non-sentinel, lost).
			// len(apiValues) > 0 so full fallback does NOT trigger.
			// Sentinel [Service N/A] is restored at its original state position (index 0);
			// Cloud Run is gone; BigQuery (new from API) is appended at the end.
			want: []string{"[Service N/A]", "BigQuery"},
		},
		// ------------------------------------------------------------------ //
		// Values already in API response are not duplicated.
		// ------------------------------------------------------------------ //
		{
			name:           "stateVals already in API are not duplicated",
			apiValues:      []string{"Cloud Run", "[Service N/A]"},
			stateVals:      []string{"Cloud Run", "[Service N/A]"},
			apiIncludeNull: true,
			want:           []string{"Cloud Run", "[Service N/A]"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeSentinelValues(tt.apiValues, tt.stateVals, tt.apiIncludeNull)

			// Treat nil and empty as equivalent for comparison.
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("length mismatch:\n  got  %v (%d)\n  want %v (%d)", got, len(got), tt.want, len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("index %d: got %q, want %q\n  full got:  %v\n  full want: %v",
						i, got[i], tt.want[i], got, tt.want)
				}
			}
		})
	}
}

// mergeSentinelValuesBuggy is the verbatim pre-fix implementation, preserved
// here to prove that the regression test cases expose the bug.
//
// The bug: a sentinel appearing before a non-sentinel in stateVals causes
// len(mergedValues) to be >0 by the time the non-sentinel is reached, so the
// "len(mergedValues)==0" full-state-fallback branch never fires and the
// non-sentinel is silently discarded.
func mergeSentinelValuesBuggy(apiValues []string, stateVals []string, apiIncludeNull bool) []string {
	apiValueSet := make(map[string]bool)
	var mergedValues []string
	for _, v := range apiValues {
		apiValueSet[v] = true
		mergedValues = append(mergedValues, v)
	}
	for _, sv := range stateVals {
		if !apiValueSet[sv] {
			if apiIncludeNull && isNAFallback(sv) {
				// Sentinel was stripped by API normalization — restore it.
				mergedValues = append(mergedValues, sv)
			} else if len(mergedValues) == 0 {
				// API returned nothing at all — fall back to full state list.
				mergedValues = stateVals
				break
			}
		}
	}
	return mergedValues
}

// TestMergeSentinelValuesBuggyRegression confirms that the old implementation
// produced incorrect results on the ordering-sensitive cases, and that the fixed
// implementation produces the correct results.
//
// If the "buggy impl" assertions in this test ever start FAILING (i.e. the buggy
// impl now produces the right result), it means the regression cases no longer
// catch the bug and should be revisited.
func TestMergeSentinelValuesBuggyRegression(t *testing.T) {
	type bugCase struct {
		name           string
		apiValues      []string
		stateVals      []string
		apiIncludeNull bool
		// correctWant is what the fixed implementation returns.
		correctWant []string
		// buggyWant is what the old implementation returns (wrong).
		buggyWant []string
	}
	cases := []bugCase{
		{
			// Bug scenario: sentinel is first in stateVals.
			// Old loop: appends "[Service N/A]" → mergedValues=["[Service N/A]"],
			// then sees "Cloud Run" (non-sentinel, not in API), checks
			// len(mergedValues)==0 → FALSE → "Cloud Run" is silently dropped.
			name:           "sentinel before non-sentinel, API empty",
			apiValues:      nil,
			stateVals:      []string{"[Service N/A]", "Cloud Run"},
			apiIncludeNull: true,
			correctWant:    []string{"[Service N/A]", "Cloud Run"}, // full fallback
			buggyWant:      []string{"[Service N/A]"},              // "Cloud Run" dropped!
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// 1. Confirm the BUGGY implementation produces the expected wrong result.
			buggyGot := mergeSentinelValuesBuggy(tc.apiValues, tc.stateVals, tc.apiIncludeNull)
			if len(buggyGot) != len(tc.buggyWant) {
				t.Errorf("buggy impl: expected len=%d got len=%d (%v); the bug may have changed",
					len(tc.buggyWant), len(buggyGot), buggyGot)
			} else {
				for i := range buggyGot {
					if buggyGot[i] != tc.buggyWant[i] {
						t.Errorf("buggy impl index %d: got %q want %q", i, buggyGot[i], tc.buggyWant[i])
					}
				}
				t.Logf("buggy impl produced wrong result (bug confirmed): %v", buggyGot)
			}

			// 2. Confirm the FIXED implementation produces the correct result.
			fixedGot := mergeSentinelValues(tc.apiValues, tc.stateVals, tc.apiIncludeNull)
			if len(fixedGot) != len(tc.correctWant) {
				t.Errorf("fixed impl: expected len=%d got len=%d (%v)", len(tc.correctWant), len(fixedGot), fixedGot)
			} else {
				for i := range fixedGot {
					if fixedGot[i] != tc.correctWant[i] {
						t.Errorf("fixed impl index %d: got %q want %q", i, fixedGot[i], tc.correctWant[i])
					}
				}
				t.Logf("fixed impl produced correct result: %v", fixedGot)
			}
		})
	}
}
