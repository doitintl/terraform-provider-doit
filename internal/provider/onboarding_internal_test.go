package provider

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	modelsv2 "github.com/doitintl/terraform-provider-doit/internal/provider/models/v2"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// newTestV2Client builds the v2 client the way the provider does: from the
// provider's retrying client, so non-2xx statuses become errors as in production.
func newTestV2Client(t *testing.T, server *httptest.Server, customerContext string) *modelsv2.ClientWithResponses {
	t.Helper()
	// NewTestServer starts on the first Client() call and sets server.URL then.
	httpClient := server.Client()
	client, err := newClientWithHTTPClient(server.URL, "test-token", customerContext, "1.9.0", "dev", httpClient)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	v2Client, err := newV2Client(client)
	if err != nil {
		t.Fatalf("failed to create v2 client: %v", err)
	}
	return v2Client
}

// onboardingTestRaw builds a plan or state value for a resource schema the way
// the framework plans a create: Computed attributes are unknown, every other
// attribute is null, and overrides replace both.
func onboardingTestRaw(t *testing.T, s schema.Schema, overrides map[string]tftypes.Value) tftypes.Value {
	t.Helper()
	ctx := t.Context()
	attrTypes := map[string]tftypes.Type{}
	values := map[string]tftypes.Value{}
	for name, a := range s.Attributes {
		tfType := a.GetType().TerraformType(ctx)
		attrTypes[name] = tfType
		if override, ok := overrides[name]; ok {
			values[name] = override
			continue
		}
		if a.IsComputed() {
			values[name] = tftypes.NewValue(tfType, tftypes.UnknownValue)
			continue
		}
		values[name] = tftypes.NewValue(tfType, nil)
	}
	for name := range overrides {
		if _, ok := s.Attributes[name]; !ok {
			t.Fatalf("override %q is not an attribute of the schema", name)
		}
	}
	return tftypes.NewValue(tftypes.Object{AttributeTypes: attrTypes}, values)
}

// writeProblem writes an RFC 9457 Problem Details body.
func writeProblem(w http.ResponseWriter, status int, code string, retryable bool) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, `{"type":"https://developer.doit.com/errors/%[2]s","title":"t","status":%[1]d,"detail":"d","instance":"https://api.doit.com/requests/req_1","code":%[2]q,"retryable":%[3]t}`, status, code, retryable)
}

func TestOnboardingProblemFromError(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		want   onboardingProblem
		wantOK bool
	}{
		{name: "nil error", err: nil},
		{name: "transport error without body", err: errors.New("dial tcp: connection refused")},
		{
			name:   "retryable problem",
			err:    errors.New(`non-retryable error: 422, body: {"code":"onboarding.billing_export_not_ready","retryable":true,"status":422}`),
			want:   onboardingProblem{Code: "onboarding.billing_export_not_ready", Retryable: true},
			wantOK: true,
		},
		{
			name:   "non-retryable problem",
			err:    errors.New(`non-retryable error: 409, body: {"code":"onboarding.connection_exists","retryable":false}`),
			want:   onboardingProblem{Code: "onboarding.connection_exists"},
			wantOK: true,
		},
		{name: "body that is not JSON", err: errors.New("non-retryable error: 502, body: <html>bad gateway</html>")},
		{name: "JSON body without a code", err: errors.New(`non-retryable error: 400, body: {"error":"bad request"}`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := onboardingProblemFromError(tt.err)
			if ok != tt.wantOK || got != tt.want {
				t.Errorf("onboardingProblemFromError() = %+v, %v; want %+v, %v", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

// TestNewV2Client_SharesProviderClientSettings checks that the v2 client sends
// requests to the provider's host with its token, User-Agent and tenant header,
// and returns error statuses through the provider's retry client.
func TestNewV2Client_SharesProviderClientSettings(t *testing.T) {
	var gotAuth, gotTenant, gotUserAgent, gotPath string
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotTenant = r.Header.Get("X-Tenant-Id")
		gotUserAgent = r.Header.Get("User-Agent")
		gotPath = r.URL.Path
		writeProblem(w, http.StatusForbidden, "forbidden", false)
	}))

	client := newTestV2Client(t, server, "tenant-1")
	_, err := client.GetGcpBillingAccountWithResponse(t.Context(), "012345-6789AB-CDEF01", &modelsv2.GetGcpBillingAccountParams{})

	if err == nil {
		t.Fatal("expected the 403 to be returned as an error by the retry client")
	}
	if problem, ok := onboardingProblemFromError(err); !ok || problem.Code != "forbidden" {
		t.Errorf("problem = %+v, %v; want code forbidden", problem, ok)
	}
	if gotPath != "/onboarding/v2/billing/gcp/accounts/012345-6789AB-CDEF01" {
		t.Errorf("path = %q", gotPath)
	}
	if gotAuth != "Bearer test-token" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer test-token")
	}
	if gotTenant != "tenant-1" {
		t.Errorf("X-Tenant-Id = %q, want %q", gotTenant, "tenant-1")
	}
	if !strings.HasPrefix(gotUserAgent, "Terraform/1.9.0 terraform-provider-doit/dev") {
		t.Errorf("User-Agent = %q", gotUserAgent)
	}
}
