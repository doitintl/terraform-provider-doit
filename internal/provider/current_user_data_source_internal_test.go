package provider

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestCurrentUserRead(t *testing.T) {
	for _, tc := range []struct {
		name            string
		body            string
		wantDomain      string
		wantEmail       string
		wantPermissions []string
	}{
		{
			name:            "permissions populated",
			body:            `{"domain":"example.com","email":"user@example.com","permissions":["CloudAnalyticsReadOnly","BillingViewer"]}`,
			wantDomain:      "example.com",
			wantEmail:       "user@example.com",
			wantPermissions: []string{"CloudAnalyticsReadOnly", "BillingViewer"},
		},
		{
			name:            "empty permissions list",
			body:            `{"domain":"example.com","email":"user@example.com","permissions":[]}`,
			wantDomain:      "example.com",
			wantEmail:       "user@example.com",
			wantPermissions: []string{},
		},
		{
			name:            "omitted permissions (DoiT employee token)",
			body:            `{"domain":"doit.com","email":"admin@doit.com"}`,
			wantDomain:      "doit.com",
			wantEmail:       "admin@doit.com",
			wantPermissions: []string{},
		},
		{
			name:            "null permissions",
			body:            `{"domain":"example.com","email":"user@example.com","permissions":null}`,
			wantDomain:      "example.com",
			wantEmail:       "user@example.com",
			wantPermissions: []string{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodGet || r.URL.Path != "/auth/v1/validate" || r.URL.RawQuery != "" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				w.Header().Set("Content-Type", "application/json")
				_, err := w.Write([]byte(tc.body))
				if err != nil {
					t.Error(err)
				}
			}))

			resp := readCurrentUser(t, server)
			if resp.Diagnostics.HasError() {
				t.Fatal(resp.Diagnostics)
			}
			if calls != 1 {
				t.Fatalf("got %d calls, want 1", calls)
			}

			var state currentUserDataSourceModel
			if diags := resp.State.Get(t.Context(), &state); diags.HasError() {
				t.Fatal(diags)
			}

			if !state.Domain.Equal(types.StringValue(tc.wantDomain)) {
				t.Errorf("domain = %s, want %s", state.Domain, tc.wantDomain)
			}
			if !state.Email.Equal(types.StringValue(tc.wantEmail)) {
				t.Errorf("email = %s, want %s", state.Email, tc.wantEmail)
			}

			if state.Permissions.IsNull() || state.Permissions.IsUnknown() {
				t.Fatalf("permissions list must not be null or unknown, got: %s", state.Permissions)
			}

			var gotPermissions []string
			if diags := state.Permissions.ElementsAs(t.Context(), &gotPermissions, false); diags.HasError() {
				t.Fatal(diags)
			}

			if !reflect.DeepEqual(gotPermissions, tc.wantPermissions) {
				t.Errorf("permissions = %v, want %v", gotPermissions, tc.wantPermissions)
			}
		})
	}
}

func TestCurrentUserReadErrors(t *testing.T) {
	for _, tc := range []struct {
		name        string
		status      int
		body        string
		contentType string
		want        string
	}{
		{"unauthorized", 401, `{"error":"invalid token"}`, "application/json", "API returned status 401"},
		{"server error", 500, `{"error":"internal error"}`, "application/json", "API returned status 500"},
		{"non JSON success", 200, "unexpected response", "text/plain", "API returned status 200"},
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

			resp := readCurrentUser(t, server)
			if !resp.Diagnostics.HasError() || !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), tc.want) {
				t.Fatalf("got %v, want error containing %q", resp.Diagnostics, tc.want)
			}
			if !resp.State.Raw.IsNull() {
				t.Fatal("failed read must not write state")
			}
		})
	}
}

func readCurrentUser(t *testing.T, server *httptest.Server) datasource.ReadResponse {
	t.Helper()
	ctx := t.Context()
	httpClient := server.Client()
	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}

	ds := NewCurrentUserDataSource()
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
