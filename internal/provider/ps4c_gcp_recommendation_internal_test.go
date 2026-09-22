package provider

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_ps4c_gcp_recommendation"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestMapGcpRecommendationToModel_FullyPopulated(t *testing.T) {
	ctx := t.Context()

	estCommitment := 125.50
	currCommitment := 100.00
	cov := 0.75
	policyId := models.CommitmentPolicyId("balanced")
	savings := 350.00
	recCommitment := 120.00
	region := "global"
	service := models.GcpRecommendationService("compute")
	usageTime := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	apiResp := &models.GcpRecommendationWithEligibleSpend{
		EstimatedEquivalentRecommendedCommitment: &estCommitment,
		Recommendation: &models.GcpRecommendation{
			CurrentCommitment:          &currCommitment,
			EstimatedAverageCoverage:   &cov,
			Policy:                     &policyId,
			PotentialAdditionalSavings: &savings,
			RecommendedCommitment:      &recCommitment,
			Region:                     &region,
			Service:                    &service,
		},
		EligibleUsage: &[]models.EligibleSpendDataPoint{
			{
				MaxUsage:    new(130.0),
				MedianUsage: new(120.0),
				MinUsage:    new(110.0),
				TotalUsage:  new(2880.0),
				UsageTime:   valueToNullable(usageTime),
			},
		},
	}

	var data ps4cGcpRecommendationDataSourceModel
	data.GcpService = types.StringValue("compute")

	diags := mapGcpRecommendationToModel(ctx, apiResp, &data)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if got := data.EstimatedEquivalentRecommendedCommitment.ValueFloat64(); got != 125.50 {
		t.Errorf("estimated_equivalent_recommended_commitment = %v, want 125.50", got)
	}
	if data.Recommendation.IsNull() {
		t.Fatalf("recommendation is null, want populated")
	}
	if got := data.Recommendation.RecommendedCommitment.ValueFloat64(); got != 120.00 {
		t.Errorf("recommendation.recommended_commitment = %v, want 120.00", got)
	}
	if got := data.Recommendation.Policy.ValueString(); got != "balanced" {
		t.Errorf("recommendation.policy = %q, want %q", got, "balanced")
	}
	if got := data.Granularity.ValueString(); got != "day" {
		t.Errorf("granularity default = %q, want 'day'", got)
	}
	if got := data.Region.ValueString(); got != "global" {
		t.Errorf("region default = %q, want 'global'", got)
	}

	elements := data.EligibleUsage.Elements()
	if len(elements) != 1 {
		t.Fatalf("eligible_usage elements count = %d, want 1", len(elements))
	}
	point, ok := elements[0].(datasource_ps4c_gcp_recommendation.EligibleUsageValue)
	if !ok {
		t.Fatalf("elements[0] is not EligibleUsageValue, got %T", elements[0])
	}
	if got := point.UsageTime.ValueString(); got != "2026-03-01T00:00:00Z" {
		t.Errorf("usage_time = %q, want %q", got, "2026-03-01T00:00:00Z")
	}
	if got := point.MaxUsage.ValueFloat64(); got != 130.0 {
		t.Errorf("max_usage = %v, want 130.0", got)
	}
}

func TestMapGcpRecommendationToModel_NullRecommendation(t *testing.T) {
	ctx := t.Context()

	estCommitment := 0.0
	apiResp := &models.GcpRecommendationWithEligibleSpend{
		EstimatedEquivalentRecommendedCommitment: &estCommitment,
		Recommendation:                           nil,
		EligibleUsage:                            &[]models.EligibleSpendDataPoint{},
	}

	var data ps4cGcpRecommendationDataSourceModel
	data.GcpService = types.StringValue("compute")

	diags := mapGcpRecommendationToModel(ctx, apiResp, &data)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if !data.Recommendation.IsNull() {
		t.Errorf("recommendation should be null")
	}
	if got := data.EstimatedEquivalentRecommendedCommitment.ValueFloat64(); got != 0.0 {
		t.Errorf("estimated_equivalent_recommended_commitment = %v, want 0.0", got)
	}
	if data.EligibleUsage.IsNull() {
		t.Errorf("eligible_usage should not be null, want empty list")
	}
	if got := len(data.EligibleUsage.Elements()); got != 0 {
		t.Errorf("eligible_usage elements count = %d, want 0", got)
	}
	if got := data.Region.ValueString(); got != "global" {
		t.Errorf("region default for compute = %q, want 'global'", got)
	}
}

func TestMapGcpRecommendationToModel_EmptyEligibleUsage(t *testing.T) {
	ctx := t.Context()

	apiResp := &models.GcpRecommendationWithEligibleSpend{
		EligibleUsage: nil,
	}

	var data ps4cGcpRecommendationDataSourceModel
	data.GcpService = types.StringValue("cloud_sql")

	diags := mapGcpRecommendationToModel(ctx, apiResp, &data)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if data.EligibleUsage.IsNull() {
		t.Errorf("eligible_usage should not be null when API returns nil")
	}
	if got := len(data.EligibleUsage.Elements()); got != 0 {
		t.Errorf("eligible_usage elements count = %d, want 0", got)
	}
}

