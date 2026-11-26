package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	modelsv2 "github.com/doitintl/terraform-provider-doit/internal/provider/models/v2"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_cloudconnect_gcp_connection"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

const testGcpConnectionJSON = `{"id":"google-cloud-100000000000000000001","etag":"\"2.b7c2d9e0\"","revision":"2",` +
	`"scopeKind":"gcp_project","organizationId":null,"projectId":"example-project","datasetId":null,` +
	`"serviceAccount":{"email":"doit-cmp@example-project.iam.gserviceaccount.com","uniqueId":"100000000000000000001","projectId":"example-project"},` +
	`"roleId":"doit_cmp_role","bindingId":"binding_gcp_project_example","featureIds":["bigquery-finops","core"],` +
	`"featureInputs":{"bigqueryFinops":{"location":"EU"}},"catalogVersion":"sha256:abc","registeredVia":"api",` +
	`"clientUserAgent":"terraform-provider-doit/dev","registration":{"status":"registered","registrationTime":"2026-11-23T12:00:00Z"},` +
	`"baselineChecks":{"status":"passed","checkTime":"2026-11-23T12:00:00Z"},"featureVerification":{"status":"not_checked"},` +
	`"followUps":[{"bindingId":"binding_gcp_project_example","featureId":"bigquery-finops","kind":"create_log_sink","status":"failed",` +
	`"lastError":{"code":"required_permission_missing","message":"Missing logging.sinks.create."}}]}`

func gcpConnectionTestSchema(t *testing.T) schema.Schema {
	t.Helper()
	schemaResp := &resource.SchemaResponse{}
	(&cloudconnectGcpConnectionResource{}).Schema(t.Context(), resource.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", schemaResp.Diagnostics)
	}
	return schemaResp.Schema
}

func tfValue(t *testing.T, value attr.Value) tftypes.Value {
	t.Helper()
	tfv, err := value.ToTerraformValue(t.Context())
	if err != nil {
		t.Fatalf("ToTerraformValue: %v", err)
	}
	return tfv
}

func gcpFeatureInputsValue(t *testing.T, location string) resource_cloudconnect_gcp_connection.FeatureInputsValue {
	t.Helper()
	ctx := t.Context()
	finops := resource_cloudconnect_gcp_connection.NewBigqueryFinopsValueMust(
		resource_cloudconnect_gcp_connection.BigqueryFinopsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"location": types.StringValue(location)},
	)
	return resource_cloudconnect_gcp_connection.NewFeatureInputsValueMust(
		resource_cloudconnect_gcp_connection.FeatureInputsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"bigquery_finops": finops},
	)
}

// gcpConnectionTestPlan plans a create of a project connection.
func gcpConnectionTestPlan(t *testing.T, featureIDs []string, featureInputs attr.Value) tfsdk.Plan {
	t.Helper()
	ctx := t.Context()
	s := gcpConnectionTestSchema(t)

	serviceAccount := resource_cloudconnect_gcp_connection.NewServiceAccountValueMust(
		resource_cloudconnect_gcp_connection.ServiceAccountValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"email":      types.StringValue("doit-cmp@example-project.iam.gserviceaccount.com"),
			"unique_id":  types.StringValue("100000000000000000001"),
			"project_id": types.StringValue("example-project"),
		},
	)
	ids, diags := types.SetValueFrom(ctx, types.StringType, featureIDs)
	if diags.HasError() {
		t.Fatalf("feature_ids: %v", diags)
	}

	return tfsdk.Plan{Schema: s, Raw: onboardingTestRaw(t, s, map[string]tftypes.Value{
		"scope_kind":      tftypes.NewValue(tftypes.String, "gcp_project"),
		"project_id":      tftypes.NewValue(tftypes.String, "example-project"),
		"service_account": tfValue(t, serviceAccount),
		"feature_ids":     tfValue(t, ids),
		"feature_inputs":  tfValue(t, featureInputs),
	})}
}

