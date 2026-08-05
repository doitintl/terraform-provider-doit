package provider_test

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"sync"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccServiceQuotasDataSource_AutoPagination tests that without max_results,
// all service quotas are fetched via auto-pagination.
func TestAccServiceQuotasDataSource_AutoPagination(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceQuotasDataSourceAutoConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_service_quotas.test", "row_count"),
					resource.TestCheckResourceAttrSet("data.doit_service_quotas.test", "items.#"),
				),
			},
			// Drift verification
			{
				Config: testAccServiceQuotasDataSourceAutoConfig(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccServiceQuotasDataSourceAutoConfig() string {
	return `
data "doit_service_quotas" "test" {
}
`
}

// TestAccServiceQuotasDataSource_FilterByCloudProvider tests filtering by
// cloud_provider for both supported values, asserting that every returned
// item matches the filter. Deliberately does not assert on how many items
// each provider returns — the test tenant's mix of aws/gcp quotas can
// change over time, and a filter can legitimately match zero items.
func TestAccServiceQuotasDataSource_FilterByCloudProvider(t *testing.T) {
	for _, cloudProvider := range []string{"aws", "gcp"} {
		t.Run(cloudProvider, func(t *testing.T) {
			resource.ParallelTest(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
				PreCheck:                 testAccPreCheckFunc(t),
				TerraformVersionChecks:   testAccTFVersionChecks,
				Steps: []resource.TestStep{
					{
						Config: testAccServiceQuotasDataSourceCloudProviderConfig(cloudProvider),
						Check: resource.ComposeAggregateTestCheckFunc(
							resource.TestCheckResourceAttrSet("data.doit_service_quotas.by_provider", "row_count"),
							testCheckAllItemsCloudProvider("data.doit_service_quotas.by_provider", cloudProvider),
						),
					},
				},
			})
		})
	}
}

// testCheckAllItemsCloudProvider verifies every items.N.cloud_provider
// attribute in state equals the expected value. Zero matching items is a
// valid outcome (vacuously satisfies the check) since a filter may
// legitimately match nothing; the empty-list-not-null mapping invariant
// itself is covered separately by TestMapServiceQuotasToModel_EmptyReturnsNonNullList.
func testCheckAllItemsCloudProvider(name, expected string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("data source not found: %s", name)
		}

		const suffix = ".cloud_provider" // "items.N.cloud_provider"; excludes the root "cloud_provider" filter attribute
		for key, val := range rs.Primary.Attributes {
			if len(key) > len(suffix) && key[len(key)-len(suffix):] == suffix {
				if val != expected {
					return fmt.Errorf("expected %s to be %q, got %q", key, expected, val)
				}
			}
		}
		return nil
	}
}

func testAccServiceQuotasDataSourceCloudProviderConfig(provider string) string {
	return fmt.Sprintf(`
data "doit_service_quotas" "by_provider" {
  cloud_provider = %[1]q
}
`, provider)
}

// TestAccServiceQuotasDataSource_MinUtilization tests filtering by
// min_utilization_percent. Thresholds are derived from the tenant's actual
// max observed utilization rather than a hardcoded value, so the test
// verifies real inclusion/exclusion behavior instead of asserting a fixed
// "currently empty" snapshot that would break the moment tenant data changes.
func TestAccServiceQuotasDataSource_MinUtilization(t *testing.T) {
	maxUtil, ok := getMaxUtilizationPercent(t)
	if !ok {
		t.Skip("No service quotas in tenant to test utilization filtering")
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				// Filtering at the tenant's max observed utilization must include
				// at least the item(s) that produced that max.
				Config: testAccServiceQuotasDataSourceMinUtilizationConfig(maxUtil),
				Check: resource.ComposeAggregateTestCheckFunc(
					testCheckResourceAttrIntNotEqual("data.doit_service_quotas.by_utilization", "row_count", "0"),
					testCheckAllItemsUtilizationAtLeast("data.doit_service_quotas.by_utilization", maxUtil),
				),
			},
		},
	})
}

