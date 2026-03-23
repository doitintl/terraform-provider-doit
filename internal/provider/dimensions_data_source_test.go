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
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccDimensionsDataSourceMaxResultsConfig("2"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
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
// Uses three chained data sources in one apply to avoid race conditions with parallel tests:
//   - "all": fetches all dimensions to get total count
//   - "first_page": fetches 1 dimension + page_token
//   - "from_token": auto-paginates from the token (should return fewer than all)
func TestAccDimensionsDataSource_PageTokenOnly(t *testing.T) {
	totalDimensions := getDimensionCount(t)
	if totalDimensions < 2 {
		t.Skipf("Need at least 2 dimensions to test page_token-only, got %d", totalDimensions)
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
data "doit_dimensions" "all" {}
data "doit_dimensions" "first_page" {
  max_results = "1"
}
data "doit_dimensions" "from_token" {
  page_token = data.doit_dimensions.first_page.page_token
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_dimensions.first_page", "dimensions.#", "1"),
					resource.TestCheckResourceAttrSet("data.doit_dimensions.first_page", "page_token"),
					testCheckResourceAttrLessThanAttr(
						"data.doit_dimensions.from_token", "row_count",
						"data.doit_dimensions.all", "row_count",
					),
					resource.TestCheckNoResourceAttr("data.doit_dimensions.from_token", "page_token"),
				),
			},
		},
	})
}

// TestAccDimensionsDataSource_MaxResultsAndPageToken tests using both parameters together.
// Uses two chained data sources in one apply to avoid race conditions with parallel tests.
func TestAccDimensionsDataSource_MaxResultsAndPageToken(t *testing.T) {
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
				Config: `
data "doit_dimensions" "first_page" {
  max_results = "1"
}
data "doit_dimensions" "paginated" {
  max_results = "1"
  page_token  = data.doit_dimensions.first_page.page_token
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_dimensions.first_page", "dimensions.#", "1"),
					resource.TestCheckResourceAttrSet("data.doit_dimensions.first_page", "page_token"),
					resource.TestCheckResourceAttr("data.doit_dimensions.paginated", "dimensions.#", "1"),
				),
			},
		},
	})
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
