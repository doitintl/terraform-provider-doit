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

// TestAccAllocationsDataSource_MaxResultsOnly tests that setting max_results limits results.
func TestAccAllocationsDataSource_MaxResultsOnly(t *testing.T) {
	allocationCount := getAllocationCount(t)
	if allocationCount < 3 {
		t.Skipf("Need at least 3 allocations to test pagination, got %d", allocationCount)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAllocationsDataSourceMaxResultsConfig("2"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_allocations.limited", "allocations.#", "2"),
					resource.TestCheckResourceAttrSet("data.doit_allocations.limited", "page_token"),
				),
			},
			{
				Config:   testAccAllocationsDataSourceMaxResultsConfig("2"),
				PlanOnly: true,
			},
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccAllocationsDataSourceMaxResultsConfig("2"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAllocationsDataSourceMaxResultsConfig(maxResults string) string {
	return fmt.Sprintf(`
data "doit_allocations" "limited" {
  max_results = "%s"
}
`, maxResults)
}

// TestAccAllocationsDataSource_PageTokenOnly tests that setting only page_token (without max_results)
// auto-paginates starting from the token, returning fewer results than a full run.
// Uses two chained data sources in one apply to avoid race conditions with parallel tests.
func TestAccAllocationsDataSource_PageTokenOnly(t *testing.T) {
	totalAllocations := getAllocationCount(t)
	if totalAllocations < 2 {
		t.Skipf("Need at least 2 allocations to test page_token-only, got %d", totalAllocations)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_allocations" "first_page" {
  max_results = "1"
}
data "doit_allocations" "from_token" {
  page_token = data.doit_allocations.first_page.page_token
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_allocations.first_page", "allocations.#", "1"),
					resource.TestCheckResourceAttrSet("data.doit_allocations.first_page", "page_token"),
					resource.TestCheckResourceAttrSet("data.doit_allocations.from_token", "row_count"),
					resource.TestCheckNoResourceAttr("data.doit_allocations.from_token", "page_token"),
				),
			},
		},
	})
}

// TestAccAllocationsDataSource_MaxResultsAndPageToken tests using both parameters together.
// Uses two chained data sources in one apply to avoid race conditions with parallel tests.
func TestAccAllocationsDataSource_MaxResultsAndPageToken(t *testing.T) {
	allocationCount := getAllocationCount(t)
	if allocationCount < 3 {
		t.Skipf("Need at least 3 allocations to test pagination, got %d", allocationCount)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_allocations" "first_page" {
  max_results = "1"
}
data "doit_allocations" "paginated" {
  max_results = "1"
  page_token  = data.doit_allocations.first_page.page_token
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_allocations.first_page", "allocations.#", "1"),
					resource.TestCheckResourceAttrSet("data.doit_allocations.first_page", "page_token"),
					resource.TestCheckResourceAttr("data.doit_allocations.paginated", "allocations.#", "1"),
				),
			},
		},
	})
}

// TestAccAllocationsDataSource_AutoPagination tests that without max_results, all allocations are fetched.
func TestAccAllocationsDataSource_AutoPagination(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAllocationsDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Just verify row_count is set and pagination completed (no page_token)
					// Don't check specific values since parallel tests may change the count
					resource.TestCheckResourceAttrSet("data.doit_allocations.test", "row_count"),
					resource.TestCheckNoResourceAttr("data.doit_allocations.test", "page_token"),
				),
			},
		},
	})
}

func testAccAllocationsDataSourceConfig() string {
	return `
data "doit_allocations" "test" {
}
`
}

// Helper functions

var (
	allocationCount     int
	allocationCountOnce sync.Once
)

func getAllocationCount(t *testing.T) int {
	t.Helper()
	allocationCountOnce.Do(func() {
		allocationCount = computeAllocationCount(t)
	})
	return allocationCount
}

func computeAllocationCount(t *testing.T) int {
	t.Helper()
	client := getAPIClient(t)
	ctx := context.Background()

	var total int
	params := &models.ListAllocationsParams{}
	for {
		resp, err := client.ListAllocationsWithResponse(ctx, params)
		if err != nil {
			t.Fatalf("Failed to list allocations: %v", err)
		}
		if resp.JSON200 == nil || resp.JSON200.Allocations == nil {
			break
		}
		total += len(*resp.JSON200.Allocations)

		if resp.JSON200.PageToken == nil || *resp.JSON200.PageToken == "" {
			break
		}
		params.PageToken = resp.JSON200.PageToken
	}
	return total
}
