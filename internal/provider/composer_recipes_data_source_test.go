package provider_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

var (
	composerRecipeInfoOnce         sync.Once
	composerRecipeTotalCount       int
	composerRecipeWithResultsID    int64
	composerRecipeWithResultsCount int
	composerRecipeInfoErr          error
)

// selectComposerRecipeTestFixture selects a recipe and its result count by querying
// the results endpoint with default filters (last_scan_only:true). A positive
// TotalResultsCount on the recipe summary may include resolved findings from earlier
// scans that are excluded by default on the results endpoint.
func selectComposerRecipeTestFixture(
	ctx context.Context,
	client *models.ClientWithResponses,
	recipes []models.ComposerRecipe,
) (recipeID int64, resultsCount int, err error) {
	for _, item := range recipes {
		if item.ActiveResultsCount == 0 && item.TotalResultsCount == 0 {
			continue
		}
		resResp, resErr := client.ListComposerRecipeResultsWithResponse(ctx, item.Id, &models.ListComposerRecipeResultsParams{
			MaxResults: new(2),
		})
		if resErr != nil {
			return 0, 0, fmt.Errorf("failed to list results for recipe %d: %w", item.Id, resErr)
		}
		if resResp.JSON200 == nil {
			return 0, 0, fmt.Errorf("unexpected status %d for recipe %d results: %s", resResp.StatusCode(), item.Id, string(resResp.Body))
		}
		if len(resResp.JSON200.Items) == 0 || resResp.JSON200.RowCount == 0 {
			continue
		}
		count := int(resResp.JSON200.RowCount)
		if count >= 2 && len(resResp.JSON200.Items) >= 2 {
			return item.Id, count, nil
		}
		if recipeID == 0 {
			recipeID = item.Id
			resultsCount = count
		}
	}
	return recipeID, resultsCount, nil
}

func initComposerRecipeTestInfo(t *testing.T) {
	t.Helper()
	composerRecipeInfoOnce.Do(func() {
		client := getAPIClient(t)
		resp, err := client.ListComposerRecipesWithResponse(t.Context(), &models.ListComposerRecipesParams{
			MaxResults: new(50),
		})
		if err != nil {
			composerRecipeInfoErr = fmt.Errorf("failed to list composer recipes: %w", err)
			return
		}
		if resp.JSON200 == nil {
			composerRecipeInfoErr = fmt.Errorf("unexpected status %d: %s", resp.StatusCode(), string(resp.Body))
			return
		}
		composerRecipeTotalCount = int(resp.JSON200.RowCount)
		composerRecipeWithResultsID, composerRecipeWithResultsCount, composerRecipeInfoErr = selectComposerRecipeTestFixture(
			t.Context(),
			client,
			resp.JSON200.Items,
		)
	})
	if composerRecipeInfoErr != nil {
		t.Fatalf("Composer recipes pre-check failed: %v", composerRecipeInfoErr)
	}
}

func TestSelectComposerRecipeTestFixture(t *testing.T) {
	t.Run("skips recipe with only older resolved findings and uses default results query count", func(t *testing.T) {
		server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/insights/v2/recipes/100/results":
				// Recipe 100 has TotalResultsCount=3 from an older scan, but 0 rows under default last_scan_only:true.
				_, _ = w.Write([]byte(`{"items": [], "rowCount": 0}`))
			case "/insights/v2/recipes/200/results":
				// Recipe 200 has TotalResultsCount=3 on summary, but only 1 row under default last_scan_only:true.
				_, _ = w.Write([]byte(`{"items": [{"id": 1, "ruleId": 200, "accountId": "a", "issuedResourceId": "r1", "issuedResourceName": "r1", "provider": "azure", "detectionTime": "2026-03-02T16:46:39Z", "isActive": true, "solved": false}], "rowCount": 1}`))
			default:
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
		}))

		client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(server.Client()))
		if err != nil {
			t.Fatal(err)
		}

		recipes := []models.ComposerRecipe{
			{Id: 100, ActiveResultsCount: 0, ResolvedResultsCount: 3, TotalResultsCount: 3},
			{Id: 200, ActiveResultsCount: 1, ResolvedResultsCount: 2, TotalResultsCount: 3},
		}

		gotID, gotCount, err := selectComposerRecipeTestFixture(t.Context(), client, recipes)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotID != 200 {
			t.Errorf("recipeID = %d, want 200 (recipe 100 has 0 default results)", gotID)
		}
		if gotCount != 1 {
			t.Errorf("resultsCount = %d, want 1 (default query rowCount, not summary TotalResultsCount=3)", gotCount)
		}
	})

	t.Run("prefers recipe with at least 2 default results for pagination tests", func(t *testing.T) {
		server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/insights/v2/recipes/200/results":
				_, _ = w.Write([]byte(`{"items": [{"id": 1, "ruleId": 200, "accountId": "a", "issuedResourceId": "r1", "issuedResourceName": "r1", "provider": "azure", "detectionTime": "2026-03-02T16:46:39Z", "isActive": true, "solved": false}], "rowCount": 1}`))
			case "/insights/v2/recipes/300/results":
				_, _ = w.Write([]byte(`{"items": [
					{"id": 2, "ruleId": 300, "accountId": "a", "issuedResourceId": "r2", "issuedResourceName": "r2", "provider": "azure", "detectionTime": "2026-03-02T16:46:39Z", "isActive": true, "solved": false},
					{"id": 3, "ruleId": 300, "accountId": "a", "issuedResourceId": "r3", "issuedResourceName": "r3", "provider": "azure", "detectionTime": "2026-03-02T16:46:39Z", "isActive": true, "solved": false}
				], "rowCount": 2}`))
			default:
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
		}))

		client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(server.Client()))
		if err != nil {
			t.Fatal(err)
		}

		recipes := []models.ComposerRecipe{
			{Id: 200, ActiveResultsCount: 1, TotalResultsCount: 1},
			{Id: 300, ActiveResultsCount: 2, TotalResultsCount: 2},
		}

		gotID, gotCount, err := selectComposerRecipeTestFixture(t.Context(), client, recipes)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotID != 300 || gotCount != 2 {
			t.Errorf("got (%d, %d), want (300, 2)", gotID, gotCount)
		}
	})
}

