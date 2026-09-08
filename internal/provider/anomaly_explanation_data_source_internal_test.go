package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_anomaly_explanation"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// readAnomalyExplanationHelper builds an anomalyExplanationDataSource backed by a
// client pointed at server, invokes Read with the given config overrides, and returns
// the resulting state and diagnostics.
func readAnomalyExplanationHelper(t *testing.T, server *httptest.Server, overrides map[string]tftypes.Value) (anomalyExplanationDataSourceModel, tfsdk.State, diag.Diagnostics) {
	t.Helper()

	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ds := &anomalyExplanationDataSource{client: client}
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

	var data anomalyExplanationDataSourceModel
	if !readResp.Diagnostics.HasError() {
		if diags := readResp.State.Get(ctx, &data); diags.HasError() {
			t.Fatalf("failed to read state: %v", diags)
		}
	}

	return data, readResp.State, readResp.Diagnostics
}

func TestAnomalyExplanationDataSource_UnknownID(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	overrides := map[string]tftypes.Value{
		"id": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
	}

	data, _, diags := readAnomalyExplanationHelper(t, server, overrides)
	if diags.HasError() {
		t.Fatalf("Read() returned unexpected diagnostics: %v", diags)
	}

	if reqs := requestCount.Load(); reqs != 0 {
		t.Errorf("expected 0 API requests when id is unknown, got %d", reqs)
	}

	if !data.Facts.IsUnknown() {
		t.Errorf("expected Facts to be unknown, got %v", data.Facts)
	}
	if !data.Explanation.IsUnknown() {
		t.Errorf("expected Explanation to be unknown, got %v", data.Explanation)
	}
	if !data.Evidence.IsUnknown() {
		t.Errorf("expected Evidence to be unknown, got %v", data.Evidence)
	}
}

func TestAnomalyExplanationDataSource_FactsMapping(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name            string
		responseJSON    string
		wantActualCost  *float64
		wantExpectedMax *float64
		wantServiceName string
		wantPlatform    string
		wantSeverity    string
	}{
		{
			name: "populated cost fields",
			responseJSON: `{
				"facts": {
					"serviceName": "BigQuery",
					"platform": "google-cloud",
					"scope": "project-123",
					"severityLevel": "warning",
					"costOfAnomaly": 42.0,
					"actualCost": 100.0,
					"expectedMaxCost": 58.0,
					"top3SKUs": [{"name": "SKU1", "cost": 10.0}]
				},
				"evidence": [{"type": "anomaly", "id": "ev-1"}],
				"explanation": {"text": "spike", "generatedBy": "ava", "aiGenerated": true}
			}`,
			wantActualCost:  new(100.0),
			wantExpectedMax: new(58.0),
			wantServiceName: "BigQuery",
			wantPlatform:    "google-cloud",
			wantSeverity:    "warning",
		},
		{
			name: "omitted nullable cost fields",
			responseJSON: `{
				"facts": {
					"serviceName": "AmazonEC2",
					"platform": "amazon-web-services",
					"scope": "123456789012",
					"severityLevel": "critical",
					"costOfAnomaly": 10.0,
					"top3SKUs": []
				},
				"evidence": [],
				"explanation": {"text": "spike", "generatedBy": "ava", "aiGenerated": true}
			}`,
			wantActualCost:  nil,
			wantExpectedMax: nil,
			wantServiceName: "AmazonEC2",
			wantPlatform:    "amazon-web-services",
			wantSeverity:    "critical",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprint(w, tc.responseJSON)
			}))
			defer server.Close()

			overrides := map[string]tftypes.Value{
				"id": tftypes.NewValue(tftypes.String, "test-anomaly-id"),
			}

			data, _, diags := readAnomalyExplanationHelper(t, server, overrides)
			if diags.HasError() {
				t.Fatalf("Read() returned unexpected diagnostics: %v", diags)
			}

			if data.Facts.IsNull() {
				t.Fatal("expected Facts to be populated, got null")
			}

			if got := data.Facts.ServiceName.ValueString(); got != tc.wantServiceName {
				t.Errorf("ServiceName = %q, want %q", got, tc.wantServiceName)
			}
			if got := data.Facts.Platform.ValueString(); got != tc.wantPlatform {
				t.Errorf("Platform = %q, want %q", got, tc.wantPlatform)
			}
			if got := data.Facts.SeverityLevel.ValueString(); got != tc.wantSeverity {
				t.Errorf("SeverityLevel = %q, want %q", got, tc.wantSeverity)
			}

			if tc.wantActualCost != nil {
				if data.Facts.ActualCost.IsNull() {
					t.Errorf("expected ActualCost to be populated, got null")
				} else if got := data.Facts.ActualCost.ValueFloat64(); got != *tc.wantActualCost {
					t.Errorf("ActualCost = %v, want %v", got, *tc.wantActualCost)
				}
			} else if !data.Facts.ActualCost.IsNull() {
				t.Errorf("expected ActualCost to be null, got %v", data.Facts.ActualCost)
			}

			if tc.wantExpectedMax != nil {
				if data.Facts.ExpectedMaxCost.IsNull() {
					t.Errorf("expected ExpectedMaxCost to be populated, got null")
				} else if got := data.Facts.ExpectedMaxCost.ValueFloat64(); got != *tc.wantExpectedMax {
					t.Errorf("ExpectedMaxCost = %v, want %v", got, *tc.wantExpectedMax)
				}
			} else if !data.Facts.ExpectedMaxCost.IsNull() {
				t.Errorf("expected ExpectedMaxCost to be null, got %v", data.Facts.ExpectedMaxCost)
			}

			// top3skus must always be a non-null list (iterable convention)
			if data.Facts.Top3skus.IsNull() {
				t.Error("expected Facts.Top3skus to be non-null (iterable convention)")
			}
		})
	}
}

