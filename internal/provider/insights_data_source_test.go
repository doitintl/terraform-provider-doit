package provider_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// TestAccInsightsDataSource_AutoPagination tests that without max_results,
// all insights are fetched via auto-pagination.
func TestAccInsightsDataSource_AutoPagination(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccInsightsDataSourceAutoConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_insights.test", "pagination.row_count"),
					resource.TestCheckResourceAttrSet("data.doit_insights.test", "results.#"),
				),
			},
			// Drift verification
			{
				Config: testAccInsightsDataSourceAutoConfig(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccInsightsDataSourceAutoConfig() string {
	return `
data "doit_insights" "test" {
}
`
}

// TestAccInsightsDataSource_MaxResultsOnly tests that setting max_results
// limits results and returns a page_token for the next page.
func TestAccInsightsDataSource_MaxResultsOnly(t *testing.T) {
	insightCount := getInsightCount(t)
	if insightCount < 2 {
		t.Skipf("Need at least 2 insights to test pagination, got %d", insightCount)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccInsightsDataSourceMaxResultsConfig(1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_insights.limited", "results.#", "1"),
					resource.TestCheckResourceAttrSet("data.doit_insights.limited", "pagination.page_token"),
				),
			},
			// Drift verification
			{
				Config: testAccInsightsDataSourceMaxResultsConfig(1),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccInsightsDataSourceMaxResultsConfig(maxResults int64) string {
	return fmt.Sprintf(`
data "doit_insights" "limited" {
  max_results = %d
}
`, maxResults)
}

// TestAccInsightsDataSource_FilterByCategory tests filtering by category.
func TestAccInsightsDataSource_FilterByCategory(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccInsightsDataSourceCategoryConfig("FinOps"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_insights.by_category", "results.#"),
					resource.TestCheckResourceAttrSet("data.doit_insights.by_category", "pagination.row_count"),
				),
			},
		},
	})
}

func testAccInsightsDataSourceCategoryConfig(category string) string {
	return fmt.Sprintf(`
data "doit_insights" "by_category" {
  category = %[1]q
}
`, category)
}

// TestAccInsightsDataSource_WithResource creates an insight, then verifies it
// appears in the list data source.
func TestAccInsightsDataSource_WithResource(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-insights-list")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccInsightsDataSourceWithResourceConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_insights.filtered", "results.#"),
				),
			},
		},
	})
}

func testAccInsightsDataSourceWithResourceConfig(rName string) string {
	return fmt.Sprintf(`
resource "doit_insight" "test" {
  key               = %[1]q
  title             = "List DS Test"
  short_description = "Created for list test"
  cloud_provider    = "gcp"
  categories        = ["FinOps"]
}

data "doit_insights" "filtered" {
  search_term = %[1]q
  depends_on  = [doit_insight.test]
}
`, rName)
}

// Helper: count total insights for pagination tests

var (
	insightCount     int
	insightCountOnce sync.Once
)

func getInsightCount(t *testing.T) int {
	t.Helper()
	insightCountOnce.Do(func() {
		insightCount = computeInsightCount(t)
	})
	return insightCount
}

func computeInsightCount(t *testing.T) int {
	t.Helper()
	client := getAPIClient(t)
	ctx := context.Background()

	var total int
	params := &models.GetInsightResultsParams{}
	for {
		resp, err := client.GetInsightResultsWithResponse(ctx, params)
		if err != nil {
			t.Fatalf("Failed to list insights: %v", err)
		}
		if resp.JSON200 == nil || resp.JSON200.Results == nil {
			break
		}
		total += len(*resp.JSON200.Results)

		if resp.JSON200.Pagination == nil || resp.JSON200.Pagination.PageToken == nil || *resp.JSON200.Pagination.PageToken == "" {
			break
		}
		params.PageToken = resp.JSON200.Pagination.PageToken
	}
	return total
}
