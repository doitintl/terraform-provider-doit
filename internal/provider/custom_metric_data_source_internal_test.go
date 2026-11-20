package provider

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_custom_metric"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// readCustomMetricHelper builds a customMetricDataSource backed by a client
// pointed at server, runs Read with the given config overrides (every other
// attribute is null) and returns the state and diagnostics.
func readCustomMetricHelper(t *testing.T, server *httptest.Server, overrides map[string]tftypes.Value) (customMetricDataSourceModel, diag.Diagnostics) {
	t.Helper()

	// NewTestServer starts on the first Client() call and sets server.URL then,
	// so the client is taken before the URL is read. The retry client is the
	// provider's own: it turns a 403 into an error, as in production.
	httpClient := server.Client()
	client, err := newClientWithHTTPClient(server.URL, "test-token", "", "1.9.0", "dev", httpClient)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ds := &customMetricDataSource{client: client}
	ctx := t.Context()

	schemaResp := &datasource.SchemaResponse{}
	ds.Schema(ctx, datasource.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("failed to get schema: %v", schemaResp.Diagnostics)
	}

	configValues := map[string]tftypes.Value{}
	attrTypes := map[string]tftypes.Type{}
	for name, a := range schemaResp.Schema.Attributes {
		tfType := a.GetType().TerraformType(ctx)
		attrTypes[name] = tfType
		if override, ok := overrides[name]; ok {
			configValues[name] = override
			continue
		}
		configValues[name] = tftypes.NewValue(tfType, nil)
	}

	config := tfsdk.Config{
		Schema: schemaResp.Schema,
		Raw:    tftypes.NewValue(tftypes.Object{AttributeTypes: attrTypes}, configValues),
	}
	readResp := &datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	ds.Read(ctx, datasource.ReadRequest{Config: config}, readResp)
	if readResp.Diagnostics.HasError() {
		return customMetricDataSourceModel{}, readResp.Diagnostics
	}

	var data customMetricDataSourceModel
	if diags := readResp.State.Get(ctx, &data); diags.HasError() {
		t.Fatalf("failed to read state: %v", diags)
	}
	return data, readResp.Diagnostics
}

func customMetricServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	return httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

func TestCustomMetricDataSource_Read(t *testing.T) {
	t.Parallel()

	server := customMetricServer(t, http.StatusOK, cmItemJSON("m1", "one"))
	data, diags := readCustomMetricHelper(t, server, map[string]tftypes.Value{"id": cmString("m1")})
	if diags.HasError() {
		t.Fatalf("Read() error: %v", diags)
	}

	if data.Id.ValueString() != "m1" || data.Name.ValueString() != "one" {
		t.Errorf("id/name = %q/%q", data.Id.ValueString(), data.Name.ValueString())
	}
	if data.Description.ValueString() != "d-m1" || data.Formula.ValueString() != "A / B" || data.UnitType.ValueString() != "number" {
		t.Errorf("description/formula/unit_type = %q/%q/%q", data.Description.ValueString(), data.Formula.ValueString(), data.UnitType.ValueString())
	}
	if data.Owner.ValueString() != "owner@example.com" || data.Etag.IsNull() {
		t.Errorf("owner = %v, etag = %v", data.Owner, data.Etag)
	}
	if data.CreateTime.ValueString() != "2026-10-12T09:30:00Z" || data.UpdateTime.ValueString() != "2026-10-13T10:00:00.5Z" {
		t.Errorf("create/update time = %q/%q", data.CreateTime.ValueString(), data.UpdateTime.ValueString())
	}
	if data.Decimals.IsNull() {
		t.Fatal("decimals is null, want {min=0, max=2}")
	}
	if data.Decimals.Min.ValueInt64() != 0 || data.Decimals.Max.ValueInt64() != 2 {
		t.Errorf("decimals = %v", data.Decimals)
	}
	var variables []datasource_custom_metric.VariablesValue
	if d := data.Variables.ElementsAs(t.Context(), &variables, false); d.HasError() {
		t.Fatalf("variables: %v", d)
	}
	if len(variables) != 2 {
		t.Fatalf("variables = %d, want 2", len(variables))
	}
	if v := variables[1]; v.Letter.ValueString() != "B" || v.AllocationId.ValueString() != "alloc-b" || v.Metric.MetricType.ValueString() != "extended" || v.Metric.Value.ValueString() != "amortized_cost" {
		t.Errorf("second variable = %v", v)
	}
}

