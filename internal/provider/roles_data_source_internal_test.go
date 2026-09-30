package provider

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_roles"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestRolesDataSourceRead(t *testing.T) {
	for _, tc := range []struct {
		name           string
		body           string
		wantRowCount   int64
		wantRolesCount int
	}{
		{
			name: "canonical roles with rowCount",
			body: `{
				"rowCount": 2,
				"roles": [
					{
						"id": "role-admin",
						"name": "Admin",
						"type": "preset",
						"description": "Administrator role with full access",
						"customer": "",
						"childTenantEligible": true,
						"permissions": ["iam.roles.read", "iam.roles.write"]
					},
					{
						"id": "role-viewer",
						"name": "Viewer",
						"type": "custom",
						"description": "",
						"customer": "cust-12345",
						"childTenantEligible": false,
						"permissions": []
					}
				]
			}`,
			wantRowCount:   2,
			wantRolesCount: 2,
		},
		{
			name: "fallback rowCount when omitted by API",
			body: `{
				"roles": [
					{
						"id": "role-editor",
						"name": "Editor",
						"type": "custom",
						"description": "Custom editor role",
						"customer": "cust-99999",
						"childTenantEligible": false,
						"permissions": ["iam.roles.write"]
					}
				]
			}`,
			wantRowCount:   1,
			wantRolesCount: 1,
		},
		{
			name:           "empty roles list",
			body:           `{"roles": []}`,
			wantRowCount:   0,
			wantRolesCount: 0,
		},
		{
			name:           "null roles with omitted rowCount",
			body:           `{}`,
			wantRowCount:   0,
			wantRolesCount: 0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodGet || r.URL.Path != "/iam/v1/roles" || r.URL.RawQuery != "" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				w.Header().Set("Content-Type", "application/json")
				_, err := w.Write([]byte(tc.body))
				if err != nil {
					t.Error(err)
				}
			}))

			resp := readRoles(t, server)
			if resp.Diagnostics.HasError() {
				t.Fatal(resp.Diagnostics)
			}
			if calls != 1 {
				t.Fatalf("got %d calls, want 1", calls)
			}

			var state rolesDataSourceModel
			if diags := resp.State.Get(t.Context(), &state); diags.HasError() {
				t.Fatal(diags)
			}

			if state.RowCount.IsNull() || state.RowCount.IsUnknown() || state.RowCount.ValueInt64() != tc.wantRowCount {
				t.Fatalf("unexpected row_count: got %s, want %d", state.RowCount, tc.wantRowCount)
			}

			if state.Roles.IsNull() || state.Roles.IsUnknown() || len(state.Roles.Elements()) != tc.wantRolesCount {
				t.Fatalf("unexpected roles list: %s", state.Roles)
			}

			var roles []datasource_roles.RolesValue
			if diags := state.Roles.ElementsAs(t.Context(), &roles, false); diags.HasError() {
				t.Fatal(diags)
			}

			if tc.name == "canonical roles with rowCount" {
				// Role 0: preset role
				r0 := roles[0]
				if !r0.Id.Equal(types.StringValue("role-admin")) {
					t.Errorf("roles[0].Id = %v, want role-admin", r0.Id)
				}
				if !r0.Name.Equal(types.StringValue("Admin")) {
					t.Errorf("roles[0].Name = %v, want Admin", r0.Name)
				}
				if !r0.RolesType.Equal(types.StringValue("preset")) {
					t.Errorf("roles[0].Type = %v, want preset", r0.RolesType)
				}
				if !r0.Description.Equal(types.StringValue("Administrator role with full access")) {
					t.Errorf("roles[0].Description = %v, want Administrator role with full access", r0.Description)
				}
				if !r0.Customer.Equal(types.StringValue("")) {
					t.Errorf("roles[0].Customer = %v, want empty string", r0.Customer)
				}
				if !r0.ChildTenantEligible.Equal(types.BoolValue(true)) {
					t.Errorf("roles[0].ChildTenantEligible = %v, want true", r0.ChildTenantEligible)
				}
				if r0.Permissions.IsNull() || len(r0.Permissions.Elements()) != 2 {
					t.Errorf("roles[0].Permissions = %v, want 2 elements", r0.Permissions)
				}

				// Role 1: custom role
				r1 := roles[1]
				if !r1.Id.Equal(types.StringValue("role-viewer")) {
					t.Errorf("roles[1].Id = %v, want role-viewer", r1.Id)
				}
				if !r1.Name.Equal(types.StringValue("Viewer")) {
					t.Errorf("roles[1].Name = %v, want Viewer", r1.Name)
				}
				if !r1.RolesType.Equal(types.StringValue("custom")) {
					t.Errorf("roles[1].Type = %v, want custom", r1.RolesType)
				}
				if !r1.Description.Equal(types.StringValue("")) {
					t.Errorf("roles[1].Description = %v, want empty string", r1.Description)
				}
				if !r1.Customer.Equal(types.StringValue("cust-12345")) {
					t.Errorf("roles[1].Customer = %v, want cust-12345", r1.Customer)
				}
				if !r1.ChildTenantEligible.Equal(types.BoolValue(false)) {
					t.Errorf("roles[1].ChildTenantEligible = %v, want false", r1.ChildTenantEligible)
				}
				if r1.Permissions.IsNull() || len(r1.Permissions.Elements()) != 0 {
					t.Errorf("roles[1].Permissions = %v, want empty list", r1.Permissions)
				}
			}
		})
	}
}

func TestRolesDataSourceReadErrors(t *testing.T) {
	for _, tc := range []struct {
		name        string
		status      int
		body        string
		contentType string
		want        string
	}{
		{"forbidden", 403, `{"error":"access denied"}`, "application/json", "API returned status 403: {\"error\":\"access denied\"}"},
		{"server error", 500, `{"error":"unavailable"}`, "application/json", "API returned status 500: {\"error\":\"unavailable\"}"},
		{"non JSON success", 200, "unexpected response", "text/plain", "API returned status 200: unexpected response"},
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

			resp := readRoles(t, server)
			if !resp.Diagnostics.HasError() || !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), tc.want) {
				t.Fatalf("got %v, want error containing %q", resp.Diagnostics, tc.want)
			}
			if !resp.State.Raw.IsNull() {
				t.Fatal("failed read must not write state")
			}
		})
	}
}

func readRoles(t *testing.T, server *httptest.Server) datasource.ReadResponse {
	t.Helper()
	ctx := t.Context()
	httpClient := server.Client()
	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}

	ds := NewRolesDataSource()
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