func TestCloudconnectGcpConnectionCreate_StoresTheRegisteredConnection(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		server, recorder := newBillingTestServer(t, map[string][]func(http.ResponseWriter){
			http.MethodPost: {jsonResponse(http.StatusCreated, testGcpConnectionJSON)},
		})

		r := &cloudconnectGcpConnectionResource{client: newTestV2Client(t, server, "")}
		plan := gcpConnectionTestPlan(t, []string{"core", "bigquery-finops"}, gcpFeatureInputsValue(t, "EU"))
		resp := &resource.CreateResponse{State: tfsdk.State{Schema: plan.Schema, Raw: tftypes.NewValue(plan.Raw.Type(), nil)}}

		r.Create(t.Context(), resource.CreateRequest{Plan: plan}, resp)

		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected errors: %v", resp.Diagnostics)
		}
		assertDistinctKeys(t, recorder.requestKeys(http.MethodPost), 1)

		warnings := resp.Diagnostics.Warnings()
		if len(warnings) != 1 || warnings[0].Summary() != "GCP Cloud Connect Follow-Up Step Failed" {
			t.Fatalf("warnings = %v, want one failed follow-up warning", warnings)
		}
		if detail := warnings[0].Detail(); !strings.Contains(detail, `"create_log_sink" for feature "bigquery-finops" failed: Missing logging.sinks.create. (required_permission_missing)`) {
			t.Errorf("warning detail = %q", detail)
		}

		var state cloudconnectGcpConnectionResourceModel
		resp.Diagnostics.Append(resp.State.Get(t.Context(), &state)...)
		if resp.Diagnostics.HasError() {
			t.Fatalf("state: %v", resp.Diagnostics)
		}
		if got := state.Id.ValueString(); got != "google-cloud-100000000000000000001" {
			t.Errorf("id = %q", got)
		}
		if got := state.Etag.ValueString(); got != `"2.b7c2d9e0"` {
			t.Errorf("etag = %q", got)
		}
		if got := state.RoleId.ValueString(); got != "doit_cmp_role" {
			t.Errorf("role_id = %q, want the API default", got)
		}
		if !state.OrganizationId.IsNull() || !state.DatasetId.IsNull() {
			t.Errorf("organization_id = %v, dataset_id = %v, want null", state.OrganizationId, state.DatasetId)
		}
		if got := state.FeatureInputs.BigqueryFinops.Location.ValueString(); got != "EU" {
			t.Errorf("feature_inputs.bigquery_finops.location = %q, want EU", got)
		}
		if got := len(state.FollowUps.Elements()); got != 1 {
			t.Errorf("follow_ups = %d elements, want 1", got)
		}
	})
}

func TestCloudconnectGcpConnectionCreate_DoesNotRetryAGateFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		server, recorder := newBillingTestServer(t, map[string][]func(http.ResponseWriter){
			http.MethodPost: {problemResponse(http.StatusUnprocessableEntity, "onboarding.registration_gate_failed", false)},
		})

		r := &cloudconnectGcpConnectionResource{client: newTestV2Client(t, server, "")}
		plan := gcpConnectionTestPlan(t, []string{"core"}, resource_cloudconnect_gcp_connection.NewFeatureInputsValueNull())
		resp := &resource.CreateResponse{State: tfsdk.State{Schema: plan.Schema, Raw: tftypes.NewValue(plan.Raw.Type(), nil)}}

		start := time.Now()
		r.Create(t.Context(), resource.CreateRequest{Plan: plan}, resp)

		if !resp.Diagnostics.HasError() {
			t.Fatal("expected an error")
		}
		if got := len(recorder.requestKeys(http.MethodPost)); got != 1 {
			t.Errorf("requests = %d, want 1", got)
		}
		if elapsed := time.Since(start); elapsed != 0 {
			t.Errorf("elapsed = %v, want 0", elapsed)
		}
	})
}

// requestJSON marshals a request body to a generic map, to check which keys it sends.
func requestJSON(t *testing.T, body any) map[string]any {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(encoded, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return out
}

func TestCloudconnectGcpConnectionToUpdateRequest_FeatureInputs(t *testing.T) {
	ctx := t.Context()
	eu := gcpFeatureInputsValue(t, "EU")
	us := gcpFeatureInputsValue(t, "US")
	null := resource_cloudconnect_gcp_connection.NewFeatureInputsValueNull()
	unknown := resource_cloudconnect_gcp_connection.NewFeatureInputsValueUnknown()

	tests := []struct {
		name      string
		planned   resource_cloudconnect_gcp_connection.FeatureInputsValue
		stored    resource_cloudconnect_gcp_connection.FeatureInputsValue
		wantKey   bool
		wantValue any
	}{
		{name: "removed block sends null", planned: null, stored: eu, wantKey: true, wantValue: nil},
		{name: "unchanged block is left out", planned: eu, stored: eu},
		{name: "never set is left out", planned: null, stored: null},
		{name: "changed location is sent", planned: us, stored: eu, wantKey: true, wantValue: map[string]any{"bigqueryFinops": map[string]any{"location": "US"}}},
		{name: "added block is sent", planned: eu, stored: null, wantKey: true, wantValue: map[string]any{"bigqueryFinops": map[string]any{"location": "EU"}}},
		{name: "unknown block is left out", planned: unknown, stored: eu},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var plan, state cloudconnectGcpConnectionResourceModel
			plan.FeatureIds = types.SetValueMust(types.StringType, []attr.Value{types.StringValue("core")})
			plan.CatalogVersion = types.StringUnknown()
			plan.FeatureInputs = tt.planned
			state.FeatureInputs = tt.stored

			req, diags := plan.toUpdateRequest(ctx, &state)
			if diags.HasError() {
				t.Fatalf("diagnostics: %v", diags)
			}
			got := requestJSON(t, req)

			value, ok := got["featureInputs"]
			if ok != tt.wantKey {
				t.Fatalf("featureInputs sent = %v, want %v (body %v)", ok, tt.wantKey, got)
			}
			if tt.wantKey {
				wantJSON, _ := json.Marshal(tt.wantValue)
				gotJSON, _ := json.Marshal(value)
				if string(gotJSON) != string(wantJSON) {
					t.Errorf("featureInputs = %s, want %s", gotJSON, wantJSON)
				}
			}
			if ids, ok := got["featureIds"].([]any); !ok || len(ids) != 1 || ids[0] != "core" {
				t.Errorf("featureIds = %v, want [core]", got["featureIds"])
			}
			if _, ok := got["catalogVersion"]; ok {
				t.Errorf("catalogVersion sent for an unknown plan value: %v", got)
			}
		})
	}
}

