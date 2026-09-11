package provider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_ps4c_gcp_resource_cuds"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestMapGcpResourceCudsToItemsList_FullyPopulated(t *testing.T) {
	ctx := t.Context()

	createdTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	customTerm := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	exportDate := time.Date(2025, 7, 1, 12, 0, 0, 0, time.UTC)
	startTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	cuds := []models.GcpResourceCud{
		{
			AutoRenew:                  new(true),
			BillingProjectNumber:       new("1234567890"),
			CommitmentNumericId:        "987654321",
			CreatedTime:                valueToNullable(createdTime),
			CustomTermEligibleUntil:    valueToNullable(customTerm),
			EndTime:                    valueToNullable(endTime),
			ExportDate:                 &exportDate,
			HardwareScope:              new("GENERAL_PURPOSE_E2"),
			HostProjectId:              new("test-project-123"),
			InventoryState:             new("ACTIVE"),
			LastMonthCPUCoverage:       new(0.85),
			LastMonthCPUSavings:        new(150.0),
			LastMonthCPUUtilization:    new(0.92),
			LastMonthMemoryCoverage:    new(0.80),
			LastMonthMemorySavings:     new(120.0),
			LastMonthMemoryUtilization: new(0.88),
			Name:                       new("resource-based-cud-1"),
			OrganizationId:             new("604768222286"),
			Plan:                       new("TWELVE_MONTH"),
			Region:                     new("us-central1"),
			ReservedResources: &[]models.GcpResourceCudReservedResourcesItem{
				{
					Amount:       "4",
					ResourceKind: "VCPU",
				},
				{
					Amount:       "16",
					ResourceKind: "MEMORY",
				},
			},
			StartTime: valueToNullable(startTime),
			State:     "ACTIVE",
		},
	}

	list, diags := mapGcpResourceCudsToItemsList(ctx, cuds)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	elements := list.Elements()
	if len(elements) != 1 {
		t.Fatalf("items has %d elements, want 1", len(elements))
	}

	item, ok := elements[0].(datasource_ps4c_gcp_resource_cuds.ItemsValue)
	if !ok {
		t.Fatalf("items[0] has unexpected type %T", elements[0])
	}

	if got := item.CommitmentNumericId.ValueString(); got != "987654321" {
		t.Errorf("commitment_numeric_id = %q, want %q", got, "987654321")
	}
	if got := item.BillingProjectNumber.ValueString(); got != "1234567890" {
		t.Errorf("billing_project_number = %q, want %q", got, "1234567890")
	}
	if got := item.HardwareScope.ValueString(); got != "GENERAL_PURPOSE_E2" {
		t.Errorf("hardware_scope = %q, want %q", got, "GENERAL_PURPOSE_E2")
	}
	if got := item.HostProjectId.ValueString(); got != "test-project-123" {
		t.Errorf("host_project_id = %q, want %q", got, "test-project-123")
	}
	if got := item.InventoryState.ValueString(); got != "ACTIVE" {
		t.Errorf("inventory_state = %q, want %q", got, "ACTIVE")
	}
	if got := item.LastMonthCpucoverage.ValueFloat64(); got != 0.85 {
		t.Errorf("last_month_cpucoverage = %v, want 0.85", got)
	}
	if got := item.LastMonthCpusavings.ValueFloat64(); got != 150.0 {
		t.Errorf("last_month_cpusavings = %v, want 150.0", got)
	}
	if got := item.LastMonthCpuutilization.ValueFloat64(); got != 0.92 {
		t.Errorf("last_month_cpuutilization = %v, want 0.92", got)
	}
	if got := item.LastMonthMemoryCoverage.ValueFloat64(); got != 0.80 {
		t.Errorf("last_month_memory_coverage = %v, want 0.80", got)
	}
	if got := item.LastMonthMemorySavings.ValueFloat64(); got != 120.0 {
		t.Errorf("last_month_memory_savings = %v, want 120.0", got)
	}
	if got := item.LastMonthMemoryUtilization.ValueFloat64(); got != 0.88 {
		t.Errorf("last_month_memory_utilization = %v, want 0.88", got)
	}
	if got := item.Name.ValueString(); got != "resource-based-cud-1" {
		t.Errorf("name = %q, want %q", got, "resource-based-cud-1")
	}
	if got := item.OrganizationId.ValueString(); got != "604768222286" {
		t.Errorf("organization_id = %q, want %q", got, "604768222286")
	}
	if got := item.Plan.ValueString(); got != "TWELVE_MONTH" {
		t.Errorf("plan = %q, want %q", got, "TWELVE_MONTH")
	}
	if got := item.Region.ValueString(); got != "us-central1" {
		t.Errorf("region = %q, want %q", got, "us-central1")
	}
	if got := item.State.ValueString(); got != "ACTIVE" {
		t.Errorf("state = %q, want %q", got, "ACTIVE")
	}

	resList := item.ReservedResources.Elements()
	if len(resList) != 2 {
		t.Fatalf("reserved_resources has %d elements, want 2", len(resList))
	}
	res0, ok := resList[0].(datasource_ps4c_gcp_resource_cuds.ReservedResourcesValue)
	if !ok {
		t.Fatalf("reserved_resources[0] has unexpected type %T", resList[0])
	}
	if got := res0.Amount.ValueString(); got != "4" {
		t.Errorf("reserved_resources[0].amount = %q, want %q", got, "4")
	}
	if got := res0.ResourceKind.ValueString(); got != "VCPU" {
		t.Errorf("reserved_resources[0].resource_kind = %q, want %q", got, "VCPU")
	}
}