func TestAccComposerRecipesDataSource_AutoPagination(t *testing.T) {
	initComposerRecipeTestInfo(t)
	if composerRecipeTotalCount == 0 {
		t.Skip("No Composer recipes in test account")
	}

	expectedCountStr := strconv.Itoa(composerRecipeTotalCount)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		Steps: []resource.TestStep{
			{
				Config: `data "doit_composer_recipes" "test" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_composer_recipes.test", "row_count", expectedCountStr),
					resource.TestCheckResourceAttr("data.doit_composer_recipes.test", "items.#", expectedCountStr),
					resource.TestCheckResourceAttrSet("data.doit_composer_recipes.test", "items.0.id"),
					resource.TestCheckResourceAttrSet("data.doit_composer_recipes.test", "items.0.title"),
					resource.TestCheckResourceAttrSet("data.doit_composer_recipes.test", "items.0.provider"),
					resource.TestCheckResourceAttrSet("data.doit_composer_recipes.test", "items.0.severity"),
					resource.TestCheckResourceAttrSet("data.doit_composer_recipes.test", "items.0.metadata.important_points.#"),
					resource.TestCheckNoResourceAttr("data.doit_composer_recipes.test", "page_token"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccComposerRecipesDataSource_MaxResultsOnly(t *testing.T) {
	initComposerRecipeTestInfo(t)
	if composerRecipeTotalCount < 2 {
		t.Skipf("Need at least 2 Composer recipes to test pagination, got %d", composerRecipeTotalCount)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_composer_recipes" "limited" {
  max_results = 1
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_composer_recipes.limited", "items.#", "1"),
					resource.TestCheckResourceAttrSet("data.doit_composer_recipes.limited", "items.0.id"),
					resource.TestCheckResourceAttrSet("data.doit_composer_recipes.limited", "row_count"),
					resource.TestCheckResourceAttrSet("data.doit_composer_recipes.limited", "page_token"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccComposerRecipesDataSource_PageTokenOnly(t *testing.T) {
	initComposerRecipeTestInfo(t)
	if composerRecipeTotalCount < 2 {
		t.Skipf("Need at least 2 Composer recipes to test page_token, got %d", composerRecipeTotalCount)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_composer_recipes" "first_page" {
  max_results = 1
}

data "doit_composer_recipes" "from_token" {
  page_token = data.doit_composer_recipes.first_page.page_token
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_composer_recipes.first_page", "items.#", "1"),
					resource.TestCheckResourceAttr("data.doit_composer_recipes.first_page", "row_count", strconv.Itoa(composerRecipeTotalCount)),
					resource.TestCheckResourceAttrSet("data.doit_composer_recipes.first_page", "page_token"),
					testCheckResourceAttrLessThan("data.doit_composer_recipes.from_token", "row_count", composerRecipeTotalCount),
					resource.TestCheckResourceAttr("data.doit_composer_recipes.from_token", "row_count", strconv.Itoa(composerRecipeTotalCount-1)),
					resource.TestCheckResourceAttr("data.doit_composer_recipes.from_token", "items.#", strconv.Itoa(composerRecipeTotalCount-1)),
					resource.TestCheckNoResourceAttr("data.doit_composer_recipes.from_token", "page_token"),
					testCheckResourceAttrNotEqualAttr(
						"data.doit_composer_recipes.first_page", "items.0.id",
						"data.doit_composer_recipes.from_token", "items.0.id",
					),
				),
			},
		},
	})
}

func TestAccComposerRecipesDataSource_MaxResultsAndPageToken(t *testing.T) {
	initComposerRecipeTestInfo(t)
	if composerRecipeTotalCount < 2 {
		t.Skipf("Need at least 2 Composer recipes to test pagination, got %d", composerRecipeTotalCount)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_composer_recipes" "page1" {
  max_results = 1
}

data "doit_composer_recipes" "page2" {
  max_results = 1
  page_token  = data.doit_composer_recipes.page1.page_token
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_composer_recipes.page1", "items.#", "1"),
					resource.TestCheckResourceAttr("data.doit_composer_recipes.page2", "items.#", "1"),
					testCheckResourceAttrNotEqualAttr(
						"data.doit_composer_recipes.page1", "items.0.id",
						"data.doit_composer_recipes.page2", "items.0.id",
					),
				),
			},
		},
	})
}

func TestAccComposerRecipesDataSource_FilterAndSort(t *testing.T) {
	initComposerRecipeTestInfo(t)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_composer_recipes" "filtered" {
  filter     = "has_active_results:true"
  sort_by    = "severity"
  sort_order = "desc"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_composer_recipes.filtered", "filter", "has_active_results:true"),
					resource.TestCheckResourceAttr("data.doit_composer_recipes.filtered", "sort_by", "severity"),
					resource.TestCheckResourceAttr("data.doit_composer_recipes.filtered", "sort_order", "desc"),
					resource.TestCheckResourceAttrSet("data.doit_composer_recipes.filtered", "row_count"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccComposerRecipesDataSource_EmptyResult(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_composer_recipes" "empty" {
  filter = "provider:vmware"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_composer_recipes.empty", "row_count", "0"),
					resource.TestCheckResourceAttr("data.doit_composer_recipes.empty", "items.#", "0"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}
