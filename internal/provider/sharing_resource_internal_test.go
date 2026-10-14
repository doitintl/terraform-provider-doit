package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/oapi-codegen/nullable"
)

// sharingTestRaw builds a raw state/plan value for the sharing schema with the
// given resource_type and public (nil = null, "?" = unknown). All other attributes are null.
func sharingTestRaw(ctx context.Context, t *testing.T, resourceType, public *string) (resource.SchemaResponse, tftypes.Value) {
	t.Helper()

	r := &sharingResource{}
	schemaResp := resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("Schema() diagnostics: %v", schemaResp.Diagnostics)
	}

	attrTypes := getAttributeTypes(ctx, schemaResp.Schema.Attributes)
	values := make(map[string]tftypes.Value, len(attrTypes))
	for name, typ := range attrTypes {
		values[name] = tftypes.NewValue(typ, nil)
	}

	str := func(v *string) tftypes.Value {
		switch {
		case v == nil:
			return tftypes.NewValue(tftypes.String, nil)
		case *v == "?":
			return tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
		default:
			return tftypes.NewValue(tftypes.String, *v)
		}
	}
	values["resource_type"] = str(resourceType)
	values["public"] = str(public)

	return schemaResp, tftypes.NewValue(tftypes.Object{AttributeTypes: attrTypes}, values)
}

func stringFromPtr(s *string) types.String {
	switch {
	case s == nil:
		return types.StringNull()
	case *s == "?":
		return types.StringUnknown()
	default:
		return types.StringValue(*s)
	}
}

func TestSharingPublicClearable(t *testing.T) {
	ctx := t.Context()

	tests := []struct {
		name         string
		resourceType *string
		config       *string // config value of public
		state        *string // prior state value of public
		plan         *string // plan value of public before the modifier runs
		want         types.String
	}{
		// Allocations: an omitted public keeps the prior value.
		{name: "allocations omitted keeps viewer", resourceType: new("allocations"), state: new("viewer"), plan: new("viewer"), want: types.StringValue("viewer")},
		{name: "allocations omitted keeps editor", resourceType: new("allocations"), state: new("editor"), plan: new("editor"), want: types.StringValue("editor")},
		{name: "allocations create stays unknown", resourceType: new("allocations"), plan: new("?"), want: types.StringUnknown()},
		{name: "allocations explicit value untouched", resourceType: new("allocations"), config: new("editor"), state: new("viewer"), plan: new("editor"), want: types.StringValue("editor")},

		// Other resource types: an omitted public clears the prior value.
		{name: "reports omitted clears viewer", resourceType: new("reports"), state: new("viewer"), plan: new("viewer"), want: types.StringNull()},
		{name: "budgets omitted clears editor", resourceType: new("budgets"), state: new("editor"), plan: new("editor"), want: types.StringNull()},
		{name: "alerts omitted clears viewer", resourceType: new("alerts"), state: new("viewer"), plan: new("viewer"), want: types.StringNull()},
		{name: "reports create stays unknown", resourceType: new("reports"), plan: new("?"), want: types.StringUnknown()},
		{name: "reports prior null stays null", resourceType: new("reports"), plan: nil, want: types.StringNull()},
		{name: "reports explicit value untouched", resourceType: new("reports"), config: new("viewer"), state: new("editor"), plan: new("viewer"), want: types.StringValue("viewer")},

		// resource_type not yet known: public is resolved after apply.
		{name: "unknown resource_type defers", resourceType: new("?"), state: new("viewer"), plan: new("viewer"), want: types.StringUnknown()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schemaResp, planRaw := sharingTestRaw(ctx, t, tt.resourceType, tt.plan)
			_, stateRaw := sharingTestRaw(ctx, t, tt.resourceType, tt.state)
			_, configRaw := sharingTestRaw(ctx, t, tt.resourceType, tt.config)

			req := planmodifier.StringRequest{
				Path:        path.Root("public"),
				Plan:        tfsdk.Plan{Schema: schemaResp.Schema, Raw: planRaw},
				State:       tfsdk.State{Schema: schemaResp.Schema, Raw: stateRaw},
				Config:      tfsdk.Config{Schema: schemaResp.Schema, Raw: configRaw},
				ConfigValue: stringFromPtr(tt.config),
				StateValue:  stringFromPtr(tt.state),
				PlanValue:   stringFromPtr(tt.plan),
			}
			resp := &planmodifier.StringResponse{PlanValue: req.PlanValue}

			sharingPublicClearable().PlanModifyString(ctx, req, resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
			}
			if !resp.PlanValue.Equal(tt.want) {
				t.Errorf("PlanValue = %v, want %v", resp.PlanValue, tt.want)
			}
		})
	}
}

