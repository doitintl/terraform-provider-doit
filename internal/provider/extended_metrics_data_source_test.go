package provider_test

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const testAccExtendedMetricsDataSourceAddr = "data.doit_extended_metrics.test"

var (
	extendedMetricsRowCount     int
	extendedMetricsRowCountOnce sync.Once
)

// getExtendedMetricsRowCount returns the number of extended metrics the tenant
// has, so the pagination tests skip when there is nothing to page through.
func getExtendedMetricsRowCount(t *testing.T) int {
	t.Helper()
	skipIfNoAcc(t)
	extendedMetricsRowCountOnce.Do(func() {
		client := getAPIClient(t)
		resp, err := client.ListExtendedMetricsWithResponse(t.Context(), &models.ListExtendedMetricsParams{MaxResults: new(int64(1))})
		if err != nil {
			t.Fatalf("Failed to list extended metrics: %v", err)
		}
		if resp.JSON200 == nil {
			t.Fatalf("Failed to list extended metrics: status %d, body: %s", resp.StatusCode(), string(resp.Body))
		}
		if rowCount := resp.JSON200.RowCount; rowCount.IsSpecified() && !rowCount.IsNull() {
			extendedMetricsRowCount = int(rowCount.MustGet())
		} else {
			extendedMetricsRowCount = len(resp.JSON200.Items)
		}
	})
	return extendedMetricsRowCount
}

// testCheckExtendedMetricsDataSources verifies that row_count equals the number
// of items and that every item's data_source is one of allowed.
func testCheckExtendedMetricsDataSources(addr string, allowed ...string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[addr]
		if !ok {
			return fmt.Errorf("resource %s not found", addr)
		}
		attrs := rs.Primary.Attributes
		count, err := strconv.Atoi(attrs["items.#"])
		if err != nil {
			return fmt.Errorf("items.# = %q: %w", attrs["items.#"], err)
		}
		if attrs["row_count"] != strconv.Itoa(count) {
			return fmt.Errorf("row_count = %q, want %d (the number of items)", attrs["row_count"], count)
		}
		for i := range count {
			source := attrs[fmt.Sprintf("items.%d.data_source", i)]
			if !slices.Contains(allowed, source) {
				return fmt.Errorf("items.%d.data_source = %q, want one of %s", i, source, strings.Join(allowed, ", "))
			}
		}
		return nil
	}
}

func TestAccExtendedMetricsDataSource_List(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: `data "doit_extended_metrics" "test" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(testAccExtendedMetricsDataSourceAddr, "row_count"),
					resource.TestCheckResourceAttrSet(testAccExtendedMetricsDataSourceAddr, "items.0.key"),
					resource.TestCheckResourceAttrSet(testAccExtendedMetricsDataSourceAddr, "items.0.label"),
					resource.TestCheckResourceAttrSet(testAccExtendedMetricsDataSourceAddr, "items.0.data_source"),
					resource.TestCheckNoResourceAttr(testAccExtendedMetricsDataSourceAddr, "page_token"),
					testCheckExtendedMetricsDataSources(testAccExtendedMetricsDataSourceAddr,
						"billing", "billing-datahub", "bqlens", "genai-collection", "kubernetes-utilization", "tokenomics"),
				),
			},
			// Drift verification: re-apply the same config should produce an empty plan.
			{
				Config: `data "doit_extended_metrics" "test" {}`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestAccExtendedMetricsDataSource_Filter(t *testing.T) {
	const config = `
data "doit_extended_metrics" "test" {
  filter = "dataSource:billing,billing-datahub"
}
`
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(testAccExtendedMetricsDataSourceAddr, "filter", "dataSource:billing,billing-datahub"),
					resource.TestCheckResourceAttrSet(testAccExtendedMetricsDataSourceAddr, "items.0.key"),
					testCheckExtendedMetricsDataSources(testAccExtendedMetricsDataSourceAddr, "billing", "billing-datahub"),
				),
			},
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestAccExtendedMetricsDataSource_InvalidFilterKey(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_extended_metrics" "test" {
  filter = "label:cost"
}
`,
				ExpectError: regexp.MustCompile(`filter_key_not_supported`),
			},
		},
	})
}

