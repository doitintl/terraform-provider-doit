package provider

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_cloudconnect_supported_features"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestCloudconnectSupportedFeaturesRead(t *testing.T) {
	for _, tc := range []struct {
		name          string
		accountID     string
		body          string
		count         int
		omittedFields bool
	}{
		{name: "aws permissions", accountID: "123456789012", body: `{"supportedFeatures":[{"name":"sandbox","hasRequiredPermissions":true},{"name":"recommendations","hasRequiredPermissions":false}]}`, count: 2},
		{name: "empty features", accountID: "123456789012", body: `{"supportedFeatures":[]}`},
		{name: "omitted features", accountID: "123456789012", body: `{}`},
		{name: "null features", accountID: "123456789012", body: `{"supportedFeatures":null}`},
		{name: "omitted feature fields", accountID: "123456789012", body: `{"supportedFeatures":[{}]}`, count: 1, omittedFields: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodGet || r.URL.Path != "/core/v1/cloudconnect/supportedFeatures/"+tc.accountID || r.URL.RawQuery != "" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				w.Header().Set("Content-Type", "application/json")
				_, err := w.Write([]byte(tc.body))
				if err != nil {
					t.Error(err)
				}
			}))
			resp := readCloudconnectSupportedFeatures(t, server, types.StringValue(tc.accountID))
			if resp.Diagnostics.HasError() {
				t.Fatal(resp.Diagnostics)
			}
			if calls != 1 {
				t.Fatalf("got %d calls, want 1", calls)
			}
			var state cloudconnectSupportedFeaturesDataSourceModel
			if diags := resp.State.Get(t.Context(), &state); diags.HasError() {
				t.Fatal(diags)
			}
			if state.AccountId.ValueString() != tc.accountID {
				t.Fatalf("account_id = %s", state.AccountId)
			}
			if state.SupportedFeatures.IsNull() || state.SupportedFeatures.IsUnknown() || len(state.SupportedFeatures.Elements()) != tc.count {
				t.Fatalf("unexpected features: %s", state.SupportedFeatures)
			}
			var features []datasource_cloudconnect_supported_features.SupportedFeaturesValue
			if diags := state.SupportedFeatures.ElementsAs(t.Context(), &features, false); diags.HasError() {
				t.Fatal(diags)
			}
			if tc.omittedFields {
				if !features[0].Name.IsNull() || !features[0].HasRequiredPermissions.IsNull() {
					t.Fatalf("omitted fields must stay null: %v", features[0])
				}
			} else if tc.count == 2 {
				if !features[0].Name.Equal(types.StringValue("sandbox")) || !features[0].HasRequiredPermissions.Equal(types.BoolValue(true)) || !features[1].Name.Equal(types.StringValue("recommendations")) || !features[1].HasRequiredPermissions.Equal(types.BoolValue(false)) {
					t.Fatalf("permissions were not preserved: %s", state.SupportedFeatures)
				}
			}
		})
	}
}

func TestCloudconnectSupportedFeaturesReadErrors(t *testing.T) {
	for _, tc := range []struct {
		name        string
		status      int
		body        string
		contentType string
		want        string
	}{
		{"not found", 404, `{"error":"account not found"}`, "application/json", "status: 404, body: {\"error\":\"account not found\"}"},
		{"forbidden", 403, `{"error":"access denied"}`, "application/json", "status: 403, body: {\"error\":\"access denied\"}"},
		{"server error", 500, `{"error":"unavailable"}`, "application/json", "status: 500, body: {\"error\":\"unavailable\"}"},
		{"non JSON success", 200, "unexpected response", "text/plain", "status: 200, body: unexpected response"},
		{"invalid JSON", 200, "{", "application/json", "unexpected end of JSON input"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				w.WriteHeader(tc.status)
				if _, err := w.Write([]byte(tc.body)); err != nil {
					t.Error(err)
				}
			}))
			resp := readCloudconnectSupportedFeatures(t, server, types.StringValue("123456789012"))
			if !resp.Diagnostics.HasError() || !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), tc.want) {
				t.Fatalf("got %v, want error containing %q", resp.Diagnostics, tc.want)
			}
			if !resp.State.Raw.IsNull() {
				t.Fatal("failed read must not write state")
			}
		})
	}
}

func TestCloudconnectSupportedFeaturesUnknownAccount(t *testing.T) {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { t.Error("unknown account must not make an API request") }))
	resp := readCloudconnectSupportedFeatures(t, server, types.StringUnknown())
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	var state cloudconnectSupportedFeaturesDataSourceModel
	if diags := resp.State.Get(t.Context(), &state); diags.HasError() {
		t.Fatal(diags)
	}
	if !state.AccountId.IsUnknown() || !state.SupportedFeatures.IsUnknown() {
		t.Fatalf("unknown account must preserve unknown state: %+v", state)
	}
}

func readCloudconnectSupportedFeatures(t *testing.T, server *httptest.Server, accountID types.String) datasource.ReadResponse {
	t.Helper()
	ctx := t.Context()
	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	ds := NewCloudconnectSupportedFeaturesDataSource()
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
		values[name] = tftypes.NewValue(tfType, nil)
	}
	accountValue, err := accountID.ToTerraformValue(ctx)
	if err != nil {
		t.Fatal(err)
	}
	values["account_id"] = accountValue
	resp := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	ds.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(tftypes.Object{AttributeTypes: attrTypes}, values)}}, &resp)
	return resp
}
