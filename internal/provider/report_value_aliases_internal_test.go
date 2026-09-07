package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestReportQueryValueAliases(t *testing.T) {
	for _, tc := range []struct {
		name    string
		details map[string]any
	}{
		{name: "absent"},
		{name: "empty", details: map[string]any{"valueAliases": map[string]any{}}},
		{name: "populated", details: map[string]any{"valueAliases": map[string]any{
			"fixed:cloud_provider": map[string]any{"datadog_prod": "Datadog (prod)"},
		}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			result := map[string]any{
				"schema":   []any{map[string]any{"name": "cloud_provider", "type": "string"}},
				"rows":     []any{[]any{"datadog_prod"}},
				"cacheHit": false,
			}
			if tc.details != nil {
				result["details"] = tc.details
			}
			server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/analytics/v1/reports/query" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(map[string]any{"result": result}); err != nil {
					t.Error(err)
				}
			}))
			client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(server.Client()))
			if err != nil {
				t.Fatal(err)
			}
			ds := &reportQueryDataSource{client: client}
			var schemaResp datasource.SchemaResponse
			ds.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
			if schemaResp.Diagnostics.HasError() {
				t.Fatal(schemaResp.Diagnostics)
			}
			attrTypes := map[string]tftypes.Type{}
			values := map[string]tftypes.Value{}
			for name, attr := range schemaResp.Schema.Attributes {
				tfType := attr.GetType().TerraformType(ctx)
				attrTypes[name] = tfType
				values[name] = tftypes.NewValue(tfType, nil)
			}
			configType := attrTypes["config"].(tftypes.Object)
			configValues := map[string]tftypes.Value{}
			for name, tfType := range configType.AttributeTypes {
				configValues[name] = tftypes.NewValue(tfType, nil)
			}
			configValues["layout"] = tftypes.NewValue(tftypes.String, "table")
			values["config"] = tftypes.NewValue(configType, configValues)
			var resp datasource.ReadResponse
			resp.State = tfsdk.State{Schema: schemaResp.Schema}
			ds.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{
				Schema: schemaResp.Schema,
				Raw:    tftypes.NewValue(tftypes.Object{AttributeTypes: attrTypes}, values),
			}}, &resp)
			if resp.Diagnostics.HasError() {
				t.Fatal(resp.Diagnostics)
			}
			var resultJSON types.String
			if diags := resp.State.GetAttribute(ctx, path.Root("result_json"), &resultJSON); diags.HasError() {
				t.Fatal(diags)
			}
			var actual map[string]any
			if err := json.Unmarshal([]byte(resultJSON.ValueString()), &actual); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(actual, result) {
				t.Fatalf("result_json = %#v, want %#v", actual, result)
			}
		})
	}
}