func TestAnomalyExplanationDataSource_ExplanationMapping(t *testing.T) {
	t.Parallel()

	responseJSON := `{
		"facts": {
			"serviceName": "BigQuery",
			"platform": "google-cloud",
			"scope": "project-123",
			"severityLevel": "warning",
			"costOfAnomaly": 42.0,
			"top3SKUs": []
		},
		"evidence": [],
		"explanation": {"text": "The cost spike is likely due to a query regression.", "generatedBy": "ava", "aiGenerated": true}
	}`

	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, responseJSON)
	}))
	defer server.Close()

	overrides := map[string]tftypes.Value{
		"id": tftypes.NewValue(tftypes.String, "test-anomaly-id"),
	}

	data, _, diags := readAnomalyExplanationHelper(t, server, overrides)
	if diags.HasError() {
		t.Fatalf("Read() returned unexpected diagnostics: %v", diags)
	}

	if data.Explanation.IsNull() {
		t.Fatal("expected Explanation to be populated, got null")
	}
	if !data.Explanation.AiGenerated.ValueBool() {
		t.Errorf("expected AiGenerated to be true, got %v", data.Explanation.AiGenerated)
	}
	if got := data.Explanation.GeneratedBy.ValueString(); got != "ava" {
		t.Errorf("GeneratedBy = %q, want %q", got, "ava")
	}
	if got := data.Explanation.Text.ValueString(); got != "The cost spike is likely due to a query regression." {
		t.Errorf("Text = %q, want the expected explanation text", got)
	}
}

func TestAnomalyExplanationDataSource_EvidenceList(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		responseJSON  string
		wantCount     int
		wantFirstType string
		wantFirstID   string
	}{
		{
			name: "populated evidence",
			responseJSON: `{
				"facts": {"serviceName": "BigQuery", "platform": "google-cloud", "scope": "project-123", "severityLevel": "warning", "costOfAnomaly": 42.0, "top3SKUs": []},
				"evidence": [{"type": "anomaly", "id": "ev-1"}, {"type": "budget", "id": "ev-2"}],
				"explanation": {"text": "spike", "generatedBy": "ava", "aiGenerated": true}
			}`,
			wantCount:     2,
			wantFirstType: "anomaly",
			wantFirstID:   "ev-1",
		},
		{
			name: "empty evidence",
			responseJSON: `{
				"facts": {"serviceName": "BigQuery", "platform": "google-cloud", "scope": "project-123", "severityLevel": "warning", "costOfAnomaly": 42.0, "top3SKUs": []},
				"evidence": [],
				"explanation": {"text": "spike", "generatedBy": "ava", "aiGenerated": true}
			}`,
			wantCount: 0,
		},
		{
			name: "omitted evidence",
			responseJSON: `{
				"facts": {"serviceName": "BigQuery", "platform": "google-cloud", "scope": "project-123", "severityLevel": "warning", "costOfAnomaly": 42.0, "top3SKUs": []},
				"explanation": {"text": "spike", "generatedBy": "ava", "aiGenerated": true}
			}`,
			wantCount: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprint(w, tc.responseJSON)
			}))
			defer server.Close()

			overrides := map[string]tftypes.Value{
				"id": tftypes.NewValue(tftypes.String, "test-anomaly-id"),
			}

			data, _, diags := readAnomalyExplanationHelper(t, server, overrides)
			if diags.HasError() {
				t.Fatalf("Read() returned unexpected diagnostics: %v", diags)
			}

			if data.Evidence.IsNull() {
				t.Fatal("expected Evidence not to be null (computed list convention)")
			}
			if data.Evidence.IsUnknown() {
				t.Fatal("expected Evidence not to be unknown")
			}

			var gotElements []datasource_anomaly_explanation.EvidenceValue
			d := data.Evidence.ElementsAs(context.Background(), &gotElements, false)
			if d.HasError() {
				t.Fatalf("ElementsAs() returned diagnostics: %v", d)
			}

			if len(gotElements) != tc.wantCount {
				t.Errorf("len(Evidence) = %d, want %d", len(gotElements), tc.wantCount)
			}

			if tc.wantCount > 0 {
				if got := gotElements[0].EvidenceType.ValueString(); got != tc.wantFirstType {
					t.Errorf("Evidence[0].type = %q, want %q", got, tc.wantFirstType)
				}
				if got := gotElements[0].Id.ValueString(); got != tc.wantFirstID {
					t.Errorf("Evidence[0].id = %q, want %q", got, tc.wantFirstID)
				}
			}
		})
	}
}

func TestAnomalyExplanationDataSource_NotFound(t *testing.T) {
	t.Parallel()

	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"error": "not found"}`)
	}))
	defer server.Close()

	overrides := map[string]tftypes.Value{
		"id": tftypes.NewValue(tftypes.String, "non-existent-id"),
	}

	_, _, diags := readAnomalyExplanationHelper(t, server, overrides)
	if !diags.HasError() {
		t.Fatal("expected error diagnostic for 404 response, got none")
	}

	found := false
	for _, d := range diags.Errors() {
		if d.Summary() == "Anomaly explanation not found" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Anomaly explanation not found' error diagnostic, got %v", diags)
	}
}
