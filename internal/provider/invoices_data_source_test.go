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

// TestAccInvoicesDataSource_MaxResultsOnly tests that setting max_results limits results.
func TestAccInvoicesDataSource_MaxResultsOnly(t *testing.T) {
	invoiceCount := getInvoiceCount(t)
	if invoiceCount < 3 {
		t.Skipf("Need at least 3 invoices to test pagination, got %d", invoiceCount)
	}

	resource.ParallelTest(t, resource.TestCase{
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
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccInvoicesDataSourceMaxResultsConfig(2),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
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

// TestAccInvoicesDataSource_PageTokenOnly tests that setting only page_token (without max_results)
// auto-paginates starting from the token, returning results.
// Uses chained data sources to avoid page token expiry.
func TestAccInvoicesDataSource_PageTokenOnly(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccInvoicesDataSourceChainedPageTokenOnly(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_invoices.from_token", "invoices.#"),
					resource.TestCheckResourceAttrSet("data.doit_invoices.from_token", "row_count"),
					resource.TestCheckNoResourceAttr("data.doit_invoices.from_token", "page_token"),
				),
			},
		},
	})
}

func testAccInvoicesDataSourceChainedPageTokenOnly() string {
	return `
data "doit_invoices" "first_page" {
  max_results = 1
}
data "doit_invoices" "from_token" {
  page_token = data.doit_invoices.first_page.page_token
}
`
}

// TestAccInvoicesDataSource_MaxResultsAndPageToken tests using both parameters together.
// Uses chained data sources to avoid page token expiry.
func TestAccInvoicesDataSource_MaxResultsAndPageToken(t *testing.T) {
	invoicesCount := getInvoiceCount(t)
	if invoicesCount < 3 {
		t.Skipf("Need at least 3 invoices to test pagination, got %d", invoicesCount)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccInvoicesDataSourceChainedMaxResultsAndPageToken(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_invoices.second_page", "invoices.#", "1"),
				),
			},
		},
	})
}

func testAccInvoicesDataSourceChainedMaxResultsAndPageToken() string {
	return `
data "doit_invoices" "first_page" {
  max_results = 1
}
data "doit_invoices" "second_page" {
  max_results = 1
  page_token  = data.doit_invoices.first_page.page_token
}
`
}

// TestAccInvoicesDataSource_AutoPagination tests that without max_results, all invoices are fetched.
func TestAccInvoicesDataSource_AutoPagination(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccInvoicesDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Just verify row_count is set and pagination completed (no page_token)
					// Don't check specific values since parallel tests may change the count
					resource.TestCheckResourceAttrSet("data.doit_invoices.test", "row_count"),
					resource.TestCheckNoResourceAttr("data.doit_invoices.test", "page_token"),
				),
			},
		},
	})
}

func testAccInvoicesDataSourceConfig() string {
	return `
data "doit_invoices" "test" {
}
`
}

// Helper functions

var (
	invoiceCount     int
	invoiceCountOnce sync.Once
)

func getInvoiceCount(t *testing.T) int {
	t.Helper()
	invoiceCountOnce.Do(func() {
		invoiceCount = computeInvoiceCount(t)
	})
	return invoiceCount
}

func computeInvoiceCount(t *testing.T) int {
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
