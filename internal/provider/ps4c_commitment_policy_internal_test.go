package provider

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_ps4c_commitment_policy"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/oapi-codegen/nullable"
)

func TestMapCommitmentPolicyDetailToModel_FullyPopulated(t *testing.T) {
	ctx := t.Context()

	createdTime := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	updatedTime := time.Date(2026, 2, 15, 14, 30, 0, 0, time.UTC)

	apiResp := &models.CommitmentPolicyDetail{
		Id:                 "balanced",
		Name:               "Balanced",
		IsBuiltIn:          true,
		TargetCoverage:     80.0,
		LookbackDays:       60,
		LadderStepPercent:  10.0,
		LadderIntervalDays: 7,
		BootstrapPercent:   35.0,
		CreatedTime:        valueToNullable(createdTime),
		UpdatedTime:        valueToNullable(updatedTime),
		Assignments: []models.CommitmentPolicyAssignment{
			{
				AccountId: "012345-6789AB-CDEF01",
				Cloud:     models.CommitmentPolicyAssignmentCloudGcp,
				Region:    valueToNullable("us_central1"),
				Service:   models.CommitmentPolicyAssignmentServiceCloudSql,
			},
			{
				AccountId: "123456789012",
				Cloud:     models.CommitmentPolicyAssignmentCloudAws,
				Region:    nullable.Nullable[string]{},
				Service:   models.CommitmentPolicyAssignmentServiceCompute,
			},
		},
	}

	var data ps4cCommitmentPolicyDataSourceModel
	diags := mapCommitmentPolicyDetailToModel(ctx, apiResp, &data)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if got := data.Id.ValueString(); got != "balanced" {
		t.Errorf("id = %q, want %q", got, "balanced")
	}
	if got := data.PolicyId.ValueString(); got != "balanced" {
		t.Errorf("policy_id = %q, want %q", got, "balanced")
	}
	if got := data.Name.ValueString(); got != "Balanced" {
		t.Errorf("name = %q, want %q", got, "Balanced")
	}
	if got := data.IsBuiltIn.ValueBool(); !got {
		t.Errorf("is_built_in = %v, want true", got)
	}
	if got := data.TargetCoverage.ValueFloat64(); got != 80.0 {
		t.Errorf("target_coverage = %v, want 80.0", got)
	}
	if got := data.LookbackDays.ValueInt64(); got != 60 {
		t.Errorf("lookback_days = %d, want 60", got)
	}
	if got := data.LadderStepPercent.ValueFloat64(); got != 10.0 {
		t.Errorf("ladder_step_percent = %v, want 10.0", got)
	}
	if got := data.LadderIntervalDays.ValueInt64(); got != 7 {
		t.Errorf("ladder_interval_days = %d, want 7", got)
	}
	if got := data.BootstrapPercent.ValueFloat64(); got != 35.0 {
		t.Errorf("bootstrap_percent = %v, want 35.0", got)
	}
	if got := data.CreatedTime.ValueString(); got != "2026-01-10T12:00:00Z" {
		t.Errorf("created_time = %q, want %q", got, "2026-01-10T12:00:00Z")
	}
	if got := data.UpdatedTime.ValueString(); got != "2026-02-15T14:30:00Z" {
		t.Errorf("updated_time = %q, want %q", got, "2026-02-15T14:30:00Z")
	}

	elements := data.Assignments.Elements()
	if len(elements) != 2 {
		t.Fatalf("assignments length = %d, want 2", len(elements))
	}

	first, ok := elements[0].(datasource_ps4c_commitment_policy.AssignmentsValue)
	if !ok {
		t.Fatalf("elements[0] is not AssignmentsValue, got %T", elements[0])
	}
	if got := first.AccountId.ValueString(); got != "012345-6789AB-CDEF01" {
		t.Errorf("assignments[0].account_id = %q, want %q", got, "012345-6789AB-CDEF01")
	}
	if got := first.Cloud.ValueString(); got != "gcp" {
		t.Errorf("assignments[0].cloud = %q, want %q", got, "gcp")
	}
	if got := first.Region.ValueString(); got != "us_central1" {
		t.Errorf("assignments[0].region = %q, want %q", got, "us_central1")
	}
	if got := first.Service.ValueString(); got != "cloud_sql" {
		t.Errorf("assignments[0].service = %q, want %q", got, "cloud_sql")
	}

	second, ok := elements[1].(datasource_ps4c_commitment_policy.AssignmentsValue)
	if !ok {
		t.Fatalf("elements[1] is not AssignmentsValue, got %T", elements[1])
	}
	if got := second.AccountId.ValueString(); got != "123456789012" {
		t.Errorf("assignments[1].account_id = %q, want %q", got, "123456789012")
	}
	if got := second.Cloud.ValueString(); got != "aws" {
		t.Errorf("assignments[1].cloud = %q, want %q", got, "aws")
	}
	if !second.Region.IsNull() {
		t.Errorf("assignments[1].region should be null for account-level scope, got %q", second.Region.ValueString())
	}
	if got := second.Service.ValueString(); got != "compute" {
		t.Errorf("assignments[1].service = %q, want %q", got, "compute")
	}
}

