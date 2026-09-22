package provider

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_ps4c_gcp_recommendations"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestMapGcpRecommendationsToItemsList_MultipleItems(t *testing.T) {
	ctx := t.Context()

	currCommitment1 := 100.0
	cov1 := 0.80
	policy1 := models.CommitmentPolicyId("balanced")
	savings1 := 250.0
	recCommitment1 := 115.0
	region1 := "global"
	service1 := models.GcpRecommendationService("compute")

	currCommitment2 := 50.0
	cov2 := 0.90
	policy2 := models.CommitmentPolicyId("custom-policy")
	savings2 := 120.0
	recCommitment2 := 65.0
	region2 := "us_central1"
	service2 := models.GcpRecommendationService("cloud_sql")

	items := []models.GcpRecommendation{
		{
			CurrentCommitment:          &currCommitment1,
			EstimatedAverageCoverage:   &cov1,
			Policy:                     &policy1,
			PotentialAdditionalSavings: &savings1,
			RecommendedCommitment:      &recCommitment1,
			Region:                     &region1,
			Service:                    &service1,
		},
		{
			CurrentCommitment:          &currCommitment2,
			EstimatedAverageCoverage:   &cov2,
			Policy:                     &policy2,
			PotentialAdditionalSavings: &savings2,
			RecommendedCommitment:      &recCommitment2,
			Region:                     &region2,
			Service:                    &service2,
		},
	}

	list, diags := mapGcpRecommendationsToItemsList(ctx, items)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	elements := list.Elements()
	if len(elements) != 2 {
		t.Fatalf("elements count = %d, want 2", len(elements))
	}

	first, ok := elements[0].(datasource_ps4c_gcp_recommendations.ItemsValue)
	if !ok {
		t.Fatalf("elements[0] is not ItemsValue, got %T", elements[0])
	}
	if got := first.Service.ValueString(); got != "compute" {
		t.Errorf("first service = %q, want 'compute'", got)
	}
	if got := first.Region.ValueString(); got != "global" {
		t.Errorf("first region = %q, want 'global'", got)
	}
	if got := first.RecommendedCommitment.ValueFloat64(); got != 115.0 {
		t.Errorf("first recommended_commitment = %v, want 115.0", got)
	}
	if got := first.Policy.ValueString(); got != "balanced" {
		t.Errorf("first policy = %q, want 'balanced'", got)
	}

	second, ok := elements[1].(datasource_ps4c_gcp_recommendations.ItemsValue)
	if !ok {
		t.Fatalf("elements[1] is not ItemsValue, got %T", elements[1])
	}
	if got := second.Service.ValueString(); got != "cloud_sql" {
		t.Errorf("second service = %q, want 'cloud_sql'", got)
	}
	if got := second.Region.ValueString(); got != "us_central1" {
		t.Errorf("second region = %q, want 'us_central1'", got)
	}
}

func TestMapGcpRecommendationsToItemsList_Empty(t *testing.T) {
	ctx := t.Context()

	list, diags := mapGcpRecommendationsToItemsList(ctx, []models.GcpRecommendation{})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if list.IsNull() {
		t.Errorf("list should not be null for empty slice")
	}
	if got := len(list.Elements()); got != 0 {
		t.Errorf("list elements count = %d, want 0", got)
	}
}

func TestPs4cGcpRecommendationsRead_Success(t *testing.T) {
	calls := 0
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/ps4commitments/v1/gcp/billing-accounts/012345-6789AB-CDEF01/recommendations" {
			t.Errorf("path = %s, want /ps4commitments/v1/gcp/billing-accounts/012345-6789AB-CDEF01/recommendations", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"items": [
				{
					"currentCommitment": 40.0,
					"estimatedAverageCoverage": 0.82,
					"policy": "balanced",
					"potentialAdditionalSavings": 120.5,
					"recommendedCommitment": 50.0,
					"region": "global",
					"service": "compute"
				}
			],
			"rowCount": 1,
			"pageToken": null
		}`))
	}))

	resp := readPs4cGcpRecommendations(t, server, nil)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if calls != 1 {
		t.Fatalf("got %d calls, want 1", calls)
	}

	var state ps4cGcpRecommendationsDataSourceModel
	if diags := resp.State.Get(t.Context(), &state); diags.HasError() {
		t.Fatal(diags)
	}

	if got := len(state.Items.Elements()); got != 1 {
		t.Errorf("items count = %d, want 1", got)
	}
	if got := state.RowCount.ValueInt64(); got != 1 {
		t.Errorf("row_count = %d, want 1", got)
	}
	if !state.PageToken.IsNull() {
		t.Errorf("page_token should be null")
	}
}

func TestPs4cGcpRecommendationsRead_NilRowCount(t *testing.T) {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"items": [
				{
					"currentCommitment": 10.0,
					"service": "compute",
					"region": "global"
				}
			]
		}`))
	}))

	resp := readPs4cGcpRecommendations(t, server, nil)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}

	var state ps4cGcpRecommendationsDataSourceModel
	if diags := resp.State.Get(t.Context(), &state); diags.HasError() {
		t.Fatal(diags)
	}

	if got := state.RowCount.ValueInt64(); got != 1 {
		t.Errorf("row_count fallback = %d, want 1", got)
	}
}

