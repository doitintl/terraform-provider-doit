package provider_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccReportsDataSource_MaxResultsOnly tests that setting max_results limits results.
func TestAccReportsDataSource_MaxResultsOnly(t *testing.T) {
	reportCount := getReportCount(t)
	if reportCount < 3 {
		t.Skipf("Need at least 3 reports to test pagination, got %d", reportCount)
	}

	resource.Test(t, resource.TestCase{
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

// TestAccReportsDataSource_PageTokenOnly tests using a page_token from a previous API call.
func TestAccReportsDataSource_PageTokenOnly(t *testing.T) {
	pageToken := getReportFirstPageToken(t, 1)
	if pageToken == "" {
		t.Skip("No page_token returned (need more than 1 report)")
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportsDataSourcePageTokenConfig(pageToken),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_reports.from_token", "reports.#"),
				),
			},
		},
	})
}

func testAccReportsDataSourcePageTokenConfig(pageToken string) string {
	return fmt.Sprintf(`
data "doit_reports" "from_token" {
  page_token = "%s"
}
`, pageToken)
}

// TestAccReportsDataSource_MaxResultsAndPageToken tests using both parameters together.
func TestAccReportsDataSource_MaxResultsAndPageToken(t *testing.T) {
	pageToken := getReportFirstPageToken(t, 1)
	if pageToken == "" {
		t.Skip("No page_token returned (need more than 1 report)")
	}

	reportCount := getReportCount(t)
	if reportCount < 3 {
		t.Skipf("Need at least 3 reports to test pagination, got %d", reportCount)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportsDataSourceMaxResultsAndPageTokenConfig("1", pageToken),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_reports.paginated", "reports.#", "1"),
				),
			},
		},
	})
}

func testAccReportsDataSourceMaxResultsAndPageTokenConfig(maxResults, pageToken string) string {
	return fmt.Sprintf(`
data "doit_reports" "paginated" {
  max_results = "%s"
  page_token  = "%s"
}
`, maxResults, pageToken)
}

// TestAccReportsDataSource_AutoPagination tests that without max_results, all reports are fetched.
func TestAccReportsDataSource_AutoPagination(t *testing.T) {
	expectedCount := getReportCount(t)
	if expectedCount == 0 {
		t.Skip("No reports available to test auto-pagination")
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccReportsDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_reports.test", "row_count", fmt.Sprintf("%d", expectedCount)),
					resource.TestCheckNoResourceAttr("data.doit_reports.test", "page_token"),
				),
			},
		},
	})
}

func testAccReportsDataSourceConfig() string {
	return `
data "doit_reports" "test" {
  sort_by    = "name"
  sort_order = "asc"
}
`
}

// Helper functions

func getReportCount(t *testing.T) int {
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

func getReportFirstPageToken(t *testing.T, maxResults int) string {
	t.Helper()
	client := getAPIClient(t)
	ctx := context.Background()

	maxResultsStr := fmt.Sprintf("%d", maxResults)
	resp, err := client.ListReportsWithResponse(ctx, &models.ListReportsParams{
		MaxResults: &maxResultsStr,
	})
	if err != nil {
		t.Fatalf("Failed to list reports: %v", err)
	}
	if resp.JSON200 == nil {
		t.Fatal("No response from API")
	}
	if resp.JSON200.PageToken == nil {
		return ""
	}
	return *resp.JSON200.PageToken
}
