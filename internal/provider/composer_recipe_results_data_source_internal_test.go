package provider

import (
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_composer_recipe_results"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/oapi-codegen/nullable"
)

func TestComposerRecipeResultsDataSource_MetadataAndConfigure(t *testing.T) {
	ds := NewComposerRecipeResultsDataSource()

	var metaResp datasource.MetadataResponse
	ds.Metadata(t.Context(), datasource.MetadataRequest{ProviderTypeName: "doit"}, &metaResp)
	if metaResp.TypeName != "doit_composer_recipe_results" {
		t.Fatalf("TypeName = %q, want %q", metaResp.TypeName, "doit_composer_recipe_results")
	}

	var configResp datasource.ConfigureResponse
	ds.(datasource.DataSourceWithConfigure).Configure(t.Context(), datasource.ConfigureRequest{ProviderData: nil}, &configResp)
	if configResp.Diagnostics.HasError() {
		t.Fatalf("unexpected error on nil ProviderData: %v", configResp.Diagnostics)
	}

	ds.(datasource.DataSourceWithConfigure).Configure(t.Context(), datasource.ConfigureRequest{ProviderData: "wrong-type"}, &configResp)
	if !configResp.Diagnostics.HasError() {
		t.Fatal("expected error on wrong ProviderData type, got none")
	}
}

func TestMapComposerRecipeResultsItemsList(t *testing.T) {
	ctx := t.Context()

	t.Run("empty slice returns non-null empty list", func(t *testing.T) {
		list, diags := mapComposerRecipeResultsItemsList(ctx, nil)
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		if list.IsNull() || list.IsUnknown() {
			t.Fatalf("expected non-null known list, got %v", list)
		}
		if len(list.Elements()) != 0 {
			t.Fatalf("expected 0 elements, got %d", len(list.Elements()))
		}
	})

	t.Run("populated, empty-tags, and null-tags items", func(t *testing.T) {
		detTime := time.Date(2026, 3, 2, 16, 46, 39, 0, time.UTC)
		resTime := time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)
		items := []models.ComposerRecipeResult{
			{
				AccountId:                               "sub-123",
				CostEstimationPotentialFreedCommitments: new(1.5),
				CostEstimationPotentialIncreasedCommitmentsRequired: new(2.5),
				CostEstimationPotentialIncreasedCosts:               new(3.5),
				CostEstimationPotentialSavings:                      new(4.5),
				DetectionTime:                                       detTime,
				FirstFlowId:                                         new("flow-1"),
				Id:                                                  52815,
				IsActive:                                            true,
				IssuedResourceId:                                    "/subscriptions/sub-123/resourceGroups/rg/providers/Microsoft.Compute/disks/d1",
				IssuedResourceName:                                  "d1",
				LastFlowId:                                          new("flow-2"),
				Provider:                                            models.ComposerProviderAzure,
				ProviderIdentifier:                                  nullable.NewNullableWithValue("/subscriptions/sub-123/disks/d1"),
				Region:                                              new("eastus"),
				ResolvedTime:                                        &resTime,
				ResourceClass:                                       new("Microsoft.Compute/disks"),
				RuleId:                                              569,
				Snoozed:                                             new(true),
				Solved:                                              true,
				Tags:                                                nullable.NewNullableWithValue(map[string]string{"env": "prod"}),
			},
			{
				AccountId:          "sub-123",
				DetectionTime:      detTime,
				Id:                 52816,
				IsActive:           true,
				IssuedResourceId:   "res-2",
				IssuedResourceName: "res-2",
				Provider:           models.ComposerProviderAzure,
				ProviderIdentifier: nullable.NewNullNullable[string](),
				RuleId:             569,
				Solved:             false,
				Tags:               nullable.NewNullableWithValue(map[string]string{}),
			},
			{
				AccountId:          "sub-123",
				DetectionTime:      detTime,
				Id:                 52817,
				IsActive:           true,
				IssuedResourceId:   "res-3",
				IssuedResourceName: "res-3",
				Provider:           models.ComposerProviderAzure,
				ProviderIdentifier: nullable.NewNullNullable[string](),
				RuleId:             569,
				Solved:             false,
				Tags:               nullable.NewNullNullable[map[string]string](),
			},
		}

		list, diags := mapComposerRecipeResultsItemsList(ctx, items)
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}

		var got []datasource_composer_recipe_results.ItemsValue
		if d := list.ElementsAs(ctx, &got, false); d.HasError() {
			t.Fatalf("ElementsAs failed: %v", d)
		}
		if len(got) != 3 {
			t.Fatalf("expected 3 items, got %d", len(got))
		}

		first := got[0]
		if first.Id.ValueInt64() != 52815 {
			t.Errorf("first.Id = %d, want 52815", first.Id.ValueInt64())
		}
		if first.DetectionTime.ValueString() != "2026-03-02T16:46:39Z" {
			t.Errorf("first.DetectionTime = %q, want %q", first.DetectionTime.ValueString(), "2026-03-02T16:46:39Z")
		}
		if first.ResolvedTime.ValueString() != "2026-03-05T10:00:00Z" {
			t.Errorf("first.ResolvedTime = %q, want %q", first.ResolvedTime.ValueString(), "2026-03-05T10:00:00Z")
		}
		if first.ProviderIdentifier.ValueString() != "/subscriptions/sub-123/disks/d1" {
			t.Errorf("first.ProviderIdentifier = %q, want /subscriptions/sub-123/disks/d1", first.ProviderIdentifier.ValueString())
		}
		var firstTags map[string]string
		if d := first.Tags.ElementsAs(ctx, &firstTags, false); d.HasError() {
			t.Fatalf("Tags ElementsAs failed: %v", d)
		}
		if firstTags["env"] != "prod" {
			t.Errorf("first.Tags = %v, want map[env:prod]", firstTags)
		}

		second := got[1]
		if !second.ProviderIdentifier.IsNull() {
			t.Errorf("expected null ProviderIdentifier on second, got %v", second.ProviderIdentifier)
		}
		if !second.ResolvedTime.IsNull() {
			t.Errorf("expected null ResolvedTime on second, got %v", second.ResolvedTime)
		}
		if second.Tags.IsNull() || len(second.Tags.Elements()) != 0 {
			t.Errorf("expected non-null empty Tags map on second, got %v", second.Tags)
		}

		third := got[2]
		if !third.Tags.IsNull() {
			t.Errorf("expected null Tags map on third when API returns null, got %v", third.Tags)
		}
	})
}

