package provider_test

import (
	"context"
	"fmt"
	"regexp"
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
// cloud_provider and that every returned item matches the filter.
func TestAccServiceQuotasDataSource_FilterByCloudProvider(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceQuotasDataSourceCloudProviderConfig("aws"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_service_quotas.by_provider", "row_count"),
					testCheckAllItemsCloudProvider("data.doit_service_quotas.by_provider", "aws"),
				),
			},
		},
	})
}

// testCheckAllItemsCloudProvider verifies every items.N.cloud_provider
// attribute in state equals the expected value, regardless of how many
// items are returned.
func testCheckAllItemsCloudProvider(name, expected string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("data source not found: %s", name)
		}

		const suffix = ".cloud_provider" // "items.N.cloud_provider"; excludes the root "cloud_provider" filter attribute
		found := false
		for key, val := range rs.Primary.Attributes {
			if len(key) > len(suffix) && key[len(key)-len(suffix):] == suffix {
				found = true
				if val != expected {
					return fmt.Errorf("expected %s to be %q, got %q", key, expected, val)
				}
			}
		}
		if !found {
			return fmt.Errorf("no items.*.cloud_provider attributes found on %s", name)
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

// TestAccServiceQuotasDataSource_EmptyResult tests that a filter matching no
// quotas returns an empty, non-null items list rather than erroring.
func TestAccServiceQuotasDataSource_EmptyResult(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceQuotasDataSourceCloudProviderConfig("gcp"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_service_quotas.by_provider", "items.#", "0"),
					resource.TestCheckResourceAttr("data.doit_service_quotas.by_provider", "row_count", "0"),
				),
			},
		},
	})
}

// TestAccServiceQuotasDataSource_MinUtilization tests filtering by
// min_utilization_percent.
func TestAccServiceQuotasDataSource_MinUtilization(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceQuotasDataSourceMinUtilizationConfig(90),
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

// Helper: count total service quotas for pagination tests.

var (
	serviceQuotaCount     int
	serviceQuotaCountOnce sync.Once
)

func getServiceQuotaCount(t *testing.T) int {
	t.Helper()
	serviceQuotaCountOnce.Do(func() {
		serviceQuotaCount = computeServiceQuotaCount(t)
	})
	return serviceQuotaCount
}

func computeServiceQuotaCount(t *testing.T) int {
	t.Helper()
	client := getAPIClient(t)
	ctx := context.Background()

	var total int
	params := &models.ListServiceQuotasParams{}
	for {
		resp, err := client.ListServiceQuotasWithResponse(ctx, params)
		if err != nil {
			t.Fatalf("Failed to list service quotas: %v", err)
		}
		if resp.JSON200 == nil {
			break
		}
		total += len(resp.JSON200.Items)

		if resp.JSON200.PageToken == nil || *resp.JSON200.PageToken == "" {
			break
		}
		params.PageToken = resp.JSON200.PageToken
	}
	return total
}
