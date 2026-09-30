package provider

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestRoleDataSourceUnknownID(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls.Add(1) }))
	id := tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	resp := readRoleDataSourceForTest(t, server, &id)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if calls.Load() != 0 {
		t.Fatalf("made %d API requests for unknown ID", calls.Load())
	}
	var data roleDataSourceModel
	if diags := resp.State.Get(t.Context(), &data); diags.HasError() {
		t.Fatal(diags)
	}
	if !data.Name.IsUnknown() ||
		!data.Description.IsUnknown() ||
		!data.Type.IsUnknown() ||
		!data.Customer.IsUnknown() ||
		!data.ChildTenantEligible.IsUnknown() ||
		!data.Permissions.IsUnknown() {
		t.Fatalf("outputs must remain unknown: %+v", data)
	}
}

func TestRoleDataSourceRead(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		roleID               string
		body                 string
		wantName             string
		wantDescription      string
		wantType             string
		wantCustomer         string
		wantChildEligible    bool
		wantPermissionsCount int
	}{
		{
			name:   "custom role with permissions",
			roleID: "role-custom-123",
			body: `{
				"id": "role-custom-123",
				"name": "Custom Editor",
				"type": "custom",
				"description": "Custom editor role",
				"customer": "cust-999",
				"childTenantEligible": true,
				"permissions": ["iam.roles.read", "iam.roles.write"]
			}`,
			wantName:             "Custom Editor",
			wantDescription:      "Custom editor role",
			wantType:             "custom",
			wantCustomer:         "cust-999",
			wantChildEligible:    true,
			wantPermissionsCount: 2,
		},
		{
			name:   "preset role",
			roleID: "role-preset-view",
			body: `{
				"id": "role-preset-view",
				"name": "View Only",
				"type": "preset",
				"description": "Read-only preset role",
				"customer": "",
				"childTenantEligible": false,
				"permissions": ["iam.roles.read"]
			}`,
			wantName:             "View Only",
			wantDescription:      "Read-only preset role",
			wantType:             "preset",
			wantCustomer:         "",
			wantChildEligible:    false,
			wantPermissionsCount: 1,
		},
		{
			name:   "role with empty permissions and description",
			roleID: "role-empty-perms",
			body: `{
				"id": "role-empty-perms",
				"name": "Minimal Role",
				"type": "custom",
				"description": "",
				"customer": "cust-456",
				"childTenantEligible": false,
				"permissions": []
			}`,
			wantName:             "Minimal Role",
			wantDescription:      "",
			wantType:             "custom",
			wantCustomer:         "cust-456",
			wantChildEligible:    false,
			wantPermissionsCount: 0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/iam/v1/roles/"+tc.roleID {
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(tc.body))
			}))

			id := tftypes.NewValue(tftypes.String, tc.roleID)
			resp := readRoleDataSourceForTest(t, server, &id)
			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
			}

			var data roleDataSourceModel
			if diags := resp.State.Get(t.Context(), &data); diags.HasError() {
				t.Fatal(diags)
			}

			if data.Id.ValueString() != tc.roleID {
				t.Errorf("Id = %q, want %q", data.Id.ValueString(), tc.roleID)
			}
			if data.Name.ValueString() != tc.wantName {
				t.Errorf("Name = %q, want %q", data.Name.ValueString(), tc.wantName)
			}
			if data.Description.ValueString() != tc.wantDescription {
				t.Errorf("Description = %q, want %q", data.Description.ValueString(), tc.wantDescription)
			}
			if data.Type.ValueString() != tc.wantType {
				t.Errorf("Type = %q, want %q", data.Type.ValueString(), tc.wantType)
			}
			if data.Customer.ValueString() != tc.wantCustomer {
				t.Errorf("Customer = %q, want %q", data.Customer.ValueString(), tc.wantCustomer)
			}
			if data.ChildTenantEligible.ValueBool() != tc.wantChildEligible {
				t.Errorf("ChildTenantEligible = %v, want %v", data.ChildTenantEligible.ValueBool(), tc.wantChildEligible)
			}
			var perms []string
			if diags := data.Permissions.ElementsAs(t.Context(), &perms, false); diags.HasError() {
				t.Fatalf("failed to decode permissions: %v", diags)
			}
			if len(perms) != tc.wantPermissionsCount {
				t.Errorf("len(Permissions) = %d, want %d", len(perms), tc.wantPermissionsCount)
			}
		})
	}
}

func TestRoleDataSourceReadErrors(t *testing.T) {
	for _, tc := range []struct {
		name        string
		status      int
		contentType string
		body        string
		want        string
	}{
		{"missing", 404, "application/json", `{"error":"missing"}`, "status 404"},
		{"server error", 500, "application/json", `{"error":"internal"}`, "status 500"},
		{"non-JSON", 200, "text/plain", "wrong content type", "status 200"},
		{"mismatched ID", 200, "application/json", `{"id":"unexpected-id","name":"Role","type":"custom","permissions":[]}`, "Requested role role-1 but API returned ID \"unexpected-id\""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			id := tftypes.NewValue(tftypes.String, "role-1")
			resp := readRoleDataSourceForTest(t, server, &id)
			if !resp.Diagnostics.HasError() || !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), tc.want) {
				t.Fatalf("diagnostics = %v, want %q", resp.Diagnostics, tc.want)
			}
			if !resp.State.Raw.IsNull() {
				t.Fatal("failed read must not write state")
			}
		})
	}
}

func readRoleDataSourceForTest(t *testing.T, server *httptest.Server, id *tftypes.Value) datasource.ReadResponse {
	t.Helper()
	ctx := t.Context()
	httpClient := server.Client()
	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}

	ds := NewRoleDataSource()
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
	if id != nil {
		values["id"] = *id
	}

	resp := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	ds.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(tftypes.Object{AttributeTypes: attrTypes}, values)}}, &resp)
	return resp
}
