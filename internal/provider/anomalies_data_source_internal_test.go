package provider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_anomalies"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// readAnomaliesHelper builds an anomaliesDataSource backed by a client pointed at
// server, invokes Read with the given config overrides, and returns the resulting state and diagnostics.
func readAnomaliesHelper(t *testing.T, server *httptest.Server, overrides map[string]tftypes.Value) (anomaliesDataSourceModel, tfsdk.State) {
	t.Helper()

	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ds := &anomaliesDataSource{client: client}
	ctx := t.Context()

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
			server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestCount.Add(1)
				w.WriteHeader(http.StatusOK)
			}))

			data, _ := readAnomaliesHelper(t, server, tc.overrides)

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
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
						"severityLevel": "critical",
						"monitorLevel": "service"
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
					"severityLevel": "warning",
					"monitorLevel": "sku"
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

	// Zero overrides = auto-pagination mode
	data, _ := readAnomaliesHelper(t, server, map[string]tftypes.Value{})

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

	summaryVal, diags := data.AnomalySummary.ToObjectValue(t.Context())
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

	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	data, _ := readAnomaliesHelper(t, server, map[string]tftypes.Value{})

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

func TestAnomaliesDataSource_EntityFieldsMapping(t *testing.T) {
	t.Parallel()

	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{
			"anomalies": [
				{
					"id": "anomaly-populated",
					"serviceName": "Anthropic Claude",
					"costOfAnomaly": 150.0,
					"startTime": 1704067200000,
					"severityLevel": "critical",
					"monitorLevel": "service",
					"scope": "user_12345",
					"entityLabel": "User",
					"entityName": "alice@example.com",
					"providerDisplayName": "Anthropic (Analytics API)"
				},
				{
					"id": "anomaly-omitted",
					"serviceName": "AmazonEC2",
					"costOfAnomaly": 50.0,
					"startTime": 1704067200000,
					"severityLevel": "warning",
					"monitorLevel": "service",
					"scope": "123456789012"
				}
			],
			"rowCount": 2,
			"totalCount": 2,
			"totalCountExact": true,
			"truncated": false,
			"anomalySummary": {
				"countBySeverity": {
					"critical": 1,
					"warning": 1,
					"information": 0
				},
				"totalCostOfAnomaly": 200.0
			}
		}`)
	}))

	data, _ := readAnomaliesHelper(t, server, map[string]tftypes.Value{})

	if got := len(data.Anomalies.Elements()); got != 2 {
		t.Fatalf("expected 2 anomalies, got %d", got)
	}

	// First anomaly: entity fields populated
	first, ok := data.Anomalies.Elements()[0].(datasource_anomalies.AnomaliesValue)
	if !ok {
		t.Fatalf("expected AnomaliesValue type, got %T", data.Anomalies.Elements()[0])
	}
	if got := first.EntityLabel.ValueString(); got != "User" {
		t.Errorf("entity_label = %q, want \"User\"", got)
	}
	if got := first.EntityName.ValueString(); got != "alice@example.com" {
		t.Errorf("entity_name = %q, want \"alice@example.com\"", got)
	}
	if got := first.ProviderDisplayName.ValueString(); got != "Anthropic (Analytics API)" {
		t.Errorf("provider_display_name = %q, want \"Anthropic (Analytics API)\"", got)
	}

	// Second anomaly: entity fields omitted (null)
	second, ok := data.Anomalies.Elements()[1].(datasource_anomalies.AnomaliesValue)
	if !ok {
		t.Fatalf("expected AnomaliesValue type, got %T", data.Anomalies.Elements()[1])
	}
	if !second.EntityLabel.IsNull() {
		t.Errorf("expected entity_label to be null, got %v", second.EntityLabel)
	}
	if !second.EntityName.IsNull() {
		t.Errorf("expected entity_name to be null, got %v", second.EntityName)
	}
	if !second.ProviderDisplayName.IsNull() {
		t.Errorf("expected provider_display_name to be null, got %v", second.ProviderDisplayName)
	}
}

func TestAnomaliesDataSource_LinkedAnomalies(t *testing.T) {
	t.Parallel()

	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{
			"anomalies": [
				{
					"id": "anomaly-1",
					"platform": "google-cloud",
					"scope": "project-1",
					"serviceName": "BigQuery",
					"costOfAnomaly": 100.0,
					"startTime": 1704067200000,
					"severityLevel": "critical",
					"monitorLevel": "service",
					"timeFrame": "day",
					"linkedAnomalies": ["anomaly-2", "anomaly-3"]
				},
				{
					"id": "anomaly-2",
					"platform": "google-cloud",
					"scope": "project-1",
					"serviceName": "BigQuery",
					"costOfAnomaly": 50.0,
					"startTime": 1704067200000,
					"severityLevel": "warning",
					"monitorLevel": "sku",
					"timeFrame": "day",
					"linkedAnomalies": []
				},
				{
					"id": "anomaly-3",
					"platform": "amazon-web-services",
					"scope": "123456789012",
					"serviceName": "AmazonEC2",
					"costOfAnomaly": 25.0,
					"startTime": 1704067200000,
					"severityLevel": "warning",
					"monitorLevel": "service",
					"timeFrame": "day"
				}
			],
			"rowCount": 3,
			"totalCount": 3,
			"totalCountExact": true,
			"truncated": false,
			"anomalySummary": {
				"countBySeverity": {
					"critical": 1,
					"warning": 2,
					"information": 0
				},
				"totalCostOfAnomaly": 175.0
			}
		}`)
	}))

	data, _ := readAnomaliesHelper(t, server, map[string]tftypes.Value{})

	if got := len(data.Anomalies.Elements()); got != 3 {
		t.Fatalf("expected 3 anomalies, got %d", got)
	}

	// First anomaly: populated linked_anomalies
	first, ok := data.Anomalies.Elements()[0].(datasource_anomalies.AnomaliesValue)
	if !ok {
		t.Fatalf("expected AnomaliesValue type, got %T", data.Anomalies.Elements()[0])
	}
	if first.LinkedAnomalies.IsNull() {
		t.Error("expected first anomaly LinkedAnomalies not to be null")
	}
	if first.LinkedAnomalies.IsUnknown() {
		t.Error("expected first anomaly LinkedAnomalies not to be unknown")
	}
	var firstElements []string
	if d := first.LinkedAnomalies.ElementsAs(t.Context(), &firstElements, false); d.HasError() {
		t.Fatalf("ElementsAs() returned diagnostics: %v", d)
	}
	if len(firstElements) != 2 {
		t.Fatalf("expected 2 linked anomalies, got %d", len(firstElements))
	}
	if firstElements[0] != "anomaly-2" || firstElements[1] != "anomaly-3" {
		t.Errorf("expected [\"anomaly-2\", \"anomaly-3\"], got %v", firstElements)
	}

	// Second anomaly: empty linked_anomalies
	second, ok := data.Anomalies.Elements()[1].(datasource_anomalies.AnomaliesValue)
	if !ok {
		t.Fatalf("expected AnomaliesValue type, got %T", data.Anomalies.Elements()[1])
	}
	if second.LinkedAnomalies.IsNull() {
		t.Error("expected second anomaly LinkedAnomalies not to be null")
	}
	if second.LinkedAnomalies.IsUnknown() {
		t.Error("expected second anomaly LinkedAnomalies not to be unknown")
	}
	if got := len(second.LinkedAnomalies.Elements()); got != 0 {
		t.Errorf("expected empty LinkedAnomalies, got %d elements", got)
	}

	// Third anomaly: omitted linked_anomalies
	third, ok := data.Anomalies.Elements()[2].(datasource_anomalies.AnomaliesValue)
	if !ok {
		t.Fatalf("expected AnomaliesValue type, got %T", data.Anomalies.Elements()[2])
	}
	if third.LinkedAnomalies.IsNull() {
		t.Error("expected third anomaly LinkedAnomalies not to be null (computed list convention)")
	}
	if third.LinkedAnomalies.IsUnknown() {
		t.Error("expected third anomaly LinkedAnomalies not to be unknown")
	}
	if got := len(third.LinkedAnomalies.Elements()); got != 0 {
		t.Errorf("expected empty LinkedAnomalies fallback, got %d elements", got)
	}
}