func TestPs4cGcpResourceCudsRead(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     *string
		maxResults *int64
		responses  []string
		wantCalls  int
		wantItems  int
		wantCount  int64
	}{
		{
			name:   "auto-pagination single page with status",
			status: new("active"),
			responses: []string{
				`{"items":[{"commitmentNumericId":"CUD-1","state":"ACTIVE"}],"rowCount":1}`,
			},
			wantCalls: 1,
			wantItems: 1,
			wantCount: 1,
		},
		{
			name: "auto-pagination multiple pages",
			responses: []string{
				`{"items":[{"commitmentNumericId":"CUD-1","state":"ACTIVE"}],"pageToken":"page-2"}`,
				`{"items":[{"commitmentNumericId":"CUD-2","state":"ACTIVE"}],"pageToken":null}`,
			},
			wantCalls: 2,
			wantItems: 2,
			wantCount: 2,
		},
		{
			name:       "user-controlled pagination",
			maxResults: new(int64(1)),
			responses: []string{
				`{"items":[{"commitmentNumericId":"CUD-1","state":"ACTIVE"}],"pageToken":"page-2","rowCount":10}`,
			},
			wantCalls: 1,
			wantItems: 1,
			wantCount: 10,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			callIdx := 0
			server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/ps4commitments/v1/gcp/billing-accounts/012345-6789AB-CDEF01/resource-based" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				if tc.status != nil {
					if q := r.URL.Query().Get("status"); q != *tc.status {
						t.Errorf("expected status=%s, got %s", *tc.status, q)
					}
				}
				if callIdx >= len(tc.responses) {
					t.Fatalf("unexpected call %d", callIdx)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.responses[callIdx]))
				callIdx++
			}))

			overrides := map[string]tftypes.Value{
				"billing_account_id": tftypes.NewValue(tftypes.String, "012345-6789AB-CDEF01"),
			}
			if tc.status != nil {
				overrides["status"] = tftypes.NewValue(tftypes.String, *tc.status)
			}
			if tc.maxResults != nil {
				overrides["max_results"] = tftypes.NewValue(tftypes.Number, float64(*tc.maxResults))
			}

			resp := readPs4cGcpResourceCuds(t, server, overrides)
			if resp.Diagnostics.HasError() {
				t.Fatal(resp.Diagnostics)
			}
			if callIdx != tc.wantCalls {
				t.Fatalf("got %d calls, want %d", callIdx, tc.wantCalls)
			}

			var state ps4cGcpResourceCudsDataSourceModel
			if diags := resp.State.Get(t.Context(), &state); diags.HasError() {
				t.Fatal(diags)
			}

			if len(state.Items.Elements()) != tc.wantItems {
				t.Errorf("got %d items, want %d", len(state.Items.Elements()), tc.wantItems)
			}
			if state.RowCount.ValueInt64() != tc.wantCount {
				t.Errorf("got row_count %d, want %d", state.RowCount.ValueInt64(), tc.wantCount)
			}
		})
	}
}

