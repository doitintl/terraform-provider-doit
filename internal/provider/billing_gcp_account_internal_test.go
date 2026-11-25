package provider

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

const (
	testBillingAccountID   = "012345-6789AB-CDEF01"
	testBillingAccountJSON = `{"id":"012345-6789AB-CDEF01","billingAccountId":"012345-6789AB-CDEF01",` +
		`"exportProjectId":"example-project","exportDatasetId":"billing_export","status":"pending"}`
)

// billingTestServer answers each request with the next response of its method
// and records the Idempotency-Key of every request.
type billingTestServer struct {
	mu        sync.Mutex
	responses map[string][]func(http.ResponseWriter)
	keys      map[string][]string
}

func newBillingTestServer(t *testing.T, responses map[string][]func(http.ResponseWriter)) (*httptest.Server, *billingTestServer) {
	t.Helper()
	recorder := &billingTestServer{responses: responses, keys: map[string][]string{}}
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder.mu.Lock()
		recorder.keys[r.Method] = append(recorder.keys[r.Method], r.Header.Get("Idempotency-Key"))
		queue := recorder.responses[r.Method]
		var respond func(http.ResponseWriter)
		if len(queue) > 0 {
			respond = queue[0]
			if len(queue) > 1 {
				recorder.responses[r.Method] = queue[1:]
			}
		}
		recorder.mu.Unlock()
		if respond == nil {
			w.WriteHeader(http.StatusNotImplemented)
			return
		}
		respond(w)
	}))
	return server, recorder
}

func (s *billingTestServer) requestKeys(method string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.keys[method]...)
}

func jsonResponse(status int, body string) func(http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func problemResponse(status int, code string, retryable bool) func(http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		writeProblem(w, status, code, retryable)
	}
}

// billingTestPlan plans a create with a 100s create timeout. The timeout is not
// a multiple of onboardingRetryInterval, so the deadline and a retry timer never
// come due at the same virtual instant.
func billingTestPlan(t *testing.T, r *billingGcpAccountResource) tfsdk.Plan {
	t.Helper()
	ctx := t.Context()
	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)
	s := schemaResp.Schema

	timeoutsType := s.Attributes["timeouts"].GetType().TerraformType(ctx)
	timeoutsObject, ok := timeoutsType.(tftypes.Object)
	if !ok {
		t.Fatalf("timeouts type = %T, want tftypes.Object", timeoutsType)
	}
	timeoutValues := map[string]tftypes.Value{}
	for name, attrType := range timeoutsObject.AttributeTypes {
		timeoutValues[name] = tftypes.NewValue(attrType, nil)
	}
	timeoutValues["create"] = tftypes.NewValue(tftypes.String, "100s")

	return tfsdk.Plan{Schema: s, Raw: onboardingTestRaw(t, s, map[string]tftypes.Value{
		"billing_account_id": tftypes.NewValue(tftypes.String, testBillingAccountID),
		"export_project_id":  tftypes.NewValue(tftypes.String, "example-project"),
		"export_dataset_id":  tftypes.NewValue(tftypes.String, "billing_export"),
		"timeouts":           tftypes.NewValue(timeoutsType, timeoutValues),
	})}
}

func createBillingTestAccount(t *testing.T, server *httptest.Server) (billingGcpAccountResourceModel, diag.Diagnostics) {
	t.Helper()
	r := &billingGcpAccountResource{client: newTestV2Client(t, server, "")}
	plan := billingTestPlan(t, r)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: plan.Schema, Raw: tftypes.NewValue(plan.Raw.Type(), nil)}}
	r.Create(t.Context(), resource.CreateRequest{Plan: plan}, resp)

	var state billingGcpAccountResourceModel
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Get(t.Context(), &state)...)
	}
	return state, resp.Diagnostics
}

