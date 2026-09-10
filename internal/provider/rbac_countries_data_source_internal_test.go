package provider

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_rbac_countries"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestRbacCountriesRead(t *testing.T) {
	for _, tc := range []struct {
		name  string
		body  string
		count int
	}{
		{
			name:  "canonical countries",
			body:  `{"countries":[{"countryCode":"GB","name":"United Kingdom"},{"countryCode":"US","name":"United States"}]}`,
			count: 2,
		},
		{
			name:  "empty countries",
			body:  `{"countries":[]}`,
			count: 0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodGet || r.URL.Path != "/rbac/v1/countries" || r.URL.RawQuery != "" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				w.Header().Set("Content-Type", "application/json")
				_, err := w.Write([]byte(tc.body))
				if err != nil {
					t.Error(err)
				}
			}))

			resp := readRbacCountries(t, server)
			if resp.Diagnostics.HasError() {
				t.Fatal(resp.Diagnostics)
			}
			if calls != 1 {
				t.Fatalf("got %d calls, want 1", calls)
			}

			var state rbacCountriesDataSourceModel
			if diags := resp.State.Get(t.Context(), &state); diags.HasError() {
				t.Fatal(diags)
			}

			if state.Countries.IsNull() || state.Countries.IsUnknown() || len(state.Countries.Elements()) != tc.count {
				t.Fatalf("unexpected countries: %s", state.Countries)
			}

			var countries []datasource_rbac_countries.CountriesValue
			if diags := state.Countries.ElementsAs(t.Context(), &countries, false); diags.HasError() {
				t.Fatal(diags)
			}

			if tc.count == 2 {
				if !countries[0].CountryCode.Equal(types.StringValue("GB")) || !countries[0].Name.Equal(types.StringValue("United Kingdom")) {
					t.Fatalf("unexpected country 0: %+v", countries[0])
				}
				if !countries[1].CountryCode.Equal(types.StringValue("US")) || !countries[1].Name.Equal(types.StringValue("United States")) {
					t.Fatalf("unexpected country 1: %+v", countries[1])
				}
			}
		})
	}
}

func TestRbacCountriesReadErrors(t *testing.T) {
	for _, tc := range []struct {
		name        string
		status      int
		body        string
		contentType string
		want        string
	}{
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

			resp := readRbacCountries(t, server)
			if !resp.Diagnostics.HasError() || !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), tc.want) {
				t.Fatalf("got %v, want error containing %q", resp.Diagnostics, tc.want)
			}
			if !resp.State.Raw.IsNull() {
				t.Fatal("failed read must not write state")
			}
		})
	}
}

func readRbacCountries(t *testing.T, server *httptest.Server) datasource.ReadResponse {
	t.Helper()
	ctx := t.Context()
	httpClient := server.Client()
	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}

	ds := NewRbacCountriesDataSource()
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

	resp := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	ds.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(tftypes.Object{AttributeTypes: attrTypes}, values)}}, &resp)
	return resp
}
