package provider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// readAnomalyHelper builds an anomalyDataSource backed by a client pointed at
// server, invokes Read with the given config overrides, and returns the resulting state and diagnostics.
func readAnomalyHelper(t *testing.T, server *httptest.Server, overrides map[string]tftypes.Value) (anomalyDataSourceModel, tfsdk.State, diag.Diagnostics) {
	t.Helper()

	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ds := &anomalyDataSource{client: client}
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

	var data anomalyDataSourceModel
	if !readResp.Diagnostics.HasError() {
		if diags := readResp.State.Get(ctx, &data); diags.HasError() {
			t.Fatalf("failed to read state: %v", diags)
		}
	}

	return data, readResp.State, readResp.Diagnostics
}

func TestAnomalyDataSource_UnknownID(t *testing.T) {
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

	data, _, diags := readAnomalyHelper(t, server, overrides)
	if diags.HasError() {
		t.Fatalf("Read() returned unexpected diagnostics: %v", diags)
	}

	if reqs := requestCount.Load(); reqs != 0 {
		t.Errorf("expected 0 API requests when id is unknown, got %d", reqs)
	}

	if !data.EntityLabel.IsUnknown() {
		t.Errorf("expected EntityLabel to be unknown, got %v", data.EntityLabel)
	}
	if !data.EntityName.IsUnknown() {
		t.Errorf("expected EntityName to be unknown, got %v", data.EntityName)
	}
	if !data.ProviderDisplayName.IsUnknown() {
		t.Errorf("expected ProviderDisplayName to be unknown, got %v", data.ProviderDisplayName)
	}
	if !data.Platform.IsUnknown() {
		t.Errorf("expected Platform to be unknown, got %v", data.Platform)
	}
	if !data.ServiceName.IsUnknown() {
		t.Errorf("expected ServiceName to be unknown, got %v", data.ServiceName)
	}
	if !data.LinkedAnomalies.IsUnknown() {
		t.Errorf("expected LinkedAnomalies to be unknown, got %v", data.LinkedAnomalies)
	}
}

func TestAnomalyDataSource_EntityFieldsMapping(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name                string
		responseJSON        string
		wantEntityLabel     *string
		wantEntityName      *string
		wantProviderDisplay *string
	}{
		{
			name: "populated entity fields",
			responseJSON: `{
				"platform": "google-cloud",
				"scope": "user_456",
				"entityLabel": "User",
				"entityName": "bob@example.com",
				"providerDisplayName": "Google Cloud Platform",
				"serviceName": "BigQuery",
				"costOfAnomaly": 42.0,
				"startTime": 1704067200000,
				"severityLevel": "warning",
				"monitorLevel": "service",
				"timeFrame": "day"
			}`,
			wantEntityLabel:     new("User"),
			wantEntityName:      new("bob@example.com"),
			wantProviderDisplay: new("Google Cloud Platform"),
		},
		{
			name: "omitted entity fields",
			responseJSON: `{
				"platform": "amazon-web-services",
				"scope": "123456789012",
				"serviceName": "AmazonEC2",
				"costOfAnomaly": 10.0,
				"startTime": 1704067200000,
				"severityLevel": "critical",
				"monitorLevel": "service",
				"timeFrame": "day"
			}`,
			wantEntityLabel:     nil,
			wantEntityName:      nil,
			wantProviderDisplay: nil,
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

			data, _, diags := readAnomalyHelper(t, server, overrides)
			if diags.HasError() {
				t.Fatalf("Read() returned unexpected diagnostics: %v", diags)
			}

			if tc.wantEntityLabel != nil {
				if got := data.EntityLabel.ValueString(); got != *tc.wantEntityLabel {
					t.Errorf("EntityLabel = %q, want %q", got, *tc.wantEntityLabel)
				}
			} else if !data.EntityLabel.IsNull() {
				t.Errorf("expected EntityLabel to be null, got %v", data.EntityLabel)
			}

			if tc.wantEntityName != nil {
				if got := data.EntityName.ValueString(); got != *tc.wantEntityName {
					t.Errorf("EntityName = %q, want %q", got, *tc.wantEntityName)
				}
			} else if !data.EntityName.IsNull() {
				t.Errorf("expected EntityName to be null, got %v", data.EntityName)
			}

			if tc.wantProviderDisplay != nil {
				if got := data.ProviderDisplayName.ValueString(); got != *tc.wantProviderDisplay {
					t.Errorf("ProviderDisplayName = %q, want %q", got, *tc.wantProviderDisplay)
				}
			} else if !data.ProviderDisplayName.IsNull() {
				t.Errorf("expected ProviderDisplayName to be null, got %v", data.ProviderDisplayName)
			}
		})
	}
}

