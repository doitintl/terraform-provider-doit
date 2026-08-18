package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// readAnomaliesHelper builds an anomaliesDataSource backed by a client pointed at
// serverURL, invokes Read with the given config overrides, and returns the resulting state and diagnostics.
func readAnomaliesHelper(t *testing.T, serverURL string, overrides map[string]tftypes.Value) (anomaliesDataSourceModel, tfsdk.State) {
	t.Helper()

	client, err := models.NewClientWithResponses(serverURL)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ds := &anomaliesDataSource{client: client}
	ctx := context.Background()

	schemaResp := &datasource.SchemaResponse{}
	ds.Schema(ctx, datasource.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("failed to get schema: %v", schemaResp.Diagnostics)
	}

	configValues := map[string]tftypes.Value{}
	attrTypes := map[string]tftypes.Type{}
	for name, attr := range schemaResp.Schema.Attributes {
		tfType := attr.GetType().TerraformType(ctx)
		attrTypes[name] = tfType
		if override, ok := overrides[name]; ok {
			configValues[name] = override
			continue
		}
		configValues[name] = tftypes.NewValue(tfType, nil)
	}

	configVal := tftypes.NewValue(tftypes.Object{AttributeTypes: attrTypes}, configValues)
	config := tfsdk.Config{
		Schema: schemaResp.Schema,
		Raw:    configVal,
	}

	readResp := &datasource.ReadResponse{
		State: tfsdk.State{Schema: schemaResp.Schema},
	}
	ds.Read(ctx, datasource.ReadRequest{Config: config}, readResp)

	if readResp.Diagnostics.HasError() {
		t.Fatalf("Read() returned diagnostics: %v", readResp.Diagnostics)
	}

	var data anomaliesDataSourceModel
	if diags := readResp.State.Get(ctx, &data); diags.HasError() {
		t.Fatalf("failed to read state: %v", diags)
	}

	return data, readResp.State
}

func TestAnomaliesDataSource_UnknownInputs(t *testing.T) {
	t.Parallel()

	unknownCases := []struct {
		name      string
		overrides map[string]tftypes.Value
	}{
		{
			name: "unknown filter",
			overrides: map[string]tftypes.Value{
				"filter": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
			},
		},
		{
			name: "unknown min_creation_time",
			overrides: map[string]tftypes.Value{
				"min_creation_time": tftypes.NewValue(tftypes.Number, tftypes.UnknownValue),
			},
		},
		{
			name: "unknown max_creation_time",
			overrides: map[string]tftypes.Value{
				"max_creation_time": tftypes.NewValue(tftypes.Number, tftypes.UnknownValue),
			},
		},
		{
			name: "unknown sort_by",
			overrides: map[string]tftypes.Value{
				"sort_by": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
			},
		},
		{
			name: "unknown sort_order",
			overrides: map[string]tftypes.Value{
				"sort_order": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
			},
		},
		{
			name: "unknown max_results",
			overrides: map[string]tftypes.Value{
				"max_results": tftypes.NewValue(tftypes.Number, tftypes.UnknownValue),
			},
		},
		{
			name: "unknown page_token",
			overrides: map[string]tftypes.Value{
				"page_token": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
			},
		},
		{
			name: "unknown include_notifications",
			overrides: map[string]tftypes.Value{
				"include_notifications": tftypes.NewValue(tftypes.Bool, tftypes.UnknownValue),
			},
		},
	}

	for _, tc := range unknownCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var requestCount atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestCount.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			data, _ := readAnomaliesHelper(t, server.URL, tc.overrides)

			if reqs := requestCount.Load(); reqs != 0 {
				t.Errorf("expected 0 API requests when inputs are unknown, got %d", reqs)
			}

			if !data.Anomalies.IsUnknown() {
				t.Errorf("expected Anomalies to be unknown, got %v", data.Anomalies)
			}
			if !data.AnomalySummary.IsUnknown() {
				t.Errorf("expected AnomalySummary to be unknown, got %v", data.AnomalySummary)
			}
			if !data.RowCount.IsUnknown() {
				t.Errorf("expected RowCount to be unknown, got %v", data.RowCount)
			}
			if !data.TotalCount.IsUnknown() {
				t.Errorf("expected TotalCount to be unknown, got %v", data.TotalCount)
			}
			if !data.TotalCountExact.IsUnknown() {
				t.Errorf("expected TotalCountExact to be unknown, got %v", data.TotalCountExact)
			}
			if !data.Truncated.IsUnknown() {
				t.Errorf("expected Truncated to be unknown, got %v", data.Truncated)
			}
			if !data.PageToken.IsUnknown() {
				t.Errorf("expected PageToken to be unknown, got %v", data.PageToken)
			}
		})
	}
}