func TestCloudconnectGcpConnectionToCreateRequest_OmittedAttributes(t *testing.T) {
	ctx := t.Context()
	plan := gcpConnectionTestPlan(t, []string{"core"}, resource_cloudconnect_gcp_connection.NewFeatureInputsValueNull())
	var model cloudconnectGcpConnectionResourceModel
	if diags := plan.Get(ctx, &model); diags.HasError() {
		t.Fatalf("plan: %v", diags)
	}

	req, diags := model.toCreateRequest(ctx)
	if diags.HasError() {
		t.Fatalf("diagnostics: %v", diags)
	}
	got := requestJSON(t, req)

	want := map[string]any{
		"scopeKind": "gcp_project",
		"projectId": "example-project",
		"serviceAccount": map[string]any{
			"email":     "doit-cmp@example-project.iam.gserviceaccount.com",
			"uniqueId":  "100000000000000000001",
			"projectId": "example-project",
		},
		"featureIds": []any{"core"},
	}
	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("create body = %s, want %s", gotJSON, wantJSON)
	}
}

func TestBuildGcpFeatureInputs(t *testing.T) {
	ctx := t.Context()

	got, diags := buildGcpFeatureInputs(ctx, nil)
	if diags.HasError() || !got.IsNull() {
		t.Errorf("nil inputs = %v (%v), want null", got, diags)
	}

	inputs := &modelsv2.GcpFeatureInputs{}
	inputs.BigqueryFinops.Set(modelsv2.GcpBigQueryFinOpsInputs{Location: valueToNullable("EU")})
	got, diags = buildGcpFeatureInputs(ctx, inputs)
	if diags.HasError() {
		t.Fatalf("diagnostics: %v", diags)
	}
	if want := gcpFeatureInputsValue(t, "EU"); !got.Equal(want) {
		t.Errorf("inputs = %v, want %v", got, want)
	}
}

func TestUseNullForUnknownGcpFeatureInputsWhenConfigNull(t *testing.T) {
	eu := gcpFeatureInputsValue(t, "EU")
	us := gcpFeatureInputsValue(t, "US")
	attrTypes := resource_cloudconnect_gcp_connection.FeatureInputsValue{}.AttributeTypes(t.Context())
	objectNull := types.ObjectNull(attrTypes)
	objectUnknown := types.ObjectUnknown(attrTypes)

	toObject := func(v resource_cloudconnect_gcp_connection.FeatureInputsValue) types.Object {
		obj, diags := v.ToObjectValue(t.Context())
		if diags.HasError() {
			t.Fatalf("ToObjectValue: %v", diags)
		}
		return obj
	}

	tests := []struct {
		name   string
		config types.Object
		plan   types.Object
		state  types.Object
		want   types.Object
	}{
		{name: "create, omitted: null instead of unknown", config: objectNull, plan: objectUnknown, state: objectNull, want: objectNull},
		{name: "update, removed: null instead of the stored value", config: objectNull, plan: toObject(eu), state: toObject(eu), want: objectNull},
		{name: "update, removed while another attribute changes", config: objectNull, plan: objectUnknown, state: toObject(eu), want: objectNull},
		{name: "configured value is kept", config: toObject(us), plan: toObject(us), state: toObject(eu), want: toObject(us)},
		{name: "unknown configured value is kept", config: objectUnknown, plan: objectUnknown, state: toObject(eu), want: objectUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := planmodifier.ObjectRequest{ConfigValue: tt.config, PlanValue: tt.plan, StateValue: tt.state}
			resp := &planmodifier.ObjectResponse{PlanValue: tt.plan}
			useNullForUnknownGcpFeatureInputsWhenConfigNull().PlanModifyObject(t.Context(), req, resp)
			if resp.Diagnostics.HasError() {
				t.Fatalf("diagnostics: %v", resp.Diagnostics)
			}
			if !resp.PlanValue.Equal(tt.want) {
				t.Errorf("plan = %v, want %v", resp.PlanValue, tt.want)
			}
		})
	}
}

