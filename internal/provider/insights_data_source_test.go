package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// TestAccInsightsDataSource_Basic tests listing insights without filters.
func TestAccInsightsDataSource_Basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccInsightsDataSourceBasicConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_insights.test", "results.#"),
				),
			},
			// Drift verification: re-apply should produce empty plan
			{
				Config: testAccInsightsDataSourceBasicConfig(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccInsightsDataSourceBasicConfig() string {
	return `
data "doit_insights" "test" {
}
`
}

// TestAccInsightsDataSource_Filtered tests listing insights with a display_status filter.
func TestAccInsightsDataSource_Filtered(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testAccInsightsDataSourceFilteredConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.doit_insights.filtered", "results.#"),
				),
			},
			// Drift verification: re-apply should produce empty plan
			{
				Config: testAccInsightsDataSourceFilteredConfig(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccInsightsDataSourceFilteredConfig() string {
	return `
data "doit_insights" "filtered" {
  display_status = ["actionable"]
  category       = ["FinOps"]
}
`
}
