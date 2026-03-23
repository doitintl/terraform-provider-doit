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
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccAlertsDataSourceMaxResultsConfig("2"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
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
// auto-paginates starting from the token, returning a different set of items.
// Uses two chained data sources in one apply:
//   - "first_page": fetches 1 alert + page_token
//   - "from_token": auto-paginates from the token (should start at a different item)
func TestAccAlertsDataSource_PageTokenOnly(t *testing.T) {
	totalAlerts := getAlertCount(t)
	if totalAlerts < 2 {
		t.Skipf("Need at least 2 alerts to test page_token-only, got %d", totalAlerts)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_alerts" "first_page" {
  max_results = "1"
}
data "doit_alerts" "from_token" {
  page_token = data.doit_alerts.first_page.page_token
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_alerts.first_page", "alerts.#", "1"),
					resource.TestCheckResourceAttrSet("data.doit_alerts.first_page", "page_token"),
					resource.TestCheckResourceAttrSet("data.doit_alerts.from_token", "row_count"),
					resource.TestCheckNoResourceAttr("data.doit_alerts.from_token", "page_token"),
					// Verify page_token actually advanced to a different starting point
					testCheckResourceAttrNotEqualAttr(
						"data.doit_alerts.first_page", "alerts.0.id",
						"data.doit_alerts.from_token", "alerts.0.id"),
				),
			},
		},
	})
}

// TestAccAlertsDataSource_MaxResultsAndPageToken tests using both max_results and page_token together.
// Uses two chained data sources in one apply to avoid race conditions with parallel tests.
func TestAccAlertsDataSource_MaxResultsAndPageToken(t *testing.T) {
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
				Config: `
data "doit_alerts" "first_page" {
  max_results = "1"
}
data "doit_alerts" "paginated" {
  max_results = "1"
  page_token  = data.doit_alerts.first_page.page_token
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_alerts.first_page", "alerts.#", "1"),
					resource.TestCheckResourceAttrSet("data.doit_alerts.first_page", "page_token"),
					resource.TestCheckResourceAttr("data.doit_alerts.paginated", "alerts.#", "1"),
					// Verify page_token actually advanced to a different page
					testCheckResourceAttrNotEqualAttr(
						"data.doit_alerts.first_page", "alerts.0.id",
						"data.doit_alerts.paginated", "alerts.0.id"),
				),
			},
		},
	})
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
