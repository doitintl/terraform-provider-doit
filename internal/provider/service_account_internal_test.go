package provider

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestMapServiceAccountToModel(t *testing.T) {
	ctx := t.Context()

	createTime := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	updateTime := time.Date(2026, 1, 16, 12, 0, 0, 0, time.UTC)

	resp := &models.ServiceAccount{
		Id:              valueToNullable("sa-123"),
		CustomerId:      new("cust-456"),
		Name:            "test-service-account",
		Description:     "A service account for testing",
		CreatedBy:       valueToNullable("Jane Doe"),
		CreatedByEmail:  valueToNullable("jane@example.com"),
		CreatedByUserId: valueToNullable("user-789"),
		CreateTime:      valueToNullable(createTime),
		UpdateTime:      valueToNullable(updateTime),
		Etag:            valueToNullable("etag-token-abc"),
		Permissions:     []string{"cloudAnalyticsReadOnly", "budgetsManager"},
	}

	var state serviceAccountResourceModel
	diags := mapServiceAccountToModel(ctx, resp, &state)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if state.Id.ValueString() != "sa-123" {
		t.Errorf("expected id 'sa-123', got '%s'", state.Id.ValueString())
	}
	if state.CustomerId.ValueString() != "cust-456" {
		t.Errorf("expected customer_id 'cust-456', got '%s'", state.CustomerId.ValueString())
	}
	if state.Name.ValueString() != "test-service-account" {
		t.Errorf("expected name 'test-service-account', got '%s'", state.Name.ValueString())
	}
	if state.Description.ValueString() != "A service account for testing" {
		t.Errorf("expected description 'A service account for testing', got '%s'", state.Description.ValueString())
	}
	if state.CreatedBy.ValueString() != "Jane Doe" {
		t.Errorf("expected created_by 'Jane Doe', got '%s'", state.CreatedBy.ValueString())
	}
	if state.CreatedByEmail.ValueString() != "jane@example.com" {
		t.Errorf("expected created_by_email 'jane@example.com', got '%s'", state.CreatedByEmail.ValueString())
	}
	if state.CreatedByUserId.ValueString() != "user-789" {
		t.Errorf("expected created_by_user_id 'user-789', got '%s'", state.CreatedByUserId.ValueString())
	}
	if state.CreateTime.ValueString() != "2026-01-15T10:00:00Z" {
		t.Errorf("expected create_time '2026-01-15T10:00:00Z', got '%s'", state.CreateTime.ValueString())
	}
	if state.UpdateTime.ValueString() != "2026-01-16T12:00:00Z" {
		t.Errorf("expected update_time '2026-01-16T12:00:00Z', got '%s'", state.UpdateTime.ValueString())
	}
	if state.Etag.ValueString() != "etag-token-abc" {
		t.Errorf("expected etag 'etag-token-abc', got '%s'", state.Etag.ValueString())
	}
	if state.Permissions.IsNull() || len(state.Permissions.Elements()) != 2 {
		t.Fatalf("expected 2 permissions, got %d", len(state.Permissions.Elements()))
	}
}

func TestMapServiceAccountToModel_EmptyPermissions(t *testing.T) {
	ctx := t.Context()

	resp := &models.ServiceAccount{
		Id:          valueToNullable("sa-456"),
		Name:        "sa-empty-perms",
		Description: "",
		Permissions: nil, // API returned nil permissions
	}

	var state serviceAccountResourceModel
	diags := mapServiceAccountToModel(ctx, resp, &state)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	// Must be an empty known list, not null (listnullread)
	if state.Permissions.IsNull() {
		t.Fatal("expected permissions to be non-null empty list, got null")
	}
	if len(state.Permissions.Elements()) != 0 {
		t.Errorf("expected 0 permissions, got %d", len(state.Permissions.Elements()))
	}
}

