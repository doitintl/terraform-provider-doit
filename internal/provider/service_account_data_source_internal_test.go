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

func readServiceAccountDataSourceForTest(t *testing.T, server *httptest.Server, id *tftypes.Value) datasource.ReadResponse {
	t.Helper()
	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	var ds datasource.DataSource
	if id == nil {
		ds = &serviceAccountsDataSource{client: client}
	} else {
		ds = &serviceAccountDataSource{client: client}
	}
	ctx := t.Context()
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

func TestServiceAccountDataSourceUnknownID(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls.Add(1) }))
	id := tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	resp := readServiceAccountDataSourceForTest(t, server, &id)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if calls.Load() != 0 {
		t.Fatalf("made %d API requests for unknown ID", calls.Load())
	}
	var data serviceAccountDataSourceModel
	if diags := resp.State.Get(t.Context(), &data); diags.HasError() {
		t.Fatal(diags)
	}
	if !data.Name.IsUnknown() || !data.Permissions.IsUnknown() || !data.Etag.IsUnknown() {
		t.Fatalf("outputs must remain unknown: %+v", data)
	}
}

func TestServiceAccountDataSourceReadErrors(t *testing.T) {
	for _, tc := range []struct {
		name        string
		list        bool
		status      int
		contentType string
		body        string
		want        string
	}{
		{"singular missing", false, 404, "application/json", `{"error":"missing"}`, "status 404"},
		{"singular non-JSON", false, 200, "text/plain", "wrong content type", "status 200"},
		{"list forbidden", true, 403, "application/json", `{"error":"denied"}`, "status 403"},
		{"list non-JSON", true, 200, "text/plain", "wrong content type", "status 200"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			var id *tftypes.Value
			if !tc.list {
				v := tftypes.NewValue(tftypes.String, "sa-1")
				id = &v
			}
			resp := readServiceAccountDataSourceForTest(t, server, id)
			if !resp.Diagnostics.HasError() || !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), tc.want) {
				t.Fatalf("diagnostics = %v, want %q", resp.Diagnostics, tc.want)
			}
		})
	}
}