func TestPs4cGcpResourceCudsDataSource_Read_Pagination(t *testing.T) {
	t.Run("max_results_only sends a single manual call and preserves the returned page_token", func(t *testing.T) {
		t.Parallel()

		var requests []*http.Request
		server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests = append(requests, r)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{
				"items": [
					{"commitmentNumericId": "CUD-1", "state": "ACTIVE"},
					{"commitmentNumericId": "CUD-2", "state": "ACTIVE"}
				],
				"pageToken": "next-page-token",
				"rowCount": 2
			}`)
		}))

		resp := readPs4cGcpResourceCuds(t, server, map[string]tftypes.Value{
			"max_results": tftypes.NewValue(tftypes.Number, 2.0),
		})
		if resp.Diagnostics.HasError() {
			t.Fatalf("Read() returned diagnostics: %v", resp.Diagnostics)
		}

		if len(requests) != 1 {
			t.Fatalf("expected exactly 1 API call in manual mode, got %d", len(requests))
		}
		if got := requests[0].URL.Query().Get("maxResults"); got != "2" {
			t.Errorf("maxResults query param = %q, want %q", got, "2")
		}
		if got := requests[0].URL.Query().Get("pageToken"); got != "" {
			t.Errorf("pageToken query param = %q, want empty", got)
		}

		var data ps4cGcpResourceCudsDataSourceModel
		if diags := resp.State.Get(t.Context(), &data); diags.HasError() {
			t.Fatalf("failed to read state: %v", diags)
		}
		if got := len(data.Items.Elements()); got != 2 {
			t.Errorf("items count = %d, want 2", got)
		}
		if got := data.RowCount.ValueInt64(); got != 2 {
			t.Errorf("row_count = %d, want 2", got)
		}
		if got := data.PageToken.ValueString(); got != "next-page-token" {
			t.Errorf("page_token = %q, want %q", got, "next-page-token")
		}
	})

	t.Run("page_token_only auto-paginates from the token until exhausted", func(t *testing.T) {
		t.Parallel()

		var requests []*http.Request
		server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests = append(requests, r)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if r.URL.Query().Get("pageToken") == "start-token" {
				_, _ = fmt.Fprint(w, `{
					"items": [{"commitmentNumericId": "CUD-3", "state": "ACTIVE"}],
					"pageToken": "second-page-token",
					"rowCount": 1
				}`)
				return
			}
			_, _ = fmt.Fprint(w, `{
				"items": [{"commitmentNumericId": "CUD-4", "state": "ACTIVE"}],
				"rowCount": 1
			}`)
		}))

		resp := readPs4cGcpResourceCuds(t, server, map[string]tftypes.Value{
			"page_token": tftypes.NewValue(tftypes.String, "start-token"),
		})
		if resp.Diagnostics.HasError() {
			t.Fatalf("Read() returned diagnostics: %v", resp.Diagnostics)
		}

		if len(requests) != 2 {
			t.Fatalf("expected 2 API calls while auto-paginating to exhaustion, got %d", len(requests))
		}
		if got := requests[0].URL.Query().Get("pageToken"); got != "start-token" {
			t.Errorf("first pageToken = %q, want %q", got, "start-token")
		}
		if got := requests[1].URL.Query().Get("pageToken"); got != "second-page-token" {
			t.Errorf("second pageToken = %q, want %q", got, "second-page-token")
		}

		var data ps4cGcpResourceCudsDataSourceModel
		if diags := resp.State.Get(t.Context(), &data); diags.HasError() {
			t.Fatalf("failed to read state: %v", diags)
		}
		if got := len(data.Items.Elements()); got != 2 {
			t.Errorf("items count = %d, want 2 (accumulated across both pages)", got)
		}
		if got := data.RowCount.ValueInt64(); got != 2 {
			t.Errorf("row_count = %d, want 2", got)
		}
		if !data.PageToken.IsNull() {
			t.Errorf("page_token = %q, want null after auto-pagination completes", data.PageToken.ValueString())
		}
	})

	t.Run("max_results and page_token together send a single manual call with both params", func(t *testing.T) {
		t.Parallel()

		var requests []*http.Request
		server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests = append(requests, r)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{
				"items": [{"commitmentNumericId": "CUD-5", "state": "ACTIVE"}],
				"pageToken": "third-page-token",
				"rowCount": 1
			}`)
		}))

		resp := readPs4cGcpResourceCuds(t, server, map[string]tftypes.Value{
			"max_results": tftypes.NewValue(tftypes.Number, 1.0),
			"page_token":  tftypes.NewValue(tftypes.String, "second-page-token"),
		})
		if resp.Diagnostics.HasError() {
			t.Fatalf("Read() returned diagnostics: %v", resp.Diagnostics)
		}

		if len(requests) != 1 {
			t.Fatalf("expected exactly 1 API call when both params are user-controlled, got %d", len(requests))
		}
		if got := requests[0].URL.Query().Get("maxResults"); got != "1" {
			t.Errorf("maxResults query param = %q, want %q", got, "1")
		}
		if got := requests[0].URL.Query().Get("pageToken"); got != "second-page-token" {
			t.Errorf("pageToken query param = %q, want %q", got, "second-page-token")
		}

		var data ps4cGcpResourceCudsDataSourceModel
		if diags := resp.State.Get(t.Context(), &data); diags.HasError() {
			t.Fatalf("failed to read state: %v", diags)
		}
		if got := len(data.Items.Elements()); got != 1 {
			t.Errorf("items count = %d, want 1", got)
		}
		if got := data.PageToken.ValueString(); got != "third-page-token" {
			t.Errorf("page_token = %q, want %q", got, "third-page-token")
		}
	})
}

