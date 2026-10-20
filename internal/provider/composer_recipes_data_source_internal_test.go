package provider

import (
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_composer_recipes"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestComposerRecipesDataSource_MetadataAndConfigure(t *testing.T) {
	ds := NewComposerRecipesDataSource()

	var metaResp datasource.MetadataResponse
	ds.Metadata(t.Context(), datasource.MetadataRequest{ProviderTypeName: "doit"}, &metaResp)
	if metaResp.TypeName != "doit_composer_recipes" {
		t.Fatalf("TypeName = %q, want %q", metaResp.TypeName, "doit_composer_recipes")
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

func TestMapComposerRecipesItemsList(t *testing.T) {
	ctx := t.Context()

	t.Run("empty slice returns non-null empty list", func(t *testing.T) {
		list, diags := mapComposerRecipesItemsList(ctx, nil)
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

	t.Run("populated and minimal items", func(t *testing.T) {
		detTime := time.Date(2026, 3, 2, 16, 46, 39, 0, time.UTC)
		items := []models.ComposerRecipe{
			{
				ActiveResultsCount:      2,
				AffectedResourceType:    new("Microsoft.Compute/disks"),
				Description:             new("Detailed description"),
				DetectionTime:           &detTime,
				EstimatedFixTimeMinutes: new(15),
				Id:                      569,
				IsDismissed:             false,
				Metadata: models.ComposerRecipeMetadata{
					Impacts:           new("Low impact"),
					ImportantPoints:   &[]string{"Point 1", "Point 2"},
					ManualRemediation: new("<p>Remediate</p>"),
					Reference:         new("https://example.com/docs"),
				},
				Provider:              models.ComposerProviderAzure,
				ResolvedResultsCount:  1,
				Severity:              models.ComposerRecipeSeverityLow,
				SnoozedResultsCount:   0,
				Status:                new(models.ComposerRecipeStatusActive),
				Title:                 "Restore Points should not sit idle",
				TotalPotentialSavings: new("12.34"),
				TotalResultsCount:     3,
			},
			{
				ActiveResultsCount:   0,
				Id:                   736,
				IsDismissed:          true,
				Metadata:             models.ComposerRecipeMetadata{},
				Provider:             models.ComposerProviderAws,
				ResolvedResultsCount: 0,
				Severity:             models.ComposerRecipeSeverityCritical,
				SnoozedResultsCount:  0,
				Title:                "Minimal recipe",
				TotalResultsCount:    0,
			},
		}

		list, diags := mapComposerRecipesItemsList(ctx, items)
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}

		var got []datasource_composer_recipes.ItemsValue
		if d := list.ElementsAs(ctx, &got, false); d.HasError() {
			t.Fatalf("ElementsAs failed: %v", d)
		}
		if len(got) != 2 {
			t.Fatalf("expected 2 items, got %d", len(got))
		}

		first := got[0]
		if first.Id.ValueInt64() != 569 {
			t.Errorf("first.Id = %d, want 569", first.Id.ValueInt64())
		}
		if first.DetectionTime.ValueString() != "2026-03-02T16:46:39Z" {
			t.Errorf("first.DetectionTime = %q, want %q", first.DetectionTime.ValueString(), "2026-03-02T16:46:39Z")
		}
		if first.EstimatedFixTimeMinutes.ValueInt64() != 15 {
			t.Errorf("first.EstimatedFixTimeMinutes = %d, want 15", first.EstimatedFixTimeMinutes.ValueInt64())
		}
		if first.Status.ValueString() != "active" {
			t.Errorf("first.Status = %q, want %q", first.Status.ValueString(), "active")
		}
		if first.TotalPotentialSavings.ValueString() != "12.34" {
			t.Errorf("first.TotalPotentialSavings = %q, want %q", first.TotalPotentialSavings.ValueString(), "12.34")
		}
		var pts []string
		if d := first.Metadata.ImportantPoints.ElementsAs(ctx, &pts, false); d.HasError() {
			t.Fatalf("ImportantPoints ElementsAs failed: %v", d)
		}
		if len(pts) != 2 || pts[0] != "Point 1" || pts[1] != "Point 2" {
			t.Errorf("first.Metadata.ImportantPoints = %v, want [Point 1, Point 2]", pts)
		}

		second := got[1]
		if second.Id.ValueInt64() != 736 {
			t.Errorf("second.Id = %d, want 736", second.Id.ValueInt64())
		}
		if !second.DetectionTime.IsNull() {
			t.Errorf("expected null DetectionTime, got %v", second.DetectionTime)
		}
		if !second.EstimatedFixTimeMinutes.IsNull() {
			t.Errorf("expected null EstimatedFixTimeMinutes, got %v", second.EstimatedFixTimeMinutes)
		}
		if !second.Status.IsNull() {
			t.Errorf("expected null Status, got %v", second.Status)
		}
		if second.Metadata.ImportantPoints.IsNull() || len(second.Metadata.ImportantPoints.Elements()) != 0 {
			t.Errorf("expected empty non-null ImportantPoints list, got %v", second.Metadata.ImportantPoints)
		}
	})
}

func TestComposerRecipesDataSource_UnknownGuard(t *testing.T) {
	for _, field := range []struct {
		name string
		val  tftypes.Value
	}{
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

			resp := readComposerRecipes(t, server, map[string]tftypes.Value{
				field.name: field.val,
			})
			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
			}

			var state composerRecipesDataSourceModel
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

func TestComposerRecipesDataSource_Read(t *testing.T) {
	t.Run("auto-pagination across two pages", func(t *testing.T) {
		var calls atomic.Int32
		server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/insights/v2/recipes" {
				t.Errorf("unexpected request: %s %s", r.Method, r.URL)
			}
			w.Header().Set("Content-Type", "application/json")
			n := calls.Add(1)
			if n == 1 {
				if got := r.URL.Query().Get("pageToken"); got != "" {
					t.Errorf("page 1 pageToken = %q, want empty", got)
				}
				_, _ = w.Write([]byte(`{
					"items": [{
						"id": 569,
						"title": "Recipe 1",
						"provider": "azure",
						"severity": "low",
						"isDismissed": false,
						"activeResultsCount": 1,
						"snoozedResultsCount": 0,
						"resolvedResultsCount": 0,
						"totalResultsCount": 1,
						"metadata": {}
					}],
					"pageToken": "next-tok",
					"rowCount": 2
				}`))
				return
			}
			if got := r.URL.Query().Get("pageToken"); got != "next-tok" {
				t.Errorf("page 2 pageToken = %q, want next-tok", got)
			}
			_, _ = w.Write([]byte(`{
				"items": [{
					"id": 736,
					"title": "Recipe 2",
					"provider": "azure",
					"severity": "medium",
					"isDismissed": false,
					"activeResultsCount": 2,
					"snoozedResultsCount": 0,
					"resolvedResultsCount": 0,
					"totalResultsCount": 2,
					"metadata": {"importantPoints": ["Check A"]}
				}],
				"rowCount": 2
			}`))
		}))

		resp := readComposerRecipes(t, server, map[string]tftypes.Value{
			"filter":     tftypes.NewValue(tftypes.String, "provider:azure"),
			"sort_by":    tftypes.NewValue(tftypes.String, "severity"),
			"sort_order": tftypes.NewValue(tftypes.String, "desc"),
		})
		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
		}
		if calls.Load() != 2 {
			t.Fatalf("expected 2 API calls, got %d", calls.Load())
		}

		var state composerRecipesDataSourceModel
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
					"id": 569,
					"title": "Recipe 1",
					"provider": "azure",
					"severity": "low",
					"isDismissed": false,
					"activeResultsCount": 1,
					"snoozedResultsCount": 0,
					"resolvedResultsCount": 0,
					"totalResultsCount": 1,
					"metadata": {}
				}],
				"pageToken": "tok-page-2",
				"rowCount": 28
			}`))
		}))

		resp := readComposerRecipes(t, server, map[string]tftypes.Value{
			"max_results": tftypes.NewValue(tftypes.Number, 1),
		})
		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
		}
		if calls.Load() != 1 {
			t.Fatalf("expected 1 API call in manual pagination mode, got %d", calls.Load())
		}

		var state composerRecipesDataSourceModel
		if diags := resp.State.Get(t.Context(), &state); diags.HasError() {
			t.Fatal(diags)
		}
		if state.RowCount.ValueInt64() != 28 {
			t.Errorf("row_count = %d, want 28", state.RowCount.ValueInt64())
		}
		if state.PageToken.ValueString() != "tok-page-2" {
			t.Errorf("page_token = %q, want tok-page-2", state.PageToken.ValueString())
		}
		if len(state.Items.Elements()) != 1 {
			t.Fatalf("expected 1 item, got %d", len(state.Items.Elements()))
		}
	})

	t.Run("server error 500", func(t *testing.T) {
		server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"code":"internal_error","message":"boom"}}`))
		}))

		resp := readComposerRecipes(t, server, nil)
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected error diagnostic on 500, got none")
		}
		if !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "500") {
			t.Fatalf("expected status 500 in error detail, got %v", resp.Diagnostics)
		}
	})
}

func readComposerRecipes(t *testing.T, server *httptest.Server, overrides map[string]tftypes.Value) datasource.ReadResponse {
	t.Helper()
	ctx := t.Context()
	httpClient := server.Client()
	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}

	ds := NewComposerRecipesDataSource()
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