func TestPs4cGcpRecommendationsRead_WithFilters(t *testing.T) {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("gcp_service"); got != "cloud_sql" {
			t.Errorf("gcp_service query param = %q, want 'cloud_sql'", got)
		}
		if got := r.URL.Query().Get("region"); got != "us_central1" {
			t.Errorf("region query param = %q, want 'us_central1'", got)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"items": [],
			"rowCount": 0
		}`))
	}))

	overrides := map[string]tftypes.Value{
		"gcp_service": tftypes.NewValue(tftypes.String, "cloud_sql"),
		"region":      tftypes.NewValue(tftypes.String, "us_central1"),
	}

	resp := readPs4cGcpRecommendations(t, server, overrides)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}

	var state ps4cGcpRecommendationsDataSourceModel
	if diags := resp.State.Get(t.Context(), &state); diags.HasError() {
		t.Fatal(diags)
	}

	if got := len(state.Items.Elements()); got != 0 {
		t.Errorf("items count = %d, want 0", got)
	}
	if got := state.RowCount.ValueInt64(); got != 0 {
		t.Errorf("row_count = %d, want 0", got)
	}
}

func TestPs4cGcpRecommendationsRead_NotFound(t *testing.T) {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":"not_found","message":"billing account not found"}`))
	}))

	resp := readPs4cGcpRecommendations(t, server, nil)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for 404, got none")
	}
	if !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "not found") {
		t.Errorf("expected error containing 'not found', got: %v", resp.Diagnostics)
	}
}

func TestPs4cGcpRecommendationsRead_UnknownInput(t *testing.T) {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("API should not be called when billing_account_id is unknown")
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	}))

	overrides := map[string]tftypes.Value{
		"billing_account_id": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
	}

	resp := readPs4cGcpRecommendations(t, server, overrides)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	var state ps4cGcpRecommendationsDataSourceModel
	if diags := resp.State.Get(t.Context(), &state); diags.HasError() {
		t.Fatal(diags)
	}

	if !state.Items.IsUnknown() {
		t.Errorf("items should be unknown")
	}
	if !state.RowCount.IsUnknown() {
		t.Errorf("row_count should be unknown")
	}
	if !state.PageToken.IsUnknown() {
		t.Errorf("page_token should be unknown")
	}
}

func TestPs4cGcpRecommendationsRead_ServerError(t *testing.T) {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"code":"internal_error","message":"backend failure"}`))
	}))

	resp := readPs4cGcpRecommendations(t, server, nil)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for 500, got none")
	}
	if !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "500") {
		t.Errorf("expected error containing '500', got: %v", resp.Diagnostics)
	}
}

func readPs4cGcpRecommendations(t *testing.T, server *httptest.Server, overrides map[string]tftypes.Value) datasource.ReadResponse {
	t.Helper()
	ctx := t.Context()
	httpClient := server.Client()
	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}

	ds := NewPs4cGcpRecommendationsDataSource()
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
		if val, ok := overrides[name]; ok {
			values[name] = val
		} else if name == "billing_account_id" {
			values[name] = tftypes.NewValue(tfType, "012345-6789AB-CDEF01")
		} else {
			values[name] = tftypes.NewValue(tfType, nil)
		}
	}

	resp := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	ds.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(tftypes.Object{AttributeTypes: attrTypes}, values)}}, &resp)
	return resp
}