func TestMapCommitmentPolicyDetailToModel_EmptyAssignments(t *testing.T) {
	ctx := t.Context()

	apiResp := &models.CommitmentPolicyDetail{
		Id:                 "max_savings",
		Name:               "Max Savings",
		IsBuiltIn:          true,
		TargetCoverage:     95.0,
		LookbackDays:       30,
		LadderStepPercent:  15.0,
		LadderIntervalDays: 7,
		BootstrapPercent:   50.0,
		CreatedTime:        nullable.Nullable[time.Time]{},
		UpdatedTime:        nullable.Nullable[time.Time]{},
		Assignments:        []models.CommitmentPolicyAssignment{},
	}

	var data ps4cCommitmentPolicyDataSourceModel
	diags := mapCommitmentPolicyDetailToModel(ctx, apiResp, &data)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if data.Assignments.IsNull() {
		t.Errorf("assignments should be an empty list, not null")
	}
	if got := len(data.Assignments.Elements()); got != 0 {
		t.Errorf("assignments length = %d, want 0", got)
	}
	if !data.CreatedTime.IsNull() {
		t.Errorf("created_time should be null, got %q", data.CreatedTime.ValueString())
	}
	if !data.UpdatedTime.IsNull() {
		t.Errorf("updated_time should be null, got %q", data.UpdatedTime.ValueString())
	}
}

func TestPs4cCommitmentPolicyRead(t *testing.T) {
	body := `{
		"id": "balanced",
		"name": "Balanced",
		"isBuiltIn": true,
		"targetCoverage": 80,
		"lookbackDays": 60,
		"ladderStepPercent": 10,
		"ladderIntervalDays": 7,
		"bootstrapPercent": 35,
		"createdTime": "2026-01-10T12:00:00Z",
		"updatedTime": "2026-02-15T14:30:00Z",
		"assignments": [
			{
				"accountId": "012345-6789AB-CDEF01",
				"cloud": "gcp",
				"service": "cloud_sql",
				"region": "us_central1"
			}
		]
	}`

	calls := 0
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodGet || r.URL.Path != "/ps4commitments/v1/general/policies/balanced" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))

	resp := readPs4cCommitmentPolicy(t, server, tftypes.NewValue(tftypes.String, "balanced"))
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if calls != 1 {
		t.Fatalf("got %d calls, want 1", calls)
	}

	var state ps4cCommitmentPolicyDataSourceModel
	if diags := resp.State.Get(t.Context(), &state); diags.HasError() {
		t.Fatal(diags)
	}

	if got := state.Id.ValueString(); got != "balanced" {
		t.Errorf("id = %q, want %q", got, "balanced")
	}
	if got := state.PolicyId.ValueString(); got != "balanced" {
		t.Errorf("policy_id = %q, want %q", got, "balanced")
	}
	if got := state.Name.ValueString(); got != "Balanced" {
		t.Errorf("name = %q, want %q", got, "Balanced")
	}
	if got := state.IsBuiltIn.ValueBool(); !got {
		t.Errorf("is_built_in = %v, want true", got)
	}
	if got := state.TargetCoverage.ValueFloat64(); got != 80.0 {
		t.Errorf("target_coverage = %v, want 80.0", got)
	}
	if got := len(state.Assignments.Elements()); got != 1 {
		t.Errorf("assignments count = %d, want 1", got)
	}
}

func TestPs4cCommitmentPolicyRead_NotFound(t *testing.T) {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":"not_found","message":"policy not found"}`))
	}))

	resp := readPs4cCommitmentPolicy(t, server, tftypes.NewValue(tftypes.String, "nonexistent"))
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for 404, got none")
	}
	if !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "not found") {
		t.Errorf("expected error containing 'not found', got: %v", resp.Diagnostics)
	}
}

func TestPs4cCommitmentPolicyRead_UnknownInput(t *testing.T) {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("API should not be called when policy_id is unknown")
	}))

	resp := readPs4cCommitmentPolicy(t, server, tftypes.NewValue(tftypes.String, tftypes.UnknownValue))
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	var state ps4cCommitmentPolicyDataSourceModel
	if diags := resp.State.Get(t.Context(), &state); diags.HasError() {
		t.Fatal(diags)
	}

	if !state.Id.IsUnknown() {
		t.Errorf("id should be unknown")
	}
	if !state.TargetCoverage.IsUnknown() {
		t.Errorf("target_coverage should be unknown")
	}
	if !state.Assignments.IsUnknown() {
		t.Errorf("assignments should be unknown")
	}
}

func readPs4cCommitmentPolicy(t *testing.T, server *httptest.Server, policyId tftypes.Value) datasource.ReadResponse {
	t.Helper()
	ctx := t.Context()
	httpClient := server.Client()
	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}

	ds := NewPs4cCommitmentPolicyDataSource()
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
		if name == "policy_id" {
			values[name] = policyId
		} else {
			values[name] = tftypes.NewValue(tfType, nil)
		}
	}

	resp := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	ds.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(tftypes.Object{AttributeTypes: attrTypes}, values)}}, &resp)
	return resp
}