func TestPs4cGcpRecommendationRead_Success(t *testing.T) {
	calls := 0
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/ps4commitments/v1/gcp/billing-accounts/012345-6789AB-CDEF01/recommendations/compute" {
			t.Errorf("path = %s, want /ps4commitments/v1/gcp/billing-accounts/012345-6789AB-CDEF01/recommendations/compute", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"estimatedEquivalentRecommendedCommitment": 50.0,
			"recommendation": {
				"currentCommitment": 40.0,
				"estimatedAverageCoverage": 0.82,
				"policy": "balanced",
				"potentialAdditionalSavings": 120.5,
				"recommendedCommitment": 50.0,
				"region": "global",
				"service": "compute"
			},
			"eligibleUsage": [
				{
					"maxUsage": 60.0,
					"medianUsage": 50.0,
					"minUsage": 40.0,
					"totalUsage": 1200.0,
					"usageTime": "2026-03-01T00:00:00Z"
				}
			]
		}`))
	}))

	resp := readPs4cGcpRecommendation(t, server, nil)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if calls != 1 {
		t.Fatalf("got %d calls, want 1", calls)
	}

	var state ps4cGcpRecommendationDataSourceModel
	if diags := resp.State.Get(t.Context(), &state); diags.HasError() {
		t.Fatal(diags)
	}

	if got := state.EstimatedEquivalentRecommendedCommitment.ValueFloat64(); got != 50.0 {
		t.Errorf("estimated_equivalent_recommended_commitment = %v, want 50.0", got)
	}
	if state.Recommendation.IsNull() {
		t.Fatalf("recommendation is null, want populated")
	}
	if got := state.Recommendation.RecommendedCommitment.ValueFloat64(); got != 50.0 {
		t.Errorf("recommendation.recommended_commitment = %v, want 50.0", got)
	}
	if got := state.Granularity.ValueString(); got != "day" {
		t.Errorf("granularity = %q, want 'day'", got)
	}
	if got := state.Region.ValueString(); got != "global" {
		t.Errorf("region = %q, want 'global'", got)
	}
	if got := len(state.EligibleUsage.Elements()); got != 1 {
		t.Errorf("eligible_usage count = %d, want 1", got)
	}
}

func TestPs4cGcpRecommendationRead_WithRegionAndGranularity(t *testing.T) {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ps4commitments/v1/gcp/billing-accounts/012345-6789AB-CDEF01/recommendations/cloud_sql" {
			t.Errorf("path = %s, want cloud_sql path", r.URL.Path)
		}
		if got := r.URL.Query().Get("region"); got != "us_central1" {
			t.Errorf("region query param = %q, want 'us_central1'", got)
		}
		if got := r.URL.Query().Get("granularity"); got != "hour" {
			t.Errorf("granularity query param = %q, want 'hour'", got)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"estimatedEquivalentRecommendedCommitment": 15.0,
			"recommendation": {
				"currentCommitment": 10.0,
				"estimatedAverageCoverage": 0.90,
				"policy": "custom-policy",
				"potentialAdditionalSavings": 45.0,
				"recommendedCommitment": 15.0,
				"region": "us_central1",
				"service": "cloud_sql"
			},
			"eligibleUsage": []
		}`))
	}))

	overrides := map[string]tftypes.Value{
		"gcp_service": tftypes.NewValue(tftypes.String, "cloud_sql"),
		"region":      tftypes.NewValue(tftypes.String, "us_central1"),
		"granularity": tftypes.NewValue(tftypes.String, "hour"),
	}

	resp := readPs4cGcpRecommendation(t, server, overrides)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}

	var state ps4cGcpRecommendationDataSourceModel
	if diags := resp.State.Get(t.Context(), &state); diags.HasError() {
		t.Fatal(diags)
	}

	if got := state.Region.ValueString(); got != "us_central1" {
		t.Errorf("region = %q, want 'us_central1'", got)
	}
	if got := state.Granularity.ValueString(); got != "hour" {
		t.Errorf("granularity = %q, want 'hour'", got)
	}
}

func TestPs4cGcpRecommendationRead_NotFound(t *testing.T) {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":"not_found","message":"recommendation not found"}`))
	}))

	resp := readPs4cGcpRecommendation(t, server, nil)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for 404, got none")
	}
	if !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "not found") {
		t.Errorf("expected error containing 'not found', got: %v", resp.Diagnostics)
	}
}

func TestPs4cGcpRecommendationRead_UnknownInput(t *testing.T) {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("API should not be called when billing_account_id is unknown")
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	}))

	overrides := map[string]tftypes.Value{
		"billing_account_id": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
	}

	resp := readPs4cGcpRecommendation(t, server, overrides)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	var state ps4cGcpRecommendationDataSourceModel
	if diags := resp.State.Get(t.Context(), &state); diags.HasError() {
		t.Fatal(diags)
	}

	if !state.EstimatedEquivalentRecommendedCommitment.IsUnknown() {
		t.Errorf("estimated_equivalent_recommended_commitment should be unknown")
	}
	if !state.Recommendation.IsUnknown() {
		t.Errorf("recommendation should be unknown")
	}
	if !state.EligibleUsage.IsUnknown() {
		t.Errorf("eligible_usage should be unknown")
	}
}

func TestPs4cGcpRecommendationRead_ServerError(t *testing.T) {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"code":"internal_error","message":"backend failure"}`))
	}))

	resp := readPs4cGcpRecommendation(t, server, nil)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for 500, got none")
	}
	if !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "500") {
		t.Errorf("expected error containing '500', got: %v", resp.Diagnostics)
	}
}

func readPs4cGcpRecommendation(t *testing.T, server *httptest.Server, overrides map[string]tftypes.Value) datasource.ReadResponse {
	t.Helper()
	ctx := t.Context()
	httpClient := server.Client()
	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}

	ds := NewPs4cGcpRecommendationDataSource()
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
		} else if name == "gcp_service" {
			values[name] = tftypes.NewValue(tfType, "compute")
		} else {
			values[name] = tftypes.NewValue(tfType, nil)
		}
	}

	resp := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	ds.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(tftypes.Object{AttributeTypes: attrTypes}, values)}}, &resp)
	return resp
}