// TestAccServiceQuotasDataSource_MinUtilization_Excludes verifies that a
// threshold just above the tenant's max observed utilization excludes every
// quota. Only meaningful when maxUtil < 100 (otherwise no valid "higher"
// threshold exists to test exclusion with).
func TestAccServiceQuotasDataSource_MinUtilization_Excludes(t *testing.T) {
	maxUtil, ok := getMaxUtilizationPercent(t)
	if !ok {
		t.Skip("No service quotas in tenant to test utilization filtering")
	}
	if maxUtil >= 100 {
		t.Skipf("Max observed utilization is already %g; no valid higher threshold to test exclusion with", maxUtil)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceQuotasDataSourceMinUtilizationConfig(math.Min(maxUtil+1, 100)),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_service_quotas.by_utilization", "items.#", "0"),
					resource.TestCheckResourceAttr("data.doit_service_quotas.by_utilization", "row_count", "0"),
				),
			},
		},
	})
}

func testAccServiceQuotasDataSourceMinUtilizationConfig(minUtilizationPercent float64) string {
	return fmt.Sprintf(`
data "doit_service_quotas" "by_utilization" {
  min_utilization_percent = %g
}
`, minUtilizationPercent)
}

// testCheckAllItemsUtilizationAtLeast verifies every items.N.utilization_percent
// attribute in state is >= the given threshold.
func testCheckAllItemsUtilizationAtLeast(name string, threshold float64) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("data source not found: %s", name)
		}

		const suffix = ".utilization_percent"
		found := false
		for key, val := range rs.Primary.Attributes {
			if len(key) > len(suffix) && key[len(key)-len(suffix):] == suffix {
				found = true
				f, err := strconv.ParseFloat(val, 64)
				if err != nil {
					return fmt.Errorf("failed to parse %s value %q as float: %w", key, val, err)
				}
				if f < threshold {
					return fmt.Errorf("expected %s (%g) to be >= %g", key, f, threshold)
				}
			}
		}
		if !found {
			return fmt.Errorf("no items.*.utilization_percent attributes found on %s", name)
		}
		return nil
	}
}

// testCheckResourceAttrIntNotEqual verifies a resource attribute exists and
// does not equal the given value.
func testCheckResourceAttrIntNotEqual(name, key, notValue string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("data source not found: %s", name)
		}
		val, ok := rs.Primary.Attributes[key]
		if !ok {
			return fmt.Errorf("attribute %s not found on %s", key, name)
		}
		if val == notValue {
			return fmt.Errorf("expected %s.%s to differ from %q, got %q", name, key, notValue, val)
		}
		return nil
	}
}

// TestAccServiceQuotasDataSource_MaxResults tests that setting max_results
// limits results and preserves the page_token returned by the API.
func TestAccServiceQuotasDataSource_MaxResults(t *testing.T) {
	quotaCount := getServiceQuotaCount(t)
	if quotaCount < 2 {
		t.Skipf("Need at least 2 service quotas to test pagination, got %d", quotaCount)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceQuotasDataSourceMaxResultsConfig(1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_service_quotas.limited", "items.#", "1"),
					resource.TestCheckResourceAttrSet("data.doit_service_quotas.limited", "page_token"),
				),
			},
			// Drift verification
			{
				Config: testAccServiceQuotasDataSourceMaxResultsConfig(1),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccServiceQuotasDataSourceMaxResultsConfig(maxResults int64) string {
	return fmt.Sprintf(`
data "doit_service_quotas" "limited" {
  max_results = %d
}
`, maxResults)
}

// TestAccServiceQuotasDataSource_PageTokenOnly tests that setting only
// page_token (without max_results) auto-paginates starting from the token,
// returning a different set of items. Mirrors
// TestAccReportsDataSource_PageTokenOnly.
func TestAccServiceQuotasDataSource_PageTokenOnly(t *testing.T) {
	quotaCount := getServiceQuotaCount(t)
	if quotaCount < 2 {
		t.Skipf("Need at least 2 service quotas to test page_token-only, got %d", quotaCount)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_service_quotas" "first_page" {
  max_results = 1
}
data "doit_service_quotas" "from_token" {
  page_token = data.doit_service_quotas.first_page.page_token
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_service_quotas.first_page", "items.#", "1"),
					resource.TestCheckResourceAttrSet("data.doit_service_quotas.first_page", "page_token"),
					resource.TestCheckResourceAttrSet("data.doit_service_quotas.from_token", "row_count"),
					resource.TestCheckNoResourceAttr("data.doit_service_quotas.from_token", "page_token"),
					// Verify page_token actually advanced to a different starting point
					testCheckResourceAttrNotEqualAttr(
						"data.doit_service_quotas.first_page", "items.0.resource.id",
						"data.doit_service_quotas.from_token", "items.0.resource.id"),
				),
			},
		},
	})
}

