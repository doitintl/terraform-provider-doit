package provider_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccAlertsDataSource_MaxResultsOnly tests that setting max_results limits results
// and returns a page_token for fetching the next page.
func TestAccAlertsDataSource_MaxResultsOnly(t *testing.T) {
	alertCount := getAlertCount(t)
	if alertCount < 3 {
		t.Skipf("Need at least 3 alerts to test pagination, got %d", alertCount)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAlertsDataSourceMaxResultsConfig("2"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_alerts.limited", "alerts.#", "2"),
					resource.TestCheckResourceAttrSet("data.doit_alerts.limited", "page_token"),
				),
			},
			{
				Config:   testAccAlertsDataSourceMaxResultsConfig("2"),
				PlanOnly: true,
			},
		},
	})
}

func testAccAlertsDataSourceMaxResultsConfig(maxResults string) string {
	return fmt.Sprintf(`
data "doit_alerts" "limited" {
  max_results = "%s"
}
`, maxResults)
}

// TestAccAlertsDataSource_PageTokenOnly tests that setting only page_token (without max_results)
// auto-paginates starting from the token, returning fewer results than a full run.
func TestAccAlertsDataSource_PageTokenOnly(t *testing.T) {
	// TODO(CMP-38591): The alerts API ignores pageToken when maxResults is not set, returning all results.
	// Remove this skip once the API supports page_token-only pagination.
	t.Skip("Skipped: alerts API ignores pageToken without maxResults (CMP-38591)")

	totalAlerts := getAlertCount(t)
	if totalAlerts < 2 {
		t.Skipf("Need at least 2 alerts to test page_token-only, got %d", totalAlerts)
	}

	pageToken := getAlertFirstPageToken(t, 1)
	if pageToken == "" {
		t.Skip("No page_token returned (need more than 1 alert)")
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAlertsDataSourcePageTokenConfig(pageToken),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_alerts.from_token", "alerts.#"),
					// Verify row_count is less than total, proving the token was honored
					testCheckResourceAttrLessThan("data.doit_alerts.from_token", "row_count", totalAlerts),
				),
			},
		},
	})
}

func testAccAlertsDataSourcePageTokenConfig(pageToken string) string {
	return fmt.Sprintf(`
data "doit_alerts" "from_token" {
  page_token = "%s"
}
`, pageToken)
}

// TestAccAlertsDataSource_MaxResultsAndPageToken tests using both max_results and page_token together.
func TestAccAlertsDataSource_MaxResultsAndPageToken(t *testing.T) {
	pageToken := getAlertFirstPageToken(t, 1)
	if pageToken == "" {
		t.Skip("No page_token returned (need more than 1 alert)")
	}

	alertCount := getAlertCount(t)
	if alertCount < 3 {
		t.Skipf("Need at least 3 alerts to test pagination, got %d", alertCount)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAlertsDataSourceMaxResultsAndPageTokenConfig("1", pageToken),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_alerts.paginated", "alerts.#", "1"),
				),
			},
		},
	})
}

func testAccAlertsDataSourceMaxResultsAndPageTokenConfig(maxResults, pageToken string) string {
	return fmt.Sprintf(`
data "doit_alerts" "paginated" {
  max_results = "%s"
  page_token  = "%s"
}
`, maxResults, pageToken)
}

// TestAccAlertsDataSource_AutoPagination tests that without max_results, all alerts are fetched.
func TestAccAlertsDataSource_AutoPagination(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAlertsDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Just verify row_count is set and pagination completed (no page_token)
					// Don't check specific values since parallel tests may change the count
					resource.TestCheckResourceAttrSet("data.doit_alerts.test", "row_count"),
					resource.TestCheckNoResourceAttr("data.doit_alerts.test", "page_token"),
				),
			},
		},
	})
}

func testAccAlertsDataSourceConfig() string {
	return `
data "doit_alerts" "test" {
}
`
}

// Helper functions

var (
	alertCount     int
	alertCountOnce sync.Once
)

func getAlertCount(t *testing.T) int {
	t.Helper()
	alertCountOnce.Do(func() {
		alertCount = computeAlertCount(t)
	})
	return alertCount
}

func computeAlertCount(t *testing.T) int {
	t.Helper()
	client := getAPIClient(t)
	ctx := context.Background()

	var total int
	params := &models.ListAlertsParams{}
	for {
		resp, err := client.ListAlertsWithResponse(ctx, params)
		if err != nil {
			t.Fatalf("Failed to list alerts: %v", err)
		}
		if resp.JSON200 == nil || resp.JSON200.Alerts == nil {
			break
		}
		total += len(*resp.JSON200.Alerts)

		if resp.JSON200.PageToken == nil || *resp.JSON200.PageToken == "" {
			break
		}
		params.PageToken = resp.JSON200.PageToken
	}
	return total
}

func getAlertFirstPageToken(t *testing.T, maxResults int) string {
	t.Helper()
	client := getAPIClient(t)
	ctx := context.Background()

	maxResultsStr := fmt.Sprintf("%d", maxResults)
	resp, err := client.ListAlertsWithResponse(ctx, &models.ListAlertsParams{
		MaxResults: &maxResultsStr,
	})
	if err != nil {
		t.Fatalf("Failed to list alerts: %v", err)
	}
	if resp.JSON200 == nil {
		t.Fatal("No response from API")
	}
	if resp.JSON200.PageToken == nil {
		return ""
	}
	return *resp.JSON200.PageToken
}
