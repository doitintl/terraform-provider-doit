package provider_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccAnomaliesDataSource_MaxResultsOnly tests that setting max_results limits results.
func TestAccAnomaliesDataSource_MaxResultsOnly(t *testing.T) {
	anomalyCount := getAnomalyCount(t)
	if anomalyCount < 3 {
		t.Skipf("Need at least 3 anomalies to test pagination, got %d", anomalyCount)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAnomaliesDataSourceMaxResultsConfig(2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_anomalies.limited", "anomalies.#", "2"),
					resource.TestCheckResourceAttrSet("data.doit_anomalies.limited", "page_token"),
				),
			},
			{
				Config:   testAccAnomaliesDataSourceMaxResultsConfig(2),
				PlanOnly: true,
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

// TestAccAnomaliesDataSource_PageTokenOnly tests using a page_token from a previous API call.
func TestAccAnomaliesDataSource_PageTokenOnly(t *testing.T) {
	pageToken := getAnomalyFirstPageToken(t, 1)
	if pageToken == "" {
		t.Skip("No page_token returned (need more than 1 anomaly)")
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAnomaliesDataSourcePageTokenConfig(pageToken),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_anomalies.from_token", "anomalies.#"),
				),
			},
		},
	})
}

func testAccAnomaliesDataSourcePageTokenConfig(pageToken string) string {
	return fmt.Sprintf(`
data "doit_anomalies" "from_token" {
  page_token = "%s"
}
`, pageToken)
}

// TestAccAnomaliesDataSource_MaxResultsAndPageToken tests using both parameters together.
func TestAccAnomaliesDataSource_MaxResultsAndPageToken(t *testing.T) {
	pageToken := getAnomalyFirstPageToken(t, 1)
	if pageToken == "" {
		t.Skip("No page_token returned (need more than 1 anomaly)")
	}

	anomalyCount := getAnomalyCount(t)
	if anomalyCount < 3 {
		t.Skipf("Need at least 3 anomalies to test pagination, got %d", anomalyCount)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAnomaliesDataSourceMaxResultsAndPageTokenConfig(1, pageToken),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_anomalies.paginated", "anomalies.#", "1"),
				),
			},
		},
	})
}

func testAccAnomaliesDataSourceMaxResultsAndPageTokenConfig(maxResults int64, pageToken string) string {
	return fmt.Sprintf(`
data "doit_anomalies" "paginated" {
  max_results = %d
  page_token  = "%s"
}
`, maxResults, pageToken)
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

func getAnomalyFirstPageToken(t *testing.T, maxResults int64) string {
	t.Helper()
	client := getAPIClient(t)
	ctx := context.Background()

	resp, err := client.ListAnomaliesWithResponse(ctx, &models.ListAnomaliesParams{
		MaxResults: &maxResults,
	})
	if err != nil {
		t.Fatalf("Failed to list anomalies: %v", err)
	}
	if resp.JSON200 == nil {
		t.Fatal("No response from API")
	}
	if resp.JSON200.PageToken == nil {
		return ""
	}
	return *resp.JSON200.PageToken
}