func TestComposerRecipeResultsDataSource_UnknownGuard(t *testing.T) {
	for _, field := range []struct {
		name string
		val  tftypes.Value
	}{
		{"recipe_id", tftypes.NewValue(tftypes.Number, tftypes.UnknownValue)},
		{"filter", tftypes.NewValue(tftypes.String, tftypes.UnknownValue)},
		{"max_results", tftypes.NewValue(tftypes.Number, tftypes.UnknownValue)},
		{"page_token", tftypes.NewValue(tftypes.String, tftypes.UnknownValue)},
		{"sort_by", tftypes.NewValue(tftypes.String, tftypes.UnknownValue)},
		{"sort_order", tftypes.NewValue(tftypes.String, tftypes.UnknownValue)},
	} {
		t.Run(field.name, func(t *testing.T) {
			server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Fatalf("API should not be called when %s is unknown, received %s %s", field.name, r.Method, r.URL)
			}))

			overrides := map[string]tftypes.Value{
				"recipe_id": tftypes.NewValue(tftypes.Number, 569),
				field.name:  field.val,
			}
			resp := readComposerRecipeResults(t, server, overrides)
			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
			}

			var state composerRecipeResultsDataSourceModel
			if diags := resp.State.Get(t.Context(), &state); diags.HasError() {
				t.Fatal(diags)
			}
			if !state.Items.IsUnknown() {
				t.Errorf("expected Items to be unknown when %s is unknown, got %v", field.name, state.Items)
			}
			if !state.RowCount.IsUnknown() {
				t.Errorf("expected RowCount to be unknown when %s is unknown, got %v", field.name, state.RowCount)
			}
		})
	}
}