func deleteBillingTestAccount(t *testing.T, server *httptest.Server) diag.Diagnostics {
	t.Helper()
	r := &billingGcpAccountResource{client: newTestV2Client(t, server, "")}
	schemaResp := &resource.SchemaResponse{}
	r.Schema(t.Context(), resource.SchemaRequest{}, schemaResp)
	state := tfsdk.State{Schema: schemaResp.Schema, Raw: onboardingTestRaw(t, schemaResp.Schema, map[string]tftypes.Value{
		"id":                 tftypes.NewValue(tftypes.String, testBillingAccountID),
		"billing_account_id": tftypes.NewValue(tftypes.String, testBillingAccountID),
		"export_project_id":  tftypes.NewValue(tftypes.String, "example-project"),
		"export_dataset_id":  tftypes.NewValue(tftypes.String, "billing_export"),
		"status":             tftypes.NewValue(tftypes.String, "active"),
		"failure_reason":     tftypes.NewValue(tftypes.String, nil),
	})}
	resp := &resource.DeleteResponse{State: state}
	r.Delete(t.Context(), resource.DeleteRequest{State: state}, resp)
	return resp.Diagnostics
}

func assertDistinctKeys(t *testing.T, keys []string, want int) {
	t.Helper()
	if len(keys) != want {
		t.Fatalf("requests = %d, want %d", len(keys), want)
	}
	seen := map[string]bool{}
	for _, key := range keys {
		if key == "" || seen[key] {
			t.Errorf("Idempotency-Keys = %v, want %d distinct non-empty keys", keys, want)
			return
		}
		seen[key] = true
	}
}

func TestBillingGcpAccountCreate_RetriesAccessNotPropagated(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		server, recorder := newBillingTestServer(t, map[string][]func(http.ResponseWriter){
			http.MethodPost: {
				problemResponse(http.StatusUnprocessableEntity, billingAccountAccessMissingCode, true),
				problemResponse(http.StatusUnprocessableEntity, billingExportNotReadyCode, true),
				jsonResponse(http.StatusCreated, testBillingAccountJSON),
			},
		})

		start := time.Now()
		state, diags := createBillingTestAccount(t, server)
		elapsed := time.Since(start)

		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		if got := state.Id.ValueString(); got != testBillingAccountID {
			t.Errorf("id = %q, want %q", got, testBillingAccountID)
		}
		if got := state.Status.ValueString(); got != "pending" {
			t.Errorf("status = %q, want pending", got)
		}
		if !state.FailureReason.IsNull() {
			t.Errorf("failure_reason = %v, want null", state.FailureReason)
		}
		assertDistinctKeys(t, recorder.requestKeys(http.MethodPost), 3)
		if want := 2 * onboardingRetryInterval; elapsed != want {
			t.Errorf("elapsed = %v, want %v", elapsed, want)
		}
	})
}

func TestBillingGcpAccountCreate_DoesNotRetryOtherErrors(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		code      string
		retryable bool
	}{
		// Retryable, but not one of the access errors: a pending connection with another export.
		{name: "activating with another export", status: http.StatusConflict, code: billingConnectionActivatingCode, retryable: true},
		{name: "access error marked not retryable", status: http.StatusUnprocessableEntity, code: billingAccountAccessMissingCode, retryable: false},
		{name: "export mismatch", status: http.StatusConflict, code: "onboarding.billing_connection_export_mismatch", retryable: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				server, recorder := newBillingTestServer(t, map[string][]func(http.ResponseWriter){
					http.MethodPost: {problemResponse(tt.status, tt.code, tt.retryable)},
				})

				start := time.Now()
				_, diags := createBillingTestAccount(t, server)
				elapsed := time.Since(start)

				if !diags.HasError() {
					t.Fatal("expected an error")
				}
				if detail := diags.Errors()[0].Detail(); !strings.Contains(detail, tt.code) {
					t.Errorf("detail %q does not contain %q", detail, tt.code)
				}
				if got := len(recorder.requestKeys(http.MethodPost)); got != 1 {
					t.Errorf("requests = %d, want 1", got)
				}
				if elapsed != 0 {
					t.Errorf("elapsed = %v, want 0", elapsed)
				}
			})
		})
	}
}

