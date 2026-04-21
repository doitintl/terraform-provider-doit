package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// TestAccInsightDataSource_Basic tests reading a single insight by source and key.
func TestAccInsightDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccInsightDataSourceBasicConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_insight.test", "source_id", "public-api"),
					resource.TestCheckResourceAttr("data.doit_insight.test", "insight_key", "test-insight-1"),
					resource.TestCheckResourceAttr("data.doit_insight.test", "title", "Test Unused EC2 Instance"),
					resource.TestCheckResourceAttr("data.doit_insight.test", "cloud_provider", "aws"),
					resource.TestCheckResourceAttrSet("data.doit_insight.test", "key"),
					resource.TestCheckResourceAttrSet("data.doit_insight.test", "source"),
					resource.TestCheckResourceAttrSet("data.doit_insight.test", "display_status"),
				),
			},
			// Drift verification: re-apply should produce empty plan
			{
				Config: testAccInsightDataSourceBasicConfig(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccInsightDataSourceBasicConfig() string {
	return `
data "doit_insight" "test" {
  source_id   = "public-api"
  insight_key = "test-insight-1"
}
`
}
