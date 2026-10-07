package provider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_ps4c_commitment_policies"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestMapCommitmentPoliciesToItemsList_FullyPopulated(t *testing.T) {
	ctx := t.Context()

	policies := []models.CommitmentPolicy{
		{
			Id:                 "conservative",
			Name:               "Conservative",
			IsBuiltIn:          true,
			TargetCoverage:     65.0,
			LookbackDays:       60,
			LadderStepPercent:  5.0,
			LadderIntervalDays: 7,
			BootstrapPercent:   20.0,
		},
		{
			Id:                 "balanced",
			Name:               "Balanced",
			IsBuiltIn:          true,
			TargetCoverage:     80.0,
			LookbackDays:       60,
			LadderStepPercent:  10.0,
			LadderIntervalDays: 7,
			BootstrapPercent:   35.0,
		},
	}

	list, diags := mapCommitmentPoliciesToItemsList(ctx, policies)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	elements := list.Elements()
	if len(elements) != 2 {
		t.Fatalf("items length = %d, want 2", len(elements))
	}

	first, ok := elements[0].(datasource_ps4c_commitment_policies.ItemsValue)
	if !ok {
		t.Fatalf("elements[0] is not ItemsValue, got %T", elements[0])
	}
	if got := first.Id.ValueString(); got != "conservative" {
		t.Errorf("items[0].id = %q, want %q", got, "conservative")
	}
	if got := first.Name.ValueString(); got != "Conservative" {
		t.Errorf("items[0].name = %q, want %q", got, "Conservative")
	}
	if got := first.IsBuiltIn.ValueBool(); !got {
		t.Errorf("items[0].is_built_in = %v, want true", got)
	}
	if got := first.TargetCoverage.ValueFloat64(); got != 65.0 {
		t.Errorf("items[0].target_coverage = %v, want 65.0", got)
	}

	second, ok := elements[1].(datasource_ps4c_commitment_policies.ItemsValue)
	if !ok {
		t.Fatalf("elements[1] is not ItemsValue, got %T", elements[1])
	}
	if got := second.Id.ValueString(); got != "balanced" {
		t.Errorf("items[1].id = %q, want %q", got, "balanced")
	}
	if got := second.TargetCoverage.ValueFloat64(); got != 80.0 {
		t.Errorf("items[1].target_coverage = %v, want 80.0", got)
	}
}

func TestMapCommitmentPoliciesToItemsList_Empty(t *testing.T) {
	ctx := t.Context()

	list, diags := mapCommitmentPoliciesToItemsList(ctx, []models.CommitmentPolicy{})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if list.IsNull() {
		t.Errorf("items should be an empty list, not null")
	}
	if got := len(list.Elements()); got != 0 {
		t.Errorf("items length = %d, want 0", got)
	}
}

func TestPs4cCommitmentPoliciesRead_AutoPagination(t *testing.T) {
	var requests []*http.Request
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if r.URL.Query().Get("pageToken") == "page2-token" {
			_, _ = fmt.Fprint(w, `{
				"items": [
					{
						"id": "balanced",
						"name": "Balanced",
						"isBuiltIn": true,
						"targetCoverage": 80,
						"lookbackDays": 60,
						"ladderStepPercent": 10,
						"ladderIntervalDays": 7,
						"bootstrapPercent": 35
					}
				],
				"pageToken": null,
				"rowCount": 2
			}`)
			return
		}

		_, _ = fmt.Fprint(w, `{
			"items": [
				{
					"id": "conservative",
					"name": "Conservative",
					"isBuiltIn": true,
					"targetCoverage": 65,
					"lookbackDays": 60,
					"ladderStepPercent": 5,
					"ladderIntervalDays": 7,
					"bootstrapPercent": 20
				}
			],
			"pageToken": "page2-token",
			"rowCount": 2
		}`)
	}))

	resp := readPs4cCommitmentPolicies(t, server, nil)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	if len(requests) != 2 {
		t.Fatalf("expected 2 API calls for auto pagination, got %d", len(requests))
	}

	var state ps4cCommitmentPoliciesDataSourceModel
	if diags := resp.State.Get(t.Context(), &state); diags.HasError() {
		t.Fatal(diags)
	}

	if got := len(state.Items.Elements()); got != 2 {
		t.Errorf("items count = %d, want 2", got)
	}
	if got := state.RowCount.ValueInt64(); got != 2 {
		t.Errorf("row_count = %d, want 2", got)
	}
	if !state.PageToken.IsNull() {
		t.Errorf("page_token should be null in auto pagination mode, got %q", state.PageToken.ValueString())
	}
}

func TestPs4cCommitmentPoliciesRead_ManualPagination_MaxResults(t *testing.T) {
	var requests []*http.Request
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		_, _ = fmt.Fprint(w, `{
			"items": [
				{
					"id": "conservative",
					"name": "Conservative",
					"isBuiltIn": true,
					"targetCoverage": 65,
					"lookbackDays": 60,
					"ladderStepPercent": 5,
					"ladderIntervalDays": 7,
					"bootstrapPercent": 20
				}
			],
			"pageToken": "next-page-token",
			"rowCount": 3
		}`)
	}))

	resp := readPs4cCommitmentPolicies(t, server, map[string]tftypes.Value{
		"max_results": tftypes.NewValue(tftypes.Number, 1.0),
	})
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	if len(requests) != 1 {
		t.Fatalf("expected exactly 1 API call in manual mode, got %d", len(requests))
	}
	if got := requests[0].URL.Query().Get("maxResults"); got != "1" {
		t.Errorf("maxResults query param = %q, want %q", got, "1")
	}

	var state ps4cCommitmentPoliciesDataSourceModel
	if diags := resp.State.Get(t.Context(), &state); diags.HasError() {
		t.Fatal(diags)
	}

	if got := len(state.Items.Elements()); got != 1 {
		t.Errorf("items count = %d, want 1", got)
	}
	if got := state.RowCount.ValueInt64(); got != 3 {
		t.Errorf("row_count = %d, want 3", got)
	}
	if got := state.PageToken.ValueString(); got != "next-page-token" {
		t.Errorf("page_token = %q, want %q", got, "next-page-token")
	}
}

