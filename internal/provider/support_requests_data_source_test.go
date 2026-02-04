package provider_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccSupportRequestsDataSource_MaxResultsOnly tests that setting max_results limits results.
func TestAccSupportRequestsDataSource_MaxResultsOnly(t *testing.T) {
	ticketCount := getSupportRequestCount(t)
	if ticketCount < 3 {
		t.Skipf("Need at least 3 support requests to test pagination, got %d", ticketCount)
	}

	resource.Test(t, resource.TestCase{
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

// TestAccSupportRequestsDataSource_PageTokenOnly tests using a page_token from a previous API call.
func TestAccSupportRequestsDataSource_PageTokenOnly(t *testing.T) {
	pageToken := getSupportRequestFirstPageToken(t, 1)
	if pageToken == "" {
		t.Skip("No page_token returned (need more than 1 support request)")
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccSupportRequestsDataSourcePageTokenConfig(pageToken),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_support_requests.from_token", "tickets.#"),
				),
			},
		},
	})
}

func testAccSupportRequestsDataSourcePageTokenConfig(pageToken string) string {
	return fmt.Sprintf(`
data "doit_support_requests" "from_token" {
  page_token = "%s"
}
`, pageToken)
}

// TestAccSupportRequestsDataSource_MaxResultsAndPageToken tests using both parameters together.
func TestAccSupportRequestsDataSource_MaxResultsAndPageToken(t *testing.T) {
	pageToken := getSupportRequestFirstPageToken(t, 1)
	if pageToken == "" {
		t.Skip("No page_token returned (need more than 1 support request)")
	}

	ticketCount := getSupportRequestCount(t)
	if ticketCount < 3 {
		t.Skipf("Need at least 3 support requests to test pagination, got %d", ticketCount)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccSupportRequestsDataSourceMaxResultsAndPageTokenConfig(1, pageToken),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_support_requests.paginated", "tickets.#", "1"),
				),
			},
		},
	})
}

func testAccSupportRequestsDataSourceMaxResultsAndPageTokenConfig(maxResults int64, pageToken string) string {
	return fmt.Sprintf(`
data "doit_support_requests" "paginated" {
  max_results = %d
  page_token  = "%s"
}
`, maxResults, pageToken)
}

// TestAccSupportRequestsDataSource_AutoPagination tests that without max_results, all support requests are fetched.
func TestAccSupportRequestsDataSource_AutoPagination(t *testing.T) {
	expectedCount := getSupportRequestCount(t)
	if expectedCount == 0 {
		t.Skip("No support requests available to test auto-pagination")
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccSupportRequestsDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_support_requests.test", "row_count", fmt.Sprintf("%d", expectedCount)),
					resource.TestCheckNoResourceAttr("data.doit_support_requests.test", "page_token"),
				),
			},
		},
	})
}

func testAccSupportRequestsDataSourceConfig() string {
	return `
data "doit_support_requests" "test" {}
`
}

// Helper functions

func getSupportRequestCount(t *testing.T) int {
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

func getSupportRequestFirstPageToken(t *testing.T, maxResults int64) string {
	t.Helper()
	client := getAPIClient(t)
	ctx := context.Background()

	resp, err := client.IdOfTicketsWithResponse(ctx, &models.IdOfTicketsParams{
		MaxResults: &maxResults,
	})
	if err != nil {
		t.Fatalf("Failed to list support requests: %v", err)
	}
	if resp.JSON200 == nil {
		t.Fatal("No response from API")
	}
	if resp.JSON200.PageToken == nil {
		return ""
	}
	return *resp.JSON200.PageToken
}
