package provider_test

import (
	"fmt"
	"strconv"
	"sync"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
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
					// anomaly_detection is a newly mapped list-item attribute. The
					// generated NewAllocationsValue constructor errors at runtime if
					// the mapping omits it, so assert it is populated.
					resource.TestCheckResourceAttrSet("data.doit_allocations.limited", "allocations.0.anomaly_detection"),
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
// auto-paginates starting from the token, returning a different set of items.
// Uses two chained data sources in one apply:
//   - "first_page": fetches 1 allocation + page_token
//   - "from_token": auto-paginates from the token (should start at a different item)
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
					// Verify page_token actually advanced to a different starting point
					testCheckResourceAttrNotEqualAttr(
						"data.doit_allocations.first_page", "allocations.0.id",
						"data.doit_allocations.from_token", "allocations.0.id"),
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
					// Verify page_token actually advanced to a different page
					testCheckResourceAttrNotEqualAttr(
						"data.doit_allocations.first_page", "allocations.0.id",
						"data.doit_allocations.paginated", "allocations.0.id"),
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

// TestAccAllocationsDataSource_NameContains tests filtering allocations with name_contains
// against both an existing allocation (via chained data source) and a non-existent name.
func TestAccAllocationsDataSource_NameContains(t *testing.T) {
	allocationCount := getAllocationCount(t)
	if allocationCount < 1 {
		t.Skipf("Need at least 1 allocation to test name_contains, got %d", allocationCount)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccAllocationsDataSourceNameContainsConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_allocations.first", "allocations.0.name"),
					resource.TestCheckResourceAttrSet("data.doit_allocations.filtered", "allocations.0.id"),
					resource.TestCheckResourceAttrSet("data.doit_allocations.filtered", "row_count"),
					resource.TestCheckResourceAttr("data.doit_allocations.empty", "row_count", "0"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.doit_allocations.empty", tfjsonpath.New("allocations"), knownvalue.ListExact([]knownvalue.Check{})),
				},
			},
			// Drift verification: re-apply the same config should produce an empty plan
			{
				Config: testAccAllocationsDataSourceNameContainsConfig(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccAllocationsDataSourceNameContainsConfig() string {
	return `
data "doit_allocations" "first" {
  max_results = "1"
}

data "doit_allocations" "filtered" {
  name_contains = data.doit_allocations.first.allocations.0.name
}

data "doit_allocations" "empty" {
  name_contains = "nonexistent-allocation-xyz-99999"
}
`
}

// TestAccAllocationsDataSource_SortByAndSortOrder tests that sort_by and
// sort_order reach the API: createTime is numeric, so the order is unambiguous.
func TestAccAllocationsDataSource_SortByAndSortOrder(t *testing.T) {
	if getAllocationCount(t) < 2 {
		t.Skip("Need at least 2 allocations to test sorting")
	}

	config := testAccAllocationsDataSourceSortConfig()
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAllocationsCreateTimeOrder("data.doit_allocations.asc", true),
					testAccCheckAllocationsCreateTimeOrder("data.doit_allocations.desc", false),
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

func testAccAllocationsDataSourceSortConfig() string {
	return `
data "doit_allocations" "asc" {
  sort_by    = "createTime"
  sort_order = "asc"
}

data "doit_allocations" "desc" {
  sort_by    = "createTime"
  sort_order = "desc"
}
`
}

// testAccCheckAllocationsCreateTimeOrder checks that the allocations of the data
// source are ordered by create_time, ascending or descending.
func testAccCheckAllocationsCreateTimeOrder(addr string, ascending bool) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[addr]
		if !ok {
			return fmt.Errorf("%s not found in state", addr)
		}
		n, err := strconv.Atoi(rs.Primary.Attributes["allocations.#"])
		if err != nil {
			return fmt.Errorf("%s: invalid allocations.#: %w", addr, err)
		}
		var prev int64
		for i := range n {
			cur, err := strconv.ParseInt(rs.Primary.Attributes[fmt.Sprintf("allocations.%d.create_time", i)], 10, 64)
			if err != nil {
				return fmt.Errorf("%s: invalid allocations.%d.create_time: %w", addr, i, err)
			}
			if i > 0 && ((ascending && cur < prev) || (!ascending && cur > prev)) {
				return fmt.Errorf("%s: allocations.%d.create_time %d breaks the order after %d (ascending=%t)", addr, i, cur, prev, ascending)
			}
			prev = cur
		}
		return nil
	}
}

// Helper functions

var (
	allocationCount     int
	allocationCountOnce sync.Once
)

func getAllocationCount(t *testing.T) int {
	t.Helper()
	skipIfNoAcc(t)
	allocationCountOnce.Do(func() {
		allocationCount = computeAllocationCount(t)
	})
	return allocationCount
}

func computeAllocationCount(t *testing.T) int {
	t.Helper()
	client := getAPIClient(t)
	ctx := t.Context()

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
