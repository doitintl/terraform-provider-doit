package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestAccInsightDataSource_Basic creates an insight via the resource, then reads
// it back through the data source and verifies field mapping.
func TestAccInsightDataSource_Basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-insight-ds")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccInsightDataSourceBasicConfig(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.doit_insight.test",
						tfjsonpath.New("key"),
						knownvalue.StringExact(rName)),
					statecheck.ExpectKnownValue(
						"data.doit_insight.test",
						tfjsonpath.New("title"),
						knownvalue.StringExact("DS Test Insight")),
					statecheck.ExpectKnownValue(
						"data.doit_insight.test",
						tfjsonpath.New("short_description"),
						knownvalue.StringExact("A test insight for data source")),
					statecheck.ExpectKnownValue(
						"data.doit_insight.test",
						tfjsonpath.New("source"),
						knownvalue.StringExact("public-api")),
					statecheck.ExpectKnownValue(
						"data.doit_insight.test",
						tfjsonpath.New("display_status"),
						knownvalue.NotNull()),
					statecheck.ExpectKnownValue(
						"data.doit_insight.test",
						tfjsonpath.New("categories"),
						knownvalue.ListSizeExact(1)),
				},
			},
			// Drift verification
			{
				Config: testAccInsightDataSourceBasicConfig(rName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccInsightDataSource_NestedObjects verifies nested computed objects
// (summary, last_status_change) are populated.
func TestAccInsightDataSource_NestedObjects(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-insight-ds-nested")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccInsightDataSourceNestedConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_insight.test", "last_updated"),
					resource.TestCheckResourceAttrSet("data.doit_insight.test", "summary.potential_daily_savings"),
				),
			},
		},
	})
}

func testAccInsightDataSourceBasicConfig(rName string) string {
	return fmt.Sprintf(`
resource "doit_insight" "test" {
  key               = %[1]q
  title             = "DS Test Insight"
  short_description = "A test insight for data source"
  cloud_provider    = "gcp"
  categories        = ["FinOps"]
}

data "doit_insight" "test" {
  source_id   = doit_insight.test.source_id
  insight_key = doit_insight.test.key
}
`, rName)
}

func testAccInsightDataSourceNestedConfig(rName string) string {
	return fmt.Sprintf(`
resource "doit_insight" "test" {
  key               = %[1]q
  title             = "DS Nested Test Insight"
  short_description = "Test nested objects"
  cloud_provider    = "aws"
  categories        = ["Security"]
}

data "doit_insight" "test" {
  source_id   = doit_insight.test.source_id
  insight_key = doit_insight.test.key
}
`, rName)
}
