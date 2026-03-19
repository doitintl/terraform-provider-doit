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

// TestAccAnomaliesDataSource_MaxResultsOnly tests that setting max_results limits results.
func TestAccAnomaliesDataSource_MaxResultsOnly(t *testing.T) {
	anomalyCount := getAnomalyCount(t)
	if anomalyCount < 2 {
		t.Skipf("Need at least 2 anomalies to test pagination, got %d", anomalyCount)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAnomaliesDataSourceMaxResultsConfig(1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_anomalies.limited", "anomalies.#", "1"),
					resource.TestCheckResourceAttrSet("data.doit_anomalies.limited", "page_token"),
				),
			},
			{
				Config:   testAccAnomaliesDataSourceMaxResultsConfig(1),
				PlanOnly: true,
			},
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccAnomaliesDataSourceMaxResultsConfig(1),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAnomaliesDataSourceMaxResultsConfig(maxResults int64) string {
	return fmt.Sprintf(`
data "doit_anomalies" "limited" {
  max_results = %d
}
`, maxResults)
}

// TestAccAnomaliesDataSource_PageTokenOnly tests that setting only page_token (without max_results)
// auto-paginates starting from the token, returning results.
// Uses chained data sources to avoid page token expiry.
func TestAccAnomaliesDataSource_PageTokenOnly(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAnomaliesDataSourceChainedPageTokenOnly(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_anomalies.from_token", "anomalies.#"),
					resource.TestCheckResourceAttrSet("data.doit_anomalies.from_token", "row_count"),
					resource.TestCheckNoResourceAttr("data.doit_anomalies.from_token", "page_token"),
				),
			},
		},
	})
}

func testAccAnomaliesDataSourceChainedPageTokenOnly() string {
	return `
data "doit_anomalies" "first_page" {
  max_results = 1
}
data "doit_anomalies" "from_token" {
  page_token = data.doit_anomalies.first_page.page_token
}
`
}

// TestAccAnomaliesDataSource_MaxResultsAndPageToken tests using both parameters together.
// Uses chained data sources to avoid page token expiry.
func TestAccAnomaliesDataSource_MaxResultsAndPageToken(t *testing.T) {
	anomaliesCount := getAnomalyCount(t)
	if anomaliesCount < 2 {
		t.Skipf("Need at least 2 anomalies to test pagination, got %d", anomaliesCount)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAnomaliesDataSourceChainedMaxResultsAndPageToken(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_anomalies.second_page", "anomalies.#", "1"),
				),
			},
		},
	})
}

func testAccAnomaliesDataSourceChainedMaxResultsAndPageToken() string {
	return `
data "doit_anomalies" "first_page" {
  max_results = 1
}
data "doit_anomalies" "second_page" {
  max_results = 1
  page_token  = data.doit_anomalies.first_page.page_token
}
`
}

// TestAccAnomaliesDataSource_AutoPagination tests that without max_results, all anomalies are fetched.
func TestAccAnomaliesDataSource_AutoPagination(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAnomaliesDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Just verify row_count is set and pagination completed (no page_token)
					// Don't check specific values since parallel tests may change the count
					resource.TestCheckResourceAttrSet("data.doit_anomalies.test", "row_count"),
					resource.TestCheckNoResourceAttr("data.doit_anomalies.test", "page_token"),
				),
			},
		},
	})
}

func testAccAnomaliesDataSourceConfig() string {
	return `
data "doit_anomalies" "test" {
}
`
}

// Helper functions

var (
	anomalyCount     int
	anomalyCountOnce sync.Once
)

func getAnomalyCount(t *testing.T) int {
	t.Helper()
	anomalyCountOnce.Do(func() {
		anomalyCount = computeAnomalyCount(t)
	})
	return anomalyCount
}

func computeAnomalyCount(t *testing.T) int {
	t.Helper()
	client := getAPIClient(t)
	ctx := context.Background()

	var total int
	params := &models.ListAnomaliesParams{}
	for {
		resp, err := client.ListAnomaliesWithResponse(ctx, params)
		if err != nil {
			t.Fatalf("Failed to list anomalies: %v", err)
		}
		if resp.JSON200 == nil || resp.JSON200.Anomalies == nil {
			break
		}
		total += len(*resp.JSON200.Anomalies)

		if resp.JSON200.PageToken == nil || *resp.JSON200.PageToken == "" {
			break
		}
		params.PageToken = resp.JSON200.PageToken
	}
	return total
}
