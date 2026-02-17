package provider_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccDimensionsDataSource_MaxResultsOnly tests that setting max_results limits results.
func TestAccDimensionsDataSource_MaxResultsOnly(t *testing.T) {
	dimensionCount := getDimensionCount(t)
	if dimensionCount < 3 {
		t.Skipf("Need at least 3 dimensions to test pagination, got %d", dimensionCount)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccDimensionsDataSourceMaxResultsConfig("2"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_dimensions.limited", "dimensions.#", "2"),
					resource.TestCheckResourceAttrSet("data.doit_dimensions.limited", "page_token"),
					// Verify field values are populated (not empty)
					resource.TestCheckResourceAttrSet("data.doit_dimensions.limited", "dimensions.0.id"),
					resource.TestCheckResourceAttrSet("data.doit_dimensions.limited", "dimensions.0.label"),
					resource.TestCheckResourceAttrSet("data.doit_dimensions.limited", "dimensions.0.type"),
				),
			},
			{
				Config:   testAccDimensionsDataSourceMaxResultsConfig("2"),
				PlanOnly: true,
			},
		},
	})
}

func testAccDimensionsDataSourceMaxResultsConfig(maxResults string) string {
	return fmt.Sprintf(`
data "doit_dimensions" "limited" {
  max_results = "%s"
}
`, maxResults)
}

// TestAccDimensionsDataSource_PageTokenOnly tests that setting only page_token (without max_results)
// auto-paginates starting from the token, returning fewer results than a full run.
func TestAccDimensionsDataSource_PageTokenOnly(t *testing.T) {
	// TODO(CMP-38591): The dimensions API ignores pageToken when maxResults is not set, returning all results.
	// Remove this skip once the API supports page_token-only pagination.
	t.Skip("Skipped: dimensions API ignores pageToken without maxResults (CMP-38591)")

	totalDimensions := getDimensionCount(t)
	if totalDimensions < 2 {
		t.Skipf("Need at least 2 dimensions to test page_token-only, got %d", totalDimensions)
	}

	pageToken := getDimensionFirstPageToken(t, 1)
	if pageToken == "" {
		t.Skip("No page_token returned (need more than 1 dimension)")
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccDimensionsDataSourcePageTokenConfig(pageToken),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_dimensions.from_token", "dimensions.#"),
					testCheckResourceAttrLessThan("data.doit_dimensions.from_token", "row_count", totalDimensions),
				),
			},
		},
	})
}

func testAccDimensionsDataSourcePageTokenConfig(pageToken string) string {
	return fmt.Sprintf(`
data "doit_dimensions" "from_token" {
  page_token = "%s"
}
`, pageToken)
}

// TestAccDimensionsDataSource_MaxResultsAndPageToken tests using both parameters together.
func TestAccDimensionsDataSource_MaxResultsAndPageToken(t *testing.T) {
	pageToken := getDimensionFirstPageToken(t, 1)
	if pageToken == "" {
		t.Skip("No page_token returned (need more than 1 dimension)")
	}

	dimensionCount := getDimensionCount(t)
	if dimensionCount < 3 {
		t.Skipf("Need at least 3 dimensions to test pagination, got %d", dimensionCount)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccDimensionsDataSourceMaxResultsAndPageTokenConfig("1", pageToken),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_dimensions.paginated", "dimensions.#", "1"),
				),
			},
		},
	})
}

func testAccDimensionsDataSourceMaxResultsAndPageTokenConfig(maxResults, pageToken string) string {
	return fmt.Sprintf(`
data "doit_dimensions" "paginated" {
  max_results = "%s"
  page_token  = "%s"
}
`, maxResults, pageToken)
}

// TestAccDimensionsDataSource_AutoPagination tests that without max_results, all dimensions are fetched.
func TestAccDimensionsDataSource_AutoPagination(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccDimensionsDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Just verify row_count is set and pagination completed (no page_token)
					// Don't check specific values since parallel tests may change the count
					resource.TestCheckResourceAttrSet("data.doit_dimensions.test", "row_count"),
					resource.TestCheckNoResourceAttr("data.doit_dimensions.test", "page_token"),
				),
			},
		},
	})
}

func testAccDimensionsDataSourceConfig() string {
	return `
data "doit_dimensions" "test" {
}
`
}

// Helper functions

var (
	dimensionCount     int
	dimensionCountOnce sync.Once
)

func getDimensionCount(t *testing.T) int {
	t.Helper()
	dimensionCountOnce.Do(func() {
		dimensionCount = computeDimensionCount(t)
	})
	return dimensionCount
}

func computeDimensionCount(t *testing.T) int {
	t.Helper()
	client := getAPIClient(t)
	ctx := context.Background()

	var total int
	params := &models.ListDimensionsParams{}
	for {
		resp, err := client.ListDimensionsWithResponse(ctx, params)
		if err != nil {
			t.Fatalf("Failed to list dimensions: %v", err)
		}
		if resp.JSON200 == nil || resp.JSON200.Dimensions == nil {
			break
		}
		total += len(*resp.JSON200.Dimensions)

		if resp.JSON200.PageToken == nil || *resp.JSON200.PageToken == "" {
			break
		}
		params.PageToken = resp.JSON200.PageToken
	}
	return total
}

func getDimensionFirstPageToken(t *testing.T, maxResults int) string {
	t.Helper()
	client := getAPIClient(t)
	ctx := context.Background()

	maxResultsStr := fmt.Sprintf("%d", maxResults)
	resp, err := client.ListDimensionsWithResponse(ctx, &models.ListDimensionsParams{
		MaxResults: &maxResultsStr,
	})
	if err != nil {
		t.Fatalf("Failed to list dimensions: %v", err)
	}
	if resp.JSON200 == nil {
		t.Fatal("No response from API")
	}
	if resp.JSON200.PageToken == nil {
		return ""
	}
	return *resp.JSON200.PageToken
}
