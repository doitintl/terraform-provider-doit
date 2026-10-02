package provider

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_service_account_tokens"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestMapServiceAccountTokensToModel(t *testing.T) {
	created := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	expires := time.Date(2027, 9, 1, 8, 0, 0, 0, time.UTC)
	items := []models.ServiceAccountToken{{
		Id: valueToNullable("tok-1"), CustomerId: new("cust-1"),
		Name: "ci", State: models.ServiceAccountTokenStateActive,
		CreateTime: valueToNullable(created), ExpiresTime: valueToNullable(expires),
	}}
	var data serviceAccountTokensDataSourceModel
	data.ServiceAccountId = types.StringValue("sa-1")
	if diags := mapServiceAccountTokensToModel(t.Context(), items, &data); diags.HasError() {
		t.Fatal(diags)
	}
	if data.Items.IsNull() || len(data.Items.Elements()) != 1 {
		t.Fatalf("items = %v, want one item", data.Items)
	}
	value, ok := data.Items.Elements()[0].(datasource_service_account_tokens.ItemsValue)
	if !ok {
		t.Fatalf("item has type %T", data.Items.Elements()[0])
	}
	if value.Id.ValueString() != "tok-1" || value.ServiceAccountId.ValueString() != "sa-1" ||
		value.CustomerId.ValueString() != "cust-1" || value.State.ValueString() != "active" ||
		value.CreateTime.ValueString() != "2026-09-01T08:00:00Z" || value.ExpiresTime.ValueString() != "2027-09-01T08:00:00Z" {
		t.Fatalf("mapped item = %v", value)
	}
	if !value.LastUsedTime.IsNull() {
		t.Fatalf("last_used_time must be null: %v", value)
	}
}

func TestMapServiceAccountTokensToModel_EmptyAndMissingID(t *testing.T) {
	var data serviceAccountTokensDataSourceModel
	data.ServiceAccountId = types.StringValue("sa-1")
	if diags := mapServiceAccountTokensToModel(t.Context(), nil, &data); diags.HasError() {
		t.Fatal(diags)
	}
	if data.Items.IsNull() || len(data.Items.Elements()) != 0 {
		t.Fatalf("items = %v, want empty non-null list", data.Items)
	}
	if diags := mapServiceAccountTokensToModel(t.Context(), []models.ServiceAccountToken{{Name: "missing-id"}}, &data); !diags.HasError() {
		t.Fatal("expected diagnostic for missing ID")
	}
}

func TestServiceAccountTokensDataSourceUnknownServiceAccountID(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewTestServer(t, http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { calls.Add(1) }))
	serviceAccountID := tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	resp := readServiceAccountTokenDataSourceForTest(t, server, &serviceAccountID, nil)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if calls.Load() != 0 {
		t.Fatalf("made %d API requests for unknown service_account_id", calls.Load())
	}
	var data serviceAccountTokensDataSourceModel
	if diags := resp.State.Get(t.Context(), &data); diags.HasError() {
		t.Fatal(diags)
	}
	if !data.Items.IsUnknown() {
		t.Fatalf("items must remain unknown: %v", data.Items)
	}
}

func TestServiceAccountTokensDataSourceReadErrors(t *testing.T) {
	for _, tc := range []struct {
		name        string
		status      int
		contentType string
		body        string
		want        string
	}{
		{"forbidden", 403, "application/json", `{"error":"denied"}`, "status 403"},
		{"non-JSON", 200, "text/plain", "wrong content type", "status 200"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			serviceAccountID := tftypes.NewValue(tftypes.String, "sa-1")
			resp := readServiceAccountTokenDataSourceForTest(t, server, &serviceAccountID, nil)
			if !resp.Diagnostics.HasError() || !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), tc.want) {
				t.Fatalf("diagnostics = %v, want %q", resp.Diagnostics, tc.want)
			}
		})
	}
}

func TestServiceAccountTokensDataSourceRead(t *testing.T) {
	var gotPath atomic.Value
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath.Store(r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items": [
			{"id": "tok-1", "serviceAccountId": "sa-1", "customerId": "cust-1", "name": "a", "state": "active",
			 "createTime": "2026-09-01T08:00:00Z", "expiresTime": "2027-09-01T08:00:00Z", "lastUsedTime": null},
			{"id": "tok-2", "serviceAccountId": "sa-1", "customerId": "cust-1", "name": "b", "state": "disabled",
			 "createTime": "2026-09-02T08:00:00Z", "expiresTime": null, "lastUsedTime": "2026-09-20T14:31:00Z"}
		]}`))
	}))
	serviceAccountID := tftypes.NewValue(tftypes.String, "sa-1")
	resp := readServiceAccountTokenDataSourceForTest(t, server, &serviceAccountID, nil)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if want := "/iam/v1/service-accounts/sa-1/api-tokens"; gotPath.Load() != want {
		t.Fatalf("path = %v, want %s", gotPath.Load(), want)
	}
	var data serviceAccountTokensDataSourceModel
	if diags := resp.State.Get(t.Context(), &data); diags.HasError() {
		t.Fatal(diags)
	}
	if data.ServiceAccountId.ValueString() != "sa-1" || len(data.Items.Elements()) != 2 {
		t.Fatalf("state = %+v", data)
	}
	second, ok := data.Items.Elements()[1].(datasource_service_account_tokens.ItemsValue)
	if !ok {
		t.Fatalf("item has type %T", data.Items.Elements()[1])
	}
	if second.Id.ValueString() != "tok-2" || second.State.ValueString() != "disabled" ||
		!second.ExpiresTime.IsNull() || second.LastUsedTime.ValueString() != "2026-09-20T14:31:00Z" {
		t.Fatalf("second item = %v", second)
	}
}

func TestServiceAccountTokensDataSourceReadEmpty(t *testing.T) {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items": []}`))
	}))
	serviceAccountID := tftypes.NewValue(tftypes.String, "sa-1")
	resp := readServiceAccountTokenDataSourceForTest(t, server, &serviceAccountID, nil)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	var data serviceAccountTokensDataSourceModel
	if diags := resp.State.Get(t.Context(), &data); diags.HasError() {
		t.Fatal(diags)
	}
	if data.Items.IsNull() || data.Items.IsUnknown() || len(data.Items.Elements()) != 0 {
		t.Fatalf("items = %v, want empty non-null list", data.Items)
	}
}