func TestOverlayServiceAccountComputedFields(t *testing.T) {
	ctx := t.Context()

	createTime := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	updateTime := time.Date(2026, 1, 16, 12, 0, 0, 0, time.UTC)

	apiResp := &models.ServiceAccount{
		Id:              valueToNullable("sa-overlay"),
		CustomerId:      new("cust-overlay"),
		Name:            "server-name",
		Description:     "server-desc",
		CreatedBy:       valueToNullable("Creator"),
		CreatedByEmail:  valueToNullable("creator@example.com"),
		CreatedByUserId: valueToNullable("uid-1"),
		CreateTime:      valueToNullable(createTime),
		UpdateTime:      valueToNullable(updateTime),
		Etag:            valueToNullable("etag-v2"),
		Permissions:     []string{"cloudAnalyticsReadOnly"},
	}

	// Scenario 1: Create overlay (all computed fields unknown)
	plan := serviceAccountResourceModel{
		Id:              types.StringUnknown(),
		CustomerId:      types.StringUnknown(),
		Name:            types.StringValue("user-name"),
		Description:     types.StringUnknown(),
		CreatedBy:       types.StringUnknown(),
		CreatedByEmail:  types.StringUnknown(),
		CreatedByUserId: types.StringUnknown(),
		CreateTime:      types.StringUnknown(),
		UpdateTime:      types.StringUnknown(),
		Etag:            types.StringUnknown(),
		Permissions:     types.ListUnknown(types.StringType),
	}

	diags := overlayServiceAccountComputedFields(ctx, apiResp, &plan)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics on create overlay: %v", diags)
	}

	if plan.Id.ValueString() != "sa-overlay" {
		t.Errorf("expected Id 'sa-overlay', got '%s'", plan.Id.ValueString())
	}
	if plan.Name.ValueString() != "user-name" {
		t.Errorf("expected Name to remain 'user-name', got '%s'", plan.Name.ValueString())
	}
	if plan.Description.ValueString() != "server-desc" {
		t.Errorf("expected Description 'server-desc', got '%s'", plan.Description.ValueString())
	}
	if plan.UpdateTime.ValueString() != "2026-01-16T12:00:00Z" {
		t.Errorf("expected UpdateTime '2026-01-16T12:00:00Z', got '%s'", plan.UpdateTime.ValueString())
	}
	if plan.Etag.ValueString() != "etag-v2" {
		t.Errorf("expected Etag 'etag-v2', got '%s'", plan.Etag.ValueString())
	}

	// Scenario 2: Update overlay with known UpdateTime and Etag (should be preserved)
	planUpdate := serviceAccountResourceModel{
		Id:              types.StringValue("sa-overlay"),
		CustomerId:      types.StringValue("cust-overlay"),
		Name:            types.StringValue("updated-user-name"),
		Description:     types.StringValue("user-desc"),
		CreatedBy:       types.StringValue("Creator"),
		CreatedByEmail:  types.StringValue("creator@example.com"),
		CreatedByUserId: types.StringValue("uid-1"),
		CreateTime:      types.StringValue("2026-01-15T10:00:00Z"),
		UpdateTime:      types.StringValue("2026-01-15T10:00:00Z"), // Prior known timestamp
		Etag:            types.StringValue("etag-v1"),              // Prior known etag
		Permissions:     types.ListUnknown(types.StringType),
	}

	diags = overlayServiceAccountComputedFields(ctx, apiResp, &planUpdate)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics on update overlay: %v", diags)
	}

	if planUpdate.UpdateTime.ValueString() != "2026-01-15T10:00:00Z" {
		t.Errorf("expected UpdateTime to preserve prior known value '2026-01-15T10:00:00Z', got '%s'", planUpdate.UpdateTime.ValueString())
	}
	if planUpdate.Etag.ValueString() != "etag-v1" {
		t.Errorf("expected Etag to preserve prior known value 'etag-v1', got '%s'", planUpdate.Etag.ValueString())
	}
	if planUpdate.Description.ValueString() != "user-desc" {
		t.Errorf("expected Description to preserve known user value, got '%s'", planUpdate.Description.ValueString())
	}
}

func TestToCreateRequest(t *testing.T) {
	ctx := t.Context()

	permsList, diags := types.ListValueFrom(ctx, types.StringType, []string{"cloudAnalyticsReadOnly"})
	if diags.HasError() {
		t.Fatalf("failed to create permissions list: %v", diags)
	}

	plan := serviceAccountResourceModel{
		Name:        types.StringValue("my-sa"),
		Description: types.StringValue("My service account"),
		Permissions: permsList,
	}

	req, diags := plan.toCreateRequest(ctx)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if req.Name != "my-sa" {
		t.Errorf("expected name 'my-sa', got '%s'", req.Name)
	}
	if req.Description == nil || *req.Description != "My service account" {
		t.Errorf("expected description 'My service account', got %v", req.Description)
	}
	if req.Permissions == nil || len(*req.Permissions) != 1 || (*req.Permissions)[0] != "cloudAnalyticsReadOnly" {
		t.Errorf("expected permissions ['cloudAnalyticsReadOnly'], got %v", req.Permissions)
	}
}