func TestCloudconnectGcpFeatureInputsValidator(t *testing.T) {
	ctx := t.Context()
	s := gcpConnectionTestSchema(t)
	eu := gcpFeatureInputsValue(t, "EU")
	finopsNull := resource_cloudconnect_gcp_connection.NewFeatureInputsValueMust(
		resource_cloudconnect_gcp_connection.FeatureInputsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"bigquery_finops": resource_cloudconnect_gcp_connection.NewBigqueryFinopsValueNull()},
	)
	ids := func(values ...attr.Value) types.Set { return types.SetValueMust(types.StringType, values) }

	tests := []struct {
		name          string
		featureInputs attr.Value
		featureIDs    types.Set
		wantError     bool
	}{
		{name: "no inputs", featureInputs: resource_cloudconnect_gcp_connection.NewFeatureInputsValueNull(), featureIDs: ids(types.StringValue("core"))},
		{name: "unknown inputs are deferred", featureInputs: resource_cloudconnect_gcp_connection.NewFeatureInputsValueUnknown(), featureIDs: ids(types.StringValue("core"))},
		{name: "inputs without bigquery_finops", featureInputs: finopsNull, featureIDs: ids(types.StringValue("core"))},
		{name: "feature enabled", featureInputs: eu, featureIDs: ids(types.StringValue("core"), types.StringValue("bigquery-finops"))},
		{name: "feature missing", featureInputs: eu, featureIDs: ids(types.StringValue("core")), wantError: true},
		{name: "unknown feature list is deferred", featureInputs: eu, featureIDs: types.SetUnknown(types.StringType)},
		{name: "unknown element is deferred", featureInputs: eu, featureIDs: ids(types.StringValue("core"), types.StringUnknown())},
		{name: "unknown element and the feature", featureInputs: eu, featureIDs: ids(types.StringUnknown(), types.StringValue("bigquery-finops"))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := tfsdk.Config{Schema: s, Raw: onboardingTestRaw(t, s, map[string]tftypes.Value{
				"feature_inputs": tfValue(t, tt.featureInputs),
				"feature_ids":    tfValue(t, tt.featureIDs),
			})}
			resp := &resource.ValidateConfigResponse{}
			cloudconnectGcpFeatureInputsValidator{}.ValidateResource(ctx, resource.ValidateConfigRequest{Config: config}, resp)

			if !tt.wantError {
				if len(resp.Diagnostics) != 0 {
					t.Errorf("diagnostics = %v, want none", resp.Diagnostics)
				}
				return
			}
			if len(resp.Diagnostics) != 1 || resp.Diagnostics[0].Severity() != diag.SeverityError ||
				resp.Diagnostics[0].Summary() != "BigQuery FinOps Inputs Without the Feature" {
				t.Fatalf("diagnostics = %v, want one error", resp.Diagnostics)
			}
			withPath, ok := resp.Diagnostics[0].(diag.DiagnosticWithPath)
			if !ok || withPath.Path().String() != "feature_inputs.bigquery_finops" {
				t.Errorf("diagnostic path = %v", resp.Diagnostics[0])
			}
		})
	}
}

func TestCloudconnectGcpConnectionDelete_RefreshesStaleEtag(t *testing.T) {
	var deleteQueries, deleteEtags []string
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			jsonResponse(http.StatusOK, testGcpConnectionJSON)(w)
			return
		}
		deleteQueries = append(deleteQueries, r.URL.RawQuery)
		deleteEtags = append(deleteEtags, r.Header.Get("If-Match"))
		if len(deleteEtags) == 1 {
			writeProblem(w, http.StatusPreconditionFailed, "precondition_failed", false)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	r := &cloudconnectGcpConnectionResource{client: newTestV2Client(t, server, "")}
	s := gcpConnectionTestSchema(t)
	state := tfsdk.State{Schema: s, Raw: onboardingTestRaw(t, s, map[string]tftypes.Value{
		"id":   tftypes.NewValue(tftypes.String, "google-cloud-100000000000000000001"),
		"etag": tftypes.NewValue(tftypes.String, `"1.stale"`),
	})}
	resp := &resource.DeleteResponse{State: state}
	r.Delete(t.Context(), resource.DeleteRequest{State: state}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}
	if want := []string{`"1.stale"`, `"2.b7c2d9e0"`}; strings.Join(deleteEtags, " ") != strings.Join(want, " ") {
		t.Errorf("If-Match = %v, want %v", deleteEtags, want)
	}
	for _, query := range deleteQueries {
		if query != "cleanupScope=doit_only" {
			t.Errorf("query = %q, want cleanupScope=doit_only", query)
		}
	}
}