func TestPs4cCommitmentPoliciesRead_ManualPagination_PageTokenOnly(t *testing.T) {
	var requests []*http.Request
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if r.URL.Query().Get("pageToken") == "start-token" {
			_, _ = fmt.Fprint(w, `{
				"items": [
					{
						"id": "balanced",
						"name": "Balanced",
						"isBuiltIn": true,
						"targetCoverage": 80,
						"lookbackDays": 60,
						"ladderStepPercent": 10,
						"ladderIntervalDays": 7,
						"bootstrapPercent": 35
					}
				],
				"pageToken": "end-token",
				"rowCount": 2
			}`)
			return
		}

		_, _ = fmt.Fprint(w, `{
			"items": [
				{
					"id": "max_savings",
					"name": "Max Savings",
					"isBuiltIn": true,
					"targetCoverage": 95,
					"lookbackDays": 30,
					"ladderStepPercent": 15,
					"ladderIntervalDays": 7,
					"bootstrapPercent": 50
				}
			],
			"pageToken": null,
			"rowCount": 2
		}`)
	}))

	resp := readPs4cCommitmentPolicies(t, server, map[string]tftypes.Value{
		"page_token": tftypes.NewValue(tftypes.String, "start-token"),
	})
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	if len(requests) != 2 {
		t.Fatalf("expected 2 API calls while auto-paginating from start token, got %d", len(requests))
	}
	if got := requests[0].URL.Query().Get("pageToken"); got != "start-token" {
		t.Errorf("first pageToken query param = %q, want %q", got, "start-token")
	}
	if got := requests[1].URL.Query().Get("pageToken"); got != "end-token" {
		t.Errorf("second pageToken query param = %q, want %q", got, "end-token")
	}

	var state ps4cCommitmentPoliciesDataSourceModel
	if diags := resp.State.Get(t.Context(), &state); diags.HasError() {
		t.Fatal(diags)
	}

	if got := len(state.Items.Elements()); got != 2 {
		t.Errorf("items count = %d, want 2", got)
	}
	if !state.PageToken.IsNull() {
		t.Errorf("page_token should be null after auto-pagination completes, got %q", state.PageToken.ValueString())
	}
}

func TestPs4cCommitmentPoliciesRead_ManualPagination_Both(t *testing.T) {
	var requests []*http.Request
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		_, _ = fmt.Fprint(w, `{
			"items": [
				{
					"id": "balanced",
					"name": "Balanced",
					"isBuiltIn": true,
					"targetCoverage": 80,
					"lookbackDays": 60,
					"ladderStepPercent": 10,
					"ladderIntervalDays": 7,
					"bootstrapPercent": 35
				}
			],
			"pageToken": "next-token-after-both",
			"rowCount": 5
		}`)
	}))

	resp := readPs4cCommitmentPolicies(t, server, map[string]tftypes.Value{
		"max_results": tftypes.NewValue(tftypes.Number, 1.0),
		"page_token":  tftypes.NewValue(tftypes.String, "input-page-token"),
	})
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	if len(requests) != 1 {
		t.Fatalf("expected exactly 1 API call in manual mode with both inputs, got %d", len(requests))
	}
	if got := requests[0].URL.Query().Get("maxResults"); got != "1" {
		t.Errorf("maxResults query param = %q, want %q", got, "1")
	}
	if got := requests[0].URL.Query().Get("pageToken"); got != "input-page-token" {
		t.Errorf("pageToken query param = %q, want %q", got, "input-page-token")
	}

	var state ps4cCommitmentPoliciesDataSourceModel
	if diags := resp.State.Get(t.Context(), &state); diags.HasError() {
		t.Fatal(diags)
	}

	if got := len(state.Items.Elements()); got != 1 {
		t.Errorf("items count = %d, want 1", got)
	}
	if got := state.RowCount.ValueInt64(); got != 5 {
		t.Errorf("row_count = %d, want 5", got)
	}
	if got := state.PageToken.ValueString(); got != "next-token-after-both" {
		t.Errorf("page_token = %q, want %q", got, "next-token-after-both")
	}
}

func TestPs4cCommitmentPoliciesRead_UnknownInputs(t *testing.T) {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("API should not be called when inputs are unknown")
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	}))

	resp := readPs4cCommitmentPolicies(t, server, map[string]tftypes.Value{
		"max_results": tftypes.NewValue(tftypes.Number, tftypes.UnknownValue),
	})
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	var state ps4cCommitmentPoliciesDataSourceModel
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

func TestPs4cCommitmentPoliciesRead_Error(t *testing.T) {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"internal server error"}`))
	}))

	resp := readPs4cCommitmentPolicies(t, server, nil)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for 500, got none")
	}
	if !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "status 500") {
		t.Errorf("expected error containing 'status 500', got: %v", resp.Diagnostics)
	}
}

func readPs4cCommitmentPolicies(t *testing.T, server *httptest.Server, overrides map[string]tftypes.Value) datasource.ReadResponse {
	t.Helper()
	ctx := t.Context()
	httpClient := server.Client()
	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}

	ds := NewPs4cCommitmentPoliciesDataSource()
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
		} else {
			values[name] = tftypes.NewValue(tfType, nil)
		}
	}

	resp := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	ds.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(tftypes.Object{AttributeTypes: attrTypes}, values)}}, &resp)
	return resp
}
