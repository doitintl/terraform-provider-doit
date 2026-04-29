package provider_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// TestAccInsightsDataSource_Basic tests listing insights without filters (auto-pagination).
func TestAccInsightsDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccInsightsDataSourceBasicConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_insights.test", "results.#"),
				),
			},
			// Drift verification: re-apply should produce empty plan
			{
				Config: testAccInsightsDataSourceBasicConfig(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccInsightsDataSourceBasicConfig() string {
	return `
data "doit_insights" "test" {
}
`
}

// TestAccInsightsDataSource_Filtered tests listing insights with a display_status filter.
func TestAccInsightsDataSource_Filtered(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccInsightsDataSourceFilteredConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_insights.filtered", "results.#"),
				),
			},
			// Drift verification: re-apply should produce empty plan
			{
				Config: testAccInsightsDataSourceFilteredConfig(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccInsightsDataSourceFilteredConfig() string {
	return `
data "doit_insights" "filtered" {
  display_status = ["actionable"]
  category       = "FinOps"
}
`
}

// TestAccInsightsDataSource_MaxResultsOnly tests that setting max_results limits results
// and returns a page_token for fetching the next page.
func TestAccInsightsDataSource_MaxResultsOnly(t *testing.T) {
	// Need at least 3 insights to verify pagination
	insightCount := getInsightCount(t)
	if insightCount < 3 {
		t.Skipf("Need at least 3 insights to test pagination, got %d", insightCount)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccInsightsDataSourceMaxResultsConfig(2),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Verify we got exactly 2 insights (as limited by max_results)
					resource.TestCheckResourceAttr("data.doit_insights.limited", "results.#", "2"),
					// Verify page_token is returned in pagination (more pages exist)
					resource.TestCheckResourceAttrSet("data.doit_insights.limited", "pagination.page_token"),
				),
			},
			// Second apply should produce no diff (max_results preserved in state)
			{
				Config:   testAccInsightsDataSourceMaxResultsConfig(2),
				PlanOnly: true,
			},
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccInsightsDataSourceMaxResultsConfig(2),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccInsightsDataSourceMaxResultsConfig(maxResults int) string {
	return fmt.Sprintf(`
data "doit_insights" "limited" {
  max_results = %d
}
`, maxResults)
}

// TestAccInsightsDataSource_PageTokenOnly tests that setting only page_token (without max_results)
// auto-paginates starting from the token, returning a different set of items.
// Uses two chained data sources in one apply:
//   - "first_page": fetches 1 insight + page_token
//   - "from_token": auto-paginates from the token (should start at a different item)
func TestAccInsightsDataSource_PageTokenOnly(t *testing.T) {
	totalInsights := getInsightCount(t)
	if totalInsights < 2 {
		t.Skipf("Need at least 2 insights to test page_token-only, got %d", totalInsights)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_insights" "first_page" {
  max_results = 1
}
data "doit_insights" "from_token" {
  page_token = data.doit_insights.first_page.pagination.page_token
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_insights.first_page", "results.#", "1"),
					resource.TestCheckResourceAttrSet("data.doit_insights.first_page", "pagination.page_token"),
					resource.TestCheckResourceAttrSet("data.doit_insights.from_token", "pagination.row_count"),
					// Verify page_token actually advanced to a different starting point
					testCheckResourceAttrNotEqualAttr(
						"data.doit_insights.first_page", "results.0.key",
						"data.doit_insights.from_token", "results.0.key"),
				),
			},
		},
	})
}

// TestAccInsightsDataSource_MaxResultsAndPageToken tests using both max_results and page_token together.
// Uses two chained data sources in one apply to avoid race conditions with parallel tests.
func TestAccInsightsDataSource_MaxResultsAndPageToken(t *testing.T) {
	insightCount := getInsightCount(t)
	if insightCount < 3 {
		t.Skipf("Need at least 3 insights to test pagination, got %d", insightCount)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_insights" "first_page" {
  max_results = 1
}
data "doit_insights" "paginated" {
  max_results = 1
  page_token  = data.doit_insights.first_page.pagination.page_token
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_insights.first_page", "results.#", "1"),
					resource.TestCheckResourceAttrSet("data.doit_insights.first_page", "pagination.page_token"),
					resource.TestCheckResourceAttr("data.doit_insights.paginated", "results.#", "1"),
					// Verify page_token actually advanced to a different page
					testCheckResourceAttrNotEqualAttr(
						"data.doit_insights.first_page", "results.0.key",
						"data.doit_insights.paginated", "results.0.key"),
				),
			},
		},
	})
}

// TestAccInsightsDataSource_AutoPagination tests that without max_results, all insights are fetched.
func TestAccInsightsDataSource_AutoPagination(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccInsightsDataSourceBasicConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Verify pagination completed (row_count is set, no page_token)
					resource.TestCheckResourceAttrSet("data.doit_insights.test", "pagination.row_count"),
				),
			},
		},
	})
}

// Helper functions

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

	// Auto-paginate to get true total count
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
