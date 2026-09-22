package provider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_ps4c_gcp_spend_cuds"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestMapGcpSpendCudsToItemsList_FullyPopulated(t *testing.T) {
	ctx := t.Context()

	endTime := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	exportDate := time.Date(2025, 7, 1, 12, 0, 0, 0, time.UTC)
	startTime := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)

	cuds := []models.GcpSpendCud{
		{
			CommitmentAmount:     new(12.5),
			CommitmentUnit:       new("$/hr"),
			ConsumptionModelId:   new("MODEL-123"),
			CudId:                "spend-cud-99",
			CudProductId:         new("prod-flexible"),
			CudProductName:       "Compute Flexible",
			CudProductType:       new("Spend-Based"),
			EndTime:              valueToNullable(endTime),
			EntitlementScope:     new("billingAccounts/012345-6789AB-CDEF01"),
			ExportDate:           &exportDate,
			LastMonthSavings:     new(345.67),
			LastMonthUtilization: new(0.95),
			Name:                 new("my-compute-flexible-cud"),
			Region:               new("global"),
			StartTime:            valueToNullable(startTime),
			State:                "ACTIVE",
			SubType:              new("COMPUTE_FLEXIBLE"),
			Term:                 new("P1Y"),
		},
	}

	list, diags := mapGcpSpendCudsToItemsList(ctx, cuds)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	elements := list.Elements()
	if len(elements) != 1 {
		t.Fatalf("items has %d elements, want 1", len(elements))
	}

	item, ok := elements[0].(datasource_ps4c_gcp_spend_cuds.ItemsValue)
	if !ok {
		t.Fatalf("items[0] has unexpected type %T", elements[0])
	}

	if got := item.CommitmentAmount.ValueFloat64(); got != 12.5 {
		t.Errorf("commitment_amount = %v, want 12.5", got)
	}
	if got := item.CommitmentUnit.ValueString(); got != "$/hr" {
		t.Errorf("commitment_unit = %q, want %q", got, "$/hr")
	}
	if got := item.ConsumptionModelId.ValueString(); got != "MODEL-123" {
		t.Errorf("consumption_model_id = %q, want %q", got, "MODEL-123")
	}
	if got := item.CudId.ValueString(); got != "spend-cud-99" {
		t.Errorf("cud_id = %q, want %q", got, "spend-cud-99")
	}
	if got := item.CudProductId.ValueString(); got != "prod-flexible" {
		t.Errorf("cud_product_id = %q, want %q", got, "prod-flexible")
	}
	if got := item.CudProductName.ValueString(); got != "Compute Flexible" {
		t.Errorf("cud_product_name = %q, want %q", got, "Compute Flexible")
	}
	if got := item.CudProductType.ValueString(); got != "Spend-Based" {
		t.Errorf("cud_product_type = %q, want %q", got, "Spend-Based")
	}
	if got := item.EndTime.ValueString(); got != endTime.Format(time.RFC3339) {
		t.Errorf("end_time = %q, want %q", got, endTime.Format(time.RFC3339))
	}
	if got := item.EntitlementScope.ValueString(); got != "billingAccounts/012345-6789AB-CDEF01" {
		t.Errorf("entitlement_scope = %q, want %q", got, "billingAccounts/012345-6789AB-CDEF01")
	}
	if got := item.ExportDate.ValueString(); got != exportDate.Format(time.RFC3339) {
		t.Errorf("export_date = %q, want %q", got, exportDate.Format(time.RFC3339))
	}
	if got := item.LastMonthSavings.ValueFloat64(); got != 345.67 {
		t.Errorf("last_month_savings = %v, want 345.67", got)
	}
	if got := item.LastMonthUtilization.ValueFloat64(); got != 0.95 {
		t.Errorf("last_month_utilization = %v, want 0.95", got)
	}
	if got := item.Name.ValueString(); got != "my-compute-flexible-cud" {
		t.Errorf("name = %q, want %q", got, "my-compute-flexible-cud")
	}
	if got := item.Region.ValueString(); got != "global" {
		t.Errorf("region = %q, want %q", got, "global")
	}
	if got := item.StartTime.ValueString(); got != startTime.Format(time.RFC3339) {
		t.Errorf("start_time = %q, want %q", got, startTime.Format(time.RFC3339))
	}
	if got := item.State.ValueString(); got != "ACTIVE" {
		t.Errorf("state = %q, want %q", got, "ACTIVE")
	}
	if got := item.SubType.ValueString(); got != "COMPUTE_FLEXIBLE" {
		t.Errorf("sub_type = %q, want %q", got, "COMPUTE_FLEXIBLE")
	}
	if got := item.Term.ValueString(); got != "P1Y" {
		t.Errorf("term = %q, want %q", got, "P1Y")
	}
}