func TestComposerRecipeResultsDataSource_Read(t *testing.T) {
	t.Run("auto-pagination across two pages", func(t *testing.T) {
		var calls atomic.Int32
		server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/insights/v2/recipes/569/results" {
				t.Errorf("unexpected request: %s %s", r.Method, r.URL)
			}
			w.Header().Set("Content-Type", "application/json")
			n := calls.Add(1)
			if n == 1 {
				_, _ = w.Write([]byte(`{
					"items": [{
						"id": 52815,
						"ruleId": 569,
						"accountId": "sub-1",
						"issuedResourceId": "res-1",
						"issuedResourceName": "res-1",
						"provider": "azure",
						"providerIdentifier": "res-1",
						"detectionTime": "2026-03-02T16:46:39Z",
						"isActive": true,
						"solved": false,
						"tags": {}
					}],
					"pageToken": "next-tok",
					"rowCount": 2
				}`))
				return
			}
			_, _ = w.Write([]byte(`{
				"items": [{
					"id": 52816,
					"ruleId": 569,
					"accountId": "sub-1",
					"issuedResourceId": "res-2",
					"issuedResourceName": "res-2",
					"provider": "azure",
					"providerIdentifier": null,
					"detectionTime": "2026-03-02T16:46:39Z",
					"isActive": true,
					"solved": false,
					"tags": null
				}],
				"rowCount": 2
			}`))
		}))

		resp := readComposerRecipeResults(t, server, map[string]tftypes.Value{
			"recipe_id":  tftypes.NewValue(tftypes.Number, 569),
			"filter":     tftypes.NewValue(tftypes.String, "status:active"),
			"sort_by":    tftypes.NewValue(tftypes.String, "detection_time"),
			"sort_order": tftypes.NewValue(tftypes.String, "desc"),
		})
		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
		}
		if calls.Load() != 2 {
			t.Fatalf("expected 2 API calls, got %d", calls.Load())
		}

		var state composerRecipeResultsDataSourceModel
		if diags := resp.State.Get(t.Context(), &state); diags.HasError() {
			t.Fatal(diags)
		}
		if state.RowCount.ValueInt64() != 2 {
			t.Errorf("row_count = %d, want 2", state.RowCount.ValueInt64())
		}
		if !state.PageToken.IsNull() {
			t.Errorf("expected null page_token after auto-pagination, got %v", state.PageToken)
		}
		if len(state.Items.Elements()) != 2 {
			t.Fatalf("expected 2 items, got %d", len(state.Items.Elements()))
		}
	})

	t.Run("manual pagination with max_results", func(t *testing.T) {
		var calls atomic.Int32
		server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			if got := r.URL.Query().Get("maxResults"); got != "1" {
				t.Errorf("maxResults = %q, want 1", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"items": [{
					"id": 52815,
					"ruleId": 569,
					"accountId": "sub-1",
					"issuedResourceId": "res-1",
					"issuedResourceName": "res-1",
					"provider": "azure",
					"providerIdentifier": "res-1",
					"detectionTime": "2026-03-02T16:46:39Z",
					"isActive": true,
					"solved": false,
					"tags": {}
				}],
				"pageToken": "tok-page-2",
				"rowCount": 2
			}`))
		}))

		resp := readComposerRecipeResults(t, server, map[string]tftypes.Value{
			"recipe_id":   tftypes.NewValue(tftypes.Number, 569),
			"max_results": tftypes.NewValue(tftypes.Number, 1),
		})
		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
		}
		if calls.Load() != 1 {
			t.Fatalf("expected 1 API call in manual pagination mode, got %d", calls.Load())
		}

		var state composerRecipeResultsDataSourceModel
		if diags := resp.State.Get(t.Context(), &state); diags.HasError() {
			t.Fatal(diags)
		}
		if state.RowCount.ValueInt64() != 2 {
			t.Errorf("row_count = %d, want 2", state.RowCount.ValueInt64())
		}
		if state.PageToken.ValueString() != "tok-page-2" {
			t.Errorf("page_token = %q, want tok-page-2", state.PageToken.ValueString())
		}
		if len(state.Items.Elements()) != 1 {
			t.Fatalf("expected 1 item, got %d", len(state.Items.Elements()))
		}
	})

	t.Run("page_token-only auto-pagination sets row_count to fetched item count", func(t *testing.T) {
		var calls atomic.Int32
		server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			n := calls.Add(1)
			if n == 1 {
				if got := r.URL.Query().Get("pageToken"); got != "tok-after-1" {
					t.Errorf("call 1 pageToken = %q, want tok-after-1", got)
				}
				if got := r.URL.Query().Get("maxResults"); got != "" {
					t.Errorf("call 1 maxResults = %q, want empty", got)
				}
				// Upstream API reports total rowCount=3 across all pages even when starting at a pageToken.
				_, _ = w.Write([]byte(`{
					"items": [{
						"id": 52816,
						"ruleId": 569,
						"accountId": "sub-1",
						"issuedResourceId": "res-2",
						"issuedResourceName": "res-2",
						"provider": "azure",
						"providerIdentifier": "res-2",
						"detectionTime": "2026-03-02T16:46:39Z",
						"isActive": true,
						"solved": false,
						"tags": {}
					}],
					"pageToken": "tok-after-2",
					"rowCount": 3
				}`))
				return
			}
			if got := r.URL.Query().Get("pageToken"); got != "tok-after-2" {
				t.Errorf("call 2 pageToken = %q, want tok-after-2", got)
			}
			_, _ = w.Write([]byte(`{
				"items": [{
					"id": 52817,
					"ruleId": 569,
					"accountId": "sub-1",
					"issuedResourceId": "res-3",
					"issuedResourceName": "res-3",
					"provider": "azure",
					"providerIdentifier": "res-3",
					"detectionTime": "2026-03-02T16:46:39Z",
					"isActive": true,
					"solved": false,
					"tags": {}
				}],
				"rowCount": 3
			}`))
		}))

		resp := readComposerRecipeResults(t, server, map[string]tftypes.Value{
			"recipe_id":  tftypes.NewValue(tftypes.Number, 569),
			"page_token": tftypes.NewValue(tftypes.String, "tok-after-1"),
		})
		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
		}
		if calls.Load() != 2 {
			t.Fatalf("expected 2 API calls in page_token-only auto-pagination, got %d", calls.Load())
		}

		var state composerRecipeResultsDataSourceModel
		if diags := resp.State.Get(t.Context(), &state); diags.HasError() {
			t.Fatal(diags)
		}
		// Per provider-wide auto-pagination convention, row_count equals len(allItems) (2 remaining items),
		// not the upstream pre-pagination total (3).
		if state.RowCount.ValueInt64() != 2 {
			t.Errorf("row_count = %d, want 2 (len(allItems))", state.RowCount.ValueInt64())
		}
		if !state.PageToken.IsNull() {
			t.Errorf("expected null page_token after auto-pagination, got %v", state.PageToken)
		}
		if len(state.Items.Elements()) != 2 {
			t.Fatalf("expected 2 items, got %d", len(state.Items.Elements()))
		}
	})

	t.Run("server error 500", func(t *testing.T) {
		server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"code":"internal_error","message":"boom"}}`))
		}))

		resp := readComposerRecipeResults(t, server, map[string]tftypes.Value{
			"recipe_id": tftypes.NewValue(tftypes.Number, 569),
		})
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected error diagnostic on 500, got none")
		}
		if !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "500") {
			t.Fatalf("expected status 500 in error detail, got %v", resp.Diagnostics)
		}
	})
}

func readComposerRecipeResults(t *testing.T, server *httptest.Server, overrides map[string]tftypes.Value) datasource.ReadResponse {
	t.Helper()
	ctx := t.Context()
	httpClient := server.Client()
	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}

	ds := NewComposerRecipeResultsDataSource()
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
	maps.Copy(values, overrides)

	resp := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	ds.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(tftypes.Object{AttributeTypes: attrTypes}, values)}}, &resp)
	return resp
}