func TestAnomaliesDataSource_AutoPaginationMetadataFromFirstPage(t *testing.T) {
	t.Parallel()

	var pageRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqNum := pageRequests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if reqNum == 1 {
			// Page 1: contains initial snapshot metadata
			_, _ = fmt.Fprint(w, `{
				"anomalies": [
					{
						"id": "anomaly-1",
						"serviceName": "AmazonEC2",
						"costOfAnomaly": 10.5,
						"startTime": 1704067200000,
						"severityLevel": "critical"
					}
				],
				"pageToken": "page-2-token",
				"rowCount": 1,
				"totalCount": 2,
				"totalCountExact": true,
				"truncated": false,
				"anomalySummary": {
					"countBySeverity": {
						"critical": 1,
						"warning": 0,
						"information": 0
					},
					"totalCostOfAnomaly": 10.5
				}
			}`)
			return
		}

		// Page 2: backend returns shifted snapshot due to concurrent write
		_, _ = fmt.Fprint(w, `{
			"anomalies": [
				{
					"id": "anomaly-2",
					"serviceName": "CloudStorage",
					"costOfAnomaly": 20.0,
					"startTime": 1704070800000,
					"severityLevel": "warning"
				}
			],
			"pageToken": null,
			"rowCount": 1,
			"totalCount": 999,
			"totalCountExact": true,
			"truncated": false,
			"anomalySummary": {
				"countBySeverity": {
					"critical": 50,
					"warning": 50,
					"information": 0
				},
				"totalCostOfAnomaly": 9999.99
			}
		}`)
	}))
	defer server.Close()

	// Zero overrides = auto-pagination mode
	data, _ := readAnomaliesHelper(t, server.URL, map[string]tftypes.Value{})

	if reqs := pageRequests.Load(); reqs != 2 {
		t.Fatalf("expected 2 API requests in auto-pagination mode, got %d", reqs)
	}

	// Collected anomalies from both pages
	if got := len(data.Anomalies.Elements()); got != 2 {
		t.Errorf("anomalies count = %d, want 2", got)
	}
	if got := data.RowCount.ValueInt64(); got != 2 {
		t.Errorf("row_count = %d, want 2 (accumulated)", got)
	}
	if !data.PageToken.IsNull() {
		t.Errorf("page_token in auto mode should be null, got %v", data.PageToken)
	}
	if got := data.Truncated.ValueBool(); got != false {
		t.Errorf("truncated in auto mode should be false, got %v", got)
	}

	// Metadata MUST be captured from Page 1 (initial query snapshot), NOT Page 2
	if got := data.TotalCount.ValueInt64(); got != 2 {
		t.Errorf("total_count = %d, want 2 (from page 1 snapshot)", got)
	}

	summaryVal, diags := data.AnomalySummary.ToObjectValue(context.Background())
	if diags.HasError() {
		t.Fatalf("failed to convert summary to object: %v", diags)
	}
	totalCostVal := summaryVal.Attributes()["total_cost_of_anomaly"].(types.Float64)
	if got := totalCostVal.ValueFloat64(); got != 10.5 {
		t.Errorf("total_cost_of_anomaly = %f, want 10.5 (from page 1 snapshot)", got)
	}
}

func TestAnomaliesDataSource_EmptyAnomaliesList(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{
			"anomalies": [],
			"rowCount": 0,
			"totalCount": 0,
			"totalCountExact": true,
			"truncated": false,
			"anomalySummary": {
				"countBySeverity": {
					"critical": 0,
					"warning": 0,
					"information": 0
				},
				"totalCostOfAnomaly": 0.0
			}
		}`)
	}))
	defer server.Close()

	data, _ := readAnomaliesHelper(t, server.URL, map[string]tftypes.Value{})

	if data.Anomalies.IsNull() {
		t.Errorf("expected Anomalies to not be null")
	}
	if data.Anomalies.IsUnknown() {
		t.Errorf("expected Anomalies to not be unknown")
	}
	if got := len(data.Anomalies.Elements()); got != 0 {
		t.Errorf("expected 0 anomalies in list, got %d", got)
	}
	if got := data.RowCount.ValueInt64(); got != 0 {
		t.Errorf("expected row_count to be 0, got %d", got)
	}
	if got := data.TotalCount.ValueInt64(); got != 0 {
		t.Errorf("expected total_count to be 0, got %d", got)
	}
}