func TestAccExtendedMetricsDataSource_Pagination(t *testing.T) {
	if n := getExtendedMetricsRowCount(t); n < 2 {
		t.Skipf("Need at least 2 extended metrics to test pagination, got %d", n)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_extended_metrics" "first_page" {
  max_results = 1
}

data "doit_extended_metrics" "from_token" {
  page_token = data.doit_extended_metrics.first_page.page_token
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_extended_metrics.first_page", "items.#", "1"),
					resource.TestCheckResourceAttr("data.doit_extended_metrics.first_page", "max_results", "1"),
					resource.TestCheckResourceAttrSet("data.doit_extended_metrics.first_page", "page_token"),
					resource.TestCheckResourceAttrSet("data.doit_extended_metrics.first_page", "row_count"),
					resource.TestCheckResourceAttrSet("data.doit_extended_metrics.from_token", "items.0.key"),
					resource.TestCheckNoResourceAttr("data.doit_extended_metrics.from_token", "max_results"),
					// The token advanced to a different starting point. Items are
					// ordered by label, then key, then data source, so the first
					// item of the second read cannot repeat the first page's.
					testCheckExtendedMetricsPageAdvanced(),
					// row_count is the total across pages for both reads.
					testCheckResourceAttrEqualAttr(
						"data.doit_extended_metrics.first_page", "row_count",
						"data.doit_extended_metrics.from_token", "row_count"),
				),
			},
		},
	})
}

// testCheckExtendedMetricsPageAdvanced fails when the read that started at the
// page token returns the same first item (key and data source) as the first page.
func testCheckExtendedMetricsPageAdvanced() resource.TestCheckFunc {
	return func(s *terraform.State) error {
		identity := func(addr string) (string, error) {
			rs, ok := s.RootModule().Resources[addr]
			if !ok {
				return "", fmt.Errorf("resource %s not found", addr)
			}
			return rs.Primary.Attributes["items.0.key"] + "@" + rs.Primary.Attributes["items.0.data_source"], nil
		}
		first, err := identity("data.doit_extended_metrics.first_page")
		if err != nil {
			return err
		}
		next, err := identity("data.doit_extended_metrics.from_token")
		if err != nil {
			return err
		}
		if first == next {
			return fmt.Errorf("page_token did not advance: both reads start at %s", first)
		}
		return nil
	}
}

func testCheckResourceAttrEqualAttr(name1, key1, name2, key2 string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs1, ok := s.RootModule().Resources[name1]
		if !ok {
			return fmt.Errorf("resource %s not found", name1)
		}
		rs2, ok := s.RootModule().Resources[name2]
		if !ok {
			return fmt.Errorf("resource %s not found", name2)
		}
		if v1, v2 := rs1.Primary.Attributes[key1], rs2.Primary.Attributes[key2]; v1 != v2 {
			return fmt.Errorf("%s.%s = %q, %s.%s = %q, want equal", name1, key1, v1, name2, key2, v2)
		}
		return nil
	}
}

// A page token is bound to the filter it was returned for: reusing it with
// another filter is rejected by the API.
func TestAccExtendedMetricsDataSource_PageTokenFilterMismatch(t *testing.T) {
	if n := getExtendedMetricsRowCount(t); n < 2 {
		t.Skipf("Need at least 2 extended metrics to get a page token, got %d", n)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_extended_metrics" "first_page" {
  max_results = 1
}

data "doit_extended_metrics" "other_filter" {
  filter     = "dataSource:billing"
  page_token = data.doit_extended_metrics.first_page.page_token
}
`,
				ExpectError: regexp.MustCompile(`pagination_token_invalid`),
			},
		},
	})
}