func TestToUpdateRequest_ClearingLifecycle(t *testing.T) {
	ctx := t.Context()

	// Scenario: Description and permissions cleared (set to null/empty)
	plan := serviceAccountResourceModel{
		Name:        types.StringValue("my-sa-cleared"),
		Description: types.StringNull(),
		Permissions: types.ListNull(types.StringType),
	}

	req, diags := plan.toUpdateRequest(ctx)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if req.Name == nil || *req.Name != "my-sa-cleared" {
		t.Errorf("expected name 'my-sa-cleared', got %v", req.Name)
	}
	// Description must be explicit empty string to clear
	if !req.Description.IsSpecified() || req.Description.MustGet() != "" {
		t.Errorf("expected description to be explicit empty string \"\", got %v", req.Description)
	}
	// Permissions must be explicit empty slice to clear
	if !req.Permissions.IsSpecified() || len(req.Permissions.MustGet()) != 0 {
		t.Errorf("expected permissions to be explicit empty slice [], got %v", req.Permissions)
	}
}

func TestServiceAccount_PopulateState_NotFound(t *testing.T) {
	ctx := t.Context()

	srv := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/iam/v1/service-accounts/missing-id" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message": "Not Found"}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))

	client := newTestAPIClient(t, srv)

	res := &serviceAccountResource{client: client}
	state := serviceAccountResourceModel{
		Id: types.StringValue("missing-id"),
	}

	diags := res.populateState(ctx, &state)
	if diags.HasError() {
		t.Fatalf("populateState should not return error on 404, got: %v", diags)
	}
	if !state.Id.IsNull() {
		t.Errorf("expected state.Id to be set to null on 404, got '%s'", state.Id.ValueString())
	}
}

// The create/update responses carry an ETag that GET never returns (nanosecond
// vs microsecond updateTime). canonicalServiceAccount must hand back the GET
// view so state holds an ETag the API accepts in If-Match.
func TestServiceAccount_CanonicalServiceAccount(t *testing.T) {
	written := &models.ServiceAccount{
		Id:         valueToNullable("sa-1"),
		Name:       "sa",
		Etag:       valueToNullable("etag-from-write"),
		UpdateTime: valueToNullable(time.Date(2026, 10, 1, 6, 45, 34, 626962627, time.UTC)),
	}

	t.Run("uses the GET response", func(t *testing.T) {
		srv := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"sa-1","name":"sa","etag":"etag-from-get","updateTime":"2026-10-01T06:45:34.626962Z","permissions":[]}`))
		}))
		res := &serviceAccountResource{client: newTestAPIClient(t, srv)}

		got := res.canonicalServiceAccount(t.Context(), written)
		if etag := nullableToPointer(got.Etag); etag == nil || *etag != "etag-from-get" {
			t.Errorf("etag = %v, want etag-from-get", etag)
		}
	})

	t.Run("falls back to the write response when GET fails", func(t *testing.T) {
		srv := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		res := &serviceAccountResource{client: newTestAPIClient(t, srv)}

		if got := res.canonicalServiceAccount(t.Context(), written); got != written {
			t.Errorf("expected the write response as fallback")
		}
	})
}

// Delete must send the live ETag, not the one in state, which may come from a
// create/update response or predate an out-of-band change.
func TestServiceAccountDelete_UsesFreshETag(t *testing.T) {
	var deleteIfMatch string
	srv := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"sa-1","name":"sa","etag":"fresh","permissions":[]}`))
		case http.MethodDelete:
			deleteIfMatch = r.Header.Get("If-Match")
			if deleteIfMatch != "fresh" {
				w.WriteHeader(http.StatusPreconditionFailed)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	res := &serviceAccountResource{client: newTestAPIClient(t, srv)}

	ctx := t.Context()
	schemaResp := &resource.SchemaResponse{}
	res.Schema(ctx, resource.SchemaRequest{}, schemaResp)

	stateValues := map[string]tftypes.Value{}
	for name, attr := range schemaResp.Schema.Attributes {
		stateValues[name] = tftypes.NewValue(attr.GetType().TerraformType(ctx), nil)
	}
	stateValues["id"] = tftypes.NewValue(tftypes.String, "sa-1")
	stateValues["etag"] = tftypes.NewValue(tftypes.String, "stale-from-state")

	state := tfsdk.State{
		Schema: schemaResp.Schema,
		Raw: tftypes.NewValue(
			tftypes.Object{AttributeTypes: getAttributeTypes(ctx, schemaResp.Schema.Attributes)},
			stateValues,
		),
	}

	deleteResp := &resource.DeleteResponse{}
	res.Delete(ctx, resource.DeleteRequest{State: state}, deleteResp)

	if deleteResp.Diagnostics.HasError() {
		t.Fatalf("Delete() failed: %v", deleteResp.Diagnostics)
	}
	if deleteIfMatch != "fresh" {
		t.Errorf("If-Match = %q, want the ETag from GET", deleteIfMatch)
	}
}
