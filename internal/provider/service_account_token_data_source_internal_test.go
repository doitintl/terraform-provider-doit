package provider

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestServiceAccountTokenDataSourceUnknownInputs(t *testing.T) {
	unknown := tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	known := tftypes.NewValue(tftypes.String, "known")
	for _, tc := range []struct {
		name             string
		serviceAccountID tftypes.Value
		id               tftypes.Value
	}{
		{"unknown id", known, unknown},
		{"unknown service_account_id", unknown, known},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewTestServer(t, http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { calls.Add(1) }))
			resp := readServiceAccountTokenDataSourceForTest(t, server, &tc.serviceAccountID, &tc.id)
			if resp.Diagnostics.HasError() {
				t.Fatal(resp.Diagnostics)
			}
			if calls.Load() != 0 {
				t.Fatalf("made %d API requests for unknown input", calls.Load())
			}
			var data serviceAccountTokenDataSourceModel
			if diags := resp.State.Get(t.Context(), &data); diags.HasError() {
				t.Fatal(diags)
			}
			if !data.Name.IsUnknown() || !data.State.IsUnknown() || !data.ExpiresTime.IsUnknown() || !data.LastUsedTime.IsUnknown() {
				t.Fatalf("outputs must remain unknown: %+v", data)
			}
		})
	}
}

func TestServiceAccountTokenDataSourceReadErrors(t *testing.T) {
	for _, tc := range []struct {
		name        string
		status      int
		contentType string
		body        string
		want        string
	}{
		{"missing", 404, "application/json", `{"error":"missing"}`, "status 404"},
		{"non-JSON", 200, "text/plain", "wrong content type", "status 200"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			serviceAccountID := tftypes.NewValue(tftypes.String, "sa-1")
			id := tftypes.NewValue(tftypes.String, "tok-1")
			resp := readServiceAccountTokenDataSourceForTest(t, server, &serviceAccountID, &id)
			if !resp.Diagnostics.HasError() || !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), tc.want) {
				t.Fatalf("diagnostics = %v, want %q", resp.Diagnostics, tc.want)
			}
		})
	}
}

func serviceAccountTokenDataSourceTestServer(t *testing.T, body string, gotPath *atomic.Value) *httptest.Server {
	t.Helper()
	return httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath.Store(r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
}

func TestServiceAccountTokenDataSourceRead(t *testing.T) {
	var gotPath atomic.Value
	server := serviceAccountTokenDataSourceTestServer(t, `{
		"id": "tok-1", "serviceAccountId": "sa-1", "customerId": "cust-1",
		"name": "ci", "state": "disabled",
		"createTime": "2026-09-01T08:00:00Z", "expiresTime": null, "lastUsedTime": null
	}`, &gotPath)
	serviceAccountID := tftypes.NewValue(tftypes.String, "sa-1")
	id := tftypes.NewValue(tftypes.String, "tok-1")
	resp := readServiceAccountTokenDataSourceForTest(t, server, &serviceAccountID, &id)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if want := "/iam/v1/service-accounts/sa-1/api-tokens/tok-1"; gotPath.Load() != want {
		t.Fatalf("path = %v, want %s", gotPath.Load(), want)
	}
	var data serviceAccountTokenDataSourceModel
	if diags := resp.State.Get(t.Context(), &data); diags.HasError() {
		t.Fatal(diags)
	}
	if data.Id.ValueString() != "tok-1" || data.ServiceAccountId.ValueString() != "sa-1" ||
		data.CustomerId.ValueString() != "cust-1" || data.Name.ValueString() != "ci" ||
		data.State.ValueString() != "disabled" || data.CreateTime.ValueString() != "2026-09-01T08:00:00Z" {
		t.Fatalf("mapped state = %+v", data)
	}
	if !data.ExpiresTime.IsNull() || !data.LastUsedTime.IsNull() {
		t.Fatalf("expires_time and last_used_time must be null: %+v", data)
	}
}

func TestServiceAccountTokenDataSourceKeepsRequestedServiceAccountID(t *testing.T) {
	var gotPath atomic.Value
	server := serviceAccountTokenDataSourceTestServer(t, `{
		"id": "tok-1", "customerId": "cust-1", "name": "ci", "state": "active",
		"createTime": "2026-09-01T08:00:00Z", "expiresTime": null, "lastUsedTime": null
	}`, &gotPath)
	serviceAccountID := tftypes.NewValue(tftypes.String, "sa-1")
	id := tftypes.NewValue(tftypes.String, "tok-1")
	resp := readServiceAccountTokenDataSourceForTest(t, server, &serviceAccountID, &id)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	var data serviceAccountTokenDataSourceModel
	if diags := resp.State.Get(t.Context(), &data); diags.HasError() {
		t.Fatal(diags)
	}
	if data.ServiceAccountId.ValueString() != "sa-1" {
		t.Fatalf("service_account_id = %q, want the requested sa-1", data.ServiceAccountId.ValueString())
	}
}

func TestServiceAccountTokenDataSourceIDMismatch(t *testing.T) {
	var gotPath atomic.Value
	server := serviceAccountTokenDataSourceTestServer(t, `{
		"id": "other", "serviceAccountId": "sa-1", "customerId": "cust-1", "name": "ci", "state": "active",
		"createTime": "2026-09-01T08:00:00Z", "expiresTime": null, "lastUsedTime": null
	}`, &gotPath)
	serviceAccountID := tftypes.NewValue(tftypes.String, "sa-1")
	id := tftypes.NewValue(tftypes.String, "tok-1")
	resp := readServiceAccountTokenDataSourceForTest(t, server, &serviceAccountID, &id)
	if !resp.Diagnostics.HasError() || !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "other") {
		t.Fatalf("diagnostics = %v, want ID mismatch error", resp.Diagnostics)
	}
}