// TestCustomMetricDataSource_NullableFields covers a metric without display
// precision, owner or timestamps.
func TestCustomMetricDataSource_NullableFields(t *testing.T) {
	t.Parallel()

	server := customMetricServer(t, http.StatusOK, `{
  "id": "m2", "etag": "e", "name": "two", "description": "", "formula": "A", "unitType": "currency", "decimals": null,
  "variables": [{"letter": "A", "metric": {"type": "basic", "value": "cost"}, "allocationId": "alloc-a"}]
}`)
	data, diags := readCustomMetricHelper(t, server, map[string]tftypes.Value{"id": cmString("m2")})
	if diags.HasError() {
		t.Fatalf("Read() error: %v", diags)
	}
	if !data.Decimals.IsNull() {
		t.Errorf("decimals = %v, want null", data.Decimals)
	}
	if !data.Owner.IsNull() || !data.CreateTime.IsNull() || !data.UpdateTime.IsNull() {
		t.Errorf("owner/create/update = %v/%v/%v, want null", data.Owner, data.CreateTime, data.UpdateTime)
	}
	if data.Variables.IsNull() || len(data.Variables.Elements()) != 1 {
		t.Errorf("variables = %v", data.Variables)
	}
}

func TestCustomMetricDataSource_NotFound(t *testing.T) {
	t.Parallel()

	server := customMetricServer(t, http.StatusNotFound, `{"error": "not found"}`)
	_, diags := readCustomMetricHelper(t, server, map[string]tftypes.Value{"id": cmString("preset")})
	if !diags.HasError() {
		t.Fatal("Read() returned no error for a 404")
	}
	if got := diags.Errors()[0].Detail(); !strings.Contains(got, "preset") || !strings.Contains(got, "404") {
		t.Errorf("error detail = %q, want it to name the ID and status", got)
	}
}

// TestCustomMetricDataSource_NotEntitled reads without the calculated metrics
// entitlement. The retry client returns a 403 as an error, so the message must
// carry the status and body.
func TestCustomMetricDataSource_NotEntitled(t *testing.T) {
	t.Parallel()

	server := customMetricServer(t, http.StatusForbidden, `{"code": "analytics.feature_not_entitled"}`)
	_, diags := readCustomMetricHelper(t, server, map[string]tftypes.Value{"id": cmString("m1")})
	if !diags.HasError() {
		t.Fatal("Read() returned no error for a 403")
	}
	if got := diags.Errors()[0].Detail(); !strings.Contains(got, "403") || !strings.Contains(got, "analytics.feature_not_entitled") {
		t.Errorf("error detail = %q, want the status and body", got)
	}
}

func TestCustomMetricDataSource_IDMismatch(t *testing.T) {
	t.Parallel()

	server := customMetricServer(t, http.StatusOK, cmItemJSON("other", "one"))
	_, diags := readCustomMetricHelper(t, server, map[string]tftypes.Value{"id": cmString("m1")})
	if !diags.HasError() {
		t.Fatal("Read() returned no error for a response with another ID")
	}
}

func TestCustomMetricDataSource_UnparsableBody(t *testing.T) {
	t.Parallel()

	server := customMetricServer(t, http.StatusOK, `not json`)
	_, diags := readCustomMetricHelper(t, server, map[string]tftypes.Value{"id": cmString("m1")})
	if !diags.HasError() {
		t.Fatal("Read() returned no error for an unparsable body")
	}
}

// TestCustomMetricDataSource_UnknownConfigDefensiveFallback exercises the
// protocol-level fallback directly: an unknown id makes no API call and leaves
// every result unknown.
func TestCustomMetricDataSource_UnknownConfigDefensiveFallback(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))

	data, diags := readCustomMetricHelper(t, server, map[string]tftypes.Value{
		"id": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
	})
	if diags.HasError() {
		t.Fatalf("Read() error: %v", diags)
	}
	if calls.Load() != 0 {
		t.Errorf("made %d API calls with an unknown id", calls.Load())
	}
	if !data.Id.IsUnknown() {
		t.Errorf("id = %v, want unknown", data.Id)
	}
	for name, unknown := range map[string]bool{
		"etag":        data.Etag.IsUnknown(),
		"name":        data.Name.IsUnknown(),
		"description": data.Description.IsUnknown(),
		"owner":       data.Owner.IsUnknown(),
		"formula":     data.Formula.IsUnknown(),
		"unit_type":   data.UnitType.IsUnknown(),
		"create_time": data.CreateTime.IsUnknown(),
		"update_time": data.UpdateTime.IsUnknown(),
		"decimals":    data.Decimals.IsUnknown(),
		"variables":   data.Variables.IsUnknown(),
	} {
		if !unknown {
			t.Errorf("%s is not unknown", name)
		}
	}
}