// TestAccServiceQuotasDataSource_MaxResultsAndPageToken tests using both
// parameters together. Mirrors TestAccReportsDataSource_MaxResultsAndPageToken.
func TestAccServiceQuotasDataSource_MaxResultsAndPageToken(t *testing.T) {
	quotaCount := getServiceQuotaCount(t)
	if quotaCount < 3 {
		t.Skipf("Need at least 3 service quotas to test max_results+page_token, got %d", quotaCount)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_service_quotas" "first_page" {
  max_results = 1
}
data "doit_service_quotas" "paginated" {
  max_results = 1
  page_token  = data.doit_service_quotas.first_page.page_token
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_service_quotas.first_page", "items.#", "1"),
					resource.TestCheckResourceAttrSet("data.doit_service_quotas.first_page", "page_token"),
					resource.TestCheckResourceAttr("data.doit_service_quotas.paginated", "items.#", "1"),
					// Verify page_token actually advanced to a different page
					testCheckResourceAttrNotEqualAttr(
						"data.doit_service_quotas.first_page", "items.0.resource.id",
						"data.doit_service_quotas.paginated", "items.0.resource.id"),
				),
			},
		},
	})
}

// TestAccServiceQuotasDataSource_InvalidCloudProvider tests that an invalid
// cloud_provider value is rejected at plan time by the generated OneOf
// validator, without making an API call.
func TestAccServiceQuotasDataSource_InvalidCloudProvider(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_service_quotas" "invalid" {
  cloud_provider = "azure"
}
`,
				ExpectError: regexp.MustCompile(`(?i)value must be one of:`),
			},
		},
	})
}

// TestAccServiceQuotasDataSource_InvalidMaxResults tests that a max_results
// value outside [1, 200] is rejected at plan time by the generated Between
// validator, without making an API call.
func TestAccServiceQuotasDataSource_InvalidMaxResults(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_service_quotas" "invalid" {
  max_results = 500
}
`,
				ExpectError: regexp.MustCompile(`(?i)value must be between 1 and 200`),
			},
		},
	})
}

// Helper: fetch every service quota once, cached, for pagination and
// utilization-threshold tests.

var (
	serviceQuotaSnapshot     []models.ServiceQuota
	serviceQuotaSnapshotOnce sync.Once
)

func getServiceQuotaSnapshot(t *testing.T) []models.ServiceQuota {
	t.Helper()
	skipIfNoAcc(t)
	serviceQuotaSnapshotOnce.Do(func() {
		serviceQuotaSnapshot = computeServiceQuotaSnapshot(t)
	})
	return serviceQuotaSnapshot
}

func computeServiceQuotaSnapshot(t *testing.T) []models.ServiceQuota {
	t.Helper()
	client := getAPIClient(t)
	ctx := context.Background()

	var all []models.ServiceQuota
	params := &models.ListServiceQuotasParams{}
	for {
		resp, err := client.ListServiceQuotasWithResponse(ctx, params)
		if err != nil {
			t.Fatalf("Failed to list service quotas: %v", err)
		}
		if resp.StatusCode() != 200 || resp.JSON200 == nil {
			t.Fatalf("Failed to list service quotas: status %d: %s", resp.StatusCode(), string(resp.Body))
		}
		all = append(all, resp.JSON200.Items...)

		if resp.JSON200.PageToken == nil || *resp.JSON200.PageToken == "" {
			break
		}
		params.PageToken = resp.JSON200.PageToken
	}
	return all
}

func getServiceQuotaCount(t *testing.T) int {
	t.Helper()
	return len(getServiceQuotaSnapshot(t))
}

// getMaxUtilizationPercent returns the highest utilization_percent among all
// service quotas in the tenant, and false if there are none.
func getMaxUtilizationPercent(t *testing.T) (float64, bool) {
	t.Helper()
	snapshot := getServiceQuotaSnapshot(t)
	if len(snapshot) == 0 {
		return 0, false
	}
	maxUtil := snapshot[0].UtilizationPercent
	for _, q := range snapshot[1:] {
		if q.UtilizationPercent > maxUtil {
			maxUtil = q.UtilizationPercent
		}
	}
	return maxUtil, true
}
