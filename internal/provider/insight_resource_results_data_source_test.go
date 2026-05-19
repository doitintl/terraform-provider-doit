package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// TestAccInsightResourceResultsDataSource_Basic creates an insight with resource
// results, then reads them back through the data source with full attribute coverage.
func TestAccInsightResourceResultsDataSource_Basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-rr-ds")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccInsightResourceResultsDataSourceBasicConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Top-level data source attributes
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "insight_key", rName),
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "source_id", "public-api"),
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "row_count", "2"),

					// List count
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "resource_results.#", "2"),

					// First result — full attribute coverage
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.resource_id", fmt.Sprintf("i-ds-%s-1", rName)),
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.account", "test-project-123"),
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.cloud_provider", "gcp"),
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.result_type", "potential_daily_savings"),
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.resolved", "false"),
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.result.value", "5.5"),
					// Null number/object fields in result sub-object are absent
					resource.TestCheckNoResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.result.critical"),
					resource.TestCheckNoResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.result.high"),
					resource.TestCheckNoResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.result.medium"),
					resource.TestCheckNoResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.result.low"),
					resource.TestCheckNoResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.result.agent_installed"),
					resource.TestCheckNoResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.result.current"),
					resource.TestCheckNoResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.result.recommendation"),
					// Optional string fields not set in config — still present as empty in nested objects
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.external_id", ""),
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.external_url", ""),
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.location", ""),
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.resource_type", ""),

					// Second result
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "resource_results.1.resource_id", fmt.Sprintf("i-ds-%s-2", rName)),
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "resource_results.1.account", "test-project-456"),
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "resource_results.1.cloud_provider", "gcp"),
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "resource_results.1.result_type", "potential_daily_savings"),
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "resource_results.1.result.value", "3.25"),
				),
			},
			// Drift verification
			{
				Config: testAccInsightResourceResultsDataSourceBasicConfig(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccInsightResourceResultsDataSource_MaxResults tests manual pagination.
func TestAccInsightResourceResultsDataSource_MaxResults(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-rr-ds-pag")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccInsightResourceResultsDataSourceMaxResultsConfig(rName, 1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "resource_results.#", "1"),
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "max_results", "1"),
					resource.TestCheckResourceAttrSet("data.doit_insight_resource_results.test", "row_count"),
					// First result — full field check
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.resource_id", fmt.Sprintf("i-ds-pag-%s-1", rName)),
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.account", "111222333444"),
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.cloud_provider", "aws"),
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.result_type", "potential_daily_savings"),
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.result.value", "10"),
				),
			},
		},
	})
}

// TestAccInsightResourceResultsDataSource_NestedFields verifies nested objects
// (result, enhancement) are populated correctly via the data source.
func TestAccInsightResourceResultsDataSource_NestedFields(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-rr-ds-nest")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccInsightResourceResultsDataSourceNestedConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Top-level
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "resource_results.#", "1"),
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.resource_id", fmt.Sprintf("i-ds-nest-%s", rName)),
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.account", "test-project-123"),
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.cloud_provider", "gcp"),
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.result_type", "potential_daily_savings"),
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.resolved", "false"),

					// Result sub-object — full field checks
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.result.value", "12.5"),
					resource.TestCheckNoResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.result.critical"),
					resource.TestCheckNoResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.result.high"),
					resource.TestCheckNoResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.result.medium"),
					resource.TestCheckNoResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.result.low"),
					resource.TestCheckNoResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.result.agent_installed"),
					resource.TestCheckNoResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.result.current"),
					resource.TestCheckNoResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.result.recommendation"),

					// Optional string fields — present as empty in nested objects
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.external_id", ""),
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.external_url", ""),
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.location", ""),
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.resource_type", ""),
				),
			},
		},
	})
}

func testAccInsightResourceResultsDataSourceBasicConfig(rName string) string {
	return fmt.Sprintf(`
resource "doit_insight" "test" {
  key               = %[1]q
  title             = "RR DS Test"
  short_description = "Resource results data source test"
  cloud_provider    = "gcp"
  categories        = ["FinOps"]
}

resource "doit_insight_resource_results" "test" {
  source_id   = doit_insight.test.source_id
  insight_key = doit_insight.test.key
  resource_results = [
    {
      resource_id    = "i-ds-%[1]s-1"
      account        = "test-project-123"
      cloud_provider = "gcp"
      result_type    = "potential_daily_savings"
      result = {
        value = 5.50
      }
    },
    {
      resource_id    = "i-ds-%[1]s-2"
      account        = "test-project-456"
      cloud_provider = "gcp"
      result_type    = "potential_daily_savings"
      result = {
        value = 3.25
      }
    },
  ]
}

data "doit_insight_resource_results" "test" {
  source_id   = doit_insight.test.source_id
  insight_key = doit_insight.test.key
  depends_on  = [doit_insight_resource_results.test]
}
`, rName)
}

func testAccInsightResourceResultsDataSourceMaxResultsConfig(rName string, maxResults int) string {
	return fmt.Sprintf(`
resource "doit_insight" "test" {
  key               = %[1]q
  title             = "RR DS Pagination Test"
  short_description = "Resource results pagination test"
  cloud_provider    = "aws"
  categories        = ["FinOps"]
}

resource "doit_insight_resource_results" "test" {
  source_id   = doit_insight.test.source_id
  insight_key = doit_insight.test.key
  resource_results = [
    {
      resource_id    = "i-ds-pag-%[1]s-1"
      account        = "111222333444"
      cloud_provider = "aws"
      result_type    = "potential_daily_savings"
      result = {
        value = 10.0
      }
    },
    {
      resource_id    = "i-ds-pag-%[1]s-2"
      account        = "111222333444"
      cloud_provider = "aws"
      result_type    = "potential_daily_savings"
      result = {
        value = 7.50
      }
    },
  ]
}

data "doit_insight_resource_results" "test" {
  source_id   = doit_insight.test.source_id
  insight_key = doit_insight.test.key
  max_results = %[2]d
  depends_on  = [doit_insight_resource_results.test]
}
`, rName, maxResults)
}

func testAccInsightResourceResultsDataSourceNestedConfig(rName string) string {
	return fmt.Sprintf(`
resource "doit_insight" "test" {
  key               = %[1]q
  title             = "RR DS Nested Test"
  short_description = "Resource results nested fields test"
  cloud_provider    = "gcp"
  categories        = ["FinOps"]
}

resource "doit_insight_resource_results" "test" {
  source_id   = doit_insight.test.source_id
  insight_key = doit_insight.test.key
  resource_results = [
    {
      resource_id    = "i-ds-nest-%[1]s"
      account        = "test-project-123"
      cloud_provider = "gcp"
      result_type    = "potential_daily_savings"
      result = {
        value = 12.50
      }
    },
  ]
}

data "doit_insight_resource_results" "test" {
  source_id   = doit_insight.test.source_id
  insight_key = doit_insight.test.key
  depends_on  = [doit_insight_resource_results.test]
}
`, rName)
}
