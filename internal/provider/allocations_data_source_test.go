package provider_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
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

// TestAccAllocationsDataSource_PageTokenOnly tests using a page_token from a previous API call.
func TestAccAllocationsDataSource_PageTokenOnly(t *testing.T) {
	pageToken := getAllocationFirstPageToken(t, 1)
	if pageToken == "" {
		t.Skip("No page_token returned (need more than 1 allocation)")
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAllocationsDataSourcePageTokenConfig(pageToken),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_allocations.from_token", "allocations.#"),
				),
			},
		},
	})
}

func testAccAllocationsDataSourcePageTokenConfig(pageToken string) string {
	return fmt.Sprintf(`
data "doit_allocations" "from_token" {
  page_token = "%s"
}
`, pageToken)
}

// TestAccAllocationsDataSource_MaxResultsAndPageToken tests using both parameters together.
func TestAccAllocationsDataSource_MaxResultsAndPageToken(t *testing.T) {
	pageToken := getAllocationFirstPageToken(t, 1)
	if pageToken == "" {
		t.Skip("No page_token returned (need more than 1 allocation)")
	}

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
				Config: testAccAllocationsDataSourceMaxResultsAndPageTokenConfig("1", pageToken),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_allocations.paginated", "allocations.#", "1"),
				),
			},
		},
	})
}

func testAccAllocationsDataSourceMaxResultsAndPageTokenConfig(maxResults, pageToken string) string {
	return fmt.Sprintf(`
data "doit_allocations" "paginated" {
  max_results = "%s"
  page_token  = "%s"
}
`, maxResults, pageToken)
}

// TestAccAllocationsDataSource_AutoPagination tests that without max_results, all allocations are fetched.
func TestAccAllocationsDataSource_AutoPagination(t *testing.T) {
	expectedCount := getAllocationCount(t)
	if expectedCount == 0 {
		t.Skip("No allocations available to test auto-pagination")
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAllocationsDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_allocations.test", "row_count", fmt.Sprintf("%d", expectedCount)),
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

func getAllocationFirstPageToken(t *testing.T, maxResults int) string {
	t.Helper()
	client := getAPIClient(t)
	ctx := context.Background()

	maxResultsStr := fmt.Sprintf("%d", maxResults)
	resp, err := client.ListAllocationsWithResponse(ctx, &models.ListAllocationsParams{
		MaxResults: &maxResultsStr,
	})
	if err != nil {
		t.Fatalf("Failed to list allocations: %v", err)
	}
	if resp.JSON200 == nil {
		t.Fatal("No response from API")
	}
	if resp.JSON200.PageToken == nil {
		return ""
	}
	return *resp.JSON200.PageToken
}
