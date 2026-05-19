package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// TestAccInsightResourceResultsDataSource_Basic creates an insight with resource
// results, then reads the results back through the data source.
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
					resource.TestCheckResourceAttrSet("data.doit_insight_resource_results.test", "resource_results.#"),
					resource.TestCheckResourceAttrSet("data.doit_insight_resource_results.test", "row_count"),
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
					resource.TestCheckResourceAttrSet("data.doit_insight_resource_results.test", "row_count"),
				),
			},
		},
	})
}

// TestAccInsightResourceResultsDataSource_NestedFields verifies nested objects
// (result, enhancement) are populated correctly.
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
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.account", "test-project-123"),
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.cloud_provider", "gcp"),
					resource.TestCheckResourceAttr("data.doit_insight_resource_results.test", "resource_results.0.result_type", "potential_daily_savings"),
					resource.TestCheckResourceAttrSet("data.doit_insight_resource_results.test", "resource_results.0.result.value"),
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
      resource_id    = "test-vm-1"
      account        = "test-project-123"
      cloud_provider = "gcp"
      result_type    = "potential_daily_savings"
      result = {
        value = 5.50
      }
    },
    {
      resource_id    = "test-vm-2"
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
      resource_id    = "i-abc001"
      account        = "111222333444"
      cloud_provider = "aws"
      result_type    = "potential_daily_savings"
      result = {
        value = 10.0
      }
    },
    {
      resource_id    = "i-abc002"
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
      resource_id    = "test-vm-nested"
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
