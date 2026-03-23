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

// TestAccReportsDataSource_MaxResultsOnly tests that setting max_results limits results.
func TestAccReportsDataSource_MaxResultsOnly(t *testing.T) {
	reportCount := getReportCount(t)
	if reportCount < 3 {
		t.Skipf("Need at least 3 reports to test pagination, got %d", reportCount)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportsDataSourceMaxResultsConfig("2"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_reports.limited", "reports.#", "2"),
					resource.TestCheckResourceAttrSet("data.doit_reports.limited", "page_token"),
				),
			},
			{
				Config:   testAccReportsDataSourceMaxResultsConfig("2"),
				PlanOnly: true,
			},
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccReportsDataSourceMaxResultsConfig("2"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccReportsDataSourceMaxResultsConfig(maxResults string) string {
	return fmt.Sprintf(`
data "doit_reports" "limited" {
  max_results = "%s"
}
`, maxResults)
}

// TestAccReportsDataSource_PageTokenOnly tests that setting only page_token (without max_results)
// auto-paginates starting from the token, returning fewer results than a full run.
// Uses three chained data sources in one apply to avoid race conditions with parallel tests:
//   - "all": fetches all reports to get total count
//   - "first_page": fetches 1 report + page_token
//   - "from_token": auto-paginates from the token (should return fewer than all)
func TestAccReportsDataSource_PageTokenOnly(t *testing.T) {
	totalReports := getReportCount(t)
	if totalReports < 2 {
		t.Skipf("Need at least 2 reports to test page_token-only, got %d", totalReports)
	}

	// Non-parallel: the "less than" assertion requires a stable resource count
	// during the apply. Parallel tests mutate counts and cause false failures.
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_reports" "all" {}
data "doit_reports" "first_page" {
  max_results = "1"
}
data "doit_reports" "from_token" {
  page_token = data.doit_reports.first_page.page_token
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_reports.first_page", "reports.#", "1"),
					resource.TestCheckResourceAttrSet("data.doit_reports.first_page", "page_token"),
					testCheckResourceAttrLessThanAttr(
						"data.doit_reports.from_token", "row_count",
						"data.doit_reports.all", "row_count",
					),
					resource.TestCheckNoResourceAttr("data.doit_reports.from_token", "page_token"),
				),
			},
		},
	})
}

// TestAccReportsDataSource_MaxResultsAndPageToken tests using both parameters together.
// Uses two chained data sources in one apply to avoid race conditions with parallel tests.
func TestAccReportsDataSource_MaxResultsAndPageToken(t *testing.T) {
	reportCount := getReportCount(t)
	if reportCount < 3 {
		t.Skipf("Need at least 3 reports to test pagination, got %d", reportCount)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_reports" "first_page" {
  max_results = "1"
}
data "doit_reports" "paginated" {
  max_results = "1"
  page_token  = data.doit_reports.first_page.page_token
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_reports.first_page", "reports.#", "1"),
					resource.TestCheckResourceAttrSet("data.doit_reports.first_page", "page_token"),
					resource.TestCheckResourceAttr("data.doit_reports.paginated", "reports.#", "1"),
					// Verify page_token actually advanced to a different page
					testCheckResourceAttrNotEqualAttr(
						"data.doit_reports.first_page", "reports.0.id",
						"data.doit_reports.paginated", "reports.0.id"),
				),
			},
		},
	})
}

// TestAccReportsDataSource_AutoPagination tests that without max_results, all reports are fetched.
func TestAccReportsDataSource_AutoPagination(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportsDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Just verify row_count is set and pagination completed (no page_token)
					// Don't check specific values since parallel tests may change the count
					resource.TestCheckResourceAttrSet("data.doit_reports.test", "row_count"),
					resource.TestCheckNoResourceAttr("data.doit_reports.test", "page_token"),
				),
			},
		},
	})
}

func testAccReportsDataSourceConfig() string {
	return `
data "doit_reports" "test" {
}
`
}

// Helper functions

var (
	reportCount     int
	reportCountOnce sync.Once
)

func getReportCount(t *testing.T) int {
	t.Helper()
	reportCountOnce.Do(func() {
		reportCount = computeReportCount(t)
	})
	return reportCount
}

func computeReportCount(t *testing.T) int {
	t.Helper()
	client := getAPIClient(t)
	ctx := context.Background()

	var total int
	params := &models.ListReportsParams{}
	for {
		resp, err := client.ListReportsWithResponse(ctx, params)
		if err != nil {
			t.Fatalf("Failed to list reports: %v", err)
		}
		if resp.JSON200 == nil || resp.JSON200.Reports == nil {
			break
		}
		total += len(*resp.JSON200.Reports)

		if resp.JSON200.PageToken == nil || *resp.JSON200.PageToken == "" {
			break
		}
		params.PageToken = resp.JSON200.PageToken
	}
	return total
}
