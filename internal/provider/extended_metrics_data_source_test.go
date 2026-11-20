package provider_test

import (
	"context"
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

// extendedMetricsProbe is what a short read of the collection says about it.
type extendedMetricsProbe struct {
	// count is the API's row count, or the items read when the API omits it.
	count int
	// apiTotal reports whether count is a total the API supplied.
	apiTotal bool
}

var (
	extendedMetricsProbeResult extendedMetricsProbe
	extendedMetricsProbeOnce   sync.Once
)

// getExtendedMetricsProbe probes the tenant once, so the pagination tests skip
// when there is nothing to page through and know whether row_count is a total.
func getExtendedMetricsProbe(t *testing.T) extendedMetricsProbe {
	t.Helper()
	skipIfNoAcc(t)
	extendedMetricsProbeOnce.Do(func() {
		probe, err := probeExtendedMetrics(t.Context(), getAPIClient(t))
		if err != nil {
			t.Fatalf("Failed to list extended metrics: %v", err)
		}
		extendedMetricsProbeResult = probe
	})
	return extendedMetricsProbeResult
}

// probeExtendedMetrics reads two items and returns the API's row count. When
// the API omits it, the items read stand in for it: two is enough for the
// pagination tests, which only need to know that there are at least two.
func probeExtendedMetrics(ctx context.Context, client *models.ClientWithResponses) (extendedMetricsProbe, error) {
	resp, err := client.ListExtendedMetricsWithResponse(ctx, &models.ListExtendedMetricsParams{MaxResults: new(int64(2))})
	if err != nil {
		return extendedMetricsProbe{}, err
	}
	if resp.JSON200 == nil {
		return extendedMetricsProbe{}, fmt.Errorf("status %d, body: %s", resp.StatusCode(), string(resp.Body))
	}
	if rowCount := resp.JSON200.RowCount; rowCount.IsSpecified() && !rowCount.IsNull() {
		return extendedMetricsProbe{count: int(rowCount.MustGet()), apiTotal: true}, nil
	}
	return extendedMetricsProbe{count: len(resp.JSON200.Items)}, nil
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
	if n := getExtendedMetricsProbe(t).count; n < 2 {
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
					testCheckExtendedMetricsPaginationRowCounts(getExtendedMetricsProbe(t).apiTotal),
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

// checkExtendedMetricsPaginationRowCounts compares the row_count and the number
// of items of the first page and of the read from its page token. A total from
// the API is the same for both reads; without one, row_count is the number of
// items each read fetched.
func checkExtendedMetricsPaginationRowCounts(firstRowCount, firstItems, tokenRowCount, tokenItems string, apiTotal bool) error {
	if apiTotal {
		if firstRowCount != tokenRowCount {
			return fmt.Errorf("row_count of the first page = %q, of the read from the token = %q, want the same total", firstRowCount, tokenRowCount)
		}
		return nil
	}
	if firstRowCount != firstItems {
		return fmt.Errorf("row_count of the first page = %q, want its %s items", firstRowCount, firstItems)
	}
	if tokenRowCount != tokenItems {
		return fmt.Errorf("row_count of the read from the token = %q, want its %s items", tokenRowCount, tokenItems)
	}
	return nil
}

func testCheckExtendedMetricsPaginationRowCounts(apiTotal bool) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		first, ok := s.RootModule().Resources["data.doit_extended_metrics.first_page"]
		if !ok {
			return fmt.Errorf("resource data.doit_extended_metrics.first_page not found")
		}
		token, ok := s.RootModule().Resources["data.doit_extended_metrics.from_token"]
		if !ok {
			return fmt.Errorf("resource data.doit_extended_metrics.from_token not found")
		}
		f, t := first.Primary.Attributes, token.Primary.Attributes
		return checkExtendedMetricsPaginationRowCounts(f["row_count"], f["items.#"], t["row_count"], t["items.#"], apiTotal)
	}
}

// A page token is bound to the filter it was returned for: reusing it with
// another filter is rejected by the API.
func TestAccExtendedMetricsDataSource_PageTokenFilterMismatch(t *testing.T) {
	if n := getExtendedMetricsProbe(t).count; n < 2 {
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
