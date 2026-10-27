package provider_test

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
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
// auto-paginates starting from the token, returning fewer results than a full run.
func TestAccInvoicesDataSource_PageTokenOnly(t *testing.T) {
	totalInvoices := getInvoiceCount(t)
	if totalInvoices < 2 {
		t.Skipf("Need at least 2 invoices to test page_token-only, got %d", totalInvoices)
	}

	pageToken := getInvoiceFirstPageToken(t, 1)
	if pageToken == "" {
		t.Skip("No page_token returned (need more than 1 invoice)")
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccInvoicesDataSourcePageTokenConfig(pageToken),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_invoices.from_token", "invoices.#"),
					testCheckResourceAttrLessThan("data.doit_invoices.from_token", "row_count", totalInvoices),
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

	resource.ParallelTest(t, resource.TestCase{
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

// TestAccInvoicesDataSource_FilterCanceled tests that filtering by status:CANCELED
// returns only canceled invoices, including the known canceled test invoice.
func TestAccInvoicesDataSource_FilterCanceled(t *testing.T) {
	invoiceID := os.Getenv("TEST_INVOICE_ID_CANCELED")
	if invoiceID == "" {
		t.Skip("TEST_INVOICE_ID_CANCELED environment variable not set")
	}

	config := testAccInvoicesDataSourceFilterConfig("status:CANCELED")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					testCheckInvoicesAll("data.doit_invoices.filtered", func(id, status string) error {
						if status != "CANCELED" {
							return fmt.Errorf("invoice %s has status %q, expected CANCELED", id, status)
						}
						return nil
					}),
					testCheckInvoicesContain("data.doit_invoices.filtered", invoiceID, true),
				),
			},
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccInvoicesDataSource_DefaultExcludesCanceled tests that canceled invoices
// are only returned when explicitly requested via the status filter.
func TestAccInvoicesDataSource_DefaultExcludesCanceled(t *testing.T) {
	invoiceID := os.Getenv("TEST_INVOICE_ID_CANCELED")
	if invoiceID == "" {
		t.Skip("TEST_INVOICE_ID_CANCELED environment variable not set")
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccInvoicesDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					testCheckInvoicesAll("data.doit_invoices.test", func(id, status string) error {
						if status == "CANCELED" {
							return fmt.Errorf("invoice %s is CANCELED but was returned without a status filter", id)
						}
						return nil
					}),
					testCheckInvoicesContain("data.doit_invoices.test", invoiceID, false),
				),
			},
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccInvoicesDataSourceConfig(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccInvoicesDataSourceFilterConfig(filter string) string {
	return fmt.Sprintf(`
data "doit_invoices" "filtered" {
  filter = %q
}
`, filter)
}

// testCheckInvoicesAll runs check against the id and status of every invoice
// in the data source and fails if the list is empty.
func testCheckInvoicesAll(name string, check func(id, status string) error) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		attrs, err := invoicesAttrs(s, name)
		if err != nil {
			return err
		}
		n, _ := strconv.Atoi(attrs["invoices.#"])
		if n == 0 {
			return fmt.Errorf("%s returned no invoices", name)
		}
		for i := range n {
			id := attrs[fmt.Sprintf("invoices.%d.id", i)]
			status := attrs[fmt.Sprintf("invoices.%d.status", i)]
			if err := check(id, status); err != nil {
				return err
			}
		}
		return nil
	}
}

// testCheckInvoicesContain checks whether the invoice list includes invoiceID.
func testCheckInvoicesContain(name, invoiceID string, want bool) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		attrs, err := invoicesAttrs(s, name)
		if err != nil {
			return err
		}
		n, _ := strconv.Atoi(attrs["invoices.#"])
		found := false
		for i := range n {
			if attrs[fmt.Sprintf("invoices.%d.id", i)] == invoiceID {
				found = true
				break
			}
		}
		if found != want {
			return fmt.Errorf("%s: invoice %s present = %t, expected %t", name, invoiceID, found, want)
		}
		return nil
	}
}

func invoicesAttrs(s *terraform.State, name string) (map[string]string, error) {
	rs, ok := s.RootModule().Resources[name]
	if !ok {
		return nil, fmt.Errorf("resource not found: %s", name)
	}
	return rs.Primary.Attributes, nil
}

// Helper functions

var (
	invoiceCount     int
	invoiceCountOnce sync.Once
)

func getInvoiceCount(t *testing.T) int {
	t.Helper()
	skipIfNoAcc(t)
	invoiceCountOnce.Do(func() {
		invoiceCount = computeInvoiceCount(t)
	})
	return invoiceCount
}

func computeInvoiceCount(t *testing.T) int {
	t.Helper()
	client := getAPIClient(t)
	ctx := t.Context()

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
	skipIfNoAcc(t)
	client := getAPIClient(t)
	ctx := t.Context()

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