func TestAnomalyDataSource_NotFound(t *testing.T) {
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

	_, _, diags := readAnomalyHelper(t, server, overrides)
	if !diags.HasError() {
		t.Fatal("expected error diagnostic for 404 response, got none")
	}

	found := false
	for _, d := range diags.Errors() {
		if d.Summary() == "Anomaly not found" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Anomaly not found' error diagnostic, got %v", diags)
	}
}

func TestAnomalyDataSource_LinkedAnomalies(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name               string
		responseJSON       string
		wantLinkedCount    int
		wantLinkedElements []string
	}{
		{
			name: "populated linked anomalies",
			responseJSON: `{
				"platform": "google-cloud",
				"scope": "project-123",
				"serviceName": "BigQuery",
				"costOfAnomaly": 42.0,
				"startTime": 1704067200000,
				"severityLevel": "warning",
				"monitorLevel": "service",
				"timeFrame": "day",
				"linkedAnomalies": ["anom-linked-1", "anom-linked-2"]
			}`,
			wantLinkedCount:    2,
			wantLinkedElements: []string{"anom-linked-1", "anom-linked-2"},
		},
		{
			name: "empty linked anomalies",
			responseJSON: `{
				"platform": "google-cloud",
				"scope": "project-123",
				"serviceName": "BigQuery",
				"costOfAnomaly": 42.0,
				"startTime": 1704067200000,
				"severityLevel": "warning",
				"monitorLevel": "service",
				"timeFrame": "day",
				"linkedAnomalies": []
			}`,
			wantLinkedCount:    0,
			wantLinkedElements: []string{},
		},
		{
			name: "omitted linked anomalies",
			responseJSON: `{
				"platform": "google-cloud",
				"scope": "project-123",
				"serviceName": "BigQuery",
				"costOfAnomaly": 42.0,
				"startTime": 1704067200000,
				"severityLevel": "warning",
				"monitorLevel": "service",
				"timeFrame": "day"
			}`,
			wantLinkedCount:    0,
			wantLinkedElements: []string{},
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

			data, _, diags := readAnomalyHelper(t, server, overrides)
			if diags.HasError() {
				t.Fatalf("Read() returned unexpected diagnostics: %v", diags)
			}

			if data.LinkedAnomalies.IsNull() {
				t.Fatal("expected LinkedAnomalies not to be null (computed list convention)")
			}
			if data.LinkedAnomalies.IsUnknown() {
				t.Fatal("expected LinkedAnomalies not to be unknown")
			}

			var gotElements []string
			d := data.LinkedAnomalies.ElementsAs(t.Context(), &gotElements, false)
			if d.HasError() {
				t.Fatalf("ElementsAs() returned diagnostics: %v", d)
			}

			if len(gotElements) != tc.wantLinkedCount {
				t.Errorf("len(LinkedAnomalies) = %d, want %d", len(gotElements), tc.wantLinkedCount)
			}

			for i, want := range tc.wantLinkedElements {
				if i < len(gotElements) && gotElements[i] != want {
					t.Errorf("LinkedAnomalies[%d] = %q, want %q", i, gotElements[i], want)
				}
			}
		})
	}
}
