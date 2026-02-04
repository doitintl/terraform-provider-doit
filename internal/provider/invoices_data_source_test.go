package provider_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccInvoicesDataSource_MaxResultsOnly tests that setting max_results limits results.
func TestAccInvoicesDataSource_MaxResultsOnly(t *testing.T) {
	invoiceCount := getInvoiceCount(t)
	if invoiceCount < 3 {
		t.Skipf("Need at least 3 invoices to test pagination, got %d", invoiceCount)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccInvoicesDataSourceMaxResultsConfig(2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_invoices.limited", "invoices.#", "2"),
					resource.TestCheckResourceAttrSet("data.doit_invoices.limited", "page_token"),
				),
			},
			{
				Config:   testAccInvoicesDataSourceMaxResultsConfig(2),
				PlanOnly: true,
			},
		},
	})
}

func testAccInvoicesDataSourceMaxResultsConfig(maxResults int64) string {
	return fmt.Sprintf(`
data "doit_invoices" "limited" {
  max_results = %d
}
`, maxResults)
}

// TestAccInvoicesDataSource_PageTokenOnly tests using a page_token from a previous API call.
func TestAccInvoicesDataSource_PageTokenOnly(t *testing.T) {
	pageToken := getInvoiceFirstPageToken(t, 1)
	if pageToken == "" {
		t.Skip("No page_token returned (need more than 1 invoice)")
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccInvoicesDataSourcePageTokenConfig(pageToken),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_invoices.from_token", "invoices.#"),
				),
			},
		},
	})
}

func testAccInvoicesDataSourcePageTokenConfig(pageToken string) string {
	return fmt.Sprintf(`
data "doit_invoices" "from_token" {
  page_token = "%s"
}
`, pageToken)
}

// TestAccInvoicesDataSource_MaxResultsAndPageToken tests using both parameters together.
func TestAccInvoicesDataSource_MaxResultsAndPageToken(t *testing.T) {
	pageToken := getInvoiceFirstPageToken(t, 1)
	if pageToken == "" {
		t.Skip("No page_token returned (need more than 1 invoice)")
	}

	invoiceCount := getInvoiceCount(t)
	if invoiceCount < 3 {
		t.Skipf("Need at least 3 invoices to test pagination, got %d", invoiceCount)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccInvoicesDataSourceMaxResultsAndPageTokenConfig(1, pageToken),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_invoices.paginated", "invoices.#", "1"),
				),
			},
		},
	})
}

func testAccInvoicesDataSourceMaxResultsAndPageTokenConfig(maxResults int64, pageToken string) string {
	return fmt.Sprintf(`
data "doit_invoices" "paginated" {
  max_results = %d
  page_token  = "%s"
}
`, maxResults, pageToken)
}

// TestAccInvoicesDataSource_AutoPagination tests that without max_results, all invoices are fetched.
func TestAccInvoicesDataSource_AutoPagination(t *testing.T) {
	expectedCount := getInvoiceCount(t)
	if expectedCount == 0 {
		t.Skip("No invoices available to test auto-pagination")
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccInvoicesDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_invoices.test", "row_count", fmt.Sprintf("%d", expectedCount)),
					resource.TestCheckNoResourceAttr("data.doit_invoices.test", "page_token"),
				),
			},
		},
	})
}

func testAccInvoicesDataSourceConfig() string {
	return `
data "doit_invoices" "test" {
  sort_by    = "createTime"
  sort_order = "desc"
}
`
}

// Helper functions

func getInvoiceCount(t *testing.T) int {
	t.Helper()
	client := getAPIClient(t)
	ctx := context.Background()

	var total int
	params := &models.ListInvoicesParams{}
	for {
		resp, err := client.ListInvoicesWithResponse(ctx, params)
		if err != nil {
			t.Fatalf("Failed to list invoices: %v", err)
		}
		if resp.JSON200 == nil || resp.JSON200.Invoices == nil {
			break
		}
		total += len(*resp.JSON200.Invoices)

		if resp.JSON200.PageToken == nil || *resp.JSON200.PageToken == "" {
			break
		}
		params.PageToken = resp.JSON200.PageToken
	}
	return total
}

func getInvoiceFirstPageToken(t *testing.T, maxResults int64) string {
	t.Helper()
	client := getAPIClient(t)
	ctx := context.Background()

	resp, err := client.ListInvoicesWithResponse(ctx, &models.ListInvoicesParams{
		MaxResults: &maxResults,
	})
	if err != nil {
		t.Fatalf("Failed to list invoices: %v", err)
	}
	if resp.JSON200 == nil {
		t.Fatal("No response from API")
	}
	if resp.JSON200.PageToken == nil {
		return ""
	}
	return *resp.JSON200.PageToken
}