func TestMapGcpSpendCudsToItemsList_EmptyAndNulls(t *testing.T) {
	ctx := t.Context()

	list, diags := mapGcpSpendCudsToItemsList(ctx, []models.GcpSpendCud{})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if list.IsNull() {
		t.Fatal("expected empty list, got null")
	}
	if len(list.Elements()) != 0 {
		t.Fatalf("expected 0 elements, got %d", len(list.Elements()))
	}

	minimalCuds := []models.GcpSpendCud{
		{
			CudId:          "minimal-cud",
			CudProductName: "Cloud SQL",
			State:          "ACTIVE",
		},
	}

	minList, diags := mapGcpSpendCudsToItemsList(ctx, minimalCuds)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	minItem, ok := minList.Elements()[0].(datasource_ps4c_gcp_spend_cuds.ItemsValue)
	if !ok {
		t.Fatalf("unexpected item type: %T", minList.Elements()[0])
	}
	if !minItem.StartTime.IsNull() {
		t.Errorf("expected null start_time, got %v", minItem.StartTime)
	}
	if !minItem.EndTime.IsNull() {
		t.Errorf("expected null end_time, got %v", minItem.EndTime)
	}
	if !minItem.ExportDate.IsNull() {
		t.Errorf("expected null export_date, got %v", minItem.ExportDate)
	}
	if !minItem.CommitmentAmount.IsNull() {
		t.Errorf("expected null commitment_amount, got %v", minItem.CommitmentAmount)
	}
}

