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

// TestAccSupportRequestsDataSource_MaxResultsOnly tests that setting max_results limits results.
func TestAccSupportRequestsDataSource_MaxResultsOnly(t *testing.T) {
	// TODO(CMP-38591): Support tickets API ignores both pageSize and maxResults.
	t.Skip("Skipping: support tickets API ignores pageSize/maxResults (CMP-38591)")

	ticketCount := getSupportRequestCount(t)
	if ticketCount < 3 {
		t.Skipf("Need at least 3 support requests to test pagination, got %d", ticketCount)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccSupportRequestsDataSourceMaxResultsConfig(2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_support_requests.limited", "tickets.#", "2"),
					resource.TestCheckResourceAttrSet("data.doit_support_requests.limited", "page_token"),
				),
			},
			{
				Config:   testAccSupportRequestsDataSourceMaxResultsConfig(2),
				PlanOnly: true,
			},
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccSupportRequestsDataSourceMaxResultsConfig(2),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccSupportRequestsDataSourceMaxResultsConfig(maxResults int64) string {
	return fmt.Sprintf(`
data "doit_support_requests" "limited" {
  max_results = %d
}
`, maxResults)
}

// TestAccSupportRequestsDataSource_PageTokenOnly tests that setting only page_token (without max_results)
// auto-paginates starting from the token, returning fewer results than a full run.
// Uses chained data sources to avoid page token expiry.
func TestAccSupportRequestsDataSource_PageTokenOnly(t *testing.T) {
	// TODO(CMP-38591): Support tickets API ignores both pageSize and maxResults.
	t.Skip("Skipping: support tickets API ignores pageSize/maxResults (CMP-38591)")
	totalSupportRequests := getSupportRequestCount(t)
	if totalSupportRequests < 2 {
		t.Skipf("Need at least 2 support_requests to test page_token-only, got %d", totalSupportRequests)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccSupportRequestsDataSourceChainedPageTokenOnly(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_support_requests.from_token", "tickets.#"),
					testCheckResourceAttrLessThan("data.doit_support_requests.from_token", "row_count", totalSupportRequests),
				),
			},
		},
	})
}

func testAccSupportRequestsDataSourceChainedPageTokenOnly() string {
	return `
data "doit_support_requests" "first_page" {
  max_results = 1
}
data "doit_support_requests" "from_token" {
  page_token = data.doit_support_requests.first_page.page_token
}
`
}

// TestAccSupportRequestsDataSource_MaxResultsAndPageToken tests using both parameters together.
// Uses chained data sources to avoid page token expiry.
func TestAccSupportRequestsDataSource_MaxResultsAndPageToken(t *testing.T) {
	// TODO(CMP-38591): Support tickets API ignores both pageSize and maxResults.
	t.Skip("Skipping: support tickets API ignores pageSize/maxResults (CMP-38591)")
	supportRequestsCount := getSupportRequestCount(t)
	if supportRequestsCount < 3 {
		t.Skipf("Need at least 3 support requests to test pagination, got %d", supportRequestsCount)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccSupportRequestsDataSourceChainedMaxResultsAndPageToken(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_support_requests.second_page", "tickets.#", "1"),
				),
			},
		},
	})
}

func testAccSupportRequestsDataSourceChainedMaxResultsAndPageToken() string {
	return `
data "doit_support_requests" "first_page" {
  max_results = 1
}
data "doit_support_requests" "second_page" {
  max_results = 1
  page_token  = data.doit_support_requests.first_page.page_token
}
`
}

// TestAccSupportRequestsDataSource_AutoPagination tests that without max_results, all support requests are fetched.
func TestAccSupportRequestsDataSource_AutoPagination(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccSupportRequestsDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Just verify row_count is set and pagination completed (no page_token)
					// Don't check specific values since parallel tests may change the count
					resource.TestCheckResourceAttrSet("data.doit_support_requests.test", "row_count"),
					resource.TestCheckNoResourceAttr("data.doit_support_requests.test", "page_token"),
				),
			},
		},
	})
}

func testAccSupportRequestsDataSourceConfig() string {
	return `
data "doit_support_requests" "test" {
}
`
}

// Helper functions

var (
	supportRequestCount     int
	supportRequestCountOnce sync.Once
)

func getSupportRequestCount(t *testing.T) int {
	t.Helper()
	supportRequestCountOnce.Do(func() {
		supportRequestCount = computeSupportRequestCount(t)
	})
	return supportRequestCount
}

func computeSupportRequestCount(t *testing.T) int {
	t.Helper()
	client := getAPIClient(t)
	ctx := context.Background()

	var total int
	params := &models.IdOfTicketsParams{}
	for {
		resp, err := client.IdOfTicketsWithResponse(ctx, params)
		if err != nil {
			t.Fatalf("Failed to list support requests: %v", err)
		}
		if resp.JSON200 == nil || resp.JSON200.Tickets == nil {
			break
		}
		total += len(*resp.JSON200.Tickets)

		if resp.JSON200.PageToken == nil || *resp.JSON200.PageToken == "" {
			break
		}
		params.PageToken = resp.JSON200.PageToken
	}
	return total
}