func TestPs4cGcpResourceCudsReadErrors(t *testing.T) {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal server error"}`))
	}))

	resp := readPs4cGcpResourceCuds(t, server, nil)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for 500, got none")
	}
	if !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "status 500") {
		t.Errorf("expected error containing 'status 500', got: %v", resp.Diagnostics)
	}
}

func readPs4cGcpResourceCuds(t *testing.T, server *httptest.Server, overrides map[string]tftypes.Value) datasource.ReadResponse {
	t.Helper()
	ctx := t.Context()
	httpClient := server.Client()
	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}

	ds := NewPs4cGcpResourceCudsDataSource()
	var configureResp datasource.ConfigureResponse
	ds.(datasource.DataSourceWithConfigure).Configure(ctx, datasource.ConfigureRequest{ProviderData: client}, &configureResp)
	if configureResp.Diagnostics.HasError() {
		t.Fatal(configureResp.Diagnostics)
	}

	var schemaResp datasource.SchemaResponse
	ds.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
	if diags := schemaResp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatal(diags)
	}

	attrTypes := map[string]tftypes.Type{}
	values := map[string]tftypes.Value{}
	for name, attribute := range schemaResp.Schema.Attributes {
		tfType := attribute.GetType().TerraformType(ctx)
		attrTypes[name] = tfType
		if val, ok := overrides[name]; ok {
			values[name] = val
		} else if name == "billing_account_id" {
			values[name] = tftypes.NewValue(tfType, "012345-6789AB-CDEF01")
		} else {
			values[name] = tftypes.NewValue(tfType, nil)
		}
	}

	resp := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	ds.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(tftypes.Object{AttributeTypes: attrTypes}, values)}}, &resp)
	return resp
}