func TestPs4cGcpSpendCudsRead(t *testing.T) {
	for _, tc := range []struct {
		name        string
		status      *string
		gcpService  *string
		region      *string
		maxResults  *int64
		responses   []string
		wantCalls   int
		wantItems   int
		wantCount   int64
		checkParams func(t *testing.T, r *http.Request)
	}{
		{
			name:   "auto-pagination single page with status",
			status: new("active"),
			responses: []string{
				`{"items":[{"cudId":"CUD-1","cudProductName":"Compute Flexible","state":"ACTIVE"}],"rowCount":1}`,
			},
			wantCalls: 1,
			wantItems: 1,
			wantCount: 1,
			checkParams: func(t *testing.T, r *http.Request) {
				if q := r.URL.Query().Get("status"); q != "active" {
					t.Errorf("expected status=active, got %s", q)
				}
			},
		},
		{
			name:       "filter by gcp_service and region",
			gcpService: new("compute"),
			region:     new("global"),
			responses: []string{
				`{"items":[{"cudId":"CUD-2","cudProductName":"Compute Flexible","state":"ACTIVE","region":"global"}],"rowCount":1}`,
			},
			wantCalls: 1,
			wantItems: 1,
			wantCount: 1,
			checkParams: func(t *testing.T, r *http.Request) {
				if q := r.URL.Query().Get("gcp_service"); q != "compute" {
					t.Errorf("expected gcp_service=compute, got %s", q)
				}
				if q := r.URL.Query().Get("region"); q != "global" {
					t.Errorf("expected region=global, got %s", q)
				}
			},
		},
		{
			name: "auto-pagination multiple pages",
			responses: []string{
				`{"items":[{"cudId":"CUD-1","cudProductName":"Compute Flexible","state":"ACTIVE"}],"pageToken":"page-2"}`,
				`{"items":[{"cudId":"CUD-2","cudProductName":"Cloud SQL","state":"ACTIVE"}],"pageToken":null}`,
			},
			wantCalls: 2,
			wantItems: 2,
			wantCount: 2,
		},
		{
			name: "auto-pagination stops when pageToken is empty string",
			responses: []string{
				`{"items":[{"cudId":"CUD-1","cudProductName":"Compute Flexible","state":"ACTIVE"}],"pageToken":"page-2"}`,
				`{"items":[{"cudId":"CUD-2","cudProductName":"Cloud SQL","state":"ACTIVE"}],"pageToken":""}`,
			},
			wantCalls: 2,
			wantItems: 2,
			wantCount: 2,
		},
		{
			name:       "user-controlled pagination",
			maxResults: new(int64(1)),
			responses: []string{
				`{"items":[{"cudId":"CUD-1","cudProductName":"Compute Flexible","state":"ACTIVE"}],"pageToken":"page-2","rowCount":10}`,
			},
			wantCalls: 1,
			wantItems: 1,
			wantCount: 10,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			callIdx := 0
			server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/ps4commitments/v1/gcp/billing-accounts/012345-6789AB-CDEF01/spend-based" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				if tc.checkParams != nil {
					tc.checkParams(t, r)
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
			if tc.gcpService != nil {
				overrides["gcp_service"] = tftypes.NewValue(tftypes.String, *tc.gcpService)
			}
			if tc.region != nil {
				overrides["region"] = tftypes.NewValue(tftypes.String, *tc.region)
			}
			if tc.maxResults != nil {
				overrides["max_results"] = tftypes.NewValue(tftypes.Number, float64(*tc.maxResults))
			}

			resp := readPs4cGcpSpendCuds(t, server, overrides)
			if resp.Diagnostics.HasError() {
				t.Fatal(resp.Diagnostics)
			}
			if callIdx != tc.wantCalls {
				t.Fatalf("got %d calls, want %d", callIdx, tc.wantCalls)
			}

			var state ps4cGcpSpendCudsDataSourceModel
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

func TestPs4cGcpSpendCudsDataSource_Read_Pagination(t *testing.T) {
	t.Run("max_results_only sends a single manual call and preserves the returned page_token", func(t *testing.T) {
		t.Parallel()

		var requests []*http.Request
		server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests = append(requests, r)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{
				"items": [
					{"cudId": "CUD-1", "cudProductName": "Compute Flexible", "state": "ACTIVE"},
					{"cudId": "CUD-2", "cudProductName": "Cloud SQL", "state": "ACTIVE"}
				],
				"pageToken": "next-page-token",
				"rowCount": 2
			}`)
		}))

		resp := readPs4cGcpSpendCuds(t, server, map[string]tftypes.Value{
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

		var data ps4cGcpSpendCudsDataSourceModel
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

	t.Run("max_results with empty string page_token in response normalizes to null", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{
				"items": [
					{"cudId": "CUD-1", "cudProductName": "Compute Flexible", "state": "ACTIVE"}
				],
				"pageToken": "",
				"rowCount": 1
			}`)
		}))

		resp := readPs4cGcpSpendCuds(t, server, map[string]tftypes.Value{
			"max_results": tftypes.NewValue(tftypes.Number, 10.0),
		})
		if resp.Diagnostics.HasError() {
			t.Fatalf("Read() returned diagnostics: %v", resp.Diagnostics)
		}

		var data ps4cGcpSpendCudsDataSourceModel
		if diags := resp.State.Get(t.Context(), &data); diags.HasError() {
			t.Fatalf("failed to read state: %v", diags)
		}
		if !data.PageToken.IsNull() {
			t.Errorf("expected page_token to be null, got %v", data.PageToken)
		}
		if got := data.RowCount.ValueInt64(); got != 1 {
			t.Errorf("row_count = %d, want 1", got)
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
					"items": [{"cudId": "CUD-3", "cudProductName": "Compute Flexible", "state": "ACTIVE"}],
					"pageToken": "second-page-token",
					"rowCount": 1
				}`)
				return
			}
			_, _ = fmt.Fprint(w, `{
				"items": [{"cudId": "CUD-4", "cudProductName": "Cloud SQL", "state": "ACTIVE"}],
				"rowCount": 1
			}`)
		}))

		resp := readPs4cGcpSpendCuds(t, server, map[string]tftypes.Value{
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

		var data ps4cGcpSpendCudsDataSourceModel
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
				"items": [{"cudId": "CUD-5", "cudProductName": "Compute Flexible", "state": "ACTIVE"}],
				"pageToken": "third-page-token",
				"rowCount": 1
			}`)
		}))

		resp := readPs4cGcpSpendCuds(t, server, map[string]tftypes.Value{
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

		var data ps4cGcpSpendCudsDataSourceModel
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

func TestPs4cGcpSpendCudsRead_UnknownConfig(t *testing.T) {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("API should not be called when config contains unknown values")
	}))

	resp := readPs4cGcpSpendCuds(t, server, map[string]tftypes.Value{
		"billing_account_id": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
	})
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	var data ps4cGcpSpendCudsDataSourceModel
	if diags := resp.State.Get(t.Context(), &data); diags.HasError() {
		t.Fatalf("failed to read state: %v", diags)
	}
	if !data.Items.IsUnknown() {
		t.Error("expected items to be unknown")
	}
	if !data.RowCount.IsUnknown() {
		t.Error("expected row_count to be unknown")
	}
	if !data.PageToken.IsUnknown() {
		t.Error("expected page_token to be unknown")
	}
}

func TestPs4cGcpSpendCudsReadErrors(t *testing.T) {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal server error"}`))
	}))

	resp := readPs4cGcpSpendCuds(t, server, nil)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for 500, got none")
	}
	if !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "status 500") {
		t.Errorf("expected error containing 'status 500', got: %v", resp.Diagnostics)
	}
}

func readPs4cGcpSpendCuds(t *testing.T, server *httptest.Server, overrides map[string]tftypes.Value) datasource.ReadResponse {
	t.Helper()
	ctx := t.Context()
	httpClient := server.Client()
	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}

	ds := NewPs4cGcpSpendCudsDataSource()
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