func TestBuildSharingRequest_Public(t *testing.T) {
	ctx := t.Context()

	tests := []struct {
		name          string
		public        types.String
		wantSpecified bool
		wantValue     string
	}{
		// An omitted public must stay omitted for every resource type: the API rejects ""
		// and null for allocations, and keeps the stored value when it is omitted.
		{name: "null is omitted", public: types.StringNull()},
		{name: "unknown is omitted", public: types.StringUnknown()},
		{name: "viewer is sent", public: types.StringValue("viewer"), wantSpecified: true, wantValue: "viewer"},
		{name: "editor is sent", public: types.StringValue("editor"), wantSpecified: true, wantValue: "editor"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var plan sharingResourceModel
			plan.Public = tt.public
			plan.Permissions = types.ListNull(types.StringType)

			req, diags := buildSharingRequest(ctx, &plan)
			if diags.HasError() {
				t.Fatalf("buildSharingRequest() diagnostics: %v", diags)
			}

			if got := req.Public.IsSpecified(); got != tt.wantSpecified {
				t.Fatalf("Public.IsSpecified() = %v, want %v", got, tt.wantSpecified)
			}
			if tt.wantSpecified {
				if req.Public.IsNull() {
					t.Fatal("Public is an explicit null, want a value")
				}
				if got := string(req.Public.MustGet()); got != tt.wantValue {
					t.Errorf("Public = %q, want %q", got, tt.wantValue)
				}
			}
		})
	}
}

// TestSharingResourceDelete_ResetsPublic verifies the permissions reset sent on destroy:
// only the owner remains, allocations are reset to "viewer" explicitly (an omitted public
// would keep a previously configured "editor"), and other types omit public.
func TestSharingResourceDelete_ResetsPublic(t *testing.T) {
	tests := []struct {
		name         string
		resourceType string
		wantPublic   *string
	}{
		{name: "allocations are reset to viewer", resourceType: "allocations", wantPublic: new("viewer")},
		{name: "budgets omit public", resourceType: "budgets"},
		{name: "reports omit public", resourceType: "reports"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()

			var putBody map[string]any
			server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.Method {
				case http.MethodGet:
					_, _ = w.Write([]byte(`{"permissions":[{"user":"owner@example.com","role":"owner"},{"user":"viewer@example.com","role":"viewer"}],"public":"editor"}`))
				case http.MethodPut:
					body, _ := io.ReadAll(r.Body)
					_ = json.Unmarshal(body, &putBody)
					_, _ = w.Write([]byte(`{}`))
				default:
					w.WriteHeader(http.StatusMethodNotAllowed)
				}
			}))
			r := &sharingResource{client: newTestAPIClient(t, server)}

			schemaResp := resource.SchemaResponse{}
			r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

			rt := tt.resourceType
			id := "resource-id"
			_, raw := sharingTestRaw(ctx, t, &rt, nil)
			var obj map[string]tftypes.Value
			if err := raw.As(&obj); err != nil {
				t.Fatalf("raw.As() error = %v", err)
			}
			obj["resource_id"] = tftypes.NewValue(tftypes.String, id)
			state := tfsdk.State{
				Schema: schemaResp.Schema,
				Raw:    tftypes.NewValue(raw.Type(), obj),
			}

			resp := &resource.DeleteResponse{}
			r.Delete(ctx, resource.DeleteRequest{State: state}, resp)
			if resp.Diagnostics.HasError() {
				t.Fatalf("Delete() diagnostics: %v", resp.Diagnostics)
			}

			perms, _ := putBody["permissions"].([]any)
			if len(perms) != 1 {
				t.Fatalf("reset permissions = %v, want only the owner", putBody["permissions"])
			}
			if owner, _ := perms[0].(map[string]any); owner["role"] != "owner" || owner["user"] != "owner@example.com" {
				t.Errorf("reset permission = %v, want the owner", perms[0])
			}

			got, present := putBody["public"]
			switch {
			case tt.wantPublic == nil && present:
				t.Errorf("reset body has public = %v, want it omitted", got)
			case tt.wantPublic != nil && got != *tt.wantPublic:
				t.Errorf("reset body public = %v, want %q", got, *tt.wantPublic)
			}
		})
	}
}

func TestMapSharingToModel_Public(t *testing.T) {
	ctx := t.Context()

	tests := []struct {
		name   string
		public nullable.Nullable[models.ResourcePermissionsResponsePublic]
		want   types.String
	}{
		{name: "viewer", public: valueToNullable(models.ResourcePermissionsResponsePublicViewer), want: types.StringValue("viewer")},
		{name: "editor", public: valueToNullable(models.ResourcePermissionsResponsePublicEditor), want: types.StringValue("editor")},
		{name: "empty string is normalized to null", public: valueToNullable(models.ResourcePermissionsResponsePublic("")), want: types.StringNull()},
		{name: "absent is null", want: types.StringNull()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiResp := &models.ResourcePermissionsResponse{Public: tt.public}

			var state sharingResourceModel
			if diags := mapSharingToModel(ctx, apiResp, &state); diags.HasError() {
				t.Fatalf("mapSharingToModel() diagnostics: %v", diags)
			}
			if !state.Public.Equal(tt.want) {
				t.Errorf("Public = %v, want %v", state.Public, tt.want)
			}
		})
	}
}