func TestBillingGcpAccountCreate_StopsAtCreateTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		server, recorder := newBillingTestServer(t, map[string][]func(http.ResponseWriter){
			http.MethodPost: {problemResponse(http.StatusUnprocessableEntity, billingExportNotReadyCode, true)},
		})

		start := time.Now()
		_, diags := createBillingTestAccount(t, server)
		elapsed := time.Since(start)

		if !diags.HasError() {
			t.Fatal("expected an error")
		}
		if detail := diags.Errors()[0].Detail(); !strings.Contains(detail, billingExportNotReadyCode) {
			t.Errorf("detail %q does not contain the last API error", detail)
		}
		// Attempts at 0s, 15s, ..., 90s; the 100s create timeout ends the wait for the next one.
		if got := len(recorder.requestKeys(http.MethodPost)); got != 7 {
			t.Errorf("requests = %d, want 7", got)
		}
		if want := 100 * time.Second; elapsed != want {
			t.Errorf("elapsed = %v, want %v", elapsed, want)
		}
	})
}

func TestBillingGcpAccountDelete(t *testing.T) {
	tests := []struct {
		name         string
		responses    []func(http.ResponseWriter)
		wantRequests int
		wantElapsed  time.Duration
		wantError    string
	}{
		{
			name:         "disconnected",
			responses:    []func(http.ResponseWriter){jsonResponse(http.StatusNoContent, "")},
			wantRequests: 1,
		},
		{
			name:         "already gone",
			responses:    []func(http.ResponseWriter){problemResponse(http.StatusNotFound, "not_found", false)},
			wantRequests: 1,
		},
		{
			name: "busy is retried",
			responses: []func(http.ResponseWriter){
				problemResponse(http.StatusConflict, billingAccountBusyCode, true),
				problemResponse(http.StatusConflict, billingAccountBusyCode, true),
				jsonResponse(http.StatusNoContent, ""),
			},
			wantRequests: 3,
			wantElapsed:  2 * onboardingRetryInterval,
		},
		{
			name:         "activating is reported with a hint",
			responses:    []func(http.ResponseWriter){problemResponse(http.StatusConflict, billingConnectionActivatingCode, true)},
			wantRequests: 1,
			wantError:    "Retry when the connection is active or failed.",
		},
		{
			name:         "other errors are reported without the hint",
			responses:    []func(http.ResponseWriter){problemResponse(http.StatusForbidden, "forbidden", false)},
			wantRequests: 1,
			wantError:    "forbidden",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				server, recorder := newBillingTestServer(t, map[string][]func(http.ResponseWriter){
					http.MethodDelete: tt.responses,
				})

				start := time.Now()
				diags := deleteBillingTestAccount(t, server)
				elapsed := time.Since(start)

				switch {
				case tt.wantError == "" && diags.HasError():
					t.Fatalf("unexpected diagnostics: %v", diags)
				case tt.wantError != "" && !diags.HasError():
					t.Fatal("expected an error")
				case tt.wantError != "":
					detail := diags.Errors()[0].Detail()
					if !strings.Contains(detail, tt.wantError) {
						t.Errorf("detail %q does not contain %q", detail, tt.wantError)
					}
					if tt.wantError == "forbidden" && strings.Contains(detail, "Retry when") {
						t.Errorf("detail %q has the activating hint for another error", detail)
					}
				}
				assertDistinctKeys(t, recorder.requestKeys(http.MethodDelete), tt.wantRequests)
				if elapsed != tt.wantElapsed {
					t.Errorf("elapsed = %v, want %v", elapsed, tt.wantElapsed)
				}
			})
		})
	}
}

func TestMapBillingGcpAccountToModel_FailedWithoutExport(t *testing.T) {
	resp := `{"id":"012345-6789AB-CDEF01","billingAccountId":"012345-6789AB-CDEF01","status":"failed","failureReason":"connect again"}`
	server, _ := newBillingTestServer(t, map[string][]func(http.ResponseWriter){
		http.MethodGet: {jsonResponse(http.StatusOK, resp)},
	})
	r := &billingGcpAccountResource{client: newTestV2Client(t, server, "")}

	var state billingGcpAccountResourceModel
	state.Id = types.StringValue(testBillingAccountID)
	diags := r.populateState(t.Context(), &state)

	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if !state.ExportProjectId.IsNull() || !state.ExportDatasetId.IsNull() {
		t.Errorf("export = %v / %v, want null / null", state.ExportProjectId, state.ExportDatasetId)
	}
	if state.Status.ValueString() != "failed" || state.FailureReason.ValueString() != "connect again" {
		t.Errorf("status = %v, failure_reason = %v", state.Status, state.FailureReason)
	}
}
