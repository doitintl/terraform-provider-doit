package provider_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func testAccBudgetsDataSourceConfig() string {
	return `
data "doit_budgets" "test" {
}
`
}

// TestAccBudgetsDataSource_MaxResultsOnly tests that setting max_results limits results
// and returns a page_token for fetching the next page.
func TestAccBudgetsDataSource_MaxResultsOnly(t *testing.T) {
	// Skip if we cannot verify pagination (need at least 3 budgets)
	budgetCount := getBudgetCount(t)
	if budgetCount < 3 {
		t.Skipf("Need at least 3 budgets to test pagination, got %d", budgetCount)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccBudgetsDataSourceMaxResultsConfig("2"),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Verify we got exactly 2 budgets (as limited by max_results)
					resource.TestCheckResourceAttr("data.doit_budgets.limited", "budgets.#", "2"),
					// Verify page_token is returned (more pages exist)
					resource.TestCheckResourceAttrSet("data.doit_budgets.limited", "page_token"),
				),
			},
			// Second apply should produce no diff (max_results preserved in state)
			{
				Config:   testAccBudgetsDataSourceMaxResultsConfig("2"),
				PlanOnly: true,
			},
		},
	})
}

func testAccBudgetsDataSourceMaxResultsConfig(maxResults string) string {
	return fmt.Sprintf(`
data "doit_budgets" "limited" {
  max_results = "%s"
}
`, maxResults)
}

// TestAccBudgetsDataSource_PageTokenOnly tests that setting only page_token (without max_results)
// auto-paginates starting from the token, returning fewer results than a full run.
func TestAccBudgetsDataSource_PageTokenOnly(t *testing.T) {
	// TODO(CMP-38591): The budgets API returns 500 when pageToken is sent without maxResults.
	// Remove this skip once the API supports page_token-only pagination.
	t.Skip("Skipped: budgets API returns 500 with pageToken without maxResults (CMP-38591)")

	// We need at least 2 budgets: one before the token and one after.
	totalBudgets := getBudgetCount(t)
	if totalBudgets < 2 {
		t.Skipf("Need at least 2 budgets to test page_token-only, got %d", totalBudgets)
	}

	// Fetch page_token after the first item
	pageToken := getFirstPageToken(t, 1)
	if pageToken == "" {
		t.Skip("No page_token returned (need more than 1 budget)")
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccBudgetsDataSourcePageTokenConfig(pageToken),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Verify we got some budgets (starting from page 2)
					resource.TestCheckResourceAttrSet("data.doit_budgets.from_token", "budgets.#"),
					// Verify row_count is less than total, proving the token was honored
					testCheckResourceAttrLessThan("data.doit_budgets.from_token", "row_count", totalBudgets),
				),
			},
		},
	})
}

func testAccBudgetsDataSourcePageTokenConfig(pageToken string) string {
	return fmt.Sprintf(`
data "doit_budgets" "from_token" {
  page_token = "%s"
}
`, pageToken)
}

// TestAccBudgetsDataSource_MaxResultsAndPageToken tests using both max_results and page_token together.
func TestAccBudgetsDataSource_MaxResultsAndPageToken(t *testing.T) {
	// Fetch page_token via API client
	pageToken := getFirstPageToken(t, 1)
	if pageToken == "" {
		t.Skip("No page_token returned (need more than 1 budget)")
	}

	// Check we have enough budgets to test with
	budgetCount := getBudgetCount(t)
	if budgetCount < 3 {
		t.Skipf("Need at least 3 budgets to test pagination, got %d", budgetCount)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccBudgetsDataSourceMaxResultsAndPageTokenConfig("1", pageToken),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Verify we got exactly 1 budget from page 2
					resource.TestCheckResourceAttr("data.doit_budgets.paginated", "budgets.#", "1"),
				),
			},
		},
	})
}

func testAccBudgetsDataSourceMaxResultsAndPageTokenConfig(maxResults, pageToken string) string {
	return fmt.Sprintf(`
data "doit_budgets" "paginated" {
  max_results = "%s"
  page_token  = "%s"
}
`, maxResults, pageToken)
}

// TestAccBudgetsDataSource_AutoPagination tests that without max_results, all budgets are fetched.
func TestAccBudgetsDataSource_AutoPagination(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccBudgetsDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Just verify row_count is set and pagination completed (no page_token)
					// Don't check specific values since parallel tests may change the count
					resource.TestCheckResourceAttrSet("data.doit_budgets.test", "row_count"),
					resource.TestCheckNoResourceAttr("data.doit_budgets.test", "page_token"),
				),
			},
		},
	})
}

// Helper functions

var (
	budgetCount     int
	budgetCountOnce sync.Once
)

func getBudgetCount(t *testing.T) int {
	t.Helper()
	budgetCountOnce.Do(func() {
		budgetCount = computeBudgetCount(t)
	})
	return budgetCount
}

func computeBudgetCount(t *testing.T) int {
	t.Helper()
	client := getAPIClient(t)
	ctx := context.Background()

	// Auto-paginate to get true total count
	var total int
	params := &models.ListBudgetsParams{}
	for {
		resp, err := client.ListBudgetsWithResponse(ctx, params)
		if err != nil {
			t.Fatalf("Failed to list budgets: %v", err)
		}
		if resp.JSON200 == nil || resp.JSON200.Budgets == nil {
			break
		}
		total += len(*resp.JSON200.Budgets)

		if resp.JSON200.PageToken == nil || *resp.JSON200.PageToken == "" {
			break
		}
		params.PageToken = resp.JSON200.PageToken
	}
	return total
}

func getFirstPageToken(t *testing.T, maxResults int) string {
	t.Helper()
	client := getAPIClient(t)
	ctx := context.Background()

	maxResultsStr := fmt.Sprintf("%d", maxResults)
	resp, err := client.ListBudgetsWithResponse(ctx, &models.ListBudgetsParams{
		MaxResults: &maxResultsStr,
	})
	if err != nil {
		t.Fatalf("Failed to list budgets: %v", err)
	}
	if resp.JSON200 == nil {
		t.Fatal("No response from API")
	}
	if resp.JSON200.PageToken == nil {
		return ""
	}
	return *resp.JSON200.PageToken
}
